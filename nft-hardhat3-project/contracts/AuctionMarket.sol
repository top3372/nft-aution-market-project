// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import {IERC721} from "@openzeppelin/contracts/token/ERC721/IERC721.sol";
import {ERC721Holder} from "@openzeppelin/contracts/token/ERC721/utils/ERC721Holder.sol";
import {Initializable} from "@openzeppelin/contracts/proxy/utils/Initializable.sol";
import {OwnableUpgradeable} from "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import {UUPSUpgradeable} from "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";

import {AggregatorV3Interface} from "@chainlink/contracts/src/v0.8/shared/interfaces/AggregatorV3Interface.sol";

/// @title AuctionMarket
/// @notice 支持 ETH 和白名单 ERC20 出价的 NFT 拍卖市场，使用 Chainlink Price Feed 统一比较美元价格。
/// @dev
/// 合约采用 UUPS 升级模式：
/// - 业务状态保存在代理合约的存储中。
/// - `initialize` 代替构造函数初始化 owner。
/// - `_authorizeUpgrade` 只允许 owner 升级实现合约。
///
/// 拍卖资金流：
/// 1. 卖家创建拍卖时，NFT 被安全转入市场合约托管。
/// 2. 出价者用 ETH 或已启用的 ERC20 出价。
/// 3. 每次出价都会用对应 Chainlink feed 把原币数量换算成 8 位 USD 值。
/// 4. 新出价必须比当前最高 USD 价值更高，旧最高出价会立即退回。
/// 5. 拍卖结束后，NFT 给最高价者，最高出价资金给卖家；如果无人出价，NFT 退回卖家。
contract AuctionMarket is
  Initializable,
  OwnableUpgradeable,
  UUPSUpgradeable,
  ERC721Holder
{
  using SafeERC20 for IERC20;

  /// @notice ETH 在本合约中的支付 token 标识。
  address public constant NATIVE_TOKEN = address(0);

  /// @notice 市场统一使用 8 位 USD 精度，匹配常见 Chainlink USD feed。
  uint8 public constant USD_DECIMALS = 8;

  /// @notice 单个拍卖的完整状态。
  /// @param seller NFT 卖家，结算时收款或无人出价时收回 NFT。
  /// @param nft ERC721 合约地址。
  /// @param tokenId 被拍卖的 NFT tokenId。
  /// @param endTime 拍卖结束时间戳。
  /// @param ended 是否已经结算，防止重复结束拍卖。
  /// @param highestBidder 当前最高价者。
  /// @param paymentToken 当前最高价使用的支付资产，address(0) 表示 ETH。
  /// @param highestBid 当前最高出价的原币数量。
  /// @param highestBidUsd 当前最高出价换算后的 8 位 USD 价值。
  struct Auction {
    address seller;
    address nft;
    uint256 tokenId;
    uint64 endTime;
    bool ended;
    address highestBidder;
    address paymentToken;
    uint256 highestBid;
    uint256 highestBidUsd;
  }

  /// @notice 某种支付资产对应的 Chainlink 价格源配置。
  /// @param feed Chainlink AggregatorV3 feed 地址。
  /// @param tokenDecimals 支付资产自身精度，例如 ETH 为 18，USDT 常见为 6。
  /// @param enabled 是否允许用该资产出价。
  struct PaymentTokenConfig {
    AggregatorV3Interface feed;
    uint8 tokenDecimals;
    bool enabled;
  }

  /// @notice 拍卖列表，auctionId 就是数组下标。
  Auction[] public auctions;

  /// @notice 支付资产白名单配置。
  mapping(address token => PaymentTokenConfig config) public paymentTokens;

  event PaymentTokenUpdated(
    address indexed token,
    address indexed feed,
    uint8 tokenDecimals,
    bool enabled
  );
  /// @notice 旧版创建拍卖事件。
  /// @dev V3 仍会同步发出该事件，便于旧 indexer 或历史工具继续识别新拍卖。
  event AuctionCreated(
    uint256 indexed auctionId,
    address indexed seller,
    address indexed nft,
    uint256 tokenId
  );
  /// @notice 出价成功事件。
  /// @dev 后端 indexer 依赖该事件写入 bids 表，并刷新 auctions 当前最高价。
  event BidPlaced(
    uint256 indexed auctionId,
    address indexed bidder,
    address indexed paymentToken,
    uint256 amount,
    uint256 amountUsd
  );
  /// @notice 拍卖结束事件。
  /// @dev winner 为 address(0) 且 amount 为 0 表示无人出价，NFT 已退回卖家。
  event AuctionEnded(
    uint256 indexed auctionId,
    address indexed winner,
    address indexed paymentToken,
    uint256 amount
  );

  error InvalidAddress();
  error InvalidDuration();
  error InvalidAmount();
  error AuctionDoesNotExist();
  error AuctionAlreadyEnded();
  error AuctionStillActive();
  error AuctionExpired();
  error BidTooLow();
  error UnsupportedPaymentToken();
  error InvalidPriceFeed();
  error IncorrectEthValue();

  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    // 锁定实现合约，避免有人绕过代理直接初始化 implementation 并取得升级权限。
    _disableInitializers();
  }

  /// @notice 初始化代理合约状态。
  /// @param initialOwner 市场管理员，负责配置支付资产和执行 UUPS 升级。
  function initialize(address initialOwner) external initializer {
    __Ownable_init(initialOwner);
  }

  /// @notice 返回当前拍卖数量，方便前端或脚本枚举。
  function auctionCount() external view returns (uint256) {
    return auctions.length;
  }

  /// @notice 配置一种可用于出价的支付资产及其 USD 价格源。
  /// @dev
  /// - `token == address(0)` 表示 ETH。
  /// - ERC20 token 的 `tokenDecimals` 必须与对应合约 `decimals()` 一致。
  /// - feed 必须返回 token/USD 或 ETH/USD 价格，不能传反向价格。
  function setPaymentToken(
    address token,
    address feed,
    uint8 tokenDecimals,
    bool enabled
  ) external onlyOwner {
    if (feed == address(0)) {
      revert InvalidAddress();
    }

    paymentTokens[token] = PaymentTokenConfig({
      feed: AggregatorV3Interface(feed),
      tokenDecimals: tokenDecimals,
      enabled: enabled
    });

    emit PaymentTokenUpdated(token, feed, tokenDecimals, enabled);
  }

  /// @notice 创建拍卖并把 NFT 转入市场合约托管。
  /// @param nft ERC721 合约地址。
  /// @param tokenId 要拍卖的 tokenId。
  /// @param duration 拍卖持续秒数。
  /// @return auctionId 新创建的拍卖 ID。
  function createAuction(
    address nft,
    uint256 tokenId,
    uint64 duration
  ) external virtual returns (uint256 auctionId) {
    if (nft == address(0)) {
      revert InvalidAddress();
    }
    if (duration == 0) {
      revert InvalidDuration();
    }

    // auctionId 使用数组下标，天然递增且方便前端/后端按 ID 查询。
    auctionId = auctions.length;
    auctions.push(
      Auction({
        seller: msg.sender,
        nft: nft,
        tokenId: tokenId,
        endTime: uint64(block.timestamp) + duration,
        ended: false,
        highestBidder: address(0),
        paymentToken: NATIVE_TOKEN,
        highestBid: 0,
        highestBidUsd: 0
      })
    );

    // 创建拍卖必须先把 NFT 托管到市场合约，否则卖家可以在拍卖期间把 NFT 转走。
    IERC721(nft).safeTransferFrom(msg.sender, address(this), tokenId);

    emit AuctionCreated(auctionId, msg.sender, nft, tokenId);
  }

  /// @notice 使用 ETH 对某个拍卖出价。
  /// @param auctionId 拍卖 ID。
  function bidWithEth(uint256 auctionId) external payable virtual {
    if (msg.value == 0) {
      revert InvalidAmount();
    }

    _placeBid(auctionId, NATIVE_TOKEN, msg.value);
  }

  /// @notice 使用白名单 ERC20 对某个拍卖出价。
  /// @param auctionId 拍卖 ID。
  /// @param token ERC20 token 地址。
  /// @param amount 出价 token 数量，按 token 自身 decimals 计量。
  function bidWithToken(
    uint256 auctionId,
    address token,
    uint256 amount
  ) external payable virtual {
    if (msg.value != 0) {
      revert IncorrectEthValue();
    }
    if (token == NATIVE_TOKEN) {
      revert InvalidAddress();
    }
    if (amount == 0) {
      revert InvalidAmount();
    }

    // ERC20 出价先把 token 转入市场合约，再统一进入 _placeBid 做价格比较。
    // 如果后续价格比较失败，revert 会回滚本次 transferFrom，不会把低价出价留在合约里。
    IERC20(token).safeTransferFrom(msg.sender, address(this), amount);
    _placeBid(auctionId, token, amount);
  }

  /// @notice 结束拍卖并完成 NFT/资金结算。
  /// @param auctionId 拍卖 ID。
  function endAuction(uint256 auctionId) external virtual {
    Auction storage auction = _auctionOf(auctionId);

    if (auction.ended) {
      revert AuctionAlreadyEnded();
    }
    if (block.timestamp < auction.endTime) {
      revert AuctionStillActive();
    }

    auction.ended = true;

    if (auction.highestBidder == address(0)) {
      // 没有任何有效出价时，结算只需要把托管 NFT 退回卖家。
      IERC721(auction.nft).safeTransferFrom(
        address(this),
        auction.seller,
        auction.tokenId
      );

      emit AuctionEnded(auctionId, address(0), NATIVE_TOKEN, 0);
      return;
    }

    // 有最高出价时，NFT 给买家，资金给卖家。V2/V3 会重写结算加入手续费。
    IERC721(auction.nft).safeTransferFrom(
      address(this),
      auction.highestBidder,
      auction.tokenId
    );
    _payout(auction.paymentToken, auction.seller, auction.highestBid);

    emit AuctionEnded(
      auctionId,
      auction.highestBidder,
      auction.paymentToken,
      auction.highestBid
    );
  }

  /// @notice 把支付资产数量换算成 8 位 USD 价值。
  /// @param token 支付资产地址，address(0) 表示 ETH。
  /// @param amount 原币数量。
  /// @return amountUsd 8 位 USD 价值，例如 1,000 美元表示为 1000_00000000。
  function quoteBidUsd(
    address token,
    uint256 amount
  ) public view returns (uint256 amountUsd) {
    PaymentTokenConfig memory config = paymentTokens[token];
    if (!config.enabled) {
      revert UnsupportedPaymentToken();
    }

    (, int256 answer, , uint256 updatedAt, ) = config.feed.latestRoundData();
    if (answer <= 0 || updatedAt == 0) {
      revert InvalidPriceFeed();
    }

    uint8 feedDecimals = config.feed.decimals();
    uint256 price = uint256(answer);

    // 公式：
    // amountUsd = amount * price(token/USD) * 10^USD_DECIMALS
    //             / 10^tokenDecimals / 10^feedDecimals
    //
    // 这样 ETH(18 位)、USDT(6 位)、WBTC(8 位) 等不同精度资产都能转换到同一 USD 精度。
    amountUsd =
      (amount * price * (10 ** USD_DECIMALS)) /
      (10 ** config.tokenDecimals) /
      (10 ** feedDecimals);
  }

  /// @dev 统一处理 ETH/ERC20 出价比较、退款和状态更新。
  function _placeBid(
    uint256 auctionId,
    address token,
    uint256 amount
  ) internal virtual {
    Auction storage auction = _auctionOf(auctionId);

    if (auction.ended) {
      revert AuctionAlreadyEnded();
    }
    if (block.timestamp >= auction.endTime) {
      revert AuctionExpired();
    }

    uint256 amountUsd = quoteBidUsd(token, amount);
    if (amountUsd <= auction.highestBidUsd) {
      // 新出价必须严格高于当前最高 USD 价值。低价出价走 refund，再 revert 回滚本次状态。
      _refund(token, msg.sender, amount);
      revert BidTooLow();
    }

    // 先缓存旧最高价信息，更新新最高价后再退款，避免退款外部调用影响状态一致性。
    address previousBidder = auction.highestBidder;
    address previousPaymentToken = auction.paymentToken;
    uint256 previousBid = auction.highestBid;

    auction.highestBidder = msg.sender;
    auction.paymentToken = token;
    auction.highestBid = amount;
    auction.highestBidUsd = amountUsd;

    if (previousBidder != address(0)) {
      _refund(previousPaymentToken, previousBidder, previousBid);
    }

    emit BidPlaced(auctionId, msg.sender, token, amount, amountUsd);
  }

  /// @dev 低价 ERC20 出价已经先转入合约，revert 会回滚 transferFrom；ETH 低价也会随 revert 退回。
  function _refund(address token, address bidder, uint256 amount) private {
    if (amount == 0) {
      return;
    }
    _payout(token, bidder, amount);
  }

  /// @dev 根据支付资产类型发送 ETH 或 ERC20。
  function _payout(address token, address to, uint256 amount) internal {
    if (token == NATIVE_TOKEN) {
      (bool ok, ) = payable(to).call{value: amount}("");
      require(ok, "ETH transfer failed");
    } else {
      IERC20(token).safeTransfer(to, amount);
    }
  }

  /// @dev 读取拍卖并统一做 ID 边界检查。
  /// @dev 作为 internal hook 暴露给后续升级版本复用，不能改变 Auction 存储布局。
  function _auctionOf(uint256 auctionId) internal view returns (Auction storage) {
    if (auctionId >= auctions.length) {
      revert AuctionDoesNotExist();
    }
    return auctions[auctionId];
  }

  /// @dev UUPS 升级授权。只有市场 owner 可以升级实现合约。
  function _authorizeUpgrade(address newImplementation) internal override onlyOwner {}
}
