package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/lib/pq"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/autovideo/project-service/internal/model"
	"github.com/autovideo/project-service/internal/repository"
	"github.com/autovideo/project-service/pkg/textdecode"
)

// Valid status transitions (state machine).
var validTransitions = map[string][]string{
	"draft":                 {"script_processing", "paused", "failed"},
	"script_processing":     {"script_processing", "script_ready", "failed", "paused"},
	"script_ready":          {"script_processing", "asset_generating", "paused", "failed"},
	"asset_generating":      {"script_processing", "asset_ready", "failed", "paused"},
	"asset_ready":           {"script_processing", "storyboard_generating", "paused", "failed"},
	"storyboard_generating": {"script_processing", "storyboard_ready", "failed", "paused"},
	"storyboard_ready":      {"script_processing", "video_generating", "paused", "failed"},
	"video_generating":      {"script_processing", "completed", "failed", "paused"},
	"completed":             {"script_processing", "paused", "failed"},
	"failed":                {"script_processing", "paused"},
	"paused":                {}, // handled specially — can go back to previous state
}

// pausableStatuses are the statuses a project can be resumed to from "paused".
var pausableStatuses = map[string]bool{
	"draft": true, "script_processing": true, "script_ready": true,
	"asset_generating": true, "asset_ready": true, "storyboard_generating": true,
	"storyboard_ready": true, "video_generating": true, "completed": true, "failed": true,
}

type CreateProjectReq struct {
	UserID              uint64                 `json:"user_id"`
	Title               string                 `json:"title" binding:"required,max=50"`
	Description         string                 `json:"description"`
	ProjectType         string                 `json:"project_type"`
	CoverURL            string                 `json:"cover_url"`
	LogoURL             string                 `json:"logo_url"`
	TargetEpisodes      int                    `json:"target_episodes"`
	StyleTags           []string               `json:"style_tags"`
	TextModelID         *uint64                `json:"text_model_id"`
	ImageModelID        *uint64                `json:"image_model_id"`
	VideoModelID        *uint64                `json:"video_model_id"`
	TTSModelID          *uint64                `json:"tts_model_id"`
	EnableDubbing       bool                   `json:"enable_dubbing"`
	EnableSubtitle      bool                   `json:"enable_subtitle"`
	VideoMode           string                 `json:"video_mode"`
	StoryboardConfig    map[string]interface{} `json:"storyboard_config"`
	WatermarkConfig     map[string]interface{} `json:"watermark_config"`
	ConsistencyStrength float64                `json:"consistency_strength"`
	ImageStylePrefix    string                 `json:"image_style_prefix"`
	// Mode: "script"（默认，完整流程）/ "storyboard"（直接上传分镜，跳过剧本步骤）
	Mode                string                 `json:"mode"`
}

type UpdateProjectReq struct {
	Title               *string                 `json:"title"`
	Description         *string                 `json:"description"`
	ProjectType         *string                 `json:"project_type"`
	CoverURL            *string                 `json:"cover_url"`
	LogoURL             *string                 `json:"logo_url"`
	Status              *string                 `json:"status"`
	TargetEpisodes      *int                    `json:"target_episodes"`
	StyleTags           *[]string               `json:"style_tags"`
	TextModelID         *uint64                 `json:"text_model_id"`
	ImageModelID        *uint64                 `json:"image_model_id"`
	VideoModelID        *uint64                 `json:"video_model_id"`
	TTSModelID          *uint64                 `json:"tts_model_id"`
	EnableDubbing       *bool                   `json:"enable_dubbing"`
	EnableSubtitle      *bool                   `json:"enable_subtitle"`
	VideoMode           *string                 `json:"video_mode"`
	StoryboardConfig    *map[string]interface{} `json:"storyboard_config"`
	WatermarkConfig     *map[string]interface{} `json:"watermark_config"`
	ConsistencyStrength *float64                `json:"consistency_strength"`
	ImageStylePrefix    *string                 `json:"image_style_prefix"`
}

type ListProjectsReq struct {
	UserID      uint64
	Keyword     string
	Status      string
	ProjectType string
	Page        int
	PageSize    int
}

