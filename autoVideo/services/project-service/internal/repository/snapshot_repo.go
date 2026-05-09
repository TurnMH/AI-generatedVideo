package repository

import (
	"gorm.io/gorm"

	"github.com/autovideo/project-service/internal/model"
)

type SnapshotRepo struct {
	db *gorm.DB
}

// NewSnapshotRepo —— 创建快照仓储实例
func NewSnapshotRepo(db *gorm.DB) *SnapshotRepo {
	return &SnapshotRepo{db: db}
}

// Create —— 新增一条项目快照记录
func (r *SnapshotRepo) Create(s *model.ProjectSnapshot) error {
	return r.db.Create(s).Error
}

// ListByProjectID —— 按项目 ID 查询所有快照，按版本号倒序
func (r *SnapshotRepo) ListByProjectID(projectID uint64) ([]model.ProjectSnapshot, error) {
	var snapshots []model.ProjectSnapshot
	err := r.db.Where("project_id = ?", projectID).
		Order("version DESC").
		Find(&snapshots).Error
	return snapshots, err
}

// NextVersion —— 返回项目的下一个可用快照版本号
// NextVersion returns the next available snapshot version for a project.
func (r *SnapshotRepo) NextVersion(projectID uint64) int {
	var maxVersion int
	r.db.Model(&model.ProjectSnapshot{}).
		Where("project_id = ?", projectID).
		Select("COALESCE(MAX(version), 0)").
		Scan(&maxVersion)
	return maxVersion + 1
}

// DeleteByProjectID —— 删除项目下所有快照记录
func (r *SnapshotRepo) DeleteByProjectID(projectID uint64) error {
	return r.db.Where("project_id = ?", projectID).Delete(&model.ProjectSnapshot{}).Error
}
