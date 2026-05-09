package repository

import (
	"context"
	"time"

	"github.com/autovideo/model-service/internal/model"
	"gorm.io/gorm"
)

// UsageRepo handles persistence for UsageRecord.
type UsageRepo struct {
	db *gorm.DB
}

// NewUsageRepo —— 创建 UsageRepo 实例
// NewUsageRepo constructs a UsageRepo.
func NewUsageRepo(db *gorm.DB) *UsageRepo {
	return &UsageRepo{db: db}
}

// Create —— 将一条用量记录写入数据库
// Create inserts a new usage record.
func (r *UsageRepo) Create(ctx context.Context, usage *model.UsageRecord) error {
	return r.db.WithContext(ctx).Create(usage).Error
}

// Query —— 按用户 ID 和时间范围查询用量记录列表
// Query returns usage records, optionally filtered by user and time range.
// A zero userID or zero time means "no filter on that dimension".
// Results are capped at 10 000 rows to prevent runaway memory usage.
func (r *UsageRepo) Query(ctx context.Context, userID uint64, start, end time.Time) ([]*model.UsageRecord, error) {
	var records []*model.UsageRecord
	q := r.db.WithContext(ctx)

	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	if !start.IsZero() {
		q = q.Where("created_at >= ?", start)
	}
	if !end.IsZero() {
		q = q.Where("created_at <= ?", end)
	}

	err := q.Order("created_at DESC").Limit(10000).Find(&records).Error
	return records, err
}

// SumCostByUser —— 汇总指定用户在给定时间范围内的总费用
// SumCostByUser returns total cost accumulated by a user within [start, end].
func (r *UsageRepo) SumCostByUser(ctx context.Context, userID uint64, start, end time.Time) (float64, error) {
	var total float64
	q := r.db.WithContext(ctx).Model(&model.UsageRecord{}).
		Select("COALESCE(SUM(cost), 0)").
		Where("user_id = ?", userID)
	if !start.IsZero() {
		q = q.Where("created_at >= ?", start)
	}
	if !end.IsZero() {
		q = q.Where("created_at <= ?", end)
	}
	err := q.Scan(&total).Error
	return total, err
}
