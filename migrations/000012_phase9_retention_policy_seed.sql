ALTER TABLE data_retention_policies
    ADD COLUMN IF NOT EXISTS priority INTEGER NOT NULL DEFAULT 100,
    ADD COLUMN IF NOT EXISTS max_rows_per_run INTEGER NOT NULL DEFAULT 50000,
    ADD COLUMN IF NOT EXISTS timeout_seconds INTEGER NOT NULL DEFAULT 120,
    ADD COLUMN IF NOT EXISTS partition_strategy TEXT NOT NULL DEFAULT 'auto',
    ADD COLUMN IF NOT EXISTS min_retention_days INTEGER NOT NULL DEFAULT 1;

INSERT INTO data_retention_policies (
    table_name, time_column, interval_filter_column, interval_filter_value,
    retention_days, chunk_size, enabled, dry_run, rollup_before_delete,
    rollup_target_table, priority, max_rows_per_run, timeout_seconds,
    partition_strategy, min_retention_days, metadata
)
VALUES
    ('derivative_klines', 'open_time', 'interval', '1m', 14, 10000, true, true, true, 'derivative_klines', 10, 50000, 180, 'auto', 7, '{"rollup_target_interval":"5m","partition_grain":"monthly"}'::jsonb),
    ('derivative_klines', 'open_time', 'interval', '5m', 180, 10000, true, true, true, 'derivative_klines', 11, 50000, 180, 'auto', 30, '{"rollup_target_interval":"15m","partition_grain":"monthly"}'::jsonb),
    ('derivative_klines', 'open_time', 'interval', '15m', 365, 10000, true, true, true, 'derivative_klines', 12, 50000, 180, 'auto', 90, '{"rollup_target_interval":"1h","partition_grain":"monthly"}'::jsonb),
    ('derivative_klines', 'open_time', 'interval', '1h', 1095, 10000, true, true, true, 'derivative_klines', 13, 50000, 180, 'auto', 365, '{"rollup_target_interval":"4h","partition_grain":"monthly"}'::jsonb),
    ('derivative_klines', 'open_time', 'interval', '4h', 1825, 10000, true, true, true, 'derivative_klines', 14, 50000, 180, 'auto', 730, '{"rollup_target_interval":"1d","partition_grain":"monthly"}'::jsonb),
    ('derivative_klines', 'open_time', 'interval', '1d', 3650, 10000, false, true, false, NULL, 15, 50000, 180, 'auto', 365, '{"long_term":true,"partition_grain":"monthly"}'::jsonb),

    ('derivative_market_snapshots', 'snapshot_time', NULL, NULL, 90, 10000, true, true, false, NULL, 20, 50000, 120, 'auto', 7, '{"partition_grain":"weekly"}'::jsonb),
    ('open_interest_snapshots', 'snapshot_time', NULL, NULL, 180, 10000, true, true, false, NULL, 21, 50000, 120, 'auto', 14, '{"partition_grain":"monthly"}'::jsonb),
    ('funding_rate_snapshots', 'snapshot_time', NULL, NULL, 90, 10000, true, true, false, NULL, 22, 50000, 120, 'auto', 30, '{"partition_grain":"monthly"}'::jsonb),
    ('funding_rate_history', 'funding_time', NULL, NULL, 1825, 10000, false, true, false, NULL, 23, 50000, 120, 'auto', 365, '{"long_term":true,"partition_grain":"monthly"}'::jsonb),
    ('long_short_ratio_snapshots', 'snapshot_time', NULL, NULL, 180, 10000, true, true, false, NULL, 24, 50000, 120, 'auto', 90, '{"partition_grain":"monthly"}'::jsonb),
    ('taker_flow_snapshots', 'snapshot_time', NULL, NULL, 90, 10000, true, true, false, NULL, 25, 50000, 120, 'auto', 30, '{"partition_grain":"monthly"}'::jsonb),
    ('cvd_snapshots', 'snapshot_time', NULL, NULL, 90, 10000, true, true, false, NULL, 26, 50000, 120, 'auto', 30, '{"partition_grain":"monthly"}'::jsonb),
    ('derivative_aggregated_snapshots', 'snapshot_time', NULL, NULL, 365, 10000, true, true, false, NULL, 27, 50000, 120, 'auto', 14, '{"partition_grain":"monthly"}'::jsonb),

    ('liquidation_events', 'event_time', NULL, NULL, 30, 10000, true, true, false, NULL, 30, 50000, 120, 'auto', 7, '{"partition_grain":"weekly","raw_events":true}'::jsonb),
    ('liquidation_aggregates', 'bucket_time', 'period', '1m', 90, 10000, true, true, false, NULL, 31, 50000, 120, 'auto', 30, '{"partition_grain":"monthly"}'::jsonb),
    ('liquidation_aggregates', 'bucket_time', 'period', '5m', 365, 10000, true, true, false, NULL, 32, 50000, 120, 'auto', 90, '{"partition_grain":"monthly"}'::jsonb),

    ('orderbook_imbalance_snapshots', 'snapshot_time', NULL, NULL, 30, 10000, true, true, false, NULL, 40, 50000, 120, 'auto', 7, '{"partition_grain":"weekly"}'::jsonb),
    ('orderbook_depth_snapshots', 'snapshot_time', NULL, NULL, 7, 5000, true, true, false, NULL, 41, 25000, 120, 'auto', 1, '{"watchlist_only":true,"partition_grain":"weekly"}'::jsonb),

    ('exchange_request_logs', 'captured_at', NULL, NULL, 30, 10000, true, true, false, NULL, 50, 50000, 120, 'delete', 7, '{}'::jsonb),
    ('raw_exchange_payloads', 'captured_at', NULL, NULL, 7, 5000, true, true, false, NULL, 51, 25000, 120, 'delete', 1, '{}'::jsonb),
    ('failed_collection_jobs', 'failed_at', NULL, NULL, 90, 10000, true, true, false, NULL, 52, 50000, 120, 'delete', 30, '{}'::jsonb),
    ('data_collection_runs', 'started_at', NULL, NULL, 90, 10000, true, true, false, NULL, 53, 50000, 120, 'delete', 30, '{}'::jsonb),
    ('data_quality_issues', 'last_seen_at', NULL, NULL, 180, 10000, true, true, false, NULL, 54, 50000, 120, 'delete', 90, '{}'::jsonb)
