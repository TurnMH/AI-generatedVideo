package repository

import (
	"context"
	"fmt"

	"github.com/autovideo/storage-service/internal/model"
	"gorm.io/gorm"
)

// FileRepo handles database operations for File records.
type FileRepo struct {
	db *gorm.DB
}

// NewFileRepo —— 创建文件数据仓库实例
// NewFileRepo creates a new FileRepo.
func NewFileRepo(db *gorm.DB) *FileRepo {
	return &FileRepo{db: db}
}

// Create —— 在数据库中插入一条文件记录
func (r *FileRepo) Create(ctx context.Context, f *model.File) error {
	if err := r.db.WithContext(ctx).Create(f).Error; err != nil {
		return fmt.Errorf("FileRepo.Create: %w", err)
	}
	return nil
}

// GetByObjectKey —— 根据 objectKey 查询并返回单条文件记录
func (r *FileRepo) GetByObjectKey(ctx context.Context, objectKey string) (*model.File, error) {
	var f model.File
	if err := r.db.WithContext(ctx).Where("object_key = ?", objectKey).First(&f).Error; err != nil {
		return nil, fmt.Errorf("FileRepo.GetByObjectKey: %w", err)
	}
	return &f, nil
}

// ListByUserID —— 按创建时间倒序返回指定用户的所有文件记录
func (r *FileRepo) ListByUserID(ctx context.Context, userID uint64) ([]model.File, error) {
	var files []model.File
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Limit(5000).Find(&files).Error; err != nil {
		return nil, fmt.Errorf("FileRepo.ListByUserID: %w", err)
	}
	return files, nil
}

// DeleteByObjectKey —— 根据 objectKey 删除数据库中对应的文件记录
func (r *FileRepo) DeleteByObjectKey(ctx context.Context, objectKey string) error {
	if err := r.db.WithContext(ctx).Where("object_key = ?", objectKey).Delete(&model.File{}).Error; err != nil {
		return fmt.Errorf("FileRepo.DeleteByObjectKey: %w", err)
	}
	return nil
}

// FindByProjectID —— 查询指定项目的文件列表，可按分类过滤
// FindByProjectID returns files for a project, optionally filtered by category.
func (r *FileRepo) FindByProjectID(ctx context.Context, projectID uint64, category string) ([]model.File, error) {
	var files []model.File
	q := r.db.WithContext(ctx).Where("project_id = ?", projectID)
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if err := q.Order("created_at DESC").Find(&files).Error; err != nil {
		return nil, fmt.Errorf("FileRepo.FindByProjectID: %w", err)
	}
	return files, nil
}

// GetStorageSummary —— 按分类汇总项目的存储用量，返回各分类的字节数和文件数
// GetStorageSummary returns aggregated storage data grouped by category.
func (r *FileRepo) GetStorageSummary(ctx context.Context, projectID uint64) (map[string]struct {
	Bytes int64
	Count int
}, error) {
	type result struct {
		Category string
		Total    int64
		Cnt      int
	}
	var results []result
	if err := r.db.WithContext(ctx).
		Model(&model.File{}).
		Select("category, SUM(size_bytes) as total, COUNT(*) as cnt").
		Where("project_id = ?", projectID).
		Group("category").
		Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("FileRepo.GetStorageSummary: %w", err)
	}

	summary := make(map[string]struct {
		Bytes int64
		Count int
	})
	for _, r := range results {
		summary[r.Category] = struct {
			Bytes int64
			Count int
		}{Bytes: r.Total, Count: r.Cnt}
	}
	return summary, nil
}

// GetBulkTotals —— 批量查询多个项目的存储总字节数，返回 project_id → total_bytes 映射
// GetBulkTotals returns total size_bytes per project for the given project IDs.
func (r *FileRepo) GetBulkTotals(ctx context.Context, projectIDs []uint64) (map[uint64]int64, error) {
	if len(projectIDs) == 0 {
		return map[uint64]int64{}, nil
	}
	type row struct {
		ProjectID uint64
		Total     int64
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Model(&model.File{}).
		Select("project_id, COALESCE(SUM(size_bytes), 0) as total").
		Where("project_id IN ?", projectIDs).
		Group("project_id").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("FileRepo.GetBulkTotals: %w", err)
	}
	result := make(map[uint64]int64, len(rows))
	for _, r := range rows {
		result[r.ProjectID] = r.Total
	}
	return result, nil
}

// DeleteByIDs deletes non-current files by IDs within a project. Returns total freed bytes.
func (r *FileRepo) DeleteByIDs(ctx context.Context, ids []uint64, projectID uint64) (int64, error) {
	var totalBytes int64
	if err := r.db.WithContext(ctx).
		Model(&model.File{}).
		Where("id IN ? AND project_id = ? AND is_current = false", ids, projectID).
		Select("COALESCE(SUM(size_bytes), 0)").
		Scan(&totalBytes).Error; err != nil {
		return 0, fmt.Errorf("FileRepo.DeleteByIDs sum: %w", err)
	}

	if err := r.db.WithContext(ctx).
		Where("id IN ? AND project_id = ? AND is_current = false", ids, projectID).
		Delete(&model.File{}).Error; err != nil {
		return 0, fmt.Errorf("FileRepo.DeleteByIDs: %w", err)
	}
	return totalBytes, nil
}

// DeleteAllHistory —— 删除项目中所有非当前版本的历史文件，返回释放的总字节数
// DeleteAllHistory deletes all non-current files for a project. Returns total freed bytes.
func (r *FileRepo) DeleteAllHistory(ctx context.Context, projectID uint64) (int64, error) {
	var totalBytes int64
	if err := r.db.WithContext(ctx).
		Model(&model.File{}).
		Where("project_id = ? AND is_current = false", projectID).
		Select("COALESCE(SUM(size_bytes), 0)").
		Scan(&totalBytes).Error; err != nil {
		return 0, fmt.Errorf("FileRepo.DeleteAllHistory sum: %w", err)
	}

	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND is_current = false", projectID).
		Delete(&model.File{}).Error; err != nil {
		return 0, fmt.Errorf("FileRepo.DeleteAllHistory: %w", err)
	}
	return totalBytes, nil
}
