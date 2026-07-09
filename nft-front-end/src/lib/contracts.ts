import { BrowserProvider, Contract, formatUnits, parseUnits, type JsonRpcSigner, type TransactionReceipt } from "ethers";
import { appConfig, requireAddress } from "./config";
import type { Auction, AuctionStatus, NftToken } from "./types";
import auctionMarketAbi from "./abis/auctionMarketV3.json";
import auctionNftAbi from "./abis/auctionNft.json";
import erc20Abi from "./abis/erc20.json";

// CreateAuctionInput 是创建 V3 拍卖需要提交给合约的参数。
// 时间已经在页面层转成秒级时间戳，startingPriceUsd 仍是用户输入的美元字符串。
export interface CreateAuctionInput {
  tokenId: string;
  startTime: number;
  endTime: number;
  startingPriceUsd: string;
}

// BidInput 是用户在拍卖详情页输入的 ERC20 出价参数。
export interface BidInput {
  auctionId: string;
  amount: string;
}

// BidWithTokenResult 保存 approve 和 bid 两笔交易的回执。
// allowance 足够时不会发 approve，approvalReceipt 会是 null。
export interface BidWithTokenResult {
  approvalReceipt: TransactionReceipt | null;
  bidReceipt: TransactionReceipt | null;
}

// PaymentTokenBalance 是“我的”页面展示 AUSD 余额所需的数据。
// raw 保留链上最小单位，formatted 用于页面直接展示。
export interface PaymentTokenBalance {
  raw: bigint;
  decimals: number;
  symbol: string;
  formatted: string;
}

// MintPaymentTokenInput 是管理员发放 AUSD 的输入。
export interface MintPaymentTokenInput {
  to: string;
  amount: string;
}

// MintAuctionNftInput 是管理员铸造 AuctionNFT 的输入。
export interface MintAuctionNftInput {
  to: string;
  tokenUri: string;
}

// MintAuctionNftResult 返回本次预计铸造出的 tokenId 和交易回执。
export interface MintAuctionNftResult {
  tokenId: string;
  receipt: TransactionReceipt | null;
}

const MAX_CHAIN_NFT_SCAN = 500n;
const MAX_CHAIN_AUCTION_SCAN = 500n;

// signerFromProvider 从浏览器钱包 provider 取得当前账户 signer。
//
// 所有写链交易都必须通过 signer 发起，前端不会接触私钥。
export async function signerFromProvider(provider: BrowserProvider): Promise<JsonRpcSigner> {
  return provider.getSigner();
}

export function getMarketContract(signer: JsonRpcSigner): Contract {
  // 市场地址必须配置为 UUPS 代理地址，前端所有写交易都通过代理合约发起。
  return new Contract(requireAddress(appConfig.marketAddress, "VITE_MARKET_ADDRESS"), auctionMarketAbi, signer);
}

export function getAuctionNftContract(signer: JsonRpcSigner): Contract {
  // 本期只支持配置文件中的 AuctionNFT，暂不允许用户随意输入其他 ERC721 合约地址。
  return new Contract(requireAddress(appConfig.auctionNftAddress, "VITE_AUCTION_NFT_ADDRESS"), auctionNftAbi, signer);
}

export function getPaymentTokenContract(signer: JsonRpcSigner): Contract {
  // V3 拍卖只支持 ERC20 出价，PAYMENT_TOKEN_ADDRESS 需要和合约白名单保持一致。
  return new Contract(requireAddress(appConfig.paymentTokenAddress, "VITE_PAYMENT_TOKEN_ADDRESS"), erc20Abi, signer);
}

// approveNft 授权市场合约托管某个 NFT。
//
// 创建拍卖前必须先授权，否则 createAuctionV3 内部 safeTransferFrom 会失败。
export async function approveNft(signer: JsonRpcSigner, tokenId: string): Promise<TransactionReceipt | null> {
  const nft = getAuctionNftContract(signer);
  const tx = await nft.approve(appConfig.marketAddress, tokenId);
  return tx.wait();
}

// isAuctionNftOwner 判断当前钱包是否有 AuctionNFT 铸造权限。
//
// AuctionNFT.safeMint 受 onlyOwner 限制，所以创建测试 NFT 前必须做这个检查。
export async function isAuctionNftOwner(signer: JsonRpcSigner, walletAddress: string): Promise<boolean> {
  const nft = getAuctionNftContract(signer);
  const owner = await nft.owner();
  return owner.toLowerCase() === walletAddress.toLowerCase();
}

