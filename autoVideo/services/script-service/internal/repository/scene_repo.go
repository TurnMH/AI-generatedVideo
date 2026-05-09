package repository

import (
	"context"

	"github.com/autovideo/script-service/internal/model"
	"gorm.io/gorm"
)

type SceneRepository interface {
	BatchCreate(ctx context.Context, scenes []*model.Scene) error
	ListByScriptID(ctx context.Context, scriptID int64) ([]*model.Scene, error)
	GetByID(ctx context.Context, id int64) (*model.Scene, error)
	Update(ctx context.Context, scene *model.Scene) error
	DeleteByScriptID(ctx context.Context, scriptID int64) error
}

type CharacterRepository interface {
	BatchCreate(ctx context.Context, chars []*model.CharacterExtracted) error
	ListByScriptID(ctx context.Context, scriptID int64) ([]*model.CharacterExtracted, error)
	DeleteByScriptID(ctx context.Context, scriptID int64) error
}

type AssetRepository interface {
	BatchCreate(ctx context.Context, assets []*model.ScriptAsset) error
	ListByScriptID(ctx context.Context, scriptID int64) ([]*model.ScriptAsset, error)
	DeleteByScriptID(ctx context.Context, scriptID int64) error
}

// --- Scene ---

type sceneRepository struct {
	db *gorm.DB
}

// NewSceneRepository —— 创建场景仓储实例，返回 SceneRepository 接口
func NewSceneRepository(db *gorm.DB) SceneRepository {
	return &sceneRepository{db: db}
}

// BatchCreate —— 批量插入场景记录到数据库
func (r *sceneRepository) BatchCreate(ctx context.Context, scenes []*model.Scene) error {
	if len(scenes) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&scenes).Error
}

// ListByScriptID —— 按场景顺序查询指定剧本的所有场景
func (r *sceneRepository) ListByScriptID(ctx context.Context, scriptID int64) ([]*model.Scene, error) {
	var scenes []*model.Scene
	if err := r.db.WithContext(ctx).
		Where("script_id = ?", scriptID).
		Order("scene_order ASC").
		Find(&scenes).Error; err != nil {
		return nil, err
	}
	return scenes, nil
}

// GetByID —— 根据 ID 查询单个场景记录
func (r *sceneRepository) GetByID(ctx context.Context, id int64) (*model.Scene, error) {
	var scene model.Scene
	if err := r.db.WithContext(ctx).First(&scene, id).Error; err != nil {
		return nil, err
	}
	return &scene, nil
}

// Update —— 保存更新后的场景记录到数据库
func (r *sceneRepository) Update(ctx context.Context, scene *model.Scene) error {
	return r.db.WithContext(ctx).Save(scene).Error
}

// DeleteByScriptID —— 删除指定剧本下的所有场景记录
func (r *sceneRepository) DeleteByScriptID(ctx context.Context, scriptID int64) error {
	return r.db.WithContext(ctx).Where("script_id = ?", scriptID).Delete(&model.Scene{}).Error
}

// --- Character ---

type characterRepository struct {
	db *gorm.DB
}

// NewCharacterRepository —— 创建角色仓储实例，返回 CharacterRepository 接口
func NewCharacterRepository(db *gorm.DB) CharacterRepository {
	return &characterRepository{db: db}
}

// BatchCreate —— 批量插入角色记录到数据库
func (r *characterRepository) BatchCreate(ctx context.Context, chars []*model.CharacterExtracted) error {
	if len(chars) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&chars).Error
}

// ListByScriptID —— 查询指定剧本的所有提取角色
func (r *characterRepository) ListByScriptID(ctx context.Context, scriptID int64) ([]*model.CharacterExtracted, error) {
	var chars []*model.CharacterExtracted
	if err := r.db.WithContext(ctx).
		Where("script_id = ?", scriptID).
		Find(&chars).Error; err != nil {
		return nil, err
	}
	return chars, nil
}

// DeleteByScriptID —— 删除指定剧本下的所有角色记录
func (r *characterRepository) DeleteByScriptID(ctx context.Context, scriptID int64) error {
	return r.db.WithContext(ctx).Where("script_id = ?", scriptID).Delete(&model.CharacterExtracted{}).Error
}

// --- Asset ---

type assetRepository struct {
	db *gorm.DB
}

// NewAssetRepository —— 创建资产仓储实例，返回 AssetRepository 接口
func NewAssetRepository(db *gorm.DB) AssetRepository {
	return &assetRepository{db: db}
}

// BatchCreate —— 批量插入资产记录到数据库
func (r *assetRepository) BatchCreate(ctx context.Context, assets []*model.ScriptAsset) error {
	if len(assets) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&assets).Error
}

// ListByScriptID —— 查询指定剧本的所有资产记录
func (r *assetRepository) ListByScriptID(ctx context.Context, scriptID int64) ([]*model.ScriptAsset, error) {
	var assets []*model.ScriptAsset
	if err := r.db.WithContext(ctx).
		Where("script_id = ?", scriptID).
		Find(&assets).Error; err != nil {
		return nil, err
	}
	return assets, nil
}

// DeleteByScriptID —— 删除指定剧本下的所有资产记录
func (r *assetRepository) DeleteByScriptID(ctx context.Context, scriptID int64) error {
	return r.db.WithContext(ctx).Where("script_id = ?", scriptID).Delete(&model.ScriptAsset{}).Error
}
