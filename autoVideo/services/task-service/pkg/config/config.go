package config

import (
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all service configuration.
type Config struct {
	Server   ServerConfig
	GRPC     GRPCConfig
	Database DatabaseConfig
	Kafka    KafkaConfig
	Gateway  GatewayConfig
	JWT      JWTConfig
}

// JWTConfig holds JWT validation settings.
type JWTConfig struct {
	AccessSecret string
}

type GatewayConfig struct {
	Addr     string
	SelfAddr string
}

type ServerConfig struct {
	Port string
}

type GRPCConfig struct {
	Port string
}

type DatabaseConfig struct {
	DSN string
}

type KafkaConfig struct {
	Brokers []string
}

// Load —— 从环境变量和配置文件读取运行时配置，返回 Config 结构体
// Load reads configuration from environment variables and config files,
// applying sensible defaults.
func Load() (*Config, error) {
	v := viper.New()

	// defaults
	v.SetDefault("http.port", "8007")
	v.SetDefault("grpc.port", "9007")
	v.SetDefault("db.dsn", "host=localhost user=postgres password=postgres dbname=task_db port=5432 sslmode=disable")
	v.SetDefault("kafka.brokers", []string{"localhost:9092"})
	v.SetDefault("gateway.addr", "http://localhost:8000")
	v.SetDefault("gateway.self_addr", "")
	v.SetDefault("jwt.secret", "autovideo-access-secret-change-in-prod")

	v.SetConfigType("yaml")

	if configFile := os.Getenv("AUTOVIDEO_CONFIG_FILE"); configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		// Load unified config from project root
		v.SetConfigName("config")
		v.AddConfigPath("../../")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
	}

	v.SetEnvPrefix("TASK")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	_ = v.ReadInConfig()

	// Merge service-specific section on top of shared values
	if sub := v.Sub("task-service"); sub != nil {
		v.MergeConfigMap(sub.AllSettings())
	}

	cfg := &Config{
		Server: ServerConfig{
			Port: v.GetString("http.port"),
		},
		GRPC: GRPCConfig{
			Port: v.GetString("grpc.port"),
		},
		Database: DatabaseConfig{
			DSN: v.GetString("db.dsn"),
		},
		Kafka: KafkaConfig{
			Brokers: v.GetStringSlice("kafka.brokers"),
		},
		Gateway: GatewayConfig{
			Addr:     v.GetString("gateway.addr"),
			SelfAddr: v.GetString("gateway.self_addr"),
		},
		JWT: JWTConfig{
			AccessSecret: v.GetString("jwt.secret"),
		},
	}
	return cfg, nil
}
