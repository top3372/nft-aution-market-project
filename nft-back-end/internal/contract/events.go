package contract

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

//go:embed abi/auction_market_v3.json
var auctionMarketABIJSON string

//go:embed abi/auction_nft.json
var auctionNFTABIJSON string

// DecodedEvent 是 indexer 在保存原始事件前使用的统一事件结构。
//
// 这里仍放在 contract 包中，是因为 ABI 解码属于“合约适配层”职责；上层 indexer
// 框架会再把它转换为 evmindexer.DecodedEvent，避免业务 handler 依赖 ABI 细节。
type DecodedEvent struct {
	Name        string
	ChainID     int64
	Contract    string
	TxHash      string
	LogIndex    uint
	BlockNumber uint64
	BlockHash   string
	PayloadJSON string
	Payload     map[string]string
}

var marketABI = mustParseABI(auctionMarketABIJSON)
var auctionNFTABI = mustParseABI(auctionNFTABIJSON)

// erc20TransferEventID 是标准 ERC20/ERC721 Transfer(address,address,uint256) 事件签名。
//
// ERC20 和 ERC721 的 Transfer 事件 topic0 相同，差异在第三个参数是否 indexed：
// - ERC721: tokenId 是 indexed，位于 topics[3]。
// - ERC20: amount 不是 indexed，位于 log.Data。
// 因此这里共用 topic0，但在不同解码函数里按各自 ABI 规则读取字段。
var erc20TransferEventID = common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")

// DecodeMarketLog 解析拍卖市场事件。
//
// AuctionMarketV3 是拍卖列表、出价历史和状态变更的主事件源。该 decoder 只做 ABI
// 到字符串 payload 的转换，不直接写数据库；数据库更新由 internal/indexer handler
// 完成，通用轮询和 cursor 由 internal/evmindexer 完成。
// 返回 ok=false 表示该日志不是当前后端关心的市场事件。
func DecodeMarketLog(chainID int64, log types.Log) (DecodedEvent, bool, error) {
	if len(log.Topics) == 0 {
		return DecodedEvent{}, false, nil
	}

	event, err := marketABI.EventByID(log.Topics[0])
	if err != nil {
		return DecodedEvent{}, false, nil
	}

	payload, ok, err := decodeKnownMarketEvent(*event, log)
	if err != nil || !ok {
		return DecodedEvent{}, ok, err
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return DecodedEvent{}, false, err
	}

	return DecodedEvent{
		Name:        event.Name,
		ChainID:     chainID,
		Contract:    strings.ToLower(log.Address.Hex()),
		TxHash:      strings.ToLower(log.TxHash.Hex()),
		LogIndex:    log.Index,
		BlockNumber: log.BlockNumber,
		BlockHash:   strings.ToLower(log.BlockHash.Hex()),
		PayloadJSON: string(payloadJSON),
		Payload:     payload,
	}, true, nil
}

// DecodeAuctionNFTLog 解析 AuctionNFT 的资产归属事件。
//
// 后端“我的 NFT”接口依赖 nfts 表的 owner_address。如果用户刚在前端 safeMint，
// 只有市场合约事件还不足以知道 NFT 归属；监听 ERC721 Transfer 后，mint、普通转账、
// 创建拍卖时转入市场合约、取消/结束拍卖时转出市场合约都能更新本地 owner 缓存。
func DecodeAuctionNFTLog(chainID int64, log types.Log) (DecodedEvent, bool, error) {
	if len(log.Topics) == 0 {
		return DecodedEvent{}, false, nil
	}

	event, err := auctionNFTABI.EventByID(log.Topics[0])
	if err != nil {
		return DecodedEvent{}, false, nil
	}
	if event.Name != "Transfer" {
		return DecodedEvent{}, false, nil
	}
	if len(log.Topics) < 4 {
		return DecodedEvent{}, false, fmt.Errorf("auction nft transfer log requires 4 topics, got %d", len(log.Topics))
	}

	payload := map[string]string{
		"from":     topicAddress(log.Topics[1]),
		"to":       topicAddress(log.Topics[2]),
		"token_id": topicUint(log.Topics[3]),
	}
	return decodedEventFromPayload(chainID, log, event.Name, payload)
}

