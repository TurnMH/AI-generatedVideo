package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/autovideo/video-service/internal/model"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// KafkaMessage is the shape of messages on video.generate.request.
type KafkaMessage struct {
	TaskID      int64    `json:"task_id"`
	VideoTaskID int64    `json:"video_task_id"`
	ImageURLs   []string `json:"image_urls"`
	ModelName   string   `json:"model_name"`
	MotionMode  string   `json:"motion_mode"`
	StylePreset string   `json:"style_preset"`
	// opt-p7: per-clip motion/camera descriptions from the storyboard, parallel to ImageURLs
	MotionDescs []string `json:"motion_descs,omitempty"`
}

// KafkaResultMessage is the shape of messages published to video.generate.result.
type KafkaResultMessage struct {
	TaskID      int64  `json:"task_id"`            // pipeline task ID from task-service (0 when not applicable)
	VideoTaskID int64  `json:"video_task_id"`
	Status      string `json:"status"`
	ResultURL   string `json:"result_url,omitempty"`
	ErrorMsg    string `json:"error_msg,omitempty"`
}

// KafkaConsumer reads from the video.generate.request topic and drives processing.
type KafkaConsumer struct {
	reader        *kafka.Reader
	writer        *kafka.Writer
	videoService  *VideoService
	logger        *zap.Logger
	maxKafkaTasks int
	wg            sync.WaitGroup // tracks in-flight task goroutines for graceful drain
}

// NewKafkaConsumer —— 创建 Kafka 消费者实例，配置 reader 和 writer
func NewKafkaConsumer(
	brokers []string,
	consumerGroup, consumerTopic, producerTopic string,
	videoService *VideoService,
	logger *zap.Logger,
	maxKafkaTasks int,
) *KafkaConsumer {
	if maxKafkaTasks <= 0 {
		maxKafkaTasks = 3
	}
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		GroupID:     consumerGroup,
		Topic:       consumerTopic,
		MinBytes:    1,
		MaxBytes:    10e6,
		StartOffset: kafka.LastOffset, // skip old messages on new consumer group
	})
	writer := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    producerTopic,
		Balancer: &kafka.LeastBytes{},
	}
	return &KafkaConsumer{
		reader:        reader,
		writer:        writer,
		videoService:  videoService,
		logger:        logger,
		maxKafkaTasks: maxKafkaTasks,
	}
}

// Start —— 开始消费 Kafka 消息，阻塞直到 ctx 被取消
// Start begins consuming messages. It blocks until ctx is cancelled.
//
// Design: messages are read eagerly from Kafka (committed immediately) and
// dispatched to goroutines that wait on the semaphore for an execution slot.
// This keeps the read loop non-blocking so Kafka heartbeats stay healthy and
// tasks are queued in memory (not stuck in the broker) even when all slots are busy.
func (c *KafkaConsumer) Start(ctx context.Context) {
	c.logger.Info("kafka consumer started", zap.Int("max_kafka_tasks", c.maxKafkaTasks))
	sem := make(chan struct{}, c.maxKafkaTasks)
	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				c.logger.Info("kafka consumer stopping")
				return
			}
			c.logger.Error("kafka read error", zap.Error(err))
			continue
		}

		// Dispatch to goroutine immediately — semaphore acquisition happens
		// inside the goroutine so the read loop is never blocked.
		c.wg.Add(1)
		go func(m kafka.Message) {
			defer c.wg.Done()
			// Wait for an execution slot; respect ctx cancellation so we don't leak goroutines.
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				// Shutdown while waiting for a slot. The Kafka message is already
				// committed; the video task is still in pending state (ProcessTask
				// was never called), so ResumePendingTasks at next startup will
				// re-dispatch it automatically.
				c.logger.Info("kafka consumer shutting down, skipping queued task")
				return
			}
			// Use an independent context so a service restart does not cancel
			// tasks that are already in-flight generating clips.
			c.handle(m)
		}(msg)
	}
}

// handle —— 处理单条 Kafka 消息，解析任务并调用视频服务执行
// handle uses context.Background() as task context root so that a graceful
// service shutdown (which cancels the consumer ctx) does not interrupt
// clip generation that is already in progress.
func (c *KafkaConsumer) handle(msg kafka.Message) {
	var km KafkaMessage
	if err := json.Unmarshal(msg.Value, &km); err != nil {
		c.logger.Error("unmarshal kafka message", zap.Error(err), zap.ByteString("raw", msg.Value))
		return
	}

	c.logger.Info("processing video task",
		zap.Int64("task_id", km.TaskID),
		zap.Int64("video_task_id", km.VideoTaskID),
		zap.String("model", km.ModelName),
		zap.Int("clips", len(km.ImageURLs)),
	)

	// Per-task timeout: generators poll for up to 15 min per clip; with up to 40 clips
	// at 3 concurrent that is ~13 rounds × 5 min worst-case = ~65 min, plus ffmpeg/upload.
	// Use 3 hours to safely cover large batches without leaving truly stuck tasks forever.
	// Root is context.Background() — NOT the consumer ctx — so service restarts do not
	// cancel tasks that are already generating clips.
	taskCtx, cancel := context.WithTimeout(context.Background(), 3*time.Hour)
	defer cancel()

	err := c.videoService.ProcessTask(taskCtx, km.VideoTaskID, km.ImageURLs, km.MotionDescs, km.ModelName, km.MotionMode, km.StylePreset)

	// Carry the original task-service TaskID through so task-service can correlate
	// the result back to its pipeline task record (task_id=0 when not from task-service).
	result := KafkaResultMessage{
		TaskID:      km.TaskID,
		VideoTaskID: km.VideoTaskID,
	}
	if err != nil {
		result.Status = model.StatusFailed
		result.ErrorMsg = err.Error()
		c.logger.Error("video task failed",
			zap.Int64("video_task_id", km.VideoTaskID),
			zap.Error(err))
	} else {
		getCtx, getCancel := context.WithTimeout(context.Background(), 10*time.Second)
		task, _ := c.videoService.GetTask(getCtx, km.VideoTaskID)
		getCancel()
		result.Status = model.StatusSucceeded
		if task != nil {
			result.ResultURL = task.ResultURL
		}
		c.logger.Info("video task succeeded", zap.Int64("video_task_id", km.VideoTaskID))
	}

	c.publishResult(result)
}

// Drain waits up to timeout for all in-flight task goroutines to finish.
// Call this after cancelling the Start context and before process exit so
// that clip generation is not interrupted by a planned restart.
func (c *KafkaConsumer) Drain(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		c.logger.Info("video task drain complete: all in-flight tasks finished")
	case <-time.After(timeout):
		c.logger.Warn("video task drain timed out: exiting with tasks still running",
			zap.Duration("timeout", timeout))
	}
}

// publishResult —— 将任务处理结果发布到 Kafka 结果 topic
// Uses a fresh background context so shutdown cancellation cannot prevent the result from being sent.
func (c *KafkaConsumer) publishResult(result KafkaResultMessage) {
	b, err := json.Marshal(result)
	if err != nil {
		c.logger.Error("marshal result", zap.Error(err))
		return
	}
	pubCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	key := []byte(fmt.Sprintf("%d", result.VideoTaskID))
	if err := c.writer.WriteMessages(pubCtx, kafka.Message{Key: key, Value: b}); err != nil {
		c.logger.Error("publish result", zap.Error(err))
	}
}

// Close —— 释放 Kafka 消费者和生产者资源
// Close releases consumer and producer resources.
func (c *KafkaConsumer) Close() error {
	_ = c.reader.Close()
	return c.writer.Close()
}
