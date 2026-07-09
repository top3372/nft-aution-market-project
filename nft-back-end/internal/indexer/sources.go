package indexer

import (
	"fmt"

	"nft-auction-backend/internal/config"
	"nft-auction-backend/internal/contract"
	"nft-auction-backend/internal/evmindexer"

	"github.com/ethereum/go-ethereum/core/types"
)

// SourceStores 是构建索引 source 时需要注入的业务仓储集合。
//
// cmd/indexer 负责创建真实 repository；测试可以注入内存 fake。这样 source 构建逻辑
// 既能复用配置，又不会和 GORM 或 RPC 客户端强绑定。
type SourceStores struct {
	Auctions AuctionStore
	Bids     BidStore
	NFTs     NFTStore
}

// BuildContractSources 根据 config.yaml 生成 evmindexer.Runner 可执行的合约 source。
//
// 配置层只描述“监听哪个合约、用哪份 ABI、从哪个区块开始”；这里负责把 ABI 名称翻译成
// 具体 decoder，把业务 source 翻译成对应 handler。这个映射是框架扩展点：后续新增
// ERC20 流水、NFT metadata 事件或其他市场事件时，只需要在这里增加一个 case。
func BuildContractSources(cfg config.Config, stores SourceStores) ([]evmindexer.ContractSource, error) {
	contracts := cfg.IndexerContracts
	if len(contracts) == 0 {
		contracts = defaultIndexerContracts(cfg)
	}

	sources := make([]evmindexer.ContractSource, 0, len(contracts))
	for _, item := range contracts {
		decoder, handler, err := decoderAndHandlerForContract(cfg, stores, item)
		if err != nil {
			return nil, err
		}
		sources = append(sources, evmindexer.ContractSource{
			Name:       item.Name,
			EventGroup: item.EventGroup,
			Address:    item.Address,
			StartBlock: item.StartBlock,
			Decoder:    decoder,
			Handler:    handler,
		})
	}
	return sources, nil
}

// defaultIndexerContracts 保持旧配置文件的可运行性。
//
// 如果部署环境还没有填写 indexer.contracts，后端默认监听市场代理合约和 AuctionNFT。
// 这比旧版只监听市场事件更完整：NFT mint 后可以同步到 nfts 表，“我的 NFT”接口不会
// 只能依赖前端临时扫链。
func defaultIndexerContracts(cfg config.Config) []config.IndexerContract {
	return []config.IndexerContract{
		{
			Name:       "market",
			EventGroup: "market",
			Address:    cfg.MarketAddress,
			ABI:        "auction_market_v3",
			StartBlock: cfg.StartBlock,
		},
		{
			Name:       "auction_nft",
			EventGroup: "auction_nft",
			Address:    cfg.AuctionNFTAddress,
			ABI:        "auction_nft",
			StartBlock: cfg.StartBlock,
		},
	}
}

// decoderAndHandlerForContract 把 ABI 名称转换成实际处理链路。
//
// 这里刻意使用 ABI 字段作为分支依据，而不是 source name。这样同一类 ABI 的合约
// 可以复用 decoder；source name 只用于日志、排错和配置阅读。
func decoderAndHandlerForContract(cfg config.Config, stores SourceStores, item config.IndexerContract) (evmindexer.Decoder, evmindexer.Handler, error) {
	switch item.ABI {
	case "auction_market_v3":
		return contractDecoder(contract.DecodeMarketLog), MarketEventHandler{
			MarketAddress: cfg.MarketAddress,
			NFTAddress:    cfg.AuctionNFTAddress,
			Auctions:      stores.Auctions,
			Bids:          stores.Bids,
		}, nil
	case "auction_nft":
		return contractDecoder(contract.DecodeAuctionNFTLog), NFTTransferHandler{
			NFTAddress: cfg.AuctionNFTAddress,
			NFTs:       stores.NFTs,
		}, nil
	case "erc20":
		return contractDecoder(contract.DecodeERC20TransferLog), NoopHandler{}, nil
	default:
		return nil, nil, fmt.Errorf("unsupported indexer contract abi %q for source %q", item.ABI, item.Name)
	}
}

// contractDecoder 是 contract.DecodedEvent 到 evmindexer.DecodedEvent 的适配器。
//
// contract 包只关心 ABI 解码，evmindexer 包只关心通用索引流程。通过这个 adapter，
// 两个包可以保持单向依赖，避免为了复用事件结构让底层合约包反向依赖框架包。
type contractDecoder func(chainID int64, log types.Log) (contract.DecodedEvent, bool, error)

func (d contractDecoder) Decode(chainID int64, log types.Log) (evmindexer.DecodedEvent, bool, error) {
	decoded, ok, err := d(chainID, log)
	if err != nil || !ok {
		return evmindexer.DecodedEvent{}, ok, err
	}
	return evmindexer.DecodedEvent{
		Name:        decoded.Name,
		ChainID:     decoded.ChainID,
		Contract:    decoded.Contract,
		TxHash:      decoded.TxHash,
		LogIndex:    decoded.LogIndex,
		BlockNumber: decoded.BlockNumber,
		BlockHash:   decoded.BlockHash,
		PayloadJSON: decoded.PayloadJSON,
		Payload:     decoded.Payload,
	}, true, nil
}
