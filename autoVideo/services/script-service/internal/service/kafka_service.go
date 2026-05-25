package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/autovideo/script-service/pkg/config"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type KafkaService interface {
	PublishAnalyzeResult(ctx context.Context, scriptID int64, status string, sceneCount int) error
	StartConsumer(ctx context.Context, scriptSvc ScriptService)
}

type kafkaService struct {
	producer *kafka.Writer
	consumer *kafka.Reader
	logger   *zap.Logger
	brokers  []string
}

// NewKafkaService —— 创建 Kafka 服务实例，初始化生产者和消费者
func NewKafkaService(cfg *config.Config, logger *zap.Logger) KafkaService {
	producer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Kafka.Brokers...),
		Topic:        cfg.Kafka.ProducerTopic,
		Balancer:     &kafka.LeastBytes{},
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}

	consumer := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  cfg.Kafka.Brokers,
		Topic:    cfg.Kafka.ConsumerTopic,
		GroupID:  "script-service-group",
		MinBytes: 1,
		MaxBytes: 10e6,
		MaxWait:  3 * time.Second,
	})

	return &kafkaService{
		producer: producer,
		consumer: consumer,
		logger:   logger,
		brokers:  cfg.Kafka.Brokers,
	}
}

type analyzeResultMsg struct {
	ScriptID   int64  `json:"script_id"`
	Status     string `json:"status"`
	SceneCount int    `json:"scene_count"`
}

// PublishAnalyzeResult —— 将剧本分析结果发布到 Kafka topic
func (s *kafkaService) PublishAnalyzeResult(ctx context.Context, scriptID int64, status string, sceneCount int) error {
	payload, err := json.Marshal(analyzeResultMsg{
		ScriptID:   scriptID,
		Status:     status,
		SceneCount: sceneCount,
	})
	if err != nil {
		return fmt.Errorf("marshal kafka message: %w", err)
	}

	msg := kafka.Message{
		Key:   []byte(fmt.Sprintf("%d", scriptID)),
		Value: payload,
	}

	if err := s.producer.WriteMessages(ctx, msg); err != nil {
		s.logger.Error("failed to publish kafka message",
			zap.Int64("script_id", scriptID),
			zap.Error(err),
		)
		return fmt.Errorf("write kafka message: %w", err)
	}

	s.logger.Info("kafka message published",
		zap.String("topic", s.producer.Topic),
		zap.Int64("script_id", scriptID),
		zap.String("status", status),
	)
	return nil
}

type analyzeRequestMsg struct {
	ScriptID int64 `json:"script_id"`
}

// StartConsumer —— 启动 Kafka 消费者协程，监听分析请求并触发剧本分析
func (s *kafkaService) StartConsumer(ctx context.Context, scriptSvc ScriptService) {
	go func() {
		s.logger.Info("kafka consumer started", zap.String("topic", s.consumer.Config().Topic))
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("kafka consumer stopped")
				_ = s.consumer.Close()
				return
			default:
			}

			m, err := s.consumer.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				s.logger.Error("kafka read message error", zap.Error(err))
				continue
			}

			var req analyzeRequestMsg
			if err := json.Unmarshal(m.Value, &req); err != nil {
				s.logger.Error("unmarshal kafka message failed",
					zap.ByteString("value", m.Value),
					zap.Error(err),
				)
				continue
			}

			s.logger.Info("received analyze request from kafka",
				zap.Int64("script_id", req.ScriptID),
			)

			if err := scriptSvc.TriggerAnalyze(ctx, req.ScriptID); err != nil {
				s.logger.Error("trigger analyze failed",
					zap.Int64("script_id", req.ScriptID),
					zap.Error(err),
				)
			}
		}
	}()
}

