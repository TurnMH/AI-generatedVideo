package model

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Character 角色资产主表
type Character struct {
	ID                 int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	ProjectID          int64          `gorm:"not null;index"           json:"project_id"`
	Name               string         `gorm:"size:128;not null"        json:"name"`
	RoleDesc           string         `gorm:"type:text"                json:"role_desc"`
	AppearanceDesc     string         `gorm:"type:text"                json:"appearance_desc"`
	ReferenceImageURL  string         `gorm:"type:text"                json:"reference_image_url"`
	StylePreset        string         `gorm:"size:64;default:'anime'"  json:"style_preset"`
	StyleReferenceURL  string         `gorm:"type:text"                json:"style_reference_url"`
	LoraModelID        string         `gorm:"size:128"                 json:"lora_model_id"`
	VoiceModel         string         `gorm:"size:128"                 json:"voice_model"` // TTS voice binding (char-c8)
	IPAdapterConfig    JSONB          `gorm:"type:jsonb;default:'{}'"  json:"ip_adapter_config"`
	FixedSeed          int64          `json:"fixed_seed"`
	ExtraConfig        JSONB          `gorm:"type:jsonb;default:'{}'"  json:"extra_config"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName —— 返回 Character 对应的数据库表名 "characters"
func (Character) TableName() string { return "characters" }

// StylePreset 系统预置风格
type StylePreset struct {
	ID             int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name           string    `gorm:"size:64;uniqueIndex;not null" json:"name"`
	Description    string    `gorm:"type:text"                    json:"description"`
	PreviewURL     string    `gorm:"type:text"                    json:"preview_url"`
	PromptSuffix   string    `gorm:"type:text"                    json:"prompt_suffix"`
	NegativePrompt string    `gorm:"type:text"                    json:"negative_prompt"`
	CreatedAt      time.Time `json:"created_at"`
}

// TableName —— 返回 StylePreset 对应的数据库表名 "style_presets"
func (StylePreset) TableName() string { return "style_presets" }

// Asset 资产管理主表（角色/场景/道具）
type Asset struct {
	ID             uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	ProjectID      uint64         `gorm:"not null;index;index:idx_assets_pid_status,priority:1" json:"project_id"`
	CharacterID    *int64         `gorm:"index"                    json:"character_id,omitempty"` // char-c9: optional link to a Character
	Type           string         `gorm:"size:20;not null" json:"type"`
	Name           string         `gorm:"size:200;not null" json:"name"`
	Description    string         `gorm:"type:text" json:"description"`
	ImageURL       string         `gorm:"type:text" json:"image_url"`
	ConsistencyRef datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"consistency_ref"`
	Metadata       datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata"`
	Status         string         `gorm:"size:50;default:pending;index:idx_assets_pid_status,priority:2;index:idx_assets_status_upd,priority:1" json:"status"`
	IsLocked       bool           `gorm:"default:false" json:"is_locked"`
	IsManual       bool           `gorm:"default:false" json:"is_manual"`
	PromptUsed     string         `gorm:"type:text" json:"prompt_used"`
	ErrorMsg       string         `gorm:"type:text" json:"error_msg"`
	AgentHistory   datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"agent_history"`
	EpisodeIDs     pq.Int64Array  `gorm:"type:integer[]" json:"episode_ids"`
	VoiceModel     string         `gorm:"size:128" json:"voice_model"` // TTS voice binding for auto-dubbing
	// PanelImages 4 视图分栏独立生成的图片 URL，顺序：closeup / front / side / back。
	// 全部成功后，服务端横向拼接写入 CompositeImageURL；ImageURL 同步指向 composite。
	PanelImages       pq.StringArray `gorm:"type:text[];default:'{}'" json:"panel_images"`
	CompositeImageURL string         `gorm:"type:text" json:"composite_image_url"`
	// Seed 本次角色四视图使用的随机种子（>0 表示锁定，-1 或 0 表示随机/未知）。
	// Doubao、SDXL、Flux、CogView 等支持；OpenAI 官方 API 不支持。
	Seed      int64     `gorm:"default:-1" json:"seed"`
	// 角色组造型字段（视频串行流程）
	GroupID      *int64 `gorm:"index"                    json:"group_id,omitempty"`      // 所属 CharacterGroup.ID
	VariantName  string `gorm:"size:100;default:''"      json:"variant_name"`            // 造型名称，如"日常便装"
	AssetSortOrder int  `gorm:"column:asset_sort_order;default:0" json:"asset_sort_order"` // 组内排序
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `gorm:"index:idx_assets_status_upd,priority:2" json:"updated_at"`
}

// Skill 角色能力卡
type Skill struct {
	ID          int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	ProjectID   int64          `gorm:"not null;index:idx_skill_project_usecase,priority:1" json:"project_id"`
	CharacterID *int64         `gorm:"index"                    json:"character_id,omitempty"`
	Name        string         `gorm:"size:128;not null"        json:"name"`
	SkillType   string         `gorm:"size:32"                  json:"skill_type"`   // combat|exploration|social|special
	UseCase     string         `gorm:"size:64;index:idx_skill_project_usecase,priority:2" json:"use_case"` // storyboard|extraction|prompt|writing|storyboard_prep
	Description string         `gorm:"type:text"                json:"description"`
	IsActive    bool           `gorm:"default:true;index:idx_skill_project_usecase,priority:3" json:"is_active"`
	SortOrder   int            `gorm:"default:0"                json:"sort_order"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index"                    json:"-"`
}

