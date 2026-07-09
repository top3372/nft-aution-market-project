import { useState } from "react";
import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { listWalletNfts } from "../lib/api";
import {
  approveNft,
  createAuction,
  isAuctionNftOwner,
  listOwnedAuctionNftsOnChain,
  mintAuctionNft,
  signerFromProvider,
} from "../lib/contracts";
import { connectWallet } from "../lib/wallet";
import type { NftToken } from "../lib/types";
import { mergeNfts } from "../lib/nfts";

/**
 * 创建拍卖页面。
 *
 * 业务流程分三段：
 * 1. 连接钱包后读取当前用户持有的 AuctionNFT；
 * 2. 如果当前钱包是 AuctionNFT owner，可以先铸造测试 NFT；
 * 3. 选择 NFT 后先 approve 给拍卖市场，再调用市场合约创建拍卖。
 */
export function CreateAuctionPage() {
  const [walletAddress, setWalletAddress] = useState("");
  const [tokenId, setTokenId] = useState("");
  const [tokenUri, setTokenUri] = useState("");
  const [chainNfts, setChainNfts] = useState<NftToken[]>([]);
  const [startTime, setStartTime] = useState("");
  const [endTime, setEndTime] = useState("");
  const [startingPriceUsd, setStartingPriceUsd] = useState("");
  const [message, setMessage] = useState("");
  const [isConnecting, setIsConnecting] = useState(false);
  const [isMinting, setIsMinting] = useState(false);
  const [canMint, setCanMint] = useState<boolean | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const nfts = useQuery({
    queryKey: ["wallet-nfts", walletAddress],
    queryFn: () => listWalletNfts(walletAddress),
    enabled: Boolean(walletAddress),
  });
  // 后端索引数据可能落后链上几秒到几个区块，因此下拉框合并 MySQL 数据和链上直读数据。
  const selectableNfts = useMemo(() => mergeNfts(nfts.data ?? [], chainNfts), [nfts.data, chainNfts]);

  /** 连接钱包并加载可用于创建拍卖的 NFT。 */
  async function connect() {
    setMessage("");
    setIsConnecting(true);
    try {
      const wallet = await connectWallet();
      const signer = await signerFromProvider(wallet.provider);
      await loadWalletNfts(signer, wallet.address);
    } catch (err) {
      setMessage(err instanceof Error ? err.message : "连接钱包失败");
    } finally {
      setIsConnecting(false);
    }
  }

  /**
   * 读取钱包链上状态。
   *
   * `isAuctionNftOwner` 决定是否显示 NFT 铸造能力；`listOwnedAuctionNftsOnChain`
   * 用来补齐 indexer 未同步的新 NFT，避免用户刚 mint 后页面看不到。
   */
  async function loadWalletNfts(signer: Awaited<ReturnType<typeof signerFromProvider>>, address: string) {
    setWalletAddress(address);
    const [ownerMatched, ownedTokens] = await Promise.all([
      isAuctionNftOwner(signer, address),
      listOwnedAuctionNftsOnChain(signer, address),
    ]);
    setCanMint(ownerMatched);
    setChainNfts(ownedTokens);
    if (!tokenId && ownedTokens.length > 0) {
      setTokenId(ownedTokens[0].tokenId);
    }
  }

  /**
   * 管理员铸造一个 AuctionNFT。
   *
   * 合约限制只有 AuctionNFT owner 能 mint；前端先做 owner 校验并给出中文提示，真正
   * 权限仍由链上 `onlyOwner` 保证。交易确认后立即重读链上 NFT 和后端索引数据。
   */
  async function mintNft() {
    setMessage("");
    setIsMinting(true);
    try {
      const wallet = await connectWallet();
      const signer = await signerFromProvider(wallet.provider);
      const ownerMatched = await isAuctionNftOwner(signer, wallet.address);
      setCanMint(ownerMatched);
      if (!ownerMatched) {
        setMessage("当前钱包不是 AuctionNFT 合约 owner，不能铸造 NFT。请切换到部署/管理员钱包。");
        return;
      }

      setMessage("等待钱包确认 NFT 铸造...");
      const result = await mintAuctionNft(signer, { to: wallet.address, tokenUri });
      setTokenId(result.tokenId);
      await loadWalletNfts(signer, wallet.address);
      await nfts.refetch();
      setMessage(`NFT 已创建，Token #${result.tokenId} 可用于创建拍卖。`);
    } catch (err) {
      setMessage(err instanceof Error ? err.message : "创建 NFT 失败");
    } finally {
      setIsMinting(false);
    }
  }

  /**
   * 授权 NFT 并创建拍卖。
   *
   * ERC721 必须先 approve 给市场合约，市场合约才能在 createAuction 时把 NFT 托管到
   * 合约内。创建成功后 indexer 会监听 AuctionCreated 事件并写入 MySQL。
   */
  async function submit() {
    setIsSubmitting(true);
    try {
      setMessage("等待钱包签名...");
      const wallet = await connectWallet();
      const signer = await signerFromProvider(wallet.provider);
      await approveNft(signer, tokenId);
      setMessage("NFT 已授权，创建拍卖中...");
      await createAuction(signer, {
        tokenId,
        startTime: Math.floor(new Date(startTime).getTime() / 1000),
        endTime: Math.floor(new Date(endTime).getTime() / 1000),
        startingPriceUsd,
      });
      setMessage("链上已确认，等待 indexer 同步。");
    } catch (err) {
      setMessage(err instanceof Error ? err.message : "创建拍卖失败");
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <section className="page narrow">
      <div className="page-head">
        <div>
          <h1>创建拍卖</h1>
          <p>先创建或读取当前 AuctionNFT，再设置拍卖时间和起拍价。</p>
        </div>
        <button onClick={connect} disabled={isConnecting}>{isConnecting ? "连接中..." : "读取我的 NFT"}</button>
      </div>

      <div className="panel">
        <h2>创建 NFT</h2>
        <label>Token URI</label>
        <input value={tokenUri} onChange={(event) => setTokenUri(event.target.value)} placeholder="ipfs://metadata.json 或 https://metadata.json" />
        <button onClick={mintNft} disabled={isMinting || !tokenUri}>
          {isMinting ? "创建中..." : "创建 NFT"}
        </button>
        {canMint === false && <p className="inline-error">当前钱包不是 AuctionNFT 合约 owner，不能铸造 NFT。</p>}
      </div>

      <label>Token ID</label>
      <select value={tokenId} onChange={(event) => setTokenId(event.target.value)}>
        <option value="">选择 NFT</option>
        {selectableNfts.map((nft) => (
          <option value={nft.tokenId} key={nft.tokenId}>
            Token #{nft.tokenId}
          </option>
        ))}
      </select>
      <label>开始时间</label>
      <input type="datetime-local" value={startTime} onChange={(event) => setStartTime(event.target.value)} />
      <label>结束时间</label>
      <input type="datetime-local" value={endTime} onChange={(event) => setEndTime(event.target.value)} />
      <label>起拍价 USD</label>
      <input value={startingPriceUsd} onChange={(event) => setStartingPriceUsd(event.target.value)} placeholder="100" />
      <button onClick={submit} disabled={isSubmitting || !tokenId || !startTime || !endTime || !startingPriceUsd}>
        {isSubmitting ? "处理中..." : "授权并创建"}
      </button>
      {message && <p className="notice">{message}</p>}
    </section>
  );
}
