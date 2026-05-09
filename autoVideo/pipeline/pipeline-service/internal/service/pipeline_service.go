package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/autovideo/pipeline-service/internal/model"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	redisPipelineTTL = 48 * time.Hour
	maxLogLines      = 50
)

// PipelineService 核心状态机服务
type PipelineService struct {
	redis      *redis.Client
	httpClient *HTTPClient
	logger     *zap.Logger
	pauseChs   sync.Map // pipeline_id -> chan bool  (true=pause, false=resume)
	abortChs   sync.Map // pipeline_id -> chan struct{}
}

// NewPipelineService —— 创建流水线服务实例，注入 Redis、HTTP 客户端和日志
func NewPipelineService(rdb *redis.Client, httpClient *HTTPClient, logger *zap.Logger) *PipelineService {
	return &PipelineService{
		redis:      rdb,
		httpClient: httpClient,
		logger:     logger,
	}
}

// ─── 公开控制方法 ──────────────────────────────────────────────────────────────

// Start 将 state 持久化并启动后台 goroutine
func (s *PipelineService) Start(ctx context.Context, state *model.PipelineState) {
	// 初始化控制 channel
	pauseCh := make(chan bool, 4)
	abortCh := make(chan struct{})
	s.pauseChs.Store(state.ID, pauseCh)
	s.abortChs.Store(state.ID, abortCh)

	state.Stage = model.StageInit
	state.Status = model.StatusRunning
	s.saveState(state)

	go s.run(context.Background(), state)
}

// Pause 暂停流水线
func (s *PipelineService) Pause(id string) error {
	v, ok := s.pauseChs.Load(id)
	if !ok {
		return fmt.Errorf("pipeline %s not found or already finished", id)
	}
	v.(chan bool) <- true
	return nil
}

// Resume 恢复流水线
func (s *PipelineService) Resume(id string) error {
	v, ok := s.pauseChs.Load(id)
	if !ok {
		return fmt.Errorf("pipeline %s not found or already finished", id)
	}
	v.(chan bool) <- false
	return nil
}

// Abort 终止流水线
func (s *PipelineService) Abort(id string) error {
	v, ok := s.abortChs.Load(id)
	if !ok {
		return fmt.Errorf("pipeline %s not found or already finished", id)
	}
	ch := v.(chan struct{})
	// 关闭 channel 触发终止（幂等保护）
	select {
	case <-ch:
		// 已经关闭
	default:
		close(ch)
	}
	return nil
}

// GetState 从 Redis 读取流水线状态
func (s *PipelineService) GetState(ctx context.Context, id string) (*model.PipelineState, error) {
	data, err := s.redis.Get(ctx, redisKey(id)).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("pipeline %s not found", id)
	}
	if err != nil {
		return nil, err
	}
	var state model.PipelineState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// ─── 内部状态机 ───────────────────────────────────────────────────────────────

// run —— 流水线状态机主循环，依次执行各阶段并处理暂停与中止信号
func (s *PipelineService) run(ctx context.Context, state *model.PipelineState) {
	defer func() {
		// 清理控制 channel
		s.pauseChs.Delete(state.ID)
		s.abortChs.Delete(state.ID)
	}()

	stages := []string{
		model.StageScriptAnalyzing,
		model.StageAssetsExtracting,
		model.StageStoryboardGen,
		model.StageImagesGenerating,
		model.StageImagesReviewing,
		model.StageVideosGenerating,
		model.StageComposing,
		model.StageAutoFixing,
	}

	for _, stage := range stages {
		if s.isAborted(state.ID) {
			state.Status = model.StatusAborted
			s.appendLog(state, fmt.Sprintf("[%s] 流水线已中止", stage))
			s.saveState(state)
			return
		}
		if err := s.waitIfPaused(ctx, state); err != nil {
			// abort 信号在等待暂停期间到来
			state.Status = model.StatusAborted
			s.saveState(state)
			return
		}

		state.Stage = stage
		state.Status = model.StatusRunning
		s.appendLog(state, fmt.Sprintf("[%s] 开始执行", stage))
		s.saveState(state)

		s.logger.Info("pipeline stage started", zap.String("pipeline_id", state.ID), zap.String("stage", stage))

		var err error
		switch stage {
		case model.StageScriptAnalyzing:
			err = s.runScriptAnalyzing(ctx, state)
		case model.StageAssetsExtracting:
			err = s.runAssetsExtracting(ctx, state)
		case model.StageStoryboardGen:
			err = s.runStoryboardGen(ctx, state)
		case model.StageImagesGenerating:
			err = s.runImagesGenerating(ctx, state)
		case model.StageImagesReviewing:
			err = s.runImagesReviewing(ctx, state)
		case model.StageVideosGenerating:
			err = s.runVideosGenerating(ctx, state)
		case model.StageComposing:
			err = s.runComposing(ctx, state)
		case model.StageAutoFixing:
			err = s.runAutoFixing(ctx, state)
		}

		if err != nil {
			state.Status = model.StatusFailed
			s.appendLog(state, fmt.Sprintf("[%s] 执行失败: %v", stage, err))
			s.saveState(state)
			s.logger.Error("pipeline stage failed",
				zap.String("pipeline_id", state.ID),
				zap.String("stage", stage),
				zap.Error(err),
			)
			return
		}

		s.appendLog(state, fmt.Sprintf("[%s] 完成 ✓", stage))
		s.saveState(state)
		s.logger.Info("pipeline stage done", zap.String("pipeline_id", state.ID), zap.String("stage", stage))
	}

	state.Stage = model.StageDone
	state.Status = model.StatusDone
	state.Progress = 100
	s.saveState(state)
	s.logger.Info("pipeline completed", zap.String("pipeline_id", state.ID))
}

