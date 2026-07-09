import { network } from "hardhat";

const proxyAddress = process.env.AUCTION_MARKET_PROXY;
const tokenName = process.env.PAYMENT_TOKEN_NAME ?? "Auction USD";
const tokenSymbol = process.env.PAYMENT_TOKEN_SYMBOL ?? "AUSD";
const tokenDecimals = Number(process.env.PAYMENT_TOKEN_DECIMALS ?? "6");
const tokenPriceUsd = BigInt(process.env.PAYMENT_TOKEN_PRICE_USD ?? "100000000");
const mintTo = process.env.PAYMENT_TOKEN_MINT_TO;
const mintAmount = BigInt(process.env.PAYMENT_TOKEN_MINT_AMOUNT ?? "0");
const configureMarket = process.env.CONFIGURE_MARKET_PAYMENT_TOKEN !== "false";
const deployMockFeed = process.env.DEPLOY_MOCK_PRICE_FEED !== "false";
const existingFeedAddress = process.env.PAYMENT_TOKEN_FEED;

if (!Number.isInteger(tokenDecimals) || tokenDecimals < 0 || tokenDecimals > 255) {
  throw new Error("PAYMENT_TOKEN_DECIMALS must be an integer between 0 and 255");
}
if (configureMarket && (proxyAddress === undefined || proxyAddress.trim() === "")) {
  throw new Error("Set AUCTION_MARKET_PROXY or set CONFIGURE_MARKET_PAYMENT_TOKEN=false");
}
if (!deployMockFeed && (existingFeedAddress === undefined || existingFeedAddress.trim() === "")) {
  throw new Error("Set PAYMENT_TOKEN_FEED when DEPLOY_MOCK_PRICE_FEED=false");
}

const { ethers } = await network.create();
const [deployer] = await ethers.getSigners();

console.log("Deploy sender:", deployer.address);
if (proxyAddress !== undefined && proxyAddress.trim() !== "") {
  console.log("AuctionMarket proxy:", proxyAddress);
}

if (configureMarket && proxyAddress !== undefined) {
  const proxyCode = await ethers.provider.getCode(proxyAddress);
  if (proxyCode === "0x") {
    throw new Error(
      `No contract code found at AUCTION_MARKET_PROXY (${proxyAddress}). Check the address and network.`,
    );
  }
}

const paymentToken = await ethers.deployContract("AuctionPaymentToken", [
  tokenName,
  tokenSymbol,
  tokenDecimals,
  deployer.address,
]);
await paymentToken.waitForDeployment();

const paymentTokenAddress = await paymentToken.getAddress();
let paymentFeedAddress = existingFeedAddress ?? "";

if (deployMockFeed) {
  const paymentFeed = await ethers.deployContract("MockV3Aggregator", [
    8,
    tokenPriceUsd,
  ]);
  await paymentFeed.waitForDeployment();
  paymentFeedAddress = await paymentFeed.getAddress();
}

let configTxHash = "";
if (configureMarket && proxyAddress !== undefined) {
  const market = await ethers.getContractAt("AuctionMarketV3", proxyAddress, deployer);
  const configTx = await market.setPaymentToken(
    paymentTokenAddress,
    paymentFeedAddress,
    tokenDecimals,
    true,
  );
  await configTx.wait();
  configTxHash = configTx.hash;
}

let mintTxHash = "";
if (mintTo !== undefined && mintTo.trim() !== "" && mintAmount > 0n) {
  const mintTx = await paymentToken.mint(mintTo, mintAmount);
  await mintTx.wait();
  mintTxHash = mintTx.hash;
}

console.log("PAYMENT_TOKEN_ADDRESS=" + paymentTokenAddress);
console.log("PAYMENT_TOKEN_FEED=" + paymentFeedAddress);
console.log("PAYMENT_TOKEN_DECIMALS=" + tokenDecimals);
console.log("PAYMENT_TOKEN_OWNER=" + deployer.address);
if (configTxHash !== "") {
  console.log("setPaymentToken tx:", configTxHash);
}
if (mintTxHash !== "") {
  console.log("PAYMENT_TOKEN_MINT_TO=" + mintTo);
  console.log("PAYMENT_TOKEN_MINT_AMOUNT=" + mintAmount.toString());
  console.log("mint tx:", mintTxHash);
}
