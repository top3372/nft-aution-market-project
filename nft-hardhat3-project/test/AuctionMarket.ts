import { expect } from "chai";
import { network } from "hardhat";

const { ethers, networkHelpers } = await network.create();

const ONE_ETH = ethers.parseEther("1");
const ETH_USD_PRICE = 2_000_00000000n;
const TOKEN_USD_PRICE = 1_00000000n;
const USD_TOKEN_DECIMALS = 6;

describe("AuctionMarket", function () {
  async function deployFixture() {
    const [owner, seller, alice, bob] = await ethers.getSigners();

    const ethFeed = await ethers.deployContract("MockV3Aggregator", [
      8,
      ETH_USD_PRICE,
    ]);
    const tokenFeed = await ethers.deployContract("MockV3Aggregator", [
      8,
      TOKEN_USD_PRICE,
    ]);
    const paymentToken = await ethers.deployContract("AuctionPaymentToken", [
      "USD Test Token",
      "USDT",
      USD_TOKEN_DECIMALS,
      owner.address,
    ]);
    const nft = await ethers.deployContract("AuctionNFT", [
      "Auction NFT",
      "ANFT",
      owner.address,
    ]);

    const implementation = await ethers.deployContract("AuctionMarket");
    const initData = implementation.interface.encodeFunctionData("initialize", [
      owner.address,
    ]);
    const proxy = await ethers.deployContract("AuctionMarketProxy", [
      await implementation.getAddress(),
      initData,
    ]);
    const market = await ethers.getContractAt(
      "AuctionMarket",
      await proxy.getAddress(),
    );

    await market.setPaymentToken(
      ethers.ZeroAddress,
      await ethFeed.getAddress(),
      18,
      true,
    );
    await market.setPaymentToken(
      await paymentToken.getAddress(),
      await tokenFeed.getAddress(),
      USD_TOKEN_DECIMALS,
      true,
    );

    await nft.safeMint(seller.address, "ipfs://auction-nft");
    await nft.connect(seller).approve(await market.getAddress(), 0n);
    await paymentToken.mint(alice.address, 5_000_000000n);
    await paymentToken.mint(bob.address, 5_000_000000n);

    return {
      owner,
      seller,
      alice,
      bob,
      ethFeed,
      tokenFeed,
      paymentToken,
      nft,
      market,
    };
  }

  async function createAuction() {
    const fixture = await networkHelpers.loadFixture(deployFixture);
    const { seller, nft, market } = fixture;

    await expect(
      market
        .connect(seller)
        .createAuction(await nft.getAddress(), 0n, 3_600n),
    )
      .to.emit(market, "AuctionCreated")
      .withArgs(0n, seller.address, await nft.getAddress(), 0n);

    return fixture;
  }

  async function upgradeAuctionToV2() {
    const fixture = await networkHelpers.loadFixture(createAuction);
    const { owner, market } = fixture;
    const v2 = await ethers.deployContract("AuctionMarketV2");

    await market
      .connect(owner)
      .upgradeToAndCall(await v2.getAddress(), "0x");

    const upgraded = await ethers.getContractAt(
      "AuctionMarketV2",
      await market.getAddress(),
    );

    return {
      ...fixture,
      market: upgraded,
    };
  }

  async function upgradeAuctionToV3() {
    const fixture = await networkHelpers.loadFixture(createAuction);
    const { owner, market, nft } = fixture;
    const v2 = await ethers.deployContract("AuctionMarketV2");

    await market
      .connect(owner)
      .upgradeToAndCall(await v2.getAddress(), "0x");

    const v2Market = await ethers.getContractAt(
      "AuctionMarketV2",
      await market.getAddress(),
    );
    const v3 = await ethers.deployContract("AuctionMarketV3");
    const initV3Data = v3.interface.encodeFunctionData("initializeV3", [
      await nft.getAddress(),
    ]);

    await v2Market
      .connect(owner)
      .upgradeToAndCall(await v3.getAddress(), initV3Data);

    const upgraded = await ethers.getContractAt(
      "AuctionMarketV3",
      await market.getAddress(),
    );

    return {
      ...fixture,
      market: upgraded,
    };
  }

  it("custodies an NFT when the seller creates an auction", async function () {
    const { seller, nft, market } =
      await networkHelpers.loadFixture(createAuction);

    const auction = await market.auctions(0n);

    expect(auction.seller).to.equal(seller.address);
    expect(auction.nft).to.equal(await nft.getAddress());
    expect(auction.tokenId).to.equal(0n);
    expect(auction.ended).to.equal(false);
    expect(await nft.ownerOf(0n)).to.equal(await market.getAddress());
  });

  it("quotes enabled ETH and ERC20 bids in 8-decimal USD units", async function () {
    const { paymentToken, market } =
      await networkHelpers.loadFixture(deployFixture);

    expect(await market.quoteBidUsd(ethers.ZeroAddress, ONE_ETH / 2n)).to.equal(
      1_000_00000000n,
    );
    expect(
      await market.quoteBidUsd(await paymentToken.getAddress(), 250_000000n),
    ).to.equal(250_00000000n);
  });

  it("compares ETH and ERC20 bids by USD value and refunds the previous bidder", async function () {
    const { alice, bob, paymentToken, market } =
      await networkHelpers.loadFixture(createAuction);
    const marketAddress = await market.getAddress();

    await expect(
      market.connect(alice).bidWithEth(0n, { value: ONE_ETH }),
    )
      .to.emit(market, "BidPlaced")
      .withArgs(0n, alice.address, ethers.ZeroAddress, ONE_ETH, 2_000_00000000n);

    await paymentToken.connect(bob).approve(marketAddress, 1_500_000000n);
    await expect(
      market
        .connect(bob)
        .bidWithToken(0n, await paymentToken.getAddress(), 1_500_000000n),
    ).to.be.revertedWithCustomError(market, "BidTooLow");

    await paymentToken.connect(bob).approve(marketAddress, 2_100_000000n);
    await expect(
      market
        .connect(bob)
        .bidWithToken(0n, await paymentToken.getAddress(), 2_100_000000n),
    )
      .to.emit(market, "BidPlaced")
      .withArgs(
        0n,
        bob.address,
        await paymentToken.getAddress(),
        2_100_000000n,
        2_100_00000000n,
      );

    const auction = await market.auctions(0n);

    expect(auction.highestBidder).to.equal(bob.address);
    expect(auction.paymentToken).to.equal(await paymentToken.getAddress());
    expect(auction.highestBid).to.equal(2_100_000000n);
    expect(await ethers.provider.getBalance(marketAddress)).to.equal(0n);
    expect(await paymentToken.balanceOf(marketAddress)).to.equal(2_100_000000n);
  });

  it("settles the auction by transferring the NFT to the winner and funds to the seller", async function () {
    const { seller, bob, nft, paymentToken, market } =
      await networkHelpers.loadFixture(createAuction);
    const bidAmount = 2_100_000000n;

    await paymentToken
      .connect(bob)
      .approve(await market.getAddress(), bidAmount);
    await market
      .connect(bob)
      .bidWithToken(0n, await paymentToken.getAddress(), bidAmount);

    await networkHelpers.time.increase(3_601);

    await expect(market.endAuction(0n))
      .to.emit(market, "AuctionEnded")
      .withArgs(0n, bob.address, await paymentToken.getAddress(), bidAmount);

    expect(await nft.ownerOf(0n)).to.equal(bob.address);
    expect(await paymentToken.balanceOf(seller.address)).to.equal(bidAmount);
  });

  it("returns the NFT to the seller when an auction ends without bids", async function () {
    const { seller, nft, market } =
      await networkHelpers.loadFixture(createAuction);

    await networkHelpers.time.increase(3_601);
    await market.endAuction(0n);

    expect(await nft.ownerOf(0n)).to.equal(seller.address);
  });

  it("rejects bids with unsupported tokens", async function () {
    const { owner, alice, market } = await networkHelpers.loadFixture(createAuction);
    const unsupportedToken = await ethers.deployContract("AuctionPaymentToken", [
      "Unsupported",
      "NOPE",
      18,
      owner.address,
    ]);

    await unsupportedToken.mint(alice.address, ONE_ETH);
    await unsupportedToken
      .connect(alice)
      .approve(await market.getAddress(), ONE_ETH);

    await expect(
      market
        .connect(alice)
        .bidWithToken(0n, await unsupportedToken.getAddress(), ONE_ETH),
    ).to.be.revertedWithCustomError(market, "UnsupportedPaymentToken");
  });

  it("upgrades through UUPS and keeps auction state", async function () {
    const { owner, seller, nft, market } =
      await networkHelpers.loadFixture(createAuction);
    const v2 = await ethers.deployContract("AuctionMarketV2");

    await market
      .connect(owner)
      .upgradeToAndCall(await v2.getAddress(), "0x");

    const upgraded = await ethers.getContractAt(
      "AuctionMarketV2",
      await market.getAddress(),
    );
    const auction = await upgraded.auctions(0n);

    expect(await upgraded.version()).to.equal("2.0.0");
    expect(auction.seller).to.equal(seller.address);
    expect(auction.nft).to.equal(await nft.getAddress());
  });

  it("lets the owner configure V2 fee parameters and preview seller net proceeds", async function () {
    const { owner, alice, market } =
      await networkHelpers.loadFixture(upgradeAuctionToV2);

    await expect(market.connect(owner).setFeeConfig(alice.address, 250n))
      .to.emit(market, "FeeConfigUpdated")
      .withArgs(alice.address, 250n);

    const [feeAmount, sellerNetAmount] =
      await market.calculateSellerNetProceeds(10_000n);

    expect(await market.feeRecipient()).to.equal(alice.address);
    expect(await market.platformFeeBps()).to.equal(250n);
    expect(feeAmount).to.equal(250n);
    expect(sellerNetAmount).to.equal(9_750n);

    await expect(
      market.connect(owner).setFeeConfig(alice.address, 10_001n),
    ).to.be.revertedWithCustomError(market, "InvalidFeeBps");
  });

  it("settles V2 ERC20 auctions by paying the fee recipient and seller net proceeds", async function () {
    const { owner, seller, alice, bob, nft, paymentToken, market } =
      await networkHelpers.loadFixture(upgradeAuctionToV2);
    const marketAddress = await market.getAddress();
    const bidAmount = 2_100_000000n;
    const feeAmount = 52_500000n;
    const sellerNetAmount = 2_047_500000n;
    const feeRecipientBalanceBefore = await paymentToken.balanceOf(
      alice.address,
    );

    await market.connect(owner).setFeeConfig(alice.address, 250n);
    await paymentToken.connect(bob).approve(marketAddress, bidAmount);
    await market
      .connect(bob)
      .bidWithToken(0n, await paymentToken.getAddress(), bidAmount);

    await networkHelpers.time.increase(3_601);

    await expect(market.endAuction(0n))
      .to.emit(market, "AuctionSettledWithFees")
      .withArgs(
        0n,
        seller.address,
        alice.address,
        await paymentToken.getAddress(),
        bidAmount,
        feeAmount,
        sellerNetAmount,
      );

    expect(await nft.ownerOf(0n)).to.equal(bob.address);
    expect(await paymentToken.balanceOf(seller.address)).to.equal(
      sellerNetAmount,
    );
    expect(await paymentToken.balanceOf(alice.address)).to.equal(
      feeRecipientBalanceBefore + feeAmount,
    );
  });

  it("upgrades to V3, initializes AuctionNFT, and keeps legacy auction state", async function () {
    const { seller, nft, market } =
      await networkHelpers.loadFixture(upgradeAuctionToV3);
    const auction = await market.auctions(0n);

    expect(await market.version()).to.equal("3.0.0");
    expect(await market.auctionNft()).to.equal(await nft.getAddress());
    expect(auction.seller).to.equal(seller.address);
    expect(auction.nft).to.equal(await nft.getAddress());
    expect(auction.tokenId).to.equal(0n);
  });

  it("creates a V3 scheduled auction for the configured AuctionNFT", async function () {
    const { seller, nft, market } =
      await networkHelpers.loadFixture(upgradeAuctionToV3);
    const now = await networkHelpers.time.latest();
    const startTime = BigInt(now + 60);
    const endTime = BigInt(now + 3_600);
    const startingPriceUsd = 100_00000000n;

    await nft.safeMint(seller.address, "ipfs://auction-nft-v3");
    await nft.connect(seller).approve(await market.getAddress(), 1n);

    await expect(
      market
        .connect(seller)
        .createAuctionV3(1n, startTime, endTime, startingPriceUsd),
    )
      .to.emit(market, "AuctionCreatedV3")
      .withArgs(1n, seller.address, 1n, startTime, endTime, startingPriceUsd);

    const extra = await market.auctionV3Extras(1n);
    expect(extra.startTime).to.equal(startTime);
    expect(extra.startingPriceUsd).to.equal(startingPriceUsd);
    expect(extra.cancelled).to.equal(false);
    expect(extra.v3Created).to.equal(true);
    expect(
      await market.nftToActiveAuctionIdPlusOne(await nft.getAddress(), 1n),
    ).to.equal(2n);
    expect(await nft.ownerOf(1n)).to.equal(await market.getAddress());
  });

  it("rejects V3 token bids before start time and below starting price", async function () {
    const { seller, bob, nft, paymentToken, market } =
      await networkHelpers.loadFixture(upgradeAuctionToV3);
    const now = await networkHelpers.time.latest();
    const startTime = BigInt(now + 60);
    const endTime = BigInt(now + 3_600);

    await nft.safeMint(seller.address, "ipfs://auction-nft-v3");
    await nft.connect(seller).approve(await market.getAddress(), 1n);
    await market
      .connect(seller)
      .createAuctionV3(1n, startTime, endTime, 500_00000000n);

    await paymentToken
      .connect(bob)
      .approve(await market.getAddress(), 600_000000n);

    await expect(
      market
        .connect(bob)
        .bidWithToken(1n, await paymentToken.getAddress(), 600_000000n),
    ).to.be.revertedWithCustomError(market, "AuctionNotStarted");

    await networkHelpers.time.increaseTo(Number(startTime));

    await expect(
      market
        .connect(bob)
        .bidWithToken(1n, await paymentToken.getAddress(), 400_000000n),
    ).to.be.revertedWithCustomError(market, "BidBelowStartingPrice");
  });

  it("lets the seller cancel a V3 auction before any bid", async function () {
    const { seller, nft, market } =
      await networkHelpers.loadFixture(upgradeAuctionToV3);
    const now = await networkHelpers.time.latest();

    await nft.safeMint(seller.address, "ipfs://auction-nft-v3");
    await nft.connect(seller).approve(await market.getAddress(), 1n);
    await market
      .connect(seller)
      .createAuctionV3(1n, BigInt(now), BigInt(now + 3_600), 100_00000000n);

    await expect(market.connect(seller).cancelAuction(1n))
      .to.emit(market, "AuctionCancelled")
      .withArgs(1n, seller.address, 1n);

    const extra = await market.auctionV3Extras(1n);
    expect(extra.cancelled).to.equal(true);
    expect(await nft.ownerOf(1n)).to.equal(seller.address);
    expect(
      await market.nftToActiveAuctionIdPlusOne(await nft.getAddress(), 1n),
    ).to.equal(0n);
  });
});
