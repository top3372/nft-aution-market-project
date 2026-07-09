import type { AuctionStatus } from "../lib/types";

// 前端展示文案和后端状态枚举保持一一对应，避免页面散落状态翻译。
const labels: Record<AuctionStatus, string> = {
  pending: "未开始",
  active: "进行中",
  ended: "已结束",
  cancelled: "已取消",
};

/** 拍卖状态徽标，用于列表和详情页保持一致的状态表达。 */
export function StatusBadge({ status }: { status: AuctionStatus }) {
  return <span className={`status status-${status}`}>{labels[status]}</span>;
}
