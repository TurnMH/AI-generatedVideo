package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/autovideo/agent-service/internal/handler"
	"github.com/autovideo/agent-service/internal/service"
	"github.com/autovideo/agent-service/pkg/config"
	"github.com/autovideo/agent-service/pkg/registry"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("load config", zap.Error(err))
	}

	tools := service.NewToolRegistry(
		service.NewProjectGetTool(cfg.Services.Project),
		service.NewScriptGenerateTool(cfg.Services.Script),
		service.NewShotPlanGenerateTool(),
		service.NewImageGenerateBatchTool(cfg.Services.Image),
		service.NewVideoGenerateBatchTool(cfg.Services.Video),
		service.NewDubbingGenerateTool(cfg.Services.Dubbing),
		service.NewTaskGetStatusTool(cfg.Services.Task),
	)
	executionStore := service.NewExecutionStore()
	if cfg.Redis.Enabled {
		rdb := redis.NewClient(&redis.Options{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})
		if err := rdb.Ping(context.Background()).Err(); err != nil {
			logger.Warn("redis ping failed, using in-memory execution store", zap.Error(err))
		} else {
			executionStore = service.NewRedisExecutionStore(rdb, cfg.Redis.KeyPrefix, time.Duration(cfg.Redis.TTLSeconds)*time.Second)
			logger.Info("agent-service using redis execution store", zap.String("addr", cfg.Redis.Addr))
		}
	}

	agentSvc := service.NewAgentService(logger, cfg.Agent.DefaultModel, cfg.Agent.SystemPrompt, tools, executionStore)
	h := handler.NewAgentHandler(agentSvc)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/health", h.Health)
	r.GET("/healthz", h.Health)

	api := r.Group("/api/v1/agent")
	{
		api.GET("/tools", h.ListTools)
		api.POST("/plans", h.BuildPlan)
		api.POST("/plans/execute", h.ExecutePlan)
		api.GET("/executions/:id", h.GetExecution)
		api.POST("/executions/:id/retry", h.RetryExecution)
		api.POST("/executions/:id/resume", h.ResumeExecution)
		api.POST("/executions/:id/replay-from/:stepId", h.ReplayFromStep)
		api.GET("/plans/:id/executions", h.ListExecutions)
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	selfAddr := cfg.Gateway.SelfAddr
	if selfAddr == "" {
		selfAddr = fmt.Sprintf("http://localhost:%d", cfg.HTTP.Port)
	}
	registry.Start(ctx, cfg.Gateway.Addr, "agent", selfAddr)

	go func() {
		logger.Info("agent-service listening", zap.Int("port", cfg.HTTP.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("listen", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("agent-service shutting down")
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown", zap.Error(err))
	}
}