ON CONFLICT (
    table_name,
    time_column,
    (COALESCE(interval_filter_column, '')),
    (COALESCE(interval_filter_value, ''))
) DO UPDATE SET
    retention_days = EXCLUDED.retention_days,
    chunk_size = EXCLUDED.chunk_size,
    enabled = EXCLUDED.enabled,
    dry_run = EXCLUDED.dry_run,
    rollup_before_delete = EXCLUDED.rollup_before_delete,
    rollup_target_table = EXCLUDED.rollup_target_table,
    priority = EXCLUDED.priority,
    max_rows_per_run = EXCLUDED.max_rows_per_run,
    timeout_seconds = EXCLUDED.timeout_seconds,
    partition_strategy = EXCLUDED.partition_strategy,
    min_retention_days = EXCLUDED.min_retention_days,
    metadata = data_retention_policies.metadata || EXCLUDED.metadata,
    updated_at = now();

CREATE INDEX IF NOT EXISTS ix_data_retention_policies_priority
    ON data_retention_policies (enabled, priority, table_name);

CREATE INDEX IF NOT EXISTS ix_derivative_klines_interval_open_time_retention
    ON derivative_klines ("interval", open_time ASC);

CREATE INDEX IF NOT EXISTS ix_derivative_market_snapshots_retention
    ON derivative_market_snapshots (snapshot_time ASC);

CREATE INDEX IF NOT EXISTS ix_open_interest_snapshots_retention
    ON open_interest_snapshots (snapshot_time ASC);

CREATE INDEX IF NOT EXISTS ix_taker_flow_snapshots_retention
    ON taker_flow_snapshots (snapshot_time ASC);

CREATE INDEX IF NOT EXISTS ix_cvd_snapshots_retention
    ON cvd_snapshots (snapshot_time ASC);

CREATE INDEX IF NOT EXISTS ix_liquidation_events_retention
    ON liquidation_events (event_time ASC);

CREATE INDEX IF NOT EXISTS ix_orderbook_imbalance_snapshots_retention
    ON orderbook_imbalance_snapshots (snapshot_time ASC);

CREATE INDEX IF NOT EXISTS ix_derivative_aggregated_snapshots_retention
    ON derivative_aggregated_snapshots (snapshot_time ASC);
