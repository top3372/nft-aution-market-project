# 在当前 Hardhat 3 项目中练习 OpenZeppelin

本文档说明如何在当前项目中练习 OpenZeppelin。重点是把学习内容落到可编译、可测试的 Hardhat 工程里。

当前项目技术栈：

```text
Hardhat 3
ethers 6
Mocha + Chai
Solidity 0.8.28
@openzeppelin/contracts 5.6.1
@openzeppelin/contracts-upgradeable 5.6.1
```

## 1. 常用命令

Windows PowerShell 中常用：

```powershell
npm run build
npm test
npm run test:mocha
npm run test:solidity
npm run coverage
npm run typecheck
```

建议每次新增 Solidity 合约后执行：

```powershell
npm run build
npm run typecheck
```

如果新增了测试，再执行：

```powershell
npm test
```

## 2. 建议创建 study 目录

为了不影响现有拍卖市场合约，学习用合约可以放在：

```text
contracts/study/
test/study/
```

示例：

```text
contracts/study/StudyToken.sol
contracts/study/StudyVault.sol
test/study/StudyToken.ts
test/study/StudyVault.ts
```

这样学习代码和项目主业务代码边界清楚。

## 3. 练习一：ERC20 固定总量代币

新建：

```text
contracts/study/StudyToken.sol
```

代码：

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

编译：

```powershell
npm run build
```

你应该理解：

```text
ERC20 构造函数设置 name 和 symbol。
_mint 给部署者创建初始供应量。
decimals 默认是 18。
```

## 4. 练习二：给 ERC20 增加 owner mint

新建：

```text
contracts/study/MintableStudyToken.sol
```

代码：

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

对应测试建议：

```text
1. 部署者是 owner。
2. owner 可以 mint。
3. 非 owner 不能 mint。
4. mint 后 totalSupply 增加。
5. mint 后接收者余额增加。
```

## 5. Hardhat 3 + ethers 测试示例

新建：

```text
test/study/MintableStudyToken.ts
```

示例：

```ts
import { expect } from "chai";
import { network } from "hardhat";

const { ethers, networkHelpers } = await network.create();

describe("MintableStudyToken", function () {
  async function deployFixture() {
    const [owner, alice] = await ethers.getSigners();
    const token = await ethers.deployContract("MintableStudyToken");
    return { token, owner, alice };
  }

  it("sets the deployer as owner", async function () {
    const { token, owner } = await networkHelpers.loadFixture(deployFixture);

    expect(await token.owner()).to.equal(owner.address);
  });

  it("allows the owner to mint", async function () {
    const { token, alice } = await networkHelpers.loadFixture(deployFixture);

    await token.mint(alice.address, 100n);

    expect(await token.balanceOf(alice.address)).to.equal(100n);
  });

  it("rejects minting from a non-owner", async function () {
    const { token, alice } = await networkHelpers.loadFixture(deployFixture);

    await expect(token.connect(alice).mint(alice.address, 100n)).to.be.revertedWithCustomError(
      token,
      "OwnableUnauthorizedAccount",
    );
  });
});
```

运行：

```powershell
npm run test:mocha -- --grep MintableStudyToken
```

## 6. 练习三：AccessControl 多角色代币

新建：

```text
contracts/study/RoleToken.sol
```

代码：

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

测试重点：

```text
1. admin 拥有 DEFAULT_ADMIN_ROLE。
2. minter 可以 mint。
3. 非 minter 不能 mint。
4. admin 可以 grantRole。
5. revokeRole 后不能继续 mint。
```

## 7. 练习四：ReentrancyGuard 提款合约

新建：

```text
contracts/study/StudyVault.sol
```

代码：

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

