import { network } from "hardhat";

const proxyAddress = process.env.AUCTION_MARKET_PROXY;
const auctionNftAddress = process.env.AUCTION_NFT_ADDRESS;

if (proxyAddress === undefined || proxyAddress.trim() === "") {
  throw new Error("Set AUCTION_MARKET_PROXY to the deployed proxy address");
}

if (auctionNftAddress === undefined || auctionNftAddress.trim() === "") {
  throw new Error("Set AUCTION_NFT_ADDRESS to the deployed AuctionNFT address");
}

const { ethers } = await network.create();
const [owner] = await ethers.getSigners();
const proxyCode = await ethers.provider.getCode(proxyAddress);
const nftCode = await ethers.provider.getCode(auctionNftAddress);

if (proxyCode === "0x") {
  throw new Error(
    `No contract code found at AUCTION_MARKET_PROXY (${proxyAddress}). Check the address and network.`,
  );
}

if (nftCode === "0x") {
  throw new Error(
    `No contract code found at AUCTION_NFT_ADDRESS (${auctionNftAddress}). Check the address and network.`,
  );
}

console.log("Upgrade sender:", owner.address);
console.log("AuctionMarket proxy:", proxyAddress);
console.log("AuctionNFT:", auctionNftAddress);

const v3 = await ethers.deployContract("AuctionMarketV3", [], owner);
await v3.waitForDeployment();

const v3Address = await v3.getAddress();
const initV3Data = v3.interface.encodeFunctionData("initializeV3", [
  auctionNftAddress,
]);

console.log("AuctionMarketV3 implementation:", v3Address);

const market = await ethers.getContractAt(
  "AuctionMarketV2",
  proxyAddress,
  owner,
);
const tx = await market.upgradeToAndCall(v3Address, initV3Data);
await tx.wait();

const upgraded = await ethers.getContractAt(
  "AuctionMarketV3",
  proxyAddress,
  owner,
);

console.log("Upgrade transaction:", tx.hash);
console.log("Proxy version after upgrade:", await upgraded.version());
console.log("AuctionNFT after upgrade:", await upgraded.auctionNft());
console.log("Auction count after upgrade:", await upgraded.auctionCount());
