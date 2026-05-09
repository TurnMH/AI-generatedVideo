package model

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Project struct {
	ID                  uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID              uint64         `gorm:"not null;index" json:"user_id"`
	Title               string         `gorm:"size:256;not null" json:"title"`
	Description         string         `gorm:"type:text" json:"description"`
	ProjectType         string         `gorm:"size:20;not null;default:video;index" json:"project_type"`
	CoverURL            string         `gorm:"type:text" json:"cover_url"`
	LogoURL             string         `gorm:"type:text" json:"logo_url"`
	ScriptFileURL       string         `gorm:"type:text" json:"script_file_url"`
	ScriptText          string         `gorm:"type:text" json:"script_text"`
	ScriptFileSize      int            `json:"script_file_size"`
	ScriptVersions      datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"script_versions"`
	Status              string         `gorm:"size:50;default:draft" json:"status"`
	Progress            datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"progress"`
	TargetEpisodes      int            `gorm:"default:0" json:"target_episodes"`
	StyleTags           pq.StringArray `gorm:"type:text[]" json:"style_tags"`
	TextModelID         *uint64        `json:"text_model_id"`
	ImageModelID        *uint64        `json:"image_model_id"`
	VideoModelID        *uint64        `json:"video_model_id"`
	TTSModelID          *uint64        `json:"tts_model_id"`
	EnableDubbing       bool           `gorm:"default:false" json:"enable_dubbing"`
	EnableSubtitle      bool           `gorm:"default:false" json:"enable_subtitle"`
	VideoMode           string         `gorm:"size:30;default:frame_animation" json:"video_mode"`
	StoryboardConfig    datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"storyboard_config"`
	WatermarkConfig     datatypes.JSON `gorm:"type:jsonb;default:'{\"enabled\":false}'" json:"watermark_config"`
	ConsistencyStrength float64        `gorm:"type:decimal(3,2);default:0.75" json:"consistency_strength"`
	ImageStylePrefix    string         `gorm:"type:text;default:''" json:"image_style_prefix"` // prepended to all image generation prompts
	// Mode 区分项目工作流模式: "script"（小说→剧本→分镜→视频完整流程）/ "storyboard"（直接上传分镜跳过剧本生成）
	Mode                string         `gorm:"size:30;default:'script'" json:"mode"`
	StorageUsedBytes    int64          `gorm:"default:0" json:"storage_used_bytes"`
	// KeywordLibrary stores the project-level keyword glossary extracted from the script:
	// {"characters":[...],"locations":[...],"events":[...],"props":[...]}
	KeywordLibrary      datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"keyword_library"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`
	Episodes            []Episode      `gorm:"foreignKey:ProjectID" json:"episodes,omitempty"`
}

type Episode struct {
	ID                uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	ProjectID         uint64         `gorm:"not null;uniqueIndex:idx_project_episode,priority:1" json:"project_id"`
	EpisodeNumber     int            `gorm:"not null;uniqueIndex:idx_project_episode,priority:2" json:"episode_number"`
	Title             string         `gorm:"size:256" json:"title"`
	Summary           string         `gorm:"type:text" json:"summary"`
	ScriptExcerpt     string         `gorm:"type:text" json:"script_excerpt"`
	WordCount         int            `gorm:"default:0" json:"word_count"`
	EstimatedDuration int            `gorm:"default:0" json:"estimated_duration"`
	Status            string         `gorm:"size:32;default:draft" json:"status"`
	Version           int            `gorm:"default:1" json:"version"`
	Keywords          pq.StringArray `gorm:"type:text[]" json:"keywords"`

	// Script optimization (小说→剧本格式转化)
	// optimize_status: '' | optimizing | done | failed
	OptimizeStatus  string         `gorm:"size:32;default:''" json:"optimize_status"`
	OptimizedText   string         `gorm:"type:text" json:"optimized_text"`
	OriginalExcerpt string         `gorm:"type:text" json:"original_excerpt"`

	// AI review result
	// review_status: '' | reviewing | done | failed
	ReviewStatus string         `gorm:"size:32;default:''" json:"review_status"`
	ReviewResult datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"review_result"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ScriptVersion struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ProjectID     uint64    `gorm:"not null;index" json:"project_id"`
	VersionNumber int       `gorm:"not null" json:"version_number"`
	FileURL       string    `gorm:"type:text" json:"file_url"`
	OSSKey        string    `gorm:"type:text" json:"oss_key"`
	FileSize      int       `json:"file_size"`
	IsCurrent     bool      `gorm:"default:false" json:"is_current"`
	CreatedAt     time.Time `json:"created_at"`
}

