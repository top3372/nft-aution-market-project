package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadConfigFromYAMLFile(t *testing.T) {
	configFile := writeConfig(t, `
app:
  env: test
http:
  addr: ":8081"
  cors_allowed_origins:
    - "http://127.0.0.1:5173"
    - "http://localhost:5173"
database:
  mysql_dsn: "auction_user:auction_pass@tcp(127.0.0.1:3306)/nft_auction?parseTime=true"
redis:
  enabled: true
  addr: "127.0.0.1:6379"
  password: "secret"
  db: 2
chain:
  network_name: sepolia
  chain_id: 11155111
  rpc_url: "https://sepolia.example"
  block_explorer_url: "https://sepolia.etherscan.io/"
contracts:
  market_address: "0xBA9af325234368184A61be6081cdFB7f02dc6405"
  auction_nft_address: "0x7913Ff1EaA12887ed80a3D35c81c0033FFafadCe"
  payment_token_address: "0x0000000000000000000000000000000000000000"
indexer:
  start_block: 123
  batch_size: 1000
  poll_interval: 10s
  confirmations: 6
  contracts:
    - name: market
      event_group: market
      address: "0xBA9af325234368184A61be6081cdFB7f02dc6405"
      abi: auction_market_v3
      start_block: 123
    - name: payment_token
      event_group: payment_token
      address: "0x0000000000000000000000000000000000000001"
      abi: erc20
      start_block: 124
`)

	cfg, err := LoadFromFile(configFile)

	require.NoError(t, err)
	require.Equal(t, "test", cfg.AppEnv)
	require.Equal(t, ":8081", cfg.HTTPAddr)
	require.Equal(t, []string{"http://127.0.0.1:5173", "http://localhost:5173"}, cfg.CORSAllowedOrigins)
	require.Equal(t, int64(11155111), cfg.ChainID)
	require.Equal(t, "https://sepolia.example", cfg.RPCURL)
	require.Equal(t, "0xba9af325234368184a61be6081cdfb7f02dc6405", cfg.MarketAddress)
	require.Equal(t, "0x7913ff1eaa12887ed80a3d35c81c0033ffafadce", cfg.AuctionNFTAddress)
	require.Equal(t, "https://sepolia.etherscan.io", cfg.BlockExplorerURL)
	require.Equal(t, uint64(123), cfg.StartBlock)
	require.True(t, cfg.RedisEnabled)
	require.Equal(t, "127.0.0.1:6379", cfg.RedisAddr)
	require.Equal(t, "secret", cfg.RedisPassword)
	require.Equal(t, 2, cfg.RedisDB)
	require.Equal(t, uint64(1000), cfg.IndexerBatchSize)
	require.Equal(t, 10*time.Second, cfg.IndexerPollInterval)
	require.Equal(t, uint64(6), cfg.IndexerConfirmations)
	require.Equal(t, []IndexerContract{
		{
			Name:       "market",
			EventGroup: "market",
			Address:    "0xba9af325234368184a61be6081cdfb7f02dc6405",
			ABI:        "auction_market_v3",
			StartBlock: 123,
		},
		{
			Name:       "payment_token",
			EventGroup: "payment_token",
			Address:    "0x0000000000000000000000000000000000000001",
			ABI:        "erc20",
			StartBlock: 124,
		},
	}, cfg.IndexerContracts)
}

func TestLoadConfigAllowsEnvironmentOverride(t *testing.T) {
	configFile := writeConfig(t, `
database:
  mysql_dsn: "from-file"
chain:
  chain_id: 11155111
  rpc_url: "https://sepolia.example"
contracts:
  market_address: "0xBA9af325234368184A61be6081cdFB7f02dc6405"
  auction_nft_address: "0x7913Ff1EaA12887ed80a3D35c81c0033FFafadCe"
`)
	t.Setenv("DATABASE_MYSQL_DSN", "from-env")
	t.Setenv("INDEXER_BATCH_SIZE", "25")

	cfg, err := LoadFromFile(configFile)

	require.NoError(t, err)
	require.Equal(t, "from-env", cfg.MySQLDSN)
	require.Equal(t, uint64(25), cfg.IndexerBatchSize)
	require.Equal(t, uint64(0), cfg.IndexerConfirmations)
	require.Equal(t, []string{"http://localhost:5173", "http://127.0.0.1:5173"}, cfg.CORSAllowedOrigins)
}

func TestLoadConfigAllowsLegacyEnvironmentNames(t *testing.T) {
	configFile := writeConfig(t, `
database:
  mysql_dsn: "from-file"
chain:
  chain_id: 11155111
  rpc_url: "https://sepolia.example"
contracts:
  market_address: "0xBA9af325234368184A61be6081cdFB7f02dc6405"
  auction_nft_address: "0x7913Ff1EaA12887ed80a3D35c81c0033FFafadCe"
indexer:
  start_block: 1
`)
	t.Setenv("MYSQL_DSN", "from-legacy-env")
	t.Setenv("START_BLOCK", "456")

	cfg, err := LoadFromFile(configFile)

	require.NoError(t, err)
	require.Equal(t, "from-legacy-env", cfg.MySQLDSN)
	require.Equal(t, uint64(456), cfg.StartBlock)
}

func TestProjectConfigIncludesSepoliaIndexerSources(t *testing.T) {
	cfg, err := LoadFromFile(filepath.Join("..", "..", "config.yaml"))

	require.NoError(t, err)
	require.Equal(t, int64(11155111), cfg.ChainID)
	require.Equal(t, "0xba9af325234368184a61be6081cdfb7f02dc6405", cfg.MarketAddress)
	require.Equal(t, "0x7913ff1eaa12887ed80a3d35c81c0033ffafadce", cfg.AuctionNFTAddress)
	require.Equal(t, "0xbe3c38a1015b4b4dacfd13c5346f1ee907d8c283", cfg.PaymentTokenAddress)
	require.Equal(t, uint64(6), cfg.IndexerConfirmations)
	require.Equal(t, []IndexerContract{
		{
			Name:       "market",
			EventGroup: "market",
			Address:    "0xba9af325234368184a61be6081cdfb7f02dc6405",
			ABI:        "auction_market_v3",
			StartBlock: 11222876,
		},
		{
			Name:       "auction_nft",
			EventGroup: "auction_nft",
			Address:    "0x7913ff1eaa12887ed80a3d35c81c0033ffafadce",
			ABI:        "auction_nft",
			StartBlock: 11222876,
		},
		{
			Name:       "payment_token",
			EventGroup: "payment_token",
			Address:    "0xbe3c38a1015b4b4dacfd13c5346f1ee907d8c283",
			ABI:        "erc20",
			StartBlock: 11222876,
		},
	}, cfg.IndexerContracts)
}

func TestLoadConfigRejectsMissingRequiredValues(t *testing.T) {
	configFile := writeConfig(t, `
app:
  env: test
`)

	_, err := LoadFromFile(configFile)

	require.Error(t, err)
	require.Contains(t, err.Error(), "database.mysql_dsn")
}

func TestLoadConfigRejectsMissingFile(t *testing.T) {
	_, err := LoadFromFile(filepath.Join(t.TempDir(), "missing.yaml"))

	require.Error(t, err)
	require.Contains(t, err.Error(), "read config file")
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}
