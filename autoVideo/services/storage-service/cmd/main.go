package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/autovideo/storage-service/internal/driver"
	"github.com/autovideo/storage-service/internal/handler"
	"github.com/autovideo/storage-service/internal/model"
	"github.com/autovideo/storage-service/internal/repository"
	"github.com/autovideo/storage-service/internal/service"
	"github.com/autovideo/storage-service/pkg/config"
	"github.com/autovideo/storage-service/pkg/registry"
	storagepb "github.com/autovideo/storage-service/proto"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// main —— 程序入口，初始化数据库、存储驱动、HTTP/gRPC 服务器并等待优雅关闭
func main() {
	log, _ := zap.NewProduction()
	defer log.Sync() //nolint:errcheck

	cfg, err := config.Load("")
	if err != nil {
		log.Fatal("failed to load config", zap.Error(err))
	}

	// ── Database ────────────────────────────────────────────────────────────
	db, err := gorm.Open(postgres.Open(cfg.Database.DSN), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database", zap.Error(err))
	}
	if err := db.AutoMigrate(&model.File{}); err != nil {
		log.Fatal("AutoMigrate failed", zap.Error(err))
	}

	// ── Storage driver ──────────────────────────────────────────────────────
	var storageDriver driver.StorageDriver
	switch cfg.Storage.Driver {
	case "minio":
		mc := cfg.Storage.Minio
		publicRead := cfg.Storage.CdnBaseURL != "" && !strings.HasPrefix(cfg.Storage.CdnBaseURL, "http://localhost")
		minioDriver, err := driver.NewMinioDriver(mc.Endpoint, mc.AccessKey, mc.SecretKey, mc.UseSSL, mc.PathStyle, publicRead)
		if err != nil {
			log.Fatal("failed to create minio driver", zap.Error(err))
		}
		storageDriver = minioDriver
		// 启动时为所有唯一物理 bucket 设置公开读策略，确保 CDN 可以无认证访问对象
		// 仅在配置了 CDN（非本地 localhost）时执行，避免影响本地开发环境
		if cfg.Storage.CdnBaseURL != "" && !strings.HasPrefix(cfg.Storage.CdnBaseURL, "http://localhost") {
			seen := map[string]struct{}{}
			b := cfg.Storage.Buckets
			for _, bkt := range []string{b.Images, b.Videos, b.Scripts, b.Characters, b.Uploads, b.Exports, b.Dubbing, b.Audios} {
				if bkt == "" {
					continue
				}
				if _, ok := seen[bkt]; ok {
					continue
				}
				seen[bkt] = struct{}{}
				if pErr := minioDriver.SetBucketPublicRead(context.Background(), bkt); pErr != nil {
					log.Warn("SetBucketPublicRead failed (bucket may already be public or permissions missing)", zap.String("bucket", bkt), zap.Error(pErr))
				} else {
					log.Info("bucket public-read policy applied", zap.String("bucket", bkt))
				}
			}
		}
	default:
		storageDriver = driver.NewLocalDriver(cfg.Storage.Local.BasePath, cfg.Server.Port)
	}

	// ── Wire dependencies ───────────────────────────────────────────────────
	fileRepo := repository.NewFileRepo(db)

	// Build logical→physical bucket name map from config.
	// When all buckets point to the same physical bucket (TOS/S3 single-bucket),
	// callers keep using logical names ("videos", "images", etc.) and storage-service
	// transparently routes them to the correct physical bucket.
	b := cfg.Storage.Buckets
	bucketMap := map[string]string{
		"images":     b.Images,
		"videos":     b.Videos,
		"scripts":    b.Scripts,
		"characters": b.Characters,
		"uploads":    b.Uploads,
		"exports":    b.Exports,
		"dubbing":    b.Dubbing,
		"audios":     b.Audios,
	}
	svc := service.NewStorageService(storageDriver, fileRepo, cfg.Storage.CdnBaseURL, bucketMap, cfg.Storage.CDNIncludeBucket)
	projectStorageSvc := service.NewProjectStorageService(fileRepo, storageDriver)

	// ── HTTP server ─────────────────────────────────────────────────────────
	gin.SetMode(cfg.Server.Mode)
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())
	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	// Serve local static files when using local driver.
	if cfg.Storage.Driver != "minio" {
		r.Static("/static", cfg.Storage.Local.BasePath)
	}

	h := handler.NewStorageHandler(svc)
	api := r.Group("/api/v1/storage")
	h.RegisterRoutes(api)

	// Project storage endpoints
	projectStorageHandler := handler.NewProjectStorageHandler(projectStorageSvc)
	v1 := r.Group("/api/v1")
	// Bulk totals (no :pid — must come before the :pid group)
	v1.GET("/storage/projects/totals", projectStorageHandler.GetBulkTotals)
	projectStorage := v1.Group("/projects/:pid/storage")
	projectStorage.GET("", projectStorageHandler.GetDetails)
	projectStorage.DELETE("", projectStorageHandler.DeleteProjectFiles)
	projectStorage.DELETE("/files", projectStorageHandler.DeleteFiles)
	projectStorage.POST("/clean-history", projectStorageHandler.CleanHistory)

	httpSrv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ── gRPC server ─────────────────────────────────────────────────────────
	grpcSrv := grpc.NewServer()
	grpcServer := storagepb.NewGRPCServer(svc)

	// grpcServer holds the StorageService implementation.
	// Without protoc-generated code, we expose it for future registration.
	_ = grpcServer

	grpcLis, err := net.Listen("tcp", ":"+cfg.GRPC.Port)
	if err != nil {
		log.Fatal("failed to listen gRPC", zap.Error(err))
	}

	// ── Graceful shutdown ────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("HTTP server starting", zap.String("port", cfg.Server.Port))
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP server error", zap.Error(err))
		}
	}()

	go func() {
		log.Info("gRPC server starting", zap.String("port", cfg.GRPC.Port))
		if err := grpcSrv.Serve(grpcLis); err != nil {
			log.Fatal("gRPC server error", zap.Error(err))
		}
	}()

	// Register with gateway for service discovery
	{
		rootCtx, rootCancel := context.WithCancel(context.Background())
		selfAddr := cfg.Gateway.SelfAddr
		if selfAddr == "" {
			selfAddr = "http://localhost:" + cfg.Server.Port
		}
		registry.Start(rootCtx, cfg.Gateway.Addr, "storage", selfAddr)
		defer rootCancel()
	}

	<-quit
	log.Info("shutting down servers...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Error("HTTP shutdown error", zap.Error(err))
	}
	grpcSrv.GracefulStop()

	sqlDB, _ := db.DB()
	if sqlDB != nil {
		sqlDB.Close()
	}

	fmt.Println("storage-service exited")
}