// mintAuctionNft 调用 AuctionNFT.safeMint 创建一个测试 NFT。
//
// Solidity 写交易返回值不会直接出现在前端交易响应中，因此发送交易前先读取 nextTokenId，
// 用它作为本次 mint 的 tokenId 展示给用户。
export async function mintAuctionNft(signer: JsonRpcSigner, input: MintAuctionNftInput): Promise<MintAuctionNftResult> {
  const nft = getAuctionNftContract(signer);
  // safeMint 返回值不会直接出现在交易响应里，发送交易前读取 nextTokenId 作为本次 tokenId。
  const tokenId = await nft.nextTokenId();
  const tx = await nft.safeMint(input.to, input.tokenUri);
  const receipt = await tx.wait();
  return { tokenId: tokenId.toString(), receipt };
}

// listOwnedAuctionNftsOnChain 从链上扫描当前钱包持有的 AuctionNFT。
//
// 后端已经通过 indexer 同步 nfts 表，但用户刚 mint 后可能还在等待确认数和轮询；
// 前端扫描 ownerOf/tokenURI 作为实时兜底。由于 AuctionNFT 没有 ERC721Enumerable，
// 这里只能从 0 扫到 nextTokenId，并设置 MAX_CHAIN_NFT_SCAN 防止页面卡住。
export async function listOwnedAuctionNftsOnChain(signer: JsonRpcSigner, walletAddress: string): Promise<NftToken[]> {
  const nft = getAuctionNftContract(signer);
  const nextTokenId = BigInt((await nft.nextTokenId()).toString());
  const scanLimit = nextTokenId > MAX_CHAIN_NFT_SCAN ? MAX_CHAIN_NFT_SCAN : nextTokenId;
  const tokens: NftToken[] = [];

  for (let tokenId = 0n; tokenId < scanLimit; tokenId++) {
    try {
      const owner = await nft.ownerOf(tokenId);
      if (owner.toLowerCase() !== walletAddress.toLowerCase()) {
        continue;
      }

      tokens.push({
        nftAddress: appConfig.auctionNftAddress,
        tokenId: tokenId.toString(),
        ownerAddress: owner,
        tokenUri: await nft.tokenURI(tokenId),
        metadataJson: "",
      });
    } catch {
      // ownerOf 对不存在或已销毁 token 会 revert；扫描模式下跳过即可。
    }
  }

  return tokens;
}

// listCreatedAuctionsOnChain 从链上读取当前钱包创建过的拍卖。
//
// 后端列表依赖 indexer，同步存在延迟；这里用于“我的”页面在交易刚确认时做兜底展示。
export async function listCreatedAuctionsOnChain(signer: JsonRpcSigner, walletAddress: string): Promise<Auction[]> {
  const market = getMarketContract(signer);
  const auctionCount = BigInt((await market.auctionCount()).toString());
  const scanLimit = auctionCount > MAX_CHAIN_AUCTION_SCAN ? MAX_CHAIN_AUCTION_SCAN : auctionCount;
  const auctions: Auction[] = [];

  for (let auctionId = 0n; auctionId < scanLimit; auctionId++) {
    const row = await market.auctions(auctionId);
    if (row.seller.toLowerCase() !== walletAddress.toLowerCase()) {
      continue;
    }

    const extra = await market.auctionV3Extras(auctionId);
    auctions.push({
      auctionId: Number(auctionId),
      seller: row.seller,
      nftAddress: row.nft,
      tokenId: row.tokenId.toString(),
      startTime: Number(extra.startTime) === 0 ? null : unixSecondsToISOString(extra.startTime),
      endTime: unixSecondsToISOString(row.endTime),
      startingPriceUsd: extra.startingPriceUsd.toString(),
      status: auctionStatus(Boolean(row.ended), Boolean(extra.cancelled), extra.startTime, row.endTime),
      highestBidder: zeroAddressToEmpty(row.highestBidder),
      highestBid: row.highestBid.toString(),
      highestBidUsd: row.highestBidUsd.toString(),
    });
  }

  return auctions;
}

// auctionStatus 根据链上 ended/cancelled 和当前时间计算展示状态。
function auctionStatus(ended: boolean, cancelled: boolean, startTime: bigint, endTime: bigint): AuctionStatus {
  const now = BigInt(Math.floor(Date.now() / 1000));
  if (cancelled) {
    return "cancelled";
  }
  if (ended || now >= endTime) {
    return "ended";
  }
  if (startTime > now) {
    return "pending";
  }
  return "active";
}

// unixSecondsToISOString 把合约秒级时间戳转成 API/组件统一使用的 ISO 字符串。
function unixSecondsToISOString(value: bigint): string {
  return new Date(Number(value) * 1000).toISOString();
}

// zeroAddressToEmpty 把合约里的 address(0) 转成空字符串，方便页面展示“无人出价”。
function zeroAddressToEmpty(value: string): string {
  return value === "0x0000000000000000000000000000000000000000" ? "" : value;
}