// DecodeERC20TransferLog 解析标准 ERC20 Transfer 事件。
//
// 第一版业务表不按 ERC20 Transfer 建余额快照，页面余额仍直接读链；但框架支持把
// AuctionPaymentToken 配为独立 source 后写入 raw event/failed event，便于后续扩展
// 管理员发币流水、用户充值流水或余额缓存。
func DecodeERC20TransferLog(chainID int64, log types.Log) (DecodedEvent, bool, error) {
	if len(log.Topics) == 0 || log.Topics[0] != erc20TransferEventID {
		return DecodedEvent{}, false, nil
	}
	if len(log.Topics) < 3 {
		return DecodedEvent{}, false, fmt.Errorf("erc20 transfer log requires 3 topics, got %d", len(log.Topics))
	}

	amount := new(big.Int)
	if len(log.Data) > 0 {
		amount.SetBytes(log.Data)
	}
	payload := map[string]string{
		"from":   topicAddress(log.Topics[1]),
		"to":     topicAddress(log.Topics[2]),
		"amount": amount.String(),
	}
	return decodedEventFromPayload(chainID, log, "Transfer", payload)
}

// decodeKnownMarketEvent 只选择当前后端已知或需要审计的市场事件。
//
// ABI 中可能包含升级、所有权或其他暂不参与业务查询的事件；这些事件返回 ok=false，
// Runner 会跳过 handler。对 FeeConfigUpdated、PaymentTokenUpdated 等事件先写 raw
// event，后续需要后台管理查询时再补业务表。
func decodeKnownMarketEvent(event abi.Event, log types.Log) (map[string]string, bool, error) {
	switch event.Name {
	case "AuctionCreatedV3":
		return decodeAuctionCreatedV3(event, log)
	case "AuctionCreated":
		return decodeAuctionCreated(event, log)
	case "BidPlaced":
		return decodeBidPlaced(event, log)
	case "AuctionCancelled":
		return decodeAuctionCancelled(event, log)
	case "AuctionEnded":
		return decodeAuctionEnded(event, log)
	case "AuctionSettledWithFees", "FeeConfigUpdated", "PaymentTokenUpdated":
		return decodeGeneric(event, log)
	default:
		return nil, false, nil
	}
}

// decodeAuctionCreatedV3 解析 V3 创建拍卖事件。
//
// auction_id、seller、token_id 是 indexed 参数，位于 topics；startTime、endTime、
// startingPriceUsd 是非 indexed 参数，位于 log.Data。
func decodeAuctionCreatedV3(event abi.Event, log types.Log) (map[string]string, bool, error) {
	values, err := unpackEvent(event, log)
	if err != nil {
		return nil, false, err
	}
	return map[string]string{
		"auction_id":         topicUint(log.Topics[1]),
		"seller":             topicAddress(log.Topics[2]),
		"token_id":           topicUint(log.Topics[3]),
		"start_time":         valueString(values["startTime"]),
		"end_time":           valueString(values["endTime"]),
		"starting_price_usd": valueString(values["startingPriceUsd"]),
	}, true, nil
}

// decodeAuctionCreated 解析旧版兼容创建事件。
//
// 保留该 decoder 是为了历史事件或旧测试兼容；V3 查询表主要依赖 AuctionCreatedV3。
func decodeAuctionCreated(event abi.Event, log types.Log) (map[string]string, bool, error) {
	values, err := unpackEvent(event, log)
	if err != nil {
		return nil, false, err
	}
	return map[string]string{
		"auction_id": topicUint(log.Topics[1]),
		"seller":     topicAddress(log.Topics[2]),
		"nft":        strings.ToLower(valueString(values["nft"])),
		"token_id":   valueString(values["tokenId"]),
	}, true, nil
}

// decodeBidPlaced 解析出价事件。
//
// payment_token 是 indexed 地址，amount 和 amountUsd 在 data 中。金额统一转成十进制
// 字符串，避免 Go/数据库整数类型无法安全承载 uint256。
func decodeBidPlaced(event abi.Event, log types.Log) (map[string]string, bool, error) {
	values, err := unpackEvent(event, log)
	if err != nil {
		return nil, false, err
	}
	return map[string]string{
		"auction_id":    topicUint(log.Topics[1]),
		"bidder":        topicAddress(log.Topics[2]),
		"payment_token": topicAddress(log.Topics[3]),
		"amount":        valueString(values["amount"]),
		"amount_usd":    valueString(values["amountUsd"]),
	}, true, nil
}

