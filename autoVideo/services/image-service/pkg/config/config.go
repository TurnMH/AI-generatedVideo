package config

import (
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	HTTP struct {
		Port int
	}
	DB struct {
		DSN string
	}
	Kafka struct {
		Brokers       []string
		ConsumerGroup string
		ConsumerTopic string
		ProducerTopic string
	}
	JWT struct {
		Secret string
	}
	Storage struct {
		BaseURL string
	}
	AuthService struct {
		BaseURL string
	}
	Models      ModelsConfig
	Concurrency ConcurrencyConfig
	Gateway     struct {
		Addr     string
		SelfAddr string
	}
}

type ModelsConfig struct {
	ComfyUIURL      string
	ComfyUIURLs     []string
	ComfyUIWorkflow string
	ReplicateKey    string
	ReplicateKeys   []string
	OpenAIKey       string
	OpenAIKeys      []string
	OpenAIBase      string
	OpenAIModels    []string // extra model names to expose via OpenAI-compatible key
	TongyiKey       string
	TongyiKeys      []string
	DashScopeBase   string
	TongyiModels    []string // extra model names to expose via DashScope key
	ZhipuKey        string
	ZhipuKeys       []string
	ZhipuBase       string
	ZhipuModels     []string // ZhipuAI CogView image generation models
	// Gemini image generation channels (proxy-based, e.g. polo, aitoo, amutes, etc.)
	// GeminiKeys and GeminiBases are parallel slices; key[i] is paired with base[i].
	// If there are fewer bases than keys the last base is reused.
	GeminiKeys       []string
	GeminiBases      []string
	GeminiFlashModel string   // API model name for the flash-image generator (default: gemini-2.5-flash-image)
	GeminiModels     []string // extra model aliases registered via Gemini generator
	// Baidu BCE image generation (vod.bj.baidubce.com/v3/aigc/image) — async task-based
	// Available models: NB, NBP, NB2, I4YG1, I4FG1, I4G1
	// ⚠️  GET polling requires BCE-AUTH-V1 HMAC; bce-v3 Bearer token is NOT accepted for GET.
	// Set baidu_image_ak / baidu_image_sk to standard IAM AK/SK for HMAC signing.
	BaiduImageKey   string // full bce-v3/ALTAK-... bearer token for task creation (POST)
	BaiduImageAK    string // standard IAM AK for BCE-AUTH-V1 HMAC signing (task polling GET)
	BaiduImageSK    string // standard IAM SK for BCE-AUTH-V1 HMAC signing (task polling GET)
	BaiduImageBase  string // e.g. https://vod.bj.baidubce.com/v3/aigc/image
	BaiduImageModel string // one of NB/NBP/NB2/I4YG1/I4FG1/I4G1, default NB
	// Qianfan image generation (qianfan.baidubce.com/v2/images/generations) — synchronous ✅
	// Uses same bce-v3 Bearer token format; OpenAI-compatible response.
	// Verified models: flux.1-schnell, stable-diffusion-xl
	QianfanImageKey    string   // bce-v3/ALTAK-... bearer token
	QianfanImageBase   string   // https://qianfan.baidubce.com/v2/images/generations
	QianfanImageModels []string // list of model names to register (each gets its own key)
	// Doubao (ByteDance Ark) image generation — OpenAI images/generations compatible
	// Endpoint: https://ark.cn-beijing.volces.com/api/v3/images/generations
	DoubaoImageKey     string   `mapstructure:"doubao_image_key"`
	DoubaoImageBase    string   `mapstructure:"doubao_image_base"`    // defaults to https://ark.cn-beijing.volces.com/api/v3/images/generations
	DoubaoImageModel   string   `mapstructure:"doubao_image_model"`   // e.g. doubao-seedream-4-0-250828
	DoubaoImageModelV3 string   `mapstructure:"doubao_image_model_v3"` // optional: e.g. doubao-seedream-3-0-t2i-250415 (must be activated in Ark console)
}

