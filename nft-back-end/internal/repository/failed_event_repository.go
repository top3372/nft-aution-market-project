package repository

import (
	"context"
	"strings"

	"nft-auction-backend/internal/evmindexer"

	"gorm.io/gorm"
)

// FailedEventRepository 负责写入 indexer_failed_events 死信表。
//
// 死信表不是业务查询表，而是运维和排错表：当解码、raw event 写入或业务 handler
// 失败时，Runner 会把失败上下文写进这里，并停止推进 cursor。这样下一轮仍会重试
// 同一批区块，同时开发者能看到失败发生在哪个阶段。
type FailedEventRepository struct {
	db *gorm.DB
}

// NewFailedEventRepository 创建索引失败事件仓储。
func NewFailedEventRepository(db *gorm.DB) *FailedEventRepository {
	return &FailedEventRepository{db: db}
}

// InsertFailed 保存一条失败事件上下文。
//
// 失败记录允许重复写入：同一个坏事件在每次重试失败时都留下时间线，便于判断是偶发
// 数据库/RPC 问题，还是稳定的解码或业务规则问题。后续如果要做自动重试队列，可再
// 基于 tx_hash + log_index + stage 做聚合。
func (r *FailedEventRepository) InsertFailed(ctx context.Context, event evmindexer.FailedEventRecord) error {
	model := IndexerFailedEventModel{
		ChainID:         event.ChainID,
		ContractAddress: normalize(event.ContractAddress),
		EventName:       strings.TrimSpace(event.EventName),
		TxHash:          normalize(event.TxHash),
		LogIndex:        event.LogIndex,
		BlockNumber:     event.BlockNumber,
		BlockHash:       normalize(event.BlockHash),
		Stage:           strings.TrimSpace(event.Stage),
		ErrorMessage:    strings.TrimSpace(event.ErrorMessage),
		PayloadJSON:     event.PayloadJSON,
	}
	return r.db.WithContext(ctx).Create(&model).Error
}
