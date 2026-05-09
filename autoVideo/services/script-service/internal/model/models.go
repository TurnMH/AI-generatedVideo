package model

import (
	"time"

	"gorm.io/gorm"
)

// Script 剧本主表
type Script struct {
	ID          int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	ProjectID   int64          `gorm:"not null;index"           json:"project_id"`
	EpisodeID   *int64         `gorm:"index"                    json:"episode_id"`
	Title       string         `gorm:"type:varchar(256)"        json:"title"`
	RawText     string         `gorm:"type:text"                json:"raw_text,omitempty"`
	FileURL     string         `gorm:"type:text"                json:"file_url"`
	FileSize    int            `gorm:"default:0"                json:"file_size"`
	Version     int            `gorm:"default:1"                json:"version"`
	ParseStatus string         `gorm:"type:varchar(32);default:'pending'" json:"parse_status"`
	LLMResult   JSONMap        `gorm:"type:jsonb"               json:"llm_result,omitempty"`
	// Mode 记录该剧本所属项目的工作流模式: "script" / "storyboard"
	Mode        string         `gorm:"type:varchar(30);default:'script'" json:"mode"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// Scene 场景表
type Scene struct {
	ID                int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ScriptID          int64     `gorm:"not null;index"           json:"script_id"`
	EpisodeID         *int64    `gorm:"index"                    json:"episode_id"`
	SceneOrder        int       `gorm:"not null"                 json:"scene_order"`
	Description       string    `gorm:"type:text"                json:"description"`
	Setting           string    `gorm:"type:text"                json:"setting"`
	Emotion           string    `gorm:"type:varchar(64)"         json:"emotion"`
	Characters        JSONSlice `gorm:"type:jsonb;default:'[]'"  json:"characters"`
	PromptDraft       string    `gorm:"type:text"                json:"prompt_draft"`
	Storyboard        JSONSlice `gorm:"type:jsonb;default:'[]'"  json:"storyboard"`
	ImageURL          string    `gorm:"type:text"                json:"image_url"`
	Status            string    `gorm:"type:varchar(32);default:'draft'" json:"status"`
	WordCount         int       `gorm:"default:0"                json:"word_count"`
	EstimatedDuration int       `gorm:"default:0"                json:"estimated_duration"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// SplitConfig 拆分配置
type SplitConfig struct {
	ID              int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ScriptID        int64     `gorm:"not null;index"           json:"script_id"`
	SplitMethod     string    `gorm:"type:varchar(50);default:'scene_based'" json:"split_method"`
	TargetWordCount int       `gorm:"default:3000"             json:"target_word_count"`
	TargetEpisodes  int       `gorm:"default:0"                json:"target_episodes"`
	CustomParams    JSONMap   `gorm:"type:jsonb;default:'{}'"  json:"custom_params"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// CharacterExtracted 抽取的角色
type CharacterExtracted struct {
	ID               int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ScriptID         int64     `gorm:"not null;index"           json:"script_id"`
	Name             string    `gorm:"type:varchar(128);not null" json:"name"`
	RoleDesc         string    `gorm:"type:text"                json:"role_desc"`
	AppearanceDesc   string    `gorm:"type:text"                json:"appearance_desc"`
	Keywords         JSONMap   `gorm:"type:jsonb;default:'{}'"  json:"keywords"`
	SkillTags        JSONSlice `gorm:"type:jsonb;default:'[]'"  json:"skill_tags"`
	FirstAppearScene int       `json:"first_appear_scene"`
	Relationships    JSONMap   `gorm:"type:jsonb;default:'{}'"  json:"relationships"`
	CreatedAt        time.Time `json:"created_at"`
}

// ScriptAsset 剧本资产（人物/地点/道具）
type ScriptAsset struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ScriptID    int64     `gorm:"not null;index"           json:"script_id"`
	AssetType   string    `gorm:"type:varchar(32)"         json:"asset_type"`
	Name        string    `gorm:"type:varchar(128)"        json:"name"`
	Description string    `gorm:"type:text"                json:"description"`
	Keywords    JSONMap   `gorm:"type:jsonb;default:'{}'"  json:"keywords"`
	SceneIDs    JSONSlice `gorm:"type:jsonb;default:'[]'"  json:"scene_ids"`
	CreatedAt   time.Time `json:"created_at"`
}

// ScriptLibrarySource 剧本库来源
type ScriptLibrarySource string

const (
	SourceUploaded ScriptLibrarySource = "uploaded"  // 用户上传
	SourceShowcase ScriptLibrarySource = "showcase"  // 推荐展示
)

// ScriptLibrary 剧本库
type ScriptLibrary struct {
	ID          int64               `gorm:"primaryKey;autoIncrement" json:"id"`
	Title       string              `gorm:"type:varchar(256);not null" json:"title"`
	Author      string              `gorm:"type:varchar(128)"        json:"author"`
	Description string              `gorm:"type:text"                json:"description"`
	CoverURL    string              `gorm:"type:text"                json:"cover_url"`
	FileURL     string              `gorm:"type:text"                json:"file_url"`
	FileSize    int                 `gorm:"default:0"                json:"file_size"`
	WordCount   int                 `gorm:"default:0"                json:"word_count"`
	Genre       string              `gorm:"type:varchar(64)"         json:"genre"`
	Tags        JSONSlice           `gorm:"type:jsonb;default:'[]'"  json:"tags"`
	Source      ScriptLibrarySource `gorm:"type:varchar(32);not null;index" json:"source"`
	ProjectID   *int64              `gorm:"index"                    json:"project_id"`
	UserID      int64               `gorm:"index"                    json:"user_id"`
	IsPublic    bool                `gorm:"default:true"             json:"is_public"`
	SortOrder   int                 `gorm:"default:0"                json:"sort_order"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
}
