package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/autovideo/project-service/pkg/config"
)

type runtimeAPIKey struct {
	Provider   string `json:"provider"`
	KeyAlias   string `json:"key_alias"`
	PlainKey   string `json:"plain_key"`
	BaseURL    string `json:"base_url"`
	ModelScope string `json:"model_scope"`
}

func applyRuntimeConfig(cfg *config.Config) error {
	keys, err := fetchRuntimeAPIKeys(cfg.AuthService.BaseURL, cfg.JWT.AccessSecret)
	if err != nil {
		return err
	}
	byProvider := groupRuntimeAPIKeys(keys)
	if key := firstRuntimeKey(byProvider["runtime.project.llm.primary"]); key != nil {
		cfg.LLM.APIKey = key.PlainKey
		if key.BaseURL != "" {
			cfg.LLM.BaseURL = key.BaseURL
		}
		if model := firstScope(key.ModelScope); model != "" {
			cfg.LLM.Model = model
		}
	}
	if key := firstRuntimeKey(byProvider["runtime.project.llm.fallback"]); key != nil {
		cfg.LLM.FallbackAPIKey = key.PlainKey
		if key.BaseURL != "" {
			cfg.LLM.FallbackBaseURL = key.BaseURL
		}
		if model := firstScope(key.ModelScope); model != "" {
			cfg.LLM.FallbackModel = model
		}
	}
	return nil
}

func fetchRuntimeAPIKeys(baseURL, secret string) ([]runtimeAPIKey, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	var lastErr error
	for attempt := 0; attempt < 10; attempt++ {
		keys, err := doFetchRuntimeAPIKeys(client, baseURL, secret)
		if err == nil {
			return keys, nil
		}
		lastErr = err
		time.Sleep(time.Second)
	}
	return nil, fmt.Errorf("fetch runtime api keys: %w", lastErr)
}

func doFetchRuntimeAPIKeys(client *http.Client, baseURL, secret string) ([]runtimeAPIKey, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/internal/runtime-api-keys", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+buildServiceToken(secret))
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth service returned %d: %s", resp.StatusCode, string(body))
	}
	var payload struct {
		Code int             `json:"code"`
		Data []runtimeAPIKey `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	return payload.Data, nil
}

func buildServiceToken(secret string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]interface{}{
		"user_id":    "1",
		"role":       "service",
		"token_type": "access",
		"iat":        time.Now().Unix(),
		"exp":        time.Now().Add(5 * time.Minute).Unix(),
	})
	payload := base64.RawURLEncoding.EncodeToString(claims)
	unsigned := header + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func groupRuntimeAPIKeys(keys []runtimeAPIKey) map[string][]runtimeAPIKey {
	grouped := make(map[string][]runtimeAPIKey)
	for _, key := range keys {
		grouped[key.Provider] = append(grouped[key.Provider], key)
	}
	return grouped
}

func firstRuntimeKey(keys []runtimeAPIKey) *runtimeAPIKey {
	if len(keys) == 0 {
		return nil
	}
	return &keys[0]
}

func firstScope(raw string) string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
	})
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			return trimmed
		}
	}
	return ""
}