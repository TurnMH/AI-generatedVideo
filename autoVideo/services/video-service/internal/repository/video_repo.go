package repository

import (
	"context"
	"time"

	"github.com/autovideo/video-service/internal/model"
	"gorm.io/gorm"
)

// VideoTaskRepo is the interface VideoService depends on.
// *VideoRepo satisfies this interface; tests can substitute a lightweight fake.
type VideoTaskRepo interface {
	// Task CRUD
	CreateTask(ctx context.Context, task *model.VideoTask) error
	GetTask(ctx context.Context, id int64) (*model.VideoTask, error)
	ListTasks(ctx context.Context, projectID, episodeID int64, page, pageSize int) ([]model.VideoTask, int64, error)
	UpdateTask(ctx context.Context, task *model.VideoTask) error
	UpdateTaskStatus(ctx context.Context, id int64, status, resultURL, errMsg string, durationSec float64) error
	SoftDeleteTask(ctx context.Context, taskID int64) error
	SetVariantGroupID(ctx context.Context, taskIDs []int64, groupID int64) error
	StatusCounts(ctx context.Context, projectID int64) (map[string]int, error)
	FindByStatus(ctx context.Context, status string, limit int) ([]model.VideoTask, error)
	FindFailedByProject(ctx context.Context, projectID int64, limit int) ([]model.VideoTask, error)
	FindStaleProcessing(ctx context.Context, olderThan time.Duration, limit int) ([]model.VideoTask, error)
	DeleteProjectData(ctx context.Context, projectID int64) error
	DeleteEpisodeData(ctx context.Context, projectID, episodeID int64) error

	// Clip operations
	CreateClip(ctx context.Context, clip *model.VideoClip) error
	UpdateClip(ctx context.Context, clip *model.VideoClip) error
	GetClipsByTaskID(ctx context.Context, taskID int64) ([]model.VideoClip, error)
	GetClipsByEpisode(ctx context.Context, projectID, episodeID int64) ([]model.VideoClip, error)
	DeleteClipsByTaskID(ctx context.Context, taskID int64) error
	UpdateComposeStage(ctx context.Context, taskID int64, stage string) error

	// Dubbing / audio lookup
	FindDubbingAudio(ctx context.Context, projectID int64, episodeID *int64) (audioURL string, subtitleURL string)
	FindDubbingVoiceConfig(ctx context.Context, projectID int64, episodeID *int64) (voiceModel, voiceRate, voicePitch, voiceVolume string)
}

// VideoRepo encapsulates database operations for video tasks and clips.
type VideoRepo struct {
	db *gorm.DB
}

// NewVideoRepo —— 创建视频仓库实例，返回 *VideoRepo
func NewVideoRepo(db *gorm.DB) *VideoRepo {
	return &VideoRepo{db: db}
}

// ----  VideoTask  ----

// CreateTask —— 在数据库中创建一条视频任务记录
func (r *VideoRepo) CreateTask(ctx context.Context, task *model.VideoTask) error {
	return r.db.WithContext(ctx).Create(task).Error
}

