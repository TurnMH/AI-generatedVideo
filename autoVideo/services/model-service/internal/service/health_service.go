package service

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/autovideo/model-service/internal/model"
	"github.com/autovideo/model-service/internal/repository"
	"github.com/autovideo/model-service/pkg/breaker"
	"go.uber.org/zap"
)

// HealthService runs background health probes against all active models.
type HealthService struct {
	modelRepo *repository.ModelRepo
	breakers  map[uint64]*breaker.Breaker
	mu        sync.RWMutex
	logger    *zap.Logger
	client    *http.Client
}

// NewHealthService —— 创建 HealthService 实例，注入模型仓库和日志
// NewHealthService constructs a HealthService.
func NewHealthService(modelRepo *repository.ModelRepo, logger *zap.Logger) *HealthService {
	return &HealthService{
		modelRepo: modelRepo,
		breakers:  make(map[uint64]*breaker.Breaker),
		logger:    logger,
		client:    &http.Client{Timeout: 3 * time.Second},
	}
}

// GetBreaker —— 获取指定模型的熔断器，不存在则自动创建
// GetBreaker returns (creating if needed) the circuit breaker for a model.
func (h *HealthService) GetBreaker(modelID uint64) *breaker.Breaker {
	h.mu.RLock()
	b, ok := h.breakers[modelID]
	h.mu.RUnlock()
	if ok {
		return b
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	// Double-check after acquiring write lock.
	if b, ok = h.breakers[modelID]; ok {
		return b
	}
	b = breaker.New(5, 60*time.Second)
	h.breakers[modelID] = b
	return b
}

// StartHealthCheck —— 启动后台协程，每 30 秒对所有活跃模型执行健康探测
// StartHealthCheck launches a background goroutine that probes all active
// models every 30 seconds until ctx is cancelled.
func (h *HealthService) StartHealthCheck(ctx context.Context) {
	go func() {
		// Run immediately on start, then every 30 s.
		h.checkAll(ctx)

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.checkAll(ctx)
			}
		}
	}()
}

// checkAll —— 并发探测所有活跃模型的健康状态
func (h *HealthService) checkAll(ctx context.Context) {
	models, err := h.modelRepo.GetAllActive(ctx)
	if err != nil {
		h.logger.Error("health-check: failed to fetch active models", zap.Error(err))
		return
	}

	var wg sync.WaitGroup
	for _, m := range models {
		wg.Add(1)
		go func(m *model.Model) {
			defer wg.Done()
			if err := h.CheckModel(ctx, m); err != nil {
				h.logger.Warn("health-check probe failed",
					zap.Uint64("model_id", m.ID),
					zap.String("name", m.Name),
					zap.Error(err),
				)
			}
		}(m)
	}
	wg.Wait()
}

// CheckModel —— 对单个模型执行 HTTP HEAD 健康探测，更新熔断器并写入健康记录
// CheckModel performs a single health probe against the model's API endpoint.
// It updates the model_health table and the model's circuit breaker.
func (h *HealthService) CheckModel(ctx context.Context, m *model.Model) error {
	status := "unknown"
	var latencyMs int64

	if m.APIEndpoint != "" {
		start := time.Now()
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, m.APIEndpoint, nil)
		if err != nil {
			status = "unhealthy"
			h.GetBreaker(m.ID).OnFailure()
		} else {
			resp, err := h.client.Do(req)
			latencyMs = time.Since(start).Milliseconds()
			if err != nil {
				status = "unhealthy"
				h.GetBreaker(m.ID).OnFailure()
			} else {
				resp.Body.Close()
				// Treat any non-5xx status as healthy (API gateways often return 401/403 for HEAD).
				if resp.StatusCode >= 500 {
					status = "unhealthy"
					h.GetBreaker(m.ID).OnFailure()
				} else {
					status = "healthy"
					h.GetBreaker(m.ID).OnSuccess()
				}
			}
		}
	} else {
		// No endpoint configured – mark as healthy (local model or config-only).
		status = "healthy"
		h.GetBreaker(m.ID).OnSuccess()
	}

	health := &model.ModelHealth{
		ModelID:   m.ID,
		Status:    status,
		LatencyMs: latencyMs,
		CheckedAt: time.Now(),
	}

	h.logger.Debug("health-check result",
		zap.Uint64("model_id", m.ID),
		zap.String("status", status),
		zap.Int64("latency_ms", latencyMs),
	)
	return h.modelRepo.SaveHealth(ctx, health)
}
