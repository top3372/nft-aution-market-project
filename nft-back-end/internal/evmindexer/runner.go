package evmindexer

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// ErrReorgDetected 表示索引器发现已处理区块的 hash 和当前链上 hash 不一致。
//
// 当前实现先选择“发现即停止”的保守策略：不自动删除业务表数据，避免误删已经被
// API 使用的数据。后续如果需要自动回滚，可以在这个错误上层接入按 block_number
// 删除 raw event 和派生表的补偿逻辑。
var ErrReorgDetected = errors.New("evm reorg detected")

// Runner 是项目内置 EVM 索引框架的调度器。
//
// 它解决所有 EVM 索引服务都需要面对的通用问题：
// - 多合约 source 配置。
// - 按区块范围 getLogs。
// - 确认数保护，避免直接处理最新头块。
// - raw event 幂等写入。
// - handler 失败时写入 dead letter，并且不推进游标。
// - 保存区块 hash，用于下一轮轻量 reorg 检测。
//
// 它不包含任何 NFT 拍卖业务规则。拍卖、NFT、支付代币等业务逻辑放在各自 handler。
type Runner struct {
	ChainID       int64
	Client        ChainClient
	BatchSize     uint64
	Confirmations uint64
	Cursors       CursorStore
	Events        EventStore
	FailedEvents  FailedEventStore
	Sources       []ContractSource
}

// RunOnce 对所有 configured sources 各执行一轮同步。
//
// 该方法适合被 cmd/indexer 的 ticker 循环调用。它每轮最多处理每个 source 的一个批次，
// 这样不会因为某个历史区间日志很多而长时间阻塞其他 source。
func (r *Runner) RunOnce(ctx context.Context) error {
	for _, source := range r.Sources {
		if err := r.runSourceOnce(ctx, source); err != nil {
			return err
		}
	}
	return nil
}

// runSourceOnce 同步单个合约 source 的一个区块批次。
//
// 一个批次的完整业务顺序是：
// 1. 校验 source 配置，避免启动后才发现 ABI 或 handler 没接上。
// 2. 读取 cursor，并用保存的 block hash 做 reorg 检测。
// 3. 计算 safe head，只处理达到确认数的区块。
// 4. getLogs 拉取事件，逐条解码、写 raw event、执行业务 handler。
// 5. 所有日志处理成功后，保存 toBlock 和 toBlock hash。
//
// 只要中间任一步失败，本方法就返回错误且不推进 cursor；下一轮会从同一区块继续重试。
func (r *Runner) runSourceOnce(ctx context.Context, source ContractSource) error {
	if err := validateSource(source); err != nil {
		return err
	}

	scope := NormalizeScope(Scope{
		ChainID:         r.ChainID,
		ContractAddress: source.Address,
		EventGroup:      source.EventGroup,
	})
	cursor, err := r.Cursors.GetCursor(ctx, scope)
	if err != nil {
		return err
	}
	if err := r.ensureCursorStillCanonical(ctx, cursor); err != nil {
		return err
	}

	head, err := r.Client.BlockNumber(ctx)
	if err != nil {
		return err
	}
	safeBlock, ok := safeHead(head, r.Confirmations)
	if !ok {
		// 当前链头还没有达到配置的确认数，直接等待下一轮，避免处理可能被回滚的区块。
		return nil
	}

	fromBlock := source.StartBlock
	if cursor.LastScannedBlock >= fromBlock {
		fromBlock = cursor.LastScannedBlock + 1
	}
	if fromBlock > safeBlock {
		return nil
	}

	// 每轮只处理一个 batch，避免历史补数据时某个 source 长时间占用进程。
	toBlock := fromBlock + r.batchSize() - 1
	if toBlock > safeBlock {
		toBlock = safeBlock
	}

	logs, err := r.Client.FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(fromBlock)),
		ToBlock:   big.NewInt(int64(toBlock)),
		Addresses: []common.Address{common.HexToAddress(source.Address)},
	})
	if err != nil {
		return err
	}

	for _, raw := range logs {
		decoded, ok, err := source.Decoder.Decode(r.ChainID, raw)
		if err != nil {
			_ = r.recordFailure(ctx, decodedFromRaw(r.ChainID, raw), "decode", err)
			return err
		}
		if !ok {
			// decoder 明确表示“不关心该事件”时，只跳过该日志，不影响批次推进。
			continue
		}

		inserted, err := r.Events.InsertOnce(ctx, eventRecordFromDecoded(decoded))
		if err != nil {
			_ = r.recordFailure(ctx, decoded, "raw_event", err)
			return err
		}
		if !inserted {
			// raw event 已存在说明该日志已经被处理过，跳过 handler，保证重启后不会重复落业务数据。
			continue
		}
		if err := source.Handler.Handle(ctx, decoded); err != nil {
			_ = r.recordFailure(ctx, decoded, "handler", err)
			return err
		}
	}

	header, err := r.Client.HeaderByNumber(ctx, big.NewInt(int64(toBlock)))
	if err != nil {
		return err
	}
	return r.Cursors.SaveCursor(ctx, scope, Cursor{
		LastScannedBlock:     toBlock,
		LastScannedBlockHash: strings.ToLower(header.Hash().Hex()),
	})
}

