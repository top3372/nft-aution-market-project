# OpenZeppelin 权限与安全模块学习

本文档整理 OpenZeppelin 中最常用的权限和安全模块：

```text
Ownable
AccessControl
ReentrancyGuard
Pausable
ERC721Holder
```

学习目标：

```text
1. 知道每个模块解决什么问题。
2. 知道什么时候该用它。
3. 知道它不能替你解决什么问题。
4. 能写出对应测试。
```

## 1. Ownable

`Ownable` 是最简单的权限控制模块。它维护一个 owner 地址，并提供 `onlyOwner` 修饰器。

导入：

```solidity
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";
```

基本用法：

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

contract AdminBox is Ownable {
    string private value;

    constructor() Ownable(msg.sender) {}

    function setValue(string calldata newValue) external onlyOwner {
        value = newValue;
    }

    function getValue() external view returns (string memory) {
        return value;
    }
}
```

重点：

```text
OpenZeppelin 5.x 的 Ownable 构造函数是 Ownable(address initialOwner)。
所以必须显式传入初始 owner。
```

常用函数：

| 函数 | 作用 |
| --- | --- |
| `owner()` | 查询当前 owner。 |
| `transferOwnership(newOwner)` | 转移 owner。 |
| `renounceOwnership()` | 放弃 owner。 |

注意：

```text
renounceOwnership 会让 owner 变成 address(0)。
之后 onlyOwner 函数将无法被正常调用。
生产合约中要谨慎暴露这个操作。
```

## 2. AccessControl

如果合约不止一个管理员角色，就用 `AccessControl`。

例如：

```text
MINTER_ROLE 只能铸币
BURNER_ROLE 只能销毁
PAUSER_ROLE 只能暂停
DEFAULT_ADMIN_ROLE 管理角色授权
```

示例：

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";

contract RoleToken is ERC20, AccessControl {
    bytes32 public constant MINTER_ROLE = keccak256("MINTER_ROLE");
    bytes32 public constant BURNER_ROLE = keccak256("BURNER_ROLE");

    constructor(address admin) ERC20("Role Token", "ROLE") {
        _grantRole(DEFAULT_ADMIN_ROLE, admin);
        _grantRole(MINTER_ROLE, admin);
        _grantRole(BURNER_ROLE, admin);
    }

    function mint(address to, uint256 amount) external onlyRole(MINTER_ROLE) {
        _mint(to, amount);
    }

    function burn(address from, uint256 amount) external onlyRole(BURNER_ROLE) {
        _burn(from, amount);
    }
}
```

核心函数：

| 函数 | 作用 |
| --- | --- |
| `hasRole(role, account)` | 查询账号是否有角色。 |
| `grantRole(role, account)` | 授权角色。 |
| `revokeRole(role, account)` | 移除角色。 |
| `renounceRole(role, callerConfirmation)` | 用户自己放弃角色。 |

注意：

```text
DEFAULT_ADMIN_ROLE 权限很高。
拥有它的账号可以授予和撤销其他角色。
```

如果项目很简单，一个 `Ownable` 足够，不要为了显得复杂而使用 `AccessControl`。

## 3. Ownable 和 AccessControl 怎么选

| 场景 | 建议 |
| --- | --- |
| 作业、小 demo、单管理员合约 | `Ownable` |
| NFT 管理员 mint | `Ownable` |
| ERC20 只有 owner 能 mint | `Ownable` |
| 多个 minter、pauser、operator | `AccessControl` |
| DAO 或治理控制多个操作权限 | `AccessControl` 或更高级权限模块 |

判断标准：

```text
如果所有敏感操作都属于同一个管理员，用 Ownable。
如果不同操作需要不同账号负责，用 AccessControl。
```

## 4. ReentrancyGuard

`ReentrancyGuard` 用于防止重入攻击。

导入：

```solidity
import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
```

典型场景：合约向用户转 ETH。

示例：

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

contract StudyVault is ReentrancyGuard {
    mapping(address account => uint256 amount) public balances;

    function deposit() external payable {
        balances[msg.sender] += msg.value;
    }

    function withdraw() external nonReentrant {
        uint256 amount = balances[msg.sender];
        require(amount > 0, "nothing to withdraw");

        balances[msg.sender] = 0;

        (bool ok, ) = msg.sender.call{value: amount}("");
        require(ok, "ETH transfer failed");
    }
}
```

为什么先把余额改成 0，再转账？

```text
这是 Checks-Effects-Interactions 模式。
Checks: 检查 amount > 0。
Effects: 先修改 balances。
Interactions: 最后和外部地址交互，发送 ETH。
```

`nonReentrant` 是额外保护，不应该替代正确的状态更新顺序。

### ReentrancyGuard 注意点

OpenZeppelin 的 `nonReentrant` 使用单一锁。

也就是说：

```text
一个 nonReentrant 函数不能直接调用另一个 nonReentrant 函数。
```

常见写法：

```solidity
function withdraw() external nonReentrant {
    _withdraw(msg.sender);
}

