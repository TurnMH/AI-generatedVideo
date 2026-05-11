package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/autovideo/project-service/internal/handler"
	"github.com/autovideo/project-service/internal/model"
	"github.com/autovideo/project-service/internal/repository"
	"github.com/autovideo/project-service/internal/service"
	_ "github.com/autovideo/project-service/pkg/codec" // registers JSON-as-proto gRPC codec
	"github.com/autovideo/project-service/pkg/config"
	"github.com/autovideo/project-service/pkg/middleware"
	"github.com/autovideo/project-service/pkg/registry"
	pb "github.com/autovideo/project-service/proto"
)

// main —— 程序入口，初始化数据库、服务、HTTP/gRPC 服务器和 Kafka，并阻塞等待优雅关闭
func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load(logger)
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}
	if err := applyRuntimeConfig(cfg); err != nil {
		logger.Fatal("failed to load runtime api keys from auth-service", zap.Error(err))
	}

	db, err := initDB(cfg.Database.DSN)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	if err := autoMigrate(db); err != nil {
		logger.Fatal("failed to run migrations", zap.Error(err))
	}

	// Repositories
	projectRepo := repository.NewProjectRepo(db)
	episodeRepo := repository.NewEpisodeRepo(db)
	storyboardRepo := repository.NewStoryboardRepo(db)
	snapshotRepo := repository.NewSnapshotRepo(db)
	scriptVersionRepo := repository.NewScriptVersionRepo(db)

	// Services
	projectSvc := service.NewProjectService(projectRepo, episodeRepo, storyboardRepo, snapshotRepo, scriptVersionRepo)
	episodeSvc := service.NewEpisodeService(episodeRepo, projectRepo, cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.Model, cfg.Storage.BaseURL)
	costSvc := service.NewCostService(projectRepo, episodeRepo)

	storyboardSvc := service.NewStoryboardService(storyboardRepo)
	storyboardSvc.SetMaxInFlight(cfg.Concurrency.MaxStoryboardInFlight)

	// Wire scene continuity auditor
	continuityAuditor := service.NewSceneContinuityAuditor(cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.Model, logger)
	storyboardSvc.SetContinuityAuditor(continuityAuditor)

	// Wire storyboard service into episode service for auto-creation
	episodeSvc.SetStoryboardService(storyboardSvc)
	episodeSvc.SetLogger(logger)

	// Kafka producer & consumer for storyboard generation
	var kafkaProducer *service.KafkaProducer
	var kafkaConsumer *service.KafkaConsumer
	var staleCleanupCancel context.CancelFunc

	if len(cfg.Kafka.Brokers) > 0 && cfg.Kafka.ConsumerTopic != "" {
		kafkaProducer = service.NewKafkaProducer(cfg.Kafka.Brokers, cfg.Kafka.ConsumerTopic, logger)
		storyboardSvc.SetKafkaProducer(kafkaProducer)
		storyboardSvc.SetLogger(logger)

		kafkaConsumer = service.NewKafkaConsumer(
			cfg.Kafka.Brokers,
			cfg.Kafka.ConsumerTopic,
			cfg.Kafka.ConsumerGroup,
			cfg.Kafka.ProducerTopic,
			storyboardSvc,
			projectRepo,
			cfg.Image.BaseURL,
			cfg.JWT.AccessSecret,
			logger,
			cfg.Concurrency.MaxStoryboardGenerations,
			cfg.LLM.BaseURL,
			cfg.LLM.APIKey,
			cfg.LLM.Model,
		)
		kafkaConsumer.SetCharacterBaseURL(cfg.Character.BaseURL)

		// Recover stale storyboards from previous crashes
		storyboardSvc.ResumeStaleStoryboards()
		staleCtx, cancel := context.WithCancel(context.Background())
		staleCleanupCancel = cancel
		go storyboardSvc.StartStaleCleanup(staleCtx, 2*time.Minute, 15*time.Minute)
	}

	// Handlers
	storageBaseURL := cfg.Storage.BaseURL
	characterBaseURL := cfg.Character.BaseURL
	imageBaseURL := cfg.Image.BaseURL
	videoBaseURL := cfg.Video.BaseURL
	episodeSvc.SetCharacterService(characterBaseURL, cfg.JWT.AccessSecret)
	episodeSvc.SetScriptService(cfg.Script.BaseURL)
	episodeSvc.SetVideoService(videoBaseURL)
	if resumed, resumeErr := episodeSvc.ResumeInterruptedEpisodeGeneration(20); resumeErr != nil {
		logger.Error("failed to resume interrupted episode generation", zap.Error(resumeErr))
	} else if resumed > 0 {
		logger.Info("resumed interrupted episode generation pipelines", zap.Int("count", resumed))
	}
	if resumed, resumeErr := episodeSvc.ResumeInterruptedAutoPreparation(20); resumeErr != nil {
		logger.Error("failed to resume interrupted auto preparation", zap.Error(resumeErr))
	} else if resumed > 0 {
		logger.Info("resumed interrupted auto preparation pipelines", zap.Int("count", resumed))
	}
	projectHandler := handler.NewProjectHandler(projectSvc, storageBaseURL, characterBaseURL, imageBaseURL, videoBaseURL, cfg.Script.BaseURL, cfg.JWT.AccessSecret)
	episodeHandler := handler.NewEpisodeHandler(episodeSvc)
	storyboardHandler := handler.NewStoryboardHandler(storyboardSvc, logger, characterBaseURL)
	costHandler := handler.NewCostHandler(costSvc, projectSvc)
	utilsHandler := handler.NewUtilsHandler(cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.Model)

	// HTTP server
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())
	router.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	router.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	v1 := router.Group("/api/v1")
	v1.Use(middleware.AuthMiddleware(cfg, logger))
	{
		projects := v1.Group("/projects")
		projects.GET("", projectHandler.List)
		projects.POST("", projectHandler.Create)
		projects.GET("/:id", projectHandler.Get)
		projects.PUT("/:id", projectHandler.Update)
		projects.DELETE("/:id", projectHandler.Delete)
		projects.POST("/:id/snapshot", projectHandler.CreateSnapshot)
		projects.GET("/:id/snapshots", projectHandler.ListSnapshots)
		projects.POST("/:id/pause", projectHandler.Pause)
		projects.POST("/:id/resume", projectHandler.Resume)
		projects.POST("/:id/clone", projectHandler.Clone)
		projects.GET("/:id/script/versions", projectHandler.GetScriptVersions)
		projects.POST("/:id/script/switch/:versionId", projectHandler.SwitchScriptVersion)
		projects.POST("/:id/script", projectHandler.UploadScript)
		projects.GET("/:id/cost-estimate", costHandler.GetCostEstimate)
		projects.GET("/:id/progress", projectHandler.GetProgress)

		projects.GET("/:id/episodes", episodeHandler.ListEpisodes)
		projects.POST("/:id/episodes", episodeHandler.CreateEpisode)
		projects.POST("/:id/episodes/generate", episodeHandler.GenerateEpisodes)
		projects.POST("/:id/episodes/extract-storyboards", episodeHandler.ExtractStoryboards)
		projects.PUT("/:id/episodes/:eid", episodeHandler.UpdateEpisode)
		projects.DELETE("/:id/episodes/:eid", episodeHandler.DeleteEpisode)
		projects.POST("/:id/episodes/:eid/polish", episodeHandler.PolishEpisode)
		projects.POST("/:id/episodes/:eid/extract-storyboards", episodeHandler.ExtractEpisodeStoryboards)
			projects.POST("/:id/episodes/:eid/optimize", episodeHandler.OptimizeEpisode)
			projects.POST("/:id/episodes/:eid/apply-optimized", episodeHandler.ApplyOptimizedText)
			projects.POST("/:id/episodes/:eid/review", episodeHandler.ReviewEpisode)
			projects.POST("/:id/episodes/:eid/auto-optimize-review", episodeHandler.AutoOptimizeReview)
			projects.POST("/:id/episodes/batch-optimize", episodeHandler.BatchOptimizeEpisodes)
			projects.POST("/:id/episodes/batch-review", episodeHandler.BatchReviewEpisodes)
		storyboards := projects.Group("/:id/storyboards")
		storyboards.GET("", storyboardHandler.ListStoryboards)
		storyboards.GET("/stats", storyboardHandler.Stats)
		storyboards.GET("/episode-stats", storyboardHandler.EpisodeStats)
		storyboards.GET("/export", storyboardHandler.Export)
		storyboards.POST("", storyboardHandler.CreateStoryboard)
		storyboards.POST("/generate-all", storyboardHandler.GenerateAll)
		storyboards.POST("/audit-continuity", storyboardHandler.AuditContinuity)
		storyboards.POST("/pause-generation", storyboardHandler.PauseGeneration)
		storyboards.POST("/resume-generation", storyboardHandler.ResumeGeneration)
		storyboards.PATCH("/config", storyboardHandler.UpdateConfig)
		storyboards.GET("/:sid", storyboardHandler.GetStoryboard)
		storyboards.PATCH("/:sid", storyboardHandler.UpdateStoryboard)
		storyboards.DELETE("/:sid", storyboardHandler.DeleteStoryboard)
		storyboards.POST("/:sid/generate", storyboardHandler.GenerateStoryboard)
		storyboards.POST("/:sid/switch-version", storyboardHandler.SwitchVersion)
		storyboards.DELETE("/:sid/versions/:vid", storyboardHandler.DeleteVersion)
		storyboards.POST("/:sid/void", storyboardHandler.VoidStoryboard)
		storyboards.POST("/:sid/retry", storyboardHandler.RetryStoryboard)
		storyboards.POST("/retry-failed", storyboardHandler.RetryBatch)
		storyboards.POST("/:sid/chat", storyboardHandler.ChatStoryboard)

		// utility routes
		utils := v1.Group("/utils")
		utils.POST("/translate", utilsHandler.TranslatePrompt)
	}

	httpServer := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// gRPC server
	grpcServer := grpc.NewServer()
	grpcSrv := &grpcProjectServer{projectSvc: projectSvc}
	pb.RegisterProjectServiceServer(grpcServer, grpcSrv)

	// Start HTTP
	go func() {
		logger.Info("HTTP server starting", zap.String("port", cfg.Server.Port))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server error", zap.Error(err))
		}
	}()

	// Start gRPC
	go func() {
		lis, err := net.Listen("tcp", ":"+cfg.GRPC.Port)
		if err != nil {
			logger.Fatal("failed to listen for gRPC", zap.Error(err))
		}
		logger.Info("gRPC server starting", zap.String("port", cfg.GRPC.Port))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal("gRPC server error", zap.Error(err))
		}
	}()

	// Start Kafka consumer
	kafkaCtx, kafkaCancel := context.WithCancel(context.Background())
	if kafkaConsumer != nil {
		go kafkaConsumer.Run(kafkaCtx)
	}

	// Config hot-reload watcher (Feature opt-p6)
	configPath := os.Getenv("AUTOVIDEO_CONFIG_FILE")
	if configPath == "" {
		configPath = "../../config.local.yaml"
	}
	config.StartWatcher(configPath, logger, func(newCfg *config.Config) {
		logger.Info("config file changed; API key/URL changes require service restart for project generators")
	})

	// Register with gateway for service discovery
	{
		rootCtx, rootCancel := context.WithCancel(context.Background())
		selfAddr := cfg.Gateway.SelfAddr
		if selfAddr == "" {
			selfAddr = "http://localhost:" + cfg.Server.Port
		}
		registry.Start(rootCtx, cfg.Gateway.Addr, "project", selfAddr)
		defer rootCancel()
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down servers...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("HTTP server forced to shutdown", zap.Error(err))
	}
	grpcServer.GracefulStop()

	// Stop Kafka
	if staleCleanupCancel != nil {
		staleCleanupCancel()
	}
	kafkaCancel()
	if kafkaProducer != nil {
		_ = kafkaProducer.Close()
	}
	if kafkaConsumer != nil {
		_ = kafkaConsumer.Close()
	}

	logger.Info("servers exited cleanly")
}

