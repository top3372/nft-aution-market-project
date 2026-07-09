package nft

import "time"

// Token 表示当前 AuctionNFT 合约下的 NFT 查询缓存。
type Token struct {
	ChainID            int64
	NFTAddress         string
	TokenID            string
	OwnerAddress       string
	TokenURI           string
	MetadataJSON       string
	LastMetadataSyncAt *time.Time
}
