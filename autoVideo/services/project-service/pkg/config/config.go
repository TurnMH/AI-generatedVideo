package config

import (
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Database    DatabaseConfig    `mapstructure:"database"`
	AuthService AuthServiceConfig `mapstructure:"auth_service"`
	JWT         JWTConfig         `mapstructure:"jwt"`
	GRPC        GRPCConfig        `mapstructure:"grpc"`
	Kafka       KafkaConfig       `mapstructure:"kafka"`
	Image       ImageConfig       `mapstructure:"image"`
	Character   CharacterConfig   `mapstructure:"character"`
	Script      ScriptConfig      `mapstructure:"script"`
	Storage     StorageConfig     `mapstructure:"storage"`
	Video       VideoConfig       `mapstructure:"video"`
	Concurrency ConcurrencyConfig `mapstructure:"concurrency"`
	LLM         LLMConfig         `mapstructure:"llm"`
	Gateway     struct {
		Addr     string `mapstructure:"addr"`
		SelfAddr string `mapstructure:"self_addr"`
	} `mapstructure:"gateway"`
}

type LLMConfig struct {
	BaseURL string `mapstructure:"base_url"`
	APIKey  string `mapstructure:"api_key"`
	Model   string `mapstructure:"model"`
	Timeout int    `mapstructure:"timeout"`
	// Fallback providers — used when the primary LLM key is empty or fails.
	// Both endpoints are OpenAI-compatible (chat/completions format).
	// BigModel (GLM-5+): base_url = https://open.bigmodel.cn/api/paas/v4
	// Qianfan (Baidu): base_url = https://qianfan.baidubce.com/v2
	FallbackBaseURL string `mapstructure:"fallback_base_url"`
	FallbackAPIKey  string `mapstructure:"fallback_api_key"`
	FallbackModel   string `mapstructure:"fallback_model"`
}

type KafkaConfig struct {
	Brokers       []string `mapstructure:"brokers"`
	ConsumerGroup string   `mapstructure:"consumer_group"`
	ConsumerTopic string   `mapstructure:"consumer_topic"`
	ProducerTopic string   `mapstructure:"producer_topic"`
}

type ImageConfig struct {
	BaseURL string `mapstructure:"base_url"`
}

type CharacterConfig struct {
	BaseURL string `mapstructure:"base_url"`
}

type ScriptConfig struct {
	BaseURL string `mapstructure:"base_url"`
}

type StorageConfig struct {
	BaseURL string `mapstructure:"base_url"`
}

type VideoConfig struct {
	BaseURL string `mapstructure:"base_url"`
}

