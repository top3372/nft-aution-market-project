package main

import (
	"log"

	"nft-auction-backend/internal/api"
	"nft-auction-backend/internal/config"
	"nft-auction-backend/internal/database"
	"nft-auction-backend/internal/repository"
	"nft-auction-backend/internal/service"
)

// main 启动 REST API 服务。
//
// API 服务只读取 MySQL 查询表，不直接访问链上 RPC。链上事件由 cmd/indexer 写入
// auctions/bids/nfts/platform_stats 后，前端再通过这里查询列表、详情、钱包资产等数据。
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	db, err := database.OpenMySQL(cfg.MySQLDSN)
	if err != nil {
		log.Fatal(err)
	}

	// Repository 负责数据库读写，Service 负责按当前 chain/market 组合查询业务数据。
	auctionRepo := repository.NewAuctionRepository(db)
	bidRepo := repository.NewBidRepository(db)
	nftRepo := repository.NewNFTRepository(db)
	statsRepo := repository.NewStatsRepository(db)

	// Router 只注入已经组装好的服务，避免 HTTP 层直接依赖 GORM 或配置细节。
	router := api.NewRouter(
		api.HandlerServices{
			Auctions: service.AuctionService{
				ChainID: cfg.ChainID,
				Market:  cfg.MarketAddress,
				Repo:    auctionRepo,
				Bids:    bidRepo,
			},
			Wallets: service.WalletService{
				ChainID:    cfg.ChainID,
				Market:     cfg.MarketAddress,
				NFTAddress: cfg.AuctionNFTAddress,
				Auctions:   auctionRepo,
				NFTRepo:    nftRepo,
			},
			Stats: service.StatsService{
				ChainID: cfg.ChainID,
				Market:  cfg.MarketAddress,
				Repo:    statsRepo,
			},
		},
		api.RouterConfig{
			CORSAllowedOrigins: cfg.CORSAllowedOrigins,
		},
	)

	if err := router.Run(cfg.HTTPAddr); err != nil {
		log.Fatal(err)
	}
}
