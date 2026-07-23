# NFT Auction Market Project

本项目包含三个子工程：

- `nft-hardhat3-project`：Hardhat 3 合约工程，包含 `AuctionMarketV3` UUPS 升级合约。
- `nft-back-end`：Go + MySQL 后端，包含 `api`、`indexer`、`worker` 三个程序。
- `nft-front-end`：React + Vite + TypeScript 前端 DApp。

## Sepolia 合约地址

- `AuctionMarket` 代理地址：`0xBA9af325234368184A61be6081cdFB7f02dc6405`
- `AuctionNFT` 地址：`0x7913Ff1EaA12887ed80a3D35c81c0033FFafadCe`
- `AuctionMarketV3` 实现合约地址：`0xb66F7BE4345C64e1f846D1AFE56196A24C85e58D`
- 当前 Sepolia 正式支付 ERC20 `AuctionPaymentToken`：`0xBE3c38a1015b4B4dacfd13C5346F1Ee907D8c283`
- 当前支付币价格源：`0xC572743ed1781afCBCDb364480227C6a69512Cf3`
- V3 升级交易：`0xcdb3e8d5381f1162da5c543759cf48dfffcf8a41ada64549ce7fe966e3b9c2a1`

前后端运行时都应该填写 `AuctionMarket` 代理地址。`AuctionMarketV3` 实现合约地址只用于升级记录和链上核对，不作为前后端写交易入口。
当前 Sepolia 支付 ERC20 已切换为 `AuctionPaymentToken`，它只允许 owner mint。前端“我的”页面会读取当前钱包的 ERC20 `balanceOf` 余额；如果当前钱包是支付代币 owner，可以在页面中执行“管理员发放 1000 AUSD”。普通用户不能自行 mint，应通过 owner/多签/运营后台或充值流程受控发放。

## 合约验证

```powershell
Set-Location nft-hardhat3-project
npm test
npm run build
npm run typecheck
```

## V3 升级

当前 Sepolia 代理已经升级到 V3，`version()` 返回 `3.0.0`，`auctionNft()` 返回 `0x7913Ff1EaA12887ed80a3D35c81c0033FFafadCe`。

```powershell
Set-Location nft-hardhat3-project
$env:AUCTION_MARKET_PROXY="0xBA9af325234368184A61be6081cdFB7f02dc6405"
$env:AUCTION_NFT_ADDRESS="0x7913Ff1EaA12887ed80a3D35c81c0033FFafadCe"
npm run upgrade:auction:v3 -- --network sepolia
```

## 后端

配置文件：`nft-back-end/config.yaml`

后端需要填写这些运行配置：

- `database.mysql_dsn`：MySQL 5.7 连接串。
- `redis.enabled / redis.addr / redis.password / redis.db`：Redis 预留配置，第一版默认不启用。
- `chain.network_name / chain.chain_id / chain.rpc_url`：链网络名称、链 ID 和 Sepolia RPC 地址。
- `contracts.market_address`：`AuctionMarket` UUPS 代理地址，后端 indexer 只监听这个地址的事件。
- `contracts.auction_nft_address`：本期允许创建拍卖的 `AuctionNFT` 合约地址。
- `contracts.payment_token_address`：V3 出价使用的 ERC20 地址，需要和链上白名单配置一致。
- `chain.block_explorer_url`：区块浏览器基础地址，用于后续生成交易或地址跳转。
- `indexer.start_block`：indexer 首次同步起始区块。
- `indexer.confirmations`：indexer 等待的确认数，Sepolia 当前配置为 6。
- `indexer.contracts`：多合约 source 配置，当前包含市场、NFT 和支付代币。

后端使用 Viper 读取 `config.yaml`。部署时可以用 `CONFIG_FILE` 指定其他 YAML 文件，也可以用环境变量覆盖配置，例如 `DATABASE_MYSQL_DSN` 覆盖 `database.mysql_dsn`。

```powershell
Set-Location nft-back-end
go test ./...
go vet ./...
$env:CONFIG_FILE="config.yaml"
go run ./cmd/api
go run ./cmd/indexer
go run ./cmd/worker
```

MySQL 5.7 建表语句：

```text
nft-back-end/migrations/001_init_mysql57.sql
```

后端已预留 Redis 配置项，第一版默认不启用。

后端 indexer 已升级为内置 EVM 索引框架：市场事件更新拍卖和出价表，`AuctionNFT.Transfer` 更新 `nfts.owner_address`，支付代币 `Transfer` 先进入原始事件表便于后续扩展代币流水。详细设计见 `docs/EVM_INDEXER_FRAMEWORK_DESIGN.md`。

## 前端

配置文件示例：`nft-front-end/.env.example`

前端需要填写这些运行配置：

- `VITE_API_BASE_URL`：Go REST API 地址。
- `VITE_NETWORK_NAME / VITE_CHAIN_ID / VITE_RPC_URL`：钱包网络校验和后续网络切换提示使用。
- `VITE_MARKET_ADDRESS`：`AuctionMarket` UUPS 代理地址，所有创建拍卖、出价、取消、结束交易都通过它发起。
- `VITE_AUCTION_NFT_ADDRESS`：创建拍卖前授权的 NFT 合约地址。
- `VITE_PAYMENT_TOKEN_ADDRESS`：ERC20 出价代币地址，也用于“我的”页面查询余额和管理员发币。
- `VITE_BLOCK_EXPLORER_URL`：区块浏览器基础地址。

```powershell
Set-Location nft-front-end
npm install
npm run dev
npm run typecheck
npm run build
```

## 详细文档

- 工程总览与学习教程：`docs/NFT_AUCTION_ENGINEERING_TUTORIAL.md`
- 命令执行、编译启动时机与 go:embed：`docs/COMMANDS_COMPILE_STARTUP_AND_GO_EMBED.md`
- 业务链路与 ERC20 支付代币：`docs/NFT_AUCTION_BUSINESS_FLOW_AND_PAYMENT_TOKEN.md`
- 项目深度业务分析：`docs/NFT_AUCTION_PROJECT_DEEP_ANALYSIS.md`
- NFT 创建与拍卖操作：`docs/NFT_MINT_AND_AUCTION_OPERATION_GUIDE.md`
- ETH 主网兑换与 AUSD 发行量：`docs/ETH_MAINNET_TOKEN_EXCHANGE_AND_SUPPLY.md`
- EVM 索引服务框架：`docs/EVM_INDEXER_FRAMEWORK_DESIGN.md`
- Indexer 架构与 Reorg 对比：`docs/INDEXER_ARCHITECTURE_AND_REORG_COMPARISON.md`
- Ponder 索引服务设计与开发计划：`docs/PONDER_INDEXER_DESIGN_AND_DEVELOPMENT_PLAN.md`
- 合约文档：`nft-hardhat3-project/docs/AUCTION_MARKET_V3_GUIDE.md`
- 正式支付代币文档：`nft-hardhat3-project/docs/AUCTION_PAYMENT_TOKEN_GUIDE.md`
- 后端文档：`nft-back-end/docs/BACKEND_CODE_GUIDE.md`
- 前端文档：`nft-front-end/docs/FRONTEND_CODE_GUIDE.md`
