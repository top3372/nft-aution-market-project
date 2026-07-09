import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getStats, listAuctions } from "../lib/api";
import { formatUsd8 } from "../lib/format";
import { AuctionCard } from "../components/AuctionCard";

/**
 * 拍卖市场首页。
 *
 * 首页只读后端索引后的拍卖列表和统计快照，不直接扫链。状态筛选会传给 Go API，
 * 由数据库查询返回当前页面需要的拍卖数据。
 */
export function HomePage() {
  const [status, setStatus] = useState("");
  const auctions = useQuery({
    queryKey: ["auctions", status],
    queryFn: () => listAuctions({ page: 1, pageSize: 24, status, sort: "end_time_asc" }),
  });
  const stats = useQuery({ queryKey: ["stats"], queryFn: getStats });

  return (
    <section className="page">
      <div className="page-head">
        <div>
          <h1>拍卖市场</h1>
          <p>查询链上拍卖、出价历史和当前状态。</p>
        </div>
        <select value={status} onChange={(event) => setStatus(event.target.value)}>
          <option value="">全部状态</option>
          <option value="pending">未开始</option>
          <option value="active">进行中</option>
          <option value="ended">已结束</option>
          <option value="cancelled">已取消</option>
        </select>
      </div>

      <div className="stats-row">
        <div><span>活跃拍卖</span><strong>{stats.data?.activeAuctionCount ?? 0}</strong></div>
        <div><span>已结束</span><strong>{stats.data?.endedAuctionCount ?? 0}</strong></div>
        <div><span>出价次数</span><strong>{stats.data?.totalBidCount ?? 0}</strong></div>
        <div><span>成交额</span><strong>{formatUsd8(stats.data?.totalVolumeUsd ?? "0")}</strong></div>
      </div>

      {auctions.isLoading && <div className="empty">加载拍卖列表...</div>}
      {auctions.error && <div className="empty error">拍卖列表加载失败</div>}
      {auctions.data?.items.length === 0 && <div className="empty">暂无拍卖</div>}
      <div className="auction-grid">
        {auctions.data?.items.map((auction) => (
          <AuctionCard auction={auction} key={auction.auctionId} />
        ))}
      </div>
    </section>
  );
}
