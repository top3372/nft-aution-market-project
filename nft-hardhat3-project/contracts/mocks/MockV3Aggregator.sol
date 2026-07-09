// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import {AggregatorV3Interface} from "@chainlink/contracts/src/v0.8/shared/interfaces/AggregatorV3Interface.sol";

/// @title MockV3Aggregator
/// @notice 测试用 Chainlink Price Feed mock。
/// @dev
/// 真实 Chainlink feed 会由预言机网络更新价格；测试中我们只需要可控的 decimals 和 answer，
/// 用于验证市场合约是否按同一 USD 精度比较 ETH/ERC20 出价。
contract MockV3Aggregator is AggregatorV3Interface {
  /// @notice mock feed 的价格小数位，模拟真实 Chainlink feed 的 decimals。
  uint8 public immutable override decimals;

  /// @dev 最近一次 mock 价格。真实 feed 会保存多轮数据，这里只保留当前轮，满足测试需要。
  int256 private _answer;
  /// @dev 当前轮次 ID，每次 updateAnswer 自增。
  uint80 private _roundId;
  /// @dev 当前轮次更新时间，市场合约会检查 updatedAt 不能为 0。
  uint256 private _updatedAt;

  /// @notice 创建 mock 价格源。
  /// @param decimals_ 价格精度，例如 8 表示 2000_00000000。
  /// @param initialAnswer 初始价格答案。
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
  /// @param newAnswer 新价格答案，按 decimals 精度传入。
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