// GetTask —— 根据 ID 查询视频任务及其关联片段，返回 *VideoTask
func (r *VideoRepo) GetTask(ctx context.Context, id int64) (*model.VideoTask, error) {
	var task model.VideoTask
	err := r.db.WithContext(ctx).
		Preload("Clips").
		First(&task, id).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// ListTasks —— 分页查询视频任务列表，支持按项目和集数过滤，返回任务切片和总数
func (r *VideoRepo) ListTasks(ctx context.Context, projectID, episodeID int64, page, pageSize int) ([]model.VideoTask, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.VideoTask{})

	// Exclude soft-deleted tasks
	query = query.Where("deleted_at IS NULL")

	// Exclude only cancelled tasks so detail pages can still inspect failed runs.
	query = query.Where("status <> ?", model.StatusCancelled)

	if projectID > 0 {
		query = query.Where("project_id = ?", projectID)
	}
	if episodeID > 0 {
		query = query.Where("episode_id = ?", episodeID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var tasks []model.VideoTask
	offset := (page - 1) * pageSize
	err := query.Preload("Clips").Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&tasks).Error
	return tasks, total, err
}

// UpdateTaskStatus —— 更新任务的状态、结果 URL、错误信息和时长
func (r *VideoRepo) UpdateTaskStatus(ctx context.Context, id int64, status, resultURL, errMsg string, durationSec float64) error {
	updates := map[string]any{
		"status":    status,
		"error_msg": errMsg,
	}
	if resultURL != "" {
		updates["result_url"] = resultURL
	}
	if durationSec > 0 {
		updates["duration_sec"] = durationSec
	}
	return r.db.WithContext(ctx).Model(&model.VideoTask{}).Where("id = ?", id).Updates(updates).Error
}

// ----  VideoClip  ----

// CreateClip —— 在数据库中创建一条视频片段记录
func (r *VideoRepo) CreateClip(ctx context.Context, clip *model.VideoClip) error {
	return r.db.WithContext(ctx).Create(clip).Error
}

// GetClipsByTaskID —— 根据任务 ID 查询其所有片段，按顺序返回
func (r *VideoRepo) GetClipsByTaskID(ctx context.Context, taskID int64) ([]model.VideoClip, error) {
	var clips []model.VideoClip
	err := r.db.WithContext(ctx).
		Where("video_task_id = ?", taskID).
		Order("clip_order").
		Find(&clips).Error
	return clips, err
}

// UpdateClip —— 更新视频片段的所有字段
func (r *VideoRepo) UpdateClip(ctx context.Context, clip *model.VideoClip) error {
	return r.db.WithContext(ctx).Save(clip).Error
}

// UpdateTask —— 保存视频任务的所有字段到数据库
// UpdateTask saves all fields of a task.
func (r *VideoRepo) UpdateTask(ctx context.Context, task *model.VideoTask) error {
	return r.db.WithContext(ctx).Save(task).Error
}

// DeleteClipsByTaskID —— 删除指定任务的所有片段记录（用于重试前清理）
// DeleteClipsByTaskID removes all clips for a task (for clean retries).
func (r *VideoRepo) DeleteClipsByTaskID(ctx context.Context, taskID int64) error {
	return r.db.WithContext(ctx).Where("video_task_id = ?", taskID).Delete(&model.VideoClip{}).Error
}

// SoftDeleteTask —— 软删除任务，设置 deleted_at 时间戳
// SoftDeleteTask marks a task as deleted by setting deleted_at.
func (r *VideoRepo) SoftDeleteTask(ctx context.Context, taskID int64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.VideoTask{}).Where("id = ?", taskID).Update("deleted_at", now).Error
}

// UpdateComposeStage —— 更新任务的合成阶段字段，用于进度跟踪
// UpdateComposeStage updates the compose_stage field for progress tracking.
func (r *VideoRepo) UpdateComposeStage(ctx context.Context, taskID int64, stage string) error {
	return r.db.WithContext(ctx).Model(&model.VideoTask{}).Where("id = ?", taskID).Update("compose_stage", stage).Error
}

// SetVariantGroupID —— 批量设置任务的 variant_group_id（feat-6 多版本关联）
// SetVariantGroupID updates the variant_group_id for a list of task IDs.
func (r *VideoRepo) SetVariantGroupID(ctx context.Context, taskIDs []int64, groupID int64) error {
	return r.db.WithContext(ctx).Model(&model.VideoTask{}).Where("id IN ?", taskIDs).Update("variant_group_id", groupID).Error
}

// FindByStatus —— 按状态查询视频任务，返回最多 limit 条记录
// FindByStatus returns tasks matching a given status.
func (r *VideoRepo) FindByStatus(ctx context.Context, status string, limit int) ([]model.VideoTask, error) {
	var tasks []model.VideoTask
	err := r.db.WithContext(ctx).Where("status = ?", status).Limit(limit).Find(&tasks).Error
	return tasks, err
}

// FindStaleProcessing —— 查询 updated_at 超过 olderThan 仍处于 processing 状态的任务
// FindStaleProcessing returns processing tasks that haven't been updated for longer than olderThan.
func (r *VideoRepo) FindStaleProcessing(ctx context.Context, olderThan time.Duration, limit int) ([]model.VideoTask, error) {
	var tasks []model.VideoTask
	cutoff := time.Now().Add(-olderThan)
	err := r.db.WithContext(ctx).
		Where("status = ? AND updated_at < ?", model.StatusProcessing, cutoff).
		Limit(limit).Find(&tasks).Error
	return tasks, err
}

// FindFailedByProject —— 查询指定项目下的失败任务，返回最多 limit 条
// FindFailedByProject returns failed tasks for a project.
func (r *VideoRepo) FindFailedByProject(ctx context.Context, projectID int64, limit int) ([]model.VideoTask, error) {
	var tasks []model.VideoTask
	err := r.db.WithContext(ctx).
		Where("project_id = ? AND status = ?", projectID, model.StatusFailed).
		Limit(limit).Find(&tasks).Error
	return tasks, err
}

