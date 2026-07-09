# NFT Auction Market

这是一个 Hardhat 3 + ethers 的 NFT 拍卖市场示例项目，支持 ERC721 NFT 拍卖、ETH/ERC20 出价、Chainlink Price Feed 风格的 USD 报价比较，以及 UUPS 代理升级。


## 项目结构

```text
contracts/
  AuctionNFT.sol                         ERC721 NFT 合约，支持铸造和转移
  AuctionMarket.sol                      UUPS 可升级拍卖市场
  AuctionMarketProxy.sol                 ERC1967/UUPS 代理包装合约
  AuctionMarketV2.sol                    升级测试用 V2 实现
  AuctionPaymentToken.sol                正式 ERC20 支付代币
  mocks/MockV3Aggregator.sol             测试用价格源
ignition/modules/
  AuctionMarket.ts                       NFT + 拍卖市场代理部署模块
test/
  AuctionMarket.ts                       拍卖、价格换算、结算、升级测试
```

## 功能说明

`AuctionNFT` 使用 OpenZeppelin ERC721URIStorage 实现 NFT 集合。合约 owner 可以调用 `safeMint(to, uri)` 铸造 NFT，并通过标准 ERC721 `approve` / `transferFrom` / `safeTransferFrom` 完成授权和转移。

`AuctionMarket` 使用 UUPS 升级模式。部署时先部署实现合约，再通过 `AuctionMarketProxy` 调用 `initialize(owner)` 初始化代理存储。只有 owner 可以调用升级入口。市场创建拍卖时会托管卖家的 NFT；出价者可以使用 ETH 或已启用的 ERC20 出价；每次出价都会通过 `AggregatorV3Interface.latestRoundData()` 读取 token/USD 价格，并统一换算成 8 位 USD 数值进行比较。

结算规则：

- 新出价的 USD 价值必须高于当前最高出价。
- 新最高价出现后，旧最高出价会立即退回原出价者。
- 拍卖结束后，NFT 转给最高价者，资金转给卖家。
- 如果无人出价，NFT 退回卖家。

## Chainlink Price Feed

合约通过 `@chainlink/contracts` 引入官方 `AggregatorV3Interface`，路径为 `@chainlink/contracts/src/v0.8/shared/interfaces/AggregatorV3Interface.sol`。部署到测试网时，直接把真实 feed 地址配置给 `setPaymentToken(token, feed, tokenDecimals, enabled)`。

示例：

- ETH 使用 `token = 0x0000000000000000000000000000000000000000`，`tokenDecimals = 18`。
- Sepolia ETH/USD feed 默认参数为 `0x694AA1769357215DE4FAC081bf1f309aDC325306`。
- ERC20 需要传入该 ERC20 对 USD 的 feed 地址和 token 自身 decimals。

本项目已引入完整 `@chainlink/contracts` 包，不再维护本地 Chainlink Price Feed 接口。

## 安装与测试

```powershell
npm install
Copy-Item .env.example .env
npm run build
npm test
npm run typecheck
```

也可以单独运行 Mocha 集成测试：

```powershell
npm run test:mocha -- --grep AuctionMarket
```

覆盖率命令：

```powershell
npm run coverage
```

## 测试报告

当前已覆盖：

- NFT 铸造、授权后创建拍卖，并由市场托管 NFT。
- ETH 和 ERC20 出价换算为 8 位 USD。
- ETH/ERC20 跨资产出价比较。
- 低价出价拒绝。
- 旧最高出价退款。
- 拍卖结束后 NFT 和资金结算。
- 无出价时 NFT 退回卖家。
- 不支持 token 出价拒绝。
- UUPS 升级后保留 V1 拍卖状态。

最近一次本地测试结果：

```text
AuctionMarket: 7 passing
All tests: 12 passing (3 solidity, 9 mocha)
Coverage: total line 87.50%, total statement 74.14%
AuctionMarket.sol: line 86.24%, statement 70.10%
```

HTML 覆盖率报告输出在 `coverage/html`，LCOV 文件输出在 `coverage/lcov.info`。

## 部署参数

部署相关参数统一放在本地 `.env` 中，模板见 `.env.example`。`.env` 不应提交到 git。环境和命令的对应关系如下：

| 发布环境 | Hardhat network | npm 命令 | 主要修改位置 | 用途 |
| --- | --- | --- | --- | --- |
| 本地 Anvil | `anvil` | `npm run deploy:anvil` | `.env` 的 `ANVIL_*` 和 `AUCTION_*` | 本地持久节点联调 |
| Sepolia 测试网 | `sepolia` | `npm run deploy:sepolia` | `.env` 的 `SEPOLIA_*` 和 `AUCTION_*` | 测试网部署验证 |
| Ethereum 主网生产 | `mainnet` | `npm run deploy:mainnet` | `.env` 的 `MAINNET_*` 和 `AUCTION_*` | 生产公网部署 |

代码中的配置入口：

- 网络 RPC、chain id、部署私钥：改 `hardhat.config.ts` 的 `networks`，日常只需要改 `.env`。
- NFT 名称、符号、ETH/USD feed：改 `.env`，读取逻辑在 `ignition/modules/AuctionMarket.ts`。
- 部署命令：改 `package.json` 的 `scripts`。
- 真实私钥和 RPC：只写本地 `.env`，不要写进 README、`.env.example` 或源码。

```powershell
Copy-Item .env.example .env
```

常用参数：

