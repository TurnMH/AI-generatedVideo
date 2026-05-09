package service

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/autovideo/auth-service/internal/model"
	"github.com/spf13/viper"
)

func buildRuntimeSystemKeys(configFile string) ([]model.SystemAPIKey, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	if configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		v.SetConfigName("config")
		v.AddConfigPath("../../")
		v.AddConfigPath(".")
	}
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var keys []model.SystemAPIKey
	add := func(provider, alias, plainKey, baseURL string, scopes ...string) {
		plainKey = strings.TrimSpace(plainKey)
		baseURL = strings.TrimSpace(baseURL)
		if plainKey == "" {
			return
		}
		key := model.SystemAPIKey{
			Provider:   provider,
			KeyAlias:   alias,
			PlainKey:   plainKey,
			BaseURL:    baseURL,
			ModelScope: joinScopes(scopes...),
			IsActive:   true,
			Status:     "active",
			CreatedAt:  time.Now(),
		}
		keys = append(keys, key)
	}

	// project-service
	add("runtime.project.llm.primary", "project-service primary llm",
		v.GetString("project-service.llm.api_key"),
		v.GetString("project-service.llm.base_url"),
		v.GetString("project-service.llm.model"),
	)
	add("runtime.project.llm.fallback", "project-service fallback llm",
		v.GetString("project-service.llm.fallback_api_key"),
		v.GetString("project-service.llm.fallback_base_url"),
		v.GetString("project-service.llm.fallback_model"),
	)

	// script-service
	add("runtime.script.openai.primary", "script-service primary openai",
		v.GetString("script-service.llm.openai.api_key"),
		v.GetString("script-service.llm.openai.base_url"),
		v.GetString("script-service.llm.openai.model"),
	)
	addParallel(keysProviderSpec{
		Provider: "runtime.script.openai.pool",
		Alias:    "script-service openai pool",
		Bases:    stringSliceSetting(v, "script-service.llm.openai.channel_bases"),
		Keys:     stringSliceSetting(v, "script-service.llm.openai.channel_keys"),
		Scope:    []string{v.GetString("script-service.llm.openai.channel_model")},
	}, &keys)

	// character-service
	add("runtime.character.llm", "character-service primary llm",
		v.GetString("character-service.llm.api_key"),
		v.GetString("character-service.llm.base_url"),
		v.GetString("character-service.llm.model"), v.GetString("character-service.llm.vision_model"),
	)
	add("runtime.character.claude", "character-service claude",
		v.GetString("character-service.claude.api_key"),
		v.GetString("character-service.claude.base_url"),
	)
	add("runtime.character.qwen", "character-service qwen",
		v.GetString("character-service.qwen.api_key"),
		v.GetString("character-service.qwen.base_url"),
	)
	add("runtime.character.zhipu", "character-service zhipu",
		v.GetString("character-service.zhipu.api_key"),
		v.GetString("character-service.zhipu.base_url"),
	)
	addParallel(keysProviderSpec{
		Provider: "runtime.character.gemini",
		Alias:    "character-service gemini",
		Bases:    splitList(v.GetString("character-service.gemini.bases")),
		Keys:     splitList(v.GetString("character-service.gemini.keys")),
		Scope:    []string{v.GetString("character-service.gemini.model")},
	}, &keys)

	// image-service
	addParallel(keysProviderSpec{
		Provider: "runtime.image.openai",
		Alias:    "image-service openai",
		Bases:    []string{v.GetString("image-service.models.openai_base")},
		Keys:     stringSliceSetting(v, "image-service.models.openai_keys"),
		Scope:    stringSliceSetting(v, "image-service.models.openai_models"),
	}, &keys)
	addParallel(keysProviderSpec{
		Provider: "runtime.image.tongyi",
		Alias:    "image-service tongyi",
		Bases:    []string{v.GetString("image-service.models.dashscope_base")},
		Keys:     stringSliceSetting(v, "image-service.models.tongyi_keys"),
		Scope:    stringSliceSetting(v, "image-service.models.tongyi_models"),
	}, &keys)
	addParallel(keysProviderSpec{
		Provider: "runtime.image.zhipu",
		Alias:    "image-service zhipu",
		Bases:    []string{v.GetString("image-service.models.zhipu_base")},
		Keys:     stringSliceSetting(v, "image-service.models.zhipu_keys"),
		Scope:    stringSliceSetting(v, "image-service.models.zhipu_models"),
	}, &keys)
	addParallel(keysProviderSpec{
		Provider: "runtime.image.gemini",
		Alias:    "image-service gemini",
		Bases:    stringSliceSetting(v, "image-service.models.gemini_bases"),
		Keys:     stringSliceSetting(v, "image-service.models.gemini_keys"),
		Scope: append([]string{v.GetString("image-service.models.gemini_flash_model")},
			stringSliceSetting(v, "image-service.models.gemini_models")...),
	}, &keys)
	add("runtime.image.replicate", "image-service replicate",
		v.GetString("image-service.models.replicate_key"),
		"",
	)
	add("runtime.image.baidu", "image-service baidu image",
		v.GetString("image-service.models.baidu_image_key"),
		v.GetString("image-service.models.baidu_image_base"),
		v.GetString("image-service.models.baidu_image_model"),
	)
	add("runtime.image.qianfan", "image-service qianfan image",
		v.GetString("image-service.models.qianfan_image_key"),
		v.GetString("image-service.models.qianfan_image_base"),
		stringSliceSetting(v, "image-service.models.qianfan_image_models")..., 
	)
	add("runtime.image.doubao", "image-service doubao image",
		v.GetString("image-service.models.doubao_image_key"),
		v.GetString("image-service.models.doubao_image_base"),
		v.GetString("image-service.models.doubao_image_model"),
		v.GetString("image-service.models.doubao_image_model_v3"),
	)

	// video-service
	addParallel(keysProviderSpec{
		Provider: "runtime.video.kling",
		Alias:    "video-service kling",
		Bases:    []string{v.GetString("video-service.models.kling_base")},
		Keys:     append(splitList(v.GetString("video-service.models.kling_key")), stringSliceSetting(v, "video-service.models.kling_keys")...),
		Scope:    []string{v.GetString("video-service.models.kling_model"), v.GetString("video-service.models.kling_omni_model")},
	}, &keys)
	add("runtime.video.aiping", "video-service aiping",
		v.GetString("video-service.models.aiping_key"),
		v.GetString("video-service.models.aiping_base"),
		v.GetString("video-service.models.kling_model"),
	)
	add("runtime.video.vclm", "video-service tencent vclm",
		joinSecret(v.GetString("video-service.models.vclm_secret_id"), v.GetString("video-service.models.vclm_secret_key")),
		v.GetString("video-service.models.vclm_region"),
	)
	add("runtime.video.wan", "video-service wan",
		joinSecret(v.GetString("video-service.models.wan_key"), v.GetString("video-service.models.wan_secret")),
		v.GetString("video-service.models.wan_base"),
	)
	add("runtime.video.runninghub", "video-service runninghub",
		v.GetString("video-service.models.runninghub_key"),
		v.GetString("video-service.models.runninghub_base"),
		v.GetString("video-service.models.runninghub_workflow"), v.GetString("video-service.models.runninghub_node_id"),
	)
	add("runtime.video.replicate", "video-service replicate",
		v.GetString("video-service.models.replicate_key"),
		"",
	)
	add("runtime.video.sora2", "video-service sora2",
		v.GetString("video-service.models.sora2_key"),
		v.GetString("video-service.models.sora2_base"),
	)
	add("runtime.video.hubagi", "video-service hubagi",
		v.GetString("video-service.models.hubagi_key"),
		v.GetString("video-service.models.hubagi_base"),
		v.GetString("video-service.models.hubagi_model"),
	)
	add("runtime.video.veo", "video-service veo",
		v.GetString("video-service.models.veo_key"),
		v.GetString("video-service.models.veo_base"),
		v.GetString("video-service.models.veo_model"),
	)
	add("runtime.video.doubao", "video-service doubao",
		v.GetString("video-service.models.doubao_key"),
		v.GetString("video-service.models.doubao_base"),
		v.GetString("video-service.models.doubao_model"),
	)
	add("runtime.video.doubao.seedance", "video-service doubao seedance",
		v.GetString("video-service.models.doubao_seedance_key"),
		v.GetString("video-service.models.doubao_seedance_base"),
		v.GetString("video-service.models.doubao_seedance_model"),
	)
	add("runtime.video.vidu", "video-service vidu",
		v.GetString("video-service.models.vidu_key"),
		v.GetString("video-service.models.vidu_base"),
		v.GetString("video-service.models.vidu_model"), v.GetString("video-service.models.vidu_mix_model"),
	)
	add("runtime.video.vidu.offpeak", "video-service vidu offpeak",
		v.GetString("video-service.models.vidu_offpeak_key"),
		v.GetString("video-service.models.vidu_base"),
		v.GetString("video-service.models.vidu_model"), v.GetString("video-service.models.vidu_mix_model"),
	)
	add("runtime.video.suanneng", "video-service suanneng",
		v.GetString("video-service.models.suanneng_key"),
		v.GetString("video-service.models.suanneng_base"),
		v.GetString("video-service.models.suanneng_model"),
	)
	add("runtime.video.gaga", "video-service gaga",
		v.GetString("video-service.models.gaga_key"),
		v.GetString("video-service.models.gaga_base"),
		"gaga-1",
	)
	add("runtime.video.baidu.bce", "video-service baidu bce",
		joinSecret(v.GetString("video-service.models.baidu_bce_key"), v.GetString("video-service.models.baidu_bce_secret")),
		"",
		v.GetString("video-service.models.baidu_bce_model"),
	)
	add("runtime.video.llm", "video-service motion llm",
		v.GetString("video-service.models.llm_key"),
		v.GetString("video-service.models.llm_base"),
		v.GetString("video-service.models.llm_model"),
	)
	add("runtime.video.music", "video-service music",
		v.GetString("video-service.models.music_key"),
		v.GetString("video-service.models.music_base"),
		v.GetString("video-service.models.music_model"),
	)

	return keys, nil
}

