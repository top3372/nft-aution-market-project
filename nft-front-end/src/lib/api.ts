import type { Auction, Bid, NftToken, PagedResult, PlatformStats, WalletAuctions } from "./types";
import { appConfig } from "./config";

/**
 * 统一封装后端 REST API 的错误信息。
 *
 * 业务页面不直接解析 HTTP Response，而是捕获 ApiError 后展示后端返回的
 * 具体错误文本；status 会保留下来，后续如果要区分 404、500 等场景时可以复用。
 */
export class ApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
  ) {
    super(message);
  }
}

/**
 * 拍卖列表查询条件。
 *
 * 这些字段会被转换成 `/api/auctions` 的 query string，由 Go 后端在数据库中
 * 做分页、状态过滤、卖家/出价人过滤和排序。这里不做链上读取，列表页展示的是
 * indexer 已经同步到 MySQL 的快照数据。
 */
export interface AuctionListParams {
  page?: number;
  pageSize?: number;
  status?: string;
  seller?: string;
  bidder?: string;
  sort?: string;
}

/**
 * 所有 REST 请求共用的轻量请求器。
 *
 * 前端约定后端返回 JSON；非 2xx 时直接把响应文本带给页面层，避免页面只显示
 * “加载失败”而丢掉后端给出的具体诊断信息。
 */
async function request<T>(path: string): Promise<T> {
  const response = await fetch(`${appConfig.apiBaseUrl}${path}`);
  if (!response.ok) {
    throw new ApiError(await response.text(), response.status);
  }
  return (await response.json()) as T;
}

/**
 * 查询拍卖市场列表。
 *
 * 空字符串和 undefined 不会写入 query string，这样“全部状态”等默认筛选不会
 * 生成无意义参数，后端可以按默认条件返回完整列表。
 */
export function listAuctions(params: AuctionListParams): Promise<PagedResult<Auction>> {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== "") {
      query.set(key, String(value));
    }
  }
  return request<PagedResult<Auction>>(`/api/auctions?${query.toString()}`);
}

/**
 * 查询单个拍卖详情。
 *
 * 详情页依赖该接口展示卖家、NFT、价格和状态；链上交易成功后会重新失效该查询，
 * 等 indexer 把最新事件写入数据库后页面即可刷新到新状态。
 */
export function getAuction(auctionId: string): Promise<Auction> {
  return request<Auction>(`/api/auctions/${auctionId}`);
}

/**
 * 查询某个拍卖的出价历史。
 *
 * 后端返回 `{ items }` 是为了和其他列表接口保持一致，组件层只需要 Bid 数组，
 * 所以这里剥离一层包装。
 */
export async function listBids(auctionId: string): Promise<Bid[]> {
  const result = await request<{ items: Bid[] }>(`/api/auctions/${auctionId}/bids`);
  return result.items;
}

/**
 * 查询平台统计快照。
 *
 * 统计数据由后端 worker 根据索引表定时汇总，前端只负责读取最新快照，不在浏览器
 * 侧重新累计成交额或拍卖数量。
 */
export function getStats(): Promise<PlatformStats> {
  return request<PlatformStats>("/api/stats");
}

/**
 * 查询钱包持有的 NFT。
 *
 * 该接口来自后端索引表，可能会比链上事件慢几个区块；页面层会再合并一次链上直读
 * 的结果，保证刚铸造或刚转移的 NFT 能尽快显示。
 */
export async function listWalletNfts(address: string): Promise<NftToken[]> {
  const result = await request<{ items: NftToken[] }>(`/api/wallets/${address}/nfts`);
  return result.items;
}

/**
 * 查询钱包创建和参与过的拍卖。
 *
 * `created` 用卖家地址过滤，`participated` 用出价人历史过滤，适合“我的”页面一次性
 * 展示用户和拍卖市场的关系。
 */
export function listWalletAuctions(address: string): Promise<WalletAuctions> {
  return request<WalletAuctions>(`/api/wallets/${address}/auctions`);
}
