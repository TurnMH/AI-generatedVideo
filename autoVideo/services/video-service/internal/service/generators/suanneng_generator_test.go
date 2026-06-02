package generators

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSuannengImg2VideoCarriesTypedReferencesAndAudio(t *testing.T) {
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
			Body:       io.NopCloser(strings.NewReader(`{"id":"task-2"}`)),
			Header:     make(http.Header),
		}, nil
	})}

	_, err := g.submit(context.Background(), VideoGenerateReq{
		Prompt:              "林夏口播新品亮点",
		GenerateMode:        "img2video",
		GenerateAudio:       true,
		SourceImageURL:      "https://example.com/source.png",
		CharacterReferences: []MediaReference{{URL: "asset://asset-456", Text: "林夏", Role: "reference_image", Index: 1}},
		AudioReferences:     []MediaReference{{URL: "https://example.com/linxia.wav", Text: "林夏", Role: "reference_audio", Index: 1}},
		VoiceText:           "林夏：今天给大家看一款新品。",
		DurationSec:         5,
	})
	if err != nil {
		t.Fatalf("submit returned error: %v", err)
	}
	if len(body.Content) != 4 {
		t.Fatalf("content len = %d, want 4", len(body.Content))
	}
	if body.Content[2].Text != "林夏" || body.Content[2].Index != 1 {
		t.Fatalf("reference_image = %#v", body.Content[2])
	}
	if body.Content[3].Role != "reference_audio" || body.Content[3].AudioURL == nil || body.Content[3].AudioURL.URL != "https://example.com/linxia.wav" {
		t.Fatalf("reference_audio = %#v", body.Content[3])
	}
}
