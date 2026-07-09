package service

import (
	"context"
	"time"

	"nft-auction-backend/internal/repository"
)

// StatsReader 抽象统计快照读取能力，便于 service 测试或替换存储实现。
type StatsReader interface {
	Latest(ctx context.Context, chainID int64, market string) (repository.PlatformStatsModel, error)
}

// StatsService 查询平台统计快照。
type StatsService struct {
	ChainID int64
	Market  string
	Repo    StatsReader
}

// Latest 返回最新统计快照。
//
// 如果 worker 还没有写入统计，返回一个零值快照，避免首页首次启动时报 404。
func (s StatsService) Latest(ctx context.Context) (repository.PlatformStatsModel, error) {
	row, err := s.Repo.Latest(ctx, s.ChainID, s.Market)
	if err == nil {
		return row, nil
	}
	if IsNotFound(err) {
		return repository.PlatformStatsModel{
			ChainID:        s.ChainID,
			MarketAddress:  s.Market,
			TotalVolumeUSD: "0",
			SnapshotTime:   time.Now().UTC(),
		}, nil
	}
	return repository.PlatformStatsModel{}, err
}
