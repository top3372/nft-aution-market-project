package indexer

import (
	"context"
	"testing"

	"nft-auction-backend/internal/config"
	"nft-auction-backend/internal/repository"

	"github.com/stretchr/testify/require"
)

func TestBuildContractSourcesUsesConfiguredContracts(t *testing.T) {
	sources, err := BuildContractSources(config.Config{
		MarketAddress:       "0xBA9af325234368184A61be6081cdFB7f02dc6405",
		AuctionNFTAddress:   "0x7913Ff1EaA12887ed80a3D35c81c0033FFafadCe",
		PaymentTokenAddress: "0xBE3c38a1015b4B4dacfd13C5346F1Ee907D8c283",
		StartBlock:          100,
		IndexerContracts: []config.IndexerContract{
			{Name: "market", EventGroup: "market", Address: "0xBA9af325234368184A61be6081cdFB7f02dc6405", ABI: "auction_market_v3", StartBlock: 101},
			{Name: "auction_nft", EventGroup: "auction_nft", Address: "0x7913Ff1EaA12887ed80a3D35c81c0033FFafadCe", ABI: "auction_nft", StartBlock: 102},
			{Name: "payment_token", EventGroup: "payment_token", Address: "0xBE3c38a1015b4B4dacfd13C5346F1Ee907D8c283", ABI: "erc20", StartBlock: 103},
		},
	}, SourceStores{
		Auctions: &recordingAuctionStore{},
		Bids:     &recordingBidStore{},
		NFTs:     &recordingNFTStore{},
	})

	require.NoError(t, err)
	require.Len(t, sources, 3)
	require.Equal(t, "market", sources[0].Name)
	require.Equal(t, uint64(101), sources[0].StartBlock)
	require.NotNil(t, sources[0].Decoder)
	require.NotNil(t, sources[0].Handler)
	require.Equal(t, "auction_nft", sources[1].Name)
	require.Equal(t, uint64(102), sources[1].StartBlock)
	require.NotNil(t, sources[1].Decoder)
	require.NotNil(t, sources[1].Handler)
	require.Equal(t, "payment_token", sources[2].Name)
	require.Equal(t, uint64(103), sources[2].StartBlock)
	require.NotNil(t, sources[2].Decoder)
	require.NotNil(t, sources[2].Handler)
}

func TestBuildContractSourcesFallsBackToMarketAndNFTWhenContractsOmitted(t *testing.T) {
	sources, err := BuildContractSources(config.Config{
		MarketAddress:     "0xBA9af325234368184A61be6081cdFB7f02dc6405",
		AuctionNFTAddress: "0x7913Ff1EaA12887ed80a3D35c81c0033FFafadCe",
		StartBlock:        100,
	}, SourceStores{
		Auctions: &recordingAuctionStore{},
		Bids:     &recordingBidStore{},
		NFTs:     &recordingNFTStore{},
	})

	require.NoError(t, err)
	require.Len(t, sources, 2)
	require.Equal(t, "market", sources[0].Name)
	require.Equal(t, "auction_nft", sources[1].Name)
	require.Equal(t, uint64(100), sources[0].StartBlock)
	require.Equal(t, uint64(100), sources[1].StartBlock)
}

func TestBuildContractSourcesRejectsUnknownABI(t *testing.T) {
	_, err := BuildContractSources(config.Config{
		MarketAddress:     "0xBA9af325234368184A61be6081cdFB7f02dc6405",
		AuctionNFTAddress: "0x7913Ff1EaA12887ed80a3D35c81c0033FFafadCe",
		IndexerContracts: []config.IndexerContract{
			{Name: "unknown", EventGroup: "unknown", Address: "0x0000000000000000000000000000000000000001", ABI: "not_supported"},
		},
	}, SourceStores{
		Auctions: &recordingAuctionStore{},
		Bids:     &recordingBidStore{},
		NFTs:     &recordingNFTStore{},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "not_supported")
}

type recordingAuctionStore struct{}

func (s *recordingAuctionStore) UpsertAuctionFromCreated(ctx context.Context, auction repository.AuctionModel) error {
	return nil
}

func (s *recordingAuctionStore) ApplyBidPlaced(ctx context.Context, bid repository.BidModel) error {
	return nil
}

func (s *recordingAuctionStore) MarkAuctionEnded(ctx context.Context, chainID int64, market string, auctionID uint64, txHash string, blockNumber uint64) error {
	return nil
}

func (s *recordingAuctionStore) MarkAuctionCancelled(ctx context.Context, chainID int64, market string, auctionID uint64, txHash string, blockNumber uint64) error {
	return nil
}

type recordingBidStore struct{}

func (s *recordingBidStore) InsertBid(ctx context.Context, bid repository.BidModel) error {
	return nil
}
