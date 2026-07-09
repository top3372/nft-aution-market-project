// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

/// @title AuctionPaymentToken
/// @notice NFT 拍卖市场使用的正式 ERC20 支付代币。
/// @dev
/// - 本合约不是拍卖市场，只负责提供 ERC20 余额、授权和转账能力。
/// - 拍卖市场通过 IERC20 接口调用本合约的 `transferFrom`，所以用户出价前必须先 `approve`。
/// - 铸币权限限制为 owner，避免任何人都能公开 mint 的发行风险。
/// - `decimals` 在构造函数中固定，部署后不能修改；市场 `setPaymentToken` 配置的 decimals
///   必须和这里返回的 decimals 保持一致。
contract AuctionPaymentToken is ERC20, Ownable {
  /// @notice 代币精度，例如 6 表示 1 个完整代币等于 1_000_000 个最小单位。
  uint8 private immutable _tokenDecimals;

  /// @notice 部署支付代币。
  /// @param name_ ERC20 名称，例如 "Auction USD"。
  /// @param symbol_ ERC20 符号，例如 "AUSD"。
  /// @param decimals_ ERC20 精度，常见稳定币为 6，ETH 风格代币为 18。
  /// @param initialOwner 初始管理员，拥有 mint 权限和后续转移 owner 的权限。
  constructor(
    string memory name_,
    string memory symbol_,
    uint8 decimals_,
    address initialOwner
  ) ERC20(name_, symbol_) Ownable(initialOwner) {
    // decimals 用 immutable 固化，部署后不会被 owner 改掉，避免市场配置和代币精度漂移。
    _tokenDecimals = decimals_;
  }

  /// @notice 返回代币精度。
  /// @dev 前端会调用该函数把用户输入数量转换成 ERC20 最小单位。
  function decimals() public view override returns (uint8) {
    return _tokenDecimals;
  }

  /// @notice 给指定地址铸造代币。
  /// @dev 只有 owner 可以调用。测试网可用于发放演示余额；生产环境应配合严格的发行规则。
  /// @param to 接收地址。
  /// @param amount 铸造数量，必须按 decimals 后的最小单位传入。
  function mint(address to, uint256 amount) external onlyOwner {
    // 这里不做业务订单校验；当前合约只提供受控 mint 能力，发放规则由 owner/后台流程保证。
    _mint(to, amount);
  }
}