// ─── 暂停/终止检测 ────────────────────────────────────────────────────────────

// isAborted —— 检查指定流水线是否已收到终止信号，返回布尔值
func (s *PipelineService) isAborted(id string) bool {
	v, ok := s.abortChs.Load(id)
	if !ok {
		return false
	}
	ch := v.(chan struct{})
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// waitIfPaused 阻塞直到收到 resume 信号；若在等待期间收到 abort 则返回 error
func (s *PipelineService) waitIfPaused(ctx context.Context, state *model.PipelineState) error {
	v, ok := s.pauseChs.Load(state.ID)
	if !ok {
		return nil
	}
	pauseCh := v.(chan bool)

	// 非阻塞检查是否有暂停信号
	select {
	case pause := <-pauseCh:
		if !pause {
			return nil // resume 信号，但我们没暂停，忽略
		}
	default:
		return nil // 没有信号，继续执行
	}

	// 收到暂停信号，更新状态并等待恢复
	state.Status = model.StatusPaused
	s.appendLog(state, "[PAUSE] 流水线已暂停，等待恢复...")
	s.saveState(state)
	s.logger.Info("pipeline paused", zap.String("pipeline_id", state.ID))

	abortV, _ := s.abortChs.Load(state.ID)
	abortCh := abortV.(chan struct{})

	for {
		select {
		case <-abortCh:
			return fmt.Errorf("aborted while paused")
		case <-ctx.Done():
			return ctx.Err()
		case resume := <-pauseCh:
			if !resume {
				state.Status = model.StatusRunning
				s.appendLog(state, "[RESUME] 流水线恢复运行")
				s.saveState(state)
				s.logger.Info("pipeline resumed", zap.String("pipeline_id", state.ID))
				return nil
			}
			// 重复收到暂停信号，忽略
		}
	}
}

// ─── Redis 辅助 ───────────────────────────────────────────────────────────────

// saveState —— 将流水线状态序列化后保存到 Redis，设置 TTL
func (s *PipelineService) saveState(state *model.PipelineState) {
	state.UpdatedAt = time.Now()
	data, err := json.Marshal(state)
	if err != nil {
		s.logger.Error("marshal pipeline state", zap.Error(err))
		return
	}
	if err := s.redis.Set(context.Background(), redisKey(state.ID), data, redisPipelineTTL).Err(); err != nil {
		s.logger.Error("save pipeline state to redis", zap.Error(err))
	}
}

// appendLog —— 向流水线状态追加一条带时间戳的日志，超过上限时裁剪旧日志
func (s *PipelineService) appendLog(state *model.PipelineState, msg string) {
	entry := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg)
	state.Logs = append(state.Logs, entry)
	if len(state.Logs) > maxLogLines {
		state.Logs = state.Logs[len(state.Logs)-maxLogLines:]
	}
}

// redisKey —— 根据流水线 ID 生成 Redis 存储键名
func redisKey(id string) string {
	return "pipeline:" + id
}
