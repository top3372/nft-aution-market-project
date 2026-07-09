/**
 * 格式化 USD 8 位精度金额。
 *
 * 合约和后端都按 Chainlink 常见的 8 decimals 保存 USD 值，例如 100 USD 会保存为
 * `10000000000`。这里使用 BigInt 避免大金额精度丢失，再裁掉末尾无意义的 0。
 */
export function formatUsd8(value: string): string {
  const raw = BigInt(value || "0");
  const whole = raw / 100000000n;
  const fraction = raw % 100000000n;
  const trimmed = fraction.toString().padStart(8, "0").replace(/0+$/, "");
  return trimmed ? `$${whole}.${trimmed}` : `$${whole}`;
}

/**
 * 缩短钱包、合约和交易哈希地址。
 *
 * 地址本身仍然保留在数据层，页面只做展示压缩，避免列表和表格被长地址撑开。
 */
export function shortenAddress(address: string): string {
  if (!address) {
    return "-";
  }
  return `${address.slice(0, 6)}...${address.slice(-4)}`;
}

/**
 * 格式化后端返回的区块时间或拍卖时间。
 *
 * null 表示 indexer 尚未拿到区块时间或链上没有该时间点，页面统一展示 `-`。
 */
export function formatDateTime(value: string | null): string {
  if (!value) {
    return "-";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "short",
    timeStyle: "medium",
  }).format(new Date(value));
}
