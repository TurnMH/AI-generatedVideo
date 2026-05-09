package repository

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/autovideo/project-service/internal/model"
)

type EpisodeRepo struct {
	db *gorm.DB
}

// NewEpisodeRepo —— 创建剧集仓储实例
func NewEpisodeRepo(db *gorm.DB) *EpisodeRepo {
	return &EpisodeRepo{db: db}
}

// Create —— 新增一条剧集记录到数据库
func (r *EpisodeRepo) Create(e *model.Episode) error {
	return r.db.Create(e).Error
}

// FindByProjectID —— 按项目 ID 查询所有剧集，按集号升序返回
func (r *EpisodeRepo) FindByProjectID(projectID uint64) ([]model.Episode, error) {
	var episodes []model.Episode
	err := r.db.Where("project_id = ?", projectID).
		Order("episode_number ASC").
		Find(&episodes).Error
	return episodes, err
}

// FindByID —— 按主键查询单条剧集记录
func (r *EpisodeRepo) FindByID(id uint64) (*model.Episode, error) {
	var episode model.Episode
	err := r.db.First(&episode, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &episode, nil
}

// Update —— 保存剧集记录的全部字段更新
func (r *EpisodeRepo) Update(e *model.Episode) error {
	return r.db.Select(
		"title", "summary", "script_excerpt", "word_count", "estimated_duration",
		"status", "version", "keywords", "updated_at",
		// optimize / review fields (must be included or optimize/review results are lost)
		"optimize_status", "optimized_text", "original_excerpt",
		"review_status", "review_result",
	).Save(e).Error
}

// DeleteByProjectID —— 删除指定项目下的所有剧集
// DeleteByProjectID removes all episodes for a project.
func (r *EpisodeRepo) DeleteByProjectID(projectID uint64) error {
	return r.db.Where("project_id = ?", projectID).Delete(&model.Episode{}).Error
}

// ReplaceAllForProject —— 在事务中原子性地替换项目的全部剧集
// ReplaceAllForProject atomically deletes old episodes and inserts new ones in a single transaction.
// This prevents partial states if the process is interrupted or called concurrently.
func (r *EpisodeRepo) ReplaceAllForProject(projectID uint64, episodes []model.Episode) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("project_id = ?", projectID).Delete(&model.Episode{}).Error; err != nil {
			return fmt.Errorf("delete old episodes: %w", err)
		}
		if len(episodes) == 0 {
			return nil
		}
		if err := tx.Create(&episodes).Error; err != nil {
			return fmt.Errorf("batch create episodes: %w", err)
		}
		return nil
	})
}

// BatchCreate —— 批量插入多条剧集记录
// BatchCreate inserts multiple episodes at once.
func (r *EpisodeRepo) BatchCreate(episodes []model.Episode) error {
	if len(episodes) == 0 {
		return nil
	}
	return r.db.Create(&episodes).Error
}

// Delete —— 按 ID 和项目 ID 删除单条剧集记录
// Delete removes an episode by ID with project ownership verification.
func (r *EpisodeRepo) Delete(id, projectID uint64) error {
	result := r.db.Where("id = ? AND project_id = ?", id, projectID).Delete(&model.Episode{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateStatus —— 更新指定剧集的状态字段
// UpdateStatus updates the status of an episode by ID.
func (r *EpisodeRepo) UpdateStatus(id uint64, status string) error {
	return r.db.Model(&model.Episode{}).Where("id = ?", id).Update("status", status).Error
}
