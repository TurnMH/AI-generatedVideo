package repository

import (
	"github.com/autovideo/character-service/internal/model"
	"gorm.io/gorm"
)

type CharacterRepo struct {
	db *gorm.DB
}

// NewCharacterRepo —— 创建角色仓库实例，返回 *CharacterRepo
func NewCharacterRepo(db *gorm.DB) *CharacterRepo {
	return &CharacterRepo{db: db}
}

// ListByProject —— 按项目 ID 查询该项目下的所有角色
func (r *CharacterRepo) ListByProject(projectID int64) ([]*model.Character, error) {
	var list []*model.Character
	err := r.db.Where("project_id = ?", projectID).Order("id desc").Find(&list).Error
	return list, err
}

// Create —— 在数据库中新增一条角色记录
func (r *CharacterRepo) Create(c *model.Character) error {
	return r.db.Create(c).Error
}

// GetByID —— 根据主键 ID 查询单个角色，返回 *model.Character
func (r *CharacterRepo) GetByID(id int64) (*model.Character, error) {
	var c model.Character
	err := r.db.First(&c, id).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Update —— 全量更新一条角色记录
func (r *CharacterRepo) Update(c *model.Character) error {
	return r.db.Save(c).Error
}

// Delete —— 根据 ID 软删除一条角色记录
func (r *CharacterRepo) Delete(id int64) error {
	return r.db.Delete(&model.Character{}, id).Error
}

// DeleteByProjectID —— 删除项目下所有角色记录
func (r *CharacterRepo) DeleteByProjectID(projectID int64) error {
	return r.db.Where("project_id = ?", projectID).Delete(&model.Character{}).Error
}

// UpdateReferenceImage —— 更新角色的参考图片 URL
func (r *CharacterRepo) UpdateReferenceImage(id int64, url string) error {
	return r.db.Model(&model.Character{}).Where("id = ?", id).
		Update("reference_image_url", url).Error
}

// UpdateStyle —— 更新角色的风格预设和风格参考图 URL
func (r *CharacterRepo) UpdateStyle(id int64, preset, styleRefURL string) error {
	return r.db.Model(&model.Character{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"style_preset":        preset,
			"style_reference_url": styleRefURL,
		}).Error
}
