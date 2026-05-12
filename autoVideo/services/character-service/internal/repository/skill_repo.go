package repository

import (
	"sync"

	"github.com/autovideo/character-service/internal/model"
	"gorm.io/gorm"
)

type SkillRepository interface {
	Create(skill *model.Skill) error
	ListByProject(projectID int64, skillType, useCase string) ([]*model.Skill, error)
	ListActiveByUseCase(projectID int64, useCase string) ([]*model.Skill, error)
	ListByCharacter(characterID int64) ([]*model.Skill, error)
	GetByID(id int64) (*model.Skill, error)
	Update(skill *model.Skill) error
	Delete(id int64) error
	DeleteAllByProject(projectID int64) error
}

type skillRepository struct {
	db                 *gorm.DB
	hasSortOrderOnce   sync.Once
	hasSortOrderColumn bool
}

func NewSkillRepository(db *gorm.DB) SkillRepository {
	return &skillRepository{db: db}
}

func (r *skillRepository) orderActiveSkills(query *gorm.DB) *gorm.DB {
	r.hasSortOrderOnce.Do(func() {
		r.hasSortOrderColumn = r.db.Migrator().HasColumn(&model.Skill{}, "sort_order")
	})
	if r.hasSortOrderColumn {
		return query.Order("sort_order ASC, created_at ASC")
	}
	return query.Order("created_at ASC")
}

func (r *skillRepository) Create(skill *model.Skill) error {
	return r.db.Create(skill).Error
}

func (r *skillRepository) ListByProject(projectID int64, skillType, useCase string) ([]*model.Skill, error) {
	q := r.db.Where("project_id = ?", projectID)
	if skillType != "" {
		q = q.Where("skill_type = ?", skillType)
	}
	if useCase != "" {
		q = q.Where("use_case = ?", useCase)
	}
	var list []*model.Skill
	if err := q.Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ListActiveByUseCase 返回指定项目中 is_active=true 且匹配 use_case 的技能列表，供 AI 管线注入使用
func (r *skillRepository) ListActiveByUseCase(projectID int64, useCase string) ([]*model.Skill, error) {
	q := r.db.Where("project_id = ? AND is_active = true", projectID)
	if useCase != "" {
		q = q.Where("use_case = ?", useCase)
	}
	var list []*model.Skill
	if err := r.orderActiveSkills(q).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (r *skillRepository) ListByCharacter(characterID int64) ([]*model.Skill, error) {
	var list []*model.Skill
	if err := r.db.Where("character_id = ?", characterID).Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (r *skillRepository) GetByID(id int64) (*model.Skill, error) {
	var skill model.Skill
	if err := r.db.First(&skill, id).Error; err != nil {
		return nil, err
	}
	return &skill, nil
}

func (r *skillRepository) Update(skill *model.Skill) error {
	return r.db.Save(skill).Error
}

func (r *skillRepository) Delete(id int64) error {
	return r.db.Delete(&model.Skill{}, id).Error
}

// DeleteAllByProject —— 删除项目下所有技能记录
func (r *skillRepository) DeleteAllByProject(projectID int64) error {
	return r.db.Where("project_id = ?", projectID).Delete(&model.Skill{}).Error
}
