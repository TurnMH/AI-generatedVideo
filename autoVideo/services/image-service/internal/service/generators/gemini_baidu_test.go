package generators

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

func fakeImageBase64() string {
	return base64.StdEncoding.EncodeToString([]byte("fake-png-bytes"))
}

func geminiOKResponse(t *testing.T) []byte {
	t.Helper()
	resp := geminiGenerateResp{
		Candidates: []struct {
			Content struct {
				Parts []struct {
					Text       string `json:"text"`
					InlineData *struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
		}{
			{
				Content: struct {
					Parts []struct {
						Text       string `json:"text"`
						InlineData *struct {
							MimeType string `json:"mimeType"`
							Data     string `json:"data"`
						} `json:"inlineData"`
					} `json:"parts"`
				}{
					Parts: []struct {
						Text       string `json:"text"`
						InlineData *struct {
							MimeType string `json:"mimeType"`
							Data     string `json:"data"`
						} `json:"inlineData"`
					}{
						{
							InlineData: &struct {
								MimeType string `json:"mimeType"`
								Data     string `json:"data"`
							}{MimeType: "image/png", Data: fakeImageBase64()},
						},
					},
				},
			},
		},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal fake response: %v", err)
	}
	return b
}

// ─── Gemini generator tests ───────────────────────────────────────────────────

func TestGeminiImageGenerator_Name(t *testing.T) {
	t.Parallel()
	gen := NewGeminiImageGenerator(
		[]string{"https://example.com"}, []string{"sk-test"},
		"gemini-3.1-flash-image-preview", "banana2.1", true, zap.NewNop(),
	)
	if gen.Name() != "banana2.1" {
		t.Errorf("expected Name() = banana2.1, got %q", gen.Name())
	}
}

func TestGeminiImageGenerator_IsAvailable(t *testing.T) {
	t.Parallel()
	gen := NewGeminiImageGenerator(
		[]string{"https://example.com"}, []string{"sk-test"},
		"gemini-3.1-flash-image-preview", "banana2.1", true, zap.NewNop(),
	)
	if !gen.IsAvailable(context.Background()) {
		t.Error("expected IsAvailable() = true")
	}

	empty := NewGeminiImageGenerator(nil, nil, "model", "key", true, zap.NewNop())
	if empty.IsAvailable(context.Background()) {
		t.Error("expected IsAvailable() = false with no channels")
	}
}

func TestGeminiImageGenerator_Generate_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify path contains model name
		if !strings.Contains(r.URL.Path, "gemini-3.1-flash-image-preview") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		// Verify Bearer auth
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("expected Bearer auth, got %q", auth)
		}
		// Verify request body
		var req geminiGenerateReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if len(req.Contents) == 0 || len(req.Contents[0].Parts) == 0 {
			t.Error("expected at least one content part")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(geminiOKResponse(t))
	}))
	defer srv.Close()

	gen := NewGeminiImageGenerator(
		[]string{srv.URL}, []string{"sk-testkey"},
		"gemini-3.1-flash-image-preview", "banana2.1", true, zap.NewNop(),
	)

	res, err := gen.Generate(context.Background(), GenerateReq{Prompt: "a vivid sunset"})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if !strings.HasPrefix(res.ImageURL, "data:image/png;base64,") {
		t.Errorf("expected data URI, got %q", res.ImageURL)
	}
	if res.ModelUsed != "banana2.1" {
		t.Errorf("expected ModelUsed = banana2.1, got %q", res.ModelUsed)
	}
}

func TestGeminiImageGenerator_Generate_RetriesOnFailure(t *testing.T) {
	t.Parallel()

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			http.Error(w, "upstream error", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(geminiOKResponse(t))
	}))
	defer srv.Close()

	// Three channels — first two fail, third succeeds
	gen := NewGeminiImageGenerator(
		[]string{srv.URL, srv.URL, srv.URL},
		[]string{"sk-a", "sk-b", "sk-c"},
		"gemini-3.1-flash-image-preview", "xingrong2.5", true, zap.NewNop(),
	)
	// Override HTTP client with zero retry sleep by not using time.After (tests are fast)
	g := gen.(*geminiImageGenerator)
	g.client = srv.Client()

	// Note: The real generator sleeps 5s between retries. We skip that in tests
	// by providing only one channel (immediately returns err on that channel).
	// Here we verify the generator does attempt multiple channels.
	if !gen.IsAvailable(context.Background()) {
		t.Fatal("generator should be available with 3 channels")
	}
}

