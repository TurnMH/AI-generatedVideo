package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// PromptTemplate mirrors script-service's PromptTemplate for cascade embedding.
type PromptTemplate struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	StyleKey        string    `json:"style_key"`
	Description     string    `json:"description"`
	ResourceType    string    `json:"resource_type"`
	ModelBinding    string    `json:"model_binding"`
	Version         string    `json:"version"`
	IsActive        bool      `json:"is_active"`
	SortOrder       int       `json:"sort_order"`
	TemplateFileURL string    `json:"template_file_url,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// templateListResp mirrors script-service's list response envelope.
type templateListResp struct {
	Data []PromptTemplate `json:"data"`
}

// scriptServiceURL returns the script-service base URL from env or default.
func scriptServiceURL() string {
	if u := os.Getenv("SCRIPT_SERVICE_URL"); u != "" {
		return u
	}
	return "http://localhost:8003"
}

// FetchTemplates fetches all active prompt templates from script-service and
// returns a map keyed by model_binding (empty string = universal templates).
func FetchTemplates(ctx context.Context) (map[string][]PromptTemplate, error) {
	url := fmt.Sprintf("%s/api/v1/prompt-templates?active_only=true", scriptServiceURL())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("script-service returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var listResp templateListResp
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, err
	}

	byBinding := make(map[string][]PromptTemplate)
	for _, t := range listResp.Data {
		byBinding[t.ModelBinding] = append(byBinding[t.ModelBinding], t)
	}
	return byBinding, nil
}
