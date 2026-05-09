package model

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/datatypes"
)

// Model represents an AI model available in the platform.
type Model struct {
	ID          uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string         `gorm:"size:128;not null" json:"name"`
	Provider    string         `gorm:"size:64;not null" json:"provider"` // openai/anthropic/replicate/local/aliyun/kuaishou/bytedance
	Type        string         `gorm:"size:32;not null" json:"type"`     // llm/image/video/audio
	APIEndpoint string         `gorm:"type:text" json:"api_endpoint"`
	IsActive    bool           `gorm:"default:true;not null" json:"is_active"`
	Priority    int            `gorm:"default:0" json:"priority"`
	CostPerUnit float64        `json:"cost_per_unit"`
	Unit        string         `gorm:"size:32" json:"unit"` // token/image/second
	Config      datatypes.JSON `gorm:"type:jsonb" json:"config"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`

	// New fields from pending upgrade
	ModelKey            string          `gorm:"size:200" json:"model_key"`
	ContextWindow       *int            `json:"context_window,omitempty"`
	InputPrice          *float64        `gorm:"type:decimal(10,6)" json:"input_price,omitempty"`
	OutputPrice         *float64        `gorm:"type:decimal(10,6)" json:"output_price,omitempty"`
	SpeedRating         string          `gorm:"size:20;default:balanced" json:"speed_rating"`
	CapabilityTags      pq.StringArray  `gorm:"type:text[];default:'{}'" json:"capability_tags"`
	SupportsConsistency bool            `gorm:"default:false" json:"supports_consistency"`
	ConsistencyMethod   string          `gorm:"size:50;default:none" json:"consistency_method"`
	VideoMode           *string         `gorm:"size:50" json:"video_mode,omitempty"`
	MaxResolution       *string         `gorm:"size:50" json:"max_resolution,omitempty"`
	SupportedRatios     pq.StringArray  `gorm:"type:text[];default:'{}'" json:"supported_ratios"`
	IsDefault           bool            `gorm:"default:false" json:"is_default"`
	APIKeyRef           *string         `gorm:"size:200" json:"api_key_ref,omitempty"`
	Description         *string         `gorm:"type:text" json:"description,omitempty"`
	// FailureReason is set when a model is known to be broken/deprecated.
	// When non-nil the frontend shows the model as disabled with a reason badge.
	FailureReason       *string         `gorm:"type:text" json:"failure_reason,omitempty"`
	// SortOrder 控制风格列表展示顺序，小先显，默认 0
	SortOrder           int             `gorm:"default:0" json:"sort_order"`
}

// ListFilter holds query parameters for filtered model listing.
type ListFilter struct {
	Type        string
	Types       []string // multi-type filter (mutilModelList); takes priority over Type when non-empty
	Provider    string
	SpeedRating string
	Enabled     *bool  // nil = no filter
	SortBy      string // "input_price_asc", "input_price_desc"
}

// ModelHealth stores the result of a single health-check probe.
type ModelHealth struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	ModelID   uint64    `gorm:"not null;index"`
	Status    string    `gorm:"size:16"` // healthy/unhealthy/unknown
	LatencyMs int64
	CheckedAt time.Time
	Model     Model `gorm:"foreignKey:ModelID"`
}

// UsageRecord tracks per-request consumption for billing/analytics.
type UsageRecord struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint64    `gorm:"index;not null" json:"user_id"`
	ModelID   uint64    `gorm:"index;not null" json:"model_id"`
	TaskID    string    `gorm:"size:128" json:"task_id"`
	UnitsUsed float64   `json:"units_used"`
	Cost      float64   `json:"cost"`
	CreatedAt time.Time `json:"created_at"`
}
