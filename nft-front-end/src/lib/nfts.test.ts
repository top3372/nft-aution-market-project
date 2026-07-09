import { describe, expect, it } from "vitest";
import { mergeAuctions, mergeNfts } from "./nfts";

describe("mergeNfts", () => {
  it("把链上扫描到的 NFT 补充到后端索引结果中", () => {
    const result = mergeNfts([], [
      {
        nftAddress: "0xNFT",
        tokenId: "1",
        ownerAddress: "0xOwner",
        tokenUri: "data:application/json;base64,test",
        metadataJson: "",
      },
    ]);

    expect(result.map((nft) => nft.tokenId)).toEqual(["1"]);
  });

  it("同一个 tokenId 同时存在时优先使用链上最新结果", () => {
    const result = mergeNfts([
      {
        nftAddress: "0xNFT",
        tokenId: "1",
        ownerAddress: "0xOld",
        tokenUri: "ipfs://old",
        metadataJson: "",
      },
    ], [
      {
        nftAddress: "0xNFT",
        tokenId: "1",
        ownerAddress: "0xNew",
        tokenUri: "ipfs://new",
        metadataJson: "",
      },
    ]);

    expect(result).toHaveLength(1);
    expect(result[0].ownerAddress).toBe("0xNew");
    expect(result[0].tokenUri).toBe("ipfs://new");
  });
});

describe("mergeAuctions", () => {
  it("把链上扫描到的拍卖补充到后端索引结果中", () => {
    const result = mergeAuctions([], [
      {
        auctionId: 1,
        seller: "0xSeller",
        nftAddress: "0xNFT",
        tokenId: "1",
        startTime: null,
        endTime: "2026-07-12T02:23:00.000Z",
        startingPriceUsd: "0",
        status: "pending",
        highestBidder: "",
        highestBid: "0",
        highestBidUsd: "0",
      },
    ]);

    expect(result.map((auction) => auction.auctionId)).toEqual([1]);
  });
});