// initDB —— 根据 DSN 初始化 PostgreSQL 数据库连接，返回 gorm.DB
func initDB(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Info),
	})
}

// autoMigrate —— 自动迁移所有数据模型的表结构
func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.Project{},
		&model.Episode{},
		&model.ProjectSnapshot{},
		&model.ScriptVersion{},
		&model.Storyboard{},
		&model.StoryboardVersion{},
	)
}

// grpcProjectServer implements pb.ProjectServiceServer.
type grpcProjectServer struct {
	pb.UnimplementedProjectServiceServer
	projectSvc *service.ProjectService
}

// GetProject —— gRPC 接口：根据项目 ID 和用户 ID 获取单个项目信息
func (s *grpcProjectServer) GetProject(ctx context.Context, req *pb.GetProjectRequest) (*pb.ProjectInfo, error) {
	project, err := s.projectSvc.Get(req.ProjectId, req.UserId)
	if err != nil {
		if err.Error() == "project not found" {
			return nil, status.Errorf(codes.NotFound, "project not found")
		}
		return nil, status.Errorf(codes.Internal, err.Error())
	}
	return &pb.ProjectInfo{
		Id:     project.ID,
		UserId: project.UserID,
		Title:  project.Title,
		Status: project.Status,
	}, nil
}

// ListProjects —— gRPC 接口：分页获取用户的项目列表
func (s *grpcProjectServer) ListProjects(ctx context.Context, req *pb.ListProjectsRequest) (*pb.ListProjectsResponse, error) {
	page := int(req.Page)
	pageSize := int(req.PageSize)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	projects, total, err := s.projectSvc.List(service.ListProjectsReq{
		UserID:   req.UserId,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	var infos []*pb.ProjectInfo
	for _, p := range projects {
		infos = append(infos, &pb.ProjectInfo{
			Id:     p.ID,
			UserId: p.UserID,
			Title:  p.Title,
			Status: p.Status,
		})
	}
	return &pb.ListProjectsResponse{
		Projects: infos,
		Total:    total,
	}, nil
}
