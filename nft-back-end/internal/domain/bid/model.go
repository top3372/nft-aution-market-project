package bid

import "time"

// Bid 表示一次链上出价事件的查询模型。
type Bid struct {
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
}
