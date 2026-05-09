package repository

import (
	"github.com/autovideo/character-service/internal/model"
	"gorm.io/gorm"
)

// CharacterGroupRepo handles DB operations for CharacterGroup.
type CharacterGroupRepo struct {
	db *gorm.DB
}

func NewCharacterGroupRepo(db *gorm.DB) *CharacterGroupRepo {
	return &CharacterGroupRepo{db: db}
}

func (r *CharacterGroupRepo) Create(g *model.CharacterGroup) error {
	return r.db.Create(g).Error
}

func (r *CharacterGroupRepo) FindByProjectID(projectID int64) ([]model.CharacterGroup, error) {
	var groups []model.CharacterGroup
	err := r.db.
		Preload("Variants").
		Where("project_id = ? AND deleted_at IS NULL", projectID).
		Order("sort_order ASC, id ASC").
		Find(&groups).Error
	return groups, err
}

func (r *CharacterGroupRepo) FindByID(id int64) (*model.CharacterGroup, error) {
	var g model.CharacterGroup
	err := r.db.Preload("Variants").First(&g, "id = ? AND deleted_at IS NULL", id).Error
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *CharacterGroupRepo) Update(g *model.CharacterGroup) error {
	return r.db.Save(g).Error
}

func (r *CharacterGroupRepo) Delete(id, projectID int64) error {
	return r.db.
		Where("id = ? AND project_id = ?", id, projectID).
		Delete(&model.CharacterGroup{}).Error
}

// AssignAssetToGroup sets group_id and variant_name on an asset.
// Returns (rowsAffected, error).
func (r *CharacterGroupRepo) AssignAssetToGroup(assetID uint64, groupID int64, variantName string, sortOrder int) (int64, error) {
	tx := r.db.Model(&model.Asset{}).
		Where("id = ?", assetID).
		Updates(map[string]interface{}{
			"group_id":         groupID,
			"variant_name":     variantName,
			"asset_sort_order": sortOrder,
		})
	return tx.RowsAffected, tx.Error
}

// RemoveAssetFromGroup clears the group association for an asset.
func (r *CharacterGroupRepo) RemoveAssetFromGroup(assetID uint64) error {
	return r.db.Model(&model.Asset{}).
		Where("id = ?", assetID).
		Updates(map[string]interface{}{
			"group_id":         nil,
			"variant_name":     "",
			"asset_sort_order": 0,
		}).Error
}
