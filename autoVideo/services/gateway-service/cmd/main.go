package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/autovideo/gateway-service/internal"
	"go.uber.org/zap"
)

func main() {
	configPath := flag.String("config", "config.local.yaml", "path to config file")
	flag.Parse()

	// Logger
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync() //nolint:errcheck

	// Config
	cfg, err := internal.LoadConfig(*configPath)
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	// Registry for dynamic service discovery
	registry := internal.NewRegistry()

	// Gateway
	gw, err := internal.NewGateway(cfg, logger, registry)
	if err != nil {
		logger.Fatal("failed to build gateway", zap.Error(err))
	}

	addr := fmt.Sprintf(":%d", cfg.Port)

	// Wrap the gateway in a mux so /_internal/ routes are handled before the
	// main Gateway (which would 401 them). These routes are unauthenticated;
	// in production they must be firewall-protected (internal traffic only).
	mux := http.NewServeMux()
	mux.HandleFunc("/_internal/register", registry.HandleRegister)
	mux.HandleFunc("/_internal/services", registry.HandleList)
	mux.Handle("/", gw)

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0, // streaming/long-polling — let per-route transport timeout handle it
		IdleTimeout:  90 * time.Second,
	}

	// Graceful shutdown on SIGINT / SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("gateway starting", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("listen error", zap.Error(err))
		}
	}()

	<-quit
	logger.Info("shutting down gateway…")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
	logger.Info("gateway stopped")
}
