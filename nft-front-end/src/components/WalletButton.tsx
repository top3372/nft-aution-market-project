import { useState } from "react";
import { Wallet } from "lucide-react";
import { connectWallet } from "../lib/wallet";
import { shortenAddress } from "../lib/format";

/**
 * 顶部钱包连接按钮。
 *
 * 这里只负责全局展示连接状态；具体业务页面会在需要签名、读取余额或发交易时再次调用
 * `connectWallet` 获取 Provider，确保用户切换账号后业务操作使用的是最新钱包。
 */
export function WalletButton() {
  const [address, setAddress] = useState("");
  const [error, setError] = useState("");
  const [isConnecting, setIsConnecting] = useState(false);

  /** 触发 MetaMask 连接并把连接错误展示在按钮下方。 */
  async function onConnect() {
    setError("");
    setIsConnecting(true);
    try {
      const wallet = await connectWallet();
      setAddress(wallet.address);
    } catch (err) {
      setError(err instanceof Error ? err.message : "连接失败");
    } finally {
      setIsConnecting(false);
    }
  }

  return (
    <div className="wallet-box">
      <button className="icon-button" onClick={onConnect} disabled={isConnecting} title="连接钱包">
        <Wallet size={18} />
        <span>{isConnecting ? "连接中..." : address ? shortenAddress(address) : "连接钱包"}</span>
      </button>
      {error && <div className="inline-error">{error}</div>}
    </div>
  );
}
