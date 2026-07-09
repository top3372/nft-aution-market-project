import { useState } from "react";
import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { isAddress } from "ethers";
import { listWalletAuctions, listWalletNfts } from "../lib/api";
import { connectWallet } from "../lib/wallet";
import {
  getPaymentTokenBalance,
  isPaymentTokenOwner,
  listCreatedAuctionsOnChain,
  listOwnedAuctionNftsOnChain,
  mintPaymentTokenByOwner,
  signerFromProvider,
  type PaymentTokenBalance,
} from "../lib/contracts";
import { appConfig } from "../lib/config";
import { mergeAuctions, mergeNfts } from "../lib/nfts";
import { AuctionCard } from "../components/AuctionCard";
import type { Auction, NftToken } from "../lib/types";

// 管理员页面每次发放的测试 AUSD 数量。正式业务可改成输入框或后台审批额度。
const ADMIN_PAYMENT_TOKEN_MINT_AMOUNT = "1000";

/**
 * “我的”页面。
 *
 * 页面同时展示三类和钱包相关的数据：持有的 NFT、创建/参与的拍卖、支付代币余额。
 * 为了让刚发生的链上操作更快显示，NFT 和我创建的拍卖会合并后端索引数据与链上直读数据。
 */
export function MyPage() {
  const [address, setAddress] = useState("");
  const [chainNfts, setChainNfts] = useState<NftToken[]>([]);
  const [chainCreatedAuctions, setChainCreatedAuctions] = useState<Auction[]>([]);
  const [paymentTokenBalance, setPaymentTokenBalance] = useState<PaymentTokenBalance | null>(null);
  const [canMintPaymentToken, setCanMintPaymentToken] = useState(false);
  const [mintRecipientAddress, setMintRecipientAddress] = useState("");
  const [message, setMessage] = useState("");
  const [messageKind, setMessageKind] = useState<"info" | "error">("info");
  const [isConnecting, setIsConnecting] = useState(false);
  const [isMintingToken, setIsMintingToken] = useState(false);
  const nfts = useQuery({ queryKey: ["my-nfts", address], queryFn: () => listWalletNfts(address), enabled: Boolean(address) });
  const auctions = useQuery({ queryKey: ["my-auctions", address], queryFn: () => listWalletAuctions(address), enabled: Boolean(address) });
  // indexer 有确认区块延迟，链上直读结果用于补齐刚 mint 或刚创建拍卖后的短暂空窗。
  const visibleNfts = useMemo(() => mergeNfts(nfts.data ?? [], chainNfts), [nfts.data, chainNfts]);
  const createdAuctions = useMemo(
    () => mergeAuctions(auctions.data?.created ?? [], chainCreatedAuctions),
    [auctions.data?.created, chainCreatedAuctions],
  );

  /**
   * 连接钱包并一次性加载“我的”页面需要的链上数据。
   *
   * 四个读取并发执行：NFT 持仓、我创建的拍卖、AUSD 余额、AUSD 管理员权限。这样页面
   * 能同时判断用户是否可发币，以及展示最新链上资产状态。
   */
  async function connect() {
    setMessage("");
    setMessageKind("info");
    setIsConnecting(true);
    try {
      const wallet = await connectWallet();
      const signer = await signerFromProvider(wallet.provider);
      setAddress(wallet.address);
      const [ownedNfts, created, tokenBalance, tokenOwnerMatched] = await Promise.all([
        listOwnedAuctionNftsOnChain(signer, wallet.address),
        listCreatedAuctionsOnChain(signer, wallet.address),
        getPaymentTokenBalance(signer, wallet.address),
        isPaymentTokenOwner(signer, wallet.address),
      ]);
      setChainNfts(ownedNfts);
      setChainCreatedAuctions(created);
      setPaymentTokenBalance(tokenBalance);
      setCanMintPaymentToken(tokenOwnerMatched);
      setMintRecipientAddress((current) => current.trim() || wallet.address);
    } catch (err) {
      setMessageKind("error");
      setMessage(err instanceof Error ? err.message : "连接钱包失败");
    } finally {
      setIsConnecting(false);
    }
  }

  /**
   * 手动刷新 AUSD 余额。
   *
   * 用户收到管理员发币、或者刚完成出价后，余额变化来自链上 ERC20 合约；这里直接读链，
   * 不等待后端 indexer。
   */
  async function refreshPaymentTokenBalance() {
    if (!address) {
      return;
    }

    setMessage("");
    setMessageKind("info");
    try {
      const wallet = await connectWallet();
      const signer = await signerFromProvider(wallet.provider);
      setPaymentTokenBalance(await getPaymentTokenBalance(signer, wallet.address));
    } catch (err) {
      setMessageKind("error");
      setMessage(err instanceof Error ? err.message : "刷新支付代币余额失败");
    }
  }

  /**
   * 管理员向指定钱包发放 AUSD。
   *
   * 前端先校验收款地址格式，再校验当前钱包是否为 AuctionPaymentToken owner；
   * 链上合约仍会做最终权限校验。发给自己时顺手刷新余额，发给别人时只展示成功消息。
   */
  async function mintPaymentToken() {
    setMessage("");
    setMessageKind("info");

    const recipientAddress = mintRecipientAddress.trim();
    if (!isAddress(recipientAddress)) {
      setMessageKind("error");
      setMessage("请输入有效的收款钱包地址。");
      return;
    }

    setIsMintingToken(true);
    try {
      const wallet = await connectWallet();
      const signer = await signerFromProvider(wallet.provider);
      setAddress(wallet.address);
      const tokenOwnerMatched = await isPaymentTokenOwner(signer, wallet.address);
      setCanMintPaymentToken(tokenOwnerMatched);
      if (!tokenOwnerMatched) {
        setMessageKind("error");
        setMessage("当前钱包不是支付代币管理员，不能发放 AuctionPaymentToken。");
        return;
      }
      await mintPaymentTokenByOwner(signer, {
        to: recipientAddress,
        amount: ADMIN_PAYMENT_TOKEN_MINT_AMOUNT,
      });
      if (recipientAddress.toLowerCase() === wallet.address.toLowerCase()) {
        setPaymentTokenBalance(await getPaymentTokenBalance(signer, wallet.address));
      }
      setMessage(`已发放 ${ADMIN_PAYMENT_TOKEN_MINT_AMOUNT} AUSD 到 ${recipientAddress}。`);
    } catch (err) {
      setMessageKind("error");
      setMessage(err instanceof Error ? err.message : "发放支付代币失败");
    } finally {
      setIsMintingToken(false);
    }
  }

  return (
    <section className="page">
      <div className="page-head">
        <div>
          <h1>我的</h1>
          <p>查看我持有的 NFT、我创建和参与的拍卖。</p>
        </div>
        <button onClick={connect} disabled={isConnecting}>
          {isConnecting ? "连接中..." : address ? "刷新钱包数据" : "连接钱包"}
        </button>
      </div>
      {message && <p className={`notice ${messageKind === "error" ? "error" : ""}`}>{message}</p>}

      <div className="panel">
        <h2>支付代币</h2>
        <div className="asset-grid">
          <div>
            <span>当前余额</span>
            <strong>
              {paymentTokenBalance ? `${paymentTokenBalance.formatted} ${paymentTokenBalance.symbol}` : address ? "读取中..." : "未连接"}
            </strong>
          </div>
          <div>
            <span>代币合约</span>
            <strong className="address-text">{appConfig.paymentTokenAddress || "未配置"}</strong>
          </div>
          <div>
            <span>发币权限</span>
            <strong>{address ? canMintPaymentToken ? "当前钱包是管理员" : "当前钱包不是管理员" : "未连接"}</strong>
          </div>
        </div>
        <label className="field">
          <span>收款钱包地址</span>
          <input
            value={mintRecipientAddress}
            onChange={(event) => setMintRecipientAddress(event.target.value)}
            placeholder="0x..."
            disabled={!address || !canMintPaymentToken || isMintingToken}
          />
        </label>
        <div className="button-row">
          <button onClick={mintPaymentToken} disabled={!address || !canMintPaymentToken || isMintingToken || !mintRecipientAddress.trim()}>
            {isMintingToken ? "发放中..." : `管理员发放 ${ADMIN_PAYMENT_TOKEN_MINT_AMOUNT} AUSD`}
          </button>
          <button onClick={refreshPaymentTokenBalance} disabled={!address || isMintingToken}>
            刷新余额
          </button>
        </div>
      </div>

      <div className="panel">
        <h2>我的 NFT</h2>
        {address && visibleNfts.length === 0 && <div className="empty">暂无 NFT</div>}
        <div className="token-list">
          {visibleNfts.map((nft) => <span key={nft.tokenId}>Token #{nft.tokenId}</span>)}
        </div>
      </div>

      <div className="panel">
        <h2>我创建的拍卖</h2>
        <div className="auction-grid">
          {createdAuctions.map((auction) => <AuctionCard key={auction.auctionId} auction={auction} />)}
        </div>
      </div>

      <div className="panel">
        <h2>我参与的拍卖</h2>
        <div className="auction-grid">
          {auctions.data?.participated.map((auction) => <AuctionCard key={auction.auctionId} auction={auction} />)}
        </div>
      </div>
    </section>
  );
}
