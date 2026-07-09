package evmindexer

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestRunnerScansOnlyConfirmedBlocksAndSavesCursorHash(t *testing.T) {
	sourceAddress := common.HexToAddress("0x0000000000000000000000000000000000000abc")
	safeBlockHash := testHeaderHash(104, "safe")
	client := &fakeChainClient{
		head: 110,
		headers: map[uint64]*types.Header{
			104: testHeader(104, "safe"),
		},
		logs: []types.Log{
			{
				Address:     sourceAddress,
				BlockNumber: 101,
				BlockHash:   testHeaderHash(101, "event"),
				TxHash:      common.HexToHash("0x01"),
				Index:       2,
			},
		},
	}
	cursors := newMemoryCursorStore()
	events := &memoryEventStore{}
	handler := &recordingHandler{}
	runner := Runner{
		ChainID:       11155111,
		Client:        client,
		BatchSize:     100,
		Confirmations: 6,
		Cursors:       cursors,
		Events:        events,
		FailedEvents:  events,
		Sources: []ContractSource{
			{
				Name:       "market",
				EventGroup: "market",
				Address:    sourceAddress.Hex(),
				StartBlock: 100,
				Decoder: DecoderFunc(func(chainID int64, log types.Log) (DecodedEvent, bool, error) {
					return DecodedEvent{
						Name:        "AuctionCreatedV3",
						ChainID:     chainID,
						Contract:    strings.ToLower(log.Address.Hex()),
						TxHash:      strings.ToLower(log.TxHash.Hex()),
						LogIndex:    log.Index,
						BlockNumber: log.BlockNumber,
						BlockHash:   strings.ToLower(log.BlockHash.Hex()),
						PayloadJSON: `{"auction_id":"1"}`,
						Payload:     map[string]string{"auction_id": "1"},
					}, true, nil
				}),
				Handler: handler,
			},
		},
	}

	err := runner.RunOnce(context.Background())

	require.NoError(t, err)
	require.Len(t, client.queries, 1)
	require.Equal(t, uint64(100), client.queries[0].FromBlock.Uint64())
	require.Equal(t, uint64(104), client.queries[0].ToBlock.Uint64())
	require.Equal(t, []common.Address{sourceAddress}, client.queries[0].Addresses)
	require.Len(t, handler.events, 1)
	require.Equal(t, "AuctionCreatedV3", handler.events[0].Name)
	cursor, err := cursors.GetCursor(context.Background(), Scope{
		ChainID:         11155111,
		ContractAddress: sourceAddress.Hex(),
		EventGroup:      "market",
	})
	require.NoError(t, err)
	require.Equal(t, uint64(104), cursor.LastScannedBlock)
	require.Equal(t, strings.ToLower(safeBlockHash.Hex()), cursor.LastScannedBlockHash)
}

func TestRunnerRecordsFailedEventAndDoesNotAdvanceCursor(t *testing.T) {
	sourceAddress := common.HexToAddress("0x0000000000000000000000000000000000000abc")
	client := &fakeChainClient{
		head: 10,
		headers: map[uint64]*types.Header{
			10: testHeader(10, "head"),
		},
		logs: []types.Log{
			{
				Address:     sourceAddress,
				BlockNumber: 10,
				BlockHash:   testHeaderHash(10, "event"),
				TxHash:      common.HexToHash("0x02"),
				Index:       1,
			},
		},
	}
	cursors := newMemoryCursorStore()
	events := &memoryEventStore{}
	runner := Runner{
		ChainID:      1,
		Client:       client,
		BatchSize:    10,
		Cursors:      cursors,
		Events:       events,
		FailedEvents: events,
		Sources: []ContractSource{
			{
				Name:       "market",
				EventGroup: "market",
				Address:    sourceAddress.Hex(),
				StartBlock: 10,
				Decoder: DecoderFunc(func(chainID int64, log types.Log) (DecodedEvent, bool, error) {
					return DecodedEvent{
						Name:        "BidPlaced",
						ChainID:     chainID,
						Contract:    strings.ToLower(log.Address.Hex()),
						TxHash:      strings.ToLower(log.TxHash.Hex()),
						LogIndex:    log.Index,
						BlockNumber: log.BlockNumber,
						BlockHash:   strings.ToLower(log.BlockHash.Hex()),
						PayloadJSON: `{"auction_id":"1"}`,
						Payload:     map[string]string{"auction_id": "1"},
					}, true, nil
				}),
				Handler: failingHandler{err: errors.New("business table update failed")},
			},
		},
	}

	err := runner.RunOnce(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), "business table update failed")
	require.Len(t, events.failed, 1)
	require.Equal(t, "handler", events.failed[0].Stage)
	require.Equal(t, "BidPlaced", events.failed[0].EventName)
	cursor, err := cursors.GetCursor(context.Background(), Scope{
		ChainID:         1,
		ContractAddress: sourceAddress.Hex(),
		EventGroup:      "market",
	})
	require.NoError(t, err)
	require.Zero(t, cursor.LastScannedBlock)
}

