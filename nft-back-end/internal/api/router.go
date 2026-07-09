package api

import (
	"net/http"

	"nft-auction-backend/internal/service"

	"github.com/gin-gonic/gin"
)

// HandlerServices 汇总 HTTP handler 需要调用的业务服务。
//
// 这里按业务域拆分为 Auctions、Wallets、Stats，避免单个 handler 直接持有多个 repository。
type HandlerServices struct {
	Auctions service.AuctionService
	Wallets  service.WalletService
	Stats    service.StatsService
}

// RouterConfig 保存 HTTP 层配置。
//
// CORSAllowedOrigins 来自 config.yaml，前端本地开发和正式域名都应在这里配置。
type RouterConfig struct {
	CORSAllowedOrigins []string
}

// NewRouter 注册 REST API 路由。Handler 只做 HTTP 参数处理，业务查询交给 service。
func NewRouter(services HandlerServices, cfg RouterConfig) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(corsMiddleware(cfg.CORSAllowedOrigins))

	handler := Handler{Services: services}
	api := router.Group("/api")
	api.GET("/health", handler.Health)
	api.GET("/auctions", handler.ListAuctions)
	api.GET("/auctions/:auctionId", handler.GetAuction)
	api.GET("/auctions/:auctionId/bids", handler.ListBids)
	api.GET("/stats", handler.GetStats)
	api.GET("/wallets/:address/nfts", handler.ListWalletNFTs)
	api.GET("/wallets/:address/auctions", handler.ListWalletAuctions)

	return router
}

// corsMiddleware 处理前端 DApp 跨域访问。
//
// 当前 API 主要被 Vite 前端调用；只允许配置中的 Origin，避免任意站点直接复用本地 API。
func corsMiddleware(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = struct{}{}
	}

	return func(ctx *gin.Context) {
		origin := ctx.GetHeader("Origin")
		if _, ok := allowed[origin]; ok {
			header := ctx.Writer.Header()
			header.Set("Access-Control-Allow-Origin", origin)
			header.Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			header.Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
			header.Set("Vary", "Origin")
		}

		if ctx.Request.Method == http.MethodOptions {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}

		ctx.Next()
	}
}
