package config

import (
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	HTTP struct {
		Port int `mapstructure:"port"`
	} `mapstructure:"http"`

	Gateway struct {
		Addr     string `mapstructure:"addr"`
		SelfAddr string `mapstructure:"self_addr"`
	} `mapstructure:"gateway"`

	Agent struct {
		DefaultModel string `mapstructure:"default_model"`
		SystemPrompt string `mapstructure:"system_prompt"`
	} `mapstructure:"agent"`

	Redis struct {
		Enabled    bool   `mapstructure:"enabled"`
		Addr       string `mapstructure:"addr"`
		Password   string `mapstructure:"password"`
		DB         int    `mapstructure:"db"`
		KeyPrefix  string `mapstructure:"key_prefix"`
		TTLSeconds int    `mapstructure:"ttl_seconds"`
	} `mapstructure:"redis"`

	Services struct {
		Project string `mapstructure:"project"`
		Script  string `mapstructure:"script"`
		Image   string `mapstructure:"image"`
		Video   string `mapstructure:"video"`
		Dubbing string `mapstructure:"dubbing"`
		Task    string `mapstructure:"task"`
	} `mapstructure:"services"`
}

func Load() (*Config, error) {
	viper.SetConfigType("yaml")
	viper.SetEnvPrefix("AGENT_SERVICE")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	viper.SetDefault("http.port", 8012)
	viper.SetDefault("gateway.addr", "http://localhost:8000")
	viper.SetDefault("gateway.self_addr", "")
	viper.SetDefault("agent.default_model", "gpt-4.1-mini")
	viper.SetDefault("agent.system_prompt", "You are an orchestration agent for AI video production. Build structured executable plans using available tools.")
	viper.SetDefault("redis.enabled", false)
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.key_prefix", "agent:execution:")
	viper.SetDefault("redis.ttl_seconds", 172800)
	viper.SetDefault("services.project", "http://localhost:8007")
	viper.SetDefault("services.script", "http://localhost:8003")
	viper.SetDefault("services.image", "http://localhost:8005")
	viper.SetDefault("services.video", "http://localhost:8006")
	viper.SetDefault("services.dubbing", "http://localhost:8006")
	viper.SetDefault("services.task", "http://localhost:8008")

	if configFile := os.Getenv("AUTOVIDEO_CONFIG_FILE"); configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.SetConfigName("config")
		viper.AddConfigPath("../../")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")
		viper.AddConfigPath("/etc/agent-service")
	}
	_ = viper.ReadInConfig()
	if sub := viper.Sub("agent-service"); sub != nil {
		viper.MergeConfigMap(sub.AllSettings())
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
