package generators

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKlingGenerator_AiPing_UsesUnifiedVideosEndpoints(t *testing.T) {
	var submitPath, queryPath, authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		switch {
		case r.Method == http.MethodPost:
			submitPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"code":0,"data":{"task_id":"task_123"}}`)
		case r.Method == http.MethodGet:
			queryPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"code":0,"status":"completed","video_url":"https://example.com/out.mp4"}`)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	gen := NewKlingGeneratorWithKeys(server.URL, "QC-test-key")
	gen.WithName("aiping")
	gen.WithModel("kling-v3")

	clip, err := gen.Generate(context.Background(), VideoGenerateReq{
		Prompt:         "test",
		SourceImageURL: "https://example.com/start.jpg",
		TailImageURL:   "https://example.com/end.jpg",
		DurationSec:    5,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if clip == nil || clip.ClipURL == "" {
		t.Fatalf("expected clip url, got %#v", clip)
	}
	if submitPath != "/videos" {
		t.Fatalf("expected submit path /videos, got %q", submitPath)
	}
	if queryPath != "/videos/task_123" {
		t.Fatalf("expected query path /videos/task_123, got %q", queryPath)
	}
	if authHeader != "Bearer QC-test-key" {
		t.Fatalf("expected Authorization Bearer QC-test-key, got %q", authHeader)
	}
}

func TestKlingGenerator_AiPing_SubmitBody_UsesOfficialFields(t *testing.T) {
	var bodyBytes []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			bodyBytes, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"code":0,"data":{"task_id":"task_123"}}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":0,"status":"completed","video_url":"https://example.com/out.mp4"}`)
	}))
	defer server.Close()

	gen := NewKlingGeneratorWithKeys(server.URL, "QC-test-key")
	gen.WithName("aiping")
	gen.WithModel("kling-v3")

	_, err := gen.Generate(context.Background(), VideoGenerateReq{
		Prompt:         "test",
		SourceImageURL: "https://example.com/start.jpg",
		TailImageURL:   "https://example.com/end.jpg",
		DurationSec:    5,
		AspectRatio:    "16:9",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		t.Fatalf("unmarshal body: %v\nbody=%s", err, string(bodyBytes))
	}
	if got := body["model"]; got != "Kling-V3" {
		t.Fatalf("expected model Kling-V3, got %#v", got)
	}
	if got := body["image"]; got != "https://example.com/start.jpg" {
		t.Fatalf("expected image field, got %#v", got)
	}
	if got := body["image_tail"]; got != "https://example.com/end.jpg" {
		t.Fatalf("expected image_tail field, got %#v", got)
	}
	if got := body["seconds"]; got != "5" {
		t.Fatalf("expected seconds=5, got %#v", got)
	}
	if _, ok := body["duration"]; ok {
		t.Fatalf("expected duration omitted for aiping official body, got body=%s", string(bodyBytes))
	}
	imgList, ok := body["image_list"].([]any)
	if !ok || len(imgList) != 2 {
		t.Fatalf("expected image_list with 2 refs, got %#v", body["image_list"])
	}
	if !strings.Contains(string(bodyBytes), "first_frame") || !strings.Contains(string(bodyBytes), "last_frame") {
		t.Fatalf("expected typed image_list refs in body, got %s", string(bodyBytes))
	}
}

func TestKlingGenerator_BearerTokenNotDoublePrefixed(t *testing.T) {
	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = io.WriteString(w, `{"code":0,"data":{"task_id":"task_123"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"code":0,"status":"completed","video_url":"https://example.com/out.mp4"}`)
	}))
	defer server.Close()

	gen := NewKlingGeneratorWithKeys(server.URL, "Bearer already-prefixed")
	gen.WithName("aiping")
	gen.WithModel("kling-v3")
	_, err := gen.Generate(context.Background(), VideoGenerateReq{Prompt: "test", SourceImageURL: "https://example.com/start.jpg"})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if authHeader != "Bearer already-prefixed" {
		t.Fatalf("expected no double Bearer prefix, got %q", authHeader)
	}
}
