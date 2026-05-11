// 【小白注解】本文件是 auth-service 的启动入口。
// 它完成以下工作：加载配置 → 连接数据库和 Redis → 注册路由 → 启动 HTTP/gRPC 服务 → 监听退出信号优雅关闭。
// 整个程序的生命周期都在 main() 函数里管理。

// 【小白注解】package main —— Go 程序的入口必须放在 package main 里，
// 编译器会从这个包中的 main() 函数开始执行。
package main

// 【小白注解】import (...) 是"分组导入"写法，用圆括号把多个包一起导入。
// 习惯上分三组：标准库 → 第三方库 → 本项目内部包，用空行隔开。
import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	grpcauth "github.com/autovideo/auth-service/internal/grpc"
	"github.com/autovideo/auth-service/internal/handler"
	"github.com/autovideo/auth-service/internal/model"
	"github.com/autovideo/auth-service/internal/repository"
	"github.com/autovideo/auth-service/internal/service"
	grpcjson "github.com/autovideo/auth-service/pkg/codec"
	"github.com/autovideo/auth-service/pkg/config"
	"github.com/autovideo/auth-service/pkg/middleware"
	"github.com/autovideo/auth-service/pkg/registry"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 【小白注解】func main() —— Go 程序的入口函数，程序运行时自动调用，不需要手动调用。
