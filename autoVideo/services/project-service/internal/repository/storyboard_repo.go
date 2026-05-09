package repository

import (
	"strings"
	"time"

	"github.com/autovideo/project-service/internal/model"
	"gorm.io/gorm"
)

type StoryboardRepo struct {
	db *gorm.DB
}

func applyStoryboardKeywordFilter(query *gorm.DB, keyword string) *gorm.DB {
	keyword = "%" + strings.TrimSpace(keyword) + "%"
	return query.Where("(scene_description ILIKE ? OR location ILIKE ? OR dialogue ILIKE ? OR array_to_string(characters, ' ') ILIKE ?)", keyword, keyword, keyword, keyword)
}

// NewStoryboardRepo —— 创建分镜仓储实例
func NewStoryboardRepo(db *gorm.DB) *StoryboardRepo {
	return &StoryboardRepo{db: db}
}

// Create —— 新增一条分镜记录到数据库
func (r *StoryboardRepo) Create(sb *model.Storyboard) error {
	return r.db.Create(sb).Error
}

// FindByID —— 按主键查询单条分镜记录，预加载版本列表
func (r *StoryboardRepo) FindByID(id uint64) (*model.Storyboard, error) {
	var sb model.Storyboard
	err := r.db.Preload("Versions").First(&sb, id).Error
	if err != nil {
		return nil, err
	}
	return &sb, nil
}

// FindByProjectID —— 按项目 ID 分页查询分镜列表，支持按集和状态过滤
func (r *StoryboardRepo) FindByProjectID(projectID uint64, episodeID *uint64, status, keyword string, page, pageSize int, includeVersions bool) ([]model.Storyboard, int64, error) {
	var storyboards []model.Storyboard
	var total int64

	query := r.db.Model(&model.Storyboard{}).Where("project_id = ?", projectID)
	if episodeID != nil {
		query = query.Where("episode_id = ?", *episodeID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if strings.TrimSpace(keyword) != "" {
		query = applyStoryboardKeywordFilter(query, keyword)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	query = query.Offset(offset).Limit(pageSize).Order("sequence_number ASC")
	if includeVersions {
		query = query.Preload("Versions")
	}
	err := query.Find(&storyboards).Error
	if err != nil {
		return nil, 0, err
	}

	return storyboards, total, nil
}

// Update —— 保存分镜记录的全部字段更新
func (r *StoryboardRepo) Update(sb *model.Storyboard) error {
	return r.db.Save(sb).Error
}

// VoidByID —— 按 ID 作废单条分镜
func (r *StoryboardRepo) VoidByID(id uint64) error {
	return r.db.Model(&model.Storyboard{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_voided": true,
			"status":    "voided",
		}).Error
}

// VoidByEpisodeID —— 作废指定剧集下的所有分镜
func (r *StoryboardRepo) VoidByEpisodeID(episodeID uint64) error {
	return r.db.Model(&model.Storyboard{}).Where("episode_id = ?", episodeID).
		Updates(map[string]interface{}{
			"is_voided": true,
			"status":    "voided",
		}).Error
}

// VoidAll —— 作废指定项目下的所有分镜
func (r *StoryboardRepo) VoidAll(projectID uint64) error {
	return r.db.Model(&model.Storyboard{}).Where("project_id = ?", projectID).
		Updates(map[string]interface{}{
			"is_voided": true,
			"status":    "voided",
		}).Error
}

// CreateVersion —— 新增一条分镜版本记录
func (r *StoryboardRepo) CreateVersion(ver *model.StoryboardVersion) error {
	return r.db.Create(ver).Error
}

// GetVersions —— 查询指定分镜的所有版本，按版本号倒序
func (r *StoryboardRepo) GetVersions(storyboardID uint64) ([]model.StoryboardVersion, error) {
	var versions []model.StoryboardVersion
	err := r.db.Where("storyboard_id = ?", storyboardID).
		Order("version_number DESC").
		Find(&versions).Error
	return versions, err
}

// SwitchVersion —— 在事务中切换分镜的当前版本
func (r *StoryboardRepo) SwitchVersion(storyboardID, versionID uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.StoryboardVersion{}).
			Where("storyboard_id = ?", storyboardID).
			Update("is_current", false).Error; err != nil {
			return err
		}
		return tx.Model(&model.StoryboardVersion{}).
			Where("id = ? AND storyboard_id = ?", versionID, storyboardID).
			Update("is_current", true).Error
	})
}

// DeleteVersion —— 按 ID 删除分镜版本记录
func (r *StoryboardRepo) DeleteVersion(versionID uint64) error {
	return r.db.Delete(&model.StoryboardVersion{}, versionID).Error
}

// UpdateVersion —— 保存分镜版本记录的全部字段更新
func (r *StoryboardRepo) UpdateVersion(ver *model.StoryboardVersion) error {
	return r.db.Save(ver).Error
}

// CountVersions —— 统计指定分镜的版本数量
func (r *StoryboardRepo) CountVersions(storyboardID uint64) (int, error) {
	var count int64
	err := r.db.Model(&model.StoryboardVersion{}).
		Where("storyboard_id = ?", storyboardID).
		Count(&count).Error
	return int(count), err
}