type ProjectSnapshot struct {
	ID           uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	ProjectID    uint64         `gorm:"not null;index" json:"project_id"`
	Version      int            `gorm:"not null" json:"version"`
	SnapshotData datatypes.JSON `gorm:"type:jsonb" json:"snapshot_data"`
	CreatedAt    time.Time      `json:"created_at"`
}

// Storyboard 分镜表
type Storyboard struct {
	ID               uint64              `gorm:"primaryKey;autoIncrement" json:"id"`
	ProjectID        uint64              `gorm:"not null;index;index:idx_sb_pid_status,priority:1" json:"project_id"`
	EpisodeID        *uint64             `gorm:"index" json:"episode_id"`
	SequenceNumber   int                 `gorm:"not null" json:"sequence_number"`
	SceneDescription string              `gorm:"type:text" json:"scene_description"`
	Characters       pq.StringArray      `gorm:"type:text[]" json:"characters"`
	Location         string              `gorm:"size:200" json:"location"`
	CameraMovement   string              `gorm:"size:100" json:"camera_movement"`
	Duration         int                 `gorm:"default:4" json:"duration"`
	AspectRatio      string              `gorm:"size:20;default:16:9" json:"aspect_ratio"`
	Resolution       string              `gorm:"size:20;default:1080p" json:"resolution"`
	VideoMode        *string             `gorm:"size:30" json:"video_mode"`
	Dialogue         string              `gorm:"type:text" json:"dialogue"`
	Mood             string              `gorm:"size:50;default:''" json:"mood"`
	CurrentVersion   int                 `gorm:"default:1" json:"current_version"`
	ImageURL         string              `gorm:"type:text" json:"image_url"`
	PromptUsed       string              `gorm:"type:text" json:"prompt_used"`
	VideoPrompt      string              `gorm:"type:text;default:''" json:"video_prompt"`
	Status           string              `gorm:"size:50;default:pending;index:idx_sb_pid_status,priority:2;index:idx_sb_status_upd,priority:1" json:"status"`
	ErrorMsg         string              `gorm:"type:text" json:"error_msg"`
	IsVoided         bool                `gorm:"default:false" json:"is_voided"`
	IsManualEdited   bool                `gorm:"default:false" json:"is_manual_edited"`
	AgentHistory     datatypes.JSON      `gorm:"type:jsonb;default:'[]'" json:"agent_history"`
	AssetIDs         pq.Int64Array       `gorm:"type:bigint[]" json:"asset_ids"`
	// 视频串行流程字段
	SceneGroupKey    string `gorm:"size:200;default:''" json:"scene_group_key"`     // 标准化场景名，用于场景内串行生成
	IsSceneFirstClip bool   `gorm:"default:false"       json:"is_scene_first_clip"` // 是否为该场景的第一个分镜
	EndFrameImageURL string `gorm:"type:text;default:''" json:"end_frame_image_url"` // 前一视频末帧图 URL（作为本视频首帧）
	// Mode 继承自 Project.Mode: "script"（完整流程）/ "storyboard"（直接上传分镜模式）
	Mode             string `gorm:"type:varchar(30);default:'script'" json:"mode"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `gorm:"index:idx_sb_status_upd,priority:2" json:"updated_at"`
	Versions         []StoryboardVersion `gorm:"foreignKey:StoryboardID" json:"versions,omitempty"`
}

// StoryboardVersion 分镜版本表
type StoryboardVersion struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	StoryboardID  uint64    `gorm:"not null;index" json:"storyboard_id"`
	VersionNumber int       `gorm:"not null" json:"version_number"`
	ImageURL      string    `gorm:"type:text" json:"image_url"`
	OSSKey        string    `gorm:"type:text" json:"oss_key"`
	SizeBytes     int64     `gorm:"default:0" json:"size_bytes"`
	PromptUsed    string    `gorm:"type:text" json:"prompt_used"`
	IsCurrent     bool      `gorm:"default:false" json:"is_current"`
	CreatedAt     time.Time `json:"created_at"`
}
