package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/autovideo/script-service/internal/model"
	"github.com/autovideo/script-service/internal/repository"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// VersionService handles script version management.
type VersionService interface {
	ListVersions(ctx context.Context, scriptID int64) ([]*model.Script, error)
	CreateVersion(ctx context.Context, scriptID int64, req *CreateVersionReq) (*model.Script, error)
	SwitchVersion(ctx context.Context, scriptID int64, versionID int64) (*model.Script, error)
}

// CreateVersionReq contains the fields for creating a new version.
type CreateVersionReq struct {
	Title    string `json:"title"`
	RawText  string `json:"raw_text"`
	FileURL  string `json:"file_url"`
	FileSize int    `json:"file_size"`
}

type versionService struct {
	scriptRepo repository.ScriptRepository
	logger     *zap.Logger
}

// NewVersionService —— 创建版本管理服务实例，返回 VersionService 接口
func NewVersionService(scriptRepo repository.ScriptRepository, logger *zap.Logger) VersionService {
	return &versionService{scriptRepo: scriptRepo, logger: logger}
}

// ListVersions —— 查询与指定剧本同项目同标题的所有版本列表
func (s *versionService) ListVersions(ctx context.Context, scriptID int64) ([]*model.Script, error) {
	script, err := s.scriptRepo.GetByID(ctx, scriptID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get script: %w", err)
	}

	versions, err := s.scriptRepo.ListVersionsByProjectAndTitle(ctx, script.ProjectID, script.Title)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	return versions, nil
}

// CreateVersion —— 基于原始剧本创建新版本，自动递增版本号，返回新版本
func (s *versionService) CreateVersion(ctx context.Context, scriptID int64, req *CreateVersionReq) (*model.Script, error) {
	original, err := s.scriptRepo.GetByID(ctx, scriptID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get script: %w", err)
	}

	maxVersion, err := s.scriptRepo.GetMaxVersion(ctx, original.ProjectID, original.Title)
	if err != nil {
		return nil, fmt.Errorf("get max version: %w", err)
	}

	title := req.Title
	if title == "" {
		title = original.Title
	}
	rawText := req.RawText
	if rawText == "" {
		rawText = original.RawText
	}

	newScript := &model.Script{
		ProjectID:   original.ProjectID,
		EpisodeID:   original.EpisodeID,
		Title:       title,
		RawText:     rawText,
		FileURL:     req.FileURL,
		FileSize:    req.FileSize,
		Version:     maxVersion + 1,
		ParseStatus: "pending",
	}

	if err := s.scriptRepo.Create(ctx, newScript); err != nil {
		return nil, fmt.Errorf("create version: %w", err)
	}

	s.logger.Info("script version created",
		zap.Int64("original_id", scriptID),
		zap.Int64("new_id", newScript.ID),
		zap.Int("version", newScript.Version),
	)
	return newScript, nil
}

// SwitchVersion —— 切换到指定版本，通过更新 updated_at 标记为当前使用版本
func (s *versionService) SwitchVersion(ctx context.Context, scriptID int64, versionID int64) (*model.Script, error) {
	// Verify the original script exists
	_, err := s.scriptRepo.GetByID(ctx, scriptID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get script: %w", err)
	}

	// Verify the target version exists
	target, err := s.scriptRepo.GetByID(ctx, versionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get target version: %w", err)
	}

	// Touch the record to mark it as the latest used version
	if err := s.scriptRepo.TouchUpdatedAt(ctx, versionID); err != nil {
		return nil, fmt.Errorf("switch version: %w", err)
	}

	s.logger.Info("script version switched",
		zap.Int64("script_id", scriptID),
		zap.Int64("version_id", versionID),
		zap.Int("version", target.Version),
	)
	return target, nil
}
