// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import {IERC721} from "@openzeppelin/contracts/token/ERC721/IERC721.sol";

import {AuctionMarket} from "./AuctionMarket.sol";

/// @title AuctionMarketV2
/// @notice 第二版市场合约，在 V1 拍卖能力上增加平台手续费和卖家净收入计算。
/// @dev
/// 升级合约最重要的规则是“只能在旧状态变量后面追加新状态变量”。
/// 因此本合约继承 AuctionMarket，不改动 V1 里的 auctions、paymentTokens 等变量顺序，
/// 只在末尾追加 feeRecipient 和 platformFeeBps。
contract AuctionMarketV2 is AuctionMarket {
  using SafeERC20 for IERC20;

  /// @notice 手续费精度分母，10000 表示 100%，250 表示 2.5%。
  uint16 public constant BPS_DENOMINATOR = 10_000;

  /// @notice 平台手续费接收地址；未配置时默认使用 owner()。
  address public feeRecipient;

  /// @notice 平台手续费费率，单位是 basis points，100 = 1%。
  uint16 public platformFeeBps;

  event FeeConfigUpdated(address indexed feeRecipient, uint16 platformFeeBps);

  event AuctionSettledWithFees(
    uint256 indexed auctionId,
    address indexed seller,
    address indexed feeRecipient,
    address paymentToken,
    uint256 grossAmount,
    uint256 feeAmount,
    uint256 sellerNetAmount
  );

  error InvalidFeeBps();

  function version() external pure returns (string memory) {
    return "2.0.0";
  }

  /// @notice 配置平台手续费参数。
  /// @param newFeeRecipient 手续费接收地址。传 address(0) 表示使用当前 owner 作为接收人。
  /// @param newPlatformFeeBps 手续费费率，单位 bps。250 表示 2.5%，最大不能超过 10000。
  function setFeeConfig(
    address newFeeRecipient,
    uint16 newPlatformFeeBps
  ) external onlyOwner {
    if (newPlatformFeeBps > BPS_DENOMINATOR) {
      revert InvalidFeeBps();
    }

    feeRecipient = newFeeRecipient;
    platformFeeBps = newPlatformFeeBps;

    emit FeeConfigUpdated(newFeeRecipient, newPlatformFeeBps);
  }

  /// @notice 按当前手续费费率预估平台手续费和卖家净收入。
  /// @param grossAmount 买家最终支付的成交金额，也就是 auction.highestBid。
  /// @return feeAmount 平台手续费金额。
  /// @return sellerNetAmount 扣除手续费后卖家实际收到的金额。
  function calculateSellerNetProceeds(
    uint256 grossAmount
  ) public view returns (uint256 feeAmount, uint256 sellerNetAmount) {
    feeAmount = (grossAmount * platformFeeBps) / BPS_DENOMINATOR;
    sellerNetAmount = grossAmount - feeAmount;
  }

  /// @notice 结束拍卖并按 V2 手续费规则结算 NFT、平台手续费和卖家净收入。
  /// @dev
  /// 这里重写 V1 的 endAuction：
  /// - 无人出价时行为保持不变，NFT 退回卖家。
  /// - 有人出价时，NFT 给最高价者。
  /// - 成交金额先拆成 feeAmount 和 sellerNetAmount。
  /// - sellerNetAmount 支付给卖家，feeAmount 支付给 feeRecipient。
  function endAuction(uint256 auctionId) external override {
    Auction storage auction = _v2AuctionOf(auctionId);

    if (auction.ended) {
      revert AuctionAlreadyEnded();
    }
    if (block.timestamp < auction.endTime) {
      revert AuctionStillActive();
    }

    auction.ended = true;

    if (auction.highestBidder == address(0)) {
      _transferAuctionNft(auction, auction.seller);

      emit AuctionEnded(auctionId, address(0), NATIVE_TOKEN, 0);
      return;
    }

    address recipient = _effectiveFeeRecipient();
    (uint256 feeAmount, uint256 sellerNetAmount) = calculateSellerNetProceeds(
      auction.highestBid
    );

    _transferAuctionNft(auction, auction.highestBidder);
    _payoutV2(auction.paymentToken, auction.seller, sellerNetAmount);

    if (feeAmount != 0) {
      _payoutV2(auction.paymentToken, recipient, feeAmount);
    }

    emit AuctionEnded(
      auctionId,
      auction.highestBidder,
      auction.paymentToken,
      auction.highestBid
    );
    emit AuctionSettledWithFees(
      auctionId,
      auction.seller,
      recipient,
      auction.paymentToken,
      auction.highestBid,
      feeAmount,
      sellerNetAmount
    );
  }

  /// @dev 读取拍卖并保持与 V1 相同的 ID 边界检查语义。
  function _v2AuctionOf(uint256 auctionId) private view returns (Auction storage) {
    if (auctionId >= auctions.length) {
      revert AuctionDoesNotExist();
    }
    return auctions[auctionId];
  }

  /// @dev 未显式配置手续费接收人时，使用 owner，避免手续费被发送到 address(0)。
  function _effectiveFeeRecipient() private view returns (address) {
    if (feeRecipient == address(0)) {
      return owner();
    }
    return feeRecipient;
  }

  /// @dev 统一转移被市场托管的 NFT，避免结算分支重复写 safeTransferFrom。
  function _transferAuctionNft(Auction storage auction, address to) private {
    IERC721(auction.nft).safeTransferFrom(address(this), to, auction.tokenId);
  }

  /// @dev 根据支付资产类型发送 ETH 或 ERC20；address(0) 表示 ETH。
  function _payoutV2(address token, address to, uint256 amount) private {
    if (amount == 0) {
      return;
    }

    if (token == NATIVE_TOKEN) {
      (bool ok, ) = payable(to).call{value: amount}("");
      require(ok, "ETH transfer failed");
    } else {
      IERC20(token).safeTransfer(to, amount);
    }
  }
}
