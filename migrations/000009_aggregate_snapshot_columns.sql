ALTER TABLE derivative_aggregated_snapshots
    ADD COLUMN IF NOT EXISTS price_avg NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS price_weighted NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS total_quote_volume_24h NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS total_open_interest_value NUMERIC(38, 18),
    ADD COLUMN IF NOT EXISTS min_funding_rate NUMERIC(18, 10),
    ADD COLUMN IF NOT EXISTS max_funding_rate NUMERIC(18, 10),
    ADD COLUMN IF NOT EXISTS available_exchanges JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS raw_by_exchange JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS ix_derivative_aggregated_snapshots_snapshot_time
    ON derivative_aggregated_snapshots (snapshot_time DESC);
