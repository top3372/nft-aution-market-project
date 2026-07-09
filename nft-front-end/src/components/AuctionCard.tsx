import { Link } from "react-router-dom";
import type { Auction } from "../lib/types";
import { formatDateTime, formatUsd8, shortenAddress } from "../lib/format";
import { StatusBadge } from "./StatusBadge";

/**
 * 拍卖卡片。
 *
 * 用于首页、“我的创建”和“我的参与”列表，展示足够的业务摘要：NFT、卖家、起拍价、
 * 当前最高价和结束时间。点击后进入详情页处理出价、取消、结束等操作。
 */
export function AuctionCard({ auction }: { auction: Auction }) {
  return (
    <Link className="auction-card" to={`/auctions/${auction.auctionId}`}>
      <div className="card-head">
        <strong>Token #{auction.tokenId}</strong>
        <StatusBadge status={auction.status} />
      </div>
      <dl className="fact-grid">
        <div>
          <dt>卖家</dt>
          <dd>{shortenAddress(auction.seller)}</dd>
        </div>
        <div>
          <dt>起拍价</dt>
          <dd>{formatUsd8(auction.startingPriceUsd)}</dd>
        </div>
        <div>
          <dt>最高价</dt>
          <dd>{formatUsd8(auction.highestBidUsd)}</dd>
        </div>
        <div>
          <dt>结束</dt>
          <dd>{formatDateTime(auction.endTime)}</dd>
        </div>
      </dl>
    </Link>
  );
}
