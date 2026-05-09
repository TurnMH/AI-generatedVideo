package repository

import (
	"github.com/autovideo/character-service/internal/model"
	"gorm.io/gorm"
)

type ProductionSkillRepository interface {
	Create(skill *model.ProductionSkill) error
	ListByProject(projectID int64) ([]*model.ProductionSkill, error)
	ListActiveByProject(projectID int64) ([]*model.ProductionSkill, error)
	GetByID(id int64) (*model.ProductionSkill, error)
	Update(skill *model.ProductionSkill) error
	Delete(id int64) error
	DeleteAllByProject(projectID int64) error
}

type productionSkillRepo struct {
	db *gorm.DB
}

func NewProductionSkillRepository(db *gorm.DB) ProductionSkillRepository {
	return &productionSkillRepo{db: db}
}

func (r *productionSkillRepo) Create(skill *model.ProductionSkill) error {
	return r.db.Create(skill).Error
}

func (r *productionSkillRepo) ListByProject(projectID int64) ([]*model.ProductionSkill, error) {
	var list []*model.ProductionSkill
	if err := r.db.Where("project_id = ?", projectID).
		Order("sort_order ASC, id ASC").
		Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (r *productionSkillRepo) ListActiveByProject(projectID int64) ([]*model.ProductionSkill, error) {
	var list []*model.ProductionSkill
	if err := r.db.Where("project_id = ? AND is_active = true", projectID).
		Order("sort_order ASC, id ASC").
		Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (r *productionSkillRepo) GetByID(id int64) (*model.ProductionSkill, error) {
	var skill model.ProductionSkill
	if err := r.db.First(&skill, id).Error; err != nil {
		return nil, err
	}
	return &skill, nil
}

func (r *productionSkillRepo) Update(skill *model.ProductionSkill) error {
	return r.db.Save(skill).Error
}

func (r *productionSkillRepo) Delete(id int64) error {
	return r.db.Delete(&model.ProductionSkill{}, id).Error
}

func (r *productionSkillRepo) DeleteAllByProject(projectID int64) error {
	return r.db.Where("project_id = ?", projectID).Delete(&model.ProductionSkill{}).Error
}