type ConcurrencyConfig struct {
	MaxWorkers       int
	PrioritySlots    int
	PerChannel       map[string]int // per-generator concurrency limits, e.g. {"gemini": 6, "dalle": 8}
	GeminiPerChannel int             // max concurrent requests per Gemini (base,key) pair (default 2)
}

func parseDelimitedKeys(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
	})
	keys := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			keys = append(keys, trimmed)
		}
	}
	return keys
}

func mergeKeys(single string, multiple []string, rawList string) []string {
	all := append([]string{}, parseDelimitedKeys(single)...)
	// Each item in multiple may itself be a comma/semicolon-delimited string
	for _, item := range multiple {
		all = append(all, parseDelimitedKeys(item)...)
	}
	all = append(all, parseDelimitedKeys(rawList)...)
	seen := make(map[string]struct{}, len(all))
	keys := make([]string, 0, len(all))
	for _, key := range all {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		keys = append(keys, trimmed)
	}
	return keys
}

func MaxProviderKeyCount(cfg *Config) int {
	maxCount := len(cfg.Models.OpenAIKeys)
	if len(cfg.Models.TongyiKeys) > maxCount {
		maxCount = len(cfg.Models.TongyiKeys)
	}
	if len(cfg.Models.ReplicateKeys) > maxCount {
		maxCount = len(cfg.Models.ReplicateKeys)
	}
	if len(cfg.Models.ComfyUIURLs) > maxCount {
		maxCount = len(cfg.Models.ComfyUIURLs)
	}
	if maxCount == 0 {
		return 1
	}
	return maxCount
}

// RecommendedMaxWorkers returns the concurrency cap for image generation workers.
// With an explicit concurrency.max_workers config it is used as-is.
// Otherwise the formula is keyCount * 4 — 4 concurrent requests per API key —
// which stays within standard rate-limit tiers (DALL-E tier-1: 5 img/min,
// Tongyi standard: 3-5 img/min).  More keys linearly scale capacity.
func RecommendedMaxWorkers(cfg *Config) int {
	if base := cfg.Concurrency.MaxWorkers; base > 0 {
		return base
	}
	keyCount := MaxProviderKeyCount(cfg)
	recommended := keyCount * 4
	if recommended < 4 {
		recommended = 4
	}
	if recommended > 80 {
		recommended = 80
	}
	return recommended
}

func RecommendedPrioritySlots(maxWorkers int) int {
	slots := maxWorkers / 2
	if slots < 2 {
		slots = 2
	}
	if slots > maxWorkers {
		slots = maxWorkers
	}
	return slots
}

// PerGeneratorSlots returns the concurrency limit for a specific generator key.
// Priority: explicit per_channel config > auto-calculated default.
// Default is maxWorkers/generatorCount, minimum 2.
func PerGeneratorSlots(cfg *Config, genKey string, totalGenerators int) int {
	if n, ok := cfg.Concurrency.PerChannel[genKey]; ok && n > 0 {
		return n
	}
	maxW := RecommendedMaxWorkers(cfg)
	if totalGenerators <= 0 {
		totalGenerators = 1
	}
	slots := maxW / totalGenerators
	if slots < 2 {
		slots = 2
	}
	return slots
}

func RecommendedLocalSlots(cfg *Config) int {
	slots := len(cfg.Models.ComfyUIURLs)
	if slots <= 0 {
		return 1
	}
	return slots
}

