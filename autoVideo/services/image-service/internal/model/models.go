package model

import (
	"time"

	"gorm.io/datatypes"
)

type ImageTask struct {
	ID                int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	SceneID           *int64         `gorm:"column:scene_id"          json:"scene_id"`
	ProjectID         int64          `gorm:"column:project_id;not null" json:"project_id"`
	UserID            int64          `gorm:"column:user_id;not null"  json:"user_id"`
	Prompt            string         `gorm:"column:prompt;not null"   json:"prompt"`
	NegativePrompt    string         `gorm:"column:negative_prompt;default:''" json:"negative_prompt"`
	TaskType          string         `gorm:"column:task_type;type:varchar(64);default:'general';index" json:"task_type"`
	StylePreset       string         `gorm:"column:style_preset;default:'anime'" json:"style_preset"`
	StyleReferenceURL string         `gorm:"column:style_reference_url" json:"style_reference_url"`
	// IsCharacterSheet marks the reference image as a 4-panel character
	// turnaround. When true, image-service injects explicit "same person,
	// 4 views" guidance to every generator that accepts a reference image.
	IsCharacterSheet  bool           `gorm:"column:is_character_sheet;default:false" json:"is_character_sheet"`
	// ReferenceImageURLs carries additional character/scene reference images
	// beyond the single StyleReferenceURL so multi-image generators (Gemini
	// parts[], Qwen-Image-Edit messages.content[], Seedream image[], gpt-image-1
	// image[]) can receive every relevant asset. Stored as JSON array of URLs.
	ReferenceImageURLs datatypes.JSON `gorm:"column:reference_image_urls;type:json" json:"reference_image_urls"`
	CharacterIDs      datatypes.JSON `gorm:"column:character_ids;default:'[]'" json:"character_ids"`
	ModelName         string         `gorm:"column:model_name;not null;default:'sdxl'" json:"model_name"`
	Width             int            `gorm:"column:width;default:512"  json:"width"`
	Height            int            `gorm:"column:height;default:768" json:"height"`
	Steps             int            `gorm:"column:steps;default:20"   json:"steps"`
	CfgScale          float64        `gorm:"column:cfg_scale;default:7.0" json:"cfg_scale"`
	Seed              int64          `gorm:"column:seed;default:-1"    json:"seed"`
	Status            string         `gorm:"column:status;default:'pending'" json:"status"`
	ResultURL         string         `gorm:"column:result_url"         json:"result_url"`
	ThumbnailURL      string         `gorm:"column:thumbnail_url"      json:"thumbnail_url"`
	ErrorMsg          string         `gorm:"column:error_msg"          json:"error_msg"`
	Metadata          datatypes.JSON `gorm:"column:metadata;default:'{}'" json:"metadata"`
	CreatedAt         time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

// TableName —— 返回 ImageTask 对应的数据库表名 "image_tasks"
func (ImageTask) TableName() string {
	return "image_tasks"
}

const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)
