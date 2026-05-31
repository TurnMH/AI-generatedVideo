package generators

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMinMaxGenerator_UsesExpectedEndpointsAndFields(t *testing.T) {
	var submitPath, submitQuery, pollPath, pollQuery, authHeader string
	var submitBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		switch r.Method {
		case http.MethodPost:
			submitPath = r.URL.Path
			submitQuery = r.URL.RawQuery
			submitBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"task_id":"task_mm_1","base_resp":{"status_code":"0","status_msg":"ok"}}`)
		case http.MethodGet:
			pollPath = r.URL.Path
			pollQuery = r.URL.RawQuery
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"status":"Success","videoDownLoadUrl":"https://example.com/mm.mp4"}`)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	gen := NewMinMaxGenerator("mm-key", server.URL, "MiniMax-Hailuo-02")
	clip, err := gen.Generate(context.Background(), VideoGenerateReq{
		Prompt:         "test prompt",
		SourceImageURL: "https://example.com/start.jpg",
		TailImageURL:   "https://example.com/end.jpg",
		DurationSec:    6,
		Resolution:     "768P",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if clip == nil || clip.ClipURL != "https://example.com/mm.mp4" {
		t.Fatalf("unexpected clip %#v", clip)
	}
	if submitPath != "/api/v3/minimax/hailuo-02/standard" {
		t.Fatalf("expected std endpoint, got path=%q query=%q", submitPath, submitQuery)
	}
	if pollPath != "/api/v3/predictions/task_mm_1/result" || pollQuery != "" {
		t.Fatalf("expected poll /api/v3/predictions/task_mm_1/result, got %q?%q", pollPath, pollQuery)
	}
	if authHeader != "Bearer mm-key" {
		t.Fatalf("expected Authorization Bearer mm-key, got %q", authHeader)
	}

	var body map[string]any
	if err := json.Unmarshal(submitBody, &body); err != nil {
		t.Fatalf("unmarshal submit body: %v body=%s", err, string(submitBody))
	}
	if got := body["model"]; got != "MiniMax-Hailuo-02" {
		t.Fatalf("expected model, got %#v", got)
	}
	if got := body["image"]; got != "https://example.com/start.jpg" {
		t.Fatalf("expected WaveSpeed image field, got %#v", got)
	}
	if got := body["end_image"]; got != "https://example.com/end.jpg" {
		t.Fatalf("expected WaveSpeed end_image field, got %#v", got)
	}
	if got := body["first_frame_image"]; got != "https://example.com/start.jpg" {
		t.Fatalf("expected MiniMax compatibility first_frame_image, got %#v", got)
	}
	if got := body["last_frame_image"]; got != "https://example.com/end.jpg" {
		t.Fatalf("expected MiniMax compatibility last_frame_image, got %#v", got)
	}
}

func TestMinMaxGenerator_Hailuo23_UsesFamilyMatchedFastEndpoint(t *testing.T) {
	var submitPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			submitPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"task_id":"task_mm_23","base_resp":{"status_code":"0","status_msg":"ok"}}`)
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"status":"Success","videoDownLoadUrl":"https://example.com/mm23.mp4"}`)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	gen := NewMinMaxGenerator("mm-key", server.URL, "MiniMax-Hailuo-2.3")
	clip, err := gen.Generate(context.Background(), VideoGenerateReq{
		Prompt:         "test prompt",
		SourceImageURL: "https://example.com/start.jpg",
		DurationSec:    6,
		Resolution:     "512P",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if clip == nil || clip.ClipURL != "https://example.com/mm23.mp4" {
		t.Fatalf("unexpected clip %#v", clip)
	}
	if submitPath != "/api/v3/minimax/hailuo-2.3/fast" {
		t.Fatalf("expected hailuo-2.3 fast endpoint from dumps1-compatible model family, got %q", submitPath)
	}
}

func TestNormalizeMinMaxEndpoint_LegacyAliases(t *testing.T) {
	cases := map[string]string{
		"/api/v3/minimax/hailuo-02/i2v-standard": "/api/v3/minimax/hailuo-02/standard",
		"/api/v3/minimax/hailuo-02/i2v-pro":      "/api/v3/minimax/hailuo-02/pro",
		"/api/v3/minimax/hailuo-02/i2v-fast":     "/api/v3/minimax/hailuo-02/fast",
		"/api/v3/minimax/hailuo-2.3/i2v-fast":    "/api/v3/minimax/hailuo-2.3/fast",
	}
	for in, want := range cases {
		if got := normalizeMinMaxEndpoint(in); got != want {
			t.Fatalf("normalizeMinMaxEndpoint(%q) = %q, want %q", in, got, want)
		}
	}
}
