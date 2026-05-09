package repository

import (
	"github.com/autovideo/script-service/internal/model"
	"gorm.io/gorm"
)

type PromptTemplateRepository interface {
	List(styleKey, resourceType, modelBinding string, activeOnly bool) ([]*model.PromptTemplate, error)
	GetByID(id int64) (*model.PromptTemplate, error)
	GetByStyleKey(styleKey string) (*model.PromptTemplate, error)
	Create(t *model.PromptTemplate) error
	Update(t *model.PromptTemplate) error
	Delete(id int64) error
}

type promptTemplateRepo struct {
	db *gorm.DB
}

func NewPromptTemplateRepository(db *gorm.DB) PromptTemplateRepository {
	return &promptTemplateRepo{db: db}
}

func (r *promptTemplateRepo) List(styleKey, resourceType, modelBinding string, activeOnly bool) ([]*model.PromptTemplate, error) {
	q := r.db.Model(&model.PromptTemplate{})
	if activeOnly {
		q = q.Where("is_active = true")
	}
	if styleKey != "" {
		q = q.Where("style_key = ?", styleKey)
	}
	if resourceType != "" {
		q = q.Where("resource_type = ?", resourceType)
	}
	if modelBinding != "" {
		q = q.Where("model_binding = ? OR model_binding = ''", modelBinding)
	}
	var list []*model.PromptTemplate
	if err := q.Order("sort_order ASC, created_at DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (r *promptTemplateRepo) GetByID(id int64) (*model.PromptTemplate, error) {
	var t model.PromptTemplate
	if err := r.db.First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *promptTemplateRepo) GetByStyleKey(styleKey string) (*model.PromptTemplate, error) {
	var t model.PromptTemplate
	if err := r.db.Where("style_key = ?", styleKey).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *promptTemplateRepo) Create(t *model.PromptTemplate) error {
	return r.db.Create(t).Error
}

func (r *promptTemplateRepo) Update(t *model.PromptTemplate) error {
	return r.db.Save(t).Error
}

func (r *promptTemplateRepo) Delete(id int64) error {
	return r.db.Delete(&model.PromptTemplate{}, id).Error
}
