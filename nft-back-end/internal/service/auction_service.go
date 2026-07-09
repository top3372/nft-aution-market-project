package service

import (
	"context"
	"errors"
	"strconv"

	"nft-auction-backend/internal/repository"

	"gorm.io/gorm"
)

// AuctionService 编排拍卖相关查询。
//
// Service 固定当前后端配置的 chain_id 和 market_address，避免每个 API 请求都让前端
// 传链 ID 或合约地址，从而减少跨网络数据混查风险。
type AuctionService struct {
	ChainID int64
	Market  string
	Repo    *repository.AuctionRepository
	Bids    *repository.BidRepository
}

// AuctionListParams 是 HTTP 查询参数在 service 层的业务表达。
type AuctionListParams struct {
	Status   string
	Seller   string
	Bidder   string
	Page     int
	PageSize int
	Sort     string
}

// PagedAuctions 是拍卖分页查询结果。
type PagedAuctions struct {
	Items    []repository.AuctionModel
	Total    int64
	Page     int
	PageSize int
}

// List 查询拍卖列表。
//
// 参数中的 status/seller/bidder/sort 来自前端页面筛选；repository 会负责归一化分页和地址。
func (s AuctionService) List(ctx context.Context, params AuctionListParams) (PagedAuctions, error) {
	rows, total, err := s.Repo.List(ctx, repository.AuctionListFilter{
		ChainID:       s.ChainID,
		MarketAddress: s.Market,
		Status:        params.Status,
		Seller:        params.Seller,
		Bidder:        params.Bidder,
		Page:          params.Page,
		PageSize:      params.PageSize,
		Sort:          params.Sort,
	})
	if err != nil {
		return PagedAuctions{}, err
	}
	page, pageSize := params.Page, params.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	return PagedAuctions{Items: rows, Total: total, Page: page, PageSize: pageSize}, nil
}

// Get 根据链上 auctionId 查询单个拍卖。
//
// auctionId 是合约数组下标，不是数据库自增 id；这和前端路由 `/auctions/:auctionId` 保持一致。
func (s AuctionService) Get(ctx context.Context, auctionIDText string) (repository.AuctionModel, error) {
	auctionID, err := strconv.ParseUint(auctionIDText, 10, 64)
	if err != nil {
		return repository.AuctionModel{}, err
	}
	return s.Repo.GetByAuctionID(ctx, s.ChainID, s.Market, auctionID)
}

// BidsForAuction 查询某个拍卖的全部出价历史。
func (s AuctionService) BidsForAuction(ctx context.Context, auctionIDText string) ([]repository.BidModel, error) {
	auctionID, err := strconv.ParseUint(auctionIDText, 10, 64)
	if err != nil {
		return nil, err
	}
	return s.Bids.ListByAuction(ctx, s.ChainID, s.Market, auctionID)
}

// IsNotFound 判断 repository 是否返回“未找到”。
//
// API 层用它把数据库不存在映射成 HTTP 404，而不是 500。
func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
