import { BrowserProvider, type Eip1193Provider } from "ethers";
import { appConfig } from "./config";

declare global {
  interface Window {
    ethereum?: Eip1193Provider;
  }
}

export interface ConnectedWallet {
  provider: BrowserProvider;
  address: string;
}

/**
 * 记录正在进行的钱包连接请求。
 *
 * MetaMask 同一 origin 同时只能有一个 `eth_requestAccounts`/权限弹窗。用户连续点击
 * “连接钱包”时复用同一个 Promise，避免触发 -32002 pending request 错误。
 */
let pendingConnection: Promise<ConnectedWallet> | null = null;

/**
 * 连接浏览器钱包并确认网络。
 *
 * 返回值只包含 ethers Provider 和当前账号地址。前端不会接触私钥，所有交易签名都
 * 由 MetaMask 等钱包插件完成。
 */
export function connectWallet(): Promise<ConnectedWallet> {
  const ethereum = window.ethereum;
  if (!ethereum) {
    throw new Error("未检测到 MetaMask 钱包");
  }

  if (pendingConnection) {
    return pendingConnection;
  }

  pendingConnection = requestWalletConnection(ethereum).catch((err: unknown) => {
    throw normalizeWalletError(err);
  }).finally(() => {
    pendingConnection = null;
  });

  return pendingConnection;
}

/**
 * 发起钱包授权并在授权后校验网络。
 *
 * 这里先拿账号再校验 Sepolia，是因为 MetaMask 的网络状态需要通过 Provider 读取；
 * 如果用户连错链，页面层会展示明确的 chainId 提示。
 */
async function requestWalletConnection(ethereum: Eip1193Provider): Promise<ConnectedWallet> {
  // 前端只请求用户钱包签名，不接触、不保存任何私钥。
  const provider = new BrowserProvider(ethereum);
  const accounts = (await provider.send("eth_requestAccounts", [])) as string[];
  await ensureSepolia(provider);
  return { provider, address: accounts[0] };
}

/**
 * 确保当前钱包网络和前端配置一致。
 *
 * 本项目部署在 Sepolia 时，拍卖合约、NFT 合约、AUSD 合约都来自同一条链；
 * 如果用户在主网或其他测试网操作，交易会打到错误网络，因此必须提前拦截。
 */
export async function ensureSepolia(provider: BrowserProvider): Promise<void> {
  const network = await provider.getNetwork();
  if (Number(network.chainId) !== appConfig.chainId) {
    throw new Error(`请切换到 ${appConfig.networkName} 网络，chainId=${appConfig.chainId}`);
  }
}

/**
 * 把钱包插件、ethers、浏览器包装出来的错误统一成人能理解的中文提示。
 *
 * ethers v6 有时会把 MetaMask 错误包在 `error/cause/data/info` 多层对象里，所以
 * 不能只读顶层 code/message；这里递归解析后再映射成业务页面可展示的文案。
 */
function normalizeWalletError(err: unknown): Error {
  const code = findKnownWalletErrorCode(err);
  const message = findWalletErrorMessage(err);
  if (code === -32002 || isPendingPermissionMessage(message)) {
    return new Error("MetaMask 已有连接请求待处理，请先在钱包弹窗中确认或取消。");
  }
  if (code === 4001 || isRejectedMessage(message)) {
    return new Error("已取消钱包连接。");
  }
  return err instanceof Error ? err : new Error("连接钱包失败");
}

/**
 * 从嵌套错误对象中提取 MetaMask 已知错误码。
 *
 * -32002 表示已有权限请求待处理，4001 表示用户拒绝。两类错误都需要页面给出明确
 * 提示，否则用户会误以为前端或合约调用失败。
 */
function findKnownWalletErrorCode(err: unknown): number | undefined {
  if (!isRecord(err)) {
    return undefined;
  }

  const code = err.code;
  if (code === -32002 || code === 4001) {
    return code;
  }

  return findKnownWalletErrorCode(err.error)
    ?? findKnownWalletErrorCode(err.cause)
    ?? findKnownWalletErrorCode(err.data)
    ?? findKnownWalletErrorCode(err.info);
}

/**
 * 递归拼接嵌套错误消息。
 *
 * 某些钱包错误没有稳定 code，但 message 中包含 `wallet_requestPermissions`
 * 或 `User rejected` 等关键字，拼接后可以覆盖更多浏览器/钱包包装形态。
 */
function findWalletErrorMessage(err: unknown): string {
  if (typeof err === "string") {
    return err;
  }
  if (err instanceof Error) {
    return err.message;
  }
  if (!isRecord(err)) {
    return "";
  }

  const message = typeof err.message === "string" ? err.message : "";
  const nestedMessage = [
    err.error,
    err.cause,
    err.data,
    err.info,
  ].map(findWalletErrorMessage).filter(Boolean).join(" ");

  return `${message} ${nestedMessage}`.trim();
}

/** 判断 MetaMask 是否已有待处理的钱包连接弹窗。 */
function isPendingPermissionMessage(message: string): boolean {
  return message.includes("wallet_requestPermissions") && message.includes("already pending");
}

/** 判断用户是否主动拒绝了钱包授权或签名。 */
function isRejectedMessage(message: string): boolean {
  return message.includes("User rejected") || message.includes("user rejected");
}

/** TypeScript 类型守卫，用于安全读取未知错误对象上的嵌套字段。 */
function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}