var allowedProjectTypes = map[string]bool{
	"video":        true,
	"video_serial": true,
	"comics":       true,
	"music":        true,
	"image":        true,
}

func normalizeProjectType(projectType string) string {
	switch projectType {
	case "comics", "music", "image", "video", "video_serial":
		return projectType
	default:
		return "video"
	}
}

type ProjectService struct {
	projectRepo       *repository.ProjectRepo
	episodeRepo       *repository.EpisodeRepo
	storyboardRepo    *repository.StoryboardRepo
	snapshotRepo      *repository.SnapshotRepo
	scriptVersionRepo *repository.ScriptVersionRepo
}

// NewProjectService —— 创建项目服务实例
func NewProjectService(
	projectRepo *repository.ProjectRepo,
	episodeRepo *repository.EpisodeRepo,
	storyboardRepo *repository.StoryboardRepo,
	snapshotRepo *repository.SnapshotRepo,
	scriptVersionRepo *repository.ScriptVersionRepo,
) *ProjectService {
	return &ProjectService{
		projectRepo:       projectRepo,
		episodeRepo:       episodeRepo,
		storyboardRepo:    storyboardRepo,
		snapshotRepo:      snapshotRepo,
		scriptVersionRepo: scriptVersionRepo,
	}
}

// Create —— 创建新项目，设置默认值并持久化
func (s *ProjectService) Create(req CreateProjectReq) (*model.Project, error) {
	videoMode := req.VideoMode
	if videoMode == "" {
		videoMode = "frame_animation"
	}
	projectType := normalizeProjectType(req.ProjectType)
	consistencyStrength := req.ConsistencyStrength
	if consistencyStrength == 0 {
		consistencyStrength = 0.75
	}

	var storyboardJSON, watermarkJSON datatypes.JSON
	if req.StoryboardConfig != nil {
		b, _ := json.Marshal(req.StoryboardConfig)
		storyboardJSON = datatypes.JSON(b)
	} else {
		storyboardJSON = datatypes.JSON([]byte("{}"))
	}
	if req.WatermarkConfig != nil {
		b, _ := json.Marshal(req.WatermarkConfig)
		watermarkJSON = datatypes.JSON(b)
	} else {
		watermarkJSON = datatypes.JSON([]byte(`{"enabled":false}`))
	}

	mode := req.Mode
	if mode != "storyboard" {
		mode = "script" // default to full pipeline
	}

	project := &model.Project{
		UserID:              req.UserID,
		Title:               req.Title,
		Description:         req.Description,
		ProjectType:         projectType,
		CoverURL:            req.CoverURL,
		LogoURL:             req.LogoURL,
		Status:              "draft",
		TargetEpisodes:      req.TargetEpisodes,
		StyleTags:           pq.StringArray(req.StyleTags),
		TextModelID:         req.TextModelID,
		ImageModelID:        req.ImageModelID,
		VideoModelID:        req.VideoModelID,
		TTSModelID:          req.TTSModelID,
		EnableDubbing:       req.EnableDubbing,
		EnableSubtitle:      req.EnableSubtitle,
		VideoMode:           videoMode,
		StoryboardConfig:    storyboardJSON,
		WatermarkConfig:     watermarkJSON,
		ConsistencyStrength: consistencyStrength,
		ImageStylePrefix:    req.ImageStylePrefix,
		Mode:                mode,
		ScriptVersions:      datatypes.JSON([]byte("[]")),
		Progress:            datatypes.JSON([]byte("{}")),
	}
	if err := s.projectRepo.Create(project); err != nil {
		return nil, err
	}
	return project, nil
}

// Get —— 按 ID 和用户 ID 获取项目详情
func (s *ProjectService) Get(id, userID uint64) (*model.Project, error) {
	project, err := s.projectRepo.FindByID(id, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("project not found")
		}
		return nil, err
	}
	if err := s.hydrateScriptText(project); err != nil {
		return nil, err
	}
	return project, nil
}

