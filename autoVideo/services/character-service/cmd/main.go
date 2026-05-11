package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/autovideo/character-service/internal/handler"
	"github.com/autovideo/character-service/internal/model"
	"github.com/autovideo/character-service/internal/repository"
	"github.com/autovideo/character-service/internal/seed"
	"github.com/autovideo/character-service/internal/service"
	"github.com/autovideo/character-service/pkg/config"
	"github.com/autovideo/character-service/pkg/middleware"
	"github.com/autovideo/character-service/pkg/registry"
	"github.com/autovideo/character-service/proto"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// main —— 服务入口，初始化数据库、Kafka、路由并启动 HTTP 服务，支持优雅关闭
func main() {
	// ── Logger ────────────────────────────────────────────────────────────────
	log, _ := zap.NewProduction()
	defer log.Sync()

	// ── Config ────────────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("load config failed", zap.Error(err))
	}
	if err := applyRuntimeConfig(cfg); err != nil {
		log.Fatal("load runtime api keys failed", zap.Error(err))
	}

	// jwtUserSecret: used to validate user-issued JWTs (from auth-service)
	// jwtSvcSecret:  used to validate service-to-service JWTs (signed by project-service etc.)
	// Both keys should share the same value in production; AccessSecret takes priority.
	jwtUserSecret := cfg.JWT.Secret
	jwtSvcSecret := cfg.JWT.AccessSecret
	if jwtSvcSecret == "" {
		jwtSvcSecret = cfg.JWT.Secret
	}

	// ── Database ──────────────────────────────────────────────────────────────
	db, err := gorm.Open(postgres.Open(cfg.DB.DSN), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		log.Fatal("connect database failed", zap.Error(err))
	}
	if err = db.AutoMigrate(&model.Character{}, &model.StylePreset{}, &model.CharacterGroup{}, &model.Asset{}, &model.Skill{}, &model.ProductionSkill{}); err != nil {
		log.Fatal("auto migrate failed", zap.Error(err))
	}

	if cfg.LLM.APIKey == "" {
		log.Warn("LLM API key not configured (llm.api_key); asset extraction and skill generation will fail")
	}

	// ── Repos / Services ──────────────────────────────────────────────────────
	charRepo := repository.NewCharacterRepo(db)
	styleRepo := repository.NewStyleRepo(db)
	assetRepo := repository.NewAssetRepo(db)
	// Recover orphaned image-generation assets from previous unclean shutdown.
	// Extraction sentinels are resumed separately below.
	if n, err := assetRepo.ResetOrphanedGeneratingOnly(); err != nil {
		log.Error("reset orphaned assets", zap.Error(err))
	} else if n > 0 {
		log.Info("reset orphaned generating assets on startup", zap.Int64("count", n))
	}
	storageClient := service.NewStorageClient(cfg.Storage.BaseURL)
	llmTimeout := time.Duration(cfg.LLM.Timeout) * time.Second
	if llmTimeout == 0 {
		llmTimeout = 120 * time.Second
	}

	charSvc := service.NewCharacterService(charRepo, styleRepo, storageClient, assetRepo, log)

	// Gemini config: bases and keys are comma-separated; pick first entry for chat routing.
	geminiBase, geminiKey := "", ""
	if bases := strings.Split(cfg.Gemini.Bases, ","); len(bases) > 0 {
		geminiBase = strings.TrimSpace(bases[0])
	}
	if keys := strings.Split(cfg.Gemini.Keys, ","); len(keys) > 0 {
		geminiKey = strings.TrimSpace(keys[0])
	}

	assetSvc := service.NewAssetService(
		assetRepo, storageClient, log,
		cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.Model, cfg.LLM.VisionModel, llmTimeout,
		cfg.Claude.BaseURL, cfg.Claude.APIKey,
		cfg.Qwen.BaseURL, cfg.Qwen.APIKey,
		cfg.Zhipu.BaseURL, cfg.Zhipu.APIKey,
		geminiBase, geminiKey,
		cfg.ModelService.BaseURL,
	)

	// ── Kafka ────────────────────────────────────────────────────────────────
	var kafkaProducer *service.KafkaProducer
	var kafkaConsumer *service.KafkaConsumer
	var kafkaCancel context.CancelFunc

	// Start periodic stale-generation cleanup (every 2 min, threshold 15 min)
	staleCtx, staleCancel := context.WithCancel(context.Background())
	go assetSvc.StartStaleCleanup(staleCtx, 2*time.Minute, 15*time.Minute)

	if len(cfg.Kafka.Brokers) > 0 && cfg.Kafka.ConsumerTopic != "" {
		kafkaProducer = service.NewKafkaProducer(cfg.Kafka.Brokers, cfg.Kafka.ConsumerTopic, log)
		assetSvc.SetKafkaProducer(kafkaProducer)

		kafkaConsumer = service.NewKafkaConsumer(
			cfg.Kafka.Brokers,
			cfg.Kafka.ConsumerGroup,
			cfg.Kafka.ConsumerTopic,
			cfg.Kafka.ProducerTopic,
			cfg.Image.BaseURL,
			cfg.ProjectService.BaseURL,
			jwtUserSecret,
			assetSvc,
			log,
			cfg.Concurrency.MaxGenerations,
		)
		var kafkaCtx context.Context
		kafkaCtx, kafkaCancel = context.WithCancel(context.Background())
		go kafkaConsumer.Start(kafkaCtx)
		// 把单栏重绘入口注入到 AssetService，供 handler 调用（复用编排层）。
		assetSvc.SetPanelRegenerator(kafkaConsumer.RegenPanel)
		log.Info("kafka integration enabled",
			zap.Strings("brokers", cfg.Kafka.Brokers),
			zap.String("consumer_topic", cfg.Kafka.ConsumerTopic),
			zap.String("producer_topic", cfg.Kafka.ProducerTopic),
		)
	}

	// ── Handlers ──────────────────────────────────────────────────────────────
	charHandler := handler.NewCharacterHandler(charSvc)
	styleHandler := handler.NewStyleHandler(charSvc)
	skillRepo := repository.NewSkillRepository(db)
	skillHandler := handler.NewSkillHandler(skillRepo)
	// Wire skill repo into asset service so generation prompts benefit from skill hint injection.
	assetSvc.SetSkillRepo(skillRepo)

	productionSkillRepo := repository.NewProductionSkillRepository(db)
	productionSkillHandler := handler.NewProductionSkillHandler(productionSkillRepo, db)

	// 角色组（视频串行流程多造型管理）
	characterGroupRepo := repository.NewCharacterGroupRepo(db)
	characterGroupHandler := handler.NewCharacterGroupHandler(characterGroupRepo, log)

	extractSvc := service.NewExtractService(
		assetSvc,
		skillRepo,
		cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.Model,
		llmTimeout,
		cfg.ProjectService.BaseURL, jwtUserSecret,
		log,
	)
	if resumed, err := extractSvc.ResumeStaleExtractions(context.Background(), 20); err != nil {
		log.Error("resume stale extractions", zap.Error(err))
	} else if resumed > 0 {
		log.Info("resumed interrupted extraction jobs", zap.Int("count", resumed))
	}

	assetHandler := handler.NewAssetHandler(assetSvc, extractSvc, log)

	// Wire Gemini multimodal channel
	if cfg.Gemini.Bases != "" && cfg.Gemini.Keys != "" {
		splitBases := splitComma(cfg.Gemini.Bases)
		splitKeys := splitComma(cfg.Gemini.Keys)
		assetHandler.SetGemini(splitBases, splitKeys, cfg.Gemini.Model)
	}

	grpcServer := proto.NewCharacterGRPCServer(charSvc)

	// Config hot-reload watcher (Feature opt-p6)
	configPath := os.Getenv("AUTOVIDEO_CONFIG_FILE")
	if configPath == "" {
		configPath = "../../config.local.yaml"
	}
	config.StartWatcher(configPath, func(newCfg *config.Config) {
		log.Info("config file changed; LLM API key/URL changes require service restart for asset generation")
	})

	// ── Gin Router ────────────────────────────────────────────────────────────
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Logger(log))

	// health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// internal RPC endpoints (no JWT)
	internalGroup := r.Group("/internal/grpc")
	grpcServer.Register(internalGroup)

	// internal seed endpoint — service-to-service only, requires role=service JWT
	r.POST("/internal/seed-skills", middleware.InternalAuth(jwtSvcSecret), func(c *gin.Context) {
		var req struct {
			ProjectID   int64  `json:"project_id" binding:"required"`
			StylePreset string `json:"style_preset"` // e.g. "live-action-film", "anime-2d"; empty = live-action default
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := seed.SeedDefaultSkillsForProject(db, req.ProjectID); err != nil {
			log.Error("seed default skills failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// Seed production skills differentiated by style preset:
		//   live-action-film / live-action-short → 摄影/灯光/美术/道具/服化/场记/剪辑/调色/字幕/音效
		//   anime-2d / anime-3d                 → 分镜/原画/背景/色彩/动效/特效/配音/音效/剪辑/字幕
		if err := seed.SeedDefaultProductionSkillsForProject(db, req.ProjectID, req.StylePreset); err != nil {
			log.Error("seed default production skills failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "style_preset": req.StylePreset})
	})

	// public API (JWT protected)
	api := r.Group("/api/v1", middleware.Auth(jwtUserSecret))
	{
		// free-form chat (reference assistant panel, no asset required)
		api.POST("/chat", assetHandler.ChatFree)
		// Gemini multimodal chat (TEXT + IMAGE output)
		api.POST("/chat/gemini", assetHandler.ChatGemini)
		// existing character endpoints
		chars := api.Group("/characters")
		chars.GET("", charHandler.List)
		chars.POST("", charHandler.Create)
		chars.GET("/:id", charHandler.Get)
		chars.PUT("/:id", charHandler.Update)
		chars.DELETE("/:id", charHandler.Delete)
		chars.POST("/:id/reference", charHandler.UploadReference)
		chars.POST("/:id/style", charHandler.SetStyle)

		api.GET("/style-presets", styleHandler.ListPresets)

		// skill management endpoints
		skills := api.Group("/skills")
		skills.GET("", skillHandler.List)
		skills.POST("", skillHandler.Create)
		skills.POST("/reseed", func(c *gin.Context) {
			var projectID int64
			if s := c.Query("project_id"); s != "" {
				id, err := strconv.ParseInt(s, 10, 64)
				if err != nil || id < 0 {
					c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
					return
				}
				projectID = id
			}
			// projectID = 0 → reseed global/default skills
			if err := seed.UpsertDefaultSkillsForProject(db, projectID); err != nil {
				log.Error("reseed skills failed", zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})
		skills.GET("/:id", skillHandler.Get)
		skills.PUT("/:id", skillHandler.Update)
		skills.DELETE("/:id", skillHandler.Delete)
		chars.GET("/:id/skills", skillHandler.ListByCharacter)

		// asset management endpoints
		projects := api.Group("/projects/:pid")
		{
			// 角色组（视频串行流程）
			cgroups := projects.Group("/character-groups")
			cgroups.GET("", characterGroupHandler.List)
			cgroups.POST("", characterGroupHandler.Create)
			cgroups.GET("/:gid", characterGroupHandler.Get)
			cgroups.PUT("/:gid", characterGroupHandler.Update)
			cgroups.DELETE("/:gid", characterGroupHandler.Delete)
			cgroups.POST("/:gid/variants", characterGroupHandler.AssignVariant)
			cgroups.DELETE("/:gid/variants/:aid", characterGroupHandler.RemoveVariant)

			assets := projects.Group("/assets")
			assets.GET("", assetHandler.ListAssets)
			assets.DELETE("", assetHandler.DeleteAllAssets)
			assets.POST("", assetHandler.CreateAsset)
			assets.POST("/extract", assetHandler.ExtractAssets)
			assets.POST("/extract-episode/:eid", assetHandler.ExtractEpisodeAssets)
			assets.POST("/generate-all", assetHandler.GenerateAllAssets)
			assets.POST("/generate-batch", assetHandler.GenerateBatchAssets)
			assets.POST("/retry-failed", assetHandler.RetryFailedAssets)
			assets.POST("/pause-generation", assetHandler.PauseAssetGeneration)
			assets.POST("/resume-generation", assetHandler.ResumeAssetGeneration)
			assets.POST("/backfill-episodes", assetHandler.BackfillEpisodeIDs)
			assets.POST("/auto-match-voices", assetHandler.AutoMatchVoices)
			assets.GET("/:id", assetHandler.GetAsset)
			assets.PATCH("/:id", assetHandler.UpdateAsset)
			assets.DELETE("/:id", assetHandler.DeleteAsset)
			assets.POST("/:id/generate", assetHandler.GenerateAsset)
			assets.POST("/:id/retry", assetHandler.RetryAsset)
			assets.POST("/:id/regen-panel", assetHandler.RegenPanel)
			assets.POST("/:id/recomposite", assetHandler.RecompositePanels)
			assets.POST("/:id/reset", assetHandler.ResetAsset)
			assets.POST("/:id/upload", assetHandler.UploadAsset)
			assets.POST("/:id/chat", assetHandler.ChatAsset)

			projects.PATCH("/consistency-config", assetHandler.UpdateConsistencyConfig)

		// production skills (影视部门技能)
		pskills := projects.Group("/production-skills")
		pskills.GET("", productionSkillHandler.List)
		pskills.POST("", productionSkillHandler.Create)
		pskills.POST("/seed-defaults", productionSkillHandler.SeedDefaults)
		pskills.POST("/reseed-defaults", productionSkillHandler.ReseedDefaults)
		pskills.GET("/:id", productionSkillHandler.Get)
		pskills.PUT("/:id", productionSkillHandler.Update)
		pskills.DELETE("/:id", productionSkillHandler.Delete)

		// DELETE /api/v1/projects/:pid/runtime-data — full project cleanup (all assets, characters, skills)
		projects.DELETE("/runtime-data", func(c *gin.Context) {
			pidStr := c.Param("pid")
			pid, err := strconv.ParseInt(pidStr, 10, 64)
			if err != nil || pid <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
				return
			}
			// 1. Force-delete all assets (including locked)
			if err := assetRepo.ForceDeleteByProjectID(uint64(pid)); err != nil {
				log.Error("force delete assets failed", zap.Int64("project_id", pid), zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			// 2. Delete all characters
			if err := charRepo.DeleteByProjectID(pid); err != nil {
				log.Error("delete characters failed", zap.Int64("project_id", pid), zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			// 3. Delete all skills
			if err := skillRepo.DeleteAllByProject(pid); err != nil {
				log.Error("delete skills failed", zap.Int64("project_id", pid), zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			// 4. Delete all production skills
			if err := productionSkillRepo.DeleteAllByProject(pid); err != nil {
				log.Error("delete production skills failed", zap.Int64("project_id", pid), zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			log.Info("project runtime data deleted", zap.Int64("project_id", pid))
			c.JSON(http.StatusOK, gin.H{"deleted": true})
		})
		}
	}

	// ── HTTP Server ───────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 180 * time.Second,
	}

	go func() {
		log.Info("character-service starting", zap.Int("port", cfg.HTTP.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	// Register with gateway for service discovery
	{
		rootCtx, rootCancel := context.WithCancel(context.Background())
		selfAddr := cfg.Gateway.SelfAddr
		if selfAddr == "" {
			selfAddr = fmt.Sprintf("http://localhost:%d", cfg.HTTP.Port)
		}
		registry.Start(rootCtx, cfg.Gateway.Addr, "character", selfAddr)
		defer rootCancel()
	}

	// graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown Kafka
	staleCancel()
	if kafkaCancel != nil {
		kafkaCancel()
	}
	if kafkaConsumer != nil {
		if err := kafkaConsumer.Close(); err != nil {
			log.Error("kafka consumer close error", zap.Error(err))
		}
	}
	if kafkaProducer != nil {
		if err := kafkaProducer.Close(); err != nil {
			log.Error("kafka producer close error", zap.Error(err))
		}
	}

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("server shutdown error", zap.Error(err))
	}
	log.Info("character-service stopped")
}
// splitComma splits a comma-separated string, trims spaces, and drops empty entries.
func splitComma(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}