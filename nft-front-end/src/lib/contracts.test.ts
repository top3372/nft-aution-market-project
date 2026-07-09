import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => {
  const nftContract = {
    nextTokenId: vi.fn(),
    owner: vi.fn(),
    ownerOf: vi.fn(),
    safeMint: vi.fn(),
    tokenURI: vi.fn(),
  };
  const marketContract = {
    auctionCount: vi.fn(),
    auctions: vi.fn(),
    auctionV3Extras: vi.fn(),
    bidWithToken: vi.fn(),
  };
  const paymentTokenContract = {
    allowance: vi.fn(),
    approve: vi.fn(),
    balanceOf: vi.fn(),
    decimals: vi.fn(),
    mint: vi.fn(),
    owner: vi.fn(),
    symbol: vi.fn(),
  };

  return {
    Contract: vi.fn(function Contract(address: string) {
      if (address === "0xBA9af325234368184A61be6081cdFB7f02dc6405") {
        return marketContract;
      }
      if (address === "0xBE3c38a1015b4B4dacfd13C5346F1Ee907D8c283") {
        return paymentTokenContract;
      }
      return nftContract;
    }),
    marketContract,
    nftContract,
    paymentTokenContract,
  };
});

vi.mock("ethers", () => ({
  Contract: mocks.Contract,
  formatUnits: vi.fn((value: bigint, decimals: number) => `${value.toString()}/${decimals}`),
  parseUnits: vi.fn((value: string) => BigInt(value)),
}));

vi.mock("./config", () => ({
  appConfig: {
    auctionNftAddress: "0x7913Ff1EaA12887ed80a3D35c81c0033FFafadCe",
    marketAddress: "0xBA9af325234368184A61be6081cdFB7f02dc6405",
    paymentTokenAddress: "0xBE3c38a1015b4B4dacfd13C5346F1Ee907D8c283",
  },
  requireAddress: (value: string) => value,
}));

import {
  bidWithToken,
  getPaymentTokenBalance,
  isAuctionNftOwner,
  isPaymentTokenOwner,
  listCreatedAuctionsOnChain,
  listOwnedAuctionNftsOnChain,
  mintPaymentTokenByOwner,
  mintAuctionNft,
} from "./contracts";

