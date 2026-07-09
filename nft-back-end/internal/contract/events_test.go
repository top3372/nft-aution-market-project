package contract

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestDecodeAuctionCreatedV3Event(t *testing.T) {
	event := marketABI.Events["AuctionCreatedV3"]
	data, err := event.Inputs.NonIndexed().Pack(uint64(100), uint64(200), big.NewInt(300_00000000))
	require.NoError(t, err)

	decoded, ok, err := DecodeMarketLog(11155111, types.Log{
		Address: common.HexToAddress("0xBA9af325234368184A61be6081cdFB7f02dc6405"),
		Topics: []common.Hash{
			event.ID,
			common.BigToHash(big.NewInt(1)),
			common.BytesToHash(common.HexToAddress("0x0000000000000000000000000000000000000abc").Bytes()),
			common.BigToHash(big.NewInt(7)),
		},
		Data:        data,
		BlockNumber: 10,
		TxHash:      common.HexToHash("0x01"),
		Index:       2,
		BlockHash:   common.HexToHash("0x02"),
	})

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "AuctionCreatedV3", decoded.Name)
	require.Equal(t, "1", decoded.Payload["auction_id"])
	require.Equal(t, "7", decoded.Payload["token_id"])
	require.Equal(t, "30000000000", decoded.Payload["starting_price_usd"])
}

func TestDecodeBidPlacedEvent(t *testing.T) {
	event := marketABI.Events["BidPlaced"]
	data, err := event.Inputs.NonIndexed().Pack(big.NewInt(1000), big.NewInt(100_00000000))
	require.NoError(t, err)

	decoded, ok, err := DecodeMarketLog(11155111, types.Log{
		Address: common.HexToAddress("0xBA9af325234368184A61be6081cdFB7f02dc6405"),
		Topics: []common.Hash{
			event.ID,
			common.BigToHash(big.NewInt(1)),
			common.BytesToHash(common.HexToAddress("0x0000000000000000000000000000000000000def").Bytes()),
			common.BytesToHash(common.HexToAddress("0x0000000000000000000000000000000000000aaa").Bytes()),
		},
		Data: data,
	})

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "BidPlaced", decoded.Name)
	require.Equal(t, "1000", decoded.Payload["amount"])
	require.Equal(t, "10000000000", decoded.Payload["amount_usd"])
}

func TestDecodeAuctionCancelledEvent(t *testing.T) {
	event := marketABI.Events["AuctionCancelled"]

	decoded, ok, err := DecodeMarketLog(11155111, types.Log{
		Address: common.HexToAddress("0xBA9af325234368184A61be6081cdFB7f02dc6405"),
		Topics: []common.Hash{
			event.ID,
			common.BigToHash(big.NewInt(1)),
			common.BytesToHash(common.HexToAddress("0x0000000000000000000000000000000000000abc").Bytes()),
			common.BigToHash(big.NewInt(7)),
		},
	})

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "AuctionCancelled", decoded.Name)
	require.Equal(t, "1", decoded.Payload["auction_id"])
	require.Equal(t, "7", decoded.Payload["token_id"])
}

func TestDecodeAuctionNFTTransferEvent(t *testing.T) {
	event := auctionNFTABI.Events["Transfer"]

	decoded, ok, err := DecodeAuctionNFTLog(11155111, types.Log{
		Address: common.HexToAddress("0x7913Ff1EaA12887ed80a3D35c81c0033FFafadCe"),
		Topics: []common.Hash{
			event.ID,
			common.BytesToHash(common.HexToAddress("0x0000000000000000000000000000000000000000").Bytes()),
			common.BytesToHash(common.HexToAddress("0x0000000000000000000000000000000000000abc").Bytes()),
			common.BigToHash(big.NewInt(7)),
		},
		BlockNumber: 10,
		TxHash:      common.HexToHash("0x03"),
		Index:       4,
		BlockHash:   common.HexToHash("0x04"),
	})

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "Transfer", decoded.Name)
	require.Equal(t, "0x0000000000000000000000000000000000000000", decoded.Payload["from"])
	require.Equal(t, "0x0000000000000000000000000000000000000abc", decoded.Payload["to"])
	require.Equal(t, "7", decoded.Payload["token_id"])
}

func TestDecodeERC20TransferEvent(t *testing.T) {
	decoded, ok, err := DecodeERC20TransferLog(11155111, types.Log{
		Address: common.HexToAddress("0xBE3c38a1015b4B4dacfd13C5346F1Ee907D8c283"),
		Topics: []common.Hash{
			erc20TransferEventID,
			common.BytesToHash(common.HexToAddress("0x0000000000000000000000000000000000000abc").Bytes()),
			common.BytesToHash(common.HexToAddress("0x0000000000000000000000000000000000000def").Bytes()),
		},
		Data:        common.BigToHash(big.NewInt(1000)).Bytes(),
		BlockNumber: 11,
		TxHash:      common.HexToHash("0x05"),
		Index:       6,
		BlockHash:   common.HexToHash("0x06"),
	})

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "Transfer", decoded.Name)
	require.Equal(t, "0x0000000000000000000000000000000000000abc", decoded.Payload["from"])
	require.Equal(t, "0x0000000000000000000000000000000000000def", decoded.Payload["to"])
	require.Equal(t, "1000", decoded.Payload["amount"])
}
