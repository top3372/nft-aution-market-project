package repository

import "time"

// AuctionModel 对应 auctions 表。金额使用字符串保存，避免 uint256 溢出。
type AuctionModel struct {
	ID                 uint64 `gorm:"primaryKey;autoIncrement;column:id"`
	ChainID            int64
	MarketAddress      string
	AuctionID          uint64
	Seller             string
	NFTAddress         string
	TokenID            string
	StartTime          *time.Time
	EndTime            time.Time
	StartingPriceUSD   string
	PaymentToken       string
	HighestBidder      *string
	HighestBid         string
	HighestBidUSD      string
	Status             string
	CreatedTxHash      string
	CreatedBlockNumber uint64
	EndedTxHash        *string
	EndedBlockNumber   *uint64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (AuctionModel) TableName() string {
	return "auctions"
}

// BidModel 对应 bids 表，用 tx_hash + log_index 保证链上事件幂等。
type BidModel struct {
	ID            uint64 `gorm:"primaryKey;autoIncrement;column:id"`
	ChainID       int64
	MarketAddress string
	AuctionID     uint64
	Bidder        string
	PaymentToken  string
	Amount        string
	AmountUSD     string
	TxHash        string
	LogIndex      uint
	BlockNumber   uint64
	BlockHash     string
	BlockTime     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (BidModel) TableName() string {
	return "bids"
}

// AuctionEventModel 保存原始事件 payload，是 indexer 幂等和排错的基础。
type AuctionEventModel struct {
	ID              uint64 `gorm:"primaryKey;autoIncrement;column:id"`
	ChainID         int64
	ContractAddress string
	EventName       string
	TxHash          string
	LogIndex        uint
	BlockNumber     uint64
	BlockHash       string
	PayloadJSON     string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (AuctionEventModel) TableName() string {
	return "auction_events"
}

// SyncCursorModel 对应 sync_cursors 表。
//
// 每个 EVM source 使用 chain_id + contract_address + event_group 保存独立同步进度。
// LastScannedBlockHash 用于下一轮启动时检查链重组，避免继续基于旧分叉更新查询表。
type SyncCursorModel struct {
	ID                   uint64 `gorm:"primaryKey;autoIncrement;column:id"`
	ChainID              int64
	ContractAddress      string
	EventGroup           string
	LastScannedBlock     uint64
	LastScannedBlockHash string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (SyncCursorModel) TableName() string {
	return "sync_cursors"
}

// IndexerFailedEventModel 对应 indexer_failed_events 表。
// 该表是索引框架的 dead-letter 队列，用于保存解码或业务落库失败的链上事件上下文。
type IndexerFailedEventModel struct {
	ID              uint64 `gorm:"primaryKey;autoIncrement;column:id"`
	ChainID         int64
	ContractAddress string
	EventName       string
	TxHash          string
	LogIndex        uint
	BlockNumber     uint64
	BlockHash       string
	Stage           string
	ErrorMessage    string
	PayloadJSON     string
	RetryCount      uint
	ResolvedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (IndexerFailedEventModel) TableName() string {
	return "indexer_failed_events"
}

// NFTModel 对应 nfts 表。
//
// 该表是 AuctionNFT 的查询缓存，主要服务“我的 NFT”和创建拍卖时的 NFT 下拉选择。
// OwnerAddress 由 AuctionNFT Transfer 事件维护；TokenURI/MetadataJSON 预留给
// metadata worker 后续异步刷新。
type NFTModel struct {
	ID                 uint64 `gorm:"primaryKey;autoIncrement;column:id"`
	ChainID            int64
	NFTAddress         string
	TokenID            string
	OwnerAddress       *string
	TokenURI           *string
	MetadataJSON       *string
	LastMetadataSyncAt *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (NFTModel) TableName() string {
	return "nfts"
}

// PlatformStatsModel 对应 platform_stats 表。
//
// 该表保存定时统计快照，避免首页或后台统计每次请求都扫描 auctions/bids 全表。
type PlatformStatsModel struct {
	ID                    uint64 `gorm:"primaryKey;autoIncrement;column:id"`
	ChainID               int64
	MarketAddress         string
	ActiveAuctionCount    uint64
	EndedAuctionCount     uint64
	CancelledAuctionCount uint64
	TotalBidCount         uint64
	TotalVolumeUSD        string
	SnapshotTime          time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func (PlatformStatsModel) TableName() string {
	return "platform_stats"
}