describe("AuctionNFT contract helpers", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("铸造 NFT 前读取 nextTokenId，并把新 NFT 铸造给当前钱包", async () => {
    const wait = vi.fn().mockResolvedValue({ hash: "0xmint" });
    mocks.nftContract.nextTokenId.mockResolvedValue(7n);
    mocks.nftContract.safeMint.mockResolvedValue({ wait });

    const result = await mintAuctionNft({} as never, {
      to: "0x1111111111111111111111111111111111111111",
      tokenUri: "ipfs://auction-demo",
    });

    expect(mocks.nftContract.nextTokenId).toHaveBeenCalledTimes(1);
    expect(mocks.nftContract.safeMint).toHaveBeenCalledWith("0x1111111111111111111111111111111111111111", "ipfs://auction-demo");
    expect(wait).toHaveBeenCalledTimes(1);
    expect(result).toEqual({ tokenId: "7", receipt: { hash: "0xmint" } });
  });

  it("判断当前钱包是否为 AuctionNFT 管理员", async () => {
    mocks.nftContract.owner.mockResolvedValue("0x1111111111111111111111111111111111111111");

    const result = await isAuctionNftOwner({} as never, "0x1111111111111111111111111111111111111111");

    expect(result).toBe(true);
  });

  it("从链上扫描当前钱包持有的 AuctionNFT", async () => {
    mocks.nftContract.nextTokenId.mockResolvedValue(3n);
    mocks.nftContract.ownerOf.mockImplementation((tokenId: bigint) => {
      if (tokenId === 0n) {
        return Promise.resolve("0x1111111111111111111111111111111111111111");
      }
      if (tokenId === 1n) {
        return Promise.resolve("0x2222222222222222222222222222222222222222");
      }
      return Promise.resolve("0x1111111111111111111111111111111111111111");
    });
    mocks.nftContract.tokenURI.mockImplementation((tokenId: bigint) => Promise.resolve(`ipfs://token-${tokenId}`));

    const result = await listOwnedAuctionNftsOnChain({} as never, "0x1111111111111111111111111111111111111111");

    expect(result).toEqual([
      {
        nftAddress: "0x7913Ff1EaA12887ed80a3D35c81c0033FFafadCe",
        tokenId: "0",
        ownerAddress: "0x1111111111111111111111111111111111111111",
        tokenUri: "ipfs://token-0",
        metadataJson: "",
      },
      {
        nftAddress: "0x7913Ff1EaA12887ed80a3D35c81c0033FFafadCe",
        tokenId: "2",
        ownerAddress: "0x1111111111111111111111111111111111111111",
        tokenUri: "ipfs://token-2",
        metadataJson: "",
      },
    ]);
  });

  it("从链上扫描当前钱包创建的拍卖", async () => {
    mocks.marketContract.auctionCount.mockResolvedValue(2n);
    mocks.marketContract.auctions.mockImplementation((auctionId: bigint) => {
      if (auctionId === 0n) {
        return Promise.resolve({
          seller: "0x1111111111111111111111111111111111111111",
          nft: "0x7913Ff1EaA12887ed80a3D35c81c0033FFafadCe",
          tokenId: 1n,
          endTime: 1783844580n,
          ended: false,
          highestBidder: "0x0000000000000000000000000000000000000000",
          paymentToken: "0x0000000000000000000000000000000000000000",
          highestBid: 0n,
          highestBidUsd: 0n,
        });
      }
      return Promise.resolve({
        seller: "0x2222222222222222222222222222222222222222",
        nft: "0x7913Ff1EaA12887ed80a3D35c81c0033FFafadCe",
        tokenId: 2n,
        endTime: 1783844580n,
        ended: false,
        highestBidder: "0x0000000000000000000000000000000000000000",
        paymentToken: "0x0000000000000000000000000000000000000000",
        highestBid: 0n,
        highestBidUsd: 0n,
      });
    });
    mocks.marketContract.auctionV3Extras.mockResolvedValue({
      startTime: 1783758180n,
      startingPriceUsd: 10000000000n,
      cancelled: false,
      v3Created: true,
    });

    const result = await listCreatedAuctionsOnChain({} as never, "0x1111111111111111111111111111111111111111");

    expect(result).toEqual([
      {
        auctionId: 0,
        seller: "0x1111111111111111111111111111111111111111",
        nftAddress: "0x7913Ff1EaA12887ed80a3D35c81c0033FFafadCe",
        tokenId: "1",
        startTime: "2026-07-11T08:23:00.000Z",
        endTime: "2026-07-12T08:23:00.000Z",
        startingPriceUsd: "10000000000",
        status: "pending",
        highestBidder: "",
        highestBid: "0",
        highestBidUsd: "0",
      },
    ]);
  });

  it("ERC20 授权不足时，出价前先 approve 市场合约", async () => {
    const approveWait = vi.fn().mockResolvedValue({ hash: "0xapprove" });
    const bidWait = vi.fn().mockResolvedValue({ hash: "0xbid" });
    const signer = {
      getAddress: vi.fn().mockResolvedValue("0x1111111111111111111111111111111111111111"),
    };
    mocks.paymentTokenContract.decimals.mockResolvedValue(6);
    mocks.paymentTokenContract.allowance.mockResolvedValue(0n);
    mocks.paymentTokenContract.approve.mockResolvedValue({ wait: approveWait });
    mocks.marketContract.bidWithToken.mockResolvedValue({ wait: bidWait });

    const result = await bidWithToken(signer as never, { auctionId: "1", amount: "100" });

    expect(mocks.paymentTokenContract.allowance).toHaveBeenCalledWith(
      "0x1111111111111111111111111111111111111111",
      "0xBA9af325234368184A61be6081cdFB7f02dc6405",
    );
    expect(mocks.paymentTokenContract.approve).toHaveBeenCalledWith(
      "0xBA9af325234368184A61be6081cdFB7f02dc6405",
      100n,
    );
    expect(mocks.marketContract.bidWithToken).toHaveBeenCalledWith(
      "1",
      "0xBE3c38a1015b4B4dacfd13C5346F1Ee907D8c283",
      100n,
    );
    expect(result).toEqual({
      approvalReceipt: { hash: "0xapprove" },
      bidReceipt: { hash: "0xbid" },
    });
  });

  it("读取当前钱包的 ERC20 支付代币余额并按 decimals 格式化", async () => {
    mocks.paymentTokenContract.decimals.mockResolvedValue(6);
    mocks.paymentTokenContract.symbol.mockResolvedValue("AUSD");
    mocks.paymentTokenContract.balanceOf.mockResolvedValue(123456000n);

    const result = await getPaymentTokenBalance({} as never, "0x1111111111111111111111111111111111111111");

    expect(mocks.paymentTokenContract.balanceOf).toHaveBeenCalledWith("0x1111111111111111111111111111111111111111");
    expect(result).toEqual({
      raw: 123456000n,
      decimals: 6,
      symbol: "AUSD",
      formatted: "123456000/6",
    });
  });

  it("判断当前钱包是否为支付代币管理员", async () => {
    mocks.paymentTokenContract.owner.mockResolvedValue("0x1111111111111111111111111111111111111111");

    const result = await isPaymentTokenOwner({} as never, "0x1111111111111111111111111111111111111111");

    expect(result).toBe(true);
  });

  it("管理员发放支付代币时按 token decimals 转换数量并调用 mint", async () => {
    const wait = vi.fn().mockResolvedValue({ hash: "0xclaim" });
    mocks.paymentTokenContract.decimals.mockResolvedValue(6);
    mocks.paymentTokenContract.mint.mockResolvedValue({ wait });

    const result = await mintPaymentTokenByOwner({} as never, {
      to: "0x1111111111111111111111111111111111111111",
      amount: "1000",
    });

    expect(mocks.paymentTokenContract.mint).toHaveBeenCalledWith("0x1111111111111111111111111111111111111111", 1000n);
    expect(wait).toHaveBeenCalledTimes(1);
    expect(result).toEqual({ hash: "0xclaim" });
  });
});