// Load —— 加载图片服务配置，从环境变量和配置文件中读取参数，返回 *Config
func Load() *Config {
	viper.SetConfigType("yaml")

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Defaults
	viper.SetDefault("http.port", 8005)
	viper.SetDefault("db.dsn", "host=postgres user=postgres password=postgres dbname=image_db port=5432 sslmode=disable TimeZone=UTC")
	viper.SetDefault("kafka.brokers", []string{"kafka:9092"})
	viper.SetDefault("kafka.consumer_group", "image-service")
	viper.SetDefault("kafka.consumer_topic", "image.generate.request")
	viper.SetDefault("kafka.producer_topic", "image.generate.result")
	viper.SetDefault("jwt.secret", "change-me-in-production")
	viper.SetDefault("storage.base_url", "http://localhost:8009")
	viper.SetDefault("models.comfyui_url", "http://comfyui:8188")
	viper.SetDefault("models.dashscope_base", "https://dashscope.aliyuncs.com")
	viper.SetDefault("models.openai_base", "https://api.openai.com")
	viper.SetDefault("models.zhipu_base", "https://open.bigmodel.cn/api/paas/v4")
	viper.SetDefault("auth_service.base_url", "http://localhost:8001")
	viper.SetDefault("gateway.addr", "http://localhost:8000")
	viper.SetDefault("gateway.self_addr", "")

	if configFile := os.Getenv("AUTOVIDEO_CONFIG_FILE"); configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		// Load unified config from project root
		viper.SetConfigName("config")
		viper.AddConfigPath("../../")
		viper.AddConfigPath(".")
		viper.AddConfigPath("/app")
	}
	_ = viper.ReadInConfig()

	// Merge service-specific section on top of shared values
	if sub := viper.Sub("image-service"); sub != nil {
		viper.MergeConfigMap(sub.AllSettings())
	}

	var cfg Config
	cfg.HTTP.Port = viper.GetInt("http.port")
	cfg.DB.DSN = viper.GetString("db.dsn")
	cfg.Kafka.Brokers = viper.GetStringSlice("kafka.brokers")
	cfg.Kafka.ConsumerGroup = viper.GetString("kafka.consumer_group")
	cfg.Kafka.ConsumerTopic = viper.GetString("kafka.consumer_topic")
	cfg.Kafka.ProducerTopic = viper.GetString("kafka.producer_topic")
	cfg.JWT.Secret = viper.GetString("jwt.secret")
	cfg.Storage.BaseURL = viper.GetString("storage.base_url")
	cfg.AuthService.BaseURL = viper.GetString("auth_service.base_url")
	cfg.Models.ComfyUIURL = viper.GetString("models.comfyui_url")
	cfg.Models.ComfyUIURLs = mergeKeys(
		cfg.Models.ComfyUIURL,
		viper.GetStringSlice("models.comfyui_urls"),
		viper.GetString("models.comfyui_urls"),
	)
	cfg.Models.ComfyUIWorkflow = viper.GetString("models.comfyui_workflow")
	cfg.Models.ReplicateKey = viper.GetString("models.replicate_key")
	cfg.Models.ReplicateKeys = mergeKeys(
		cfg.Models.ReplicateKey,
		viper.GetStringSlice("models.replicate_keys"),
		viper.GetString("models.replicate_keys"),
	)
	cfg.Models.OpenAIKey = viper.GetString("models.openai_key")
	cfg.Models.OpenAIKeys = mergeKeys(
		cfg.Models.OpenAIKey,
		viper.GetStringSlice("models.openai_keys"),
		viper.GetString("models.openai_keys"),
	)
	cfg.Models.OpenAIBase = viper.GetString("models.openai_base")
	cfg.Models.OpenAIModels = mergeKeys(
		"",
		viper.GetStringSlice("models.openai_models"),
		viper.GetString("models.openai_models"),
	)
	cfg.Models.TongyiKey = viper.GetString("models.tongyi_key")
	cfg.Models.TongyiKeys = mergeKeys(
		cfg.Models.TongyiKey,
		viper.GetStringSlice("models.tongyi_keys"),
		viper.GetString("models.tongyi_keys"),
	)
	cfg.Models.DashScopeBase = viper.GetString("models.dashscope_base")
	cfg.Models.TongyiModels = mergeKeys(
		"",
		viper.GetStringSlice("models.tongyi_models"),
		viper.GetString("models.tongyi_models"),
	)
	cfg.Models.ZhipuKey = viper.GetString("models.zhipu_key")
	cfg.Models.ZhipuKeys = mergeKeys(
		cfg.Models.ZhipuKey,
		viper.GetStringSlice("models.zhipu_keys"),
		viper.GetString("models.zhipu_keys"),
	)
	cfg.Models.ZhipuBase = viper.GetString("models.zhipu_base")
	cfg.Models.ZhipuModels = mergeKeys(
		"",
		viper.GetStringSlice("models.zhipu_models"),
		viper.GetString("models.zhipu_models"),
	)
	// Gemini image channels
	cfg.Models.GeminiKeys = mergeKeys(
		viper.GetString("models.gemini_key"),
		viper.GetStringSlice("models.gemini_keys"),
		viper.GetString("models.gemini_keys"),
	)
	cfg.Models.GeminiBases = mergeKeys(
		viper.GetString("models.gemini_base"),
		viper.GetStringSlice("models.gemini_bases"),
		viper.GetString("models.gemini_bases"),
	)
	cfg.Models.GeminiFlashModel = viper.GetString("models.gemini_flash_model")
	if cfg.Models.GeminiFlashModel == "" {
		cfg.Models.GeminiFlashModel = "gemini-2.5-flash-image"
	}
	cfg.Models.GeminiModels = mergeKeys(
		"",
		viper.GetStringSlice("models.gemini_models"),
		viper.GetString("models.gemini_models"),
	)
	// Baidu BCE image channel
	cfg.Models.BaiduImageKey = viper.GetString("models.baidu_image_key")
	cfg.Models.BaiduImageAK = viper.GetString("models.baidu_image_ak")
	cfg.Models.BaiduImageSK = viper.GetString("models.baidu_image_sk")
	cfg.Models.BaiduImageBase = viper.GetString("models.baidu_image_base")
	cfg.Models.BaiduImageModel = viper.GetString("models.baidu_image_model")
	// Qianfan image channel (synchronous, OpenAI-compatible)
	cfg.Models.QianfanImageKey = viper.GetString("models.qianfan_image_key")
	cfg.Models.QianfanImageBase = viper.GetString("models.qianfan_image_base")
	cfg.Models.QianfanImageModels = mergeKeys(
		"",
		viper.GetStringSlice("models.qianfan_image_models"),
		viper.GetString("models.qianfan_image_models"),
	)
	// Doubao (Ark) image channel
	cfg.Models.DoubaoImageKey = viper.GetString("models.doubao_image_key")
	cfg.Models.DoubaoImageBase = viper.GetString("models.doubao_image_base")
	cfg.Models.DoubaoImageModel = viper.GetString("models.doubao_image_model")
	cfg.Models.DoubaoImageModelV3 = viper.GetString("models.doubao_image_model_v3")
	cfg.Concurrency.MaxWorkers = viper.GetInt("concurrency.max_workers")
	cfg.Concurrency.PrioritySlots = viper.GetInt("concurrency.priority_slots")
	cfg.Concurrency.GeminiPerChannel = viper.GetInt("concurrency.gemini_per_channel")
	if cfg.Concurrency.GeminiPerChannel <= 0 {
		cfg.Concurrency.GeminiPerChannel = 2
	}
	perChannelRaw := viper.GetStringMap("concurrency.per_channel")
	cfg.Concurrency.PerChannel = make(map[string]int, len(perChannelRaw))
	for k, v := range perChannelRaw {
		switch val := v.(type) {
		case int:
			cfg.Concurrency.PerChannel[k] = val
		case float64:
			cfg.Concurrency.PerChannel[k] = int(val)
		}
	}
	cfg.Gateway.Addr = viper.GetString("gateway.addr")
	cfg.Gateway.SelfAddr = viper.GetString("gateway.self_addr")

	return &cfg
}

// StartWatcher polls the config file at path every 30 seconds and calls onChange
// when the file's modification time changes. It runs in a background goroutine.
func StartWatcher(path string, onChange func(*Config)) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		var lastMod time.Time
		if info, err := os.Stat(path); err == nil {
			lastMod = info.ModTime()
		}
		for range ticker.C {
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			if info.ModTime().After(lastMod) {
				lastMod = info.ModTime()
				onChange(Load())
			}
		}
	}()
}
