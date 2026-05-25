CREATE INDEX IF NOT EXISTS ix_derivative_aggregated_snapshots_api_latest
    ON derivative_aggregated_snapshots (symbol_id, snapshot_time DESC)
    INCLUDE (
        price_avg,
        price_weighted,
        total_volume_24h,
        total_open_interest,
        avg_funding_rate,
        total_cvd,
        total_liquidation_usd,
        avg_basis_percent,
        exchange_count
    );

CREATE INDEX IF NOT EXISTS ix_derivative_klines_api_symbol_interval_time
    ON derivative_klines (symbol_id, "interval", open_time DESC);

CREATE INDEX IF NOT EXISTS ix_open_interest_history_api_symbol_period_time
    ON open_interest_history (symbol_id, period, "timestamp" DESC);

CREATE INDEX IF NOT EXISTS ix_funding_rate_history_api_symbol_time
    ON funding_rate_history (symbol_id, funding_time DESC);

CREATE INDEX IF NOT EXISTS ix_long_short_ratio_api_symbol_period_time
    ON long_short_ratio_snapshots (symbol_id, period, snapshot_time DESC);

CREATE INDEX IF NOT EXISTS ix_taker_flow_api_symbol_period_time
    ON taker_flow_snapshots (symbol_id, period, snapshot_time DESC);

CREATE INDEX IF NOT EXISTS ix_cvd_api_symbol_period_time
    ON cvd_snapshots (symbol_id, period, snapshot_time DESC);

CREATE INDEX IF NOT EXISTS ix_liquidation_aggregates_api_symbol_period_time
    ON liquidation_aggregates (symbol_id, period, bucket_time DESC);

CREATE INDEX IF NOT EXISTS ix_basis_premium_api_symbol_time
    ON basis_premium_snapshots (symbol_id, snapshot_time DESC);

CREATE INDEX IF NOT EXISTS ix_orderbook_imbalance_api_symbol_time
    ON orderbook_imbalance_snapshots (symbol_id, snapshot_time DESC);

CREATE INDEX IF NOT EXISTS ix_collector_health_api_exchange_heartbeat
    ON collector_health (exchange, heartbeat_at DESC);

CREATE INDEX IF NOT EXISTS ix_data_quality_issues_api_status_seen
    ON data_quality_issues (status, last_seen_at DESC);

CREATE INDEX IF NOT EXISTS ix_data_gaps_api_status_start
    ON data_gaps (backfill_status, gap_start DESC);
