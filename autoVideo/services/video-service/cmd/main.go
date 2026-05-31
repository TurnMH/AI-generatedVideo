package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/autovideo/video-service/internal/handler"
	"github.com/autovideo/video-service/internal/model"
	"github.com/autovideo/video-service/internal/repository"
	"github.com/autovideo/video-service/internal/service"
	"github.com/autovideo/video-service/internal/service/generators"
	"github.com/autovideo/video-service/pkg/config"
	"github.com/autovideo/video-service/pkg/middleware"
	"github.com/autovideo/video-service/pkg/registry"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type generatorRegistrationSummary struct {
	Name            string
	RuntimeProvider string
	NativeAudio     bool
	Available       bool
	ModelHint       string
	BaseHint        string
}

func runtimeProviderForGeneratorName(name string) string {
	switch name {
	case "kling":
		return "runtime.video.kling"
	case "aiping":
		return "runtime.video.aiping"
	case "tencent-vclm":
		return "runtime.video.vclm"
	case "wan":
		return "runtime.video.wan"
	case "comfyui-video":
		return "runtime.video.comfyui"
	case "runninghub":
		return "runtime.video.runninghub"
	case "cogvideo":
		return "runtime.video.replicate"
	case "sora2":
		return "runtime.video.sora2"
	case "doubao":
		return "runtime.video.doubao"
	case "doubao-seedance":
		return "runtime.video.doubao.seedance"
	case "vidu", "vidu-mix":
		return "runtime.video.vidu"
	case "vidu-offpeak", "vidu-mix-offpeak":
		return "runtime.video.vidu.offpeak"
	case "suanneng":
		return "runtime.video.suanneng"
	case "minmax":
		return "runtime.video.minmax"
	case "gaga":
		return "runtime.video.gaga"
	case "baidu-bce":
		return "runtime.video.baidu.bce"
	default:
		if name == "hubagi-voe3.1" || name == "hubagi-TC-GV" || len(name) >= 7 && name[:7] == "hubagi-" {
			return "runtime.video.veo"
		}
		return ""
	}
}

func logGeneratorRegistrationSummaries(logger *zap.Logger, summaries []generatorRegistrationSummary) {
	sort.Slice(summaries, func(i, j int) bool { return summaries[i].Name < summaries[j].Name })
	for _, item := range summaries {
		logger.Info("video generator registered",
			zap.String("generator", item.Name),
			zap.String("runtime_provider", item.RuntimeProvider),
			zap.Bool("native_audio", item.NativeAudio),
			zap.Bool("available", item.Available),
			zap.String("model_hint", item.ModelHint),
			zap.String("base_hint", item.BaseHint),
		)
	}
}

