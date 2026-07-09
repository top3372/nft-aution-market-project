package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nft-auction-backend/internal/config"
	"nft-auction-backend/internal/contract"
	"nft-auction-backend/internal/database"
	"nft-auction-backend/internal/evmindexer"
	"nft-auction-backend/internal/indexer"
	"nft-auction-backend/internal/repository"
)

// main 启动链上事件索引进程。
//
// 本进程不提供 HTTP API，只负责把 Sepolia 上的合约事件同步到 MySQL 查询表：
// - AuctionMarketV3 事件生成拍卖列表、出价历史和状态。
// - AuctionNFT Transfer 事件维护“我的 NFT”查询缓存。
// - AuctionPaymentToken Transfer 事件第一版只写 raw event，后续可扩展为代币流水。
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	db, err := database.OpenMySQL(cfg.MySQLDSN)
	if err != nil {
		log.Fatal(err)
	}

	client, err := contract.DialRPC(ctx, cfg.RPCURL)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	auctionRepo := repository.NewAuctionRepository(db)
	bidRepo := repository.NewBidRepository(db)
	nftRepo := repository.NewNFTRepository(db)
	sources, err := indexer.BuildContractSources(cfg, indexer.SourceStores{
		Auctions: auctionRepo,
		Bids:     bidRepo,
		NFTs:     nftRepo,
	})
	if err != nil {
		log.Fatal(err)
	}

	runner := &evmindexer.Runner{
		ChainID:       cfg.ChainID,
		Client:        client,
		BatchSize:     cfg.IndexerBatchSize,
		Confirmations: cfg.IndexerConfirmations,
		Events:        repository.NewEventRepository(db),
		Cursors:       repository.NewCursorRepository(db),
		FailedEvents:  repository.NewFailedEventRepository(db),
		Sources:       sources,
	}

	ticker := time.NewTicker(cfg.IndexerPollInterval)
	defer ticker.Stop()

	for {
		if err := runner.RunOnce(ctx); err != nil {
			log.Printf("indexer run failed: %v", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
