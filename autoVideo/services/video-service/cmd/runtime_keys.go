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

	"github.com/autovideo/video-service/pkg/config"
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
	if kling := byProvider["runtime.video.kling"]; len(kling) > 0 {
		cfg.Models.KlingKey = kling[0].PlainKey
		cfg.Models.KlingKeys = make([]string, 0, len(kling)-1)
		for i, key := range kling {
			if i > 0 {
				cfg.Models.KlingKeys = append(cfg.Models.KlingKeys, key.PlainKey)
			}
			if key.BaseURL != "" {
				cfg.Models.KlingBase = key.BaseURL
			}
		}
		scopes := splitScope(kling[0].ModelScope)
		if len(scopes) > 0 { cfg.Models.KlingModel = scopes[0] }
		if len(scopes) > 1 { cfg.Models.KlingOmniModel = scopes[1] }
	}
	applySingle(byProvider["runtime.video.aiping"], &cfg.Models.AipingBase, &cfg.Models.AipingKey, func(scopes []string) {
		if len(scopes) > 0 { cfg.Models.KlingModel = scopes[0] }
	})
	applyPair(byProvider["runtime.video.vclm"], &cfg.Models.VclmSecretID, &cfg.Models.VclmSecretKey, &cfg.Models.VclmRegion)
	applyPair(byProvider["runtime.video.wan"], &cfg.Models.WanKey, &cfg.Models.WanSecret, &cfg.Models.WanBase)
	applySingle(byProvider["runtime.video.runninghub"], &cfg.Models.RunningHubBase, &cfg.Models.RunningHubKey, func(scopes []string) {
		if len(scopes) > 0 { cfg.Models.RunningHubWorkflow = scopes[0] }
		if len(scopes) > 1 { cfg.Models.RunningHubNodeID = scopes[1] }
	})
	applySingle(byProvider["runtime.video.replicate"], nil, &cfg.Models.ReplicateKey, nil)
	applySingle(byProvider["runtime.video.sora2"], &cfg.Models.Sora2Base, &cfg.Models.Sora2Key, nil)
	applySingle(byProvider["runtime.video.hubagi"], &cfg.Models.HubagiBase, &cfg.Models.HubagiKey, func(scopes []string) {
		if len(scopes) > 0 { cfg.Models.HubagiModel = scopes[0] }
	})
	applySingle(byProvider["runtime.video.veo"], &cfg.Models.VeoBase, &cfg.Models.VeoKey, func(scopes []string) {
		if len(scopes) > 0 { cfg.Models.VeoModel = scopes[0] }
	})
	applySingle(byProvider["runtime.video.doubao"], &cfg.Models.DoubaoBase, &cfg.Models.DoubaoKey, func(scopes []string) {
		if len(scopes) > 0 { cfg.Models.DoubaoModel = scopes[0] }
	})
	applySingle(byProvider["runtime.video.doubao.seedance"], &cfg.Models.DoubaoSeedanceBase, &cfg.Models.DoubaoSeedanceKey, func(scopes []string) {
		if len(scopes) > 0 { cfg.Models.DoubaoSeedanceModel = scopes[0] }
	})
	applySingle(byProvider["runtime.video.vidu"], &cfg.Models.ViduBase, &cfg.Models.ViduKey, func(scopes []string) {
		if len(scopes) > 0 { cfg.Models.ViduModel = scopes[0] }
		if len(scopes) > 1 { cfg.Models.ViduMixModel = scopes[1] }
	})
	applySingle(byProvider["runtime.video.vidu.offpeak"], &cfg.Models.ViduBase, &cfg.Models.ViduOffpeakKey, nil)
	applySingle(byProvider["runtime.video.suanneng"], &cfg.Models.SuannengBase, &cfg.Models.SuannengKey, func(scopes []string) {
		if len(scopes) > 0 { cfg.Models.SuannengModel = scopes[0] }
	})
	applySingle(byProvider["runtime.video.gaga"], &cfg.Models.GagaBase, &cfg.Models.GagaKey, nil)
	applyPair(byProvider["runtime.video.baidu.bce"], &cfg.Models.BaiduBCEKey, &cfg.Models.BaiduBCESecret, nil)
	if baidu := byProvider["runtime.video.baidu.bce"]; len(baidu) > 0 {
		scopes := splitScope(baidu[0].ModelScope)
		if len(scopes) > 0 { cfg.Models.BaiduBCEModel = scopes[0] }
	}
	applySingle(byProvider["runtime.video.llm"], &cfg.Models.LLMBase, &cfg.Models.LLMKey, func(scopes []string) {
		if len(scopes) > 0 { cfg.Models.LLMModel = scopes[0] }
	})
	applySingle(byProvider["runtime.video.music"], &cfg.Models.MusicBase, &cfg.Models.MusicKey, func(scopes []string) {
		if len(scopes) > 0 { cfg.Models.MusicModel = scopes[0] }
	})
	return nil
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

func applyPair(keys []runtimeAPIKey, first, second, baseOrRegion *string) {
	if len(keys) == 0 {
		return
	}
	parts := strings.SplitN(keys[0].PlainKey, ":", 2)
	if len(parts) > 0 {
		*first = parts[0]
	}
	if len(parts) > 1 {
		*second = parts[1]
	}
	if baseOrRegion != nil && keys[0].BaseURL != "" {
		*baseOrRegion = keys[0].BaseURL
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