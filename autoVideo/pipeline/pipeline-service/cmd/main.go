package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/autovideo/pipeline-service/internal/handler"
	"github.com/autovideo/pipeline-service/internal/service"
	"github.com/autovideo/pipeline-service/pkg/config"
	"github.com/autovideo/pipeline-service/pkg/middleware"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// main —— 程序入口，初始化日志、配置、Redis、Kafka 监听与 HTTP 服务并优雅退出
func main() {
	// ─── 日志 ─────────────────────────────────────────────────────────────────
	logger, err := zap.NewProduction()
	if err != nil {
		panic("failed to init logger: " + err.Error())
	}
	defer logger.Sync()

	// ─── 配置 ─────────────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	// ─── Redis ────────────────────────────────────────────────────────────────
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Warn("redis ping failed, continuing anyway", zap.Error(err))
	}
	cancel()

	// ─── 服务层 ───────────────────────────────────────────────────────────────
	httpClient := service.NewHTTPClient(
		cfg.Services.ScriptBase,
		cfg.Services.CharacterBase,
		cfg.Services.ImageBase,
		cfg.Services.VideoBase,
		cfg.InternalToken,
	)

	pipelineSvc := service.NewPipelineService(rdb, httpClient, logger)

	// ─── Kafka 监听 ───────────────────────────────────────────────────────────
	kafkaListener := service.NewKafkaListener(
		cfg.Kafka.Brokers,
		cfg.Kafka.GroupID,
		"pipeline-events",
		pipelineSvc,
		logger,
	)

	bgCtx, bgCancel := context.WithCancel(context.Background())
	go kafkaListener.Run(bgCtx)

	// ─── HTTP 路由 ────────────────────────────────────────────────────────────
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.Logger(logger))

	pipelineHandler := handler.NewPipelineHandler(pipelineSvc, logger)

	v1 := router.Group("/api/v1")
	{
		// 健康检查（无需鉴权）
		v1.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "pipeline-service"})
		})

		pipeline := v1.Group("/pipeline")
		pipeline.Use(middleware.Auth(cfg.JWT.Secret))
		{
			pipeline.POST("/start", pipelineHandler.StartPipeline)
			pipeline.GET("/:pipeline_id/status", pipelineHandler.GetStatus)
			pipeline.POST("/:pipeline_id/pause", pipelineHandler.PausePipeline)
			pipeline.POST("/:pipeline_id/resume", pipelineHandler.ResumePipeline)
			pipeline.POST("/:pipeline_id/abort", pipelineHandler.AbortPipeline)
		}
	}

	// ─── 启动 HTTP Server ─────────────────────────────────────────────────────
	addr := fmt.Sprintf(":%d", cfg.HTTP.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logger.Info("pipeline-service starting", zap.String("addr", addr))

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("listen failed", zap.Error(err))
		}
	}()

	// ─── 优雅退出 ─────────────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down pipeline-service...")

	bgCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	}

	logger.Info("pipeline-service stopped")
}
