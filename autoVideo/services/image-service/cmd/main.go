package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/autovideo/image-service/internal/handler"
	"github.com/autovideo/image-service/internal/model"
	"github.com/autovideo/image-service/internal/repository"
	"github.com/autovideo/image-service/internal/service"
	"github.com/autovideo/image-service/internal/service/generators"
	"github.com/autovideo/image-service/pkg/config"
	"github.com/autovideo/image-service/pkg/middleware"
	"github.com/autovideo/image-service/pkg/registry"
	"github.com/autovideo/image-service/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// main —— 图片服务入口，初始化数据库、生成器、Kafka 消费者和 HTTP 路由并启动服务
func main() {
	cfg := config.Load()
	if err := applyRuntimeConfig(cfg); err != nil {
		panic(err)
	}

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	db, err := gorm.Open(postgres.Open(cfg.DB.DSN), &gorm.Config{})
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	// Tune connection pool for high-concurrency image task workloads.
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(50)
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetConnMaxLifetime(time.Hour)
		sqlDB.SetConnMaxIdleTime(10 * time.Minute)
	}
	if err := db.AutoMigrate(&model.ImageTask{}); err != nil {
		logger.Fatal("auto migrate image tasks", zap.Error(err))
	}

	imageRepo := repository.NewImageRepo(db)

	gens := map[string]generators.ImageGenerator{
		"sdxl":   generators.NewSDXLGenerator(cfg.Models.ComfyUIURLs, cfg.Models.ComfyUIWorkflow, logger),
		"flux":   generators.NewFluxGenerator(cfg.Models.ReplicateKeys, logger),
		"dalle":  generators.NewDalleGenerator(cfg.Models.OpenAIKeys, cfg.Models.OpenAIBase, logger),
		"tongyi": generators.NewTongyiGenerator(cfg.Models.TongyiKeys, cfg.Models.DashScopeBase, logger),
	}
	// Register additional OpenAI-compatible image models from config (openai_models list)
	for _, modelName := range cfg.Models.OpenAIModels {
		if modelName == "" {
			continue
		}
		gens[modelName] = generators.NewDalleGeneratorForModel(cfg.Models.OpenAIKeys, cfg.Models.OpenAIBase, modelName, modelName, logger)
	}
	// Ensure "gpt-image-1.5" is always registered under its exact name (alias to "dalle" generator).
	if _, exists := gens["gpt-image-1.5"]; !exists {
		if len(cfg.Models.OpenAIKeys) > 0 {
			gens["gpt-image-1.5"] = generators.NewDalleGeneratorForModel(cfg.Models.OpenAIKeys, cfg.Models.OpenAIBase, "gpt-image-1.5", "gpt-image-1.5", logger)
		}
	}
	// Also register "gpt-image-1" for newer proxies that use that name.
	if _, exists := gens["gpt-image-1"]; !exists {
		if len(cfg.Models.OpenAIKeys) > 0 {
			gens["gpt-image-1"] = generators.NewDalleGeneratorForModel(cfg.Models.OpenAIKeys, cfg.Models.OpenAIBase, "gpt-image-1", "gpt-image-1", logger)
		}
	}
	// Register additional DashScope/Tongyi image models from config (tongyi_models list)
	for _, modelName := range cfg.Models.TongyiModels {
		if modelName == "" || modelName == "wanx2.1-t2i-turbo" {
			continue // already registered as "tongyi"
		}
		gens[modelName] = generators.NewTongyiGeneratorForModel(cfg.Models.TongyiKeys, cfg.Models.DashScopeBase, modelName, modelName, logger)
	}
	// Register ZhipuAI CogView image generation models.
	// CogView uses the same OpenAI-compatible /images/generations endpoint so we
	// reuse DalleGeneratorForModel pointed at ZhipuAI's base URL.
	// Primary model "cogview-3-plus" is registered under both its full name and
	// the short alias "cogview" for backward-compatibility.
	if len(cfg.Models.ZhipuKeys) > 0 {
		zhipuBase := cfg.Models.ZhipuBase
		defaultCogView := "cogview-3-plus"
		gens["cogview"] = generators.NewDalleGeneratorForModel(cfg.Models.ZhipuKeys, zhipuBase, defaultCogView, "cogview", logger)
		for _, modelName := range cfg.Models.ZhipuModels {
			if modelName == "" {
				continue
			}
			gens[modelName] = generators.NewDalleGeneratorForModel(cfg.Models.ZhipuKeys, zhipuBase, modelName, modelName, logger)
		}
	}

	// Register Gemini image generation channels (香蕉2.0 / 香蕉2.1 / 星融2.5beta).
	// Each model is registered under its canonical name and common aliases.
	if len(cfg.Models.GeminiKeys) > 0 {
		bases := cfg.Models.GeminiBases

		// Primary registrations
		flashModel := cfg.Models.GeminiFlashModel // configurable via models.gemini_flash_model
		geminiPerChan := cfg.Concurrency.GeminiPerChannel
		gemini3Flash := generators.NewGeminiImageGenerator(
			bases, cfg.Models.GeminiKeys,
			flashModel, "gemini-3.1-flash-image", true, logger, geminiPerChan)
		gens["gemini-3.1-flash-image"] = gemini3Flash
		// Also register under the raw API model name so frontend selections match directly
		if flashModel != "" && flashModel != "gemini-3.1-flash-image" {
			gens[flashModel] = gemini3Flash
		}
		gens["banana2.1"] = gemini3Flash   // 香蕉2.1 alias
		gens["xingrong2.5"] = gemini3Flash // 星融2.5beta alias
		gens["nano-banana"] = gemini3Flash  // nano-banana → gemini-2.5-flash-image (ppai img_new has no channel for images/generations)

		gemini3Pro := generators.NewGeminiImageGenerator(
			bases, cfg.Models.GeminiKeys,
			"gemini-3-pro-image-preview", "gemini-3-pro-image", true, logger, geminiPerChan)
		gens["gemini-3-pro-image"] = gemini3Pro
		gens["gemini-3-pro-image-preview"] = gemini3Pro // DB model_key alias
		gens["banana2.0"] = gemini3Pro                  // 香蕉2.0 alias
		gens["gemini-2.5-flash-image"] = gemini3Flash   // DB model_key alias → 复用 flash 渠道

		// Extra model aliases from config
		for _, modelName := range cfg.Models.GeminiModels {
			if modelName == "" {
				continue
			}
			if _, exists := gens[modelName]; !exists {
				gens[modelName] = generators.NewGeminiImageGenerator(
					bases, cfg.Models.GeminiKeys,
					modelName, modelName, true, logger, geminiPerChan)
			}
		}
	}

	// Register Baidu BCE image channel (百度渠道融图) — async task-based.
	// ⚠️ GET task polling requires BCE-AUTH-V1 HMAC; bce-v3 Bearer token is NOT accepted.
	// baidu_image_ak / baidu_image_sk = standard IAM AK/SK for HMAC signing.
	if cfg.Models.BaiduImageKey != "" {
		baiduModel := cfg.Models.BaiduImageModel
		if baiduModel == "" {
			baiduModel = "NB"
		}
		baiduGen := generators.NewBaiduImageGeneratorWithCredentials(
			cfg.Models.BaiduImageKey,
			cfg.Models.BaiduImageAK,
			cfg.Models.BaiduImageSK,
			cfg.Models.BaiduImageBase, baiduModel, "baidu-img", logger)
		gens["baidu-img"] = baiduGen
	}

	// Register Qianfan image channel (百度千帆) — synchronous OpenAI-compatible API.
	// Verified models: flux.1-schnell ✅  stable-diffusion-xl ✅
	// Qianfan only accepts English prompts; a translator using the ppai OpenAI pool
	// is injected so Chinese prompts are automatically converted before submission.
	if cfg.Models.QianfanImageKey != "" {
		qianfanEndpoint := cfg.Models.QianfanImageBase
		if qianfanEndpoint == "" {
			qianfanEndpoint = "https://qianfan.baidubce.com/v2/images/generations"
		}
		// Build a prompt translator backed by the main OpenAI key pool (ppai).
		qianfanTranslator := generators.NewPromptTranslator(cfg.Models.OpenAIKeys, cfg.Models.OpenAIBase, logger)
		for _, modelName := range cfg.Models.QianfanImageModels {
			if modelName == "" {
				continue
			}
			genKey := "qianfan-" + modelName
			if _, exists := gens[genKey]; !exists {
				// 千帆 API prompt 上限约 2000 字符，超出会报 invalid_image_generation_prompt。
				gens[genKey] = generators.NewDalleGeneratorWithTranslator(
					[]string{cfg.Models.QianfanImageKey}, qianfanEndpoint, modelName, genKey, 2000, qianfanTranslator, logger)
			}
			// Also register under exact model name for direct selection
			if _, exists := gens[modelName]; !exists {
				gens[modelName] = gens[genKey]
			}
		}
	}

	// Register Doubao/Ark image channel (doubao-seedream-4-0 等).
	// Uses OpenAI images/generations compatible API at /api/v3/images/generations.
	if cfg.Models.DoubaoImageKey != "" {
		doubaoModel := cfg.Models.DoubaoImageModel
		if doubaoModel == "" {
			doubaoModel = "doubao-seedream-4-0-250828"
		}
		doubaoEndpoint := cfg.Models.DoubaoImageBase
		if doubaoEndpoint == "" {
			doubaoEndpoint = "https://ark.cn-beijing.volces.com/api/v3/images/generations"
		}
		gens["doubao-image"] = generators.NewDalleGeneratorWithEndpoint(
			[]string{cfg.Models.DoubaoImageKey}, doubaoEndpoint, doubaoModel, "doubao-image", logger)
		// Also register under the model name alias for direct lookup
		if doubaoModel != "doubao-image" {
			gens[doubaoModel] = gens["doubao-image"]
		}
		// doubao-seedream-3-0-t2i: only register if doubao_image_model_v3 is explicitly configured.
		// The model endpoint (e.g. doubao-seedream-3-0-t2i-250415) must be activated in your Ark
		// console; otherwise the API returns 404 "does not exist or you do not have access".
		if v3Model := cfg.Models.DoubaoImageModelV3; v3Model != "" {
			gens["doubao-seedream-3-0-t2i"] = generators.NewDalleGeneratorWithEndpoint(
				[]string{cfg.Models.DoubaoImageKey}, doubaoEndpoint, v3Model, "doubao-seedream-3-0-t2i", logger)
			gens[v3Model] = gens["doubao-seedream-3-0-t2i"]
		}
	}

	imageSvc := service.NewImageService(imageRepo, gens, cfg, logger)

	if len(gens) == 0 {
		logger.Warn("no image generators configured; all generation requests will fail — set models.openai_keys, models.replicate_keys, models.comfyui_urls, or models.tongyi_keys")
	}

	// Config hot-reload watcher (Feature opt-p6)
	configPath := os.Getenv("AUTOVIDEO_CONFIG_FILE")
	if configPath == "" {
		configPath = "../../config.local.yaml"
	}
	config.StartWatcher(configPath, func(newCfg *config.Config) {
		logger.Info("config file changed; API key/URL changes require service restart for image generators")
	})

	// Resume orphaned tasks from previous unclean shutdown
	go imageSvc.ResumeOrphanedTasks(context.Background())

	// Periodic cleanup of stuck running tasks (every 3 minutes)
	go imageSvc.StartStaleCleanup(context.Background(), 3*time.Minute)

	consumer := service.NewKafkaConsumer(cfg, imageSvc, logger)
	go func() {
		if err := consumer.Start(context.Background()); err != nil {
			logger.Error("kafka consumer error", zap.Error(err))
		}
	}()

	imageHandler := handler.NewImageHandler(imageSvc, logger)

	router := gin.New()
	router.Use(middleware.Logger(logger))
	router.Use(gin.Recovery())
	router.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	router.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	router.NoRoute(func(c *gin.Context) {
		response.Error(c, http.StatusNotFound, "route not found")
	})

	v1 := router.Group("/api/v1")
	v1.Use(middleware.Auth(cfg.JWT.Secret))
	{
		images := v1.Group("/images")
		images.GET("/model-status", imageHandler.ModelStatus)
		images.GET("/model-capabilities", imageHandler.ModelCapabilities)
		images.GET("/gemini-channels", imageHandler.GeminiChannels)
		images.POST("/generate", imageHandler.Generate)
		images.GET("/tasks/:id", imageHandler.GetTask)
		images.GET("/tasks", imageHandler.ListTasks)
		images.POST("/tasks/:id/retry", imageHandler.RetryTask)
		images.DELETE("/tasks/:id", imageHandler.CancelTask)
	}
	v1.DELETE("/projects/:pid/images/runtime-data", imageHandler.DeleteProjectData)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // image generation can take minutes — per-handler ctx controls timeout
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("image-service starting", zap.Int("port", cfg.HTTP.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server listen error", zap.Error(err))
		}
	}()

	// Register with gateway for service discovery
	{
		rootCtx, rootCancel := context.WithCancel(context.Background())
		selfAddr := cfg.Gateway.SelfAddr
		if selfAddr == "" {
			selfAddr = fmt.Sprintf("http://localhost:%d", cfg.HTTP.Port)
		}
		registry.Start(rootCtx, cfg.Gateway.Addr, "image", selfAddr)
		defer rootCancel()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("server forced shutdown", zap.Error(err))
	}
	logger.Info("server exited")
}
