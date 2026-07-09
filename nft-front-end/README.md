# NFT Auction Frontend

React + Vite + TypeScript 前端，提供 NFT 拍卖列表、详情、创建拍卖、出价、取消和结束入口。

## 配置

复制 `.env.example` 并填写实际值：

```powershell
Copy-Item .env.example .env
```

需要配置 API 地址、Sepolia 网络参数、市场代理合约地址、AuctionNFT 地址和 ERC20 payment token 地址。

核心配置项：

- `VITE_API_BASE_URL`：Go REST API 地址。
- `VITE_NETWORK_NAME / VITE_CHAIN_ID / VITE_RPC_URL`：前端钱包网络校验和后续网络切换提示使用。
- `VITE_MARKET_ADDRESS`：`AuctionMarket` UUPS 代理地址，不是实现合约地址。
- `VITE_AUCTION_NFT_ADDRESS`：创建拍卖前需要授权的 NFT 合约地址。
- `VITE_PAYMENT_TOKEN_ADDRESS`：V3 出价使用的 ERC20 地址，需要和链上白名单一致。
- `VITE_BLOCK_EXPLORER_URL`：区块浏览器基础地址。

当前 Sepolia V3 地址：

- `VITE_MARKET_ADDRESS=0xBA9af325234368184A61be6081cdFB7f02dc6405`
- `VITE_AUCTION_NFT_ADDRESS=0x7913Ff1EaA12887ed80a3D35c81c0033FFafadCe`
- `AuctionMarketV3 implementation=0xb66F7BE4345C64e1f846D1AFE56196A24C85e58D`

前端写交易必须连接代理地址 `VITE_MARKET_ADDRESS`。实现合约地址仅作为升级记录，不直接给钱包调用。

## 开发

```powershell
npm install
npm run dev
```

## 验证

```powershell
npm run typecheck
npm run build
```
