package service

import (
	"context"
	"testing"

	"nft-auction-backend/internal/repository"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type statsReaderFunc func(ctx context.Context, chainID int64, market string) (repository.PlatformStatsModel, error)

func (fn statsReaderFunc) Latest(ctx context.Context, chainID int64, market string) (repository.PlatformStatsModel, error) {
	return fn(ctx, chainID, market)
}

func TestStatsServiceReturnsZeroSnapshotWhenNoStatsExist(t *testing.T) {
	service := StatsService{
		ChainID: 11155111,
		Market:  "0xmarket",
		Repo: statsReaderFunc(func(context.Context, int64, string) (repository.PlatformStatsModel, error) {
			return repository.PlatformStatsModel{}, gorm.ErrRecordNotFound
		}),
	}

	row, err := service.Latest(context.Background())

	require.NoError(t, err)
	require.Equal(t, int64(11155111), row.ChainID)
	require.Equal(t, "0xmarket", row.MarketAddress)
	require.Equal(t, uint64(0), row.ActiveAuctionCount)
	require.Equal(t, uint64(0), row.EndedAuctionCount)
	require.Equal(t, uint64(0), row.TotalBidCount)
	require.Equal(t, "0", row.TotalVolumeUSD)
	require.False(t, row.SnapshotTime.IsZero())
}
