package generators

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
)

// failoverGenerator wraps an ordered list of channel generators (each pointing at
// a distinct provider base+key) and tries them in order until one succeeds. It is
// used to make a single logical model (e.g. gpt-image-2) resilient to a channel
// being banned, rate-limited, or out of credit: when one channel fails the request
// transparently falls through to the next.
type failoverGenerator struct {
	name     string
	channels []ImageGenerator
	logger   *zap.Logger
}

// NewFailoverGenerator —— 创建多渠道故障转移生成器。channels 按优先级顺序排列，
// 第一个为主渠道；任一渠道失败（非内容审核类错误）时自动尝试下一个。
func NewFailoverGenerator(name string, channels []ImageGenerator, logger *zap.Logger) ImageGenerator {
	filtered := make([]ImageGenerator, 0, len(channels))
	for _, c := range channels {
		if c != nil {
			filtered = append(filtered, c)
		}
	}
	return &failoverGenerator{name: name, channels: filtered, logger: logger}
}

// Name —— 返回逻辑模型名（如 gpt-image-2）
func (g *failoverGenerator) Name() string { return g.name }

// IsAvailable —— 任一子渠道可用即视为可用
func (g *failoverGenerator) IsAvailable(ctx context.Context) bool {
	for _, c := range g.channels {
		if c.IsAvailable(ctx) {
			return true
		}
	}
	return false
}

// RefCapability —— 以主渠道的参考图能力为准（同一模型在不同渠道能力一致）
func (g *failoverGenerator) RefCapability() RefCapability {
	if len(g.channels) > 0 {
		return g.channels[0].RefCapability()
	}
	return RefCapability{Mode: RefModeT2I}
}

// Generate —— 依次尝试各渠道，返回首个成功结果。内容审核类错误直接返回（换渠道无意义）。
func (g *failoverGenerator) Generate(ctx context.Context, req GenerateReq) (*GenerateRes, error) {
	var lastErr error
	tried := 0
	for i, c := range g.channels {
		if !c.IsAvailable(ctx) {
			continue
		}
		tried++
		res, err := c.Generate(ctx, req)
		if err == nil {
			if i > 0 {
				g.logger.Info("failover: succeeded on backup channel",
					zap.String("model", g.name), zap.Int("channel_index", i))
			}
			return res, nil
		}
		// Content moderation rejects the prompt itself — other channels will reject
		// it too, so stop early and surface the moderation error to the caller.
		if isContentModerationError(err) {
			return nil, err
		}
		lastErr = err
		g.logger.Warn("failover: channel failed, trying next",
			zap.String("model", g.name), zap.Int("channel_index", i), zap.Error(err))
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("failover: no available channel for %s (tried %d)", g.name, tried)
	}
	return nil, lastErr
}

func isContentModerationError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "content moderation") ||
		strings.Contains(msg, "moderation_blocked") ||
		strings.Contains(msg, "safety_violations")
}
