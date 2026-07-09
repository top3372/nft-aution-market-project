import type { Bid } from "../lib/types";
import { formatDateTime, formatUsd8, shortenAddress } from "../lib/format";

/**
 * 出价历史表。
 *
 * 每一行对应链上 BidPlaced 事件，`txHash + logIndex` 是事件唯一位置。表格展示原始
 * ERC20 数量、折算 USD、交易哈希和事件区块时间，方便核对链上行为和后端索引结果。
 */
export function BidHistory({ bids }: { bids: Bid[] }) {
  if (bids.length === 0) {
    return <div className="empty">暂无出价记录</div>;
  }

  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th>出价人</th>
            <th>金额</th>
            <th>USD</th>
            <th>交易</th>
            <th>时间</th>
          </tr>
        </thead>
        <tbody>
          {bids.map((bid) => (
            <tr key={`${bid.txHash}-${bid.logIndex}`}>
              <td>{shortenAddress(bid.bidder)}</td>
              <td>{bid.amount}</td>
              <td>{formatUsd8(bid.amountUsd)}</td>
              <td>{shortenAddress(bid.txHash)}</td>
              <td>{formatDateTime(bid.blockTime)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
