package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// StringArray is a []string that serializes to/from JSON for PostgreSQL JSONB.
type StringArray []string

// Value —— 将 StringArray 序列化为 JSON 字符串，用于数据库写入
func (s StringArray) Value() (driver.Value, error) {
	b, err := json.Marshal(s)
	return string(b), err
}

// Scan —— 从数据库读取 JSON 数据并反序列化为 StringArray
func (s *StringArray) Scan(src any) error {
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("StringArray: unsupported type %T", src)
	}
	return json.Unmarshal(b, s)
}

// RenderConfig stores arbitrary render settings as a JSONB column.
type RenderConfig map[string]any

// Value —— 将 RenderConfig 序列化为 JSON 字符串，用于数据库写入
func (r RenderConfig) Value() (driver.Value, error) {
	if r == nil {
		return "{}", nil
	}
	b, err := json.Marshal(r)
	return string(b), err
}

// Scan —— 从数据库读取 JSON 数据并反序列化为 RenderConfig
func (r *RenderConfig) Scan(src any) error {
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	case nil:
		*r = RenderConfig{}
		return nil
	default:
		return fmt.Errorf("RenderConfig: unsupported type %T", src)
	}
	return json.Unmarshal(b, r)
}

// VideoTask represents a video generation task.
type VideoTask struct {
	ID           int64        `gorm:"primaryKey;autoIncrement" json:"id"`
	ProjectID    int64        `gorm:"not null" json:"project_id"`
	EpisodeID    *int64       `json:"episode_id"`
	UserID       int64        `gorm:"not null" json:"user_id"`
	ImageURLs    StringArray  `gorm:"type:jsonb;default:'[]'" json:"image_urls"`
	StylePreset  string       `gorm:"default:'anime'" json:"style_preset"`
	MotionMode   string       `gorm:"default:'gentle'" json:"motion_mode"`
	AudioURL     string       `json:"audio_url"`
	SubtitleText string       `json:"subtitle_text"`
	ModelName    string       `gorm:"default:'kling'" json:"model_name"`
	Status       string       `gorm:"default:'pending'" json:"status"`
	ResultURL    string       `json:"result_url"`
	DurationSec  float64      `json:"duration_sec"`
	ErrorMsg     string       `json:"error_msg"`
	VideoMode    string       `gorm:"default:'frame_animation'" json:"video_mode"`
	HlsURL       string       `gorm:"column:hls_url;default:''" json:"hls_url"`
	RenderConfig    RenderConfig `gorm:"type:jsonb;default:'{}'" json:"render_config"`
	ExportFormat    string       `gorm:"default:'mp4'" json:"export_format"`
	ComposeStage    string       `gorm:"type:varchar(32);default:''" json:"compose_stage"`
	SceneDescription string      `gorm:"type:text;default:''" json:"scene_description"`
	// feat-6: multi-version support — tasks in the same variant group share VariantGroupID
	VariantGroupID *int64 `gorm:"index" json:"variant_group_id,omitempty"`
	VariantIndex   int    `gorm:"default:0" json:"variant_index"`
	// 视频串行生成字段
	SerialScene    bool        `gorm:"default:false"              json:"serial_scene"`     // 是否启用场景内串行生成
	SceneGroupKeys StringArray `gorm:"type:jsonb;default:'[]'"    json:"scene_group_keys"` // 与 ImageURLs 一一对应的场景 key
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
	DeletedAt    *time.Time   `gorm:"index" json:"deleted_at,omitempty"`

	Clips []VideoClip `gorm:"foreignKey:VideoTaskID" json:"clips,omitempty"`
}

// TableName —— 返回 VideoTask 对应的数据库表名 "video_tasks"
func (VideoTask) TableName() string { return "video_tasks" }

// VideoClip represents a single clip within a video task.
type VideoClip struct {
	ID             int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	VideoTaskID    int64     `gorm:"not null" json:"video_task_id"`
	ClipOrder      int       `gorm:"not null" json:"clip_order"`
	SourceImageURL string    `gorm:"not null" json:"source_image_url"`
	ClipURL        string    `json:"clip_url"`
	DurationSec    float64   `json:"duration_sec"`
	ModelUsed      string    `json:"model_used"`
	Status         string    `gorm:"default:'pending'" json:"status"`
	ErrorMsg       string    `json:"error_msg"`
	// 视频串行生成字段
	SceneGroupKey         string  `gorm:"size:200;default:''"   json:"scene_group_key"`          // 所属场景 key
	SceneSeq              int     `gorm:"default:0"             json:"scene_seq"`                // 组内序号（0-based）
	EndFrameImageURL      string  `gorm:"type:text;default:''"  json:"end_frame_image_url"`      // 本片段末帧 URL（下一片段参考用）
	ChainFailureAnalysis  string  `gorm:"type:text;default:''"  json:"chain_failure_analysis,omitempty"` // AI 串行失败诊断（JSON: reason+suggestions）
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// TableName —— 返回 VideoClip 对应的数据库表名 "video_clips"
func (VideoClip) TableName() string { return "video_clips" }

// DubbingTask represents an async dubbing or subtitle generation task.
type DubbingTask struct {
	ID              int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ProjectID       int64     `gorm:"not null;index" json:"project_id"`
	EpisodeID       int64     `gorm:"not null;index" json:"episode_id"`
	StoryboardID    *int64    `gorm:"index" json:"storyboard_id,omitempty"` // optional: scoped to a single storyboard
	UserID          int64     `gorm:"not null;index" json:"user_id"`
	TaskType        string    `gorm:"type:varchar(32);not null;default:'dubbing'" json:"task_type"` // dubbing | subtitle
	SourceText      string    `gorm:"type:text" json:"source_text"`
	VoiceModel      string    `gorm:"type:varchar(64);default:'default'" json:"voice_model"`
	VoiceRate       string    `gorm:"type:varchar(16);default:'+0%'" json:"voice_rate"`
	VoicePitch      string    `gorm:"type:varchar(16);default:'+0Hz'" json:"voice_pitch"`
	VoiceVolume     string    `gorm:"type:varchar(16);default:'+0%'" json:"voice_volume"`
	CustomAudioURL  string    `gorm:"type:text" json:"custom_audio_url"` // feat-7: bypass TTS, use this audio directly
	Status          string    `gorm:"type:varchar(32);index;default:'pending'" json:"status"`
	ChunksDone      int       `gorm:"default:0" json:"chunks_done"`
	ChunksTotal     int       `gorm:"default:0" json:"chunks_total"`
	AudioURL        string    `gorm:"type:text" json:"audio_url"`
	SubtitleURL     string    `gorm:"type:text" json:"subtitle_url"`
	DurationSec     float64   `json:"duration_sec"`
	ErrorMsg        string    `gorm:"type:text" json:"error_msg"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// TableName —— 返回 DubbingTask 对应的数据库表名 "dubbing_tasks"
func (DubbingTask) TableName() string { return "dubbing_tasks" }

// Status constants.
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusSucceeded  = "succeeded"
	StatusFailed     = "failed"
	StatusPaused     = "paused"
	StatusCancelled  = "cancelled"
)

// ActiveStatuses lists statuses that should appear in normal listings.
var ActiveStatuses = []string{StatusPending, StatusProcessing, StatusSucceeded, StatusPaused}

// Compose stage constants.
const (
	ComposeStageNone      = ""
	ComposeStageConcating = "concatenating"
	ComposeStageAudio     = "adding_audio"
	ComposeStageSubtitle  = "adding_subtitle"
	ComposeStageUploading = "uploading"
	ComposeStageDone      = "done"
)
