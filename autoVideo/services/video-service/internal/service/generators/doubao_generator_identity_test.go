package generators

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDoubaoStartEnd2VideoForSeedanceDoesNotMixReferenceImages(t *testing.T) {
	var body doubaoSubmitReq
	g := NewDoubaoSeedanceGenerator("test-key", "https://example.com", "doubao-seedance-1-5-pro-251215", "doubao-seedance")
	g.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		defer r.Body.Close()
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("unmarshal submit body: %v", err)
		}
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"id":"task-1","status":"queued"}`)),
			Header:     make(http.Header),
		}, nil
	})}

	_, err := g.submit(context.Background(), VideoGenerateReq{
		Prompt:             "保持同一人物形象稳定",
		GenerateMode:       "startEnd2video",
		SourceImageURL:     "https://example.com/first.png",
		TailImageURL:       "https://example.com/last.png",
		CharacterImageURLs: []string{"https://example.com/ref-a.png", "https://example.com/ref-b.png"},
		DurationSec:        5,
		AspectRatio:        "16:9",
		Resolution:         "720p",
	})
	if err != nil {
		t.Fatalf("submit returned error: %v", err)
	}

	if len(body.Content) != 3 {
		t.Fatalf("content len = %d, want 3", len(body.Content))
	}
	if body.Content[1].Role != "first_frame" || body.Content[1].ImageURL == nil || body.Content[1].ImageURL.URL != "https://example.com/first.png" {
		t.Fatalf("first_frame item = %#v", body.Content[1])
	}
	if body.Content[2].Role != "last_frame" || body.Content[2].ImageURL == nil || body.Content[2].ImageURL.URL != "https://example.com/last.png" {
		t.Fatalf("last_frame item = %#v", body.Content[2])
	}
}

func TestDoubaoSeedanceImg2VideoDoesNotMixReferenceImages(t *testing.T) {
	var body doubaoSubmitReq
	g := NewDoubaoSeedanceGenerator("test-key", "https://example.com", "doubao-seedance-1-5-pro-251215", "doubao-seedance")
	g.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		defer r.Body.Close()
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("unmarshal submit body: %v", err)
		}
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"id":"task-2","status":"queued"}`)),
			Header:     make(http.Header),
		}, nil
	})}

	_, err := g.submit(context.Background(), VideoGenerateReq{
		Prompt:             "保持同一人物形象稳定",
		GenerateMode:       "img2video",
		SourceImageURL:     "https://example.com/source.png",
		CharacterImageURLs: []string{"https://example.com/ref-a.png", "https://example.com/ref-b.png"},
		DurationSec:        5,
		AspectRatio:        "16:9",
		Resolution:         "720p",
	})
	if err != nil {
		t.Fatalf("submit returned error: %v", err)
	}

	if len(body.Content) != 2 {
		t.Fatalf("content len = %d, want 2", len(body.Content))
	}
	if body.Content[0].Role != "" || body.Content[0].ImageURL == nil || body.Content[0].ImageURL.URL != "https://example.com/source.png" {
		t.Fatalf("source item = %#v", body.Content[0])
	}
	if body.Content[1].Type != "text" {
		t.Fatalf("text item = %#v", body.Content[1])
	}
}

func TestDoubaoSeedanceReference2VideoCarriesReferenceImages(t *testing.T) {
	var body doubaoSubmitReq
	g := NewDoubaoSeedanceGenerator("test-key", "https://example.com", "doubao-seedance-1-5-pro-251215", "doubao-seedance")
	g.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		defer r.Body.Close()
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("unmarshal submit body: %v", err)
		}
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"id":"task-ref","status":"queued"}`)),
			Header:     make(http.Header),
		}, nil
	})}

	_, err := g.submit(context.Background(), VideoGenerateReq{
		Prompt:             "保持同一人物形象稳定",
		GenerateMode:       "reference2video",
		SourceImageURL:     "https://example.com/source.png",
		CharacterImageURLs: []string{"https://example.com/ref-a.png", "https://example.com/ref-b.png"},
		DurationSec:        5,
		AspectRatio:        "16:9",
		Resolution:         "720p",
	})
	if err != nil {
		t.Fatalf("submit returned error: %v", err)
	}
	if len(body.Content) != 3 {
		t.Fatalf("content len = %d, want 3", len(body.Content))
	}
	if body.Content[0].Type != "text" {
		t.Fatalf("text item = %#v", body.Content[0])
	}
	for i, want := range []string{"https://example.com/ref-a.png", "https://example.com/ref-b.png"} {
		item := body.Content[1+i]
		if item.Role != "reference_image" || item.ImageURL == nil || item.ImageURL.URL != want {
			t.Fatalf("reference_image[%d] = %#v, want %s", i, item, want)
		}
	}
}

func TestDoubaoPlainImg2VideoKeepsRolelessSourceForNonSeedance(t *testing.T) {
	var body doubaoSubmitReq
	g := NewDoubaoGenerator("test-key", "https://example.com", "V4.0", "doubao")
	g.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		defer r.Body.Close()
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("unmarshal submit body: %v", err)
		}
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"id":"task-3","status":"queued"}`)),
			Header:     make(http.Header),
		}, nil
	})}

	_, err := g.submit(context.Background(), VideoGenerateReq{
		Prompt:             "保持同一人物形象稳定",
		GenerateMode:       "img2video",
		SourceImageURL:     "https://example.com/source.png",
		CharacterImageURLs: []string{"https://example.com/ref-a.png"},
		DurationSec:        5,
	})
	if err != nil {
		t.Fatalf("submit returned error: %v", err)
	}
	if body.Content[0].Role != "" {
		t.Fatalf("non-seedance source role = %q, want empty", body.Content[0].Role)
	}
}
