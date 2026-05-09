package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── mockNetError implements net.Error for isRetryableError tests ──────────────

type mockNetError struct{ timeout bool }

func (e *mockNetError) Error() string   { return "mock network error" }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return true }

// ── isRetryableError ──────────────────────────────────────────────────────────

func TestIsRetryableError_HTTP429(t *testing.T) {
	assert.True(t, isRetryableError(&httpStatusError{code: http.StatusTooManyRequests}))
}

func TestIsRetryableError_HTTP503(t *testing.T) {
	assert.True(t, isRetryableError(&httpStatusError{code: http.StatusServiceUnavailable}))
}

func TestIsRetryableError_HTTP400(t *testing.T) {
	assert.False(t, isRetryableError(&httpStatusError{code: http.StatusBadRequest}))
}

func TestIsRetryableError_ContextCanceled(t *testing.T) {
	assert.False(t, isRetryableError(context.Canceled))
}

func TestIsRetryableError_NetError(t *testing.T) {
	assert.True(t, isRetryableError(&mockNetError{}))
}

// ── retryLLMCall ─────────────────────────────────────────────────────────────

func TestRetryLLMCall_SuccessFirstTry(t *testing.T) {
	calls := 0
	err := retryLLMCall(func() error {
		calls++
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, calls)
}

// TestRetryLLMCall_SuccessSecondTry incurs a ~1 s sleep (the first retry backoff).
func TestRetryLLMCall_SuccessSecondTry(t *testing.T) {
	calls := 0
	err := retryLLMCall(func() error {
		calls++
		if calls == 1 {
			return &httpStatusError{code: http.StatusServiceUnavailable}
		}
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 2, calls)
}

func TestRetryLLMCall_FailImmediatelyOn400(t *testing.T) {
	calls := 0
	want := &httpStatusError{code: http.StatusBadRequest, body: "bad input"}
	err := retryLLMCall(func() error {
		calls++
		return want
	})

	// Non-retryable → only one attempt, error returned as-is.
	assert.Equal(t, 1, calls)
	require.Error(t, err)
	var httpErr *httpStatusError
	require.True(t, errors.As(err, &httpErr))
	assert.Equal(t, http.StatusBadRequest, httpErr.code)
}

// TestRetryLLMCall_ExhaustRetriesOn503 incurs ~7 s of sleep (1+2+4 s backoffs).
// Run with -short to skip.
func TestRetryLLMCall_ExhaustRetriesOn503(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow retry-exhaustion test in short mode")
	}
	calls := 0
	err := retryLLMCall(func() error {
		calls++
		return &httpStatusError{code: http.StatusServiceUnavailable}
	})

	// 4 total attempts: attempt 0,1,2,3 (loop condition: attempt <= len(delays) = 3)
	assert.Equal(t, 4, calls)
	require.Error(t, err)
	var httpErr *httpStatusError
	require.True(t, errors.As(err, &httpErr))
	assert.Equal(t, http.StatusServiceUnavailable, httpErr.code)
}

// ── sanitizeJSONContent ───────────────────────────────────────────────────────

func TestSanitizeJSONContent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strips json code block wrapper",
			input: "```json\n{\"key\":\"value\"}\n```",
			want:  `{"key":"value"}`,
		},
		{
			name:  "strips plain code block wrapper",
			input: "```\n{\"key\":\"value\"}\n```",
			want:  `{"key":"value"}`,
		},
		{
			name:  "leaves plain JSON unchanged",
			input: `{"key":"value"}`,
			want:  `{"key":"value"}`,
		},
		{
			name:  "trims surrounding whitespace",
			input: "  {\"key\":\"value\"}  ",
			want:  `{"key":"value"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sanitizeJSONContent(tt.input))
		})
	}
}

// ── openAIClient HTTP mock tests ──────────────────────────────────────────────

func makeTestOpenAIClient(baseURL string) *openAIClient {
	return &openAIClient{
		baseURL: baseURL,
		apiKey:  "test-api-key",
		model:   "gpt-test",
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

// buildOpenAIResponse encodes an openAIResponse whose sole choice contains content.
func buildOpenAIResponseBody(t *testing.T, content string) []byte {
	t.Helper()
	resp := openAIResponse{
		Choices: []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}{{
			Message: struct {
				Content string `json:"content"`
			}{Content: content},
		}},
	}
	b, err := json.Marshal(resp)
	require.NoError(t, err)
	return b
}

func TestCompleteText_ValidResponse(t *testing.T) {
	want := `{"episodes":[],"characters":[],"assets":[]}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildOpenAIResponseBody(t, want))
	}))
	defer ts.Close()

	c := makeTestOpenAIClient(ts.URL)
	got, err := c.completeText(context.Background(), "system prompt", "user prompt")

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestCompleteText_HTTP400Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"invalid request"}}`))
	}))
	defer ts.Close()

	c := makeTestOpenAIClient(ts.URL)
	result, err := c.completeText(context.Background(), "system", "user")

	assert.Error(t, err)
	assert.Empty(t, result)
	var httpErr *httpStatusError
	require.True(t, errors.As(err, &httpErr))
	assert.Equal(t, http.StatusBadRequest, httpErr.code)
}

func TestAnalyzeChunk_ParsesFullResult(t *testing.T) {
	analysis := AnalysisResult{
		Episodes: []EpisodeResult{{
			ID:    "ep1",
			Title: "Episode 1",
			Scenes: []SceneResult{{
				ID:          "scene_001",
				Description: "A dark room",
				Setting:     "night office",
				Emotion:     "suspense",
				Characters:  []string{"Alice"},
				PromptDraft: "dark office room, night, suspense, cinematic lighting",
			}},
		}},
		Characters: []CharacterResult{{
			Name:     "Alice",
			RoleDesc: "protagonist",
		}},
		Assets: []AssetResult{{
			Type:        "place",
			Name:        "office",
			Description: "dark, night-time office",
		}},
	}

	contentBytes, err := json.Marshal(analysis)
	require.NoError(t, err)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildOpenAIResponseBody(t, string(contentBytes)))
	}))
	defer ts.Close()

	c := makeTestOpenAIClient(ts.URL)
	result, err := c.analyzeChunk(context.Background(), "test script text")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Episodes, 1)
	assert.Equal(t, "ep1", result.Episodes[0].ID)
	require.Len(t, result.Episodes[0].Scenes, 1)
	assert.Equal(t, "scene_001", result.Episodes[0].Scenes[0].ID)
	assert.Equal(t, []string{"Alice"}, result.Episodes[0].Scenes[0].Characters)
	require.Len(t, result.Characters, 1)
	assert.Equal(t, "Alice", result.Characters[0].Name)
	require.Len(t, result.Assets, 1)
	assert.Equal(t, "place", result.Assets[0].Type)
}

func TestAnalyzeChunk_ErrorPropagation(t *testing.T) {
	// 400 is non-retryable → immediately returns error without retrying.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"bad request"}}`))
	}))
	defer ts.Close()

	c := makeTestOpenAIClient(ts.URL)
	result, err := c.analyzeChunk(context.Background(), "script text")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestAnalyzeChunk_InvalidJSONFromLLM(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// LLM returns syntactically broken JSON.
		w.Write(buildOpenAIResponseBody(t, "not-json-at-all"))
	}))
	defer ts.Close()

	c := makeTestOpenAIClient(ts.URL)
	result, err := c.analyzeChunk(context.Background(), "script")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestUpdateConfig_UpdatesFieldsUnderLock(t *testing.T) {
	c := makeTestOpenAIClient("http://old-url.example.com")
	c.UpdateConfig("new-api-key", "http://new-url.example.com")

	c.mu.RLock()
	defer c.mu.RUnlock()
	assert.Equal(t, "new-api-key", c.apiKey)
	assert.Equal(t, "http://new-url.example.com", c.baseURL)
}

