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
	DB struct {
		DSN string `mapstructure:"dsn"`
	} `mapstructure:"db"`
	Redis struct {
		Addr     string `mapstructure:"addr"`
		Password string `mapstructure:"password"`
		DB       int    `mapstructure:"db"`
	} `mapstructure:"redis"`
	Kafka struct {
		Brokers       []string `mapstructure:"brokers"`
		ProducerTopic string   `mapstructure:"producer_topic"`
		ConsumerTopic string   `mapstructure:"consumer_topic"`
	} `mapstructure:"kafka"`
	LLM struct {
		Provider string `mapstructure:"provider"`
		OpenAI   struct {
			BaseURL string `mapstructure:"base_url"`
			APIKey  string `mapstructure:"api_key"`
			Model   string `mapstructure:"model"`
			// Multi-channel pool for concurrent requests (GPT-5.4 and similar).
			// ChannelBases and ChannelKeys are parallel slices; base[i] paired with key[i].
			// ChannelModel overrides the per-request model for all pool channels.
			ChannelBases  []string `mapstructure:"channel_bases"`
			ChannelKeys   []string `mapstructure:"channel_keys"`
			ChannelModel  string   `mapstructure:"channel_model"`
		} `mapstructure:"openai"`
		Claude struct {
			BaseURL string `mapstructure:"base_url"`
			APIKey  string `mapstructure:"api_key"`
			Model   string `mapstructure:"model"`
		} `mapstructure:"claude"`
	} `mapstructure:"llm"`
	JWT struct {
		Secret string `mapstructure:"secret"`
	} `mapstructure:"jwt"`
	Storage struct {
		ServiceURL string `mapstructure:"service_url"`
	} `mapstructure:"storage"`
	ImageService struct {
		BaseURL      string `mapstructure:"base_url"`
		DefaultModel string `mapstructure:"default_model"`
	} `mapstructure:"image_service"`
	CharacterService struct {
		URL string `mapstructure:"url"`
	} `mapstructure:"character_service"`
	AuthService struct {
		BaseURL string `mapstructure:"base_url"`
	} `mapstructure:"auth_service"`
	Gateway struct {
		Addr     string `mapstructure:"addr"`
		SelfAddr string `mapstructure:"self_addr"`
	} `mapstructure:"gateway"`
	AllowedOrigins []string `mapstructure:"allowed_origins"`
}

// Load —— 加载剧本服务配置，从环境变量和配置文件中读取参数，返回 *Config 和错误
func Load() (*Config, error) {
	viper.SetConfigType("yaml")

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// defaults
	viper.SetDefault("http.port", 8003)
	viper.SetDefault("llm.provider", "openai")
	viper.SetDefault("llm.openai.model", "gpt-4o-mini")
	viper.SetDefault("llm.openai.base_url", "https://api.openai.com/v1")
	viper.SetDefault("kafka.producer_topic", "script.analyze.result")
	viper.SetDefault("kafka.consumer_topic", "script.analyze.request")
	viper.SetDefault("image_service.base_url", "http://localhost:8005")
	viper.SetDefault("image_service.default_model", "gpt-image-1.5")
	viper.SetDefault("character_service.url", "http://localhost:8004")
	viper.SetDefault("auth_service.base_url", "http://localhost:8001")
	viper.SetDefault("gateway.addr", "http://localhost:8000")
	viper.SetDefault("gateway.self_addr", "")
	viper.SetDefault("allowed_origins", []string{"http://localhost:3000", "http://127.0.0.1:3000"})

	if configFile := os.Getenv("AUTOVIDEO_CONFIG_FILE"); configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		// Load unified config from project root
		viper.SetConfigName("config")
		viper.AddConfigPath("../../")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")
		viper.AddConfigPath("/app/config")
	}

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	// Merge service-specific section on top of shared values
	if sub := viper.Sub("script-service"); sub != nil {
		viper.MergeConfigMap(sub.AllSettings())
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
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
				if cfg, err := Load(); err == nil {
					onChange(cfg)
				}
			}
		}
	}()
}
