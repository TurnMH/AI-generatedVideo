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

	"github.com/autovideo/image-service/pkg/config"
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
	applyPool(byProvider["runtime.image.openai"], &cfg.Models.OpenAIBase, &cfg.Models.OpenAIKeys, &cfg.Models.OpenAIModels)
	applyPool(byProvider["runtime.image.tongyi"], &cfg.Models.DashScopeBase, &cfg.Models.TongyiKeys, &cfg.Models.TongyiModels)
	applyPool(byProvider["runtime.image.zhipu"], &cfg.Models.ZhipuBase, &cfg.Models.ZhipuKeys, &cfg.Models.ZhipuModels)
	applyPool(byProvider["runtime.image.gemini"], nil, &cfg.Models.GeminiKeys, &cfg.Models.GeminiModels)
	if gemini := byProvider["runtime.image.gemini"]; len(gemini) > 0 {
		bases := make([]string, 0, len(gemini))
		for _, key := range gemini {
			bases = append(bases, key.BaseURL)
			if cfg.Models.GeminiFlashModel == "" {
				cfg.Models.GeminiFlashModel = firstScope(key.ModelScope)
			}
		}
		cfg.Models.GeminiBases = bases
	}
	applySingle(byProvider["runtime.image.replicate"], nil, &cfg.Models.ReplicateKey, nil)
	applySingle(byProvider["runtime.image.baidu"], &cfg.Models.BaiduImageBase, &cfg.Models.BaiduImageKey, func(scopes []string) {
		if len(scopes) > 0 { cfg.Models.BaiduImageModel = scopes[0] }
	})
	applySingle(byProvider["runtime.image.qianfan"], &cfg.Models.QianfanImageBase, &cfg.Models.QianfanImageKey, func(scopes []string) {
		cfg.Models.QianfanImageModels = scopes
	})
	applySingle(byProvider["runtime.image.doubao"], &cfg.Models.DoubaoImageBase, &cfg.Models.DoubaoImageKey, func(scopes []string) {
		if len(scopes) > 0 { cfg.Models.DoubaoImageModel = scopes[0] }
		if len(scopes) > 1 { cfg.Models.DoubaoImageModelV3 = scopes[1] }
	})
	return nil
}

func applyPool(keys []runtimeAPIKey, baseURL *string, keyList *[]string, models *[]string) {
	if len(keys) == 0 {
		return
	}
	values := make([]string, 0, len(keys))
	scopeSeen := map[string]struct{}{}
	scopes := make([]string, 0)
	for _, key := range keys {
		values = append(values, key.PlainKey)
		if baseURL != nil && *baseURL == "" && key.BaseURL != "" {
			*baseURL = key.BaseURL
		}
		for _, scope := range splitScope(key.ModelScope) {
			if _, ok := scopeSeen[scope]; ok {
				continue
			}
			scopeSeen[scope] = struct{}{}
			scopes = append(scopes, scope)
		}
	}
	*keyList = values
	if models != nil && len(scopes) > 0 {
		*models = scopes
	}
}

func applySingle(keys []runtimeAPIKey, baseURL, value *string, scopeFn func([]string)) {
	if len(keys) == 0 {
		return
	}
	if baseURL != nil && keys[0].BaseURL != "" {
		*baseURL = keys[0].BaseURL
	}
	if value != nil {
		*value = keys[0].PlainKey
	}
	if scopeFn != nil {
		scopeFn(splitScope(keys[0].ModelScope))
	}
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
		"user_id":    1,
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