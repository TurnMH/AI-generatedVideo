package generators

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSuannengImg2VideoUsesDedicatedFirstFrameFlow(t *testing.T) {
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
	if len(body.Content) != 2 {
		t.Fatalf("content len = %d, want 2", len(body.Content))
	}
	if body.Content[1].Role != "first_frame" || body.Content[1].ImageURL == nil || body.Content[1].ImageURL.URL != "https://example.com/source.png" {
		t.Fatalf("first_frame item = %#v", body.Content[1])
	}
}

func TestSuannengReference2VideoCarriesTypedReferencesAndAudio(t *testing.T) {
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
		GenerateMode:        "reference2video",
		GenerateAudio:       true,
		CharacterReferences: []MediaReference{{URL: "asset://asset-456", Text: "林夏", Role: "reference_image", Index: 1}},
		AudioReferences:     []MediaReference{{URL: "https://example.com/linxia.wav", Text: "林夏", Role: "reference_audio", Index: 1}},
		VoiceText:           "林夏：今天给大家看一款新品。",
		DurationSec:         5,
	})
	if err != nil {
		t.Fatalf("submit returned error: %v", err)
	}
	if len(body.Content) != 3 {
		t.Fatalf("content len = %d, want 3", len(body.Content))
	}
	if body.Content[1].Role != "reference_image" || body.Content[1].Text != "林夏" || body.Content[1].Index != 1 {
		t.Fatalf("reference_image = %#v", body.Content[1])
	}
	if body.Content[2].Role != "reference_audio" || body.Content[2].AudioURL == nil || body.Content[2].AudioURL.URL != "https://example.com/linxia.wav" {
		t.Fatalf("reference_audio = %#v", body.Content[2])
	}
}
