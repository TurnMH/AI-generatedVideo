package config

import (
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	HTTP struct {
		Port int `mapstructure:"port"`
	} `mapstructure:"http"`
	GRPCPort int `mapstructure:"grpc_port"`
	DB       struct {
		DSN string `mapstructure:"dsn"`
	} `mapstructure:"db"`
	JWT struct {
		Secret       string `mapstructure:"secret"`
		AccessSecret string `mapstructure:"access_secret"`
	} `mapstructure:"jwt"`
	Storage struct {
		BaseURL string `mapstructure:"base_url"`
	} `mapstructure:"storage"`
	Kafka struct {
		Brokers       []string `mapstructure:"brokers"`
		ConsumerGroup string   `mapstructure:"consumer_group"`
		ConsumerTopic string   `mapstructure:"consumer_topic"`
		ProducerTopic string   `mapstructure:"producer_topic"`
	} `mapstructure:"kafka"`
	Image struct {
		BaseURL string `mapstructure:"base_url"`
	} `mapstructure:"image"`
	LLM struct {
		BaseURL     string `mapstructure:"base_url"`
		APIKey      string `mapstructure:"api_key"`
		Model       string `mapstructure:"model"`
		VisionModel string `mapstructure:"vision_model"`
		Timeout     int    `mapstructure:"timeout"`
	} `mapstructure:"llm"`
	Gemini struct {
		Bases string `mapstructure:"bases"` // comma-separated proxy base URLs
		Keys  string `mapstructure:"keys"`  // comma-separated API keys (parallel to bases)
		Model string `mapstructure:"model"` // e.g. gemini-2.0-flash-exp
	} `mapstructure:"gemini"`
	Claude struct {
		BaseURL string `mapstructure:"base_url"`
		APIKey  string `mapstructure:"api_key"`
	} `mapstructure:"claude"`
	Qwen struct {
		BaseURL string `mapstructure:"base_url"`
		APIKey  string `mapstructure:"api_key"`
	} `mapstructure:"qwen"`
	Zhipu struct {
		BaseURL string `mapstructure:"base_url"`
		APIKey  string `mapstructure:"api_key"`
	} `mapstructure:"zhipu"`
	ProjectService struct {
		BaseURL string `mapstructure:"base_url"`
	} `mapstructure:"project_service"`
	ModelService struct {
		BaseURL string `mapstructure:"base_url"`
	} `mapstructure:"model_service"`
	Concurrency struct {
		MaxGenerations int `mapstructure:"max_generations"`
	} `mapstructure:"concurrency"`
	AuthService struct {
		BaseURL string `mapstructure:"base_url"`
	} `mapstructure:"auth_service"`
	Gateway struct {
		Addr     string `mapstructure:"addr"`
		SelfAddr string `mapstructure:"self_addr"`
	} `mapstructure:"gateway"`
}

// Load —— 加载配置文件和环境变量，返回合并后的 *Config
func Load() (*Config, error) {
	viper.SetConfigType("yaml")

	viper.SetEnvPrefix("CHARACTER")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// defaults
	viper.SetDefault("http.port", 8004)
	viper.SetDefault("grpc_port", 9004)
	viper.SetDefault("db.dsn", "host=localhost user=postgres password=postgres dbname=character_db port=5432 sslmode=disable TimeZone=Asia/Shanghai")
	viper.SetDefault("jwt.secret", "autovideo-access-secret-change-in-prod")
	viper.SetDefault("jwt.access_secret", "autovideo-access-secret-change-in-prod")
	viper.SetDefault("storage.base_url", "http://storage-service:8003")
	viper.SetDefault("kafka.consumer_group", "character-service")
	viper.SetDefault("kafka.consumer_topic", "asset.generate.request")
	viper.SetDefault("kafka.producer_topic", "asset.generate.result")
	viper.SetDefault("image.base_url", "http://localhost:8005")
	viper.SetDefault("llm.base_url", "https://api.easyart.cc/v1")
	viper.SetDefault("llm.api_key", "")
	viper.SetDefault("llm.model", "gpt-5.4-mini")
	viper.SetDefault("llm.vision_model", "")
	viper.SetDefault("llm.timeout", 120)
	viper.SetDefault("project_service.base_url", "http://localhost:8002")
	viper.SetDefault("auth_service.base_url", "http://localhost:8001")
	viper.SetDefault("model_service.base_url", "http://localhost:8008")
	viper.SetDefault("gateway.addr", "http://localhost:8000")
	viper.SetDefault("gateway.self_addr", "")

	if configFile := os.Getenv("AUTOVIDEO_CONFIG_FILE"); configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		// Load unified config from project root
		viper.SetConfigName("config")
		viper.AddConfigPath("../../")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")
	}

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}
	if err := mergeOverrideConfig(); err != nil {
		return nil, err
	}

	// Merge service-specific section on top of shared values
	if sub := viper.Sub("character-service"); sub != nil {
		viper.MergeConfigMap(sub.AllSettings())
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func mergeOverrideConfig() error {
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
	return viper.MergeConfig(file)
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

// StartWatcher polls the config file at path every 30 seconds and calls onChange
// when the file's modification time changes. It runs in a background goroutine.
func StartWatcher(path string, onChange func(*Config)) {
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
			if cfg, err := Load(); err == nil {
				onChange(cfg)
			}
		}
	}()
}
