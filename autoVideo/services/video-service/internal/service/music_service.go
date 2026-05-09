package service

// music_service.go — Kafka consumer for music.generate.request
// Calls an OpenAI-compatible audio/speech API (e.g. SiliconFlow),
// uploads the result to storage-service, then publishes a result message
// to music.generate.result so the task-service can mark the task done.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// MusicKafkaMessage is the payload published on music.generate.request.
type MusicKafkaMessage struct {
	TaskID      int64  `json:"task_id"`
	ProjectID   int64  `json:"project_id"`
	ModelName   string `json:"model_name"`
	Title       string `json:"title"`
	Prompt      string `json:"prompt"`
	Mood        string `json:"mood"`
	Instruments string `json:"instruments"`
	DurationSec int    `json:"duration_sec"`
	HasVocals   bool   `json:"has_vocals"`
	Lyrics      string `json:"lyrics"`
}

// MusicResultMessage is the payload published to music.generate.result.
type MusicResultMessage struct {
	TaskID    int64  `json:"task_id"`
	Status    string `json:"status"`
	ResultURL string `json:"result_url,omitempty"`
	ErrorMsg  string `json:"error_msg,omitempty"`
}

// MusicKafkaConsumer consumes music generation tasks from Kafka.
type MusicKafkaConsumer struct {
	reader     *kafka.Reader
	writer     *kafka.Writer
	apiKey     string
	apiBase    string
	apiModel   string
	storageURL string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewMusicKafkaConsumer creates a new MusicKafkaConsumer.
// apiKey/apiBase/apiModel configure the audio generation API.
// storageURL is the base URL of the storage-service for audio uploads.
func NewMusicKafkaConsumer(
	brokers []string,
	apiKey, apiBase, apiModel, storageURL string,
	logger *zap.Logger,
) *MusicKafkaConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		GroupID:     "music-service",
		Topic:       "music.generate.request",
		MinBytes:    1,
		MaxBytes:    1e6,
		StartOffset: kafka.FirstOffset,
	})
	writer := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    "music.generate.result",
		Balancer: &kafka.LeastBytes{},
	}
	return &MusicKafkaConsumer{
		reader:     reader,
		writer:     writer,
		apiKey:     apiKey,
		apiBase:    apiBase,
		apiModel:   apiModel,
		storageURL: storageURL,
		httpClient: &http.Client{Timeout: 5 * time.Minute},
		logger:     logger,
	}
}

// Start begins consuming messages until ctx is cancelled.
func (c *MusicKafkaConsumer) Start(ctx context.Context) {
	c.logger.Info("music kafka consumer started")
	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				c.logger.Info("music kafka consumer stopping")
				return
			}
			c.logger.Error("music kafka read error", zap.Error(err))
			continue
		}
		go c.handle(ctx, msg)
	}
}

