package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// NFTRepository 负责 nfts 表的写入和查询。
//
// nfts 表是 AuctionNFT 的本地查询缓存，当前 owner 由 AuctionNFT.Transfer 事件维护。
type NFTRepository struct {
	db *gorm.DB
}

// NewNFTRepository 创建 NFT 仓储。
func NewNFTRepository(db *gorm.DB) *NFTRepository {
	return &NFTRepository{db: db}
}

// UpsertNFT 写入或更新 NFT 缓存。
//
// chain_id + nft_address + token_id 是 NFT 业务唯一键。Transfer 事件只更新 owner；
// 后续 metadata worker 可以复用同一方法更新 token_uri 和 metadata_json。
func (r *NFTRepository) UpsertNFT(ctx context.Context, token NFTModel) error {
	token.NFTAddress = normalize(token.NFTAddress)
	if token.OwnerAddress != nil {
		owner := normalize(*token.OwnerAddress)
		token.OwnerAddress = &owner
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "chain_id"},
			{Name: "nft_address"},
			{Name: "token_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"owner_address",
			"token_uri",
			"metadata_json",
			"last_metadata_sync_at",
			"updated_at",
		}),
	}).Create(&token).Error
}

// ListByOwner 查询某钱包当前持有的 NFT。
//
// “我的 NFT”和创建拍卖页的 NFT 下拉框使用该查询。
func (r *NFTRepository) ListByOwner(ctx context.Context, chainID int64, nftAddress string, owner string) ([]NFTModel, error) {
	var rows []NFTModel
	err := r.db.WithContext(ctx).
		Where("chain_id = ? AND nft_address = ? AND owner_address = ?", chainID, normalize(nftAddress), normalize(owner)).
		Order("id DESC").
		Find(&rows).Error
	return rows, err
}