function emergencyWithdraw() external nonReentrant {
    _withdraw(msg.sender);
}

function _withdraw(address account) private {
    // 真正逻辑放在 private 函数中
}
```

## 5. Pausable

`Pausable` 用于紧急暂停。

常见场景：

```text
发现合约异常时，暂停 deposit、withdraw、bid、mint 等关键操作。
```

示例：

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import {ERC20Pausable} from "@openzeppelin/contracts/token/ERC20/extensions/ERC20Pausable.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

contract PausableStudyToken is ERC20, ERC20Pausable, Ownable {
    constructor() ERC20("Pausable Study Token", "PST") Ownable(msg.sender) {
        _mint(msg.sender, 1_000_000 * 10 ** decimals());
    }

    function pause() external onlyOwner {
        _pause();
    }

    function unpause() external onlyOwner {
        _unpause();
    }

    function _update(address from, address to, uint256 value) internal override(ERC20, ERC20Pausable) {
        super._update(from, to, value);
    }
}
```

解释：

```text
ERC20Pausable 会在 _update 中检查 paused 状态。
所以暂停后 transfer、mint、burn 都会被限制。
```

如果业务只想暂停某些函数，可以直接使用 `whenNotPaused`：

```solidity
function deposit() external payable whenNotPaused {
    // ...
}
```

## 6. ERC721Holder

如果合约需要通过 `safeTransferFrom` 接收 ERC721，需要实现 ERC721 接收接口。

OpenZeppelin 提供：

```solidity
import {ERC721Holder} from "@openzeppelin/contracts/token/ERC721/utils/ERC721Holder.sol";
```

示例：

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import {IERC721} from "@openzeppelin/contracts/token/ERC721/IERC721.sol";
import {ERC721Holder} from "@openzeppelin/contracts/token/ERC721/utils/ERC721Holder.sol";

contract NftEscrow is ERC721Holder {
    function depositNft(address nft, uint256 tokenId) external {
        IERC721(nft).safeTransferFrom(msg.sender, address(this), tokenId);
    }
}
```

当前项目的拍卖市场合约接收用户 NFT 托管，也需要这个能力。

## 7. 安全模块不能替你做什么

OpenZeppelin 能减少底层错误，但不能自动保证业务安全。

| 模块 | 能做什么 | 不能做什么 |
| --- | --- | --- |
| `Ownable` | 限制只有 owner 调用。 | 判断 owner 是不是可信、私钥是否安全。 |
| `AccessControl` | 管理多角色权限。 | 自动设计正确的角色边界。 |
| `ReentrancyGuard` | 防止重入进入受保护函数。 | 修复错误的业务流程、错误的价格计算。 |
| `Pausable` | 提供暂停开关。 | 自动发现异常、自动暂停。 |
| `ERC721Holder` | 允许合约接收 ERC721。 | 判断 NFT 是否应该被接收。 |

写合约时仍然要自己做业务校验。

## 8. 权限测试清单

每个权限函数至少测：

```text
1. 授权账号调用成功。
2. 未授权账号调用 revert。
3. 权限转移后，旧账号失去权限。
4. 权限转移后，新账号获得权限。
```

Ownable 示例测试目标：

```text
owner 可以调用 setValue。
非 owner 调用 setValue 会 revert。
owner 可以 transferOwnership。
新 owner 可以调用 setValue。
旧 owner 不能再调用 setValue。
```

AccessControl 示例测试目标：

```text
MINTER_ROLE 可以 mint。
没有 MINTER_ROLE 不能 mint。
DEFAULT_ADMIN_ROLE 可以 grantRole。
没有 admin role 不能 grantRole。
revokeRole 后原 minter 不能再 mint。
```

## 9. 安全测试清单

ReentrancyGuard：

```text
1. 正常 withdraw 成功。
2. withdraw 后余额变成 0。
3. 重复 withdraw 会失败。
4. 用攻击合约尝试重入，应该失败。
```

Pausable：

```text
1. 未暂停时关键函数可用。
2. pause 后关键函数 revert。
3. unpause 后关键函数恢复。
4. 非 owner 不能 pause/unpause。
```

ERC721Holder：

```text
1. safeTransferFrom 到合约成功。
2. NFT owner 变成托管合约。
3. 不符合业务条件的 NFT 应该被拒绝。
```

## 10. 一句话总结

权限和安全模块的正确学习方式是：

```text
Ownable 解决单管理员。
AccessControl 解决多角色。
ReentrancyGuard 防重入，但仍要先改状态再外部调用。
Pausable 提供紧急刹车，但需要自己决定暂停哪些入口。
ERC721Holder 让合约能接收 NFT，但业务是否接收还要自己判断。
```

