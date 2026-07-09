package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nft-auction-backend/internal/config"
	"nft-auction-backend/internal/database"
	"nft-auction-backend/internal/repository"
	"nft-auction-backend/internal/worker"
)

// main 启动后台 worker 进程。
//
// worker 和 API/indexer 分离运行：API 负责实时查询，indexer 负责链上事件同步，
// worker 负责可异步完成的统计聚合和 metadata 刷新，避免这些任务阻塞用户请求。
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

	// StatsWorker 当前每分钟聚合一次拍卖状态数量，后续可增加成交额和其他指标。
	statsWorker := worker.StatsWorker{
		ChainID: cfg.ChainID,
		Market:  cfg.MarketAddress,
		Source:  repository.NewAuctionRepository(db),
		Sink:    repository.NewStatsRepository(db),
	}
	// MetadataWorker 当前是预留入口，后续用于拉取 tokenURI 指向的 JSON 元数据。
	metadataWorker := worker.MetadataWorker{}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		if err := statsWorker.RunOnce(ctx); err != nil {
			log.Printf("stats worker failed: %v", err)
		}
		if err := metadataWorker.RunOnce(ctx); err != nil && ctx.Err() == nil {
			log.Printf("metadata worker failed: %v", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
