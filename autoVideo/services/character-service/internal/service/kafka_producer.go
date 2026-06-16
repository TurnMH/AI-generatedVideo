package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// AssetGenerateRequest is published to Kafka to trigger image generation.
type AssetGenerateRequest struct {
	AssetID      uint64 `json:"asset_id"`
	ProjectID    uint64 `json:"project_id"`
	Type         string `json:"type"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Prompt       string `json:"prompt"`
	PromptSuffix string `json:"prompt_suffix,omitempty"` // extra text appended after style-aware composition
	StylePreset  string `json:"style_preset,omitempty"`  // overrides project-level style when set
	ModelName    string `json:"model_name,omitempty"`
	// RawPrompt: 当为 true 且 Prompt 非空时，消费端把 Prompt 当作用户手工锁定的「最终提示词」直接使用，
	// 跳过 LLM 精炼 / 套版面模板 / 视觉 hint 拼接 / 最终润色，完全由用户掌控成图提示词。
	RawPrompt bool `json:"raw_prompt,omitempty"`
}

// AssetGenerateResult is published to Kafka after generation completes or fails.
type AssetGenerateResult struct {
	AssetID  uint64 `json:"asset_id"`
	Status   string `json:"status"`
	ImageURL string `json:"image_url,omitempty"`
	ErrorMsg string `json:"error_msg,omitempty"`
}

type KafkaProducer struct {
	writer *kafka.Writer
	logger *zap.Logger
}

// NewKafkaProducer —— 创建 Kafka 生产者实例，返回 *KafkaProducer
func NewKafkaProducer(brokers []string, topic string, logger *zap.Logger) *KafkaProducer {
	return &KafkaProducer{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    topic,
			Balancer: &kafka.LeastBytes{},
		},
		logger: logger,
	}
}

// PublishGenerate —— 将资产生成请求发布到 Kafka 主题
func (p *KafkaProducer) PublishGenerate(ctx context.Context, req AssetGenerateRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(fmt.Sprintf("asset-%d", req.AssetID)),
		Value: data,
	})
}

// Close —— 关闭 Kafka 写入器连接
func (p *KafkaProducer) Close() error {
	return p.writer.Close()
}
