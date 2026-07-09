package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StatsRepository 负责 platform_stats 表的读写。
//
// platform_stats 保存 worker 定时计算出的快照，避免 API 请求实时扫描大量拍卖记录。
type StatsRepository struct {
	db *gorm.DB
}

// NewStatsRepository 创建统计仓储。
func NewStatsRepository(db *gorm.DB) *StatsRepository {
	return &StatsRepository{db: db}
}

// SaveSnapshot 保存某个时间点的平台统计。
//
// 使用 chain_id + market_address + snapshot_time 幂等，worker 重试同一分钟快照时会更新旧记录。
func (r *StatsRepository) SaveSnapshot(ctx context.Context, stats PlatformStatsModel) error {
	stats.MarketAddress = normalize(stats.MarketAddress)
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "chain_id"},
			{Name: "market_address"},
			{Name: "snapshot_time"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"active_auction_count",
			"ended_auction_count",
			"cancelled_auction_count",
			"total_bid_count",
			"total_volume_usd",
			"updated_at",
		}),
	}).Create(&stats).Error
}

// Latest 查询最近一次统计快照。
func (r *StatsRepository) Latest(ctx context.Context, chainID int64, market string) (PlatformStatsModel, error) {
	var row PlatformStatsModel
	err := r.db.WithContext(ctx).
		Where("chain_id = ? AND market_address = ?", chainID, normalize(market)).
		Order("snapshot_time DESC").
		First(&row).Error
	return row, err
}