type keysProviderSpec struct {
	Provider string
	Alias    string
	Bases    []string
	Keys     []string
	Scope    []string
}

func addParallel(spec keysProviderSpec, keys *[]model.SystemAPIKey) {
	if len(spec.Keys) == 0 {
		return
	}
	baseFallback := ""
	if len(spec.Bases) > 0 {
		baseFallback = strings.TrimSpace(spec.Bases[0])
	}
	scope := joinScopes(spec.Scope...)
	for i, key := range spec.Keys {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		base := baseFallback
		if i < len(spec.Bases) && strings.TrimSpace(spec.Bases[i]) != "" {
			base = strings.TrimSpace(spec.Bases[i])
		}
		*keys = append(*keys, model.SystemAPIKey{
			Provider:   spec.Provider,
			KeyAlias:   fmt.Sprintf("%s #%d", spec.Alias, i+1),
			PlainKey:   trimmedKey,
			BaseURL:    base,
			ModelScope: scope,
			IsActive:   true,
			Status:     "active",
			CreatedAt:  time.Now(),
		})
	}
}

func stringSliceSetting(v *viper.Viper, path string) []string {
	return splitList(v.GetString(path))
}

func splitList(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
	})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func joinScopes(scopes ...string) string {
	clean := make([]string, 0, len(scopes))
	seen := map[string]struct{}{}
	for _, scope := range scopes {
		for _, item := range splitList(scope) {
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			clean = append(clean, item)
		}
	}
	return strings.Join(clean, ",")
}

func joinSecret(parts ...string) string {
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			clean = append(clean, trimmed)
		}
	}
	if len(clean) == 0 {
		return ""
	}
	return strings.Join(clean, ":")
}

func defaultConfigFile() string {
	if path := os.Getenv("AUTOVIDEO_CONFIG_FILE"); path != "" {
		return path
	}
	return "../../config.local.yaml"
}