contract StudyVault is ReentrancyGuard {
    mapping(address account => uint256 amount) public balances;

    function deposit() external payable {
        require(msg.value > 0, "zero deposit");
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

测试重点：

```text
1. deposit 后余额增加。
2. withdraw 后用户拿回 ETH。
3. withdraw 后 balances[msg.sender] 变成 0。
4. 没有余额时 withdraw revert。
5. 攻击合约尝试重入时失败。
```

## 8. 练习五：ERC721 托管

如果你要理解当前拍卖市场项目，必须掌握 ERC721 托管。

合约接收 NFT 时建议继承：

```solidity
import {ERC721Holder} from "@openzeppelin/contracts/token/ERC721/utils/ERC721Holder.sol";
```

学习流程：

```text
1. 部署一个 ERC721。
2. mint 一个 token 给 seller。
3. seller approve 托管合约。
4. 托管合约调用 safeTransferFrom 把 NFT 转到自己名下。
5. 检查 ownerOf(tokenId) 是否等于托管合约地址。
```

当前项目里的拍卖市场就是这个思路：

```text
卖家先 approve 市场。
市场 createAuction 时把 NFT 转入自己名下。
拍卖结束后再转给赢家或退回卖家。
```

## 9. 练习六：Upgradeable ERC20

升级合约使用 `@openzeppelin/contracts-upgradeable`。

示例：

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import {Initializable} from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import {ERC20Upgradeable} from "@openzeppelin/contracts-upgradeable/token/ERC20/ERC20Upgradeable.sol";
import {OwnableUpgradeable} from "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import {UUPSUpgradeable} from "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";

contract UpgradeableStudyToken is Initializable, ERC20Upgradeable, OwnableUpgradeable, UUPSUpgradeable {
    constructor() {
        _disableInitializers();
    }

    function initialize(address initialOwner) public initializer {
        __ERC20_init("Upgradeable Study Token", "UST");
        __Ownable_init(initialOwner);
        __UUPSUpgradeable_init();

        _mint(initialOwner, 1_000_000 * 10 ** decimals());
    }

    function mint(address to, uint256 amount) external onlyOwner {
        _mint(to, amount);
    }

    function _authorizeUpgrade(address newImplementation) internal override onlyOwner {}
}
```

重点：

```text
1. 不使用业务 constructor 初始化 name、symbol、owner。
2. 使用 initialize。
3. implementation constructor 只锁初始化器。
4. UUPS 必须实现 _authorizeUpgrade。
5. 升级时不能乱改旧状态变量顺序。
```

本项目已有完整 UUPS 示例，可以继续看：

```text
contracts/AuctionMarket.sol
contracts/AuctionMarketProxy.sol
contracts/AuctionMarketV2.sol
docs/PROXY_AND_UPGRADE_GUIDE.md
```

## 10. 推荐练习任务

按下面顺序练：

```text
任务 1：写 StudyToken，固定总量 ERC20。
验证：npm run build。

任务 2：写 MintableStudyToken，只有 owner 可以 mint。
验证：写 owner 和非 owner 测试。

任务 3：写 RoleToken，用 AccessControl 分 minter 和 burner。
验证：grantRole、revokeRole、onlyRole 测试。

任务 4：写 StudyVault，用 ReentrancyGuard 保护 withdraw。
验证：正常提款、重复提款、攻击合约重入。

任务 5：写 PausableStudyToken。
验证：pause 后 transfer 失败，unpause 后恢复。

任务 6：写 UpgradeableStudyToken。
验证：通过 proxy 初始化，升级后状态保留。
```

## 11. 每次练习后的复盘问题

完成一个练习后，回答这些问题：

```text
1. 我继承了哪些 OpenZeppelin 合约？
2. 每个父合约提供了什么能力？
3. 我使用了哪些 modifier？
4. 哪些函数是 public/external 入口？
5. 哪些函数是 internal 扩展点？
6. 哪些行为需要权限？
7. 哪些行为需要 revert 测试？
8. 是否有外部调用？是否需要防重入？
9. 如果是升级合约，是否使用 initialize？
10. 如果要升级，storage layout 是否安全？
```

## 12. 常见编译错误

### 12.1 Ownable 构造函数参数错误

错误：

```solidity
constructor() Ownable() {}
```

正确：

```solidity
constructor() Ownable(msg.sender) {}
```

原因：

```text
OpenZeppelin 5.x 的 Ownable 需要 initialOwner。
```

### 12.2 多继承 override 错误

如果同时继承 `ERC20` 和 `ERC20Pausable`，需要按编译器提示补齐 override：

```solidity
function _update(address from, address to, uint256 value) internal override(ERC20, ERC20Pausable) {
    super._update(from, to, value);
}
```

### 12.3 升级合约误用 constructor

错误：

```solidity
constructor() ERC20("Token", "TKN") {}
```

正确：

```solidity
function initialize(address initialOwner) public initializer {
    __ERC20_init("Token", "TKN");
    __Ownable_init(initialOwner);
}
```

## 13. 一句话总结

在当前项目学 OpenZeppelin，最有效的方法是：

```text
每学一个模块，就写一个最小合约。
每写一个合约，就写对应测试。
每次都运行 npm run build 和相关测试。
```

