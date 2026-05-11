// 本文件的作用：读取并解析应用配置（YAML 文件 / 环境变量），
// 将配置映射到 Go 结构体中，供其他模块使用。
// 使用了第三方库 viper 来完成配置管理。

// package —— 声明当前文件属于哪个包。同一目录下的所有 .go 文件必须用同一个包名。
package config

// import —— 导入需要使用的包。
// 标准库（如 "log"）和第三方库（如 viper）用空行分隔，这是 Go 的惯例。
import (
	"log"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config is the top-level configuration.
type Config struct {
	Server   ServerConfig   // 字段名大写开头 → 公开（exported），包外可访问
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Crypto   CryptoConfig
	GRPC     GRPCConfig
	Gateway  GatewayConfig
}

// struct tag（结构体标签）—— 反引号 `` 中的内容，如 `mapstructure:"port"`。
// 它是元数据，告诉 viper 解析 YAML 时用 "port" 这个键来填充 Port 字段。
type ServerConfig struct {
	Port string `mapstructure:"port"` // string 是 Go 的字符串类型
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	DSN string `mapstructure:"dsn"` // DSN = 数据库连接字符串
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"` // int 是整数类型
}

type JWTConfig struct {
	AccessSecret  string `mapstructure:"access_secret"`
	RefreshSecret string `mapstructure:"refresh_secret"`
	AccessTTL     int    `mapstructure:"access_ttl"`  // minutes
	RefreshTTL    int    `mapstructure:"refresh_ttl"` // days
}

type CryptoConfig struct {
	EncryptionKey string `mapstructure:"encryption_key"` // 32 bytes hex
}

type GRPCConfig struct {
	Port string `mapstructure:"port"`
}

type GatewayConfig struct {
	Addr     string `mapstructure:"addr"`
	SelfAddr string `mapstructure:"self_addr"`
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

// func Load() *Config —— 函数签名解读：
//   - func     : 关键字，声明一个函数
//   - Load     : 函数名，大写开头 → 公开，包外可以调用
//   - ()       : 无参数
//   - *Config  : 返回值类型。* 表示"指针"，即返回的是 Config 的内存地址，
//                调用者通过指针可以直接访问原始数据，避免拷贝整个结构体。
func Load() *Config {
	viper.SetConfigType("yaml")
	viper.AutomaticEnv() // 自动将环境变量绑定到配置

	// SetDefault —— 为每个配置项设置默认值，如果 YAML 和环境变量都没有提供，就用这些值。
	viper.SetDefault("server.port", "8001")
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("database.dsn", "host=localhost user=postgres password=postgres dbname=auth_db port=5432 sslmode=disable TimeZone=Asia/Shanghai")
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("jwt.access_ttl", 15)
	viper.SetDefault("jwt.refresh_ttl", 7)
	viper.SetDefault("jwt.access_secret", "autovideo-access-secret-change-in-prod")
	viper.SetDefault("jwt.refresh_secret", "autovideo-refresh-secret-change-in-prod")
	viper.SetDefault("crypto.encryption_key", "0123456789abcdef0123456789abcdef")
	viper.SetDefault("grpc.port", "9001")
	viper.SetDefault("gateway.addr", "http://localhost:8000")
	viper.SetDefault("gateway.self_addr", "")

	if configFile := os.Getenv("AUTOVIDEO_CONFIG_FILE"); configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		// Load unified config from project root
		viper.SetConfigName("config")
		viper.AddConfigPath("../../")
		viper.AddConfigPath(".")
	}

	// if err := ...; err != nil —— Go 最经典的错误处理模式：
	//   1. := 是"短变量声明"，自动推断类型并赋值（只能在函数内使用）
	//   2. Go 没有 try-catch，函数通过返回 error 值来报告错误
	//   3. err != nil 表示"出错了"，nil 相当于其他语言的 null
	if err := viper.ReadInConfig(); err != nil {
		// log.Printf —— 打印日志但不终止程序（对比下面的 log.Fatalf 会终止）
		log.Printf("config file not found, using defaults/env: %v", err)
	}
	if err := mergeOverrideConfig(); err != nil {
		log.Printf("failed to merge override config: %v", err)
	}

	// Merge service-specific section on top of shared values
	// := 再次出现：sub 的类型由右侧返回值自动推断
	if sub := viper.Sub("auth-service"); sub != nil {
		viper.MergeConfigMap(sub.AllSettings())
	}

	// var cfg Config —— 用 var 关键字声明变量 cfg，类型为 Config。
	// var 声明会自动将变量初始化为"零值"（结构体的零值是所有字段都为零值）。
	var cfg Config

	// &cfg —— & 是"取地址"运算符，把 cfg 的内存地址传给 Unmarshal，
	// 这样 Unmarshal 可以直接修改 cfg 的内容（而不是修改副本）。
	if err := viper.Unmarshal(&cfg); err != nil {
		// log.Fatalf —— 打印错误日志后立刻终止程序（Fatal = 致命）
		log.Fatalf("failed to unmarshal config: %v", err)
	}

	// return &cfg —— 返回 cfg 的指针。调用者拿到 *Config 就能访问解析好的配置。
	return &cfg
}
