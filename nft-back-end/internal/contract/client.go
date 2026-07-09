package contract

import (
	"context"

	"github.com/ethereum/go-ethereum/ethclient"
)

// DialRPC 创建以太坊 RPC 客户端。调用方负责关闭底层连接。
func DialRPC(ctx context.Context, rpcURL string) (*ethclient.Client, error) {
	return ethclient.DialContext(ctx, rpcURL)
}
