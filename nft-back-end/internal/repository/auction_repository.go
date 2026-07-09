package repository

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AuctionRepository 负责 auctions 表的写入和查询。
//
// auctions 是前端拍卖列表的主查询表，数据主要来自 AuctionCreatedV3、BidPlaced、
// AuctionEnded、AuctionCancelled 等链上事件。
type AuctionRepository struct {
	db *gorm.DB
}

// NewAuctionRepository 创建拍卖仓储。
func NewAuctionRepository(db *gorm.DB) *AuctionRepository {
	return &AuctionRepository{db: db}
}

// UpsertAuctionFromCreated 根据创建事件写入或更新拍卖主表。
//
// 使用 chain_id + market_address + auction_id 作为业务唯一键，保证 indexer 重放历史
// 或重复扫描同一事件时不会创建重复拍卖。
func (r *AuctionRepository) UpsertAuctionFromCreated(ctx context.Context, auction AuctionModel) error {
	auction.MarketAddress = normalize(auction.MarketAddress)
	auction.NFTAddress = normalize(auction.NFTAddress)
	auction.Seller = normalize(auction.Seller)

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "chain_id"},
			{Name: "market_address"},
			{Name: "auction_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"seller",
			"nft_address",
			"token_id",
			"start_time",
			"end_time",
			"starting_price_usd",
			"status",
			"updated_at",
		}),
	}).Create(&auction).Error
}

// ApplyBidPlaced 根据 BidPlaced 事件刷新拍卖当前最高价。
//
// bids 表保存完整出价流水，auctions 表只保存列表查询需要的最新最高价和最高出价人。
func (r *AuctionRepository) ApplyBidPlaced(ctx context.Context, bid BidModel) error {
	highestBidder := normalize(bid.Bidder)
	return r.db.WithContext(ctx).Model(&AuctionModel{}).
		Where("chain_id = ? AND market_address = ? AND auction_id = ?", bid.ChainID, normalize(bid.MarketAddress), bid.AuctionID).
		Updates(map[string]any{
			"payment_token":   normalize(bid.PaymentToken),
			"highest_bidder":  highestBidder,
			"highest_bid":     bid.Amount,
			"highest_bid_usd": bid.AmountUSD,
			"status":          "active",
		}).Error
}

// MarkAuctionEnded 把拍卖标记为 ended。
//
// 该方法由 AuctionEnded 事件触发，记录终态交易 hash 和区块号。
func (r *AuctionRepository) MarkAuctionEnded(ctx context.Context, chainID int64, market string, auctionID uint64, txHash string, blockNumber uint64) error {
	return r.markTerminal(ctx, chainID, market, auctionID, txHash, blockNumber, "ended")
}

// MarkAuctionCancelled 把拍卖标记为 cancelled。
//
// 该方法由 V3 AuctionCancelled 事件触发，只允许无人出价取消的拍卖进入该状态。
func (r *AuctionRepository) MarkAuctionCancelled(ctx context.Context, chainID int64, market string, auctionID uint64, txHash string, blockNumber uint64) error {
	return r.markTerminal(ctx, chainID, market, auctionID, txHash, blockNumber, "cancelled")
}

// AuctionListFilter 是 repository 层的拍卖查询条件。
//
// Service 会注入固定 chain_id/market_address，API 层只暴露 status/seller/bidder/page/sort。
type AuctionListFilter struct {
	ChainID       int64
	MarketAddress string
	Status        string
	Seller        string
	Bidder        string
	Page          int
	PageSize      int
	Sort          string
}

// List 查询拍卖列表并返回总数。
//
// 状态筛选会结合当前时间计算 pending/active/ended，使页面不必等待 worker 刷新时间状态。
func (r *AuctionRepository) List(ctx context.Context, filter AuctionListFilter) ([]AuctionModel, int64, error) {
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	query := r.db.WithContext(ctx).Model(&AuctionModel{}).
		Where("chain_id = ? AND market_address = ?", filter.ChainID, normalize(filter.MarketAddress))
	if filter.Status != "" {
		query = applyStatusFilter(query, filter.Status, time.Now().UTC())
	}
	if filter.Seller != "" {
		query = query.Where("seller = ?", normalize(filter.Seller))
	}
	if filter.Bidder != "" {
		query = query.Where("highest_bidder = ?", normalize(filter.Bidder))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []AuctionModel
	err := query.Order(orderBy(filter.Sort)).
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&rows).Error
	return rows, total, err
}

// GetByAuctionID 按链上 auctionId 查询拍卖。
//
// 注意 auctionId 是合约里的数组下标，不是数据库自增 id。
func (r *AuctionRepository) GetByAuctionID(ctx context.Context, chainID int64, market string, auctionID uint64) (AuctionModel, error) {
	var row AuctionModel
	err := r.db.WithContext(ctx).
		Where("chain_id = ? AND market_address = ? AND auction_id = ?", chainID, normalize(market), auctionID).
		First(&row).Error
	return row, err
}

// markTerminal 写入 ended/cancelled 终态字段。
//
// 终态只由链上事件驱动，API 不提供直接修改状态的写接口。
func (r *AuctionRepository) markTerminal(ctx context.Context, chainID int64, market string, auctionID uint64, txHash string, blockNumber uint64, status string) error {
	hash := strings.ToLower(strings.TrimSpace(txHash))
	return r.db.WithContext(ctx).Model(&AuctionModel{}).
		Where("chain_id = ? AND market_address = ? AND auction_id = ?", chainID, normalize(market), auctionID).
		Updates(map[string]any{
			"status":             status,
			"ended_tx_hash":      hash,
			"ended_block_number": blockNumber,
		}).Error
}

// normalizePage 统一分页边界。
//
// pageSize 最大 100，防止前端一次请求拉取过多数据影响数据库。
func normalizePage(page int, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

// orderBy 把前端 sort 参数转换成安全的 SQL order 片段。
//
// 不直接拼接用户输入，避免任意 SQL 排序表达式注入。
func orderBy(sort string) string {
	switch sort {
	case "end_time_asc":
		return "end_time ASC"
	case "created_desc":
		return "created_block_number DESC"
	default:
		return "end_time DESC"
	}
}

// applyStatusFilter 把业务状态转换成数据库查询条件。
//
// pending/active/ended 会结合 start_time/end_time 动态判断；cancelled 只能来自链上取消事件。
func applyStatusFilter(query *gorm.DB, status string, now time.Time) *gorm.DB {
	switch status {
	case "pending":
		return query.Where("status NOT IN ? AND start_time IS NOT NULL AND start_time > ?", []string{"cancelled", "ended"}, now)
	case "active":
		return query.Where("status NOT IN ? AND (start_time IS NULL OR start_time <= ?) AND end_time > ?", []string{"cancelled", "ended"}, now, now)
	case "ended":
		return query.Where("status = ? OR (status != ? AND end_time <= ?)", "ended", "cancelled", now)
	case "cancelled":
		return query.Where("status = ?", "cancelled")
	default:
		return query.Where("status = ?", status)
	}
}
