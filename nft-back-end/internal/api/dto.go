package api

import (
	"time"

	"nft-auction-backend/internal/repository"
)

// AuctionDTO 是前端拍卖列表和详情使用的响应结构。
//
// 金额字段保持字符串，避免 JavaScript number 丢失 uint256 精度。
type AuctionDTO struct {
	AuctionID        uint64  `json:"auctionId"`
	Seller           string  `json:"seller"`
	NFTAddress       string  `json:"nftAddress"`
	TokenID          string  `json:"tokenId"`
	StartTime        *string `json:"startTime"`
	EndTime          string  `json:"endTime"`
	StartingPriceUSD string  `json:"startingPriceUsd"`
	Status           string  `json:"status"`
	HighestBidder    string  `json:"highestBidder"`
	HighestBid       string  `json:"highestBid"`
	HighestBidUSD    string  `json:"highestBidUsd"`
}

// BidDTO 是拍卖详情页的出价历史响应结构。
//
// txHash/logIndex/blockNumber 保留链上定位信息，便于前端跳转区块浏览器或排查事件。
type BidDTO struct {
	Bidder       string  `json:"bidder"`
	PaymentToken string  `json:"paymentToken"`
	Amount       string  `json:"amount"`
	AmountUSD    string  `json:"amountUsd"`
	TxHash       string  `json:"txHash"`
	LogIndex     uint    `json:"logIndex"`
	BlockNumber  uint64  `json:"blockNumber"`
	BlockTime    *string `json:"blockTime"`
}

// NFTDTO 是“我的 NFT”和创建拍卖页 NFT 下拉框使用的响应结构。
type NFTDTO struct {
	NFTAddress   string `json:"nftAddress"`
	TokenID      string `json:"tokenId"`
	OwnerAddress string `json:"ownerAddress"`
	TokenURI     string `json:"tokenUri"`
	MetadataJSON string `json:"metadataJson"`
}

// StatsDTO 是首页或后台展示的平台统计快照。
type StatsDTO struct {
	ActiveAuctionCount    uint64 `json:"activeAuctionCount"`
	EndedAuctionCount     uint64 `json:"endedAuctionCount"`
	CancelledAuctionCount uint64 `json:"cancelledAuctionCount"`
	TotalBidCount         uint64 `json:"totalBidCount"`
	TotalVolumeUSD        string `json:"totalVolumeUsd"`
	SnapshotTime          string `json:"snapshotTime"`
}

// PagedResponse 是列表接口统一分页响应。
type PagedResponse[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

// auctionDTO 把数据库拍卖模型转换成前端 JSON。
//
// 展示状态会结合当前时间动态计算，避免仅依赖数据库 status 导致过期拍卖仍显示 active。
func auctionDTO(row repository.AuctionModel) AuctionDTO {
	return AuctionDTO{
		AuctionID:        row.AuctionID,
		Seller:           row.Seller,
		NFTAddress:       row.NFTAddress,
		TokenID:          row.TokenID,
		StartTime:        formatOptionalTime(row.StartTime),
		EndTime:          formatTime(row.EndTime),
		StartingPriceUSD: row.StartingPriceUSD,
		Status:           currentAuctionStatus(row, time.Now().UTC()),
		HighestBidder:    stringValue(row.HighestBidder),
		HighestBid:       row.HighestBid,
		HighestBidUSD:    row.HighestBidUSD,
	}
}

// bidDTO 把出价模型转换为 API 输出。
func bidDTO(row repository.BidModel) BidDTO {
	return BidDTO{
		Bidder:       row.Bidder,
		PaymentToken: row.PaymentToken,
		Amount:       row.Amount,
		AmountUSD:    row.AmountUSD,
		TxHash:       row.TxHash,
		LogIndex:     row.LogIndex,
		BlockNumber:  row.BlockNumber,
		BlockTime:    formatOptionalTime(row.BlockTime),
	}
}

// nftDTO 把 NFT 查询缓存转换为 API 输出。
func nftDTO(row repository.NFTModel) NFTDTO {
	return NFTDTO{
		NFTAddress:   row.NFTAddress,
		TokenID:      row.TokenID,
		OwnerAddress: stringValue(row.OwnerAddress),
		TokenURI:     stringValue(row.TokenURI),
		MetadataJSON: stringValue(row.MetadataJSON),
	}
}

// statsDTO 把统计快照转换为 API 输出。
func statsDTO(row repository.PlatformStatsModel) StatsDTO {
	return StatsDTO{
		ActiveAuctionCount:    row.ActiveAuctionCount,
		EndedAuctionCount:     row.EndedAuctionCount,
		CancelledAuctionCount: row.CancelledAuctionCount,
		TotalBidCount:         row.TotalBidCount,
		TotalVolumeUSD:        row.TotalVolumeUSD,
		SnapshotTime:          formatTime(row.SnapshotTime),
	}
}

// formatOptionalTime 将可空数据库时间转成 RFC3339 字符串。
func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := formatTime(*value)
	return &formatted
}

// formatTime 统一 API 中的时间格式。
func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

// stringValue 把数据库可空字符串转换成前端更好处理的空字符串。
func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// currentAuctionStatus 计算当前展示状态。
//
// cancelled/ended 是链上终态，优先级最高；pending/active/ended 可根据 start/end 时间
// 动态判断，减少 worker 延迟对页面展示的影响。
func currentAuctionStatus(row repository.AuctionModel, now time.Time) string {
	switch row.Status {
	case "cancelled":
		return "cancelled"
	case "ended":
		return "ended"
	}
	if !row.EndTime.After(now) {
		return "ended"
	}
	if row.StartTime != nil && row.StartTime.After(now) {
		return "pending"
	}
	return "active"
}
