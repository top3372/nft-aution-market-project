/**
 * 前端运行时配置。
 *
 * 所有合约地址、链 ID、后端 API 地址都从 Vite 环境变量读取，便于本地、Sepolia
 * 和未来主网部署分别维护配置，而不用改业务代码。
 */
export const appConfig = {
  apiBaseUrl: import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080",
  networkName: import.meta.env.VITE_NETWORK_NAME ?? "sepolia",
  chainId: Number(import.meta.env.VITE_CHAIN_ID ?? "11155111"),
  rpcUrl: import.meta.env.VITE_RPC_URL ?? "",
  marketAddress: import.meta.env.VITE_MARKET_ADDRESS ?? "",
  auctionNftAddress: import.meta.env.VITE_AUCTION_NFT_ADDRESS ?? "",
  paymentTokenAddress: import.meta.env.VITE_PAYMENT_TOKEN_ADDRESS ?? "",
  blockExplorerUrl: trimTrailingSlash(import.meta.env.VITE_BLOCK_EXPLORER_URL ?? "https://sepolia.etherscan.io"),
};

/**
 * 读取合约地址时的显式校验。
 *
 * 合约交互函数调用前先检查配置，能把“没有配置 AUCTION_MARKET_PROXY / NFT / AUSD”
 * 转换成清晰错误，而不是让 ethers 对空地址报晦涩异常。
 */
export function requireAddress(value: string, name: string): string {
  if (!value) {
    throw new Error(`${name} is not configured`);
  }
  return value;
}

/**
 * 规范区块浏览器根地址。
 *
 * 统一去掉尾部斜杠后，后续拼接 `/tx/<hash>`、`/address/<addr>` 时不会出现双斜杠。
 */
function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, "");
}
