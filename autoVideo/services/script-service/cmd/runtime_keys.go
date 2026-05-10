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

	"github.com/autovideo/script-service/pkg/config"
)

type runtimeAPIKey struct {
	Provider   string `json:"provider"`
	KeyAlias   string `json:"key_alias"`
	PlainKey   string `json:"plain_key"`
	BaseURL    string `json:"base_url"`
	ModelScope string `json:"model_scope"`
}

func applyRuntimeConfig(cfg *config.Config) error {
	keys, err := fetchRuntimeAPIKeys(cfg.AuthService.BaseURL, cfg.JWT.Secret)
	if err != nil {
		return err
	}
	byProvider := groupRuntimeAPIKeys(keys)
	if primary := firstRuntimeKey(byProvider["runtime.script.openai.primary"]); primary != nil {
		cfg.LLM.OpenAI.APIKey = primary.PlainKey
		if primary.BaseURL != "" {
			cfg.LLM.OpenAI.BaseURL = primary.BaseURL
		}
		if model := firstScope(primary.ModelScope); model != "" {
			cfg.LLM.OpenAI.Model = model
		}
	}
	pool := byProvider["runtime.script.openai.pool"]
	if len(pool) > 0 {
		cfg.LLM.OpenAI.ChannelBases = make([]string, 0, len(pool))
		cfg.LLM.OpenAI.ChannelKeys = make([]string, 0, len(pool))
		for _, key := range pool {
			cfg.LLM.OpenAI.ChannelBases = append(cfg.LLM.OpenAI.ChannelBases, key.BaseURL)
			cfg.LLM.OpenAI.ChannelKeys = append(cfg.LLM.OpenAI.ChannelKeys, key.PlainKey)
			if cfg.LLM.OpenAI.ChannelModel == "" {
				cfg.LLM.OpenAI.ChannelModel = firstScope(key.ModelScope)
			}
		}
	}
	return nil
}

func fetchRuntimeAPIKeys(baseURL, secret string) ([]runtimeAPIKey, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, fmt.Errorf("auth service base url is empty")
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

func firstScope(raw string) string {
	parts := splitScope(raw)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
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