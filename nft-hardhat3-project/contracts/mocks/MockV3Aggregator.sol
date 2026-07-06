// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import {AggregatorV3Interface} from "@chainlink/contracts/src/v0.8/shared/interfaces/AggregatorV3Interface.sol";

/// @title MockV3Aggregator
/// @notice 测试用 Chainlink Price Feed mock。
/// @dev
/// 真实 Chainlink feed 会由预言机网络更新价格；测试中我们只需要可控的 decimals 和 answer，
/// 用于验证市场合约是否按同一 USD 精度比较 ETH/ERC20 出价。
contract MockV3Aggregator is AggregatorV3Interface {
  uint8 public immutable override decimals;

  int256 private _answer;
  uint80 private _roundId;
  uint256 private _updatedAt;

  constructor(uint8 decimals_, int256 initialAnswer) {
    decimals = decimals_;
    updateAnswer(initialAnswer);
  }

  /// @inheritdoc AggregatorV3Interface
  function description() external pure override returns (string memory) {
    return "MockV3Aggregator";
  }

  /// @inheritdoc AggregatorV3Interface
  function version() external pure override returns (uint256) {
    return 1;
  }

  /// @notice 更新 mock 价格，供测试模拟价格变化。
  function updateAnswer(int256 newAnswer) public {
    _answer = newAnswer;
    _roundId++;
    _updatedAt = block.timestamp;
  }

  /// @inheritdoc AggregatorV3Interface
  function getRoundData(
    uint80 requestedRoundId
  )
    external
    view
    override
    returns (
      uint80 roundId,
      int256 answer,
      uint256 startedAt,
      uint256 updatedAt,
      uint80 answeredInRound
    )
  {
    if (requestedRoundId != _roundId || _updatedAt == 0) {
      revert("No data present");
    }

    return (_roundId, _answer, _updatedAt, _updatedAt, _roundId);
  }

  /// @inheritdoc AggregatorV3Interface
  function latestRoundData()
    external
    view
    override
    returns (
      uint80 roundId,
      int256 answer,
      uint256 startedAt,
      uint256 updatedAt,
      uint80 answeredInRound
    )
  {
    return (_roundId, _answer, _updatedAt, _updatedAt, _roundId);
  }
}
