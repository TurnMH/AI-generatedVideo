package model

import (
	"time"

	"gorm.io/datatypes"
)

type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskSucceeded TaskStatus = "succeeded"
	TaskFailed    TaskStatus = "failed"
	TaskCancelled TaskStatus = "cancelled"
)

type Task struct {
	ID         uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskType   string         `gorm:"size:64;not null;index" json:"task_type"`
	Payload    datatypes.JSON `gorm:"type:jsonb" json:"payload"`
	Priority   int            `gorm:"default:0;index" json:"priority"`
	Status     TaskStatus     `gorm:"size:32;default:pending;index" json:"status"`
	RetryCount int            `gorm:"default:0" json:"retry_count"`
	MaxRetries int            `gorm:"default:3" json:"max_retries"`
	UserID     uint64         `gorm:"not null;index" json:"user_id"`
	ErrorMsg   string         `gorm:"type:text" json:"error_msg,omitempty"`
	Result     datatypes.JSON `gorm:"type:jsonb" json:"result,omitempty"`
	StartedAt  *time.Time     `json:"started_at,omitempty"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

type TaskProgress struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID    uint64    `gorm:"not null;index" json:"task_id"`
	Progress  int       `gorm:"not null" json:"progress"`
	Message   string    `gorm:"type:text" json:"message"`
	CreatedAt time.Time `json:"created_at"`
}
