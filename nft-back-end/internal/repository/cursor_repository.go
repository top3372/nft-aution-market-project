package repository

import (
	"context"
	"strings"

	"nft-auction-backend/internal/evmindexer"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CursorRepository 负责 sync_cursors 表的读写。
//
// EVM 索引服务不能只记“扫到了哪个区块号”，还需要记该区块 hash。下一轮启动时如果
// 同一高度 hash 改变，说明发生过链重组，框架会停止推进，避免继续基于旧分叉数据
// 更新 auctions/bids/nfts 查询表。
type CursorRepository struct {
	db *gorm.DB
}

// NewCursorRepository 创建游标仓储。
//
// 该仓储实现 evmindexer.CursorStore，cmd/indexer 会直接注入到 Runner。
func NewCursorRepository(db *gorm.DB) *CursorRepository {
	return &CursorRepository{db: db}
}

// GetCursor 读取某个合约 source 的同步进度。
//
// scope 使用 chain_id + contract_address + event_group 作为业务唯一键，保证市场、
// NFT、支付代币可以各自独立同步和回放。
func (r *CursorRepository) GetCursor(ctx context.Context, scope evmindexer.Scope) (evmindexer.Cursor, error) {
	scope = evmindexer.NormalizeScope(scope)
	var cursor SyncCursorModel
	err := r.db.WithContext(ctx).
		Where("chain_id = ? AND contract_address = ? AND event_group = ?", scope.ChainID, scope.ContractAddress, scope.EventGroup).
		First(&cursor).Error
	if err == nil {
		return evmindexer.Cursor{
			LastScannedBlock:     cursor.LastScannedBlock,
			LastScannedBlockHash: normalize(cursor.LastScannedBlockHash),
		}, nil
	}
	if errorsIsRecordNotFound(err) {
		return evmindexer.Cursor{}, nil
	}
	return evmindexer.Cursor{}, err
}

// SaveCursor 在一个批次全部处理成功后保存新的同步点。
//
// Runner 只有在 raw event 写入、业务 handler 执行、toBlock header hash 读取都成功后
// 才会调用本方法；因此这里的 upsert 代表“这个 source 到该区块已经完整落库”。
func (r *CursorRepository) SaveCursor(ctx context.Context, scope evmindexer.Scope, cursor evmindexer.Cursor) error {
	scope = evmindexer.NormalizeScope(scope)
	model := SyncCursorModel{
		ChainID:              scope.ChainID,
		ContractAddress:      scope.ContractAddress,
		EventGroup:           scope.EventGroup,
		LastScannedBlock:     cursor.LastScannedBlock,
		LastScannedBlockHash: normalize(cursor.LastScannedBlockHash),
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "chain_id"},
			{Name: "contract_address"},
			{Name: "event_group"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"last_scanned_block", "last_scanned_block_hash", "updated_at"}),
	}).Create(&model).Error
}

// normalize 统一数据库里保存的钱包/合约/hash 字符串格式。
//
// 链上地址大小写不影响语义，但数据库查询和唯一键比较需要稳定格式，所以统一转小写。
func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// errorsIsRecordNotFound 屏蔽 GORM 的不存在错误，让调用方把“从未同步过”当作 0 游标。
func errorsIsRecordNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}