type ConcurrencyConfig struct {
	MaxStoryboardGenerations int `mapstructure:"max_storyboard_generations"`
	MaxStoryboardInFlight    int `mapstructure:"max_storyboard_in_flight"`
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

func normalizedStringSlice(raw any) []string {
	switch value := raw.(type) {
	case []string:
		normalized := make([]string, 0, len(value))
		for _, item := range value {
			trimmed := strings.TrimSpace(item)
			if trimmed != "" {
				normalized = append(normalized, trimmed)
			}
		}
		return normalized
	case []any:
		normalized := make([]string, 0, len(value))
		for _, item := range value {
			text, ok := item.(string)
			if !ok {
				continue
			}
			trimmed := strings.TrimSpace(text)
			if trimmed != "" {
				normalized = append(normalized, trimmed)
			}
		}
		return normalized
	case string:
		return parseDelimitedKeys(value)
	default:
		return nil
	}
}

func resolveServiceBaseURL(v *viper.Viper, canonicalPath string, fallbackPaths ...string) string {
	if value := strings.TrimSpace(v.GetString(canonicalPath)); value != "" && value != "http://localhost:8004" && value != "http://localhost:8003" {
		return value
	}
	for _, path := range fallbackPaths {
		value := strings.TrimSpace(v.GetString(path))
		if value != "" {
			return value
		}
	}
	return strings.TrimSpace(v.GetString(canonicalPath))
}

func mergeOverrideConfig(v *viper.Viper) error {
	overrideFile := strings.TrimSpace(os.Getenv("AUTOVIDEO_CONFIG_OVERRIDE_FILE"))
	if overrideFile == "" {
		return nil
	}
	file, err := os.Open(overrideFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()
	return v.MergeConfig(file)
}

func watchedConfigPaths(path string) []string {
	paths := make([]string, 0, 2)
	seen := map[string]struct{}{}
	for _, candidate := range []string{strings.TrimSpace(path), strings.TrimSpace(os.Getenv("AUTOVIDEO_CONFIG_OVERRIDE_FILE"))} {
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		paths = append(paths, candidate)
	}
	return paths
}

func providerKeyCount(singlePath, listPath string) int {
	keys := append([]string{}, parseDelimitedKeys(viper.GetString(singlePath))...)
	keys = append(keys, viper.GetStringSlice(listPath)...)
	keys = append(keys, parseDelimitedKeys(viper.GetString(listPath))...)
	seen := map[string]struct{}{}
	count := 0
	for _, key := range keys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		count++
	}
	return count
}

// recommendedStoryboardConcurrency returns (generations, inFlight) based on how
// many image-service workers are available.
//   - generations: how many Kafka dispatch messages to send per tick
//     (= imageWorkers; each worker gets ~1 task in flight at a time)
//   - inFlight: max storyboards tracked as "generating" before dispatch pauses
//     (= generations * 2 provides a shallow queue buffer)
//
// Extra API keys scale generations linearly.
func recommendedStoryboardConcurrency(imageWorkers, maxProviderKeys int) (int, int) {
	if imageWorkers <= 0 {
		imageWorkers = 4
	}
	generations := imageWorkers
	if maxProviderKeys > 1 {
		generations += maxProviderKeys - 1
	}
	if generations < 4 {
		generations = 4
	}
	if generations > 48 {
		generations = 48
	}
	inFlight := generations * 2
	if inFlight < 8 {
		inFlight = 8
	}
	return generations, inFlight
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
}

type DatabaseConfig struct {
	DSN string `mapstructure:"dsn"`
}

type AuthServiceConfig struct {
	GRPCAddr string `mapstructure:"grpc_addr"`
	BaseURL  string `mapstructure:"base_url"`
}

type JWTConfig struct {
	AccessSecret string `mapstructure:"access_secret"`
}

type GRPCConfig struct {
	Port string `mapstructure:"port"`
}

// Load —— 加载应用配置，按优先级合并配置文件、环境变量和默认值
func Load(logger *zap.Logger) (*Config, error) {
	viper.SetDefault("server.port", "8002")
	viper.SetDefault("database.dsn", "host=localhost user=postgres password=postgres dbname=project_db port=5432 sslmode=disable")
	viper.SetDefault("auth_service.grpc_addr", "localhost:9001")
	viper.SetDefault("auth_service.base_url", "http://localhost:8001")
	viper.SetDefault("jwt.access_secret", "autovideo-access-secret-change-in-prod")
	viper.SetDefault("grpc.port", "9002")
	viper.SetDefault("kafka.brokers", []string{"localhost:9092"})
	viper.SetDefault("kafka.consumer_group", "project-service")
	viper.SetDefault("kafka.consumer_topic", "storyboard.generate.request")
	viper.SetDefault("kafka.producer_topic", "storyboard.generate.result")
	viper.SetDefault("image.base_url", "http://localhost:8005")
	viper.SetDefault("character.base_url", "http://localhost:8004")
	viper.SetDefault("script.base_url", "http://localhost:8003")
	viper.SetDefault("storage.base_url", "http://localhost:8009")
	viper.SetDefault("video.base_url", "http://localhost:8006")
	viper.SetDefault("concurrency.max_storyboard_generations", 0)
	viper.SetDefault("concurrency.max_storyboard_in_flight", 0)
	viper.SetDefault("gateway.addr", "http://localhost:8000")
	viper.SetDefault("gateway.self_addr", "")
	viper.SetDefault("llm.base_url", "https://api.easyart.cc/v1")
	viper.SetDefault("llm.api_key", "")
	viper.SetDefault("llm.model", "gpt-5.4-mini")
	viper.SetDefault("llm.timeout", 120)
	// Fallback LLM provider — e.g. BigModel (GLM-5) or Baidu Qianfan
	viper.SetDefault("llm.fallback_base_url", "")
	viper.SetDefault("llm.fallback_api_key", "")
	viper.SetDefault("llm.fallback_model", "")

	viper.SetConfigType("yaml")

	if configFile := os.Getenv("AUTOVIDEO_CONFIG_FILE"); configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		// Load unified config from project root
		viper.SetConfigName("config")
		viper.AddConfigPath("../../")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")
		viper.AddConfigPath("/etc/project-service")
	}

	viper.SetEnvPrefix("PROJECT")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	sharedKafkaBrokers := []string(nil)
	serviceKafkaBrokers := []string(nil)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
		if logger != nil {
			logger.Warn("config file not found, using defaults and environment variables")
		}
	}
	if err := mergeOverrideConfig(viper.GetViper()); err != nil {
		return nil, err
	}
	sharedKafkaBrokers = normalizedStringSlice(viper.Get("kafka.brokers"))

	// Merge service-specific section on top of shared values
	if sub := viper.Sub("project-service"); sub != nil {
		serviceKafkaBrokers = normalizedStringSlice(sub.Get("kafka.brokers"))
		viper.MergeConfigMap(sub.AllSettings())
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	if len(serviceKafkaBrokers) > 0 {
		cfg.Kafka.Brokers = serviceKafkaBrokers
	} else if len(sharedKafkaBrokers) > 0 {
		cfg.Kafka.Brokers = sharedKafkaBrokers
	}
	cfg.Character.BaseURL = resolveServiceBaseURL(viper.GetViper(), "character.base_url", "character_service.base_url", "character_service.url")
	cfg.Script.BaseURL = resolveServiceBaseURL(viper.GetViper(), "script.base_url", "script_service.base_url", "script_service.url")
	imageWorkers := viper.GetInt("image-service.concurrency.max_workers")
	maxProviderKeys := 1
	for _, count := range []int{
		providerKeyCount("image-service.models.openai_key", "image-service.models.openai_keys"),
		providerKeyCount("image-service.models.tongyi_key", "image-service.models.tongyi_keys"),
		providerKeyCount("image-service.models.replicate_key", "image-service.models.replicate_keys"),
	} {
		if count > maxProviderKeys {
			maxProviderKeys = count
		}
	}
	recommendedGenerations, recommendedInFlight := recommendedStoryboardConcurrency(imageWorkers, maxProviderKeys)
	if cfg.Concurrency.MaxStoryboardGenerations <= 0 {
		cfg.Concurrency.MaxStoryboardGenerations = recommendedGenerations
	}
	if cfg.Concurrency.MaxStoryboardInFlight <= 0 {
		cfg.Concurrency.MaxStoryboardInFlight = recommendedInFlight
	}
	return &cfg, nil
}

// StartWatcher polls the config file at path every 30 seconds and calls onChange
// when the file's modification time changes. It runs in a background goroutine.
func StartWatcher(path string, logger *zap.Logger, onChange func(*Config)) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		lastMod := map[string]time.Time{}
		for _, candidate := range watchedConfigPaths(path) {
			if info, err := os.Stat(candidate); err == nil {
				lastMod[candidate] = info.ModTime()
			}
		}
		for range ticker.C {
			changed := false
			for _, candidate := range watchedConfigPaths(path) {
				info, err := os.Stat(candidate)
				if err != nil {
					continue
				}
				if info.ModTime().After(lastMod[candidate]) {
					lastMod[candidate] = info.ModTime()
					changed = true
				}
			}
			if !changed {
				continue
			}
			cfg, err := Load(logger)
			if err != nil {
				if logger != nil {
					logger.Warn("config hot-reload failed", zap.Error(err))
				}
				continue
			}
			onChange(cfg)
		}
	}()
}