// StatusCounts —— 按状态分组统计项目下分镜数量
// StatusCounts returns the number of storyboards per status for a project.
func (r *StoryboardRepo) StatusCounts(projectID uint64) (map[string]int64, error) {
	type row struct {
		Status string
		Cnt    int64
	}
	var rows []row
	err := r.db.Model(&model.Storyboard{}).
		Select("status, count(*) as cnt").
		Where("project_id = ?", projectID).
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]int64, len(rows))
	for _, r := range rows {
		m[r.Status] = r.Cnt
	}
	return m, nil
}

// CountByProjectAndStatus returns the number of storyboards in the given status for a project.
func (r *StoryboardRepo) CountByProjectAndStatus(projectID uint64, status string) (int64, error) {
	var count int64
	err := r.db.Model(&model.Storyboard{}).
		Where("project_id = ? AND status = ?", projectID, status).
		Count(&count).Error
	return count, err
}

// ResetGeneratingToPending —— 将 generating 状态的分镜重置为 pending
// ResetGeneratingToPending resets all "generating" storyboards back to "pending".
func (r *StoryboardRepo) ResetGeneratingToPending() (int64, error) {
	res := r.db.Model(&model.Storyboard{}).
		Where("status = ?", "generating").
		Updates(map[string]interface{}{
			"status": "pending",
		})
	return res.RowsAffected, res.Error
}

// ResetStaleGenerating —— 将超时的 generating 状态分镜重置为 failed
// ResetStaleGenerating marks storyboards stuck in "generating" longer than the threshold as failed.
func (r *StoryboardRepo) ResetStaleGenerating(threshold time.Duration) (int64, error) {
	cutoff := time.Now().Add(-threshold)
	res := r.db.Model(&model.Storyboard{}).
		Where("status = ? AND updated_at < ?", "generating", cutoff).
		Updates(map[string]interface{}{
			"status":     "failed",
			"error_msg":  "storyboard generation timed out (stale cleanup)",
			"updated_at": "now()",
		})
	return res.RowsAffected, res.Error
}

// FindPendingOrFailed —— 查询项目下待生成或失败的分镜列表
// FindPendingOrFailed returns storyboards eligible for (re)generation.
func (r *StoryboardRepo) FindPendingOrFailed(projectID uint64, limit int) ([]model.Storyboard, error) {
	var sbs []model.Storyboard
	err := r.db.Where("project_id = ? AND status IN ?", projectID, []string{"pending", "failed"}).
		Order("id ASC").
		Limit(limit).
		Find(&sbs).Error
	return sbs, err
}

// FindByProjectAndStatuses returns a bounded set of storyboards eligible for dispatch.
func (r *StoryboardRepo) FindByProjectAndStatuses(projectID uint64, episodeID *uint64, statuses []string, limit int) ([]model.Storyboard, error) {
	var sbs []model.Storyboard
	query := r.db.Where("project_id = ? AND status IN ?", projectID, statuses)
	if episodeID != nil {
		query = query.Where("episode_id = ?", *episodeID)
	}
	err := query.
		Order("sequence_number ASC, id ASC").
		Limit(limit).
		Find(&sbs).Error
	return sbs, err
}

// ResetEpisodeCompletedToPending resets completed (non-voided) storyboards for an episode to
// pending and clears their image URLs, allowing force-regeneration.
func (r *StoryboardRepo) ResetEpisodeCompletedToPending(projectID, episodeID uint64) (int64, error) {
	result := r.db.Model(&model.Storyboard{}).
		Where("project_id = ? AND episode_id = ? AND status = ? AND is_voided = ?", projectID, episodeID, "completed", false).
		Updates(map[string]interface{}{"status": "pending", "image_url": "", "error_message": ""})
	return result.RowsAffected, result.Error
}

// FindAdjacentBySequence returns the nearest preceding and following storyboards for
// visual continuity context injection. Both results can be nil if no neighbours exist.
func (r *StoryboardRepo) FindAdjacentBySequence(projectID uint64, sequenceNumber int, episodeID *uint64) (prev *model.Storyboard, next *model.Storyboard) {
	base := r.db.Model(&model.Storyboard{}).Where("project_id = ? AND is_voided = false", projectID)
	if episodeID != nil {
		base = base.Where("episode_id = ?", *episodeID)
	}
	var prevSB, nextSB model.Storyboard
	if err := base.Where("sequence_number < ?", sequenceNumber).
		Order("sequence_number DESC").Limit(1).First(&prevSB).Error; err == nil {
		prev = &prevSB
	}
	if err := base.Where("sequence_number > ?", sequenceNumber).
		Order("sequence_number ASC").Limit(1).First(&nextSB).Error; err == nil {
		next = &nextSB
	}
	return prev, next
}