// GetNoAuth retrieves a project by ID without user ownership check.
// Used for service-to-service calls where ownership enforcement is not needed.
func (s *ProjectService) GetNoAuth(id uint64) (*model.Project, error) {
	project, err := s.projectRepo.FindByIDNoAuth(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("project not found")
		}
		return nil, err
	}
	if err := s.hydrateScriptText(project); err != nil {
		return nil, err
	}
	return project, nil
}

// List —— 分页查询用户的项目列表
func (s *ProjectService) List(req ListProjectsReq) ([]model.Project, int64, error) {
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	projectType := ""
	if req.ProjectType != "" {
		projectType = normalizeProjectType(req.ProjectType)
	}
	return s.projectRepo.List(req.UserID, req.Keyword, req.Status, projectType, page, pageSize)
}

// Update —— 按请求字段局部更新项目信息
func (s *ProjectService) Update(id, userID uint64, req UpdateProjectReq) (*model.Project, error) {
	project, err := s.projectRepo.FindByID(id, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("project not found")
		}
		return nil, err
	}

	if req.Title != nil {
		project.Title = *req.Title
	}
	if req.Description != nil {
		project.Description = *req.Description
	}
	if req.ProjectType != nil {
		project.ProjectType = normalizeProjectType(*req.ProjectType)
	}
	if req.CoverURL != nil {
		project.CoverURL = *req.CoverURL
	}
	if req.LogoURL != nil {
		project.LogoURL = *req.LogoURL
	}
	if req.Status != nil {
		project.Status = *req.Status
	}
	if req.TargetEpisodes != nil {
		project.TargetEpisodes = *req.TargetEpisodes
	}
	if req.StyleTags != nil {
		project.StyleTags = pq.StringArray(*req.StyleTags)
	}
	if req.TextModelID != nil {
		project.TextModelID = req.TextModelID
	}
	if req.ImageModelID != nil {
		project.ImageModelID = req.ImageModelID
	}
	if req.VideoModelID != nil {
		project.VideoModelID = req.VideoModelID
	}
	if req.TTSModelID != nil {
		project.TTSModelID = req.TTSModelID
	}
	if req.EnableDubbing != nil {
		project.EnableDubbing = *req.EnableDubbing
	}
	if req.EnableSubtitle != nil {
		project.EnableSubtitle = *req.EnableSubtitle
	}
	if req.VideoMode != nil {
		project.VideoMode = *req.VideoMode
	}
	if req.StoryboardConfig != nil {
		b, _ := json.Marshal(*req.StoryboardConfig)
		project.StoryboardConfig = datatypes.JSON(b)
	}
	if req.WatermarkConfig != nil {
		b, _ := json.Marshal(*req.WatermarkConfig)
		project.WatermarkConfig = datatypes.JSON(b)
	}
	if req.ConsistencyStrength != nil {
		project.ConsistencyStrength = *req.ConsistencyStrength
	}
	if req.ImageStylePrefix != nil {
		project.ImageStylePrefix = *req.ImageStylePrefix
	}
	project.UpdatedAt = time.Now()

	if err := s.projectRepo.Update(project); err != nil {
		return nil, err
	}
	return project, nil
}

// Delete —— 软删除指定项目
func (s *ProjectService) Delete(id, userID uint64) error {
	if _, err := s.projectRepo.FindByID(id, userID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("project not found")
		}
		return err
	}

	if err := s.storyboardRepo.DeleteByProjectID(id); err != nil {
		return err
	}
	if err := s.episodeRepo.DeleteByProjectID(id); err != nil {
		return err
	}
	if err := s.snapshotRepo.DeleteByProjectID(id); err != nil {
		return err
	}
	if err := s.scriptVersionRepo.DeleteByProjectID(id); err != nil {
		return err
	}

	err := s.projectRepo.SoftDelete(id, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("project not found")
	}
	return err
}

// UpdateStatus —— 校验状态机后更新项目状态
// UpdateStatus validates and applies a status transition using the state machine.
func (s *ProjectService) UpdateStatus(id, userID uint64, newStatus string) error {
	project, err := s.projectRepo.FindByID(id, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("project not found")
		}
		return err
	}

	allowed, ok := validTransitions[project.Status]
	if !ok {
		return fmt.Errorf("unknown current status: %s", project.Status)
	}

	valid := false
	for _, s := range allowed {
		if s == newStatus {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid status transition from %s to %s", project.Status, newStatus)
	}

	return s.projectRepo.UpdateStatus(id, userID, newStatus)
}

