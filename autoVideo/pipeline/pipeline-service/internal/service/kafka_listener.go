package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/autovideo/pipeline-service/internal/model"
	kafka "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// KafkaEvent 通用事件结构
type KafkaEvent struct {
	Type      string          `json:"type"`       // e.g. "image.task.done"
	PipelineID string         `json:"pipeline_id"`
	Payload   json.RawMessage `json:"payload"`
}

// KafkaListener 监听各服务发布的完成事件
type KafkaListener struct {
	reader  *kafka.Reader
	svc     *PipelineService
	logger  *zap.Logger
}

// NewKafkaListener —— 创建 Kafka 监听器，订阅指定 topic 消费事件消息
func NewKafkaListener(brokers []string, groupID string, topic string, svc *PipelineService, logger *zap.Logger) *KafkaListener {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		GroupID:  groupID,
		Topic:    topic,
		MinBytes: 1,
		MaxBytes: 1 << 20, // 1 MB
	})
	return &KafkaListener{
		reader: reader,
		svc:    svc,
		logger: logger,
	}
}

// Run 启动监听循环，阻塞运行，ctx 取消时退出
func (l *KafkaListener) Run(ctx context.Context) {
	l.logger.Info("kafka listener started")
	defer l.reader.Close()

	for {
		msg, err := l.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				l.logger.Info("kafka listener stopped")
				return
			}
			l.logger.Error("kafka read message error", zap.Error(err))
			continue
		}

		l.handleMessage(ctx, msg)
	}
}

// handleMessage —— 解析 Kafka 消息并根据事件类型分发到对应处理函数
func (l *KafkaListener) handleMessage(ctx context.Context, msg kafka.Message) {
	var event KafkaEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		l.logger.Warn("failed to unmarshal kafka event", zap.Error(err), zap.ByteString("value", msg.Value))
		return
	}

	if event.PipelineID == "" {
		return
	}

	l.logger.Info("kafka event received",
		zap.String("type", event.Type),
		zap.String("pipeline_id", event.PipelineID),
	)

	state, err := l.svc.GetState(ctx, event.PipelineID)
	if err != nil {
		l.logger.Warn("pipeline not found for kafka event",
			zap.String("pipeline_id", event.PipelineID),
			zap.Error(err),
		)
		return
	}

	switch event.Type {
	case "image.task.done":
		l.handleImageTaskDone(ctx, state, event.Payload)
	case "video.task.done":
		l.handleVideoTaskDone(ctx, state, event.Payload)
	case "script.analyze.done":
		l.handleScriptAnalyzeDone(ctx, state, event.Payload)
	default:
		l.logger.Debug("unhandled kafka event type", zap.String("type", event.Type))
	}
}

// handleScriptAnalyzeDone —— 处理剧本分析完成事件，记录日志并保存状态
func (l *KafkaListener) handleScriptAnalyzeDone(ctx context.Context, state *model.PipelineState, payload json.RawMessage) {
	l.svc.appendLog(state, fmt.Sprintf("[KAFKA] script.analyze.done 事件收到"))
	l.svc.saveState(state)
}

// handleImageTaskDone —— 处理图片任务完成事件，记录任务状态日志
func (l *KafkaListener) handleImageTaskDone(ctx context.Context, state *model.PipelineState, payload json.RawMessage) {
	var p struct {
		TaskID int64  `json:"task_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		l.logger.Warn("failed to unmarshal image.task.done payload", zap.Error(err))
		return
	}
	l.svc.appendLog(state, fmt.Sprintf("[KAFKA] image task %d 完成，状态: %s", p.TaskID, p.Status))
	l.svc.saveState(state)
}

// handleVideoTaskDone —— 处理视频任务完成事件，记录任务状态日志
func (l *KafkaListener) handleVideoTaskDone(ctx context.Context, state *model.PipelineState, payload json.RawMessage) {
	var p struct {
		TaskID int64  `json:"task_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		l.logger.Warn("failed to unmarshal video.task.done payload", zap.Error(err))
		return
	}
	l.svc.appendLog(state, fmt.Sprintf("[KAFKA] video task %d 完成，状态: %s", p.TaskID, p.Status))
	l.svc.saveState(state)
}