// FindPrecedingWithImage returns the nearest preceding storyboard (by sequence_number) that
// has a non-empty image_url. Used as the visual continuity anchor when the immediately
// preceding storyboard is a serial non-first clip (which has no image of its own).
func (r *StoryboardRepo) FindPrecedingWithImage(projectID uint64, sequenceNumber int, episodeID *uint64) *model.Storyboard {
	base := r.db.Model(&model.Storyboard{}).
		Where("project_id = ? AND is_voided = false AND sequence_number < ? AND image_url != ''", projectID, sequenceNumber)
	if episodeID != nil {
		base = base.Where("episode_id = ?", *episodeID)
	}
	var sb model.Storyboard
	if err := base.Order("sequence_number DESC").Limit(1).First(&sb).Error; err == nil {
		return &sb
	}
	return nil
}

func (r *StoryboardRepo) DeleteByProjectID(projectID uint64) error {
	// Delete versions for storyboards belonging to this project
	var sbIDs []uint64
	r.db.Model(&model.Storyboard{}).Where("project_id = ?", projectID).Pluck("id", &sbIDs)
	if len(sbIDs) > 0 {
		r.db.Where("storyboard_id IN ?", sbIDs).Delete(&model.StoryboardVersion{})
	}
	return r.db.Where("project_id = ?", projectID).Delete(&model.Storyboard{}).Error
}

func (r *StoryboardRepo) DeleteByEpisodeID(projectID, episodeID uint64) error {
	var sbIDs []uint64
	r.db.Model(&model.Storyboard{}).Where("project_id = ? AND episode_id = ?", projectID, episodeID).Pluck("id", &sbIDs)
	if len(sbIDs) > 0 {
		r.db.Where("storyboard_id IN ?", sbIDs).Delete(&model.StoryboardVersion{})
	}
	return r.db.Where("project_id = ? AND episode_id = ?", projectID, episodeID).Delete(&model.Storyboard{}).Error
}

func (r *StoryboardRepo) MaxSequenceNumber(projectID uint64) (int, error) {
	var maxSeq int
	if err := r.db.Model(&model.Storyboard{}).Where("project_id = ?", projectID).Select("COALESCE(MAX(sequence_number), 0)").Scan(&maxSeq).Error; err != nil {
		return 0, err
	}
	return maxSeq, nil
}

func (r *StoryboardRepo) ReindexSequenceNumbers(projectID uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var storyboards []model.Storyboard
		if err := tx.
			Model(&model.Storyboard{}).
			Joins("LEFT JOIN episodes ON episodes.id = storyboards.episode_id").
			Where("storyboards.project_id = ?", projectID).
			Order("COALESCE(episodes.episode_number, 2147483647) ASC").
			Order("storyboards.sequence_number ASC").
			Order("storyboards.id ASC").
			Find(&storyboards).Error; err != nil {
			return err
		}
		for i := range storyboards {
			newSeq := i + 1
			if storyboards[i].SequenceNumber == newSeq {
				continue
			}
			if err := tx.Model(&model.Storyboard{}).
				Where("id = ?", storyboards[i].ID).
				Update("sequence_number", newSeq).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteByID —— 永久删除单条分镜及其所有版本
// DeleteByID hard-deletes a storyboard and all its versions.
func (r *StoryboardRepo) DeleteByID(id uint64) error {
	if err := r.db.Where("storyboard_id = ?", id).Delete(&model.StoryboardVersion{}).Error; err != nil {
		return err
	}
	return r.db.Delete(&model.Storyboard{}, id).Error
}

// EpisodeCompletedCounts returns the count of completed storyboards per episode for a project.
func (r *StoryboardRepo) EpisodeCompletedCounts(projectID uint64) (map[uint64]int64, error) {
	type row struct {
		EpisodeID uint64
		Cnt       int64
	}
	var rows []row
	err := r.db.Model(&model.Storyboard{}).
		Select("episode_id, count(*) as cnt").
		Where("project_id = ? AND status = 'completed' AND image_url IS NOT NULL AND image_url != '' AND episode_id IS NOT NULL", projectID).
		Group("episode_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	m := make(map[uint64]int64, len(rows))
	for _, row := range rows {
		m[row.EpisodeID] = row.Cnt
	}
	return m, nil
}

// GetCompletedWithImages returns all non-voided, completed storyboards that have an image URL.
// Results are ordered by scene_order so the export ZIP reflects the script sequence.
func (r *StoryboardRepo) GetCompletedWithImages(projectID uint64) ([]model.Storyboard, error) {
	var sbs []model.Storyboard
	err := r.db.
		Where("project_id = ? AND status = 'completed' AND image_url IS NOT NULL AND image_url != '' AND is_voided = false", projectID).
		Order("sequence_number ASC, id ASC").
		Find(&sbs).Error
	return sbs, err
}

// FindAllActive returns all non-voided storyboards for a project/episode in
// sequence order. Used by the scene continuity auditor.
func (r *StoryboardRepo) FindAllActive(projectID uint64, episodeID *uint64) ([]model.Storyboard, error) {
	var sbs []model.Storyboard
	query := r.db.Where("project_id = ? AND is_voided = false", projectID)
	if episodeID != nil {
		query = query.Where("episode_id = ?", *episodeID)
	}
	err := query.
		Order("sequence_number ASC, id ASC").
		Find(&sbs).Error
	return sbs, err
}
