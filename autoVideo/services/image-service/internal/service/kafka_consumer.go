package service

import (
	"context"
	"encoding/json"

	"github.com/autovideo/image-service/internal/model"
	"github.com/autovideo/image-service/internal/stylepreset"
	"github.com/autovideo/image-service/pkg/config"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type consumeMsg struct {
	TaskID         int64  `json:"task_id"`
	ImageTaskID    int64  `json:"image_task_id"`
	ModelName      string `json:"model_name"`
	Prompt         string `json:"prompt"`
	NegativePrompt string `json:"negative_prompt"`
	TaskType       string `json:"task_type"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
}

type resultMsg struct {
	TaskID      int64  `json:"task_id"`
	ImageTaskID int64  `json:"image_task_id"`
	Status      string `json:"status"`
	ResultURL   string `json:"result_url"`
}

type KafkaConsumer struct {
	reader   *kafka.Reader
	writer   *kafka.Writer
	imageSvc ImageService
	logger   *zap.Logger
}

// NewKafkaConsumer —— 创建 Kafka 消费者实例，初始化 reader 和 writer
func NewKafkaConsumer(cfg *config.Config, imageSvc ImageService, logger *zap.Logger) *KafkaConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: cfg.Kafka.Brokers,
		GroupID: cfg.Kafka.ConsumerGroup,
		Topic:   cfg.Kafka.ConsumerTopic,
	})

	writer := &kafka.Writer{
		Addr:     kafka.TCP(cfg.Kafka.Brokers...),
		Topic:    cfg.Kafka.ProducerTopic,
		Balancer: &kafka.LeastBytes{},
	}

	return &KafkaConsumer{
		reader:   reader,
		writer:   writer,
		imageSvc: imageSvc,
		logger:   logger,
	}
}

// Start —— 启动 Kafka 消费循环，持续读取消息并异步处理图片生成任务
func (c *KafkaConsumer) Start(ctx context.Context) error {
	c.logger.Info("kafka consumer started")
	defer c.reader.Close()
	defer c.writer.Close()

	for {
		m, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			c.logger.Error("kafka read error", zap.Error(err))
			continue
		}

		var msg consumeMsg
		if err := json.Unmarshal(m.Value, &msg); err != nil {
			c.logger.Error("kafka unmarshal error", zap.Error(err), zap.ByteString("value", m.Value))
			continue
		}

		c.logger.Info("kafka message received",
			zap.Int64("task_id", msg.TaskID),
			zap.Int64("image_task_id", msg.ImageTaskID),
		)

		// Concurrency is controlled per-generator inside imageService.
		go func(msg consumeMsg) {
			c.process(ctx, msg)
		}(msg)
	}
}

// process —— 处理单条 Kafka 消息，执行图片生成并将结果发布到结果 topic
func (c *KafkaConsumer) process(ctx context.Context, msg consumeMsg) {
	task := &model.ImageTask{
		ID:             msg.ImageTaskID,
		ModelName:      msg.ModelName,
		Prompt:         msg.Prompt,
		NegativePrompt: msg.NegativePrompt,
		TaskType:       msg.TaskType,
		Width:          msg.Width,
		Height:         msg.Height,
		Steps:          20,
		CfgScale:       7.0,
		Seed:           -1,
		StylePreset:    stylepreset.Default,
	}

	c.imageSvc.RunGeneration(ctx, task)

	// Fetch updated task to get result URL and status.
	updated, err := c.imageSvc.GetTask(ctx, msg.ImageTaskID)
	status := model.StatusSucceeded
	resultURL := ""
	if err != nil {
		c.logger.Error("fetch updated task failed", zap.Error(err))
		status = model.StatusFailed
	} else {
		status = updated.Status
		resultURL = updated.ResultURL
	}

	result := resultMsg{
		TaskID:      msg.TaskID,
		ImageTaskID: msg.ImageTaskID,
		Status:      status,
		ResultURL:   resultURL,
	}
	payload, _ := json.Marshal(result)

	if err := c.writer.WriteMessages(ctx, kafka.Message{Value: payload}); err != nil {
		c.logger.Error("kafka write result failed", zap.Error(err))
	} else {
		c.logger.Info("kafka result published",
			zap.Int64("task_id", msg.TaskID),
			zap.String("status", status),
		)
	}
}
