package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestBuildGeminiChannelsNormalizesBasesAndFallsBack(t *testing.T) {
	channels := buildGeminiChannels([]string{
		"https://api.easyart.cc/v1/",
		"https://poloai.top/v1beta",
	}, []string{"key-1", "key-2", "key-3"})

	if got, want := len(channels), 3; got != want {
		t.Fatalf("len(channels) = %d, want %d", got, want)
	}

	wantBases := []string{
		"https://api.easyart.cc",
		"https://poloai.top",
		"https://poloai.top",
	}
	wantKeys := []string{"key-1", "key-2", "key-3"}
	for i, channel := range channels {
		if channel.base != wantBases[i] {
			t.Fatalf("channel[%d].base = %q, want %q", i, channel.base, wantBases[i])
		}
		if channel.key != wantKeys[i] {
			t.Fatalf("channel[%d].key = %q, want %q", i, channel.key, wantKeys[i])
		}
	}
}

func TestFetchGeminiPartsFromChannelsRetriesLaterChannel(t *testing.T) {
	firstCalls := 0
	firstServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstCalls++
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"not found"}}`))
	}))
	defer firstServer.Close()

	secondCalls := 0
	secondServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondCalls++
		if got, want := r.URL.Path, "/v1beta/models/gemini-test:generateContent"; got != want {
			t.Fatalf("request path = %q, want %q", got, want)
		}
		if got, want := r.URL.Query().Get("key"), "key-2"; got != want {
			t.Fatalf("request key = %q, want %q", got, want)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"},{"inlineData":{"mimeType":"image/png","data":"YWJj"}}]}}]}`))
	}))
	defer secondServer.Close()

	parts, err := fetchGeminiPartsFromChannels(
		context.Background(),
		&http.Client{Timeout: time.Second},
		[]geminiChannel{
			{base: firstServer.URL, key: "key-1"},
			{base: secondServer.URL, key: "key-2"},
		},
		"gemini-test",
		[]geminiMessage{{Role: "user", Content: "hello"}},
		zap.NewNop(),
	)
	if err != nil {
		t.Fatalf("fetchGeminiPartsFromChannels() error = %v", err)
	}
	if firstCalls != 1 {
		t.Fatalf("first server calls = %d, want 1", firstCalls)
	}
	if secondCalls != 1 {
		t.Fatalf("second server calls = %d, want 1", secondCalls)
	}
	if got, want := len(parts), 2; got != want {
		t.Fatalf("len(parts) = %d, want %d", got, want)
	}
	if parts[0].Type != "text" || parts[0].Text != "ok" {
		t.Fatalf("first part = %#v, want text ok", parts[0])
	}
	if parts[1].Type != "image" || parts[1].MimeType != "image/png" || parts[1].Data != "YWJj" {
		t.Fatalf("second part = %#v, want image/png YWJj", parts[1])
	}
}