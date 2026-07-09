package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config 保存三个后端进程共享的运行配置。
// 配置层只负责读取和校验 config.yaml/环境变量，不创建数据库连接或链客户端。
type Config struct {
	AppEnv               string
	HTTPAddr             string
	CORSAllowedOrigins   []string
	MySQLDSN             string
	NetworkName          string
	ChainID              int64
	RPCURL               string
	MarketAddress        string
	AuctionNFTAddress    string
	PaymentTokenAddress  string
	BlockExplorerURL     string
	RedisAddr            string
	RedisPassword        string
	RedisDB              int
	RedisEnabled         bool
	StartBlock           uint64
	IndexerBatchSize     uint64
	IndexerPollInterval  time.Duration
	IndexerConfirmations uint64
	IndexerContracts     []IndexerContract
}

// IndexerContract 描述一个需要被 EVM 索引器扫描的合约。
//
// 旧版 indexer 只监听 AuctionMarket 一个地址；新框架支持把市场、NFT、支付代币
// 都作为独立 source 配置。ABI 字段用于选择事件解码器，event_group 用于隔离 cursor。
type IndexerContract struct {
	Name       string `mapstructure:"name"`
	EventGroup string `mapstructure:"event_group"`
	Address    string `mapstructure:"address"`
	ABI        string `mapstructure:"abi"`
	StartBlock uint64 `mapstructure:"start_block"`
}

// Load 从 config.yaml 读取配置，并允许环境变量覆盖。
// 默认读取当前工作目录下的 config.yaml；部署时可以通过 CONFIG_FILE 指定其他 YAML 文件。
func Load() (Config, error) {
	return LoadFromFile(strings.TrimSpace(os.Getenv("CONFIG_FILE")))
}

// LoadFromFile 便于测试或特殊进程显式指定配置文件。
func LoadFromFile(path string) (Config, error) {
	reader := newReader(path)
	if err := reader.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("read config file: %w", err)
	}

	cfg := Config{
		AppEnv:               reader.GetString("app.env"),
		HTTPAddr:             reader.GetString("http.addr"),
		CORSAllowedOrigins:   normalizeStringList(reader.GetStringSlice("http.cors_allowed_origins")),
		MySQLDSN:             strings.TrimSpace(reader.GetString("database.mysql_dsn")),
		NetworkName:          reader.GetString("chain.network_name"),
		ChainID:              reader.GetInt64("chain.chain_id"),
		RPCURL:               strings.TrimSpace(reader.GetString("chain.rpc_url")),
		MarketAddress:        normalizeAddress(reader.GetString("contracts.market_address")),
		AuctionNFTAddress:    normalizeAddress(reader.GetString("contracts.auction_nft_address")),
		PaymentTokenAddress:  normalizeAddress(reader.GetString("contracts.payment_token_address")),
		BlockExplorerURL:     strings.TrimRight(strings.TrimSpace(reader.GetString("chain.block_explorer_url")), "/"),
		RedisAddr:            strings.TrimSpace(reader.GetString("redis.addr")),
		RedisPassword:        reader.GetString("redis.password"),
		RedisDB:              reader.GetInt("redis.db"),
		RedisEnabled:         reader.GetBool("redis.enabled"),
		StartBlock:           reader.GetUint64("indexer.start_block"),
		IndexerBatchSize:     reader.GetUint64("indexer.batch_size"),
		IndexerPollInterval:  reader.GetDuration("indexer.poll_interval"),
		IndexerConfirmations: reader.GetUint64("indexer.confirmations"),
	}
	if err := reader.UnmarshalKey("indexer.contracts", &cfg.IndexerContracts); err != nil {
		return Config{}, fmt.Errorf("read indexer.contracts: %w", err)
	}
	cfg.IndexerContracts = normalizeIndexerContracts(cfg.IndexerContracts)

	return validate(cfg)
}

