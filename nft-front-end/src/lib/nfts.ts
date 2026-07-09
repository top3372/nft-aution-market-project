import type { Auction, NftToken } from "./types";

/**
 * 合并后端索引 NFT 和链上直读 NFT。
 *
 * indexer 写入 MySQL 需要等待确认区块，刚铸造的 NFT 可能还没同步到后端；链上直读
 * 能补上这个延迟窗口。以 tokenId 为业务主键去重，后出现的数据覆盖前面的数据。
 */
export function mergeNfts(indexedNfts: NftToken[], chainNfts: NftToken[]): NftToken[] {
  const byTokenId = new Map<string, NftToken>();
  for (const nft of [...indexedNfts, ...chainNfts]) {
    byTokenId.set(nft.tokenId, nft);
  }
  return [...byTokenId.values()].sort((left, right) => Number(left.tokenId) - Number(right.tokenId));
}

/**
 * 合并后端索引拍卖和链上直读拍卖。
 *
 * “我的”页面需要尽快看到刚创建的拍卖，因此会把 indexer 数据和链上 owner 查询结果
 * 放在一起展示；auctionId 是合约层业务主键，按它去重后再倒序显示最新拍卖。
 */
export function mergeAuctions(indexedAuctions: Auction[], chainAuctions: Auction[]): Auction[] {
  const byAuctionId = new Map<number, Auction>();
  for (const auction of [...indexedAuctions, ...chainAuctions]) {
    byAuctionId.set(auction.auctionId, auction);
  }
  return [...byAuctionId.values()].sort((left, right) => right.auctionId - left.auctionId);
}
