package indexer

import (
	"context"
	"strconv"
	"strings"
	"time"

	"nft-auction-backend/internal/evmindexer"
	"nft-auction-backend/internal/repository"
)

type AuctionStore interface {
	UpsertAuctionFromCreated(ctx context.Context, auction repository.AuctionModel) error
	ApplyBidPlaced(ctx context.Context, bid repository.BidModel) error
	MarkAuctionEnded(ctx context.Context, chainID int64, market string, auctionID uint64, txHash string, blockNumber uint64) error
	MarkAuctionCancelled(ctx context.Context, chainID int64, market string, auctionID uint64, txHash string, blockNumber uint64) error
}

type BidStore interface {
	InsertBid(ctx context.Context, bid repository.BidModel) error
}

// NFTStore 抽象 NFT 查询缓存的写入能力。
//
// indexer 只关心“链上 Transfer 事件发生后，查询表 owner 应该变成什么”，具体是
// GORM、事务还是测试 fake，由 repository 层决定。
type NFTStore interface {
	UpsertNFT(ctx context.Context, token repository.NFTModel) error
}

// MarketEventHandler 把已解码的链上事件应用到查询表。
//
// 拍卖市场事件是 REST API 的核心数据来源：
// - AuctionCreatedV3 生成 auctions 主表记录。
// - BidPlaced 追加 bids 出价历史，并刷新 auctions 当前最高价。
// - AuctionEnded/AuctionCancelled 把拍卖改为终态。
//
// Handler 不直接访问 RPC，也不处理游标；这些横切能力由 evmindexer.Runner 统一完成。
type MarketEventHandler struct {
	MarketAddress string
	NFTAddress    string
	Auctions      AuctionStore
	Bids          BidStore
}

// Handle 根据事件名称分发到具体业务落库方法。
//
// 未进入查询表的市场事件（如 FeeConfigUpdated）会保留在 raw event 表里，但这里返回
// nil，不阻塞游标推进；将来需要 API 查询这些事件时再补对应 handler。
func (h MarketEventHandler) Handle(ctx context.Context, event evmindexer.DecodedEvent) error {
	switch event.Name {
	case "AuctionCreatedV3":
		return h.handleAuctionCreatedV3(ctx, event)
	case "BidPlaced":
		return h.handleBidPlaced(ctx, event)
	case "AuctionEnded":
		auctionID := parseUint(event.Payload["auction_id"])
		return h.Auctions.MarkAuctionEnded(ctx, event.ChainID, h.MarketAddress, auctionID, event.TxHash, event.BlockNumber)
	case "AuctionCancelled":
		auctionID := parseUint(event.Payload["auction_id"])
		return h.Auctions.MarkAuctionCancelled(ctx, event.ChainID, h.MarketAddress, auctionID, event.TxHash, event.BlockNumber)
	default:
		return nil
	}
}

// handleAuctionCreatedV3 把 V3 创建事件写成拍卖查询主表。
//
// 合约中的 auction_id 是链上业务主键，数据库还会有自增 id；这里按业务主键 upsert，
// 这样 indexer 重放或重复启动时不会制造重复拍卖。
func (h MarketEventHandler) handleAuctionCreatedV3(ctx context.Context, event evmindexer.DecodedEvent) error {
	startTime := unixTime(event.Payload["start_time"])
	endTime := unixTime(event.Payload["end_time"])
	return h.Auctions.UpsertAuctionFromCreated(ctx, repository.AuctionModel{
		ChainID:            event.ChainID,
		MarketAddress:      h.MarketAddress,
		AuctionID:          parseUint(event.Payload["auction_id"]),
		Seller:             event.Payload["seller"],
		NFTAddress:         h.NFTAddress,
		TokenID:            event.Payload["token_id"],
		StartTime:          &startTime,
		EndTime:            endTime,
		StartingPriceUSD:   event.Payload["starting_price_usd"],
		PaymentToken:       "0x0000000000000000000000000000000000000000",
		HighestBid:         "0",
		HighestBidUSD:      "0",
		Status:             "pending",
		CreatedTxHash:      event.TxHash,
		CreatedBlockNumber: event.BlockNumber,
	})
}

// handleBidPlaced 先保存出价流水，再刷新拍卖主表当前最高价。
//
// bids 表用 tx_hash + log_index 保证事件幂等；auctions 表保存面向列表查询的最新状态。
// 两张表都更新后，前端才能同时展示“出价历史”和“当前最高价”。
func (h MarketEventHandler) handleBidPlaced(ctx context.Context, event evmindexer.DecodedEvent) error {
	bid := repository.BidModel{
		ChainID:       event.ChainID,
		MarketAddress: h.MarketAddress,
		AuctionID:     parseUint(event.Payload["auction_id"]),
		Bidder:        event.Payload["bidder"],
		PaymentToken:  event.Payload["payment_token"],
		Amount:        event.Payload["amount"],
		AmountUSD:     event.Payload["amount_usd"],
		TxHash:        event.TxHash,
		LogIndex:      event.LogIndex,
		BlockNumber:   event.BlockNumber,
		BlockHash:     event.BlockHash,
	}
	if err := h.Bids.InsertBid(ctx, bid); err != nil {
		return err
	}
	return h.Auctions.ApplyBidPlaced(ctx, bid)
}

// NFTTransferHandler 把 AuctionNFT 的 Transfer 事件同步到 nfts 查询缓存。
//
// 前端“我的 NFT”和“创建拍卖时选择 NFT”都依赖当前 owner。只监听 AuctionMarket 事件
// 无法知道用户刚 mint 的 NFT，所以 NFT 合约必须作为独立 source 接入 EVM 索引框架。
type NFTTransferHandler struct {
	NFTAddress string
	NFTs       NFTStore
}

// Handle 处理 ERC721 Transfer。
//
// Transfer 的业务含义：
// - from = zero，to = 用户：铸造 NFT。
// - from = 用户，to = 市场合约：创建拍卖后 NFT 被托管。
// - from = 市场合约，to = 用户/卖家：拍卖结束或取消后 NFT 转出。
// - to = zero：销毁 NFT，本地 owner 置空。
func (h NFTTransferHandler) Handle(ctx context.Context, event evmindexer.DecodedEvent) error {
	if event.Name != "Transfer" {
		return nil
	}

	owner := strings.ToLower(strings.TrimSpace(event.Payload["to"]))
	var ownerPtr *string
	if owner != "" && owner != zeroAddress {
		ownerPtr = &owner
	}

	return h.NFTs.UpsertNFT(ctx, repository.NFTModel{
		ChainID:      event.ChainID,
		NFTAddress:   h.NFTAddress,
		TokenID:      event.Payload["token_id"],
		OwnerAddress: ownerPtr,
	})
}

// NoopHandler 用于只需要 raw event 审计、暂时不更新业务查询表的 source。
//
// 例如 AuctionPaymentToken 的 Transfer 事件第一版只先落 raw event/failed event，
// 用户余额仍由前端直接读链；以后要做代币流水 API 时，可以替换成真正的业务 handler。
type NoopHandler struct{}

func (NoopHandler) Handle(context.Context, evmindexer.DecodedEvent) error {
	return nil
}

func parseUint(value string) uint64 {
	parsed, _ := strconv.ParseUint(value, 10, 64)
	return parsed
}

func unixTime(value string) time.Time {
	return time.Unix(int64(parseUint(value)), 0).UTC()
}

const zeroAddress = "0x0000000000000000000000000000000000000000"