// TableName —— 返回 Skill 对应的数据库表名 "skills"
func (Skill) TableName() string { return "skills" }

// ProductionSkill 影视部门技能
// 每条记录代表一个影视部门（如摄影、灯光、美术）的注入指令，
// 在分集剧本润色（PolishEpisode）时注入 LLM system prompt，
// 输出带有内联标注的 script_excerpt，格式如：[摄影:特写镜头] [字幕:对白内容]
type ProductionSkill struct {
	ID           int64          `gorm:"primaryKey;autoIncrement"              json:"id"`
	ProjectID    int64          `gorm:"not null;index:idx_ps_project_dept"    json:"project_id"`
	// department 固定枚举：dp|gaffer|art|prop|costume|sound|subtitle|director|editor|colorist
	Department   string         `gorm:"size:32;not null;index:idx_ps_project_dept" json:"department"`
	Name         string         `gorm:"size:128;not null"                     json:"name"`         // 中文部门名称，如"摄影指导"
	LabelTag     string         `gorm:"size:32;not null"                      json:"label_tag"`    // 内联标注标签，如"摄影"
	SystemPrompt string         `gorm:"type:text"                             json:"system_prompt"` // 注入 LLM 的指令
	IsActive     bool           `gorm:"default:true"                          json:"is_active"`
	SortOrder    int            `gorm:"default:0"                             json:"sort_order"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index"                                 json:"-"`
}

func (ProductionSkill) TableName() string { return "production_skills" }

// ─── 角色组（视频串行流程）─────────────────────────────────────────────────────

// CharacterGroup 角色组：一个"角色身份"（如李明），组内挂多个 Asset 造型变体。
// 音色绑定在角色组层级，所有造型共用同一音色。
type CharacterGroup struct {
	ID             int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	ProjectID      int64          `gorm:"not null;index"           json:"project_id"`
	Name           string         `gorm:"size:100;not null"        json:"name"`           // 角色名，与剧本中人物名对应
	Description    string         `gorm:"type:text"                json:"description"`
	VoiceModel     string         `gorm:"size:200"                 json:"voice_model"`     // TTS 音色 ID（角色级）
	VoiceSampleURL string         `gorm:"type:text"                json:"voice_sample_url"` // 试听参考音频 URL
	SortOrder      int            `gorm:"default:0"                json:"sort_order"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	Variants       []Asset        `gorm:"foreignKey:GroupID"       json:"variants,omitempty"` // 造型列表
}

func (CharacterGroup) TableName() string { return "character_groups" }

