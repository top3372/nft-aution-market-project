/**
 * 前端展示用的拍卖状态。
 *
 * Go 后端会根据链上事件、开始/结束时间以及取消/结算事件计算状态，前端只使用这个
 * 枚举做筛选、徽标和按钮状态展示。
 */
export type AuctionStatus = "pending" | "active" | "ended" | "cancelled";

/**
 * 拍卖列表和详情页共用的 DTO。
 *
 * 金额字段统一使用 string，是为了完整保留链上 uint256 和 USD 8 位精度，避免
 * JavaScript number 在大整数金额上丢精度。
 */
export interface Auction {
  auctionId: number;
  seller: string;
  nftAddress: string;
  tokenId: string;
  startTime: string | null;
  endTime: string;
  startingPriceUsd: string;
  status: AuctionStatus;
  highestBidder: string;
  highestBid: string;
  highestBidUsd: string;
}

/**
 * 出价历史 DTO。
 *
 * `txHash + logIndex` 对应链上事件的唯一位置，页面表格也使用它生成稳定 key；
 * `amount` 是 ERC20 原始数量，`amountUsd` 是后端按价格预言机折算后的 USD 8 位值。
 */
export interface Bid {
  bidder: string;
  paymentToken: string;
  amount: string;
  amountUsd: string;
  txHash: string;
  logIndex: number;
  blockNumber: number;
  blockTime: string | null;
}

/**
 * 钱包 NFT DTO。
 *
 * 后端通过 NFT Transfer 事件维护 owner 缓存；metadata 字段预留给 worker 后续拉取
 * Token URI JSON 后展示名称、图片等扩展信息。
 */
export interface NftToken {
  nftAddress: string;
  tokenId: string;
  ownerAddress: string;
  tokenUri: string;
  metadataJson: string;
}

/**
 * 平台统计快照 DTO。
 *
 * 统计值来自后端 worker 汇总后的 MySQL 快照，避免首页每次打开都扫描全部拍卖和出价表。
 */
export interface PlatformStats {
  activeAuctionCount: number;
  endedAuctionCount: number;
  cancelledAuctionCount: number;
  totalBidCount: number;
  totalVolumeUsd: string;
  snapshotTime: string;
}

/** 通用分页返回结构，当前主要用于拍卖列表。 */
export interface PagedResult<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
}

/** “我的”页面一次性展示用户创建和参与过的拍卖。 */
export interface WalletAuctions {
  created: Auction[];
  participated: Auction[];
}
