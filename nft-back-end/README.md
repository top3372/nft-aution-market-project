# NFT Auction Backend

Go + MySQL 后端，负责索引 NFT 拍卖合约事件并提供 REST API。

## 配置

后端使用 Viper 读取 `config.yaml`。本地直接修改 `config.yaml` 即可启动：

```powershell
Copy-Item config.yaml config.local.yaml
$env:CONFIG_FILE="config.local.yaml"
```

如果不设置 `CONFIG_FILE`，程序默认读取当前工作目录下的 `config.yaml`。`.env.example` 只作为环境变量覆盖参考，不是主配置文件。

必须配置 MySQL、RPC URL 和合约地址。Redis 配置已预留，默认不启用。

核心配置项：

- `database.mysql_dsn`：MySQL 5.7 连接串。
- `redis.enabled / redis.addr / redis.password / redis.db`：Redis 预留配置。
- `chain.network_name / chain.chain_id / chain.rpc_url`：链网络名称、链 ID 和 RPC 地址。
- `contracts.market_address`：`AuctionMarket` UUPS 代理地址，indexer 按该地址过滤链上事件。
- `contracts.auction_nft_address`：本期业务支持的 NFT 合约地址。
- `contracts.payment_token_address`：V3 拍卖出价使用的 ERC20 地址。
- `chain.block_explorer_url`：区块浏览器基础地址，预留给交易链接和地址链接。
- `indexer.start_block`：首次同步起始区块。
- `indexer.batch_size / indexer.poll_interval`：indexer 每批扫描区块数和轮询间隔。
- `indexer.confirmations`：确认数，当前 Sepolia 配置为 6。
- `indexer.contracts`：多合约 source 配置，当前监听 `AuctionMarket`、`AuctionNFT` 和 `AuctionPaymentToken`。

环境变量覆盖规则：Viper 会把 YAML 路径中的 `.` 转成 `_` 并转大写，例如 `database.mysql_dsn` 可用 `DATABASE_MYSQL_DSN` 覆盖。旧变量名 `MYSQL_DSN / RPC_URL / MARKET_ADDRESS / AUCTION_NFT_ADDRESS` 也保留兼容。

当前 Sepolia V3 地址：

- `MARKET_ADDRESS=0xBA9af325234368184A61be6081cdFB7f02dc6405`
- `AUCTION_NFT_ADDRESS=0x7913Ff1EaA12887ed80a3D35c81c0033FFafadCe`
- `AuctionMarketV3 implementation=0xb66F7BE4345C64e1f846D1AFE56196A24C85e58D`

后端只需要配置代理地址 `MARKET_ADDRESS`。实现合约地址仅作为升级记录，不参与事件过滤。

`cmd/indexer` 使用项目内置 EVM 索引框架：

- `AuctionMarketV3` 事件更新拍卖列表、出价历史和状态。
- `AuctionNFT.Transfer` 事件更新 `nfts.owner_address`，用于“我的 NFT”和创建拍卖页选择 NFT。
- `AuctionPaymentToken.Transfer` 事件当前只写原始事件，页面余额仍直接读取链上 `balanceOf`。
- `sync_cursors.last_scanned_block_hash` 用于轻量 reorg 检测。
- `indexer_failed_events` 用于记录解码或业务落库失败事件。

## 数据库

MySQL 5.7 建表语句：

```text
migrations/001_init_mysql57.sql
migrations/001_init.sql
```

## 运行

```powershell
$env:CONFIG_FILE="config.yaml"
go run ./cmd/api
go run ./cmd/indexer
go run ./cmd/worker
```

## 验证

```powershell
go test ./...
go vet ./...
```