func newReader(path string) *viper.Viper {
	reader := viper.New()
	reader.SetConfigType("yaml")

	if path != "" {
		reader.SetConfigFile(path)
	} else {
		reader.SetConfigName("config")
		reader.AddConfigPath(".")
	}

	// 设置默认值能让 config.yaml 只保留真正需要按环境修改的配置。
	reader.SetDefault("app.env", "local")
	reader.SetDefault("http.addr", ":8080")
	reader.SetDefault("http.cors_allowed_origins", []string{"http://localhost:5173", "http://127.0.0.1:5173"})
	reader.SetDefault("chain.network_name", "sepolia")
	reader.SetDefault("chain.block_explorer_url", "https://sepolia.etherscan.io")
	reader.SetDefault("contracts.payment_token_address", "0x0000000000000000000000000000000000000000")
	reader.SetDefault("redis.enabled", false)
	reader.SetDefault("redis.addr", "127.0.0.1:6379")
	reader.SetDefault("redis.db", 0)
	reader.SetDefault("indexer.start_block", 0)
	reader.SetDefault("indexer.batch_size", 1000)
	reader.SetDefault("indexer.poll_interval", 10*time.Second)
	reader.SetDefault("indexer.confirmations", 0)

	// 允许环境变量覆盖 YAML。例：database.mysql_dsn -> DATABASE_MYSQL_DSN。
	reader.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	reader.AutomaticEnv()
	bindLegacyEnv(reader)

	return reader
}

func bindLegacyEnv(reader *viper.Viper) {
	// 兼容早期 .env.example 中的平铺变量名，同时推荐新部署使用按 YAML 路径展开的变量名。
	_ = reader.BindEnv("database.mysql_dsn", "DATABASE_MYSQL_DSN", "MYSQL_DSN")
	_ = reader.BindEnv("chain.network_name", "CHAIN_NETWORK_NAME", "NETWORK_NAME")
	_ = reader.BindEnv("chain.chain_id", "CHAIN_CHAIN_ID", "CHAIN_ID")
	_ = reader.BindEnv("chain.rpc_url", "CHAIN_RPC_URL", "RPC_URL")
	_ = reader.BindEnv("chain.block_explorer_url", "CHAIN_BLOCK_EXPLORER_URL", "BLOCK_EXPLORER_URL")
	_ = reader.BindEnv("contracts.market_address", "CONTRACTS_MARKET_ADDRESS", "MARKET_ADDRESS")
	_ = reader.BindEnv("contracts.auction_nft_address", "CONTRACTS_AUCTION_NFT_ADDRESS", "AUCTION_NFT_ADDRESS")
	_ = reader.BindEnv("contracts.payment_token_address", "CONTRACTS_PAYMENT_TOKEN_ADDRESS", "PAYMENT_TOKEN_ADDRESS")
	_ = reader.BindEnv("indexer.start_block", "INDEXER_START_BLOCK", "START_BLOCK")
}

func normalizeStringList(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func validate(cfg Config) (Config, error) {
	if cfg.MySQLDSN == "" {
		return Config{}, errors.New("database.mysql_dsn is required")
	}
	if cfg.RPCURL == "" {
		return Config{}, errors.New("chain.rpc_url is required")
	}
	if cfg.ChainID == 0 {
		return Config{}, errors.New("chain.chain_id is required")
	}
	if cfg.MarketAddress == "" {
		return Config{}, errors.New("contracts.market_address is required")
	}
	if cfg.AuctionNFTAddress == "" {
		return Config{}, errors.New("contracts.auction_nft_address is required")
	}
	if cfg.IndexerBatchSize == 0 {
		return Config{}, errors.New("indexer.batch_size must be greater than 0")
	}
	if cfg.IndexerPollInterval <= 0 {
		return Config{}, errors.New("indexer.poll_interval must be greater than 0")
	}
	for _, contract := range cfg.IndexerContracts {
		if contract.Name == "" {
			return Config{}, errors.New("indexer.contracts.name is required")
		}
		if contract.EventGroup == "" {
			return Config{}, errors.New("indexer.contracts.event_group is required")
		}
		if contract.Address == "" {
			return Config{}, errors.New("indexer.contracts.address is required")
		}
		if contract.ABI == "" {
			return Config{}, errors.New("indexer.contracts.abi is required")
		}
	}
	return cfg, nil
}

func normalizeAddress(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeIndexerContracts(contracts []IndexerContract) []IndexerContract {
	result := make([]IndexerContract, 0, len(contracts))
	for _, contract := range contracts {
		result = append(result, IndexerContract{
			Name:       strings.TrimSpace(contract.Name),
			EventGroup: strings.TrimSpace(contract.EventGroup),
			Address:    normalizeAddress(contract.Address),
			ABI:        strings.TrimSpace(contract.ABI),
			StartBlock: contract.StartBlock,
		})
	}
	return result
}
