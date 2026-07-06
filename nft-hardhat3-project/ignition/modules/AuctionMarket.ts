import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

const ETH_DECIMALS = 18;

const networkFeedEnvByName: Record<string, string> = {
  anvil: "ANVIL_ETH_USD_FEED",
  hardhatMainnet: "ANVIL_ETH_USD_FEED",
  localhost: "ANVIL_ETH_USD_FEED",
  mainnet: "MAINNET_ETH_USD_FEED",
  sepolia: "SEPOLIA_ETH_USD_FEED",
};

function getCliOption(optionName: string): string | undefined {
  const equalPrefix = `${optionName}=`;
  const equalArg = process.argv.find((arg) => arg.startsWith(equalPrefix));
  if (equalArg !== undefined) {
    return equalArg.slice(equalPrefix.length);
  }

  const optionIndex = process.argv.indexOf(optionName);
  if (optionIndex === -1) {
    return undefined;
  }

  return process.argv[optionIndex + 1];
}

function requireEnv(name: string): string {
  const value = process.env[name]?.trim();
  if (value === undefined || value === "") {
    throw new Error(`Missing required environment variable ${name}`);
  }

  return value;
}

function resolveEthUsdFeed(): string {
  const explicitFeed = process.env.AUCTION_ETH_USD_FEED?.trim();
  if (explicitFeed) {
    return explicitFeed;
  }

  const networkName = process.env.DEPLOY_NETWORK ?? getCliOption("--network");
  const feedEnvName =
    networkName === undefined
      ? "ANVIL_ETH_USD_FEED"
      : networkFeedEnvByName[networkName] ?? "AUCTION_ETH_USD_FEED";

  return requireEnv(feedEnvName);
}

export default buildModule("AuctionMarketModule", (m) => {
  const owner = m.getAccount(0);
  const nftName = m.getParameter(
    "nftName",
    requireEnv("AUCTION_NFT_NAME"),
  );
  const nftSymbol = m.getParameter(
    "nftSymbol",
    requireEnv("AUCTION_NFT_SYMBOL"),
  );

  const ethUsdFeed = m.getParameter("ethUsdFeed", resolveEthUsdFeed());

  const nft = m.contract("AuctionNFT", [nftName, nftSymbol, owner]);
  const implementation = m.contract("AuctionMarket");
  const initData = m.encodeFunctionCall(implementation, "initialize", [owner]);
  const proxy = m.contract("AuctionMarketProxy", [implementation, initData]);
  const market = m.contractAt("AuctionMarket", proxy, {
    id: "AuctionMarketProxyAsMarket",
  });

  m.call(market, "setPaymentToken", [
    "0x0000000000000000000000000000000000000000",
    ethUsdFeed,
    ETH_DECIMALS,
    true,
  ]);

  return { nft, implementation, proxy, market };
});
