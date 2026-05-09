package config

import (
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	Server   ServerConfig   `mapstructure:"http"`
	Database DatabaseConfig `mapstructure:"db"`
	Storage  StorageConfig  `mapstructure:"storage"`
	GRPC     GRPCConfig     `mapstructure:"grpc"`
	Gateway  struct {
		Addr     string `mapstructure:"addr"`
		SelfAddr string `mapstructure:"self_addr"`
	} `mapstructure:"gateway"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	DSN string `mapstructure:"dsn"`
}

type StorageConfig struct {
	Driver           string      `mapstructure:"driver"`
	Minio            MinioConfig `mapstructure:"minio"`
	Local            LocalConfig `mapstructure:"local"`
	CdnBaseURL       string      `mapstructure:"cdn_base_url"`
	CDNIncludeBucket bool        `mapstructure:"cdn_include_bucket"` // true=本地MinIO; false=生产CDN(桶名在CDN层绑定)
	Buckets          Buckets     `mapstructure:"buckets"`
}

type MinioConfig struct {
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	UseSSL    bool   `mapstructure:"use_ssl"`
	PathStyle bool   `mapstructure:"path_style"` // true = path-style for TOS/S3
}

type LocalConfig struct {
	BasePath string `mapstructure:"base_path"`
}

type Buckets struct {
	Images     string `mapstructure:"images"`
	Videos     string `mapstructure:"videos"`
	Scripts    string `mapstructure:"scripts"`
	Characters string `mapstructure:"characters"`
	Uploads    string `mapstructure:"uploads"`
	Exports    string `mapstructure:"exports"`
	Dubbing    string `mapstructure:"dubbing"` // 配音音频
	Audios     string `mapstructure:"audios"`  // 背景音乐
}

type GRPCConfig struct {
	Port string `mapstructure:"port"`
}

// Load —— 从配置文件和环境变量加载应用配置，返回 *Config
// Load reads configuration from file and environment variables.
func Load(cfgFile string) (*Config, error) {
	viper.SetConfigType("yaml")

	viper.SetEnvPrefix("STORAGE")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	setDefaults()

	if cfgFile == "" {
		cfgFile = os.Getenv("AUTOVIDEO_CONFIG_FILE")
	}

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
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

	// Merge service-specific section on top of shared values
	if sub := viper.Sub("storage-service"); sub != nil {
		viper.MergeConfigMap(sub.AllSettings())
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// setDefaults —— 为所有配置项设置默认值
func setDefaults() {
	viper.SetDefault("http.port", "8009")
	viper.SetDefault("http.mode", "debug")
	viper.SetDefault("db.dsn", "host=localhost user=postgres password=postgres dbname=storage_db port=5432 sslmode=disable")
	viper.SetDefault("storage.driver", "minio")
	viper.SetDefault("storage.minio.endpoint", "localhost:9000")
	viper.SetDefault("storage.minio.access_key", "minioadmin")
	viper.SetDefault("storage.minio.secret_key", "minioadmin")
	viper.SetDefault("storage.minio.use_ssl", false)
	viper.SetDefault("storage.local.base_path", "./local-storage")
	viper.SetDefault("storage.cdn_base_url", "http://localhost:9000")
	viper.SetDefault("storage.cdn_include_bucket", true) // default true for local MinIO compatibility
	viper.SetDefault("buckets.images", "images")
	viper.SetDefault("buckets.videos", "videos")
	viper.SetDefault("buckets.scripts", "scripts")
	viper.SetDefault("buckets.characters", "characters")
	viper.SetDefault("buckets.uploads", "uploads")
	viper.SetDefault("buckets.exports", "exports")
	viper.SetDefault("buckets.dubbing", "dubbing")
	viper.SetDefault("buckets.audios", "audios")
	viper.SetDefault("grpc.port", "9009")
	viper.SetDefault("gateway.addr", "http://localhost:8000")
	viper.SetDefault("gateway.self_addr", "")
}
