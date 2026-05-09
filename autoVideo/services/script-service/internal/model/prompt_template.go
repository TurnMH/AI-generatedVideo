package model

import "time"

// PromptTemplate 提示词模板
type PromptTemplate struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name         string    `gorm:"size:128;not null"        json:"name"`
	StyleKey     string    `gorm:"size:64;not null"         json:"style_key"`
	Description  string    `gorm:"type:text"                json:"description"`
	Content      string    `gorm:"type:text;not null"       json:"content"`
	ResourceType string    `gorm:"size:32"                  json:"resource_type"` // character|scene|item|storyboard
	ModelBinding string    `gorm:"size:128"                 json:"model_binding"` // model name or empty for universal
	Version      string    `gorm:"size:32"                  json:"version"`
	IsActive     bool      `gorm:"default:true"             json:"is_active"`
	SortOrder    int       `gorm:"default:0"                json:"sort_order"`
	// TemplateFileURL 风格对应的可下载剪映模版文件地址（后台填写，前端展示下载链接）
	TemplateFileURL string  `gorm:"type:text"               json:"template_file_url,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName —— 返回 PromptTemplate 对应的数据库表名
func (PromptTemplate) TableName() string { return "prompt_templates" }
