package indexer

import (
	"context"
	"testing"

	"nft-auction-backend/internal/evmindexer"
	"nft-auction-backend/internal/repository"

	"github.com/stretchr/testify/require"
)

func TestNFTTransferHandlerUpsertsOwnerCache(t *testing.T) {
	store := &recordingNFTStore{}
	handler := NFTTransferHandler{
		NFTAddress: "0x7913Ff1EaA12887ed80a3D35c81c0033FFafadCe",
		NFTs:       store,
	}

	err := handler.Handle(context.Background(), evmindexer.DecodedEvent{
		Name:    "Transfer",
		ChainID: 11155111,
		Payload: map[string]string{
			"to":       "0x0000000000000000000000000000000000000abc",
			"token_id": "7",
		},
	})

	require.NoError(t, err)
	require.Len(t, store.tokens, 1)
	require.Equal(t, int64(11155111), store.tokens[0].ChainID)
	require.Equal(t, "0x7913Ff1EaA12887ed80a3D35c81c0033FFafadCe", store.tokens[0].NFTAddress)
	require.Equal(t, "7", store.tokens[0].TokenID)
	require.NotNil(t, store.tokens[0].OwnerAddress)
	require.Equal(t, "0x0000000000000000000000000000000000000abc", *store.tokens[0].OwnerAddress)
}

func TestNFTTransferHandlerMarksBurnedTokenOwnerEmpty(t *testing.T) {
	store := &recordingNFTStore{}
	handler := NFTTransferHandler{
		NFTAddress: "0x7913Ff1EaA12887ed80a3D35c81c0033FFafadCe",
		NFTs:       store,
	}

	err := handler.Handle(context.Background(), evmindexer.DecodedEvent{
		Name:    "Transfer",
		ChainID: 11155111,
		Payload: map[string]string{
			"to":       "0x0000000000000000000000000000000000000000",
			"token_id": "7",
		},
	})

	require.NoError(t, err)
	require.Len(t, store.tokens, 1)
	require.Nil(t, store.tokens[0].OwnerAddress)
}

type recordingNFTStore struct {
	tokens []repository.NFTModel
}

func (s *recordingNFTStore) UpsertNFT(ctx context.Context, token repository.NFTModel) error {
	s.tokens = append(s.tokens, token)
	return nil
}