// FindDubbingAudio —— 查找项目下最新成功的配音任务的音频和字幕 URL
// FindDubbingAudio looks up the latest succeeded dubbing task's audio_url
// for the given project and optional episode. Returns empty string if none found.
func (r *VideoRepo) FindDubbingAudio(ctx context.Context, projectID int64, episodeID *int64) (audioURL string, subtitleURL string) {
	var task model.DubbingTask
	q := r.db.WithContext(ctx).
		Where("project_id = ? AND task_type = 'dubbing' AND status = 'succeeded' AND audio_url != ''", projectID)
	if episodeID != nil && *episodeID > 0 {
		q = q.Where("episode_id = ?", *episodeID)
	}
	if err := q.Order("created_at DESC").First(&task).Error; err != nil {
		return "", ""
	}
	return task.AudioURL, task.SubtitleURL
}

// FindDubbingVoiceConfig returns the voice configuration (model/rate/pitch/volume)
// from the latest dubbing task for the given project/episode, or zero-valued
// strings when nothing is found. Used by per-clip audio synthesis to inherit
// the voice the user already picked for bulk dubbing.
func (r *VideoRepo) FindDubbingVoiceConfig(ctx context.Context, projectID int64, episodeID *int64) (voiceModel, voiceRate, voicePitch, voiceVolume string) {
	var task model.DubbingTask
	q := r.db.WithContext(ctx).
		Where("project_id = ? AND task_type = 'dubbing'", projectID)
	if episodeID != nil && *episodeID > 0 {
		q = q.Where("episode_id = ?", *episodeID)
	}
	if err := q.Order("created_at DESC").First(&task).Error; err != nil {
		return "", "", "", ""
	}
	return task.VoiceModel, task.VoiceRate, task.VoicePitch, task.VoiceVolume
}

// StatusCounts —— 按状态分组统计项目下的任务数量，返回 map[status]count
// StatusCounts returns task counts grouped by status for a project.
func (r *VideoRepo) StatusCounts(ctx context.Context, projectID int64) (map[string]int, error) {
	type row struct {
		Status string
		Count  int
	}
	var rows []row
	err := r.db.WithContext(ctx).Model(&model.VideoTask{}).
		Select("status, count(*) as count").
		Where("project_id = ?", projectID).
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]int, len(rows))
	for _, r := range rows {
		m[r.Status] = r.Count
	}
	return m, nil
}

// DeleteProjectData —— 删除项目下所有视频任务、片段和配音任务
func (r *VideoRepo) DeleteProjectData(ctx context.Context, projectID int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		subQuery := tx.Model(&model.VideoTask{}).Select("id").Where("project_id = ?", projectID)
		if err := tx.Where("video_task_id IN (?)", subQuery).Delete(&model.VideoClip{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ?", projectID).Delete(&model.DubbingTask{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ?", projectID).Delete(&model.VideoTask{}).Error; err != nil {
			return err
		}
		return nil
	})
}

// GetClipsByEpisode —— 按 project_id + episode_id 查询所有已生成的 VideoClip，按 clip_order 排序
func (r *VideoRepo) GetClipsByEpisode(ctx context.Context, projectID, episodeID int64) ([]model.VideoClip, error) {
	subQuery := r.db.WithContext(ctx).Model(&model.VideoTask{}).Select("id").
		Where("project_id = ? AND episode_id = ? AND deleted_at IS NULL", projectID, episodeID)
	var clips []model.VideoClip
	err := r.db.WithContext(ctx).
		Where("video_task_id IN (?) AND clip_url != ''", subQuery).
		Order("clip_order ASC").
		Find(&clips).Error
	return clips, err
}

// DeleteEpisodeData —— 删除指定剧集下所有视频任务、片段和配音任务，用于剧集删除时级联清理
func (r *VideoRepo) DeleteEpisodeData(ctx context.Context, projectID, episodeID int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		subQuery := tx.Model(&model.VideoTask{}).Select("id").
			Where("project_id = ? AND episode_id = ?", projectID, episodeID)
		if err := tx.Where("video_task_id IN (?)", subQuery).Delete(&model.VideoClip{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ? AND episode_id = ?", projectID, episodeID).Delete(&model.DubbingTask{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ? AND episode_id = ?", projectID, episodeID).Delete(&model.VideoTask{}).Error; err != nil {
			return err
		}
		return nil
	})
}
