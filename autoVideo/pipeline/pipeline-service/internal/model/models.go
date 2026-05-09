package model

import "time"

// 阶段常量
const (
	StageInit             = "INIT"
	StageScriptAnalyzing  = "SCRIPT_ANALYZING"
	StageAssetsExtracting = "ASSETS_EXTRACTING"
	StageStoryboardGen    = "STORYBOARD_GENERATING"
	StageImagesGenerating = "IMAGES_GENERATING"
	StageImagesReviewing  = "IMAGES_REVIEWING"
	StageVideosGenerating = "VIDEOS_GENERATING"
	StageComposing        = "COMPOSING"
	StageAutoFixing       = "AUTO_FIXING"
	StageDone             = "DONE"
)

// 状态常量
const (
	StatusRunning = "running"
	StatusPaused  = "paused"
	StatusDone    = "done"
	StatusFailed  = "failed"
	StatusAborted = "aborted"
)

type PipelineConfig struct {
	EpisodeCount   int    `json:"episode_count"`
	StylePreset    string `json:"style_preset"`
	QualityMode    string `json:"quality_mode"` // quality/balanced/speed/cost
	AutoFix        bool   `json:"auto_fix"`
	ImageModel     string `json:"image_model"`    // auto/sdxl/flux/dalle
	VideoModel     string `json:"video_model"`    // auto/kling/wan/cogvideo
	EnableAudio    bool   `json:"enable_audio"`
	EnableSubtitle bool   `json:"enable_subtitle"`
	MaxConcurrent  int    `json:"max_concurrent"` // 默认5
}

type PipelineState struct {
	ID           string         `json:"id"`
	ProjectID    int64          `json:"project_id"`
	ScriptID     int64          `json:"script_id"`
	Config       PipelineConfig `json:"config"`
	Stage        string         `json:"stage"`
	Status       string         `json:"status"`
	Logs         []string       `json:"logs"`
	Progress     int            `json:"progress"`
	SceneIDs     []int64        `json:"scene_ids"`
	ImageTaskIDs []int64        `json:"image_task_ids"`
	VideoTaskIDs []int64        `json:"video_task_ids"`
	StartedAt    time.Time      `json:"started_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	Report       *PipelineReport `json:"report,omitempty"`
}

type PipelineReport struct {
	TotalScenes   int     `json:"total_scenes"`
	SuccessImages int     `json:"success_images"`
	FailedImages  int     `json:"failed_images"`
	TotalDuration float64 `json:"total_duration_sec"`
	FailedScenes  []int64 `json:"failed_scenes"`
}

// HTTP请求/响应结构体

type StartPipelineReq struct {
	ProjectID int64          `json:"project_id" binding:"required"`
	ScriptID  int64          `json:"script_id" binding:"required"`
	Config    PipelineConfig `json:"config"`
}

// script-service 响应
type ScriptDetail struct {
	ID          int64  `json:"id"`
	ParseStatus string `json:"parse_status"`
	Title       string `json:"title"`
}

type Character struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	RoleDesc       string `json:"role_desc"`
	AppearanceDesc string `json:"appearance_desc"`
}

type Scene struct {
	ID          int64  `json:"id"`
	SceneNumber int    `json:"scene_number"`
	EpisodeID   int64  `json:"episode_id"`
	PromptDraft string `json:"prompt_draft"`
	Description string `json:"description"`
}

// character-service 请求
type CreateCharacterReq struct {
	ProjectID      int64  `json:"project_id"`
	Name           string `json:"name"`
	RoleDesc       string `json:"role_desc"`
	AppearanceDesc string `json:"appearance_desc"`
	StylePreset    string `json:"style_preset"`
}

// image-service 请求/响应
type CreateImageReq struct {
	SceneID     int64  `json:"scene_id"`
	Prompt      string `json:"prompt"`
	Model       string `json:"model"`
	StylePreset string `json:"style_preset"`
	ProjectID   int64  `json:"project_id"`
}

type ImageTask struct {
	ID      int64  `json:"id"`
	Status  string `json:"status"` // pending/running/succeeded/failed
	ImageURL string `json:"image_url"`
	SceneID int64  `json:"scene_id"`
}

// video-service 请求/响应
type CreateVideoReq struct {
	ProjectID   int64   `json:"project_id"`
	EpisodeID   int64   `json:"episode_id"`
	ImageTaskIDs []int64 `json:"image_task_ids"`
	Model       string  `json:"model"`
	StylePreset string  `json:"style_preset"`
}

type VideoTask struct {
	ID       int64  `json:"id"`
	Status   string `json:"status"` // pending/running/succeeded/failed
	VideoURL string `json:"video_url"`
	EpisodeID int64 `json:"episode_id"`
	Duration float64 `json:"duration"`
}