func TestRunnerReturnsReorgErrorWhenStoredCursorHashDiffersFromChain(t *testing.T) {
	sourceAddress := common.HexToAddress("0x0000000000000000000000000000000000000abc")
	client := &fakeChainClient{
		head: 120,
		headers: map[uint64]*types.Header{
			100: testHeader(100, "new-chain"),
		},
	}
	cursors := newMemoryCursorStore()
	scope := Scope{ChainID: 1, ContractAddress: sourceAddress.Hex(), EventGroup: "market"}
	require.NoError(t, cursors.SaveCursor(context.Background(), scope, Cursor{
		LastScannedBlock:     100,
		LastScannedBlockHash: strings.ToLower(testHeaderHash(100, "old-chain").Hex()),
	}))
	runner := Runner{
		ChainID:       1,
		Client:        client,
		BatchSize:     10,
		Confirmations: 6,
		Cursors:       cursors,
		Events:        &memoryEventStore{},
		FailedEvents:  &memoryEventStore{},
		Sources: []ContractSource{
			{
				Name:       "market",
				EventGroup: "market",
				Address:    sourceAddress.Hex(),
				StartBlock: 90,
				Decoder:    DecoderFunc(func(int64, types.Log) (DecodedEvent, bool, error) { return DecodedEvent{}, false, nil }),
				Handler:    &recordingHandler{},
			},
		},
	}

	err := runner.RunOnce(context.Background())

	require.ErrorIs(t, err, ErrReorgDetected)
	require.Empty(t, client.queries)
}

type fakeChainClient struct {
	head    uint64
	headers map[uint64]*types.Header
	logs    []types.Log
	queries []ethereum.FilterQuery
}

func (c *fakeChainClient) BlockNumber(context.Context) (uint64, error) {
	return c.head, nil
}

func (c *fakeChainClient) HeaderByNumber(_ context.Context, number *big.Int) (*types.Header, error) {
	if number == nil {
		return c.headers[c.head], nil
	}
	return c.headers[number.Uint64()], nil
}

func (c *fakeChainClient) FilterLogs(_ context.Context, query ethereum.FilterQuery) ([]types.Log, error) {
	c.queries = append(c.queries, query)
	return c.logs, nil
}

type memoryCursorStore struct {
	cursors map[Scope]Cursor
}

func newMemoryCursorStore() *memoryCursorStore {
	return &memoryCursorStore{cursors: make(map[Scope]Cursor)}
}

func (s *memoryCursorStore) GetCursor(_ context.Context, scope Scope) (Cursor, error) {
	return s.cursors[NormalizeScope(scope)], nil
}

func (s *memoryCursorStore) SaveCursor(_ context.Context, scope Scope, cursor Cursor) error {
	s.cursors[NormalizeScope(scope)] = cursor
	return nil
}

type memoryEventStore struct {
	records []EventRecord
	failed  []FailedEventRecord
}

func (s *memoryEventStore) InsertOnce(_ context.Context, event EventRecord) (bool, error) {
	s.records = append(s.records, event)
	return true, nil
}

func (s *memoryEventStore) InsertFailed(_ context.Context, event FailedEventRecord) error {
	s.failed = append(s.failed, event)
	return nil
}

type recordingHandler struct {
	events []DecodedEvent
}

func (h *recordingHandler) Handle(_ context.Context, event DecodedEvent) error {
	h.events = append(h.events, event)
	return nil
}

type failingHandler struct {
	err error
}

func (h failingHandler) Handle(context.Context, DecodedEvent) error {
	return h.err
}

func testHeader(number uint64, extra string) *types.Header {
	return &types.Header{
		Number: big.NewInt(int64(number)),
		Extra:  []byte(extra),
	}
}

func testHeaderHash(number uint64, extra string) common.Hash {
	return testHeader(number, extra).Hash()
}
