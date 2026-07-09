package service

import (
	"context"

	"nft-auction-backend/internal/repository"
)

// WalletService 编排钱包维度的查询。
//
// “我的”页面需要同时看到 NFT、我创建的拍卖、我参与的拍卖。这里把这些查询收敛到
// 当前配置的 chain/market/NFT 合约范围内，避免跨链或跨合约数据混在一起。
type WalletService struct {
	ChainID    int64
	Market     string
	NFTAddress string
	Auctions   *repository.AuctionRepository
	NFTRepo    *repository.NFTRepository
}

// NFTs 查询某个钱包当前持有的 AuctionNFT。
//
// 数据来自 indexer 对 AuctionNFT.Transfer 事件的同步；前端仍会做链上扫描作为延迟兜底。
func (s WalletService) NFTs(ctx context.Context, wallet string) ([]repository.NFTModel, error) {
	return s.NFTRepo.ListByOwner(ctx, s.ChainID, s.NFTAddress, wallet)
}

// AuctionsForWallet 查询钱包创建和参与过的拍卖。
//
// created 使用 seller 过滤，participated 使用 highest_bidder 过滤。当前版本只展示当前最高
// 出价仍属于该钱包的拍卖；如果要展示历史参与记录，可改为从 bids 表按 bidder 聚合。
func (s WalletService) AuctionsForWallet(ctx context.Context, wallet string) (created []repository.AuctionModel, participated []repository.AuctionModel, err error) {
	created, _, err = s.Auctions.List(ctx, repository.AuctionListFilter{
		ChainID:       s.ChainID,
		MarketAddress: s.Market,
		Seller:        wallet,
		Page:          1,
		PageSize:      100,
		Sort:          "created_desc",
	})
	if err != nil {
		return nil, nil, err
	}
	participated, _, err = s.Auctions.List(ctx, repository.AuctionListFilter{
		ChainID:       s.ChainID,
		MarketAddress: s.Market,
		Bidder:        wallet,
		Page:          1,
		PageSize:      100,
		Sort:          "created_desc",
	})
	return created, participated, err
}
