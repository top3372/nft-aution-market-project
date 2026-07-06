import { network } from "hardhat";

const proxyAddress = process.env.AUCTION_MARKET_PROXY;

if (proxyAddress === undefined || proxyAddress.trim() === "") {
  throw new Error("Set AUCTION_MARKET_PROXY to the deployed proxy address");
}

const { ethers } = await network.create();
const [owner] = await ethers.getSigners();
const proxyCode = await ethers.provider.getCode(proxyAddress);

if (proxyCode === "0x") {
  throw new Error(
    `No contract code found at AUCTION_MARKET_PROXY (${proxyAddress}). Check the address and network.`,
  );
}

console.log("Upgrade sender:", owner.address);
console.log("AuctionMarket proxy:", proxyAddress);

const v2 = await ethers.deployContract("AuctionMarketV2", [], owner);
await v2.waitForDeployment();

const v2Address = await v2.getAddress();
console.log("AuctionMarketV2 implementation:", v2Address);

const market = await ethers.getContractAt("AuctionMarket", proxyAddress, owner);
const tx = await market.upgradeToAndCall(v2Address, "0x");
await tx.wait();

const upgraded = await ethers.getContractAt(
  "AuctionMarketV2",
  proxyAddress,
  owner,
);

console.log("Upgrade transaction:", tx.hash);
console.log("Proxy version after upgrade:", await upgraded.version());