// createAuction 调用 AuctionMarketV3.createAuctionV3 创建拍卖。
//
// 合约内部会把 NFT 从卖家钱包转入市场托管，并触发 AuctionCreatedV3 供后端 indexer 同步。
export async function createAuction(signer: JsonRpcSigner, input: CreateAuctionInput): Promise<TransactionReceipt | null> {
  const market = getMarketContract(signer);
  // 合约使用 8 位 USD 精度，前端输入美元数后转换成整数传入。
  const startingPriceUsd = parseUnits(input.startingPriceUsd, 8);
  const tx = await market.createAuctionV3(input.tokenId, input.startTime, input.endTime, startingPriceUsd);
  return tx.wait();
}

// approvePaymentToken 授权市场合约扣除用户的 AUSD。
//
// ERC20 出价依赖 transferFrom，所以用户第一次出价或 allowance 不足时必须先 approve。
export async function approvePaymentToken(signer: JsonRpcSigner, amount: bigint): Promise<TransactionReceipt | null> {
  const token = getPaymentTokenContract(signer);
  // approve 和 bid 是两笔交易，UI 需要分别展示 pending 状态。
  const tx = await token.approve(appConfig.marketAddress, amount);
  return tx.wait();
}

// getPaymentTokenBalance 读取当前钱包 AUSD 余额。
//
// 余额属于资金类实时数据，页面直接读链上 balanceOf；后端 payment_token source 当前只做事件审计。
export async function getPaymentTokenBalance(signer: JsonRpcSigner, walletAddress: string): Promise<PaymentTokenBalance> {
  const token = getPaymentTokenContract(signer);
  const [raw, decimals, symbol] = await Promise.all([
    token.balanceOf(walletAddress),
    token.decimals(),
    token.symbol(),
  ]);
  const numericDecimals = Number(decimals);
  const rawBalance = BigInt(raw.toString());

  return {
    raw: rawBalance,
    decimals: numericDecimals,
    symbol,
    formatted: formatUnits(rawBalance, numericDecimals),
  };
}

// isPaymentTokenOwner 判断当前钱包是否为 AUSD 管理员。
//
// 只有 owner 可以调用 AuctionPaymentToken.mint，普通用户只能通过管理员发放或转账获得 AUSD。
export async function isPaymentTokenOwner(signer: JsonRpcSigner, walletAddress: string): Promise<boolean> {
  const token = getPaymentTokenContract(signer);
  const owner = await token.owner();
  return owner.toLowerCase() === walletAddress.toLowerCase();
}

// mintPaymentTokenByOwner 由管理员给指定地址发放 AUSD。
//
// input.amount 是页面输入的完整代币数量，函数内部按 decimals 转成 ERC20 最小单位。
export async function mintPaymentTokenByOwner(
  signer: JsonRpcSigner,
  input: MintPaymentTokenInput,
): Promise<TransactionReceipt | null> {
  const token = getPaymentTokenContract(signer);
  // AuctionPaymentToken 的 mint 受 onlyOwner 限制，普通用户不能通过页面自行增发。
  const decimals = Number(await token.decimals());
  const amount = parseUnits(input.amount, decimals);
  const tx = await token.mint(input.to, amount);
  return tx.wait();
}

// bidWithToken 完成 ERC20 出价流程。
//
// 业务上可能需要两笔交易：
// 1. allowance 不足时先 approve 市场合约。
// 2. 再调用 bidWithToken 提交正式出价。
// 如果 allowance 已足够，则只发送出价交易。
export async function bidWithToken(signer: JsonRpcSigner, input: BidInput): Promise<BidWithTokenResult> {
  const market = getMarketContract(signer);
  const token = getPaymentTokenContract(signer);
  const decimals = Number(await token.decimals());
  const amount = parseUnits(input.amount, decimals);
  const owner = await signer.getAddress();
  const allowance = BigInt((await token.allowance(owner, appConfig.marketAddress)).toString());
  let approvalReceipt: TransactionReceipt | null = null;

  if (allowance < amount) {
    approvalReceipt = await approvePaymentToken(signer, amount);
  }

  const tx = await market.bidWithToken(input.auctionId, appConfig.paymentTokenAddress, amount);
  return {
    approvalReceipt,
    bidReceipt: await tx.wait(),
  };
}

// cancelAuction 调用市场合约取消无人出价的 V3 拍卖。
//
// 合约会校验调用者必须是卖家，且当前拍卖不能已有出价。
export async function cancelAuction(signer: JsonRpcSigner, auctionId: string): Promise<TransactionReceipt | null> {
  const market = getMarketContract(signer);
  const tx = await market.cancelAuction(auctionId);
  return tx.wait();
}

// endAuction 调用市场合约结束拍卖。
//
// 合约会根据是否有最高出价决定 NFT 归属，并按 V2/V3 规则结算资金和手续费。
export async function endAuction(signer: JsonRpcSigner, auctionId: string): Promise<TransactionReceipt | null> {
  const market = getMarketContract(signer);
  const tx = await market.endAuction(auctionId);
  return tx.wait();
}
