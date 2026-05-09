package config

import (
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all runtime configuration.
type Config struct {
	HTTP     HTTPConfig
	GRPC     GRPCConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Log      LogConfig
	Gateway  GatewayConfig
}

type GatewayConfig struct {
	Addr     string
	SelfAddr string
}

type HTTPConfig struct {
	Port string
}

type GRPCConfig struct {
	Port string
}

type DatabaseConfig struct {
	DSN string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type LogConfig struct {
	Level string
}

// Load —— 从环境变量和配置文件读取运行时配置，返回 Config 结构体
// Load reads configuration from environment variables and an optional config file.
func Load() (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("http.port", "8008")
	v.SetDefault("grpc.port", "9008")
	v.SetDefault("db.dsn", "host=localhost user=postgres password=postgres dbname=model_db port=5432 sslmode=disable TimeZone=Asia/Shanghai")
	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("log.level", "info")
	v.SetDefault("gateway.addr", "http://localhost:8000")
	v.SetDefault("gateway.self_addr", "")

	// Environment variables (e.g. MODEL_HTTP_PORT, MODEL_DB_DSN)
	v.SetEnvPrefix("MODEL")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetConfigType("yaml")

	if configFile := os.Getenv("AUTOVIDEO_CONFIG_FILE"); configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		// Load unified config from project root
		v.SetConfigName("config")
		v.AddConfigPath("../../")
		v.AddConfigPath(".")
		v.AddConfigPath("/etc/model-service/")
	}

	_ = v.ReadInConfig()

	// Merge service-specific section on top of shared values
	if sub := v.Sub("model-service"); sub != nil {
		v.MergeConfigMap(sub.AllSettings())
	}

	cfg := &Config{
		HTTP: HTTPConfig{
			Port: v.GetString("http.port"),
		},
		GRPC: GRPCConfig{
			Port: v.GetString("grpc.port"),
		},
		Database: DatabaseConfig{
			DSN: v.GetString("db.dsn"),
		},
		Redis: RedisConfig{
			Addr:     v.GetString("redis.addr"),
			Password: v.GetString("redis.password"),
			DB:       v.GetInt("redis.db"),
		},
		Log: LogConfig{
			Level: v.GetString("log.level"),
		},
		Gateway: GatewayConfig{
			Addr:     v.GetString("gateway.addr"),
			SelfAddr: v.GetString("gateway.self_addr"),
		},
	}
	return cfg, nil
}
