// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import {ERC721URIStorage} from "@openzeppelin/contracts/token/ERC721/extensions/ERC721URIStorage.sol";
import {ERC721} from "@openzeppelin/contracts/token/ERC721/ERC721.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

/// @title AuctionNFT
/// @notice 一个用于拍卖市场演示的 ERC721 NFT 合约。
/// @dev
/// - 继承 OpenZeppelin ERC721URIStorage，支持每个 token 单独保存 tokenURI。
/// - 铸造权限限制为 owner，方便项目方或脚本集中铸造测试 NFT。
/// - NFT 的转移、授权和安全转移逻辑全部沿用 OpenZeppelin 标准实现。
contract AuctionNFT is ERC721URIStorage, Ownable {
  /// @notice 下一个将被铸造的 tokenId。
  /// @dev 从 0 开始自增，便于测试和文档说明。
  uint256 public nextTokenId;

  /// @notice 部署 NFT 合约。
  /// @param name_ NFT 集合名称。
  /// @param symbol_ NFT 集合符号。
  /// @param initialOwner 初始管理员，拥有铸造权限。
  constructor(
    string memory name_,
    string memory symbol_,
    address initialOwner
  ) ERC721(name_, symbol_) Ownable(initialOwner) {}

  /// @notice 铸造一个 NFT，并设置对应的元数据 URI。
  /// @dev
  /// 前端创建拍卖前会调用该方法生成测试 NFT。由于该方法受 onlyOwner 保护，
  /// 普通卖家钱包不能随意增发 NFT；生产环境可按业务需要改成白名单或公开 mint。
  /// @param to NFT 接收人。
  /// @param uri tokenURI，通常是 IPFS/HTTPS 元数据地址。
  /// @return tokenId 本次铸造出的 tokenId。
  function safeMint(
    address to,
    string calldata uri
  ) external onlyOwner returns (uint256 tokenId) {
    // 先读取当前 nextTokenId 作为本次 tokenId，再自增，保证 tokenId 从 0 连续递增。
    tokenId = nextTokenId;
    nextTokenId++;

    // _safeMint 会检查接收方是否能接收 ERC721；_setTokenURI 保存元数据地址。
    _safeMint(to, tokenId);
    _setTokenURI(tokenId, uri);
  }
}