func TestUpdateConfig_IgnoresEmptyValues(t *testing.T) {
	c := makeTestOpenAIClient("http://original.example.com")
	c.apiKey = "original-key"

	c.UpdateConfig("", "")

	c.mu.RLock()
	defer c.mu.RUnlock()
	assert.Equal(t, "original-key", c.apiKey)
	assert.Equal(t, "http://original.example.com", c.baseURL)
}

// ── Multi-channel pool round-robin tests ──────────────────────────────────────

// makeMultiChannelClient builds an openAIClient with N fake channel servers.
// Each server records which key it received so the test can verify rotation.
func makeMultiChannelClient(t *testing.T, n int) (*openAIClient, [][]string) {
	t.Helper()
	received := make([][]string, n)
	servers := make([]*httptest.Server, n)
	bases := make([]string, n)
	keys := make([]string, n)

	for i := 0; i < n; i++ {
		idx := i // capture for closure
		received[idx] = []string{}
		servers[idx] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			received[idx] = append(received[idx], r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			w.Write(buildOpenAIResponseBody(t, `{"episodes":[]}`))
		}))
		t.Cleanup(servers[idx].Close)
		bases[idx] = servers[idx].URL
		keys[idx] = "sk-chan" + string(rune('A'+idx))
	}

	channels := make([]llmChannel, n)
	for i := 0; i < n; i++ {
		channels[i] = llmChannel{baseURL: bases[i], apiKey: keys[i], model: "gpt-5.4"}
	}
	c := &openAIClient{
		baseURL:  bases[0],
		apiKey:   keys[0],
		model:    "gpt-5.4",
		client:   &http.Client{Timeout: 5 * time.Second},
		channels: channels,
	}
	return c, received
}

