CREATE TABLE IF NOT EXISTS long_short_ratio_snapshots (
    id BIGSERIAL PRIMARY KEY,
    symbol_id BIGINT NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    exchange TEXT NOT NULL,
    market_type TEXT NOT NULL,
    source_symbol TEXT NOT NULL,
    period TEXT NOT NULL,
    snapshot_time TIMESTAMPTZ NOT NULL,
    long_account_ratio NUMERIC(18, 10),
    short_account_ratio NUMERIC(18, 10),
    long_short_ratio NUMERIC(18, 10),
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (exchange = lower(exchange))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_long_short_ratio_snapshots_symbol_exchange_period_time
    ON long_short_ratio_snapshots (symbol_id, exchange, period, snapshot_time);

CREATE INDEX IF NOT EXISTS ix_long_short_ratio_snapshots_symbol_time
    ON long_short_ratio_snapshots (symbol_id, snapshot_time DESC);

CREATE TABLE IF NOT EXISTS taker_flow_snapshots (
    id BIGSERIAL PRIMARY KEY,
    symbol_id BIGINT NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    exchange TEXT NOT NULL,
    market_type TEXT NOT NULL,
    source_symbol TEXT NOT NULL,
    period TEXT NOT NULL,
    snapshot_time TIMESTAMPTZ NOT NULL,
    buy_volume NUMERIC(38, 18),
    sell_volume NUMERIC(38, 18),
    buy_quote_volume NUMERIC(38, 18),
    sell_quote_volume NUMERIC(38, 18),
    buy_sell_ratio NUMERIC(18, 10),
    net_quote_flow NUMERIC(38, 18),
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (exchange = lower(exchange))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_taker_flow_snapshots_symbol_exchange_period_time
    ON taker_flow_snapshots (symbol_id, exchange, period, snapshot_time);

CREATE INDEX IF NOT EXISTS ix_taker_flow_snapshots_symbol_time
    ON taker_flow_snapshots (symbol_id, snapshot_time DESC);

CREATE TABLE IF NOT EXISTS cvd_snapshots (
    id BIGSERIAL PRIMARY KEY,
    symbol_id BIGINT NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    exchange TEXT NOT NULL,
    market_type TEXT NOT NULL,
    source_symbol TEXT NOT NULL,
    period TEXT NOT NULL,
    snapshot_time TIMESTAMPTZ NOT NULL,
    cvd_value NUMERIC(38, 18),
    cvd_delta NUMERIC(38, 18),
    buy_volume NUMERIC(38, 18),
    sell_volume NUMERIC(38, 18),
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (exchange = lower(exchange))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_cvd_snapshots_symbol_exchange_period_time
    ON cvd_snapshots (symbol_id, exchange, period, snapshot_time);

CREATE INDEX IF NOT EXISTS ix_cvd_snapshots_symbol_time
    ON cvd_snapshots (symbol_id, snapshot_time DESC);

CREATE TABLE IF NOT EXISTS liquidation_events (
    id BIGSERIAL PRIMARY KEY,
    event_key TEXT NOT NULL,
    exchange TEXT NOT NULL,
    market_type TEXT NOT NULL,
    symbol_id BIGINT REFERENCES symbols(id) ON DELETE SET NULL,
    source_symbol TEXT NOT NULL,
    event_time TIMESTAMPTZ NOT NULL,
    side TEXT NOT NULL,
    price NUMERIC(38, 18),
    quantity NUMERIC(38, 18),
    notional NUMERIC(38, 18),
    order_id TEXT,
    trade_id TEXT,
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    captured_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (exchange = lower(exchange)),
    CHECK (side IN ('buy', 'sell', 'long', 'short', 'unknown'))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_liquidation_events_event_key
    ON liquidation_events (event_key);

CREATE INDEX IF NOT EXISTS ix_liquidation_events_symbol_time
    ON liquidation_events (symbol_id, event_time DESC);

CREATE INDEX IF NOT EXISTS ix_liquidation_events_exchange_time
    ON liquidation_events (exchange, event_time DESC);

CREATE TABLE IF NOT EXISTS liquidation_aggregates (
    id BIGSERIAL PRIMARY KEY,
    symbol_id BIGINT NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    exchange TEXT NOT NULL,
    market_type TEXT NOT NULL,
    source_symbol TEXT NOT NULL,
    period TEXT NOT NULL,
    bucket_time TIMESTAMPTZ NOT NULL,
    long_liquidation_count BIGINT NOT NULL DEFAULT 0,
    short_liquidation_count BIGINT NOT NULL DEFAULT 0,
    long_liquidation_notional NUMERIC(38, 18) NOT NULL DEFAULT 0,
    short_liquidation_notional NUMERIC(38, 18) NOT NULL DEFAULT 0,
    total_liquidation_notional NUMERIC(38, 18) NOT NULL DEFAULT 0,
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (exchange = lower(exchange))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_liquidation_aggregates_symbol_exchange_period_bucket
    ON liquidation_aggregates (symbol_id, exchange, period, bucket_time);

CREATE INDEX IF NOT EXISTS ix_liquidation_aggregates_symbol_bucket
    ON liquidation_aggregates (symbol_id, bucket_time DESC);

CREATE TABLE IF NOT EXISTS basis_premium_snapshots (
    id BIGSERIAL PRIMARY KEY,
    symbol_id BIGINT NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    exchange TEXT NOT NULL,
    market_type TEXT NOT NULL,
    source_symbol TEXT NOT NULL,
    snapshot_time TIMESTAMPTZ NOT NULL,
    mark_price NUMERIC(38, 18),
    index_price NUMERIC(38, 18),
    basis NUMERIC(38, 18),
    basis_percent NUMERIC(18, 10),
    premium_index NUMERIC(18, 10),
    funding_rate NUMERIC(18, 10),
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (exchange = lower(exchange))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_basis_premium_snapshots_symbol_exchange_time
    ON basis_premium_snapshots (symbol_id, exchange, snapshot_time);

CREATE INDEX IF NOT EXISTS ix_basis_premium_snapshots_symbol_time
    ON basis_premium_snapshots (symbol_id, snapshot_time DESC);

CREATE TABLE IF NOT EXISTS orderbook_imbalance_snapshots (
    id BIGSERIAL PRIMARY KEY,
    symbol_id BIGINT NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    exchange TEXT NOT NULL,
    market_type TEXT NOT NULL,
    source_symbol TEXT NOT NULL,
    snapshot_time TIMESTAMPTZ NOT NULL,
    depth_levels INTEGER NOT NULL DEFAULT 20,
    bid_notional NUMERIC(38, 18),
    ask_notional NUMERIC(38, 18),
    imbalance_ratio NUMERIC(18, 10),
    spread_bps NUMERIC(18, 8),
    mid_price NUMERIC(38, 18),
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (exchange = lower(exchange)),
    CHECK (depth_levels > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_orderbook_imbalance_snapshots_symbol_exchange_time_depth
    ON orderbook_imbalance_snapshots (symbol_id, exchange, snapshot_time, depth_levels);

CREATE INDEX IF NOT EXISTS ix_orderbook_imbalance_snapshots_symbol_time
    ON orderbook_imbalance_snapshots (symbol_id, snapshot_time DESC);

CREATE TABLE IF NOT EXISTS orderbook_depth_snapshots (
    id BIGSERIAL PRIMARY KEY,
    symbol_id BIGINT NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    exchange TEXT NOT NULL,
    market_type TEXT NOT NULL,
    source_symbol TEXT NOT NULL,
    snapshot_time TIMESTAMPTZ NOT NULL,
    depth_levels INTEGER NOT NULL,
    watchlist_only BOOLEAN NOT NULL DEFAULT true,
    bid_depth JSONB NOT NULL DEFAULT '[]'::jsonb,
    ask_depth JSONB NOT NULL DEFAULT '[]'::jsonb,
    checksum TEXT,
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (exchange = lower(exchange)),
    CHECK (depth_levels > 0),
    CHECK (watchlist_only = true)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_orderbook_depth_snapshots_symbol_exchange_time_depth
    ON orderbook_depth_snapshots (symbol_id, exchange, snapshot_time, depth_levels);

CREATE INDEX IF NOT EXISTS ix_orderbook_depth_snapshots_symbol_time
    ON orderbook_depth_snapshots (symbol_id, snapshot_time DESC);

CREATE TABLE IF NOT EXISTS exchange_divergence_snapshots (
    id BIGSERIAL PRIMARY KEY,
    symbol_id BIGINT NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    data_type TEXT NOT NULL,
    snapshot_time TIMESTAMPTZ NOT NULL,
    reference_exchange TEXT NOT NULL,
    compared_exchange TEXT NOT NULL,
    reference_value NUMERIC(38, 18),
    compared_value NUMERIC(38, 18),
    divergence_abs NUMERIC(38, 18),
    divergence_bps NUMERIC(18, 8),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (reference_exchange = lower(reference_exchange)),
    CHECK (compared_exchange = lower(compared_exchange))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_exchange_divergence_snapshots_identity
    ON exchange_divergence_snapshots (symbol_id, data_type, reference_exchange, compared_exchange, snapshot_time);

CREATE INDEX IF NOT EXISTS ix_exchange_divergence_snapshots_symbol_time
    ON exchange_divergence_snapshots (symbol_id, snapshot_time DESC);

CREATE TABLE IF NOT EXISTS derivative_aggregated_snapshots (
    id BIGSERIAL PRIMARY KEY,
    symbol_id BIGINT NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    snapshot_time TIMESTAMPTZ NOT NULL,
    exchange_count INTEGER NOT NULL DEFAULT 0,
    weighted_price NUMERIC(38, 18),
    median_price NUMERIC(38, 18),
    best_bid NUMERIC(38, 18),
    best_ask NUMERIC(38, 18),
    total_volume_24h NUMERIC(38, 18),
    total_open_interest NUMERIC(38, 18),
    avg_funding_rate NUMERIC(18, 10),
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (exchange_count >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_derivative_aggregated_snapshots_symbol_time
    ON derivative_aggregated_snapshots (symbol_id, snapshot_time);

CREATE INDEX IF NOT EXISTS ix_derivative_aggregated_snapshots_symbol_time
    ON derivative_aggregated_snapshots (symbol_id, snapshot_time DESC);

CREATE TABLE IF NOT EXISTS market_structure_snapshots (
    id BIGSERIAL PRIMARY KEY,
    symbol_id BIGINT NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    exchange TEXT NOT NULL DEFAULT 'aggregate',
    market_type TEXT,
    source_symbol TEXT,
    period TEXT NOT NULL,
    snapshot_time TIMESTAMPTZ NOT NULL,
    trend TEXT,
    support_price NUMERIC(38, 18),
    resistance_price NUMERIC(38, 18),
    breakout_state TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (exchange = lower(exchange))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_market_structure_snapshots_symbol_exchange_period_time
    ON market_structure_snapshots (symbol_id, exchange, period, snapshot_time);

CREATE INDEX IF NOT EXISTS ix_market_structure_snapshots_symbol_time
    ON market_structure_snapshots (symbol_id, snapshot_time DESC);

CREATE TABLE IF NOT EXISTS volatility_snapshots (
    id BIGSERIAL PRIMARY KEY,
    symbol_id BIGINT NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    exchange TEXT NOT NULL DEFAULT 'aggregate',
    market_type TEXT,
    source_symbol TEXT,
    period TEXT NOT NULL,
    snapshot_time TIMESTAMPTZ NOT NULL,
    realized_volatility NUMERIC(18, 10),
    atr NUMERIC(38, 18),
    high_price NUMERIC(38, 18),
    low_price NUMERIC(38, 18),
    close_price NUMERIC(38, 18),
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (exchange = lower(exchange))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_volatility_snapshots_symbol_exchange_period_time
    ON volatility_snapshots (symbol_id, exchange, period, snapshot_time);

CREATE INDEX IF NOT EXISTS ix_volatility_snapshots_symbol_time
    ON volatility_snapshots (symbol_id, snapshot_time DESC);