```dotenv
AUCTION_NFT_NAME="Auction NFT"
AUCTION_NFT_SYMBOL=ANFT

SEPOLIA_RPC_URL=
SEPOLIA_PRIVATE_KEY=
SEPOLIA_ETH_USD_FEED=0x694AA1769357215DE4FAC081bf1f309aDC325306

MAINNET_RPC_URL=
MAINNET_PRIVATE_KEY=
MAINNET_ETH_USD_FEED=0x5f4eC3Df9cbd43714FE2740f5E3616155c5b8419

ANVIL_RPC_URL=http://127.0.0.1:8545
ANVIL_ETH_USD_FEED=0x694AA1769357215DE4FAC081bf1f309aDC325306
```

`AUCTION_ETH_USD_FEED` 是可选的全局覆盖项；留空时，部署模块会按 `--network` 自动选择 `SEPOLIA_ETH_USD_FEED`、`MAINNET_ETH_USD_FEED` 或 `ANVIL_ETH_USD_FEED`。未传 `--network` 的本地 Ignition 工具命令默认使用 `ANVIL_ETH_USD_FEED`。

## 部署到 Sepolia

先在 `.env` 中填写：

```dotenv
SEPOLIA_RPC_URL=
SEPOLIA_PRIVATE_KEY=
SEPOLIA_ETH_USD_FEED=0x694AA1769357215DE4FAC081bf1f309aDC325306
```

部署：

```powershell
npm run deploy:sepolia
```

如需临时覆盖 NFT 名称、符号或 ETH/USD feed，仍可使用 Ignition 参数文件；参数文件优先级高于 `.env` 中的部署默认值。

## 部署到 Ethereum Mainnet

这是生产 ETH 公网发布环境，对应 `hardhat.config.ts` 里的 `mainnet` 网络和 `package.json` 里的 `deploy:mainnet` 脚本。

先在 `.env` 中填写：

```dotenv
MAINNET_RPC_URL=
MAINNET_PRIVATE_KEY=
MAINNET_ETH_USD_FEED=0x5f4eC3Df9cbd43714FE2740f5E3616155c5b8419
```

如果要修改生产发布参数，改这些位置：

- 生产 RPC：`.env` 的 `MAINNET_RPC_URL`。
- 生产部署账户：`.env` 的 `MAINNET_PRIVATE_KEY`。
- 生产 ETH/USD 价格源：`.env` 的 `MAINNET_ETH_USD_FEED`。
- NFT 名称和符号：`.env` 的 `AUCTION_NFT_NAME`、`AUCTION_NFT_SYMBOL`。
- 网络名、chain id 或账户读取方式：`hardhat.config.ts` 的 `mainnet` 配置。
- 部署模块逻辑或构造参数：`ignition/modules/AuctionMarket.ts`。

部署：

```powershell
npm run deploy:mainnet
```

主网部署会消耗真实 ETH。执行前请确认：

- `MAINNET_RPC_URL` 指向 Ethereum mainnet，不是 Sepolia、Anvil 或其他链。
- `MAINNET_PRIVATE_KEY` 是专门用于部署的生产钱包，并且有足够 ETH 支付 gas。
- `MAINNET_ETH_USD_FEED` 是 Ethereum mainnet 的 ETH/USD feed。
- `AUCTION_NFT_NAME` 和 `AUCTION_NFT_SYMBOL` 是最终生产值。
- Ignition 输出的 owner/deployer 地址符合预期，再确认部署提示。

本地模拟链部署验证：

```powershell
npx hardhat ignition deploy ignition/modules/AuctionMarket.ts --network hardhatMainnet
```

`hardhatMainnet` 是单次命令内的临时链，命令结束后状态会丢失。如果要先部署再运行升级脚本，请开一个持久本地节点：

```powershell
npx hardhat node --network hardhatMainnet
```

然后在另一个 PowerShell 窗口使用 `--network localhost` 部署和升级。

## 部署到本地 Anvil

先启动本地 Anvil。默认 RPC 地址是 `http://127.0.0.1:8545`，chain id 为 `31337`：

```powershell
anvil --host 127.0.0.1 --port 8545 --chain-id 31337
```

如果 Anvil 使用了其他 RPC 地址，可以在部署前覆盖：

```powershell
$env:ANVIL_RPC_URL="http://127.0.0.1:8545"
```

部署到 Anvil：

```powershell
npm run deploy:anvil
```

等价的 Hardhat 命令：

```powershell
npx hardhat --network anvil ignition deploy --deployment-id anvil-local ignition/modules/AuctionMarket.ts
```

`--deployment-id anvil-local` 会把 Anvil 部署记录和已有 `chain-31337` 本地模拟链记录分开，避免两个同为 chain id `31337` 的本地网络共用同一份 Ignition journal。

当前部署模块只把 ETH/USD feed 地址写入市场配置，不会在部署阶段读取价格。若要在 Anvil 上完整测试出价报价，请先部署本地 mock price feed，再用 owner 调用 `setPaymentToken(token, feed, tokenDecimals, enabled)` 把 feed 更新为 Anvil 上的 mock 地址。

## 升级合约

升级脚本会部署 `AuctionMarketV2`，然后调用代理上的 `upgradeToAndCall`。真实网络升级前必须确认当前账号是市场 owner。

PowerShell 示例：

```powershell
$env:AUCTION_MARKET_PROXY="<deployed-proxy-address>"
npm run upgrade:auction -- --network sepolia
```

本地模拟链示例：

```powershell
$env:AUCTION_MARKET_PROXY="<local-proxy-address>"
npm run upgrade:auction -- --network localhost
```

## 部署地址

当前仓库已提供部署模块，但本地没有真实 Sepolia RPC 和私钥，因此没有执行测试网部署。实际部署后请记录：

```text
AuctionNFT:
AuctionMarket implementation:
AuctionMarket proxy:
AuctionPaymentToken: 0xBE3c38a1015b4B4dacfd13C5346F1Ee907D8c283
```

