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
	if len(params) != 7 {
		t.Fatalf("expected 7 params for veo, got %d", len(params))
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
	if params[3].Key != "person_generation" || params[3].Default != "allow_adult" {
		t.Fatalf("unexpected person_generation param: %+v", params[3])
	}
	if params[4].Key != "seed" || params[5].Key != "reference_images" || params[6].Key != "last_frame" {
		t.Fatalf("unexpected advanced params: %+v", params)
	}
}

func TestHubagiGeneratorSubmitIncludesVeoOptions(t *testing.T) {
	type capturedReferenceImage struct {
		ImageURL string `json:"image_url"`
	}
	type capturedLastFrame struct {
		ImageURL string `json:"image_url"`
	}
	type capturedReq struct {
		Model            string                   `json:"model"`
		ImageURL         string                   `json:"image_url"`
		Prompt           string                   `json:"prompt"`
		Duration         int                      `json:"duration"`
		AspectRatio      string                   `json:"aspect_ratio"`
		Resolution       string                   `json:"resolution"`
		Seed             int                      `json:"seed"`
		PersonGeneration string                   `json:"person_generation"`
		ReferenceImages  []capturedReferenceImage `json:"reference_images"`
		LastFrame        *capturedLastFrame       `json:"last_frame"`
		NumberOfVideos   int                      `json:"number_of_videos"`
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
		SourceImageURL:    "https://example.com/input.png",
		Prompt:            "test prompt",
		DurationSec:       6,
		AspectRatio:       "9:16",
		Resolution:        "1080P",
		Seed:              42,
		PersonGeneration:  "allow_adult",
		ReferenceImages:   []string{"https://example.com/ref-1.png", "https://example.com/ref-1.png", "https://example.com/ref-2.png"},
		LastFrameImageURL: "https://example.com/last.png",
		NumberOfVideos:    1,
	})
	if err != nil {
		t.Fatalf("submit returned error: %v", err)
	}
	if got.Model != "voe3.1" || got.ImageURL == "" || got.Prompt == "" {
		t.Fatalf("unexpected basic request payload: %+v", got)
	}
	if got.Duration != 8 {
		t.Fatalf("expected duration coerced to 8 for 1080p/reference mode, got %d", got.Duration)
	}
	if got.AspectRatio != "9:16" {
		t.Fatalf("expected aspect_ratio 9:16, got %q", got.AspectRatio)
	}
	if got.Resolution != "1080p" {
		t.Fatalf("expected normalized resolution 1080p, got %q", got.Resolution)
	}
	if got.Seed != 42 {
		t.Fatalf("expected seed 42, got %d", got.Seed)
	}
	if got.PersonGeneration != "allow_adult" {
		t.Fatalf("expected person_generation allow_adult, got %q", got.PersonGeneration)
	}
	if len(got.ReferenceImages) != 2 {
		t.Fatalf("expected 2 deduped reference_images, got %+v", got.ReferenceImages)
	}
	if got.LastFrame == nil || got.LastFrame.ImageURL != "https://example.com/last.png" {
		t.Fatalf("unexpected last_frame payload: %+v", got.LastFrame)
	}
	if got.NumberOfVideos != 1 {
		t.Fatalf("expected number_of_videos 1, got %d", got.NumberOfVideos)
	}
}
