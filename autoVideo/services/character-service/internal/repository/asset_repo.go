package repository

import (
	"time"

	"github.com/autovideo/character-service/internal/model"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

type AssetRepo struct {
	db *gorm.DB
}

// NewAssetRepo —— 创建资产仓库实例，返回 *AssetRepo
func NewAssetRepo(db *gorm.DB) *AssetRepo {
	return &AssetRepo{db: db}
}

// Create —— 在数据库中新增一条资产记录
func (r *AssetRepo) Create(asset *model.Asset) error {
	return r.db.Create(asset).Error
}

// FindByID —— 根据主键 ID 查询单条资产，返回 *model.Asset
func (r *AssetRepo) FindByID(id uint64) (*model.Asset, error) {
	var asset model.Asset
	err := r.db.First(&asset, id).Error
	if err != nil {
		return nil, err
	}
	return &asset, nil
}

func applyEpisodeAssetFilter(query *gorm.DB, episodeID *uint64) *gorm.DB {
	if episodeID == nil {
		return query
	}
	return query.Where("episode_ids @> ARRAY[?]::integer[]", int64(*episodeID))
}

func applyAssetKeywordFilter(query *gorm.DB, keyword string) *gorm.DB {
	keyword = "%" + keyword + "%"
	return query.Where("(name ILIKE ? OR description ILIKE ?)", keyword, keyword)
}

// FindByProjectID —— 按项目 ID 和可选类型查询资产列表
func (r *AssetRepo) FindByProjectID(projectID uint64, assetType, keyword string, episodeID *uint64) ([]model.Asset, error) {
	var assets []model.Asset
	query := r.db.Where("project_id = ?", projectID)
	if assetType != "" {
		query = query.Where("type = ?", assetType)
	}
	if keyword != "" {
		query = applyAssetKeywordFilter(query, keyword)
	}
	query = applyEpisodeAssetFilter(query, episodeID)
	err := query.Order("id desc").Find(&assets).Error
	return assets, err
}

// FindRecoverablePending returns pending assets that still carry an image task ID in metadata.
func (r *AssetRepo) FindRecoverablePending(limit int) ([]model.Asset, error) {
	var assets []model.Asset
	query := r.db.
		Where("status = ?", "pending").
		Where("name <> ?", "__extracting__").
		Where("metadata -> 'generation_progress' ->> 'task_id' IS NOT NULL").
		Order("updated_at asc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&assets).Error
	return assets, err
}

// FindActiveExtractionSentinels returns in-flight extraction sentinels that can be resumed.
func (r *AssetRepo) FindActiveExtractionSentinels(limit int) ([]model.Asset, error) {
	var assets []model.Asset
	query := r.db.
		Where("name = ? AND status = ?", "__extracting__", "extracting").
		Order("updated_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&assets).Error
	return assets, err
}

// PaginatedResult holds a page of assets plus total count.
type PaginatedResult struct {
	Items []model.Asset `json:"items"`
	Total int64         `json:"total"`
	Page  int           `json:"page"`
	Size  int           `json:"page_size"`
}

// FindByProjectIDPaginated —— 按项目 ID 分页查询资产，返回分页结果和总数
// FindByProjectIDPaginated returns a page of assets with total count.
func (r *AssetRepo) FindByProjectIDPaginated(projectID uint64, assetType, status, keyword string, episodeID *uint64, page, pageSize int) (*PaginatedResult, error) {
	query := r.db.Model(&model.Asset{}).Where("project_id = ?", projectID)
	if assetType != "" {
		query = query.Where("type = ?", assetType)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if keyword != "" {
		query = applyAssetKeywordFilter(query, keyword)
	}
	query = applyEpisodeAssetFilter(query, episodeID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var assets []model.Asset
	err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&assets).Error
	if err != nil {
		return nil, err
	}

	return &PaginatedResult{
		Items: assets,
		Total: total,
		Page:  page,
		Size:  pageSize,
	}, nil
}

func (r *AssetRepo) FindByProjectTypeAndName(projectID uint64, assetType, name string) (*model.Asset, error) {
	var asset model.Asset
	err := r.db.
		Where("project_id = ? AND type = ? AND LOWER(name) = LOWER(?)", projectID, assetType, name).
		Order("id desc").
		First(&asset).Error
	if err != nil {
		return nil, err
	}
	return &asset, nil
}

// Update —— 全量更新一条资产记录
func (r *AssetRepo) Update(asset *model.Asset) error {
	return r.db.Save(asset).Error
}

// UpdateStatusOnly —— 仅更新资产的状态和错误信息字段
// UpdateStatusOnly updates only status + error_msg columns (avoids full-row write).
func (r *AssetRepo) UpdateStatusOnly(id uint64, status string, errMsg string) error {
	return r.db.Model(&model.Asset{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     status,
			"error_msg":  errMsg,
			"updated_at": "now()",
		}).Error
}

// UpdateGenerated —— 更新图像生成结果（图片URL、提示词、状态）
// UpdateGenerated updates only image generation result columns.
func (r *AssetRepo) UpdateGenerated(id uint64, imageURL, prompt, status string) error {
	return r.db.Model(&model.Asset{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"image_url":   imageURL,
			"prompt_used": prompt,
			"status":      status,
			"error_msg":   "",
			"updated_at":  "now()",
		}).Error
}

// VoidAssetImage —— 清空已生成图片并将状态重置为 pending（强制重新生成前调用）
// Uses targeted Updates(map) so empty strings are always persisted.
func (r *AssetRepo) VoidAssetImage(id uint64) error {
	return r.db.Model(&model.Asset{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     "pending",
			"image_url":  "",
			"error_msg":  "",
			"updated_at": time.Now(),
		}).Error
}

// BatchUpdateStatus —— 批量更新多个资产的状态
// BatchUpdateStatus updates status for multiple asset IDs in one query.
func (r *AssetRepo) BatchUpdateStatus(ids []uint64, status string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Model(&model.Asset{}).Where("id IN ?", ids).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": "now()",
		}).Error
}

// Delete —— 根据 ID 删除一条资产记录
func (r *AssetRepo) Delete(id uint64) error {
	return r.db.Delete(&model.Asset{}, id).Error
}

// DeleteSentinel —— 删除项目的提取进度哨兵记录（__extracting__）
// Called before starting a new extraction so stale sentinels don't block the UI.
func (r *AssetRepo) DeleteSentinel(projectID uint64) {
	r.db.Where("project_id = ? AND name = ?", projectID, "__extracting__").Delete(&model.Asset{})
}

// DeleteByProjectID —— 删除项目下所有未锁定的资产
func (r *AssetRepo) DeleteByProjectID(projectID uint64) error {
	return r.db.Where("project_id = ? AND is_locked = false", projectID).Delete(&model.Asset{}).Error
}

// ForceDeleteByProjectID —— 强制删除项目下所有资产（包括锁定资产），用于项目删除时
func (r *AssetRepo) ForceDeleteByProjectID(projectID uint64) error {
	return r.db.Where("project_id = ?", projectID).Delete(&model.Asset{}).Error
}

// DeleteByCharacterID —— 删除与指定角色关联的所有资产，用于角色删除时级联清理
func (r *AssetRepo) DeleteByCharacterID(characterID int64) error {
	return r.db.Where("character_id = ?", characterID).Delete(&model.Asset{}).Error
}

func removeEpisodeID(ids []int64, episodeID int64) []int64 {
	filtered := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id != episodeID {
			filtered = append(filtered, id)
		}
	}
	return filtered
}

// ClearEpisodeAssets removes or detaches previously extracted, unlocked assets for one episode.
func (r *AssetRepo) ClearEpisodeAssets(projectID, episodeID uint64) error {
	var assets []model.Asset
	if err := r.db.
		Where("project_id = ? AND is_locked = false AND is_manual = false AND name <> ?", projectID, "__extracting__").
		Where("? = ANY(episode_ids)", int64(episodeID)).
		Find(&assets).Error; err != nil {
		return err
	}

	for i := range assets {
		remaining := removeEpisodeID(assets[i].EpisodeIDs, int64(episodeID))
		if len(remaining) == 0 {
			if err := r.db.Delete(&assets[i]).Error; err != nil {
				return err
			}
			continue
		}
		if err := r.db.Model(&model.Asset{}).
			Where("id = ?", assets[i].ID).
			Updates(map[string]interface{}{
				"episode_ids": pq.Int64Array(remaining),
				"updated_at":  time.Now(),
			}).Error; err != nil {
			return err
		}
	}

	return nil
}

// LockByProjectID —— 锁定项目下所有资产，防止被删除或修改
func (r *AssetRepo) LockByProjectID(projectID uint64) error {
	return r.db.Model(&model.Asset{}).
		Where("project_id = ?", projectID).
		Update("is_locked", true).Error
}

// UnlockByIDs —— 解锁指定 ID 列表的资产
func (r *AssetRepo) UnlockByIDs(ids []uint64) error {
	return r.db.Model(&model.Asset{}).
		Where("id IN ?", ids).
		Update("is_locked", false).Error
}

// CountByProject —— 统计项目下资产总数和已完成数量
func (r *AssetRepo) CountByProject(projectID uint64) (total int64, completed int64, err error) {
	err = r.db.Model(&model.Asset{}).Where("project_id = ?", projectID).Count(&total).Error
	if err != nil {
		return
	}
	err = r.db.Model(&model.Asset{}).Where("project_id = ? AND status = ?", projectID, "completed").Count(&completed).Error
	return
}

// ResetOrphanedGenerating —— 启动时将残留的 generating/extracting 状态重置为 pending
// ResetOrphanedGenerating resets all "generating" and "extracting" assets back to "pending".
// Call on startup to recover from unclean shutdown.
func (r *AssetRepo) ResetOrphanedGenerating() (int64, error) {
	// Delete sentinel assets
	r.db.Where("name = ? AND status = ?", "__extracting__", "extracting").Delete(&model.Asset{})
	// Reset generating → pending
	tx := r.db.Model(&model.Asset{}).
		Where("status IN ?", []string{"generating", "extracting"}).
		Update("status", "pending")
	return tx.RowsAffected, tx.Error
}

// ResetOrphanedGeneratingOnly resets only image generation tasks and leaves
// extraction sentinels for the extraction recovery flow.
func (r *AssetRepo) ResetOrphanedGeneratingOnly() (int64, error) {
	tx := r.db.Model(&model.Asset{}).
		Where("status = ?", "generating").
		Update("status", "pending")
	return tx.RowsAffected, tx.Error
}

// ResetStaleGenerating —— 将超时的 generating 状态资产重置为 failed
// ResetStaleGenerating resets assets stuck in "generating" longer than the threshold to "failed".
func (r *AssetRepo) ResetStaleGenerating(threshold time.Duration) (int64, error) {
	cutoff := time.Now().Add(-threshold)
	// Delete stale sentinel assets
	r.db.Where("name = ? AND status = ? AND updated_at < ?", "__extracting__", "extracting", cutoff).Delete(&model.Asset{})
	// Reset stale generating → failed
	tx := r.db.Model(&model.Asset{}).
		Where("status = ? AND updated_at < ?", "generating", cutoff).
		Updates(map[string]interface{}{
			"status":     "failed",
			"error_msg":  "generation timed out (stale cleanup)",
			"updated_at": "now()",
		})
	return tx.RowsAffected, tx.Error
}