// Pause —— 暂停项目，将当前状态保存到 progress 中
// Pause sets the project status to "paused", storing the previous status in progress JSONB.
func (s *ProjectService) Pause(id, userID uint64) error {
	project, err := s.projectRepo.FindByID(id, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("project not found")
		}
		return err
	}

	if project.Status == "paused" {
		return errors.New("project is already paused")
	}

	// Store previous status in progress JSON for resume.
	var progressMap map[string]interface{}
	if len(project.Progress) > 0 {
		_ = json.Unmarshal(project.Progress, &progressMap)
	}
	if progressMap == nil {
		progressMap = make(map[string]interface{})
	}
	progressMap["previous_status"] = project.Status
	b, _ := json.Marshal(progressMap)
	project.Progress = datatypes.JSON(b)
	project.Status = "paused"
	project.UpdatedAt = time.Now()

	return s.projectRepo.Update(project)
}

// Resume —— 恢复已暂停的项目到之前的状态
// Resume restores the project to its previous status before being paused.
func (s *ProjectService) Resume(id, userID uint64) error {
	project, err := s.projectRepo.FindByID(id, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("project not found")
		}
		return err
	}

	if project.Status != "paused" {
		return errors.New("project is not paused")
	}

	var progressMap map[string]interface{}
	if len(project.Progress) > 0 {
		_ = json.Unmarshal(project.Progress, &progressMap)
	}

	prevStatus, _ := progressMap["previous_status"].(string)
	if prevStatus == "" || !pausableStatuses[prevStatus] {
		prevStatus = "draft"
	}

	delete(progressMap, "previous_status")
	b, _ := json.Marshal(progressMap)
	project.Progress = datatypes.JSON(b)
	project.Status = prevStatus
	project.UpdatedAt = time.Now()

	return s.projectRepo.Update(project)
}

// Clone —— 复制项目及其配置，返回新的草稿项目
// Clone duplicates a project and its episodes for the same user.
func (s *ProjectService) Clone(id, userID uint64) (*model.Project, error) {
	project, err := s.projectRepo.FindByID(id, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("project not found")
		}
		return nil, err
	}

	clone := &model.Project{
		UserID:              project.UserID,
		Title:               project.Title + " (copy)",
		Description:         project.Description,
		ProjectType:         project.ProjectType,
		CoverURL:            project.CoverURL,
		LogoURL:             project.LogoURL,
		ScriptFileURL:       project.ScriptFileURL,
		ScriptText:          project.ScriptText,
		ScriptFileSize:      project.ScriptFileSize,
		ScriptVersions:      project.ScriptVersions,
		Status:              "draft",
		Progress:            datatypes.JSON([]byte("{}")),
		TargetEpisodes:      project.TargetEpisodes,
		StyleTags:           project.StyleTags,
		TextModelID:         project.TextModelID,
		ImageModelID:        project.ImageModelID,
		VideoModelID:        project.VideoModelID,
		TTSModelID:          project.TTSModelID,
		EnableDubbing:       project.EnableDubbing,
		EnableSubtitle:      project.EnableSubtitle,
		VideoMode:           project.VideoMode,
		StoryboardConfig:    project.StoryboardConfig,
		WatermarkConfig:     project.WatermarkConfig,
		ConsistencyStrength: project.ConsistencyStrength,
	}

	if err := s.projectRepo.Create(clone); err != nil {
		return nil, err
	}
	return clone, nil
}

// GetScriptVersions —— 获取项目的所有剧本版本列表
// GetScriptVersions returns all script versions for a project.
func (s *ProjectService) GetScriptVersions(projectID, userID uint64) ([]model.ScriptVersion, error) {
	_, err := s.projectRepo.FindByID(projectID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("project not found")
		}
		return nil, err
	}
	return s.scriptVersionRepo.ListByProjectID(projectID)
}

