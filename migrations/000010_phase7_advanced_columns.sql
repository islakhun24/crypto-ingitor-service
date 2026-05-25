ALTER TABLE long_short_ratio_snapshots
    ADD COLUMN IF NOT EXISTS long_ratio NUMERIC(18, 10),
    ADD COLUMN IF NOT EXISTS short_ratio NUMERIC(18, 10),
    ADD COLUMN IF NOT EXISTS long_position_ratio NUMERIC(18, 10),
    ADD COLUMN IF NOT EXISTS short_position_ratio NUMERIC(18, 10),
    ADD COLUMN IF NOT EXISTS top_trader_long_ratio NUMERIC(18, 10),
    ADD COLUMN IF NOT EXISTS top_trader_short_ratio NUMERIC(18, 10);

ALTER TABLE taker_flow_snapshots
    ADD COLUMN IF NOT EXISTS taker_buy_volume NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS taker_sell_volume NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS taker_buy_quote_volume NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS taker_sell_quote_volume NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS buy_sell_delta NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS buy_sell_delta_quote NUMERIC(38, 18);

ALTER TABLE cvd_snapshots
    ADD COLUMN IF NOT EXISTS cvd_change NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS cvd_change_percent NUMERIC(18, 10);

ALTER TABLE liquidation_events
    ADD COLUMN IF NOT EXISTS usd_value NUMERIC(38, 18);

ALTER TABLE liquidation_aggregates
    ADD COLUMN IF NOT EXISTS long_liquidation_usd NUMERIC(38, 18) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS short_liquidation_usd NUMERIC(38, 18) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS total_liquidation_usd NUMERIC(38, 18) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS largest_liquidation_usd NUMERIC(38, 18) NOT NULL DEFAULT 0;

ALTER TABLE basis_premium_snapshots
    ADD COLUMN IF NOT EXISTS futures_price NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS spot_price NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS basis_value NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS annualized_basis_percent NUMERIC(18, 10);

ALTER TABLE orderbook_imbalance_snapshots
    ADD COLUMN IF NOT EXISTS mid_price NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS spread_percent NUMERIC(18, 10),
    ADD COLUMN IF NOT EXISTS bid_depth_usd NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS ask_depth_usd NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS bid_depth_1pct_usd NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS ask_depth_1pct_usd NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS bid_depth_2pct_usd NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS ask_depth_2pct_usd NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS bid_depth_5pct_usd NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS ask_depth_5pct_usd NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS imbalance_percent NUMERIC(18, 10);

ALTER TABLE exchange_divergence_snapshots
    ADD COLUMN IF NOT EXISTS price_min NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS price_max NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS price_spread_percent NUMERIC(18, 10),
    ADD COLUMN IF NOT EXISTS oi_min NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS oi_max NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS oi_spread_percent NUMERIC(18, 10),
    ADD COLUMN IF NOT EXISTS funding_min NUMERIC(18, 10),
    ADD COLUMN IF NOT EXISTS funding_max NUMERIC(18, 10),
    ADD COLUMN IF NOT EXISTS funding_spread NUMERIC(18, 10),
    ADD COLUMN IF NOT EXISTS volume_min NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS volume_max NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS volume_spread_percent NUMERIC(18, 10),
    ADD COLUMN IF NOT EXISTS strongest_exchange TEXT,
    ADD COLUMN IF NOT EXISTS weakest_exchange TEXT,
    ADD COLUMN IF NOT EXISTS raw_by_exchange JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE derivative_aggregated_snapshots
    ADD COLUMN IF NOT EXISTS total_taker_buy_volume NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS total_taker_sell_volume NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS total_buy_sell_delta NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS total_cvd NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS total_long_liquidation_usd NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS total_short_liquidation_usd NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS total_liquidation_usd NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS avg_basis_percent NUMERIC(18, 10),
    ADD COLUMN IF NOT EXISTS avg_orderbook_imbalance_percent NUMERIC(18, 10),
    ADD COLUMN IF NOT EXISTS total_bid_depth_usd NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS total_ask_depth_usd NUMERIC(38, 18);

CREATE INDEX IF NOT EXISTS ix_taker_flow_snapshots_symbol_time
    ON taker_flow_snapshots (symbol_id, snapshot_time DESC);

CREATE INDEX IF NOT EXISTS ix_cvd_snapshots_symbol_exchange_period_time
    ON cvd_snapshots (symbol_id, exchange, period, snapshot_time DESC);

CREATE INDEX IF NOT EXISTS ix_liquidation_events_usd_time
    ON liquidation_events (usd_value, event_time DESC);
