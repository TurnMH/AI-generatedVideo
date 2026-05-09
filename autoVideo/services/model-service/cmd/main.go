package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/autovideo/model-service/internal/handler"
	"github.com/autovideo/model-service/internal/model"
	"github.com/autovideo/model-service/internal/repository"
	"github.com/autovideo/model-service/internal/service"
	"github.com/autovideo/model-service/pkg/config"
	"github.com/autovideo/model-service/pkg/registry"
	pb "github.com/autovideo/model-service/proto"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	grpcstatus "google.golang.org/grpc/status"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// main —— 模型服务入口，初始化数据库、Redis、HTTP/gRPC 服务器并启动后台健康检查
func main() {
	// ── Logger ────────────────────────────────────────────────────────────────
	log, _ := zap.NewProduction()
	defer log.Sync() //nolint:errcheck

	// ── Config ────────────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("load config", zap.Error(err))
	}

	// ── Database ──────────────────────────────────────────────────────────────
	db, err := gorm.Open(postgres.Open(cfg.Database.DSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatal("connect database", zap.Error(err))
	}
	if err := db.AutoMigrate(&model.Model{}, &model.ModelHealth{}, &model.UsageRecord{}); err != nil {
		log.Fatal("auto migrate", zap.Error(err))
	}

	// ── Redis ─────────────────────────────────────────────────────────────────
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	ctx := context.Background()
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		log.Warn("redis ping failed – continuing without cache", zap.Error(err))
	}

	// ── Repositories ──────────────────────────────────────────────────────────
	modelRepo := repository.NewModelRepo(db)
	usageRepo := repository.NewUsageRepo(db)

	// ── Services ──────────────────────────────────────────────────────────────
	healthSvc := service.NewHealthService(modelRepo, log)
	modelSvc := service.NewModelService(modelRepo, usageRepo, log)
	routerSvc := service.NewRouterService(modelRepo, healthSvc, rdb, log)

	// ── Background health check ───────────────────────────────────────────────
	bgCtx, bgCancel := context.WithCancel(ctx)
	defer bgCancel()
	healthSvc.StartHealthCheck(bgCtx)

	// ── HTTP server (Gin) ─────────────────────────────────────────────────────
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(requestLogger(log))

	h := handler.NewModelHandler(modelSvc, healthSvc, usageRepo, log)
	h.RegisterRoutes(router)

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	httpServer := &http.Server{
		Addr:         ":" + cfg.HTTP.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ── gRPC server ───────────────────────────────────────────────────────────
	grpcServer := grpc.NewServer()
	pb.RegisterModelServiceServer(grpcServer, &grpcModelService{
		modelRepo: modelRepo,
		usageRepo: usageRepo,
		routerSvc: routerSvc,
		log:       log,
	})
	reflection.Register(grpcServer)

	grpcLis, err := net.Listen("tcp", ":"+cfg.GRPC.Port)
	if err != nil {
		log.Fatal("gRPC listen", zap.Error(err))
	}

	// ── Start servers ─────────────────────────────────────────────────────────
	go func() {
		log.Info("HTTP server starting", zap.String("port", cfg.HTTP.Port))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP server error", zap.Error(err))
		}
	}()

	go func() {
		log.Info("gRPC server starting", zap.String("port", cfg.GRPC.Port))
		if err := grpcServer.Serve(grpcLis); err != nil {
			log.Fatal("gRPC server error", zap.Error(err))
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────

	// Register with gateway for service discovery (reuse bgCtx which is cancelled on shutdown)
	{
		selfAddr := cfg.Gateway.SelfAddr
		if selfAddr == "" {
			selfAddr = "http://localhost:" + cfg.HTTP.Port
		}
		registry.Start(bgCtx, cfg.Gateway.Addr, "model", selfAddr)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutdown signal received")

	bgCancel()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()

	if err := httpServer.Shutdown(shutCtx); err != nil {
		log.Error("HTTP shutdown error", zap.Error(err))
	}
	grpcServer.GracefulStop()

	log.Info("server exited cleanly")
}

// requestLogger —— Gin 中间件，为每个 HTTP 请求输出一条日志
// requestLogger is a simple Gin middleware that emits one log line per request.
func requestLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("http",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
		)
	}
}

// ─── gRPC service implementation ─────────────────────────────────────────────

type grpcModelService struct {
	pb.UnimplementedModelServiceServer
	modelRepo *repository.ModelRepo
	usageRepo *repository.UsageRepo
	routerSvc *service.RouterService
	log       *zap.Logger
}

// RouteModel —— gRPC 接口：根据任务类型和质量模式路由到最优模型，返回模型信息与密钥
func (g *grpcModelService) RouteModel(ctx context.Context, req *pb.RouteRequest) (*pb.RouteResponse, error) {
	result, err := g.routerSvc.Route(ctx, service.RouteRequest{
		TaskType:     req.GetTaskType(),
		QualityMode:  service.QualityMode(req.GetQualityMode()),
		UserID:       req.GetUserId(),
		ByokProvider: req.GetByokProvider(),
	})
	if err != nil {
		g.log.Warn("gRPC RouteModel", zap.Error(err))
		return nil, grpcstatus.Errorf(codes.NotFound, err.Error())
	}

	configJSON, _ := json.Marshal(result.Config)
	return &pb.RouteResponse{
		ModelId:     result.ModelID,
		ModelName:   result.ModelName,
		Provider:    result.Provider,
		ApiEndpoint: result.APIEndpoint,
		ApiKey:      result.APIKey,
		ConfigJson:  configJSON,
	}, nil
}

// RecordUsage —— gRPC 接口：记录一次模型调用的用量和费用
func (g *grpcModelService) RecordUsage(ctx context.Context, req *pb.RecordUsageRequest) (*pb.RecordUsageResponse, error) {
	usage := &model.UsageRecord{
		UserID:    req.GetUserId(),
		ModelID:   req.GetModelId(),
		TaskID:    req.GetTaskId(),
		UnitsUsed: req.GetUnitsUsed(),
		Cost:      req.GetCost(),
	}
	if err := g.usageRepo.Create(ctx, usage); err != nil {
		g.log.Error("gRPC RecordUsage", zap.Error(err))
		return &pb.RecordUsageResponse{Success: false}, grpcstatus.Errorf(codes.Internal, err.Error())
	}
	return &pb.RecordUsageResponse{Success: true}, nil
}

// GetModelInfo —— gRPC 接口：根据模型 ID 查询并返回模型基本信息
func (g *grpcModelService) GetModelInfo(ctx context.Context, req *pb.GetModelRequest) (*pb.ModelInfo, error) {
	m, err := g.modelRepo.GetByID(ctx, req.GetModelId())
	if err != nil {
		return nil, grpcstatus.Errorf(codes.NotFound, "model %d not found", req.GetModelId())
	}
	return &pb.ModelInfo{
		Id:          m.ID,
		Name:        m.Name,
		Provider:    m.Provider,
		Type:        m.Type,
		ApiEndpoint: m.APIEndpoint,
	}, nil
}
