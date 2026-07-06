# OpenZeppelin ERC20 源码阅读与用法

本文档专门讲 `@openzeppelin/contracts/token/ERC20/ERC20.sol`。目标不是逐行背源码，而是看懂 ERC20 的核心状态、调用链和扩展点。

当前项目使用 OpenZeppelin Contracts 5.6.1。源码可以直接在本地打开：

```text
node_modules/@openzeppelin/contracts/token/ERC20/ERC20.sol
node_modules/@openzeppelin/contracts/token/ERC20/IERC20.sol
node_modules/@openzeppelin/contracts/token/ERC20/extensions/
```

## 1. ERC20 先理解什么

ERC20 是同质化代币标准。最核心的行为只有几类：

| 行为 | 函数 |
| --- | --- |
| 查询总量 | `totalSupply()` |
| 查询余额 | `balanceOf(account)` |
| 直接转账 | `transfer(to, value)` |
| 授权别人花自己的钱 | `approve(spender, value)` |
| 查询授权额度 | `allowance(owner, spender)` |
| 使用授权额度转账 | `transferFrom(from, to, value)` |

先把这 6 个函数看懂，再看扩展。

## 2. 核心状态变量

OpenZeppelin ERC20 的核心状态可以简化理解为：

```solidity
mapping(address account => uint256) private _balances;
mapping(address account => mapping(address spender => uint256)) private _allowances;
uint256 private _totalSupply;
string private _name;
string private _symbol;
```

解释：

| 状态 | 含义 |
| --- | --- |
| `_balances` | 每个地址有多少 token。 |
| `_allowances` | 某个 owner 授权某个 spender 使用多少 token。 |
| `_totalSupply` | 当前总发行量。 |
| `_name` | 代币名称。 |
| `_symbol` | 代币符号。 |

`decimals()` 默认返回 18，但它不影响链上的余额计算，只影响前端展示。

例如：

```text
链上余额：1000000000000000000
decimals: 18
前端展示：1.0
```

## 3. transfer 调用链

直接转账入口：

```solidity
function transfer(address to, uint256 value) public virtual returns (bool) {
    address owner = _msgSender();
    _transfer(owner, to, value);
    return true;
}
```

调用链：

```text
transfer(to, value)
  -> _transfer(from, to, value)
    -> _update(from, to, value)
```

`_transfer` 负责检查零地址：

```text
from 不能是 address(0)
to 不能是 address(0)
```

真正改余额的是 `_update`。

## 4. _update 是 OpenZeppelin 5.x 的重点

OpenZeppelin 5.x 中，ERC20 的核心扩展点是：

```solidity
function _update(address from, address to, uint256 value) internal virtual
```

它统一处理三种情况：

| 情况 | 含义 |
| --- | --- |
| `from == address(0)` | mint，凭空铸造 token。 |
| `to == address(0)` | burn，销毁 token。 |
| `from` 和 `to` 都不是零地址 | 普通转账。 |

简化逻辑：

```text
如果 from 是零地址：
  totalSupply 增加
否则：
  检查 from 余额够不够
  扣 from 余额

如果 to 是零地址：
  totalSupply 减少
否则：
  增加 to 余额

触发 Transfer 事件
```

所以在 5.x 中，如果要定制转账规则，通常重写 `_update()`，而不是重写 `transfer()`。

## 5. mint 和 burn 为什么也走 _update

ERC20 的 `_mint`：

```text
_mint(account, value)
  -> _update(address(0), account, value)
```

ERC20 的 `_burn`：

```text
_burn(account, value)
  -> _update(account, address(0), value)
```

这样设计的好处是：

```text
转账、铸造、销毁都经过同一个余额更新入口。
如果子合约要增加统一限制，可以在 _update 里做。
```

例如，你想暂停所有 token 变化，应该让 transfer、mint、burn 都受限制。

## 6. approve 和 transferFrom 调用链

授权：

```text
approve(spender, value)
  -> _approve(owner, spender, value)
```

使用授权转账：

```text
transferFrom(from, to, value)
  -> _spendAllowance(from, spender, value)
  -> _transfer(from, to, value)
    -> _update(from, to, value)
```

理解授权时，要分清 3 个角色：

| 角色 | 含义 |
| --- | --- |
| `owner` | token 的拥有者。 |
| `spender` | 被授权花钱的人。 |
| `to` | 最终收款地址。 |

例子：

```text
Alice 有 100 个 token
Alice approve Bob 30 个 token
Bob 调用 transferFrom(Alice, Carol, 20)
结果：Alice 减 20，Carol 加 20，Bob 的 allowance 剩 10
```

## 7. 最小 ERC20 示例

可以新建：

```text
contracts/study/StudyToken.sol
```

示例代码：

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";