// decodeAuctionCancelled 解析取消拍卖事件。
//
// 取消事件只需要 auction_id 就能定位业务表；seller/token_id 保存在 payload 中，便于
// 排错或后续扩展取消记录 API。
func decodeAuctionCancelled(event abi.Event, log types.Log) (map[string]string, bool, error) {
	return map[string]string{
		"auction_id": topicUint(log.Topics[1]),
		"seller":     topicAddress(log.Topics[2]),
		"token_id":   topicUint(log.Topics[3]),
	}, true, nil
}

// decodeAuctionEnded 解析结束拍卖事件。
//
// 当前 handler 用 auction_id 标记终态；winner/payment_token/amount 先保存在 raw event，
// 后续如需成交流水或结算报表，可以从该 payload 扩展业务表。
func decodeAuctionEnded(event abi.Event, log types.Log) (map[string]string, bool, error) {
	values, err := unpackEvent(event, log)
	if err != nil {
		return nil, false, err
	}
	return map[string]string{
		"auction_id":    topicUint(log.Topics[1]),
		"winner":        topicAddress(log.Topics[2]),
		"payment_token": topicAddress(log.Topics[3]),
		"amount":        valueString(values["amount"]),
	}, true, nil
}

// decodeGeneric 解析暂时不进入查询表但需要审计的事件。
//
// 例如手续费配置、支付代币配置变化，对前端拍卖列表不是实时必要字段，但保留 raw
// event 后可以排查“某次出价为什么使用这个 token/费率”。
func decodeGeneric(event abi.Event, log types.Log) (map[string]string, bool, error) {
	values, err := unpackEvent(event, log)
	if err != nil {
		return nil, false, err
	}
	payload := make(map[string]string, len(values))
	for key, value := range values {
		payload[key] = valueString(value)
	}
	return payload, true, nil
}

// decodedEventFromPayload 把已经按 ABI 规则解析出的字段统一包装为 DecodedEvent。
//
// 不同合约事件最终都需要 tx_hash、log_index、block_number 等链上元数据来做幂等、
// 查询和排错，因此这里集中填充，避免每个 decoder 重复写一份容易漏字段的代码。
func decodedEventFromPayload(chainID int64, log types.Log, name string, payload map[string]string) (DecodedEvent, bool, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return DecodedEvent{}, false, err
	}
	return DecodedEvent{
		Name:        name,
		ChainID:     chainID,
		Contract:    strings.ToLower(log.Address.Hex()),
		TxHash:      strings.ToLower(log.TxHash.Hex()),
		LogIndex:    log.Index,
		BlockNumber: log.BlockNumber,
		BlockHash:   strings.ToLower(log.BlockHash.Hex()),
		PayloadJSON: string(payloadJSON),
		Payload:     payload,
	}, true, nil
}

// unpackEvent 解包非 indexed 参数。
//
// indexed 参数不会出现在 log.Data，而是写入 topics；具体事件 decoder 会结合 topics
// 和本函数返回的 data 字段组成完整 payload。
func unpackEvent(event abi.Event, log types.Log) (map[string]any, error) {
	values := make(map[string]any)
	if len(log.Data) > 0 {
		if err := event.Inputs.UnpackIntoMap(values, log.Data); err != nil {
			return nil, err
		}
	}
	return values, nil
}

// topicUint 把 indexed uint256 topic 转成十进制字符串。
func topicUint(topic common.Hash) string {
	return new(big.Int).SetBytes(topic.Bytes()).String()
}

// topicAddress 把 indexed address topic 转成小写 0x 地址。
//
// EVM topic 是 32 字节，address 只占低 20 字节，所以需要截取后 20 字节。
func topicAddress(topic common.Hash) string {
	return strings.ToLower(common.BytesToAddress(topic.Bytes()[12:]).Hex())
}

// valueString 把 ABI 解包后的常见类型统一转成字符串。
//
// 后端 raw event payload 使用 map[string]string，是为了让 MySQL 5.7 LONGTEXT JSON、
// API 输出和排错日志都保持简单稳定。
func valueString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case common.Address:
		return strings.ToLower(typed.Hex())
	case *big.Int:
		return typed.String()
	case uint64:
		return fmt.Sprintf("%d", typed)
	case uint16:
		return fmt.Sprintf("%d", typed)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", typed)
	}
}

// mustParseABI 在进程启动时解析内嵌 ABI。
//
// ABI 是随代码发布的静态资源，如果解析失败说明构建产物本身有问题，直接 panic 比
// 运行时静默跳过事件更安全。
func mustParseABI(input string) abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(input))
	if err != nil {
		panic(err)
	}
	return parsed
}
