package api

import (
	"net/http"
	"strconv"

	"nft-auction-backend/internal/service"

	"github.com/gin-gonic/gin"
)

// Handler 是 REST API 的 HTTP 入口。
//
// Handler 只负责读取 query/path 参数、调用 service、把 repository model 转成 DTO。
// 业务规则和数据库细节分别放在 service/repository 层。
type Handler struct {
	Services HandlerServices
}

// Health 返回服务存活状态，便于本地联调和部署健康检查。
func (h Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ListAuctions 返回拍卖列表。
//
// 首页和市场页通过该接口按状态、卖家、出价人、分页和排序查询 MySQL 中的索引结果。
func (h Handler) ListAuctions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	result, err := h.Services.Auctions.List(c.Request.Context(), service.AuctionListParams{
		Status:   c.Query("status"),
		Seller:   c.Query("seller"),
		Bidder:   c.Query("bidder"),
		Page:     page,
		PageSize: pageSize,
		Sort:     c.Query("sort"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	items := make([]AuctionDTO, 0, len(result.Items))
	for _, row := range result.Items {
		items = append(items, auctionDTO(row))
	}
	c.JSON(http.StatusOK, PagedResponse[AuctionDTO]{
		Items:    items,
		Total:    result.Total,
		Page:     result.Page,
		PageSize: result.PageSize,
	})
}

// GetAuction 返回单个链上 auctionId 对应的拍卖详情。
func (h Handler) GetAuction(c *gin.Context) {
	row, err := h.Services.Auctions.Get(c.Request.Context(), c.Param("auctionId"))
	if err != nil {
		writeQueryError(c, err)
		return
	}
	c.JSON(http.StatusOK, auctionDTO(row))
}

// ListBids 返回某个拍卖的出价历史。
//
// 出价历史来自 BidPlaced 事件，按 block_number/log_index 升序展示链上发生顺序。
func (h Handler) ListBids(c *gin.Context) {
	rows, err := h.Services.Auctions.BidsForAuction(c.Request.Context(), c.Param("auctionId"))
	if err != nil {
		writeQueryError(c, err)
		return
	}
	items := make([]BidDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, bidDTO(row))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// GetStats 返回平台统计快照。
//
// 当前统计由 worker 写入 platform_stats；如果还没有快照，service 会返回零值统计。
func (h Handler) GetStats(c *gin.Context) {
	row, err := h.Services.Stats.Latest(c.Request.Context())
	if err != nil {
		writeQueryError(c, err)
		return
	}
	c.JSON(http.StatusOK, statsDTO(row))
}

// ListWalletNFTs 返回某个钱包当前持有的 AuctionNFT。
//
// 数据来自 indexer 监听 AuctionNFT.Transfer 后维护的 nfts.owner_address。
func (h Handler) ListWalletNFTs(c *gin.Context) {
	rows, err := h.Services.Wallets.NFTs(c.Request.Context(), c.Param("address"))
	if err != nil {
		writeQueryError(c, err)
		return
	}
	items := make([]NFTDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, nftDTO(row))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// ListWalletAuctions 返回钱包创建和参与过的拍卖。
//
// created 按 seller 匹配；participated 按 highest_bidder 匹配，适合“我的”页面展示。
func (h Handler) ListWalletAuctions(c *gin.Context) {
	created, participated, err := h.Services.Wallets.AuctionsForWallet(c.Request.Context(), c.Param("address"))
	if err != nil {
		writeQueryError(c, err)
		return
	}

	createdDTO := make([]AuctionDTO, 0, len(created))
	for _, row := range created {
		createdDTO = append(createdDTO, auctionDTO(row))
	}
	participatedDTO := make([]AuctionDTO, 0, len(participated))
	for _, row := range participated {
		participatedDTO = append(participatedDTO, auctionDTO(row))
	}

	c.JSON(http.StatusOK, gin.H{
		"created":      createdDTO,
		"participated": participatedDTO,
	})
}

// writeQueryError 统一把 service/repository 错误转换成 HTTP 响应。
//
// gorm.ErrRecordNotFound 对用户是 404，其他数据库或业务错误返回 500。
func writeQueryError(c *gin.Context, err error) {
	if service.IsNotFound(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}
