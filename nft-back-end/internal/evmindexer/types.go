package evmindexer

import (
	"context"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
)

// Scope 描述一条索引游标的业务范围。
//
// 同一个后端可以同时索引多个合约、多个事件组，例如：
// - market: AuctionMarketV3 的拍卖和出价事件。
// - auction_nft: AuctionNFT 的 Transfer 事件。
// - payment_token: AuctionPaymentToken 的 Transfer 事件。
//
// 游标按 chain_id + contract_address + event_group 隔离，避免一个合约的同步进度
// 影响另一个合约，也方便以后按合约单独重放历史事件。
type Scope struct {
	ChainID         int64
	ContractAddress string
	EventGroup      string
}

// Cursor 是 EVM 索引器保存的同步进度。
//
// LastScannedBlock 是最后一个已经完整处理成功的区块。
// LastScannedBlockHash 是该区块当时的 hash，用于下一轮启动时检测轻量 reorg：
// 如果同一高度的链上 hash 已经变化，说明当前数据库里的派生状态可能来自旧分叉。
type Cursor struct {
	LastScannedBlock     uint64
	LastScannedBlockHash string
}

// DecodedEvent 是框架层和业务 handler 之间传递的统一事件对象。
//
// 框架只关心链上元数据和幂等字段；具体业务字段统一放在 Payload/PayloadJSON。
// 这样 AuctionMarket、AuctionNFT、ERC20 Transfer 等事件都能复用同一个 runner。
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

// EventRecord 是写入原始事件表的最小信息。
//
// 原始事件表是所有业务查询表的审计来源：即使 handler 后续逻辑调整，也可以通过
// tx_hash + log_index 找到当时处理过的链上事件。
type EventRecord struct {
	ChainID         int64
	ContractAddress string
	EventName       string
	TxHash          string
	LogIndex        uint
	BlockNumber     uint64
	BlockHash       string
	PayloadJSON     string
}

// FailedEventRecord 是索引死信记录。
//
// 当事件解码或业务 handler 失败时，runner 不推进游标，而是先把失败上下文写入
// dead-letter 表。这样后续排查时能看到失败发生在哪个阶段、哪条交易、哪个日志索引。
type FailedEventRecord struct {
	ChainID         int64
	ContractAddress string
	EventName       string
	TxHash          string
	LogIndex        uint
	BlockNumber     uint64
	BlockHash       string
	Stage           string
	ErrorMessage    string
	PayloadJSON     string
}

// ContractSource 是一个可索引合约的数据源定义。
//
// 业务上每个 source 代表一个合约地址和一组事件处理规则。框架负责拉日志、确认数、
// cursor 和失败事件；Decoder/Handler 负责 ABI 解码和业务表更新。
type ContractSource struct {
	Name       string
	EventGroup string
	Address    string
	StartBlock uint64
	Decoder    Decoder
	Handler    Handler
}

// ChainClient 抽象 go-ethereum 的 RPC 客户端能力，便于测试中注入内存 fake。
//
// BlockNumber 用于获取当前头块；HeaderByNumber 用于记录/校验 block hash；
// FilterLogs 用于按区块范围批量读取合约事件。
type ChainClient interface {
	BlockNumber(ctx context.Context) (uint64, error)
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
	FilterLogs(ctx context.Context, query ethereum.FilterQuery) ([]types.Log, error)
}

// CursorStore 保存和读取每个 source 的同步游标。
type CursorStore interface {
	GetCursor(ctx context.Context, scope Scope) (Cursor, error)
	SaveCursor(ctx context.Context, scope Scope, cursor Cursor) error
}

// EventStore 保存已解码的原始事件，并用 tx_hash + log_index 保证幂等。
type EventStore interface {
	InsertOnce(ctx context.Context, event EventRecord) (bool, error)
}

// FailedEventStore 保存解码失败或业务处理失败的事件上下文。
type FailedEventStore interface {
	InsertFailed(ctx context.Context, event FailedEventRecord) error
}

// Decoder 把 EVM 原始日志解析为业务可理解的事件。
//
// ok=false 表示该日志虽然来自被监听合约，但不是当前业务关心的事件，例如 ABI 中
// 存在的升级事件、OwnershipTransferred 等可以由 decoder 选择跳过。
type Decoder interface {
	Decode(chainID int64, log types.Log) (DecodedEvent, bool, error)
}

// DecoderFunc 让普通函数可以直接作为 Decoder 使用。
type DecoderFunc func(chainID int64, log types.Log) (DecodedEvent, bool, error)

func (f DecoderFunc) Decode(chainID int64, log types.Log) (DecodedEvent, bool, error) {
	return f(chainID, log)
}

// Handler 把已解码事件应用到业务查询表。
//
// Handler 应保持幂等：同一事件重复执行时不应产生重复业务数据。框架会先写原始事件
// 做第一层幂等，但业务表通常仍需要自己的唯一键保护。
type Handler interface {
	Handle(ctx context.Context, event DecodedEvent) error
}

// NormalizeScope 统一 scope 中用于唯一键比较的字符串。
//
// repository 层也需要相同规则来读写 sync_cursors；把规则放在框架包里，可以避免
// runner 和数据库仓储对 contract_address/event_group 的处理不一致。
func NormalizeScope(scope Scope) Scope {
	return Scope{
		ChainID:         scope.ChainID,
		ContractAddress: strings.ToLower(strings.TrimSpace(scope.ContractAddress)),
		EventGroup:      strings.TrimSpace(scope.EventGroup),
	}
}
