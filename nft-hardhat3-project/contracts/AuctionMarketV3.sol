// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import {IERC721} from "@openzeppelin/contracts/token/ERC721/IERC721.sol";

import {AuctionMarketV2} from "./AuctionMarketV2.sol";

/// @title AuctionMarketV3
/// @notice 在 V2 手续费拍卖基础上增加预约开始、起拍价、取消和固定 AuctionNFT 约束。
/// @dev
/// 本合约用于 UUPS 升级，必须遵守存储兼容：
/// - 不能修改 V1/V2 已有状态变量的顺序、名称和类型。
/// - V3 只能在 V2 状态变量后追加新状态。
/// - 旧 Auction struct 不新增字段，V3 扩展信息放到 auctionV3Extras。
contract AuctionMarketV3 is AuctionMarketV2 {
  using SafeERC20 for IERC20;

  /// @notice V3 专属的拍卖扩展信息。
  /// @param startTime 拍卖开始时间，未到该时间不能出价。
  /// @param startingPriceUsd 起拍价，使用与 quoteBidUsd 相同的 8 位 USD 精度。
  /// @param cancelled 是否已取消。取消只允许无人出价时由卖家执行。
  /// @param v3Created 是否由 createAuctionV3 创建，用于区分旧 V1/V2 拍卖。
  struct AuctionV3Extra {
    uint64 startTime;
    uint256 startingPriceUsd;
    bool cancelled;
    bool v3Created;
  }

  /// @notice 第一版 DApp 只允许拍卖这个 NFT 合约里的 token。
  address public auctionNft;

  /// @notice auctionId 到 V3 扩展信息的映射，旧拍卖的 v3Created 为 false。
  mapping(uint256 auctionId => AuctionV3Extra extra) public auctionV3Extras;

  /// @notice NFT 到活跃 V3 拍卖的映射，存 auctionId + 1 以避免 0 值歧义。
  mapping(address nft => mapping(uint256 tokenId => uint256 auctionIdPlusOne))
    public nftToActiveAuctionIdPlusOne;

  event AuctionCreatedV3(
    uint256 indexed auctionId,
    address indexed seller,
    uint256 indexed tokenId,
    uint64 startTime,
    uint64 endTime,
    uint256 startingPriceUsd
  );

  /// @notice V3 拍卖取消事件。
  /// @dev 后端 indexer 监听该事件，把 auctions.status 标记为 cancelled。
  event AuctionCancelled(
    uint256 indexed auctionId,
    address indexed seller,
    uint256 indexed tokenId
  );

  error V3AlreadyInitialized();
  error UnsupportedAuctionNft();
  error InvalidAuctionTime();
  error AuctionNotStarted();
  error AuctionCancelledAlready();
  error NotAuctionSeller();
  error AuctionHasBid();
  error BidBelowStartingPrice();
  error LegacyCreateAuctionDisabled();
  error NativeBidNotSupportedForV3();
  error AuctionAlreadyActiveForNft();

  function version() external pure override returns (string memory) {
    return "3.0.0";
  }

  /// @notice 初始化 V3 支持的唯一 NFT 合约地址。
  /// @dev 通过 upgradeToAndCall 在升级交易中执行；只能初始化一次。
  function initializeV3(address auctionNft_) external onlyOwner {
    if (auctionNft != address(0)) {
      revert V3AlreadyInitialized();
    }
    if (auctionNft_ == address(0)) {
      revert InvalidAddress();
    }

    // 固定 AuctionNFT 地址后，前端和后端都可以只处理这一套 NFT，降低第一版业务复杂度。
    auctionNft = auctionNft_;
  }

  /// @notice V3 后不再允许使用旧入口创建新拍卖，避免绕过 startTime 和起拍价规则。
  function createAuction(
    address,
    uint256,
    uint64
  ) external pure override returns (uint256) {
    revert LegacyCreateAuctionDisabled();
  }

  /// @notice 创建 V3 拍卖，并把配置好的 AuctionNFT token 托管到市场合约。
  /// @param tokenId 要拍卖的 AuctionNFT tokenId。
  /// @param startTime 拍卖开始时间。
  /// @param endTime 拍卖结束时间。
  /// @param startingPriceUsd 起拍价，8 位 USD 精度。
  function createAuctionV3(
    uint256 tokenId,
    uint64 startTime,
    uint64 endTime,
    uint256 startingPriceUsd
  ) external returns (uint256 auctionId) {
    if (auctionNft == address(0)) {
      revert InvalidAddress();
    }
    if (startTime >= endTime || endTime <= block.timestamp) {
      revert InvalidAuctionTime();
    }
    if (nftToActiveAuctionIdPlusOne[auctionNft][tokenId] != 0) {
      revert AuctionAlreadyActiveForNft();
    }

    // V3 不修改 V1 的 Auction 结构，把 startTime/startingPriceUsd 放入单独 mapping。
    // 这样升级时不会破坏代理合约中已经存在的 auctions 数组存储布局。
    auctionId = auctions.length;
    auctions.push(
      Auction({
        seller: msg.sender,
        nft: auctionNft,
        tokenId: tokenId,
        endTime: endTime,
        ended: false,
        highestBidder: address(0),
        paymentToken: NATIVE_TOKEN,
        highestBid: 0,
        highestBidUsd: 0
      })
    );

    auctionV3Extras[auctionId] = AuctionV3Extra({
      startTime: startTime,
      startingPriceUsd: startingPriceUsd,
      cancelled: false,
      v3Created: true
    });
    // 使用 auctionId + 1 是为了让 0 表示“没有活跃拍卖”。
    nftToActiveAuctionIdPlusOne[auctionNft][tokenId] = auctionId + 1;

    // NFT 在创建拍卖时进入市场托管；取消或结束时再转出。
    IERC721(auctionNft).safeTransferFrom(msg.sender, address(this), tokenId);

    // 同时发旧事件和 V3 专属事件：旧事件保证兼容，V3 事件提供开始时间和起拍价。
    emit AuctionCreated(auctionId, msg.sender, auctionNft, tokenId);
    emit AuctionCreatedV3(
      auctionId,
      msg.sender,
      tokenId,
      startTime,
      endTime,
      startingPriceUsd
    );
  }

  /// @notice ETH 只兼容旧拍卖；V3 拍卖按本期设计只接受 ERC20 paymentToken。
  function bidWithEth(uint256 auctionId) external payable override {
    if (auctionV3Extras[auctionId].v3Created) {
      revert NativeBidNotSupportedForV3();
    }
    if (msg.value == 0) {
      revert InvalidAmount();
    }

    _placeBid(auctionId, NATIVE_TOKEN, msg.value);
  }

  /// @notice 使用 ERC20 对拍卖出价；V3 拍卖会额外检查开始时间和起拍价。
  function bidWithToken(
    uint256 auctionId,
    address token,
    uint256 amount
  ) external payable override {
    if (msg.value != 0) {
      revert IncorrectEthValue();
    }
    if (token == NATIVE_TOKEN) {
      revert InvalidAddress();
    }
    if (amount == 0) {
      revert InvalidAmount();
    }

    // V3 的支付资产必须是白名单 ERC20。转账后若起拍价/最高价检查失败，revert 会回滚转账。
    IERC20(token).safeTransferFrom(msg.sender, address(this), amount);
    _placeBid(auctionId, token, amount);
  }

  /// @notice 卖家在无人出价时取消 V3 拍卖，并取回 NFT。
  function cancelAuction(uint256 auctionId) external {
    Auction storage auction = _v2AuctionOf(auctionId);
    AuctionV3Extra storage extra = auctionV3Extras[auctionId];

    if (!extra.v3Created) {
      revert AuctionDoesNotExist();
    }
    if (auction.seller != msg.sender) {
      revert NotAuctionSeller();
    }
    if (auction.ended) {
      revert AuctionAlreadyEnded();
    }
    if (extra.cancelled) {
      revert AuctionCancelledAlready();
    }
    if (auction.highestBidder != address(0)) {
      revert AuctionHasBid();
    }

    extra.cancelled = true;
    auction.ended = true;
    // 清理活跃映射后，同一个 NFT 可以再次创建新拍卖。
    nftToActiveAuctionIdPlusOne[auction.nft][auction.tokenId] = 0;

    IERC721(auction.nft).safeTransferFrom(address(this), auction.seller, auction.tokenId);

    emit AuctionCancelled(auctionId, auction.seller, auction.tokenId);
  }

  /// @notice 结束拍卖；V3 拍卖会先拒绝已取消状态，并在结算后清理活跃 NFT 映射。
  function endAuction(uint256 auctionId) external override {
    Auction storage auction = _v2AuctionOf(auctionId);
    AuctionV3Extra memory extra = auctionV3Extras[auctionId];

    if (extra.v3Created && extra.cancelled) {
      revert AuctionCancelledAlready();
    }

    _endAuctionWithV2Fees(auctionId);

    if (extra.v3Created) {
      // 正常结算后也要释放 NFT -> auctionId 的占用关系。
      nftToActiveAuctionIdPlusOne[auction.nft][auction.tokenId] = 0;
    }
  }

  /// @dev V3 出价检查放在 super._placeBid 前，避免低于起拍价的转账被保留。
  function _placeBid(
    uint256 auctionId,
    address token,
    uint256 amount
  ) internal override {
    AuctionV3Extra memory extra = auctionV3Extras[auctionId];

    if (extra.v3Created) {
      if (extra.cancelled) {
        revert AuctionCancelledAlready();
      }
      if (block.timestamp < extra.startTime) {
        revert AuctionNotStarted();
      }

      // 起拍价用 USD 价值比较，不关心用户选择的 ERC20 原币精度。
      uint256 amountUsd = quoteBidUsd(token, amount);
      if (amountUsd < extra.startingPriceUsd) {
        revert BidBelowStartingPrice();
      }
    }

    super._placeBid(auctionId, token, amount);
  }
}
