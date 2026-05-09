package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/autovideo/model-service/internal/model"
	"github.com/autovideo/model-service/internal/repository"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// QualityMode selects the routing strategy.
type QualityMode string

const (
	QualityPriority QualityMode = "quality"  // highest-capability models
	SpeedPriority   QualityMode = "speed"    // lowest-latency models
	CostPriority    QualityMode = "cost"     // cheapest per-unit
	Domestic        QualityMode = "domestic" // Chinese-cloud providers only
	Custom          QualityMode = "custom"   // caller supplies explicit provider via ByokProvider
)

// RouteRequest carries the selection criteria for choosing a model.
type RouteRequest struct {
	TaskType     string      // llm / image / video / audio
	QualityMode  QualityMode
	UserID       uint64
	ByokProvider string // user-supplied provider name (BYOK)
}

// RouteResult is the selected model together with its resolved credentials.
type RouteResult struct {
	ModelID     uint64
	ModelName   string
	Provider    string
	APIEndpoint string
	APIKey      string
	Config      map[string]interface{}
}

// RouterService selects the best available model for a given task.
type RouterService struct {
	modelRepo     *repository.ModelRepo
	healthService *HealthService
	redis         *redis.Client
	logger        *zap.Logger
}

// NewRouterService —— 创建 RouterService 实例，注入仓库、健康服务、Redis 和日志
// NewRouterService constructs a RouterService.
func NewRouterService(
	modelRepo *repository.ModelRepo,
	healthService *HealthService,
	rdb *redis.Client,
	logger *zap.Logger,
) *RouterService {
	return &RouterService{
		modelRepo:     modelRepo,
		healthService: healthService,
		redis:         rdb,
		logger:        logger,
	}
}

// Route —— 根据任务类型、质量模式和供应商筛选最优可用模型，返回模型信息及 API 密钥
// Route selects the optimal model according to the routing rules:
//  1. If ByokProvider is set, restrict candidates to that provider.
//  2. Apply QualityMode filter.
//  3. Exclude models whose circuit breaker is open.
//  4. Return the highest-priority surviving candidate.
//  5. Resolve the API key (Redis cache → config fallback).
func (r *RouterService) Route(ctx context.Context, req RouteRequest) (*RouteResult, error) {
	candidates, err := r.modelRepo.GetActiveByType(ctx, req.TaskType)
	if err != nil {
		return nil, fmt.Errorf("router: fetch candidates: %w", err)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("router: no active models for task_type=%s", req.TaskType)
	}

	// Step 1 – BYOK provider filter.
	if req.ByokProvider != "" {
		filtered := filterByProvider(candidates, req.ByokProvider)
		if len(filtered) > 0 {
			candidates = filtered
		}
	}

	// Step 2 – quality-mode filter.
	candidates = r.filterByMode(candidates, req.QualityMode)

	// Step 3 – circuit-breaker filter.
	available := make([]*model.Model, 0, len(candidates))
	for _, m := range candidates {
		if r.healthService.GetBreaker(m.ID).Allow() {
			available = append(available, m)
		}
	}
	if len(available) == 0 {
		return nil, fmt.Errorf("router: all models for task_type=%s mode=%s are circuit-broken",
			req.TaskType, req.QualityMode)
	}

	// Step 4 – highest priority (already sorted by repo).
	selected := available[0]

	// Step 5 – resolve API key.
	apiKey, err := r.getAPIKey(ctx, selected)
	if err != nil {
		r.logger.Warn("router: api key resolution failed",
			zap.Uint64("model_id", selected.ID), zap.Error(err))
	}

	var configMap map[string]interface{}
	if len(selected.Config) > 0 {
		_ = json.Unmarshal(selected.Config, &configMap)
	}

	r.logger.Info("router: selected model",
		zap.Uint64("model_id", selected.ID),
		zap.String("name", selected.Name),
		zap.String("provider", selected.Provider),
	)

	return &RouteResult{
		ModelID:     selected.ID,
		ModelName:   selected.Name,
		Provider:    selected.Provider,
		APIEndpoint: selected.APIEndpoint,
		APIKey:      apiKey,
		Config:      configMap,
	}, nil
}

// filterByProvider —— 过滤并返回指定供应商的模型列表
// filterByProvider keeps only models from the given provider.
func filterByProvider(models []*model.Model, provider string) []*model.Model {
	out := make([]*model.Model, 0, len(models))
	for _, m := range models {
		if m.Provider == provider {
			out = append(out, m)
		}
	}
	return out
}

// domesticProviders is the set of Chinese-cloud AI providers.
var domesticProviders = map[string]bool{
	"aliyun":    true,
	"kuaishou":  true,
	"bytedance": true,
	"deepseek":  true,
	"baidu":     true,
	"zhipu":     true,
}

// filterByMode —— 按质量模式（质量优先/速度优先/成本优先/国内/自定义）筛选候选模型
// filterByMode applies quality-mode–specific filtering.
// If the filter produces an empty set the original list is returned unchanged
// so we always have at least one candidate.
func (r *RouterService) filterByMode(models []*model.Model, mode QualityMode) []*model.Model {
	var filtered []*model.Model

	switch mode {
	case QualityPriority:
		// High-capability: keep models with priority >= 8.
		for _, m := range models {
			if m.Priority >= 8 {
				filtered = append(filtered, m)
			}
		}

	case SpeedPriority:
		// Low-latency proxy: keep priority >= 5.
		for _, m := range models {
			if m.Priority >= 5 {
				filtered = append(filtered, m)
			}
		}

	case CostPriority:
		// Cheapest first: cost_per_unit < 0.01 or local provider.
		for _, m := range models {
			if m.CostPerUnit < 0.01 || m.Provider == "local" {
				filtered = append(filtered, m)
			}
		}

	case Domestic:
		for _, m := range models {
			if domesticProviders[m.Provider] {
				filtered = append(filtered, m)
			}
		}

	case Custom:
		// Custom mode: no additional filtering; rely on ByokProvider set upstream.
		return models
	}

	if len(filtered) > 0 {
		return filtered
	}
	return models // fallback: all candidates
}

// getAPIKey —— 先从 Redis 缓存获取模型 API 密钥，未命中则回退到 JSONB 配置字段
// getAPIKey retrieves the API key for a model, checking Redis first.
// In production this would call a secrets-manager/KMS; here we read from config.
func (r *RouterService) getAPIKey(ctx context.Context, m *model.Model) (string, error) {
	cacheKey := fmt.Sprintf("model:apikey:%d", m.ID)

	val, err := r.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		return val, nil
	}

	// Fallback: read from model's JSONB config field.
	var cfg map[string]interface{}
	if len(m.Config) > 0 {
		if jsonErr := json.Unmarshal(m.Config, &cfg); jsonErr == nil {
			if key, ok := cfg["api_key"].(string); ok && key != "" {
				// Cache for 5 minutes to avoid repeated JSON parsing.
				_ = r.redis.Set(ctx, cacheKey, key, 5*time.Minute).Err()
				return key, nil
			}
		}
	}

	return "", nil
}
