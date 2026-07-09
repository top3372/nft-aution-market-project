package worker

import (
	"context"
	"time"

	"nft-auction-backend/internal/repository"
)

// StatsSource 抽象拍卖查询能力。
//
// StatsWorker 只需要按状态统计数量，不关心底层是否来自 MySQL、缓存或测试 fake。
type StatsSource interface {
	List(ctx context.Context, filter repository.AuctionListFilter) ([]repository.AuctionModel, int64, error)
}

// StatsSink 抽象统计快照保存能力。
type StatsSink interface {
	SaveSnapshot(ctx context.Context, stats repository.PlatformStatsModel) error
}

// StatsWorker 聚合平台统计数据，供首页快速查询。
type StatsWorker struct {
	ChainID int64
	Market  string
	Source  StatsSource
	Sink    StatsSink
}

// RunOnce 计算并保存一次平台统计快照。
//
// 当前快照包含 active/ended/cancelled 数量。TotalVolumeUSD 暂时保留为 0，
// 后续可以从 ended 拍卖或 bids 表聚合成交额。
func (w StatsWorker) RunOnce(ctx context.Context) error {
	_, active, err := w.Source.List(ctx, repository.AuctionListFilter{
		ChainID:       w.ChainID,
		MarketAddress: w.Market,
		Status:        "active",
		Page:          1,
		PageSize:      1,
	})
	if err != nil {
		return err
	}
	_, ended, err := w.Source.List(ctx, repository.AuctionListFilter{
		ChainID:       w.ChainID,
		MarketAddress: w.Market,
		Status:        "ended",
		Page:          1,
		PageSize:      1,
	})
	if err != nil {
		return err
	}
	_, cancelled, err := w.Source.List(ctx, repository.AuctionListFilter{
		ChainID:       w.ChainID,
		MarketAddress: w.Market,
		Status:        "cancelled",
		Page:          1,
		PageSize:      1,
	})
	if err != nil {
		return err
	}

	return w.Sink.SaveSnapshot(ctx, repository.PlatformStatsModel{
		ChainID:               w.ChainID,
		MarketAddress:         w.Market,
		ActiveAuctionCount:    uint64(active),
		EndedAuctionCount:     uint64(ended),
		CancelledAuctionCount: uint64(cancelled),
		TotalVolumeUSD:        "0",
		SnapshotTime:          time.Now().UTC(),
	})
}