func main() {
	// 加载配置：优先读 config.local.yaml，其次环境变量，最后回落默认值（定义在 pkg/config）。
	// 【小白注解】:= 是"短变量声明"，自动推断类型并赋值，只能在函数内部使用。
	cfg := config.Load()

	// 初始化 zap logger
	// 【小白注解】var 是显式声明变量，需要指定类型。适合先声明、后赋值的场景。
	var zapLogger *zap.Logger
	var err error
	if cfg.Server.Mode == "release" {
		zapLogger, err = zap.NewProduction()
	} else {
		zapLogger, err = zap.NewDevelopment()
	}
	if err != nil {
		log.Fatalf("failed to init logger: %v", err)
	}
	// 【小白注解】defer 会把函数调用推迟到当前函数（main）返回前才执行，
	// 常用于资源清理（关闭文件、刷新日志等），无论中途是否出错都会执行。
	defer zapLogger.Sync() //nolint:errcheck

	// 初始化 PostgreSQL
	// 【小白注解】&gorm.Config{} —— & 取地址，得到指针；{} 是结构体字面量初始化。
	gormCfg := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	}
	db, err := gorm.Open(postgres.Open(cfg.Database.DSN), gormCfg)
	if err != nil {
		zapLogger.Fatal("failed to connect database", zap.Error(err))
	}

	// 自动迁移表结构
	if err := db.AutoMigrate(
		&model.User{},
		&model.OAuthAccount{},
		&model.RefreshToken{},
		&model.UserAPIKey{},
		&model.SystemAPIKey{},
		&model.Permission{},
		&model.RolePermission{},
	); err != nil {
		zapLogger.Fatal("auto migrate failed", zap.Error(err))
	}

	// 初始化 Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	// 【小白注解】context.WithTimeout 创建一个带超时的上下文（context）。
	// context 是 Go 中管理请求生命周期的核心机制，超时/取消会自动传播给下游调用。
	// cancel 是配套的取消函数，必须调用以释放资源，所以紧跟 defer cancel()。
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		zapLogger.Warn("redis not available, rate limiting disabled", zap.Error(err))
	}

	// 初始化 repositories
	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewTokenRepository(db)
	apiKeyRepo := repository.NewAPIKeyRepository(db)
	sysAPIKeyRepo := repository.NewSystemAPIKeyRepository(db)

	// 初始化 services
	authSvc := service.NewAuthService(userRepo, tokenRepo, cfg)
	apiKeySvc := service.NewAPIKeyService(apiKeyRepo, sysAPIKeyRepo, cfg)
	configFile := os.Getenv("AUTOVIDEO_CONFIG_FILE")
	if configFile == "" {
		configFile = "../../config.local.yaml"
	}
	if err := apiKeySvc.SyncRuntimeAPIKeys(configFile); err != nil {
		zapLogger.Warn("sync runtime api keys failed", zap.String("config_file", configFile), zap.Error(err))
	} else {
		zapLogger.Info("runtime api keys synced from config", zap.String("config_file", configFile))
	}

	// 初始化 handlers
	authHandler := handler.NewAuthHandler(authSvc)
	apiKeyHandler := handler.NewAPIKeyHandler(apiKeySvc)

	// 设置 Gin 模式
	gin.SetMode(cfg.Server.Mode)

	// 设置 Gin 路由
	r := gin.New()
	r.Use(middleware.Logger(zapLogger))
	r.Use(middleware.RateLimit(rdb, 100, time.Minute))
	r.Use(gin.Recovery())

	// 健康检查
	// 【小白注解】func(c *gin.Context) { ... } 是匿名函数（也叫闭包），
	// 直接作为参数传递，不需要单独命名。
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "auth-service"})
	})

	v1 := r.Group("/api/v1")
	auth := v1.Group("/auth")
	{
		// 公开接口
		auth.POST("/register", authHandler.RegisterHandler)
		auth.POST("/login/password", authHandler.LoginPasswordHandler)
		auth.POST("/token/refresh", authHandler.RefreshHandler)

		// 需要认证的接口
		protected := auth.Group("/")
		protected.Use(middleware.AuthRequired(cfg.JWT.AccessSecret))
		{
			protected.POST("/logout", authHandler.LogoutHandler)
			protected.GET("/me", authHandler.MeHandler)
			protected.PUT("/me", authHandler.UpdateMeHandler)

			// API Key 管理
			protected.GET("/api-keys", apiKeyHandler.ListAPIKeysHandler)
			protected.POST("/api-keys", apiKeyHandler.AddAPIKeyHandler)
			protected.DELETE("/api-keys/:id", apiKeyHandler.DeleteAPIKeyHandler)
			protected.GET("/system-api-keys", apiKeyHandler.ListSystemAPIKeysHandler)
		}
	}

	internal := r.Group("/internal")
	internal.Use(middleware.InternalAuth(cfg.JWT.AccessSecret))
	{
		internal.GET("/runtime-api-keys", apiKeyHandler.ListRuntimeAPIKeysHandler)
	}

	// 启动 gRPC server（goroutine）
	// 【小白注解】go func() { ... }() —— go 关键字启动一个 goroutine（轻量级并发线程）。
	// 这里把 gRPC 服务放到后台运行，不会阻塞后续代码。
	go func() {
		if err := startGRPCServer(cfg.GRPC.Port, authSvc, cfg.JWT.AccessSecret, zapLogger); err != nil {
			zapLogger.Error("gRPC server error", zap.Error(err))
		}
	}()

	go watchRuntimeKeySync(apiKeySvc, configFile, zapLogger)

	// 启动 HTTP server
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 【小白注解】再启一个 goroutine 跑 HTTP 服务，这样 main 函数可以继续往下走去监听退出信号。
	go func() {
		zapLogger.Info("HTTP server starting", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Register with gateway for service discovery
	{
		rootCtx, rootCancel := context.WithCancel(context.Background())
		selfAddr := cfg.Gateway.SelfAddr
		if selfAddr == "" {
			selfAddr = "http://localhost:" + cfg.Server.Port
		}
		registry.Start(rootCtx, cfg.Gateway.Addr, "auth", selfAddr)
		defer rootCancel()
	}

	// 等待终止信号，优雅关闭
	// 【小白注解】make(chan os.Signal, 1) —— make 是 Go 内置函数，用来创建 channel、slice、map。
	// chan 是通道，goroutine 之间通信的管道。这里创建了一个缓冲为 1 的信号通道。
	quit := make(chan os.Signal, 1)
	// 【小白注解】signal.Notify 让操作系统的 SIGINT（Ctrl+C）和 SIGTERM 信号发送到 quit 通道。
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	// 【小白注解】<-quit 从通道接收数据，如果通道为空会阻塞等待。
	// 程序会停在这里，直到收到终止信号才继续往下执行关闭逻辑。
	<-quit

	zapLogger.Info("Shutting down server...")
	// 【小白注解】再次创建带超时的 context，给服务器 5 秒时间处理完正在进行的请求后再关闭。
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		zapLogger.Fatal("Server forced to shutdown", zap.Error(err))
	}
	zapLogger.Info("Server exited")
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

func watchRuntimeKeySync(apiKeySvc service.APIKeyService, configFile string, logger *zap.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	lastMod := map[string]time.Time{}
	for _, candidate := range watchedConfigPaths(configFile) {
		if info, err := os.Stat(candidate); err == nil {
			lastMod[candidate] = info.ModTime()
		}
	}
	for range ticker.C {
		changed := false
		for _, candidate := range watchedConfigPaths(configFile) {
			info, err := os.Stat(candidate)
			if err != nil {
				continue
			}
			if !info.ModTime().After(lastMod[candidate]) {
				continue
			}
			lastMod[candidate] = info.ModTime()
			changed = true
		}
		if !changed {
			continue
		}
		if err := apiKeySvc.SyncRuntimeAPIKeys(configFile); err != nil {
			logger.Warn("sync runtime api keys on config change failed", zap.String("config_file", configFile), zap.Error(err))
			continue
		}
		logger.Info("runtime api keys resynced from config change", zap.String("config_file", configFile))
	}
}

// startGRPCServer 启动 gRPC 服务（目前注册空实现，可在后续扩展）
func startGRPCServer(port string, authSvc service.AuthService, accessSecret string, zapLogger *zap.Logger) error {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}
	s := grpc.NewServer(grpc.ForceServerCodec(grpcjson.JSONCodec{}))
	grpcauth.RegisterAuthServiceServer(s, grpcauth.NewHandler(authSvc, accessSecret))
	zapLogger.Info("gRPC server starting", zap.String("addr", ":"+port))
	return s.Serve(lis)
}