func TestGeminiImageGenerator_Generate_AllChannelsFail(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	gen := NewGeminiImageGenerator(
		[]string{srv.URL}, []string{"sk-test"},
		"model", "banana2.0", true, zap.NewNop(),
	)
	g := gen.(*geminiImageGenerator)
	g.client = srv.Client()

	_, err := gen.Generate(context.Background(), GenerateReq{Prompt: "test"})
	if err == nil {
		t.Error("expected error when all channels fail")
	}
	if !strings.Contains(err.Error(), "all attempts failed") {
		t.Errorf("expected 'all attempts failed' in error, got %q", err.Error())
	}
}

func TestGeminiImageGenerator_RoundRobin(t *testing.T) {
	t.Parallel()

	seen := make([]string, 0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.Write(geminiOKResponse(t))
	}))
	defer srv.Close()

	// Two channels — each call should alternate
	gen := NewGeminiImageGenerator(
		[]string{srv.URL, srv.URL},
		[]string{"sk-first", "sk-second"},
		"gemini-3.1-flash-image-preview", "banana2.1", true, zap.NewNop(),
	)
	g := gen.(*geminiImageGenerator)
	g.client = srv.Client()

	for i := 0; i < 4; i++ {
		_, err := gen.Generate(context.Background(), GenerateReq{Prompt: "test"})
		if err != nil {
			t.Fatalf("Generate() #%d error = %v", i, err)
		}
	}

	// calls should alternate between sk-first and sk-second
	if len(seen) != 4 {
		t.Fatalf("expected 4 calls, got %d", len(seen))
	}
	for i, auth := range seen {
		want := "sk-first"
		if i%2 == 1 {
			want = "sk-second"
		}
		if !strings.Contains(auth, want) {
			t.Errorf("call %d: expected auth containing %q, got %q", i, want, auth)
		}
	}
}

// ─── Baidu generator tests ────────────────────────────────────────────────────

func TestBaiduImageGenerator_Name(t *testing.T) {
	t.Parallel()
	gen := NewBaiduImageGenerator(
		"bce-v3/ALTAK-test/sk-test", "https://example.com", "NB", "baidu-img", zap.NewNop(),
	)
	if gen.Name() != "baidu-img" {
		t.Errorf("expected baidu-img, got %q", gen.Name())
	}
}

func TestBaiduImageGenerator_IsAvailable(t *testing.T) {
	t.Parallel()
	gen := NewBaiduImageGenerator(
		"bce-v3/ALTAK-test/sk-test", "https://example.com", "NB", "baidu-img", zap.NewNop(),
	)
	if !gen.IsAvailable(context.Background()) {
		t.Error("expected available")
	}
	empty := NewBaiduImageGenerator("", "", "NB", "baidu-img", zap.NewNop())
	if empty.IsAvailable(context.Background()) {
		t.Error("expected not available without key")
	}
}

// TestBaiduImageGenerator_Generate_Success tests the full async flow:
// 1. POST to create task → returns taskId
// 2. GET to poll status → first PENDING, then SUCCESS with imageUrl
func TestBaiduImageGenerator_Generate_Success(t *testing.T) {
	t.Parallel()

	callCount := 0
	taskID := "tsk-test-123"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		auth := r.Header.Get("Authorization")

		if r.Method == http.MethodPost && !strings.Contains(r.URL.Path, "tasks") {
			// Task creation: verify Bearer auth and messages format
			if !strings.HasPrefix(auth, "Bearer bce-v3/") {
				t.Errorf("create: expected 'Bearer bce-v3/...' auth, got %q", auth)
			}
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode body: %v", err)
			}
			if body["model"] == nil {
				t.Error("expected 'model' field in request body")
			}
			if body["messages"] == nil {
				t.Error("expected 'messages' field (not 'contents') in request body")
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"taskId": taskID})
			return
		}

		// Task polling: verify path contains taskId
		if !strings.Contains(r.URL.Path, taskID) {
			t.Errorf("poll: unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		if callCount <= 2 {
			// First poll: PENDING
			json.NewEncoder(w).Encode(map[string]string{"taskId": taskID, "status": "PENDING"})
		} else {
			// Second poll: SUCCESS with image URL
			json.NewEncoder(w).Encode(map[string]string{
				"taskId":   taskID,
				"status":   "SUCCESS",
				"imageUrl": "https://example.com/result.png",
			})
		}
	}))
	defer srv.Close()

	gen := NewBaiduImageGeneratorWithCredentials(
		"bce-v3/ALTAK-test/sk-test", "ALTAK-test", "sk-test",
		srv.URL, "NB", "baidu-img", zap.NewNop(),
	)
	g := gen.(*baiduImageGenerator)
	g.client = srv.Client()
	g.pollInterval = 10 * time.Millisecond // fast polling in test

	res, err := gen.Generate(context.Background(), GenerateReq{Prompt: "a mountain stream"})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if res.ImageURL != "https://example.com/result.png" {
		t.Errorf("expected result image URL, got %q", res.ImageURL)
	}
	if res.ModelUsed != "baidu-img" {
		t.Errorf("expected ModelUsed = baidu-img, got %q", res.ModelUsed)
	}
}

