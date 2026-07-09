// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import {ERC1967Proxy} from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";

/// @title AuctionMarketProxy
/// @notice 拍卖市场使用的 ERC1967/UUPS 代理合约。
/// @dev
/// OpenZeppelin 的 ERC1967Proxy 已经实现了标准代理逻辑：
/// - 业务调用通过 fallback delegatecall 到 implementation。
/// - implementation 地址存放在 EIP-1967 标准槽位。
/// - 构造时可传入初始化 calldata，直接初始化代理存储。
///
/// 这里做一层项目本地包装，是为了让 Hardhat 为代理生成稳定 artifact，
/// 测试和部署脚本可以像部署普通合约一样部署代理。
contract AuctionMarketProxy is ERC1967Proxy {
  /// @notice 部署代理并可选执行初始化调用。
  /// @param implementation 初始实现合约地址，例如 AuctionMarket 或 AuctionMarketV3。
  /// @param data 初始化 calldata，通常是 `initialize(initialOwner)` 的 ABI 编码。
  constructor(
    address implementation,
    bytes memory data
  ) ERC1967Proxy(implementation, data) {}
}
