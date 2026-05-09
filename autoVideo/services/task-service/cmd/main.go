package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/autovideo/task-service/internal/handler"
	"github.com/autovideo/task-service/internal/hub"
	"github.com/autovideo/task-service/internal/model"
	"github.com/autovideo/task-service/internal/repository"
	"github.com/autovideo/task-service/internal/service"
	"github.com/autovideo/task-service/pkg/config"
	"github.com/autovideo/task-service/pkg/registry"
	taskproto "github.com/autovideo/task-service/proto"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// main —— 任务服务入口，初始化数据库、Kafka、WebSocket Hub、HTTP/gRPC 服务器并启动后台任务
func main() {
	// ── logger ───────────────────────────────────────────────────────────────
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// ── config ───────────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("load config", zap.Error(err))
	}

	// ── database ─────────────────────────────────────────────────────────────
	db, err := gorm.Open(postgres.Open(cfg.Database.DSN), &gorm.Config{})
	if err != nil {
		logger.Fatal("connect database", zap.Error(err))
	}
	if err := db.AutoMigrate(&model.Task{}, &model.TaskProgress{}); err != nil {
		logger.Fatal("auto-migrate", zap.Error(err))
	}

	// ── repositories ─────────────────────────────────────────────────────────
	taskRepo := repository.NewTaskRepo(db)
	progRepo := repository.NewProgressRepo(db)

	// ── websocket hub ─────────────────────────────────────────────────────────
	wsHub := hub.NewHub(logger)
	go wsHub.Run()

	// ── kafka service ─────────────────────────────────────────────────────────
	kafkaSvc := service.NewKafkaService(cfg.Kafka.Brokers, logger)
	defer kafkaSvc.Close()

	// ── task service ──────────────────────────────────────────────────────────
	taskSvc := service.NewTaskService(taskRepo, progRepo, kafkaSvc, wsHub, logger)

	// ── handlers ─────────────────────────────────────────────────────────────
	taskHandler := handler.NewTaskHandler(taskSvc, logger)
	wsHandler := handler.NewWSHandler(wsHub, logger, cfg.JWT.AccessSecret)

	// ── root context with graceful-shutdown ───────────────────────────────────
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	// ── kafka consumers ───────────────────────────────────────────────────────
	resultTopics := []string{
		service.TopicScriptAnalyzeRes,
		service.TopicScriptQuickGenerateRes,
		service.TopicImageGenerateRes,
		service.TopicVideoGenerateRes,
		service.TopicMusicGenerateRes,
		service.TopicTaskCompleted,
		service.TopicTaskFailed,
		service.TopicTaskProgress,
	}
	for _, topic := range resultTopics {
		topic := topic
		kafkaSvc.StartConsumer(ctx, topic, "task-service-group", func(data []byte) error {
			return handleKafkaMessage(ctx, topic, data, taskSvc, logger)
		})
	}

	// ── background jobs ───────────────────────────────────────────────────────
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				taskSvc.RetryFailedTasks(ctx)
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				taskSvc.TimeoutCheck(ctx)
			}
		}
	}()

	// ── HTTP server ───────────────────────────────────────────────────────────
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(ginZapLogger(logger))
	router.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	router.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	api := router.Group("/api/v1")
	{
		tasks := api.Group("/tasks")
		tasks.POST("", taskHandler.CreateTask)
		tasks.GET("", taskHandler.ListTasks)
		tasks.GET("/:id", taskHandler.GetTask)
		tasks.POST("/:id/cancel", taskHandler.CancelTask)
		tasks.GET("/:id/progress", taskHandler.GetProgress)
	}

	ws := router.Group("/ws")
	{
		ws.GET("/tasks/:id", wsHandler.SubscribeTask)
		ws.GET("/projects/:project_id", wsHandler.SubscribeProject)
	}

	httpServer := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("HTTP server starting", zap.String("port", cfg.Server.Port))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server error", zap.Error(err))
		}
	}()

	// ── gRPC server ───────────────────────────────────────────────────────────
	grpcServer := grpc.NewServer()
	grpcImpl := taskproto.NewGRPCServer(taskSvc, logger)
	taskproto.RegisterTaskServiceServer(grpcServer, grpcImpl)

	wg.Add(1)
	go func() {
		defer wg.Done()
		lis, err := net.Listen("tcp", ":"+cfg.GRPC.Port)
		if err != nil {
			logger.Fatal("gRPC listen error", zap.Error(err))
		}
		logger.Info("gRPC server starting", zap.String("port", cfg.GRPC.Port))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("gRPC server error", zap.Error(err))
		}
	}()

	// ── wait for signal ───────────────────────────────────────────────────────

	// Register with gateway for service discovery (reuse kafka ctx which is cancelled on shutdown)
	{
		selfAddr := cfg.Gateway.SelfAddr
		if selfAddr == "" {
			selfAddr = "http://localhost:" + cfg.Server.Port
		}
		registry.Start(ctx, cfg.Gateway.Addr, "task", selfAddr)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down...")
	cancel()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	if err := httpServer.Shutdown(shutCtx); err != nil {
		logger.Error("HTTP shutdown error", zap.Error(err))
	}
	grpcServer.GracefulStop()

	wg.Wait()
	logger.Info("task-service stopped")
}

// handleKafkaMessage —— 将收到的 Kafka 结果消息路由到对应的 TaskService 方法进行处理
// handleKafkaMessage routes incoming Kafka result messages to the appropriate TaskService method.
func handleKafkaMessage(ctx context.Context, topic string, data []byte, svc *service.TaskService, logger *zap.Logger) error {
	var msg struct {
		TaskID   uint64          `json:"task_id"`
		Status   string          `json:"status"`
		ErrorMsg string          `json:"error_msg"`
		Progress int             `json:"progress"`
		Message  string          `json:"message"`
		Result   json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		logger.Warn("kafka message parse error", zap.String("topic", topic), zap.Error(err))
		return nil // don't retry unparseable messages
	}

	switch topic {
	case service.TopicTaskProgress:
		return svc.UpdateProgress(msg.TaskID, msg.Progress, msg.Message)
	case service.TopicScriptQuickGenerateRes:
		if msg.ErrorMsg != "" {
			return svc.UpdateStatus(msg.TaskID, model.TaskFailed, msg.ErrorMsg)
		}
		if len(msg.Result) > 0 {
			return svc.UpdateResult(msg.TaskID, msg.Result)
		}
		// 空 result 也算完成（罕见，但不应当标失败）
		return svc.UpdateStatus(msg.TaskID, model.TaskSucceeded, "")
	case service.TopicTaskCompleted, service.TopicScriptAnalyzeRes, service.TopicImageGenerateRes, service.TopicVideoGenerateRes, service.TopicMusicGenerateRes:
		return svc.UpdateStatus(msg.TaskID, model.TaskSucceeded, "")
	case service.TopicTaskFailed:
		return svc.UpdateStatus(msg.TaskID, model.TaskFailed, msg.ErrorMsg)
	default:
		logger.Debug("unhandled topic", zap.String("topic", topic))
	}
	return nil
}

// ginZapLogger —— Gin 中间件，使用 zap 为每个 HTTP 请求记录日志
// ginZapLogger is a minimal Gin middleware that logs with zap.
func ginZapLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info("http",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
		)
	}
}