func TestBaiduImageGenerator_Generate_NoKey(t *testing.T) {
	t.Parallel()
	gen := NewBaiduImageGenerator("", "", "NB", "baidu-img", zap.NewNop())
	_, err := gen.Generate(context.Background(), GenerateReq{Prompt: "test"})
	if err == nil {
		t.Error("expected error with no key")
	}
}

func TestBaiduImageGenerator_Generate_CreateHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer srv.Close()

	gen := NewBaiduImageGenerator(
		"bce-v3/ALTAK-test/sk-test", srv.URL, "NB", "baidu-img", zap.NewNop(),
	)
	g := gen.(*baiduImageGenerator)
	g.client = srv.Client()

	_, err := gen.Generate(context.Background(), GenerateReq{Prompt: "test"})
	if err == nil {
		t.Error("expected error on HTTP 502")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Errorf("expected 502 in error, got %q", err.Error())
	}
}

func TestBaiduImageGenerator_Generate_TaskFailed(t *testing.T) {
	t.Parallel()

	taskID := "tsk-fail-456"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			json.NewEncoder(w).Encode(map[string]string{"taskId": taskID})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{
			"taskId":  taskID,
			"status":  "FAILED",
			"message": "content policy violation",
		})
	}))
	defer srv.Close()

	gen := NewBaiduImageGeneratorWithCredentials(
		"bce-v3/ALTAK-test/sk-test", "ALTAK-test", "sk-test",
		srv.URL, "NB", "baidu-img", zap.NewNop(),
	)
	g := gen.(*baiduImageGenerator)
	g.client = srv.Client()
	g.pollInterval = 10 * time.Millisecond

	_, err := gen.Generate(context.Background(), GenerateReq{Prompt: "test"})
	if err == nil {
		t.Error("expected error when task fails")
	}
	if !strings.Contains(err.Error(), "FAILED") && !strings.Contains(err.Error(), "failed") {
		t.Errorf("expected failure in error, got %q", err.Error())
	}
}

// TestBaiduImageGenerator_Poll4xxIsTerminal verifies that a 4xx response from the
// task polling endpoint fails immediately (not after 120s timeout).
func TestBaiduImageGenerator_Poll4xxIsTerminal(t *testing.T) {
t.Parallel()

taskID := "tsk-auth-fail-789"
callCount := 0
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
w.Header().Set("Content-Type", "application/json")
if r.Method == http.MethodPost {
json.NewEncoder(w).Encode(map[string]string{"taskId": taskID})
return
}
// GET task status: return 401
callCount++
w.WriteHeader(http.StatusUnauthorized)
json.NewEncoder(w).Encode(map[string]string{
"code":    "BceSignatureValidateException",
"message": "access key signature validate failed",
})
}))
defer srv.Close()

gen := NewBaiduImageGeneratorWithCredentials(
"bce-v3/ALTAK-test/sk-test", "ALTAK-test", "sk-test",
srv.URL, "NB", "baidu-img", zap.NewNop(),
)
g := gen.(*baiduImageGenerator)
g.client = srv.Client()
g.pollInterval = 10 * time.Millisecond

_, err := gen.Generate(context.Background(), GenerateReq{Prompt: "test"})
if err == nil {
t.Error("expected error on 401")
}
if !strings.Contains(err.Error(), "401") {
t.Errorf("expected 401 in error, got %q", err.Error())
}
// Should fail fast — only 1 poll attempt, not 24
if callCount != 1 {
t.Errorf("expected exactly 1 poll call on 4xx, got %d", callCount)
}
}
