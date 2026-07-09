package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BidRepository 负责 bids 表的写入和查询。
//
// bids 表保存完整出价历史，主要来自链上 BidPlaced 事件。
type BidRepository struct {
	db *gorm.DB
}

// NewBidRepository 创建出价仓储。
func NewBidRepository(db *gorm.DB) *BidRepository {
	return &BidRepository{db: db}
}

// InsertBid 写入一条出价记录。
//
// 使用 tx_hash + log_index 幂等；同一链上事件被重复扫描时直接忽略，避免出价历史重复。
func (r *BidRepository) InsertBid(ctx context.Context, bid BidModel) error {
	bid.MarketAddress = normalize(bid.MarketAddress)
	bid.Bidder = normalize(bid.Bidder)
	bid.PaymentToken = normalize(bid.PaymentToken)

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tx_hash"}, {Name: "log_index"}},
		DoNothing: true,
	}).Create(&bid).Error
}

// ListByAuction 按拍卖 ID 查询出价历史。
//
// 排序按 block_number、log_index 升序，和链上日志发生顺序一致。
func (r *BidRepository) ListByAuction(ctx context.Context, chainID int64, market string, auctionID uint64) ([]BidModel, error) {
	var rows []BidModel
	err := r.db.WithContext(ctx).
		Where("chain_id = ? AND market_address = ? AND auction_id = ?", chainID, normalize(market), auctionID).
		Order("block_number ASC, log_index ASC").
		Find(&rows).Error
	return rows, err
}
