package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/autovideo/script-service/internal/model"
	"github.com/autovideo/script-service/internal/repository"
	"github.com/autovideo/script-service/internal/service"
	"github.com/gin-gonic/gin"
)

// ── stubs ─────────────────────────────────────────────────────────────────────

type stubScriptLibraryRepo struct{}

func (s *stubScriptLibraryRepo) Create(_ context.Context, _ *model.ScriptLibrary) error {
	return nil
}
func (s *stubScriptLibraryRepo) FindByID(_ context.Context, _ int64) (*model.ScriptLibrary, error) {
	return nil, nil
}
func (s *stubScriptLibraryRepo) Update(_ context.Context, _ *model.ScriptLibrary) error {
	return nil
}
func (s *stubScriptLibraryRepo) Delete(_ context.Context, _ int64) error { return nil }
func (s *stubScriptLibraryRepo) List(_ context.Context, _ string, _ int64) ([]model.ScriptLibrary, error) {
	return nil, nil
}

var _ repository.ScriptLibraryRepository = (*stubScriptLibraryRepo)(nil)

type stubLLMClient struct {
	result *service.ScriptGenerateResult
	err    error
}

func (s *stubLLMClient) Analyze(_ context.Context, _ string) (*service.AnalysisResult, error) {
	return nil, nil
}
func (s *stubLLMClient) GenerateScript(_ context.Context, _ *service.ScriptGenerateReq) (*service.ScriptGenerateResult, error) {
	return s.result, s.err
}
func (s *stubLLMClient) ExtractCharacters(_ context.Context, _ string) ([]service.CharacterExtractInfo, error) {
	return nil, nil
}
func (s *stubLLMClient) UpdateConfig(_, _ string) {}

var _ service.LLMClient = (*stubLLMClient)(nil)

// ── helpers ───────────────────────────────────────────────────────────────────

func newTestRouter(llm service.LLMClient) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewScriptLibraryHandler(&stubScriptLibraryRepo{}, llm)
	r := gin.New()
	r.POST("/generate", h.GenerateAI)
	return r
}

func makeGenerateReq(t *testing.T) *bytes.Buffer {
	t.Helper()
	body, err := json.Marshal(map[string]interface{}{
		"mode":  "script",
		"title": "测试剧本",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return bytes.NewBuffer(body)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestGenerateAI_ReturnsOK_WhenContentPresent(t *testing.T) {
	llm := &stubLLMClient{
		result: &service.ScriptGenerateResult{Content: "第一章 英雄起源\n\n从前在山谷里..."},
	}
	r := newTestRouter(llm)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/generate", makeGenerateReq(t))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["code"].(float64) != 200 {
		t.Errorf("expected code 200 in body, got %v", resp["code"])
	}
}

func TestGenerateAI_Returns422_WhenContentEmpty(t *testing.T) {
	llm := &stubLLMClient{
		result: &service.ScriptGenerateResult{Content: ""},
	}
	r := newTestRouter(llm)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/generate", makeGenerateReq(t))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d — body: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	msg, _ := resp["message"].(string)
	if msg == "" {
		t.Errorf("expected non-empty error message in body, got %v", resp)
	}
}

func TestGenerateAI_Returns422_WhenContentWhitespaceOnly(t *testing.T) {
	llm := &stubLLMClient{
		result: &service.ScriptGenerateResult{Content: "   \n\t  "},
	}
	r := newTestRouter(llm)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/generate", makeGenerateReq(t))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for whitespace-only content, got %d — body: %s", w.Code, w.Body.String())
	}
}

func TestGenerateAI_Returns422_WhenResultNil(t *testing.T) {
	llm := &stubLLMClient{result: nil}
	r := newTestRouter(llm)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/generate", makeGenerateReq(t))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for nil result, got %d — body: %s", w.Code, w.Body.String())
	}
}

func TestGenerateAI_Returns503_WhenLLMNil(t *testing.T) {
	r := newTestRouter(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/generate", makeGenerateReq(t))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d — body: %s", w.Code, w.Body.String())
	}
}
