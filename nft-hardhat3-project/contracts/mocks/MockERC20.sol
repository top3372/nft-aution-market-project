// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";

/// @title MockERC20
/// @notice 测试用 ERC20，允许任意地址铸造，便于拍卖测试构造余额。
/// @dev 生产环境不要使用这种开放 mint 权限的 token。
contract MockERC20 is ERC20 {
  uint8 private immutable _customDecimals;

  constructor(
    string memory name_,
    string memory symbol_,
    uint8 decimals_
  ) ERC20(name_, symbol_) {
    _customDecimals = decimals_;
  }

  function decimals() public view override returns (uint8) {
    return _customDecimals;
  }

  function mint(address to, uint256 amount) external {
    _mint(to, amount);
  }
}