// SwitchScriptVersion —— 将指定版本设为当前剧本版本
// SwitchScriptVersion sets a specific version as the current script version.
func (s *ProjectService) SwitchScriptVersion(projectID, userID, versionID uint64) error {
	project, err := s.projectRepo.FindByID(projectID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("project not found")
		}
		return err
	}

	sv, err := s.scriptVersionRepo.FindByID(versionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("script version not found")
		}
		return err
	}
	if sv.ProjectID != projectID {
		return errors.New("script version does not belong to this project")
	}

	if err := s.scriptVersionRepo.SetCurrent(projectID, versionID); err != nil {
		return err
	}

	project.ScriptFileURL = sv.FileURL
	project.ScriptText = ""
	if err := s.hydrateScriptText(project); err != nil {
		return err
	}
	return s.projectRepo.Update(project)
}

// CreateSnapshot —— 将项目完整状态序列化为 JSON 快照并保存
// CreateSnapshot captures the full project state (including episodes) as a JSON snapshot.
func (s *ProjectService) CreateSnapshot(projectID, userID uint64) (*model.ProjectSnapshot, error) {
	project, err := s.projectRepo.FindByID(projectID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("project not found")
		}
		return nil, err
	}

	data, err := json.Marshal(project)
	if err != nil {
		return nil, err
	}

	version := s.snapshotRepo.NextVersion(projectID)
	snapshot := &model.ProjectSnapshot{
		ProjectID:    projectID,
		Version:      version,
		SnapshotData: datatypes.JSON(data),
	}

	if err := s.snapshotRepo.Create(snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

// ListSnapshots —— 获取项目的所有快照列表
func (s *ProjectService) ListSnapshots(projectID, userID uint64) ([]model.ProjectSnapshot, error) {
	_, err := s.projectRepo.FindByID(projectID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("project not found")
		}
		return nil, err
	}

	return s.snapshotRepo.ListByProjectID(projectID)
}

// UploadScript —— 保存剧本内容与文件 URL，并将项目置为待手动分集状态
// UploadScript saves the uploaded script and leaves the project ready for manual episode generation.
func (s *ProjectService) UploadScript(projectID, userID uint64, fileURL, filename string, fileSize int64, scriptText string) error {
	project, err := s.projectRepo.FindByID(projectID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("project not found")
		}
		return err
	}

	// Update project fields
	project.ScriptFileURL = fileURL
	project.ScriptText = scriptText
	project.ScriptFileSize = int(fileSize)
	project.Status = "script_ready"
	project.Progress = datatypes.JSON([]byte("{}"))
	if err := s.projectRepo.Update(project); err != nil {
		return fmt.Errorf("update project script url: %w", err)
	}

	// Mark previous versions as not current
	if err := s.scriptVersionRepo.SetAllNotCurrent(projectID); err != nil {
		return fmt.Errorf("reset script versions: %w", err)
	}

	// Get next version number
	nextVer := s.scriptVersionRepo.NextVersionNumber(projectID)

	// Create a new script version record
	sv := &model.ScriptVersion{
		ProjectID:     projectID,
		VersionNumber: nextVer,
		FileURL:       fileURL,
		FileSize:      int(fileSize),
		OSSKey:        filename,
		IsCurrent:     true,
	}
	if err := s.scriptVersionRepo.Create(sv); err != nil {
		return fmt.Errorf("create script version: %w", err)
	}
	return nil
}

func (s *ProjectService) hydrateScriptText(project *model.Project) error {
	if project == nil || project.ScriptFileURL == "" || project.ScriptText != "" {
		return nil
	}

	req, err := http.NewRequest(http.MethodGet, project.ScriptFileURL, nil)
	if err != nil {
		return fmt.Errorf("build script fetch request: %w", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch script file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch script file: status %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read script file: %w", err)
	}

	decoded, err := textdecode.Decode(raw)
	if err != nil {
		return fmt.Errorf("decode script file: %w", err)
	}

	project.ScriptText = decoded
	if err := s.projectRepo.Update(project); err != nil {
		return fmt.Errorf("persist decoded script text: %w", err)
	}
	return nil
}