// ensureCursorStillCanonical 用保存的 cursor block hash 做轻量链重组检测。
//
// Sepolia 或其他 EVM 链发生短 reorg 时，同一个 block number 可能对应不同 hash。
// 如果数据库里已经基于旧 hash 生成了业务表，继续向后同步会让查询表混入两个分叉
// 的数据。当前策略是发现后立即返回 ErrReorgDetected，由运维或后续补偿任务决定
// 是否回滚到更早区块重新索引。
func (r *Runner) ensureCursorStillCanonical(ctx context.Context, cursor Cursor) error {
	if cursor.LastScannedBlock == 0 || strings.TrimSpace(cursor.LastScannedBlockHash) == "" {
		return nil
	}

	header, err := r.Client.HeaderByNumber(ctx, big.NewInt(int64(cursor.LastScannedBlock)))
	if err != nil {
		return err
	}
	if !strings.EqualFold(header.Hash().Hex(), cursor.LastScannedBlockHash) {
		return fmt.Errorf("%w: cursor block %d stored hash %s current hash %s",
			ErrReorgDetected,
			cursor.LastScannedBlock,
			cursor.LastScannedBlockHash,
			strings.ToLower(header.Hash().Hex()),
		)
	}
	return nil
}

// recordFailure 把解码、raw event 写入或 handler 失败记录到 dead-letter 表。
//
// 失败记录本身不能影响原错误返回：如果 dead-letter 写入也失败，Runner 仍应该返回
// 最初的业务错误，并且保持 cursor 不推进。调用方目前忽略这里的返回值，原因是
// dead-letter 是排错辅助，不应掩盖真正导致同步中断的错误。
func (r *Runner) recordFailure(ctx context.Context, event DecodedEvent, stage string, cause error) error {
	if r.FailedEvents == nil {
		return nil
	}
	return r.FailedEvents.InsertFailed(ctx, FailedEventRecord{
		ChainID:         event.ChainID,
		ContractAddress: event.Contract,
		EventName:       event.Name,
		TxHash:          event.TxHash,
		LogIndex:        event.LogIndex,
		BlockNumber:     event.BlockNumber,
		BlockHash:       event.BlockHash,
		Stage:           stage,
		ErrorMessage:    cause.Error(),
		PayloadJSON:     event.PayloadJSON,
	})
}

// batchSize 返回实际使用的区块批次大小。
//
// 配置缺省时使用 1000，适合小型 DApp 在 Sepolia 上补历史；如果 RPC 有日志数量限制，
// 可以在 config.yaml 中调小 indexer.batch_size。
func (r *Runner) batchSize() uint64 {
	if r.BatchSize == 0 {
		return 1000
	}
	return r.BatchSize
}

// validateSource 在每轮执行前校验 source 必填项。
//
// 这里保留运行时校验，是因为 source 可能来自配置文件、测试 fake 或后续动态构建；
// 尽早返回清晰错误比让 nil decoder/handler panic 更容易排查。
func validateSource(source ContractSource) error {
	if strings.TrimSpace(source.Name) == "" {
		return errors.New("evm indexer source name is required")
	}
	if strings.TrimSpace(source.EventGroup) == "" {
		return fmt.Errorf("evm indexer source %s event_group is required", source.Name)
	}
	if !common.IsHexAddress(source.Address) {
		return fmt.Errorf("evm indexer source %s address is invalid", source.Name)
	}
	if source.Decoder == nil {
		return fmt.Errorf("evm indexer source %s decoder is required", source.Name)
	}
	if source.Handler == nil {
		return fmt.Errorf("evm indexer source %s handler is required", source.Name)
	}
	return nil
}

// safeHead 根据当前链头和确认数计算可以安全处理的最高区块。
//
// confirmations=6 表示当前 head 为 110 时，最多处理到 104。这样即使最新几个区块
// 发生短 reorg，已经写入 MySQL 的查询数据也更稳定。
func safeHead(head uint64, confirmations uint64) (uint64, bool) {
	if head < confirmations {
		return 0, false
	}
	return head - confirmations, true
}

// eventRecordFromDecoded 提取 raw event 表需要的字段。
//
// DecodedEvent 还包含业务 payload map，raw event 表保存 payload_json 即可，方便排错
// 和后续重放，同时避免把 Go map 直接耦合到数据库结构。
func eventRecordFromDecoded(event DecodedEvent) EventRecord {
	return EventRecord{
		ChainID:         event.ChainID,
		ContractAddress: event.Contract,
		EventName:       event.Name,
		TxHash:          event.TxHash,
		LogIndex:        event.LogIndex,
		BlockNumber:     event.BlockNumber,
		BlockHash:       event.BlockHash,
		PayloadJSON:     event.PayloadJSON,
	}
}

// decodedFromRaw 在解码失败时生成最小失败上下文。
//
// 解码阶段可能还不知道事件名称和 payload，但 tx_hash、log_index、block_number 仍然
// 足够定位链上日志，因此 dead-letter 里至少要保留这些元数据。
func decodedFromRaw(chainID int64, raw types.Log) DecodedEvent {
	return DecodedEvent{
		ChainID:     chainID,
		Contract:    strings.ToLower(raw.Address.Hex()),
		TxHash:      strings.ToLower(raw.TxHash.Hex()),
		LogIndex:    raw.Index,
		BlockNumber: raw.BlockNumber,
		BlockHash:   strings.ToLower(raw.BlockHash.Hex()),
	}
}
