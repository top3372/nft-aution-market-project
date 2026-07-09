package api

import (
	"testing"
	"time"

	"nft-auction-backend/internal/repository"

	"github.com/stretchr/testify/require"
)

func TestCurrentAuctionStatusUsesStartAndEndTime(t *testing.T) {
	now := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
	start := now.Add(-time.Hour)

	status := currentAuctionStatus(repository.AuctionModel{
		Status:    "pending",
		StartTime: &start,
		EndTime:   now.Add(time.Hour),
	}, now)

	require.Equal(t, "active", status)
}

func TestCurrentAuctionStatusKeepsTerminalStatus(t *testing.T) {
	now := time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC)
	start := now.Add(-time.Hour)

	status := currentAuctionStatus(repository.AuctionModel{
		Status:    "cancelled",
		StartTime: &start,
		EndTime:   now.Add(time.Hour),
	}, now)

	require.Equal(t, "cancelled", status)
}
