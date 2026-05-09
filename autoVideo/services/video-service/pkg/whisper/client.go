// Package whisper provides a client for the whisper-sidecar transcription service (feat-4).
// The sidecar runs faster-whisper and exposes POST /transcribe.
package whisper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client calls the whisper-sidecar REST API.
type Client struct {
	baseURL string
	client  *http.Client
}

// NewClient creates a Whisper client. baseURL should be e.g. "http://whisper-sidecar:8010".
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 10 * time.Minute}, // transcription can take a while
	}
}

// TranscribeRequest is the request body for POST /transcribe.
type TranscribeRequest struct {
	AudioURL      string `json:"audio_url"`
	Language      string `json:"language"`
	ReferenceText string `json:"reference_text,omitempty"`
}

// Segment is a single timed subtitle segment.
type Segment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// TranscribeResponse is returned by the sidecar.
type TranscribeResponse struct {
	SRT      string    `json:"srt"`
	Segments []Segment `json:"segments"`
	Language string    `json:"language"`
}

// Transcribe calls the sidecar to transcribe the given audio URL.
func (c *Client) Transcribe(ctx context.Context, audioURL, language string) (*TranscribeResponse, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("whisper sidecar URL not configured")
	}
	body, err := json.Marshal(TranscribeRequest{AudioURL: audioURL, Language: language})
	if err != nil {
		return nil, fmt.Errorf("whisper: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/transcribe", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("whisper: transcribe request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("whisper sidecar error %d: %s", resp.StatusCode, string(raw))
	}

	var result TranscribeResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("whisper: parse response: %w", err)
	}
	return &result, nil
}

// IsAvailable performs a lightweight health check against the sidecar.
func (c *Client) IsAvailable(ctx context.Context) bool {
	if c.baseURL == "" {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
