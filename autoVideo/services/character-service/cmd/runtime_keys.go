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

	"github.com/autovideo/character-service/pkg/config"
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
	if key := firstRuntimeKey(byProvider["runtime.character.llm"]); key != nil {
		cfg.LLM.APIKey = key.PlainKey
		if key.BaseURL != "" {
			cfg.LLM.BaseURL = key.BaseURL
		}
		scopes := splitScope(key.ModelScope)
		if len(scopes) > 0 {
			cfg.LLM.Model = scopes[0]
		}
		if len(scopes) > 1 {
			cfg.LLM.VisionModel = scopes[1]
		}
	}
	applySingleProvider(byProvider["runtime.character.claude"], &cfg.Claude.BaseURL, &cfg.Claude.APIKey)
	applySingleProvider(byProvider["runtime.character.qwen"], &cfg.Qwen.BaseURL, &cfg.Qwen.APIKey)
	applySingleProvider(byProvider["runtime.character.zhipu"], &cfg.Zhipu.BaseURL, &cfg.Zhipu.APIKey)
	gemini := byProvider["runtime.character.gemini"]
	if len(gemini) > 0 {
		bases := make([]string, 0, len(gemini))
		keysOut := make([]string, 0, len(gemini))
		for _, key := range gemini {
			bases = append(bases, key.BaseURL)
			keysOut = append(keysOut, key.PlainKey)
			if cfg.Gemini.Model == "" {
				cfg.Gemini.Model = firstScope(key.ModelScope)
			}
		}
		cfg.Gemini.Bases = strings.Join(bases, ",")
		cfg.Gemini.Keys = strings.Join(keysOut, ",")
	}
	return nil
}

func applySingleProvider(keys []runtimeAPIKey, baseURL, apiKey *string) {
	if len(keys) == 0 {
		return
	}
	if keys[0].BaseURL != "" {
		*baseURL = keys[0].BaseURL
	}
	*apiKey = keys[0].PlainKey
}

func fetchRuntimeAPIKeys(baseURL, secret string) ([]runtimeAPIKey, error) {
	if secret == "" {
		secret = "autovideo-access-secret-change-in-prod"
	}
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

func splitScope(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func firstScope(raw string) string {
	parts := splitScope(raw)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}