func TestMultiChannel_RoundRobinDistribution(t *testing.T) {
	t.Parallel()
	c, received := makeMultiChannelClient(t, 3)

	calls := 6 // two full rounds
	for i := 0; i < calls; i++ {
		_, _ = c.completeTextWithModel(context.Background(), "sys", "user", "")
	}

	// Each of the 3 channels should have received exactly 2 calls
	for i, r := range received {
		assert.Equal(t, 2, len(r), "channel %d expected 2 calls, got %d", i, len(r))
	}
}

func TestMultiChannel_UsesChannelModel(t *testing.T) {
	t.Parallel()

	var capturedBodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capturedBodies = append(capturedBodies, string(body))
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildOpenAIResponseBody(t, `{"episodes":[]}`))
	}))
	defer srv.Close()

	c := &openAIClient{
		baseURL: srv.URL,
		apiKey:  "sk-fallback",
		model:   "gpt-fallback",
		client:  &http.Client{Timeout: 5 * time.Second},
		channels: []llmChannel{
			{baseURL: srv.URL, apiKey: "sk-poolkey", model: "gpt-5.4"},
		},
	}

	_, _ = c.completeTextWithModel(context.Background(), "sys", "user", "")

	require.Len(t, capturedBodies, 1)
	var req openAIRequest
	require.NoError(t, json.Unmarshal([]byte(capturedBodies[0]), &req))
	// Channel model ("gpt-5.4") must be used, not the fallback model
	assert.Equal(t, "gpt-5.4", req.Model, "expected channel model gpt-5.4 in request body")
}

func TestMultiChannel_FallsBackToSingleChannel(t *testing.T) {
	t.Parallel()

	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildOpenAIResponseBody(t, `{"episodes":[]}`))
	}))
	defer srv.Close()

	// No channels configured — should fall back to single baseURL/apiKey
	c := makeTestOpenAIClient(srv.URL)
	_, _ = c.completeTextWithModel(context.Background(), "sys", "user", "")
	assert.Equal(t, 1, calls)
}
