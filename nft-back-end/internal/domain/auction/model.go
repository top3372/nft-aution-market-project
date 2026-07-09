package auction

import "time"

// Status 是后端查询模型中的拍卖状态。
// 链上事件是最终事实来源；数据库状态用于列表筛选和页面展示。
type Status string

const (
	StatusPending   Status = "pending"
	StatusActive    Status = "active"
	StatusEnded     Status = "ended"
	StatusCancelled Status = "cancelled"
)

// ResolveStatus 根据链上终态和当前时间计算展示状态。
// ended/cancelled 由链上事件决定，pending/active 可由 worker 按时间刷新。
func ResolveStatus(now time.Time, startTime *time.Time, endTime time.Time, ended bool, cancelled bool) Status {
	if cancelled {
		return StatusCancelled
	}
	if ended || !now.Before(endTime) {
		return StatusEnded
	}
	if startTime != nil && now.Before(*startTime) {
		return StatusPending
	}
	return StatusActive
}

// Auction 是服务层使用的拍卖领域对象，金额字段保存链上十进制字符串。
type Auction struct {
	ChainID          int64
	MarketAddress    string
	AuctionID        uint64
	Seller           string
	NFTAddress       string
	TokenID          string
	StartTime        *time.Time
	EndTime          time.Time
	StartingPriceUSD string
	PaymentToken     string
	HighestBidder    string
	HighestBid       string
	HighestBidUSD    string
	Status           Status
}
