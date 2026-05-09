package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	HTTP struct {
		Port int `mapstructure:"port"`
	} `mapstructure:"http"`
	Redis struct {
		Addr     string `mapstructure:"addr"`
		Password string `mapstructure:"password"`
		DB       int    `mapstructure:"db"`
	} `mapstructure:"redis"`
	JWT struct {
		Secret string `mapstructure:"secret"`
	} `mapstructure:"jwt"`
	InternalToken string `mapstructure:"internal_token"`
	Services      struct {
		ScriptBase    string `mapstructure:"script_base"`
		CharacterBase string `mapstructure:"character_base"`
		ImageBase     string `mapstructure:"image_base"`
		VideoBase     string `mapstructure:"video_base"`
	} `mapstructure:"services"`
	Kafka struct {
		Brokers []string `mapstructure:"brokers"`
		GroupID string   `mapstructure:"group_id"`
	} `mapstructure:"kafka"`
}

// Load —— 加载配置文件和环境变量，返回解析后的 Config 结构体
func Load() (*Config, error) {
	v := viper.New()

	// 默认值
	v.SetDefault("http.port", 8010)
	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.db", 0)
	v.SetDefault("services.script_base", "http://localhost:8003")
	v.SetDefault("services.character_base", "http://localhost:8004")
	v.SetDefault("services.image_base", "http://localhost:8005")
	v.SetDefault("services.video_base", "http://localhost:8006")
	v.SetDefault("kafka.brokers", []string{"localhost:9092"})
	v.SetDefault("kafka.group_id", "pipeline-service")
	v.SetDefault("internal_token", "internal-secret")

	v.SetConfigType("yaml")

	// Load unified config from project root
	v.SetConfigName("config")
	v.AddConfigPath("../../")
	v.AddConfigPath(".")
	v.AddConfigPath("./configs")
	v.AddConfigPath("/app/configs")

	// 支持环境变量覆盖，如 PIPELINE_HTTP_PORT
	v.SetEnvPrefix("PIPELINE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	_ = v.ReadInConfig()

	// Merge service-specific section on top of shared values
	if sub := v.Sub("pipeline-service"); sub != nil {
		v.MergeConfigMap(sub.AllSettings())
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
