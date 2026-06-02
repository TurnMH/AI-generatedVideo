package generators

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSuannengImg2VideoCarriesSourceAndReferenceImages(t *testing.T) {
	var body suannengSubmitReq
	g := NewSuannengGenerator("test-key", "https://example.com/tasks", "doubao-seedance-1-5-pro-251215")
	g.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		defer r.Body.Close()
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("unmarshal submit body: %v", err)
		}
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"id":"task-1"}`)),
			Header:     make(http.Header),
		}, nil
	})}

	_, err := g.submit(context.Background(), VideoGenerateReq{
		Prompt:             "保持同一人物形象稳定",
		GenerateMode:       "img2video",
		SourceImageURL:     "https://example.com/source.png",
		CharacterImageURLs: []string{"asset://asset-456", "https://example.com/ref-b.png"},
		DurationSec:        5,
	})
	if err != nil {
		t.Fatalf("submit returned error: %v", err)
	}
	if len(body.Content) != 4 {
		t.Fatalf("content len = %d, want 4", len(body.Content))
	}
	if body.Content[0].ImageURL == nil || body.Content[0].ImageURL.URL != "https://example.com/source.png" {
		t.Fatalf("source item = %#v", body.Content[0])
	}
	if body.Content[2].Role != "reference_image" || body.Content[2].ImageURL == nil || body.Content[2].ImageURL.URL != "asset://asset-456" {
		t.Fatalf("reference_image[0] = %#v", body.Content[2])
	}
}
