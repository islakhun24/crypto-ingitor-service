CREATE TABLE IF NOT EXISTS derivative_market_snapshots (
    id BIGSERIAL PRIMARY KEY,
    symbol_id BIGINT NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    exchange TEXT NOT NULL,
    market_type TEXT NOT NULL,
    source_symbol TEXT NOT NULL,
    snapshot_time TIMESTAMPTZ NOT NULL,
    last_price NUMERIC(38, 18),
    mark_price NUMERIC(38, 18),
    index_price NUMERIC(38, 18),
    bid_price NUMERIC(38, 18),
    ask_price NUMERIC(38, 18),
    volume_24h NUMERIC(38, 18),
    quote_volume_24h NUMERIC(38, 18),
    price_change_percent_24h NUMERIC(18, 8),
    open_interest NUMERIC(38, 18),
    funding_rate NUMERIC(18, 10),
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (exchange = lower(exchange))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_derivative_market_snapshots_symbol_exchange_time
    ON derivative_market_snapshots (symbol_id, exchange, snapshot_time);

CREATE INDEX IF NOT EXISTS ix_derivative_market_snapshots_symbol_time
    ON derivative_market_snapshots (symbol_id, snapshot_time DESC);

CREATE INDEX IF NOT EXISTS ix_derivative_market_snapshots_exchange_time
    ON derivative_market_snapshots (exchange, snapshot_time DESC);

CREATE TABLE IF NOT EXISTS derivative_klines (
    id BIGSERIAL PRIMARY KEY,
    symbol_id BIGINT NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    exchange TEXT NOT NULL,
    market_type TEXT NOT NULL,
    source_symbol TEXT NOT NULL,
    "interval" TEXT NOT NULL,
    open_time TIMESTAMPTZ NOT NULL,
    close_time TIMESTAMPTZ NOT NULL,
    open_price NUMERIC(38, 18) NOT NULL,
    high_price NUMERIC(38, 18) NOT NULL,
    low_price NUMERIC(38, 18) NOT NULL,
    close_price NUMERIC(38, 18) NOT NULL,
    volume NUMERIC(38, 18),
    quote_volume NUMERIC(38, 18),
    trade_count BIGINT,
    taker_buy_volume NUMERIC(38, 18),
    taker_buy_quote_volume NUMERIC(38, 18),
    is_closed BOOLEAN NOT NULL DEFAULT true,
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (exchange = lower(exchange))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_derivative_klines_symbol_exchange_interval_open_time
    ON derivative_klines (symbol_id, exchange, "interval", open_time);

CREATE INDEX IF NOT EXISTS ix_derivative_klines_symbol_open_time
    ON derivative_klines (symbol_id, open_time DESC);

CREATE INDEX IF NOT EXISTS ix_derivative_klines_exchange_interval_open_time
    ON derivative_klines (exchange, "interval", open_time DESC);

CREATE TABLE IF NOT EXISTS open_interest_snapshots (
    id BIGSERIAL PRIMARY KEY,
    symbol_id BIGINT NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    exchange TEXT NOT NULL,
    market_type TEXT NOT NULL,
    source_symbol TEXT NOT NULL,
    snapshot_time TIMESTAMPTZ NOT NULL,
    open_interest NUMERIC(38, 18) NOT NULL,
    open_interest_value NUMERIC(38, 18),
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (exchange = lower(exchange))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_open_interest_snapshots_symbol_exchange_time
    ON open_interest_snapshots (symbol_id, exchange, snapshot_time);

CREATE INDEX IF NOT EXISTS ix_open_interest_snapshots_symbol_time
    ON open_interest_snapshots (symbol_id, snapshot_time DESC);

CREATE TABLE IF NOT EXISTS open_interest_history (
    id BIGSERIAL PRIMARY KEY,
    symbol_id BIGINT NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    exchange TEXT NOT NULL,
    market_type TEXT NOT NULL,
    source_symbol TEXT NOT NULL,
    period TEXT NOT NULL,
    "timestamp" TIMESTAMPTZ NOT NULL,
    open_interest NUMERIC(38, 18) NOT NULL,
    open_interest_value NUMERIC(38, 18),
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (exchange = lower(exchange))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_open_interest_history_symbol_exchange_period_timestamp
    ON open_interest_history (symbol_id, exchange, period, "timestamp");

CREATE INDEX IF NOT EXISTS ix_open_interest_history_symbol_timestamp
    ON open_interest_history (symbol_id, "timestamp" DESC);

CREATE TABLE IF NOT EXISTS funding_rate_snapshots (
    id BIGSERIAL PRIMARY KEY,
    symbol_id BIGINT NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    exchange TEXT NOT NULL,
    market_type TEXT NOT NULL,
    source_symbol TEXT NOT NULL,
    snapshot_time TIMESTAMPTZ NOT NULL,
    funding_rate NUMERIC(18, 10) NOT NULL,
    next_funding_time TIMESTAMPTZ,
    mark_price NUMERIC(38, 18),
    index_price NUMERIC(38, 18),
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (exchange = lower(exchange))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_funding_rate_snapshots_symbol_exchange_time
    ON funding_rate_snapshots (symbol_id, exchange, snapshot_time);

CREATE INDEX IF NOT EXISTS ix_funding_rate_snapshots_symbol_time
    ON funding_rate_snapshots (symbol_id, snapshot_time DESC);

CREATE TABLE IF NOT EXISTS funding_rate_history (
    id BIGSERIAL PRIMARY KEY,
    symbol_id BIGINT NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    exchange TEXT NOT NULL,
    market_type TEXT NOT NULL,
    source_symbol TEXT NOT NULL,
    funding_time TIMESTAMPTZ NOT NULL,
    funding_rate NUMERIC(18, 10) NOT NULL,
    realized_rate NUMERIC(18, 10),
    mark_price NUMERIC(38, 18),
    raw_data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (exchange = lower(exchange))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_funding_rate_history_symbol_exchange_funding_time
    ON funding_rate_history (symbol_id, exchange, funding_time);

CREATE INDEX IF NOT EXISTS ix_funding_rate_history_symbol_funding_time
    ON funding_rate_history (symbol_id, funding_time DESC);
