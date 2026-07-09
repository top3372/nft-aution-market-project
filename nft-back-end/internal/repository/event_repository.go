package repository

import (
	"context"
	"errors"
	"strings"

	"nft-auction-backend/internal/evmindexer"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// EventRepository 负责原始链上事件的幂等写入。
//
// auction_events 是所有派生查询表的审计入口。即使 handler 后续有 bug，也可以通过
// 原始事件表定位具体 tx_hash/log_index，再决定是否重放或人工修复业务表。
type EventRepository struct {
	db *gorm.DB
}

func NewEventRepository(db *gorm.DB) *EventRepository {
	return &EventRepository{db: db}
}

// InsertOnce 插入一条原始事件。
//
// tx_hash + log_index 是 EVM 日志的天然唯一键。重复键不是错误，而是说明该事件已经
// 被之前的 indexer 批次处理过；Runner 收到 inserted=false 后会跳过业务 handler，
// 避免重复写 bids、重复刷新状态。
// 返回 inserted=false 表示同一个 tx_hash + log_index 已经处理过，业务 handler 应跳过。
func (r *EventRepository) InsertOnce(ctx context.Context, event evmindexer.EventRecord) (bool, error) {
	model := AuctionEventModel{
		ChainID:         event.ChainID,
		ContractAddress: normalize(event.ContractAddress),
		EventName:       strings.TrimSpace(event.EventName),
		TxHash:          normalize(event.TxHash),
		LogIndex:        event.LogIndex,
		BlockNumber:     event.BlockNumber,
		BlockHash:       normalize(event.BlockHash),
		PayloadJSON:     event.PayloadJSON,
	}

	err := r.db.WithContext(ctx).Create(&model).Error
	if err == nil {
		return true, nil
	}
	if isDuplicateKey(err) {
		return false, nil
	}
	return false, err
}

// isDuplicateKey 兼容 MySQL 驱动错误和测试/不同驱动返回的普通错误文本。
func isDuplicateKey(err error) bool {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate")
}
