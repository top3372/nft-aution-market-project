package database

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInitialMigrationDefinesAutoIDsAndBusinessKeys(t *testing.T) {
	path := filepath.Join("..", "..", "migrations", "001_init.sql")
	content, err := os.ReadFile(path)
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT")
	require.Contains(t, sql, "UNIQUE KEY uk_auctions_chain_market_auction")
	require.Contains(t, sql, "UNIQUE KEY uk_bids_tx_log")
	require.Contains(t, sql, "UNIQUE KEY uk_events_tx_log")
	require.Contains(t, sql, "UNIQUE KEY uk_nfts_chain_contract_token")
	require.Contains(t, sql, "UNIQUE KEY uk_sync_cursors_scope")
	require.Contains(t, sql, "last_scanned_block_hash VARCHAR(66)")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS indexer_failed_events")
	require.Contains(t, sql, "KEY idx_failed_events_retry")
	require.NotContains(t, sql, "CHECK (")
	require.NotContains(t, sql, "CONSTRAINT")
}