// main —— 程序入口，初始化日志、数据库、FFmpeg、生成器、Kafka 等组件并启动 HTTP 服务
func main() {
	// ── Logger ──────────────────────────────────────────────
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// ── Config ──────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("load config", zap.Error(err))
	}
	if err := applyRuntimeConfig(cfg); err != nil {
		logger.Warn("load runtime api keys failed; continuing with config defaults", zap.Error(err))
	}

	// ── Database ────────────────────────────────────────────
	db, err := gorm.Open(postgres.Open(cfg.DB.DSN), &gorm.Config{})
	if err != nil {
		logger.Fatal("connect db", zap.Error(err))
	}
	// AutoMigrate (in production you'd use the SQL migration file instead)
	if err := db.AutoMigrate(&model.VideoTask{}, &model.VideoClip{}, &model.DubbingTask{}); err != nil {
		logger.Fatal("auto migrate", zap.Error(err))
	}

	// ── FFmpeg ──────────────────────────────────────────────
	ffmpegSvc := service.NewFFmpegService(cfg.FFmpeg.TempDir, cfg.FFmpeg.Bin)
	if err := ffmpegSvc.EnsureTempDir(); err != nil {
		logger.Fatal("ensure temp dir", zap.Error(err))
	}

	// ── Generators ──────────────────────────────────────────
	var gens []generators.VideoGenerator
	var genSummaries []generatorRegistrationSummary
	// Kling: prefer kling_keys pool; fall back to single kling_key
	klingKeys := append([]string{cfg.Models.KlingKey}, cfg.Models.KlingKeys...)
	if gen := generators.NewKlingGeneratorWithKeys(cfg.Models.KlingBase, klingKeys...); gen.IsAvailable(context.Background()) {
		if cfg.Models.KlingModel != "" {
			gen.WithModel(cfg.Models.KlingModel)
		}
		if cfg.Models.KlingOmniModel != "" {
			gen.WithOmniModel(cfg.Models.KlingOmniModel)
		}
		gens = append(gens, gen)
		genSummaries = append(genSummaries, generatorRegistrationSummary{Name: gen.Name(), RuntimeProvider: runtimeProviderForGeneratorName(gen.Name()), NativeAudio: gen.SupportsNativeAudio(), Available: true, ModelHint: cfg.Models.KlingModel, BaseHint: cfg.Models.KlingBase})
	}
	if cfg.Models.AipingKey != "" {
		aipingBase := cfg.Models.AipingBase
		if aipingBase == "" {
			aipingBase = "https://aiping.cn"
		}
		aipingGen := generators.NewKlingGeneratorWithKeys(aipingBase, cfg.Models.AipingKey)
		aipingModel := "kling-v3"
		if cfg.Models.AipingModel != "" {
			aipingModel = cfg.Models.AipingModel
		}
		aipingGen.WithModel(aipingModel)
		aipingGen.WithName("aiping")
		gens = append(gens, aipingGen)
		genSummaries = append(genSummaries, generatorRegistrationSummary{Name: aipingGen.Name(), RuntimeProvider: runtimeProviderForGeneratorName(aipingGen.Name()), NativeAudio: aipingGen.SupportsNativeAudio(), Available: true, ModelHint: aipingModel, BaseHint: aipingBase})
	}
	if cfg.Models.VclmSecretID != "" && cfg.Models.VclmSecretKey != "" {
		gen := generators.NewTencentVCLMGenerator(cfg.Models.VclmSecretID, cfg.Models.VclmSecretKey, cfg.Models.VclmRegion)
		gens = append(gens, gen)
		genSummaries = append(genSummaries, generatorRegistrationSummary{Name: gen.Name(), RuntimeProvider: runtimeProviderForGeneratorName(gen.Name()), NativeAudio: gen.SupportsNativeAudio(), Available: true, ModelHint: "", BaseHint: cfg.Models.VclmRegion})
	}
	if cfg.Models.WanKey != "" {
		gen := generators.NewWanGenerator(cfg.Models.WanKey, cfg.Models.WanSecret, cfg.Models.WanBase)
		gens = append(gens, gen)
		genSummaries = append(genSummaries, generatorRegistrationSummary{Name: gen.Name(), RuntimeProvider: runtimeProviderForGeneratorName(gen.Name()), NativeAudio: gen.SupportsNativeAudio(), Available: true, ModelHint: "wanx2.1-i2v-turbo", BaseHint: cfg.Models.WanBase})
	}
	if cfg.Models.ComfyUIURL != "" {
		gen := generators.NewComfyUIVideoGenerator(cfg.Models.ComfyUIURL, cfg.Models.ComfyUIWorkflow)
		if cfg.Models.ComfyUIIPAdapter != "" {
			gen.WithIPAdapter(cfg.Models.ComfyUIIPAdapter)
		}
		if cfg.Models.ComfyUILoRAModel != "" {
			gen.WithLoRA(cfg.Models.ComfyUILoRAModel, cfg.Models.ComfyUILoRAWeight)
		}
		gens = append(gens, gen)
		genSummaries = append(genSummaries, generatorRegistrationSummary{Name: gen.Name(), RuntimeProvider: runtimeProviderForGeneratorName(gen.Name()), NativeAudio: gen.SupportsNativeAudio(), Available: true, ModelHint: cfg.Models.ComfyUIWorkflow, BaseHint: cfg.Models.ComfyUIURL})
	}
	if cfg.Models.RunningHubKey != "" {
		gen := generators.NewRunningHubGenerator(cfg.Models.RunningHubKey, cfg.Models.RunningHubBase, cfg.Models.RunningHubWorkflow, cfg.Models.RunningHubNodeID)
		gens = append(gens, gen)
		genSummaries = append(genSummaries, generatorRegistrationSummary{Name: gen.Name(), RuntimeProvider: runtimeProviderForGeneratorName(gen.Name()), NativeAudio: gen.SupportsNativeAudio(), Available: true, ModelHint: cfg.Models.RunningHubWorkflow, BaseHint: cfg.Models.RunningHubBase})
	}
	if cfg.Models.ReplicateKey != "" {
		gen := generators.NewCogVideoGenerator(cfg.Models.ReplicateKey)
		gens = append(gens, gen)
		genSummaries = append(genSummaries, generatorRegistrationSummary{Name: gen.Name(), RuntimeProvider: runtimeProviderForGeneratorName(gen.Name()), NativeAudio: gen.SupportsNativeAudio(), Available: true, ModelHint: "cogvideo", BaseHint: ""})
	}
	if cfg.Models.Sora2Key != "" {
		gen := generators.NewSora2Generator(cfg.Models.Sora2Key, cfg.Models.Sora2Base)
		gens = append(gens, gen)
		genSummaries = append(genSummaries, generatorRegistrationSummary{Name: gen.Name(), RuntimeProvider: runtimeProviderForGeneratorName(gen.Name()), NativeAudio: gen.SupportsNativeAudio(), Available: true, ModelHint: "sora2", BaseHint: cfg.Models.Sora2Base})
	}
	if cfg.Models.HubagiKey != "" {
		gen := generators.NewHubagiGenerator(cfg.Models.HubagiKey, cfg.Models.HubagiBase, cfg.Models.HubagiModel)
		gens = append(gens, gen)
		genSummaries = append(genSummaries, generatorRegistrationSummary{Name: gen.Name(), RuntimeProvider: runtimeProviderForGeneratorName(gen.Name()), NativeAudio: gen.SupportsNativeAudio(), Available: true, ModelHint: cfg.Models.HubagiModel, BaseHint: cfg.Models.HubagiBase})
	}
	if cfg.Models.VeoKey != "" {
		gen := generators.NewHubagiGenerator(cfg.Models.VeoKey, cfg.Models.VeoBase, cfg.Models.VeoModel)
		gens = append(gens, gen)
		genSummaries = append(genSummaries, generatorRegistrationSummary{Name: gen.Name(), RuntimeProvider: runtimeProviderForGeneratorName(gen.Name()), NativeAudio: gen.SupportsNativeAudio(), Available: true, ModelHint: cfg.Models.VeoModel, BaseHint: cfg.Models.VeoBase})
	}
	if cfg.Models.DoubaoKey != "" {
		gen := generators.NewDoubaoGenerator(cfg.Models.DoubaoKey, cfg.Models.DoubaoBase, cfg.Models.DoubaoModel, "doubao")
		gens = append(gens, gen)
		genSummaries = append(genSummaries, generatorRegistrationSummary{Name: gen.Name(), RuntimeProvider: runtimeProviderForGeneratorName(gen.Name()), NativeAudio: gen.SupportsNativeAudio(), Available: true, ModelHint: cfg.Models.DoubaoModel, BaseHint: cfg.Models.DoubaoBase})
	}
	if cfg.Models.DoubaoSeedanceKey != "" {
		gen := generators.NewDoubaoSeedanceGenerator(cfg.Models.DoubaoSeedanceKey, cfg.Models.DoubaoSeedanceBase, cfg.Models.DoubaoSeedanceModel, "doubao-seedance")
		gens = append(gens, gen)
		genSummaries = append(genSummaries, generatorRegistrationSummary{Name: gen.Name(), RuntimeProvider: runtimeProviderForGeneratorName(gen.Name()), NativeAudio: gen.SupportsNativeAudio(), Available: true, ModelHint: cfg.Models.DoubaoSeedanceModel, BaseHint: cfg.Models.DoubaoSeedanceBase})
	}
	if cfg.Models.ViduKey != "" {
		viduBase := cfg.Models.ViduBase
		if viduBase == "" {
			viduBase = "https://api.vidu.cn/ent/v2"
		}
		viduModel := cfg.Models.ViduModel
		if viduModel == "" {
			viduModel = "viduq3-pro"
		}
		gen := generators.NewViduGenerator(cfg.Models.ViduKey, viduBase, viduModel, "vidu")
		gens = append(gens, gen)
		genSummaries = append(genSummaries, generatorRegistrationSummary{Name: gen.Name(), RuntimeProvider: runtimeProviderForGeneratorName(gen.Name()), NativeAudio: gen.SupportsNativeAudio(), Available: true, ModelHint: viduModel, BaseHint: viduBase})
		viduMixModel := cfg.Models.ViduMixModel
		if viduMixModel == "" {
			viduMixModel = "viduq3-mix"
		}
		mixGen := generators.NewViduGenerator(cfg.Models.ViduKey, viduBase, viduMixModel, "vidu-mix")
		gens = append(gens, mixGen)
		genSummaries = append(genSummaries, generatorRegistrationSummary{Name: mixGen.Name(), RuntimeProvider: runtimeProviderForGeneratorName(mixGen.Name()), NativeAudio: mixGen.SupportsNativeAudio(), Available: true, ModelHint: viduMixModel, BaseHint: viduBase})
	}
	if cfg.Models.ViduOffpeakKey != "" {
		viduBase := cfg.Models.ViduBase
		if viduBase == "" {
			viduBase = "https://api.vidu.cn/ent/v2"
		}
		viduModel := cfg.Models.ViduModel
		if viduModel == "" {
			viduModel = "viduq3-pro"
		}
		viduMixModel := cfg.Models.ViduMixModel
		if viduMixModel == "" {
			viduMixModel = "viduq3-mix"
		}
		offpeakGen := generators.NewViduGenerator(cfg.Models.ViduOffpeakKey, viduBase, viduModel, "vidu-offpeak")
		gens = append(gens, offpeakGen)
		genSummaries = append(genSummaries, generatorRegistrationSummary{Name: offpeakGen.Name(), RuntimeProvider: runtimeProviderForGeneratorName(offpeakGen.Name()), NativeAudio: offpeakGen.SupportsNativeAudio(), Available: true, ModelHint: viduModel, BaseHint: viduBase})
		offpeakMixGen := generators.NewViduGenerator(cfg.Models.ViduOffpeakKey, viduBase, viduMixModel, "vidu-mix-offpeak")
		gens = append(gens, offpeakMixGen)
		genSummaries = append(genSummaries, generatorRegistrationSummary{Name: offpeakMixGen.Name(), RuntimeProvider: runtimeProviderForGeneratorName(offpeakMixGen.Name()), NativeAudio: offpeakMixGen.SupportsNativeAudio(), Available: true, ModelHint: viduMixModel, BaseHint: viduBase})
	}
	if cfg.Models.SuannengKey != "" {
		gen := generators.NewSuannengGenerator(cfg.Models.SuannengKey, cfg.Models.SuannengBase, cfg.Models.SuannengModel)
		gens = append(gens, gen)
		genSummaries = append(genSummaries, generatorRegistrationSummary{Name: gen.Name(), RuntimeProvider: runtimeProviderForGeneratorName(gen.Name()), NativeAudio: gen.SupportsNativeAudio(), Available: true, ModelHint: cfg.Models.SuannengModel, BaseHint: cfg.Models.SuannengBase})
	}
	if cfg.Models.MinMaxKey != "" {
		minmaxBase := cfg.Models.MinMaxBase
		if minmaxBase == "" {
			minmaxBase = "https://api.wavespeed.ai"
		}
		minmaxGen := generators.NewMinMaxGenerator(cfg.Models.MinMaxKey, minmaxBase, cfg.Models.MinMaxModel,
			generators.WithMinMaxEndpoints(cfg.Models.MinMaxImg2VideoEndpointStd, cfg.Models.MinMaxImg2VideoEndpointPro, cfg.Models.MinMaxImg2VideoEndpointFast, cfg.Models.MinMaxQueryEndpoint, cfg.Models.MinMaxFileRetrieveEndpoint),
			generators.WithMinMaxFlags(cfg.Models.MinMaxFastPretreatment, cfg.Models.MinMaxPromptOptimizer),
		)
		gens = append(gens, minmaxGen)
		genSummaries = append(genSummaries, generatorRegistrationSummary{Name: minmaxGen.Name(), RuntimeProvider: runtimeProviderForGeneratorName(minmaxGen.Name()), NativeAudio: minmaxGen.SupportsNativeAudio(), Available: true, ModelHint: cfg.Models.MinMaxModel, BaseHint: minmaxBase})
	}
	if cfg.Models.GagaKey != "" {
		gen := generators.NewGagaGenerator(cfg.Models.GagaKey, cfg.Models.GagaBase, "gaga-1")
		gens = append(gens, gen)
		genSummaries = append(genSummaries, generatorRegistrationSummary{Name: gen.Name(), RuntimeProvider: runtimeProviderForGeneratorName(gen.Name()), NativeAudio: gen.SupportsNativeAudio(), Available: true, ModelHint: "gaga-1", BaseHint: cfg.Models.GagaBase})
	}
	if cfg.Models.BaiduBCEKey != "" {
		gen := generators.NewBaiduBCEGenerator(cfg.Models.BaiduBCEKey, cfg.Models.BaiduBCESecret, cfg.Models.BaiduBCEModel)
		gens = append(gens, gen)
		genSummaries = append(genSummaries, generatorRegistrationSummary{Name: gen.Name(), RuntimeProvider: runtimeProviderForGeneratorName(gen.Name()), NativeAudio: gen.SupportsNativeAudio(), Available: true, ModelHint: cfg.Models.BaiduBCEModel, BaseHint: "bce-auth-v1"})
	}
	logGeneratorRegistrationSummaries(logger, genSummaries)
	if len(gens) == 0 {
		logger.Warn("no video generators configured — requests will fail")
	}

	// ── Services ────────────────────────────────────────────
	repo := repository.NewVideoRepo(db)
	videoSvc := service.NewVideoService(repo, ffmpegSvc, gens, cfg.Storage.BaseURL, cfg.Character.BaseURL, logger, cfg.Concurrency.MaxClips, cfg.Concurrency.LocalMaxClips)
	videoSvc.SetKafkaWriter(cfg.Kafka.Brokers, cfg.Kafka.ConsumerTopic)

	// 视频串行生成：末帧提取服务
	if cfg.FrameExtractor.BaseURL != "" {
		videoSvc.SetFrameExtractorURL(cfg.FrameExtractor.BaseURL)
		logger.Info("frame_extractor: enabled", zap.String("url", cfg.FrameExtractor.BaseURL))
	}
	if cfg.Whisper.URL != "" {
		videoSvc.SetWhisperURL(cfg.Whisper.URL)
		logger.Info("whisper: enabled", zap.String("url", cfg.Whisper.URL))
	}

	// opt-motion-llm: wire LLM motion prompt refiner when configured.
	// Falls back to sora2 channel if llm_key is unset but sora2_key is present.
	llmKey := cfg.Models.LLMKey
	llmBase := cfg.Models.LLMBase
	if llmKey == "" && cfg.Models.Sora2Key != "" {
		llmKey = cfg.Models.Sora2Key
		if llmBase == "" {
			llmBase = cfg.Models.Sora2Base
		}
	}
	if motionSvc := service.NewMotionPromptService(llmKey, llmBase, cfg.Models.LLMModel, logger); motionSvc != nil {
		videoSvc.SetMotionPromptService(motionSvc)
		logger.Info("motion_prompt: LLM refiner enabled", zap.String("model", cfg.Models.LLMModel))
	}
	if analyzerSvc := service.NewSerialFailureAnalyzer(llmKey, llmBase, cfg.Models.LLMModel, logger); analyzerSvc != nil {
		videoSvc.SetSerialFailureAnalyzer(analyzerSvc)
		logger.Info("serial_failure_analyzer: LLM analyzer enabled", zap.String("model", cfg.Models.LLMModel))
	}

	watermarkSvc := service.NewWatermarkService(ffmpegSvc, logger)
	dubbingSvc := service.NewDubbingService(logger, cfg.Storage.BaseURL, db)
	dubbingSvc.SetCharacterBaseURL(cfg.Character.BaseURL)
	if cfg.Whisper.URL != "" {
		dubbingSvc.SetWhisperURL(cfg.Whisper.URL) // feat-4
	}
	// Enable per-clip audio-video synthesis: each storyboard clip gets its own
	// TTS audio muxed before concat, eliminating dialogue↔visual drift.
	videoSvc.SetDubbingService(dubbingSvc)

	// ── Kafka Consumer ──────────────────────────────────────
	kafkaConsumer := service.NewKafkaConsumer(
		cfg.Kafka.Brokers,
		cfg.Kafka.ConsumerGroup,
		cfg.Kafka.ConsumerTopic,
		cfg.Kafka.ProducerTopic,
		videoSvc,
		logger,
		cfg.Concurrency.MaxKafkaTasks,
	)

	ctx, cancel := context.WithCancel(context.Background())
	go kafkaConsumer.Start(ctx)

	// ── Music Kafka Consumer ─────────────────────────────
	// Falls back to sora2 API key/base if music-specific key is absent.
	musicKey := cfg.Models.MusicKey
	musicBase := cfg.Models.MusicBase
	if musicKey == "" && cfg.Models.Sora2Key != "" {
		musicKey = cfg.Models.Sora2Key
		if musicBase == "" {
			musicBase = cfg.Models.Sora2Base
		}
	}
	if musicKey != "" {
		musicConsumer := service.NewMusicKafkaConsumer(
			cfg.Kafka.Brokers,
			musicKey, musicBase, cfg.Models.MusicModel,
			cfg.Storage.BaseURL,
			logger,
		)
		go musicConsumer.Start(ctx)
		logger.Info("music consumer started", zap.String("model", cfg.Models.MusicModel))
	} else {
		logger.Warn("music consumer disabled — set models.music_key in config.local.yaml to enable")
	}

	// Config hot-reload watcher (Feature opt-p6)
	configPath := os.Getenv("AUTOVIDEO_CONFIG_FILE")
	if configPath == "" {
		configPath = "../../config.local.yaml"
	}
	config.StartWatcher(configPath, func(newCfg *config.Config) {
		logger.Info("config file changed; API key/URL changes require service restart for video generators")
	})

	// Startup recovery: re-dispatch orphaned and pending tasks
	go func() {
		time.Sleep(3 * time.Second) // wait for Kafka consumer to be ready
		videoSvc.ResumeOrphanedTasks(context.Background())
		videoSvc.ResumePendingTasks(context.Background())
		dubbingSvc.ResumeOrphanedTasks(context.Background())
		dubbingSvc.ResumePendingTasks(context.Background())
	}()

	// Periodic watchdog: mark video tasks stuck in "processing" > 3h 10min as failed.
	// 3h 10min = 3h task ctx timeout + 10 min grace; runs every 5 min.
	go videoSvc.WatchStaleTasks(ctx, 3*time.Hour+10*time.Minute, 5*time.Minute)

	// ── HTTP Server ──────────────────────────────────────────
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(middleware.Logger(logger), gin.Recovery())
	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	videoHandler := handler.NewVideoHandler(videoSvc, watermarkSvc, logger)
	dubbingHandler := handler.NewDubbingHandler(dubbingSvc, logger)
	publishHandler := handler.NewPublishHandler(cfg.Models.UploadPostKey, cfg.Models.UploadPostURL, logger) // feat-11

	api := r.Group("/api/v1")
	{
		// Public voice list endpoint (no auth required)
		api.GET("/voices", dubbingHandler.ListVoices)

		videos := api.Group("/videos")
		// Public download endpoint
		videos.GET("/:id/download", videoHandler.Download)

		// Authenticated endpoints
		auth := videos.Group("")
		auth.Use(middleware.Auth(cfg.JWT.Secret, logger))
		{
			auth.GET("/model-status", videoHandler.ModelStatus)
			auth.POST("/generate", videoHandler.Generate)
			auth.GET("/tasks", videoHandler.ListTasks)
			auth.GET("/tasks/:id", videoHandler.GetTask)
			auth.DELETE("/tasks/:id", videoHandler.DeleteTask)
			auth.POST("/tasks/:id/compose", videoHandler.Compose)
			auth.POST("/ad-compose", videoHandler.ComposeAdVideo)
		}

		// Project-scoped video endpoints
		projects := api.Group("/projects")
		projects.Use(middleware.Auth(cfg.JWT.Secret, logger))
		{
			pVideos := projects.Group("/:pid/videos")
			pVideos.GET("", videoHandler.ListProjectVideos)
			pVideos.GET("/stats", videoHandler.VideoStats)
			pVideos.DELETE("/runtime-data", videoHandler.DeleteProjectData)
			pVideos.POST("/generate", videoHandler.GenerateProjectVideo)
			pVideos.POST("/generate-batch", videoHandler.GenerateProjectVideosBatch)
			pVideos.POST("/content-extract", videoHandler.ExtractVideoContent)
			pVideos.POST("/generate-variants", videoHandler.GenerateVariants) // feat-6
			pVideos.POST("/retry-failed", videoHandler.RetryAllFailed)
			pVideos.POST("/:vid/pause", videoHandler.PauseVideo)
			pVideos.POST("/:vid/resume", videoHandler.ResumeVideo)
			pVideos.POST("/:vid/retry", videoHandler.RetryVideo)
			pVideos.POST("/:vid/clips/:cid/retry", videoHandler.RetryVideoClip)
			pVideos.GET("/:vid/export", videoHandler.ExportVideo)
			pVideos.POST("/:vid/watermark", videoHandler.ApplyWatermark)
			pVideos.POST("/:vid/publish", publishHandler.PublishVideo) // feat-11

			projects.GET("/:pid/dubbing/voices", dubbingHandler.VoiceCatalog)
			projects.POST("/:pid/dubbing/voices/recommend", dubbingHandler.RecommendVoice)
			projects.POST("/:pid/dubbing/voices/recommend-batch", dubbingHandler.RecommendVoicesBatch)
			projects.POST("/:pid/dubbing/voices/preview", dubbingHandler.PreviewVoice)
			projects.POST("/:pid/dubbing/voices/apply-recommended", dubbingHandler.ApplyRecommendedVoices)
			projects.POST("/:pid/dubbing/generate", dubbingHandler.GenerateDubbing)
			projects.POST("/:pid/dubbing/generate-batch", dubbingHandler.GenerateDubbingBatch)
			projects.GET("/:pid/dubbing/tasks", dubbingHandler.ListTasks)
			projects.GET("/:pid/dubbing/storyboard-tasks", dubbingHandler.ListStoryboardTasks)
			projects.DELETE("/:pid/dubbing/runtime-data", dubbingHandler.DeleteProjectData)
			projects.GET("/:pid/dubbing/tasks/:tid", dubbingHandler.GetTask)
			projects.POST("/:pid/dubbing/tasks/:tid/retry", dubbingHandler.RetryTask)
			projects.POST("/:pid/dubbing/tasks/retry-batch", dubbingHandler.RetryTasksBatch)
			projects.POST("/:pid/subtitle/generate", dubbingHandler.GenerateSubtitle)
			projects.POST("/:pid/subtitle/generate-batch", dubbingHandler.GenerateSubtitleBatch)
			// Storyboard-scoped TTS / subtitles
			projects.POST("/:pid/storyboards/:sid/dubbing", dubbingHandler.GenerateStoryboardDubbing)
			projects.POST("/:pid/storyboards/:sid/subtitle", dubbingHandler.GenerateStoryboardSubtitle)
			// Episode-level cleanup: cascade delete VideoTask+DubbingTask for one episode
			projects.DELETE("/:pid/episodes/:eid/videos/runtime-data", videoHandler.DeleteEpisodeData)
			// 自动审片: 查询 shots_metadata 或触发 clip-service 流水线
			projects.GET("/:pid/episodes/:eid/videos/shots-metadata", videoHandler.GetEpisodeShotsMetadata)
			projects.POST("/:pid/episodes/:eid/videos/clip-trigger", videoHandler.TriggerClipPipeline)
		}
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // Disabled: per-handler ctx timeout controls long requests (dubbing/video)
	}

	go func() {
		logger.Info("http server listening", zap.Int("port", cfg.HTTP.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("listen", zap.Error(err))
		}
	}()

	// Register with gateway for service discovery (reuse kafka ctx which is cancelled on shutdown)
	{
		selfAddr := cfg.Gateway.SelfAddr
		if selfAddr == "" {
			selfAddr = fmt.Sprintf("http://localhost:%d", cfg.HTTP.Port)
		}
		registry.Start(ctx, cfg.Gateway.Addr, "video", selfAddr)
	}

	// ── Graceful Shutdown ────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down...")
	cancel() // stop Kafka read loop and all other ctx-derived work

	// Wait for in-flight video tasks to finish generating clips.
	// Tasks still running after the drain timeout will remain in "processing"
	// state and will be reset to "pending" by ResumeOrphanedTasks on next startup.
	logger.Info("draining in-flight video tasks (up to 5 min)...")
	kafkaConsumer.Drain(5 * time.Minute)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown", zap.Error(err))
	}
	if err := kafkaConsumer.Close(); err != nil {
		logger.Error("kafka close", zap.Error(err))
	}
	if err := videoSvc.CloseKafka(); err != nil {
		logger.Error("kafka writer close", zap.Error(err))
	}
	logger.Info("stopped")
}
