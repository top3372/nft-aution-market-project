import { useState } from "react";
import { useParams } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { getAuction, listBids } from "../lib/api";
import { cancelAuction, bidWithToken, endAuction, signerFromProvider } from "../lib/contracts";
import { connectWallet } from "../lib/wallet";
import { formatDateTime, formatUsd8, shortenAddress } from "../lib/format";
import { StatusBadge } from "../components/StatusBadge";
import { BidHistory } from "../components/BidHistory";

/**
 * 拍卖详情页。
 *
 * 详情和出价历史来自后端索引库；出价、取消、结束是直接发链上交易。交易确认后重新
 * 失效 React Query 缓存，让页面等待 indexer 同步最新事件。
 */
export function AuctionDetailPage() {
  const { auctionId = "" } = useParams();
  const queryClient = useQueryClient();
  const [bidAmount, setBidAmount] = useState("");
  const [message, setMessage] = useState("");
  const [pendingAction, setPendingAction] = useState<"bid" | "cancel" | "end" | "">("");
  const auction = useQuery({ queryKey: ["auction", auctionId], queryFn: () => getAuction(auctionId) });
  const bids = useQuery({ queryKey: ["bids", auctionId], queryFn: () => listBids(auctionId) });

  /**
   * 执行详情页上的链上交易。
   *
   * `bid` 会先在合约交互层 approve AUSD 再出价；`cancel` 只能由卖家在未成交时取消；
   * `end` 会触发市场合约结算 NFT 和最高出价。按钮共用 pendingAction 防止重复提交。
   */
  async function runTx(action: "bid" | "cancel" | "end") {
    setPendingAction(action);
    try {
      setMessage("等待钱包签名...");
      const wallet = await connectWallet();
      const signer = await signerFromProvider(wallet.provider);
      if (action === "bid") {
        await bidWithToken(signer, { auctionId, amount: bidAmount });
      } else if (action === "cancel") {
        await cancelAuction(signer, auctionId);
      } else {
        await endAuction(signer, auctionId);
      }
      setMessage("链上已确认，等待索引同步...");
      await queryClient.invalidateQueries({ queryKey: ["auction", auctionId] });
      await queryClient.invalidateQueries({ queryKey: ["bids", auctionId] });
    } catch (err) {
      setMessage(err instanceof Error ? err.message : "交易提交失败");
    } finally {
      setPendingAction("");
    }
  }

  if (auction.isLoading) {
    return <div className="empty">加载拍卖详情...</div>;
  }
  if (!auction.data) {
    return <div className="empty error">拍卖不存在或 API 未同步</div>;
  }

  return (
    <section className="page detail">
      <div className="page-head">
        <div>
          <h1>Token #{auction.data.tokenId}</h1>
          <p>{shortenAddress(auction.data.nftAddress)}</p>
        </div>
        <StatusBadge status={auction.data.status} />
      </div>

      <div className="detail-grid">
        <div className="panel">
          <h2>拍卖信息</h2>
          <dl className="fact-grid">
            <div><dt>卖家</dt><dd>{shortenAddress(auction.data.seller)}</dd></div>
            <div><dt>开始</dt><dd>{formatDateTime(auction.data.startTime)}</dd></div>
            <div><dt>结束</dt><dd>{formatDateTime(auction.data.endTime)}</dd></div>
            <div><dt>起拍价</dt><dd>{formatUsd8(auction.data.startingPriceUsd)}</dd></div>
            <div><dt>最高价</dt><dd>{formatUsd8(auction.data.highestBidUsd)}</dd></div>
            <div><dt>最高价者</dt><dd>{shortenAddress(auction.data.highestBidder)}</dd></div>
          </dl>
        </div>

        <div className="panel">
          <h2>操作</h2>
          <input value={bidAmount} onChange={(event) => setBidAmount(event.target.value)} placeholder="ERC20 出价数量" />
          <div className="button-row">
            <button onClick={() => runTx("bid")} disabled={Boolean(pendingAction)}>
              {pendingAction === "bid" ? "处理中..." : "出价"}
            </button>
            <button onClick={() => runTx("cancel")} disabled={Boolean(pendingAction)}>
              {pendingAction === "cancel" ? "处理中..." : "取消"}
            </button>
            <button onClick={() => runTx("end")} disabled={Boolean(pendingAction)}>
              {pendingAction === "end" ? "处理中..." : "结束"}
            </button>
          </div>
          {message && <p className="notice">{message}</p>}
        </div>
      </div>

      <div className="panel">
        <h2>出价历史</h2>
        <BidHistory bids={bids.data ?? []} />
      </div>
    </section>
  );
}
