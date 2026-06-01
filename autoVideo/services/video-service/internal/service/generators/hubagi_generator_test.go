package generators

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHubagiGeneratorParamOptionsForVeo(t *testing.T) {
	g := NewHubagiGenerator("key", "https://example.com", "voe3.1")
	params := g.ParamOptions()
	if len(params) != 3 {
		t.Fatalf("expected 3 params for veo, got %d", len(params))
	}
	if params[0].Key != "duration" || params[0].Default != "8" {
		t.Fatalf("unexpected duration param: %+v", params[0])
	}
	if params[1].Key != "aspect_ratio" || len(params[1].Values) != 2 {
		t.Fatalf("unexpected aspect ratio param: %+v", params[1])
	}
	if params[2].Key != "resolution" || len(params[2].Values) != 3 {
		t.Fatalf("unexpected resolution param: %+v", params[2])
	}
}

func TestHubagiGeneratorSubmitIncludesVeoOptions(t *testing.T) {
	type capturedReq struct {
		Model       string `json:"model"`
		ImageURL    string `json:"image_url"`
		Prompt      string `json:"prompt"`
		Duration    int    `json:"duration"`
		AspectRatio string `json:"aspect_ratio"`
		Resolution  string `json:"resolution"`
	}

	var got capturedReq
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/video/generations" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":123,"status":"pending"}`))
	}))
	defer server.Close()

	g := NewHubagiGenerator("key", server.URL, "voe3.1")
	_, err := g.submit(context.Background(), VideoGenerateReq{
		SourceImageURL: "https://example.com/input.png",
		Prompt:         "test prompt",
		DurationSec:    6,
		AspectRatio:    "9:16",
		Resolution:     "1080P",
	})
	if err != nil {
		t.Fatalf("submit returned error: %v", err)
	}
	if got.Model != "voe3.1" || got.ImageURL == "" || got.Prompt == "" {
		t.Fatalf("unexpected basic request payload: %+v", got)
	}
	if got.Duration != 6 {
		t.Fatalf("expected duration 6, got %d", got.Duration)
	}
	if got.AspectRatio != "9:16" {
		t.Fatalf("expected aspect_ratio 9:16, got %q", got.AspectRatio)
	}
	if got.Resolution != "1080p" {
		t.Fatalf("expected normalized resolution 1080p, got %q", got.Resolution)
	}
}