func (c *MusicKafkaConsumer) handle(ctx context.Context, msg kafka.Message) {
	var km MusicKafkaMessage
	if err := json.Unmarshal(msg.Value, &km); err != nil {
		c.logger.Error("music: unmarshal message failed", zap.Error(err), zap.ByteString("raw", msg.Value))
		return
	}

	// task-service publishes raw Task JSON with shape: {"id":N,"task_type":"music_generate","payload":{...}}.
	// The fields we need (task_id, project_id, model_name, prompt, …) live in the nested payload.
	// Detect this format when task_id is missing and try to extract from payload.
	if km.TaskID == 0 {
		var envelope struct {
			ID      int64           `json:"id"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(msg.Value, &envelope); err == nil && envelope.ID != 0 {
			var inner MusicKafkaMessage
			if err := json.Unmarshal(envelope.Payload, &inner); err == nil {
				inner.TaskID = envelope.ID
				km = inner
				c.logger.Info("music: parsed task-service envelope format", zap.Int64("task_id", km.TaskID))
			}
		}
	}

	if km.TaskID == 0 {
		c.logger.Error("music: message has no task_id, skipping", zap.ByteString("raw", msg.Value))
		return
	}

	c.logger.Info("music: processing task", zap.Int64("task_id", km.TaskID), zap.String("model", km.ModelName))

	audioURL, err := c.generateAndUpload(ctx, &km)

	result := MusicResultMessage{TaskID: km.TaskID}
	if err != nil {
		c.logger.Error("music: generation failed", zap.Int64("task_id", km.TaskID), zap.Error(err))
		result.Status = "failed"
		result.ErrorMsg = err.Error()
	} else {
		result.Status = "succeeded"
		result.ResultURL = audioURL
	}

	data, _ := json.Marshal(result)
	_ = c.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(fmt.Sprintf("%d", km.TaskID)),
		Value: data,
	})
}

// generateAndUpload calls the audio API, downloads the audio bytes,
// and uploads them to storage-service, returning the public URL.
func (c *MusicKafkaConsumer) generateAndUpload(ctx context.Context, km *MusicKafkaMessage) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("music API key not configured; set models.music_key in config.local.yaml")
	}

	// Build natural-language prompt from structured fields.
	prompt := buildMusicPrompt(km)

	modelName := km.ModelName
	if modelName == "" {
		modelName = c.apiModel
	}

	audioBytes, err := c.callAudioAPI(ctx, modelName, prompt)
	if err != nil {
		return "", fmt.Errorf("audio API: %w", err)
	}

	url, err := c.uploadAudio(ctx, km.TaskID, km.ProjectID, audioBytes)
	if err != nil {
		return "", fmt.Errorf("upload audio: %w", err)
	}
	return url, nil
}

// callAudioAPI calls a SiliconFlow-compatible /v1/audio/speech endpoint.
// The response body is raw audio bytes (mp3 or wav).
func (c *MusicKafkaConsumer) callAudioAPI(ctx context.Context, modelName, prompt string) ([]byte, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"model":           modelName,
		"input":           prompt,
		"voice":           "default",
		"response_format": "mp3",
	})

	apiURL := c.apiBase + "/v1/audio/speech"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(errBody))
	}

	return io.ReadAll(resp.Body)
}

// uploadAudio uploads raw audio bytes to storage-service and returns the public URL.
func (c *MusicKafkaConsumer) uploadAudio(ctx context.Context, taskID, projectID int64, audioBytes []byte) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("bucket", "audios")
	_ = w.WriteField("user_id", "1")
	_ = w.WriteField("project_id", fmt.Sprintf("%d", projectID))
	_ = w.WriteField("category", "music")
	fw, err := w.CreateFormFile("file", fmt.Sprintf("music_%d_%d.mp3", taskID, time.Now().Unix()))
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(audioBytes); err != nil {
		return "", err
	}
	w.Close()

	uploadURL := fmt.Sprintf("%s/api/v1/storage/upload", c.storageURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("storage upload: status %d — %s", resp.StatusCode, string(respBody))
	}

	var ur struct {
		Data struct {
			CdnURL    string `json:"cdn_url"`
			ObjectKey string `json:"object_key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &ur); err == nil && ur.Data.CdnURL != "" {
		return ur.Data.CdnURL, nil
	}
	return "", fmt.Errorf("no url in storage response: %s", string(respBody))
}

// buildMusicPrompt creates a single text prompt from structured music fields.
func buildMusicPrompt(km *MusicKafkaMessage) string {
	parts := []string{}
	if km.Title != "" {
		parts = append(parts, "Title: "+km.Title)
	}
	if km.Mood != "" {
		parts = append(parts, "Mood: "+km.Mood)
	}
	if km.Instruments != "" {
		parts = append(parts, "Instruments: "+km.Instruments)
	}
	if km.HasVocals {
		parts = append(parts, "Has vocals: yes")
	}
	if km.Lyrics != "" {
		parts = append(parts, "Lyrics: "+km.Lyrics)
	}
	if km.Prompt != "" {
		parts = append(parts, km.Prompt)
	}
	if km.DurationSec > 0 {
		parts = append(parts, fmt.Sprintf("Duration: %d seconds", km.DurationSec))
	}
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ". "
		}
		result += p
	}
	return result
}
