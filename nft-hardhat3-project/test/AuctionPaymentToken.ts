import { expect } from "chai";
import { network } from "hardhat";

const { ethers } = await network.create();

describe("AuctionPaymentToken", function () {
  it("deploys with fixed metadata and owner", async function () {
    const [owner] = await ethers.getSigners();

    const token = await ethers.deployContract("AuctionPaymentToken", [
      "Auction USD",
      "AUSD",
      6,
      owner.address,
    ]);

    expect(await token.name()).to.equal("Auction USD");
    expect(await token.symbol()).to.equal("AUSD");
    expect(await token.decimals()).to.equal(6);
    expect(await token.owner()).to.equal(owner.address);
  });

  it("allows only the owner to mint payment tokens", async function () {
    const [owner, bidder] = await ethers.getSigners();
    const token = await ethers.deployContract("AuctionPaymentToken", [
      "Auction USD",
      "AUSD",
      6,
      owner.address,
    ]);

    await token.mint(bidder.address, 1_000_000n);

    expect(await token.balanceOf(bidder.address)).to.equal(1_000_000n);
    await expect(
      token.connect(bidder).mint(bidder.address, 1_000_000n),
    ).to.be.revertedWithCustomError(token, "OwnableUnauthorizedAccount");
  });
});
