ALTER TABLE derivative_aggregated_snapshots
    ADD COLUMN IF NOT EXISTS window_metrics JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS metrics JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS quality_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS anomaly_flags JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE market_structure_snapshots
    ADD COLUMN IF NOT EXISTS trend_direction TEXT NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS structure_state TEXT NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS last_swing_high NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS last_swing_low NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS support_levels JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS resistance_levels JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS price_position NUMERIC(18, 10);

ALTER TABLE volatility_snapshots
    ADD COLUMN IF NOT EXISTS atr_percent NUMERIC(18, 10),
    ADD COLUMN IF NOT EXISTS realized_volatility_percent NUMERIC(18, 10),
    ADD COLUMN IF NOT EXISTS range_percent_24h NUMERIC(18, 10),
    ADD COLUMN IF NOT EXISTS range_percent_7d NUMERIC(18, 10);

ALTER TABLE exchange_divergence_snapshots
    ADD COLUMN IF NOT EXISTS metrics JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS quality_metadata JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS ix_derivative_market_snapshots_symbol_exchange_time
    ON derivative_market_snapshots (symbol_id, exchange, snapshot_time DESC);

CREATE INDEX IF NOT EXISTS ix_derivative_klines_symbol_interval_open_time
    ON derivative_klines (symbol_id, "interval", open_time DESC);

CREATE INDEX IF NOT EXISTS ix_funding_rate_snapshots_symbol_window
    ON funding_rate_snapshots (symbol_id, snapshot_time DESC);

CREATE INDEX IF NOT EXISTS ix_basis_premium_snapshots_symbol_window
    ON basis_premium_snapshots (symbol_id, snapshot_time DESC);

CREATE INDEX IF NOT EXISTS ix_exchange_divergence_snapshots_symbol_data_time
    ON exchange_divergence_snapshots (symbol_id, data_type, snapshot_time DESC);
