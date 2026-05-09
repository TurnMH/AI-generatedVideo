package service

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// Topic constants for kafka-go consumers/producers.
const (
	TopicScriptAnalyzeReq       = "script.analyze.request"
	TopicScriptQuickGenerateReq = "script.quick_generate.request"
	TopicImageGenerateReq       = "image.generate.request"
	TopicVideoGenerateReq       = "video.generate.request"
	TopicMusicGenerateReq       = "music.generate.request"
	TopicScriptAnalyzeRes       = "script.analyze.result"
	TopicScriptQuickGenerateRes = "script.quick_generate.result"
	TopicImageGenerateRes       = "image.generate.result"
	TopicVideoGenerateRes       = "video.generate.result"
	TopicMusicGenerateRes       = "music.generate.result"
	TopicTaskCompleted          = "task.completed"
	TopicTaskFailed             = "task.failed"
	TopicTaskProgress           = "task.progress"
)

// KafkaService manages kafka-go writers and consumers.
type KafkaService struct {
	writers map[string]*kafka.Writer
	brokers []string
	logger  *zap.Logger
}

// NewKafkaService —— 创建 KafkaService 实例，初始化 broker 连接
// NewKafkaService creates a KafkaService with the provided broker addresses.
func NewKafkaService(brokers []string, logger *zap.Logger) *KafkaService {
	return &KafkaService{
		writers: make(map[string]*kafka.Writer),
		brokers: brokers,
		logger:  logger,
	}
}

// writer —— 获取或惰性创建指定 topic 的 Kafka Writer
// writer returns or lazily creates a writer for the given topic.
func (k *KafkaService) writer(topic string) *kafka.Writer {
	if w, ok := k.writers[topic]; ok {
		return w
	}
	w := &kafka.Writer{
		Addr:         kafka.TCP(k.brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Async:        false,
	}
	k.writers[topic] = w
	return w
}

// Publish —— 向指定 Kafka topic 发送消息，附带优先级 header
// Publish sends a message to a Kafka topic. The priority is attached as an X-Priority header.
func (k *KafkaService) Publish(ctx context.Context, topic string, key string, value []byte, priority int) error {
	w := k.writer(topic)
	msg := kafka.Message{
		Key:   []byte(key),
		Value: value,
		Headers: []kafka.Header{
			{Key: "X-Priority", Value: []byte(fmt.Sprintf("%d", priority))},
		},
	}
	if err := w.WriteMessages(ctx, msg); err != nil {
		k.logger.Error("kafka publish error", zap.String("topic", topic), zap.Error(err))
		return err
	}
	k.logger.Debug("kafka published", zap.String("topic", topic), zap.String("key", key))
	return nil
}

// StartConsumer —— 启动后台协程消费指定 Kafka topic，支持自动重连
// StartConsumer starts a Kafka consumer in a goroutine.
// It automatically reconnects on reader errors and commits only after a successful handler call.
func (k *KafkaService) StartConsumer(ctx context.Context, topic, groupID string, handler func([]byte) error) {
	go func() {
		for {
			reader := kafka.NewReader(kafka.ReaderConfig{
				Brokers:  k.brokers,
				Topic:    topic,
				GroupID:  groupID,
				MinBytes: 1,
				MaxBytes: 10e6,
			})

			k.logger.Info("kafka consumer started", zap.String("topic", topic), zap.String("group", groupID))

			for {
				select {
				case <-ctx.Done():
					reader.Close()
					k.logger.Info("kafka consumer stopped", zap.String("topic", topic))
					return
				default:
				}

				msg, err := reader.ReadMessage(ctx)
				if err != nil {
					if ctx.Err() != nil {
						reader.Close()
						return
					}
					k.logger.Warn("kafka read error, reconnecting", zap.String("topic", topic), zap.Error(err))
					reader.Close()
					break // reconnect
				}

				if err := handler(msg.Value); err != nil {
					k.logger.Error("kafka handler error", zap.String("topic", topic), zap.Error(err))
					// do not commit — message will be redelivered
					continue
				}
			}

			// check context before reconnecting
			if ctx.Err() != nil {
				return
			}
		}
	}()
}

// PublishBatch —— 批量发送消息到指定 Kafka topic，减少网络往返
// PublishBatch sends multiple messages to a Kafka topic in a single write call.
func (k *KafkaService) PublishBatch(ctx context.Context, topic string, messages []kafka.Message) error {
	if len(messages) == 0 {
		return nil
	}
	w := k.writer(topic)
	if err := w.WriteMessages(ctx, messages...); err != nil {
		k.logger.Error("kafka batch publish error", zap.String("topic", topic), zap.Int("count", len(messages)), zap.Error(err))
		return err
	}
	k.logger.Debug("kafka batch published", zap.String("topic", topic), zap.Int("count", len(messages)))
	return nil
}

// Close —— 关闭所有 Kafka Writer 连接
// Close shuts down all writers.
func (k *KafkaService) Close() {
	for topic, w := range k.writers {
		if err := w.Close(); err != nil {
			k.logger.Warn("kafka writer close error", zap.String("topic", topic), zap.Error(err))
		}
	}
}
