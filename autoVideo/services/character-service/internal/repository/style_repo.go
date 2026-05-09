package repository

import (
	"github.com/autovideo/character-service/internal/model"
	"gorm.io/gorm"
)

type StyleRepo struct {
	db *gorm.DB
}

// NewStyleRepo —— 创建风格仓库实例，返回 *StyleRepo
func NewStyleRepo(db *gorm.DB) *StyleRepo {
	return &StyleRepo{db: db}
}

// List —— 查询所有风格预设，按 ID 升序排列
func (r *StyleRepo) List() ([]*model.StylePreset, error) {
	var list []*model.StylePreset
	err := r.db.Order("id asc").Find(&list).Error
	return list, err
}

// GetByName —— 根据名称查询单个风格预设
func (r *StyleRepo) GetByName(name string) (*model.StylePreset, error) {
	var s model.StylePreset
	err := r.db.Where("name = ?", name).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}
