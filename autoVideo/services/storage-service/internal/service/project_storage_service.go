package service

import (
	"context"
	"fmt"

	"github.com/autovideo/storage-service/internal/driver"
	"github.com/autovideo/storage-service/internal/model"
	"github.com/autovideo/storage-service/internal/repository"
)

// CategorySummary holds aggregated data for a file category.
type CategorySummary struct {
	Bytes int64 `json:"bytes"`
	Count int   `json:"count"`
}

// StorageDetails is the response for project storage details.
type StorageDetails struct {
	TotalBytes int64                       `json:"total_bytes"`
	Categories map[string]*CategorySummary `json:"categories"`
	Files      []model.File                `json:"files"`
}

// ProjectStorageService handles project-scoped storage operations.
type ProjectStorageService struct {
	repo   *repository.FileRepo
	driver driver.StorageDriver
}

// NewProjectStorageService —— 创建项目存储服务实例
// NewProjectStorageService creates a ProjectStorageService.
func NewProjectStorageService(repo *repository.FileRepo, storageDriver driver.StorageDriver) *ProjectStorageService {
	return &ProjectStorageService{repo: repo, driver: storageDriver}
}

// GetStorageDetails —— 获取项目按分类汇总的存储详情，返回总字节数、各分类摘要及文件列表
// GetStorageDetails returns storage details grouped by category for a project.
func (s *ProjectStorageService) GetStorageDetails(ctx context.Context, projectID uint64, category string) (*StorageDetails, error) {
	files, err := s.repo.FindByProjectID(ctx, projectID, category)
	if err != nil {
		return nil, fmt.Errorf("find by project: %w", err)
	}

	categories := make(map[string]*CategorySummary)
	var totalBytes int64

	for _, f := range files {
		cat, ok := categories[f.Category]
		if !ok {
			cat = &CategorySummary{}
			categories[f.Category] = cat
		}
		cat.Bytes += f.SizeBytes
		cat.Count++
		totalBytes += f.SizeBytes
	}

	return &StorageDetails{
		TotalBytes: totalBytes,
		Categories: categories,
		Files:      files,
	}, nil
}

// GetBulkTotals —— 批量返回多项目的存储总字节数
// GetBulkTotals returns total bytes per project for the given project IDs.
func (s *ProjectStorageService) GetBulkTotals(ctx context.Context, projectIDs []uint64) (map[uint64]int64, error) {
	return s.repo.GetBulkTotals(ctx, projectIDs)
}

// DeleteFiles —— 按 ID 列表删除项目中的非当前版本文件，返回释放的字节数
// DeleteFiles deletes specified non-current files by IDs for a project.
func (s *ProjectStorageService) DeleteFiles(ctx context.Context, projectID uint64, fileIDs []uint64) (int64, error) {
	if len(fileIDs) == 0 {
		return 0, nil
	}
	deletedBytes, err := s.repo.DeleteByIDs(ctx, fileIDs, projectID)
	if err != nil {
		return 0, fmt.Errorf("delete files: %w", err)
	}
	return deletedBytes, nil
}

// CleanHistory —— 清理项目中所有非当前版本的历史文件，返回释放的字节数
// CleanHistory deletes all non-current files for a project.
func (s *ProjectStorageService) CleanHistory(ctx context.Context, projectID uint64) (int64, error) {
	deletedBytes, err := s.repo.DeleteAllHistory(ctx, projectID)
	if err != nil {
		return 0, fmt.Errorf("clean history: %w", err)
	}
	return deletedBytes, nil
}

// DeleteProjectFiles —— 删除项目下所有文件记录及其底层对象
func (s *ProjectStorageService) DeleteProjectFiles(ctx context.Context, projectID uint64) (int64, int, error) {
	files, err := s.repo.FindByProjectID(ctx, projectID, "")
	if err != nil {
		return 0, 0, fmt.Errorf("find project files: %w", err)
	}

	var totalBytes int64
	for _, file := range files {
		if err := s.driver.Delete(ctx, file.Bucket, file.ObjectKey); err != nil {
			return totalBytes, 0, fmt.Errorf("delete object %s: %w", file.ObjectKey, err)
		}
		if err := s.repo.DeleteByObjectKey(ctx, file.ObjectKey); err != nil {
			return totalBytes, 0, fmt.Errorf("delete file record %s: %w", file.ObjectKey, err)
		}
		totalBytes += file.SizeBytes
	}

	return totalBytes, len(files), nil
}
