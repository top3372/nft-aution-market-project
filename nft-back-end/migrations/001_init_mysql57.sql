-- NFT 拍卖 DApp 初始化表结构，兼容 MySQL 5.7。
-- 设计原则：
-- 1. 每张业务表都使用自增 id 作为数据库主键，便于内部关联、分页和排查问题。
-- 2. 链上身份使用独立业务唯一键，例如 chain_id + market_address + auction_id。
-- 3. uint256 金额使用 VARCHAR(78) 保存十进制字符串，避免数据库整数溢出和精度丢失。
-- 4. MySQL 5.7 环境下不依赖 CHECK 约束；JSON 内容使用 LONGTEXT 保存。

CREATE TABLE IF NOT EXISTS auctions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '数据库自增主键',
  chain_id BIGINT NOT NULL COMMENT '链 ID，例如 Sepolia 为 11155111',
  market_address VARCHAR(42) NOT NULL COMMENT '拍卖市场代理合约地址，小写 0x 地址',
  auction_id BIGINT UNSIGNED NOT NULL COMMENT '链上拍卖 ID，对应合约 auctions 数组下标',
  seller VARCHAR(42) NOT NULL COMMENT '卖家钱包地址，小写 0x 地址',
  nft_address VARCHAR(42) NOT NULL COMMENT 'NFT 合约地址，小写 0x 地址',
  token_id VARCHAR(78) NOT NULL COMMENT 'NFT tokenId，使用十进制字符串保存',
  start_time DATETIME(3) NULL COMMENT 'V3 拍卖开始时间；旧拍卖可为空',
  end_time DATETIME(3) NOT NULL COMMENT '拍卖结束时间',
  starting_price_usd VARCHAR(78) NOT NULL DEFAULT '0' COMMENT 'V3 起拍价，8 位 USD 精度，旧拍卖为 0',
  payment_token VARCHAR(42) NOT NULL DEFAULT '0x0000000000000000000000000000000000000000' COMMENT '当前最高价支付 token 地址',
  highest_bidder VARCHAR(42) NULL COMMENT '当前最高出价人地址；无人出价时为空',
  highest_bid VARCHAR(78) NOT NULL DEFAULT '0' COMMENT '当前最高出价原币数量，十进制字符串',
  highest_bid_usd VARCHAR(78) NOT NULL DEFAULT '0' COMMENT '当前最高出价 USD 价值，8 位精度',
  status VARCHAR(24) NOT NULL DEFAULT 'pending' COMMENT '拍卖状态：pending/active/ended/cancelled',
  created_tx_hash VARCHAR(66) NOT NULL COMMENT '创建拍卖交易 hash',
  created_block_number BIGINT UNSIGNED NOT NULL COMMENT '创建拍卖所在区块号',
  ended_tx_hash VARCHAR(66) NULL COMMENT '结束或取消拍卖交易 hash',
  ended_block_number BIGINT UNSIGNED NULL COMMENT '结束或取消拍卖所在区块号',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '数据库创建时间',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '数据库更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_auctions_chain_market_auction (chain_id, market_address, auction_id),
  KEY idx_auctions_nft_status (chain_id, market_address, nft_address, token_id, status),
  KEY idx_auctions_status_end_time (status, end_time),
  KEY idx_auctions_seller (seller),
  KEY idx_auctions_highest_bidder (highest_bidder),
  KEY idx_auctions_nft_token (nft_address, token_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='拍卖查询主表';

CREATE TABLE IF NOT EXISTS bids (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '数据库自增主键',
  chain_id BIGINT NOT NULL COMMENT '链 ID',
  market_address VARCHAR(42) NOT NULL COMMENT '拍卖市场代理合约地址，小写 0x 地址',
  auction_id BIGINT UNSIGNED NOT NULL COMMENT '链上拍卖 ID',
  bidder VARCHAR(42) NOT NULL COMMENT '出价人钱包地址，小写 0x 地址',
  payment_token VARCHAR(42) NOT NULL COMMENT '支付 token 地址，小写 0x 地址',
  amount VARCHAR(78) NOT NULL COMMENT '出价原币数量，十进制字符串',
  amount_usd VARCHAR(78) NOT NULL COMMENT '出价 USD 价值，8 位精度',
  tx_hash VARCHAR(66) NOT NULL COMMENT '出价交易 hash',
  log_index INT UNSIGNED NOT NULL COMMENT '事件在交易日志中的序号',
  block_number BIGINT UNSIGNED NOT NULL COMMENT '事件所在区块号',
  block_hash VARCHAR(66) NOT NULL COMMENT '事件所在区块 hash',
  block_time DATETIME(3) NULL COMMENT '事件所在区块时间',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '数据库创建时间',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '数据库更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_bids_tx_log (tx_hash, log_index),
  KEY idx_bids_auction (chain_id, market_address, auction_id),
  KEY idx_bids_bidder (bidder),
  KEY idx_bids_block_number (block_number)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='出价历史表';

CREATE TABLE IF NOT EXISTS auction_events (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '数据库自增主键',
  chain_id BIGINT NOT NULL COMMENT '链 ID',
  contract_address VARCHAR(42) NOT NULL COMMENT '产生事件的合约地址，小写 0x 地址',
  event_name VARCHAR(80) NOT NULL COMMENT '事件名称',
  tx_hash VARCHAR(66) NOT NULL COMMENT '交易 hash',
  log_index INT UNSIGNED NOT NULL COMMENT '事件在交易日志中的序号',
  block_number BIGINT UNSIGNED NOT NULL COMMENT '事件所在区块号',
  block_hash VARCHAR(66) NOT NULL COMMENT '事件所在区块 hash',
  payload_json LONGTEXT NOT NULL COMMENT '事件解析后的 JSON 字符串，MySQL 5.7 用 LONGTEXT 保存',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '数据库创建时间',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '数据库更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_events_tx_log (tx_hash, log_index),
  KEY idx_events_contract_block (chain_id, contract_address, block_number),
  KEY idx_events_name_block (event_name, block_number)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='链上原始事件表';

CREATE TABLE IF NOT EXISTS sync_cursors (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '数据库自增主键',
  chain_id BIGINT NOT NULL COMMENT '链 ID',
  contract_address VARCHAR(42) NOT NULL COMMENT '同步目标合约地址，小写 0x 地址',
  event_group VARCHAR(64) NOT NULL COMMENT '同步分组，例如 market 或 nft',
  last_scanned_block BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '最后成功扫描到的区块号',
  last_scanned_block_hash VARCHAR(66) NOT NULL DEFAULT '' COMMENT '最后成功扫描区块的 hash，用于检测链重组',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '数据库创建时间',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '数据库更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_sync_cursors_scope (chain_id, contract_address, event_group)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='链上事件同步游标表';

CREATE TABLE IF NOT EXISTS indexer_failed_events (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '数据库自增主键',
  chain_id BIGINT NOT NULL COMMENT '链 ID',
  contract_address VARCHAR(42) NOT NULL COMMENT '产生事件的合约地址，小写 0x 地址',
  event_name VARCHAR(80) NOT NULL DEFAULT '' COMMENT '事件名称，解码失败时可能为空',
  tx_hash VARCHAR(66) NOT NULL COMMENT '交易 hash',
  log_index INT UNSIGNED NOT NULL COMMENT '事件在交易日志中的序号',
  block_number BIGINT UNSIGNED NOT NULL COMMENT '事件所在区块号',
  block_hash VARCHAR(66) NOT NULL COMMENT '事件所在区块 hash',
  stage VARCHAR(32) NOT NULL COMMENT '失败阶段：decode/raw_event/handler',
  error_message TEXT NOT NULL COMMENT '失败原因',
  payload_json LONGTEXT NOT NULL COMMENT '已解码 payload，解码失败时为空字符串',
  retry_count INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '人工或后续任务重试次数',
  resolved_at DATETIME(3) NULL COMMENT '问题确认处理完成时间',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '数据库创建时间',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '数据库更新时间',
  PRIMARY KEY (id),
  KEY idx_failed_events_retry (resolved_at, retry_count, block_number),
  KEY idx_failed_events_tx_log (tx_hash, log_index),
  KEY idx_failed_events_contract_block (chain_id, contract_address, block_number)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='索引失败事件死信表';

CREATE TABLE IF NOT EXISTS nfts (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '数据库自增主键',
  chain_id BIGINT NOT NULL COMMENT '链 ID',
  nft_address VARCHAR(42) NOT NULL COMMENT 'NFT 合约地址，小写 0x 地址',
  token_id VARCHAR(78) NOT NULL COMMENT 'NFT tokenId，使用十进制字符串保存',
  owner_address VARCHAR(42) NULL COMMENT '当前 owner 地址，小写 0x 地址；未知时为空',
  token_uri TEXT NULL COMMENT 'NFT tokenURI',
  metadata_json LONGTEXT NULL COMMENT 'NFT 元数据 JSON 字符串，MySQL 5.7 用 LONGTEXT 保存',
  last_metadata_sync_at DATETIME(3) NULL COMMENT '最后一次同步元数据时间',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '数据库创建时间',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '数据库更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_nfts_chain_contract_token (chain_id, nft_address, token_id),
  KEY idx_nfts_owner (owner_address),
  KEY idx_nfts_metadata_sync (last_metadata_sync_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='NFT 资产和元数据缓存表';

CREATE TABLE IF NOT EXISTS platform_stats (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '数据库自增主键',
  chain_id BIGINT NOT NULL COMMENT '链 ID',
  market_address VARCHAR(42) NOT NULL COMMENT '拍卖市场代理合约地址，小写 0x 地址',
  active_auction_count BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '活跃拍卖数量',
  ended_auction_count BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '已结束拍卖数量',
  cancelled_auction_count BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '已取消拍卖数量',
  total_bid_count BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '总出价次数',
  total_volume_usd VARCHAR(78) NOT NULL DEFAULT '0' COMMENT '总成交额，8 位 USD 精度',
  snapshot_time DATETIME(3) NOT NULL COMMENT '统计快照时间',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '数据库创建时间',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '数据库更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_platform_stats_snapshot (chain_id, market_address, snapshot_time),
  KEY idx_platform_stats_market_time (chain_id, market_address, snapshot_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='平台统计快照表';
