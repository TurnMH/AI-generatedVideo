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

const (
	topicQuickGenerateReq = "script.quick_generate.request"
	topicQuickGenerateRes = "script.quick_generate.result"
)

type KafkaService interface {
	PublishAnalyzeResult(ctx context.Context, scriptID int64, status string, sceneCount int) error
	StartConsumer(ctx context.Context, scriptSvc ScriptService)
	StartQuickGenerateConsumer(ctx context.Context, llmClient LLMClient)
}

type kafkaService struct {
	producer      *kafka.Writer
	quickProducer *kafka.Writer
	consumer      *kafka.Reader
	logger        *zap.Logger
	brokers       []string
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

	quickProducer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Kafka.Brokers...),
		Topic:        topicQuickGenerateRes,
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
		producer:      producer,
		quickProducer: quickProducer,
		consumer:      consumer,
		logger:        logger,
		brokers:       cfg.Kafka.Brokers,
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

// quickGenerateTaskMsg 是从 task-service 发来的 quick generate 任务消息体
type quickGenerateTaskMsg struct {
	ID      uint64 `json:"id"`
	Payload struct {
		Mode            string `json:"mode"`
		Premise         string `json:"premise"`
		Genre           string `json:"genre"`
		Platform        string `json:"platform"`
		DeliveryFormat  string `json:"delivery_format"`
		EpisodeDuration string `json:"episode_duration"`
		Tone            string `json:"tone"`
		Requirements    string `json:"requirements"`
		TargetWords     int    `json:"target_words"`
		ChapterCount    int    `json:"chapter_count"`
	} `json:"payload"`
}

type quickGenerateResultMsg struct {
	TaskID uint64              `json:"task_id"`
	Result *ScriptGenerateResult `json:"result"`
}

type quickGenerateFailMsg struct {
	TaskID   uint64 `json:"task_id"`
	ErrorMsg string `json:"error_msg"`
}

// StartQuickGenerateConsumer —— 启动 quick_generate 任务消费者，消费 Kafka 消息并调用 LLM 生成剧本
// 使用 FetchMessage + 手动 CommitMessages，确保服务重启时未处理完的消息会被重新投递
func (s *kafkaService) StartQuickGenerateConsumer(ctx context.Context, llmClient LLMClient) {
	go func() {
		reader := kafka.NewReader(kafka.ReaderConfig{
			Brokers:  s.brokers,
			Topic:    topicQuickGenerateReq,
			GroupID:  "script-service-quick-group",
			MinBytes: 1,
			MaxBytes: 10e6,
			MaxWait:  3 * time.Second,
		})
		defer reader.Close()

		s.logger.Info("quick_generate kafka consumer started", zap.String("topic", topicQuickGenerateReq))

		for {
			select {
			case <-ctx.Done():
				s.logger.Info("quick_generate kafka consumer stopped")
				return
			default:
			}

			// FetchMessage 不自动提交 offset，只有处理成功后才手动 commit
			m, err := reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				s.logger.Error("quick_generate kafka read error", zap.Error(err))
				continue
			}

			var task quickGenerateTaskMsg
			if err := json.Unmarshal(m.Value, &task); err != nil {
				s.logger.Error("unmarshal quick_generate task failed", zap.Error(err))
				// 无法解析的消息直接 commit 跳过，避免无限循环
				_ = reader.CommitMessages(ctx, m)
				continue
			}

			s.logger.Info("processing quick_generate task", zap.Uint64("task_id", task.ID))

			p := task.Payload
			mode := p.Mode
			if mode == "" {
				mode = "script"
			}

			// 使用独立的 180s 超时 context，避免受全局 ctx 取消影响导致消息重投
			llmCtx, llmCancel := context.WithTimeout(context.Background(), 180*time.Second)
			result, err := llmClient.GenerateScript(llmCtx, &ScriptGenerateReq{
				Mode:            mode,
				Premise:         p.Premise,
				Genre:           p.Genre,
				Platform:        p.Platform,
				DeliveryFormat:  p.DeliveryFormat,
				EpisodeDuration: p.EpisodeDuration,
				Tone:            p.Tone,
				Requirements:    p.Requirements,
				TargetWords:     p.TargetWords,
				ChapterCount:    p.ChapterCount,
			})
			llmCancel()

			if err != nil {
				s.logger.Error("quick_generate llm error", zap.Uint64("task_id", task.ID), zap.Error(err))
				failPayload, _ := json.Marshal(quickGenerateFailMsg{TaskID: task.ID, ErrorMsg: err.Error()})
				_ = s.quickProducer.WriteMessages(ctx, kafka.Message{
					Key:   []byte(fmt.Sprintf("%d", task.ID)),
					Value: failPayload,
				})
				// LLM 失败后 commit，避免无限重试同一个失败任务
				_ = reader.CommitMessages(ctx, m)
				continue
			}

			resPayload, err := json.Marshal(quickGenerateResultMsg{TaskID: task.ID, Result: result})
			if err != nil {
				s.logger.Error("marshal quick_generate result failed", zap.Error(err))
				// 序列化失败极少发生，commit 跳过
				_ = reader.CommitMessages(ctx, m)
				continue
			}

			if err := s.quickProducer.WriteMessages(ctx, kafka.Message{
				Key:   []byte(fmt.Sprintf("%d", task.ID)),
				Value: resPayload,
			}); err != nil {
				s.logger.Error("quick_generate publish result failed", zap.Uint64("task_id", task.ID), zap.Error(err))
				// 发布失败不 commit，下次重启会重新处理
			} else {
				s.logger.Info("quick_generate result published", zap.Uint64("task_id", task.ID))
				// 只有成功发布结果后才 commit offset
				if err := reader.CommitMessages(ctx, m); err != nil {
					s.logger.Warn("quick_generate commit offset failed", zap.Uint64("task_id", task.ID), zap.Error(err))
				}
			}
		}
	}()
}
