package worker

import "context"

// MetadataWorker 预留 NFT 元数据刷新入口。
// 第一版先保持轻量结构，后续可以在这里接入 IPFS/HTTP metadata 拉取和失败重试。
type MetadataWorker struct{}

// RunOnce 执行一次 NFT metadata 刷新。
//
// 当前版本尚未接入 IPFS/HTTP 拉取，只响应 ctx 取消。保留该方法是为了让 worker
// 进程结构稳定，后续可以在这里按 nfts.last_metadata_sync_at 扫描待刷新 NFT。
func (w MetadataWorker) RunOnce(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