contract StudyToken is ERC20 {
    constructor() ERC20("Study Token", "STUDY") {
        _mint(msg.sender, 1_000_000 * 10 ** decimals());
    }
}
```

说明：

```text
ERC20("Study Token", "STUDY") 设置名称和符号。
_mint(msg.sender, ...) 给部署者铸造初始供应量。
decimals() 默认是 18。
```

## 8. 带 owner 的 mint 示例

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

contract MintableStudyToken is ERC20, Ownable {
    constructor() ERC20("Mintable Study Token", "MST") Ownable(msg.sender) {
        _mint(msg.sender, 1_000_000 * 10 ** decimals());
    }

    function mint(address to, uint256 amount) external onlyOwner {
        _mint(to, amount);
    }
}
```

重点：

```text
OpenZeppelin 5.x 的 Ownable 构造函数需要 initialOwner。
所以这里写 Ownable(msg.sender)。
```

旧教程里常见的 `Ownable()` 写法，不适合当前 5.x 版本。

## 9. 重写 decimals 示例

有些 token 使用 6 位精度，例如 USDT 风格：

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";

contract SixDecimalToken is ERC20 {
    constructor() ERC20("Six Decimal Token", "SDT") {
        _mint(msg.sender, 1_000_000 * 10 ** decimals());
    }

    function decimals() public pure override returns (uint8) {
        return 6;
    }
}
```

注意：

```text
decimals 只是展示精度。
链上所有计算仍然使用 uint256 的最小单位。
```

## 10. 重写 _update 添加转账限制

例如，禁止向黑名单地址转账：

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

contract BlocklistToken is ERC20, Ownable {
    mapping(address account => bool blocked) public blocked;

    constructor() ERC20("Blocklist Token", "BLK") Ownable(msg.sender) {
        _mint(msg.sender, 1_000_000 * 10 ** decimals());
    }

    function setBlocked(address account, bool value) external onlyOwner {
        blocked[account] = value;
    }

    function _update(address from, address to, uint256 value) internal override {
        require(!blocked[from], "from blocked");
        require(!blocked[to], "to blocked");
        super._update(from, to, value);
    }
}
```

需要思考的问题：

```text
1. 这个限制会影响 transfer 吗？会。
2. 会影响 transferFrom 吗？会。
3. 会影响 mint 吗？会，因为 from 是 address(0)，to 是接收者。
4. 会影响 burn 吗？会，因为 to 是 address(0)，from 是销毁者。
```

如果不想限制 mint 或 burn，需要在 `_update` 里区分 `address(0)`。

## 11. ERC20 常见扩展

OpenZeppelin 已经提供很多扩展，不需要自己重新造：

| 扩展 | 用途 |
| --- | --- |
| `ERC20Burnable` | 允许持有人销毁自己的 token。 |
| `ERC20Pausable` | 暂停 token 转账、铸造、销毁。 |
| `ERC20Permit` | 支持 EIP-2612 签名授权。 |
| `ERC20Votes` | 治理投票快照能力。 |
| `ERC20Capped` | 限制总发行量上限。 |

学习建议：

```text
先会用 ERC20。
再学 ERC20Burnable 和 ERC20Pausable。
最后再学 ERC20Permit、ERC20Votes。
```

## 12. ERC20 测试应该覆盖什么

最少测试这些：

```text
1. name、symbol、decimals 是否正确。
2. 初始 totalSupply 是否正确。
3. 初始余额是否给到部署者。
4. transfer 成功路径。
5. transfer 余额不足会 revert。
6. approve 后 allowance 是否正确。
7. transferFrom 会扣 allowance。
8. 只有 owner 可以 mint。
9. 非 owner 调用 mint 会 revert。
```

如果你重写了 `_update()`，必须额外测试：

```text
1. transfer 是否被限制。
2. transferFrom 是否被限制。
3. mint 是否被限制。
4. burn 是否被限制。
5. 特殊地址 address(0) 是否处理正确。
```

## 13. 常见误区

### 13.1 decimals 会影响计算

错误理解：

```text
decimals 越大，token 越值钱。
```

正确理解：

```text
decimals 只是显示单位。
1 个 18 位 token 在链上写作 1 * 10^18。
1 个 6 位 token 在链上写作 1 * 10^6。
```

### 13.2 直接改 OpenZeppelin 源码

不要修改：

```text
node_modules/@openzeppelin/contracts/...
```

正确做法：

```text
继承 OpenZeppelin 合约，在自己的合约中扩展。
```

### 13.3 重写 transfer 而不是 _update

在 OpenZeppelin 5.x 中，很多统一转账限制应该写在 `_update()`。

如果只重写 `transfer()`，可能漏掉：

```text
transferFrom
mint
burn
扩展合约内部调用
```

### 13.4 忘记 super

重写 `_update()` 时，通常最后要调用：

```solidity
super._update(from, to, value);
```

否则余额不会真的改变。

## 14. 一句话总结

ERC20 的核心不是 `transfer()` 这一行，而是：

```text
余额存在 _balances。
授权存在 _allowances。
转账、铸造、销毁最终都进入 _update。
OpenZeppelin 5.x 中，定制 token 行为优先考虑重写 _update。
```

