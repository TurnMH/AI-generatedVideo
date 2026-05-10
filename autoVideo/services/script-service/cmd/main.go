package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/autovideo/script-service/internal/handler"
	"github.com/autovideo/script-service/internal/model"
	"github.com/autovideo/script-service/internal/repository"
	"github.com/autovideo/script-service/internal/seed"
	"github.com/autovideo/script-service/internal/service"
	"github.com/autovideo/script-service/pkg/config"
	"github.com/autovideo/script-service/pkg/middleware"
	"github.com/autovideo/script-service/pkg/registry"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// main —— 剧本服务入口，初始化数据库、LLM 客户端、Kafka 消费者和 HTTP 路由并启动服务
func main() {
	// Logger
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync() //nolint:errcheck

	// Config
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}
	if err := applyRuntimeConfig(cfg); err != nil {
		logger.Fatal("failed to load runtime api keys from auth-service", zap.Error(err))
	}

	// DB
	db, err := gorm.Open(postgres.Open(cfg.DB.DSN), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		logger.Fatal("failed to connect db", zap.Error(err))
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	if err := db.AutoMigrate(
		&model.Script{},
		&model.Scene{},
		&model.SplitConfig{},
		&model.CharacterExtracted{},
		&model.ScriptAsset{},
		&model.ScriptLibrary{},
		&model.PromptTemplate{},
	); err != nil {
		logger.Fatal("failed to migrate db", zap.Error(err))
	}

	// Seed default prompt templates (no-op if table already has data)
	if err := seed.SeedPromptTemplates(db); err != nil {
		logger.Warn("failed to seed prompt templates", zap.Error(err))
	}

	// Repositories
	scriptRepo := repository.NewScriptRepository(db)
	sceneRepo := repository.NewSceneRepository(db)
	characterRepo := repository.NewCharacterRepository(db)
	assetRepo := repository.NewAssetRepository(db)
	splitConfigRepo := repository.NewSplitConfigRepository(db)
	libraryRepo := repository.NewScriptLibraryRepository(db)
	promptTemplateRepo := repository.NewPromptTemplateRepository(db)

	// LLM Client — 使用权重自动切换 fallback 链：OpenAI → Claude → Qwen → Zhipu
	llmClient := service.NewFallbackLLMClient(cfg)

	// Kafka Service
	kafkaSvc := service.NewKafkaService(cfg, logger)

	// Script Service
	scriptSvc := service.NewScriptService(
		scriptRepo, sceneRepo, characterRepo, assetRepo, splitConfigRepo,
		llmClient, kafkaSvc, logger, cfg.ImageService.BaseURL,
		cfg.ImageService.DefaultModel, cfg.CharacterService.URL, cfg.JWT.Secret,
	)

	// Version Service
	versionSvc := service.NewVersionService(scriptRepo, logger)

	// Config hot-reload watcher (Feature opt-p6)
	configPath := os.Getenv("AUTOVIDEO_CONFIG_FILE")
	if configPath == "" {
		configPath = "../../config.local.yaml"
	}
	config.StartWatcher(configPath, func(newCfg *config.Config) {
		logger.Info("config file changed, reloading LLM settings",
			zap.String("base_url", newCfg.LLM.OpenAI.BaseURL),
		)
		llmClient.UpdateConfig(newCfg.LLM.OpenAI.APIKey, newCfg.LLM.OpenAI.BaseURL)
	})

	// Start Kafka consumers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	kafkaSvc.StartConsumer(ctx, scriptSvc)
	kafkaSvc.StartQuickGenerateConsumer(ctx, llmClient)

	// Handlers
	scriptHandler := handler.NewScriptHandler(scriptSvc)
	sceneHandler := handler.NewSceneHandler(scriptSvc)
	versionHandler := handler.NewVersionHandler(versionSvc)
	splitHandler := handler.NewSplitHandler(scriptSvc)
	libraryHandler := handler.NewScriptLibraryHandler(libraryRepo, llmClient)
	promptTemplateHandler := handler.NewPromptTemplateHandler(promptTemplateRepo)

	// Router
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(middleware.Logger(logger))
	r.Use(middleware.CORS(cfg.AllowedOrigins...))
	r.Use(gin.Recovery())

	v1 := r.Group("/api/v1")
	scripts := v1.Group("/scripts")
	scripts.Use(middleware.Auth(cfg.JWT.Secret))
	{
		scripts.GET("", scriptHandler.List)
		scripts.POST("/upload", scriptHandler.Upload)
		scripts.GET("/:id", scriptHandler.GetByID)
		scripts.DELETE("/:id", scriptHandler.Delete)
		scripts.POST("/:id/analyze", scriptHandler.Analyze)
		scripts.GET("/:id/scenes", sceneHandler.GetScenes)
		scripts.PUT("/:id/scenes/:sid", sceneHandler.UpdateScene)
		scripts.GET("/:id/characters", scriptHandler.GetCharacters)
		scripts.GET("/:id/assets", scriptHandler.GetAssets)

		// Version management
		scripts.GET("/:id/versions", versionHandler.ListVersions)
		scripts.POST("/:id/versions", versionHandler.CreateVersion)
		scripts.POST("/:id/versions/:vid/switch", versionHandler.SwitchVersion)

		// Split configuration
		scripts.GET("/:id/split-config", splitHandler.GetSplitConfig)
		scripts.PUT("/:id/split-config", splitHandler.UpdateSplitConfig)
		scripts.POST("/:id/re-split", splitHandler.ReSplit)
	}

	// Scene-level routes (scenes addressed by their own ID)
	scenes := v1.Group("/scenes")
	scenes.Use(middleware.Auth(cfg.JWT.Secret))
	{
		scenes.POST("/:id/generate-image", sceneHandler.GenerateImage)
		scenes.POST("/batch-generate", sceneHandler.BatchGenerate)
	}

	// Script Library routes
	library := v1.Group("/script-library")
	library.Use(middleware.Auth(cfg.JWT.Secret))
	{
		library.GET("", libraryHandler.List)
		library.POST("", libraryHandler.Create)
		library.POST("/generate", libraryHandler.GenerateAI)
		library.PUT("/:id", libraryHandler.Update)
		library.DELETE("/:id", libraryHandler.Delete)
	}

	// Prompt Template routes
	prompts := v1.Group("/prompt-templates")
	prompts.Use(middleware.Auth(cfg.JWT.Secret))
	{
		prompts.GET("", promptTemplateHandler.List)
		prompts.POST("", promptTemplateHandler.Create)
		prompts.POST("/reseed", func(c *gin.Context) {
			if err := seed.ForceReseedPromptTemplates(db); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})
		prompts.GET("/:id", promptTemplateHandler.Get)
		prompts.PUT("/:id", promptTemplateHandler.Update)
		prompts.DELETE("/:id", promptTemplateHandler.Delete)
		prompts.POST("/:id/preview", promptTemplateHandler.Preview)
	}

	// Health check (no auth)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Project-level runtime data cleanup (called by project-service on project delete)
	projects := v1.Group("/projects/:pid")
	projects.DELETE("/runtime-data", func(c *gin.Context) {
		pidStr := c.Param("pid")
		pid, err := strconv.ParseInt(pidStr, 10, 64)
		if err != nil || pid <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
			return
		}
		ctx := context.Background()
		// Find all script IDs for this project
		scriptIDs, err := scriptRepo.FindIDsByProjectID(ctx, pid)
		if err != nil {
			logger.Error("find script IDs failed", zap.Int64("project_id", pid), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// Delete child records for each script
		for _, sid := range scriptIDs {
			if err := sceneRepo.DeleteByScriptID(ctx, sid); err != nil {
				logger.Error("delete scenes failed", zap.Int64("script_id", sid), zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if err := characterRepo.DeleteByScriptID(ctx, sid); err != nil {
				logger.Error("delete characters failed", zap.Int64("script_id", sid), zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if err := assetRepo.DeleteByScriptID(ctx, sid); err != nil {
				logger.Error("delete assets failed", zap.Int64("script_id", sid), zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if err := splitConfigRepo.DeleteByScriptID(ctx, sid); err != nil {
				logger.Error("delete split config failed", zap.Int64("script_id", sid), zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		// Delete all scripts for the project
		if err := scriptRepo.DeleteByProjectID(ctx, pid); err != nil {
			logger.Error("delete scripts failed", zap.Int64("project_id", pid), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		logger.Info("project script data deleted", zap.Int64("project_id", pid), zap.Int("scripts", len(scriptIDs)))
		c.JSON(http.StatusOK, gin.H{"deleted": true, "scripts": len(scriptIDs)})
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler: r,
	}

	go func() {
		logger.Info("script-service starting", zap.Int("port", cfg.HTTP.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server failed", zap.Error(err))
		}
	}()

	// Register with gateway for service discovery (reuse kafka ctx which is cancelled on shutdown)
	{
		selfAddr := cfg.Gateway.SelfAddr
		if selfAddr == "" {
			selfAddr = fmt.Sprintf("http://localhost:%d", cfg.HTTP.Port)
		}
		registry.Start(ctx, cfg.Gateway.Addr, "script", selfAddr)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	cancel() // stop kafka consumer

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced to shutdown", zap.Error(err))
	}
	logger.Info("server exited")
}
