CREATE TABLE IF NOT EXISTS data_retention_policies (
    id BIGSERIAL PRIMARY KEY,
    table_name TEXT NOT NULL,
    time_column TEXT NOT NULL,
    interval_filter_column TEXT,
    interval_filter_value TEXT,
    retention_days INTEGER NOT NULL,
    chunk_size INTEGER NOT NULL DEFAULT 10000,
    enabled BOOLEAN NOT NULL DEFAULT true,
    dry_run BOOLEAN NOT NULL DEFAULT true,
    rollup_before_delete BOOLEAN NOT NULL DEFAULT false,
    rollup_target_table TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (retention_days > 0),
    CHECK (chunk_size > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_data_retention_policies_identity
    ON data_retention_policies (
        table_name,
        time_column,
        COALESCE(interval_filter_column, ''),
        COALESCE(interval_filter_value, '')
    );

CREATE INDEX IF NOT EXISTS ix_data_retention_policies_enabled
    ON data_retention_policies (enabled, table_name);

CREATE TABLE IF NOT EXISTS data_cleanup_runs (
    id BIGSERIAL PRIMARY KEY,
    policy_id BIGINT REFERENCES data_retention_policies(id) ON DELETE SET NULL,
    run_key TEXT NOT NULL,
    table_name TEXT NOT NULL,
    status TEXT NOT NULL,
    dry_run BOOLEAN NOT NULL,
    cutoff_time TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    rows_matched BIGINT NOT NULL DEFAULT 0,
    rows_deleted BIGINT NOT NULL DEFAULT 0,
    error_message TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    CHECK (status IN ('running', 'succeeded', 'failed', 'cancelled')),
    CHECK (rows_matched >= 0),
    CHECK (rows_deleted >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_data_cleanup_runs_run_key
    ON data_cleanup_runs (run_key);

CREATE INDEX IF NOT EXISTS ix_data_cleanup_runs_policy_started
    ON data_cleanup_runs (policy_id, started_at DESC);

CREATE INDEX IF NOT EXISTS ix_data_cleanup_runs_status_started
    ON data_cleanup_runs (status, started_at DESC);

CREATE TABLE IF NOT EXISTS data_rollup_runs (
    id BIGSERIAL PRIMARY KEY,
    policy_id BIGINT REFERENCES data_retention_policies(id) ON DELETE SET NULL,
    run_key TEXT NOT NULL,
    source_table TEXT NOT NULL,
    target_table TEXT NOT NULL,
    period TEXT NOT NULL,
    status TEXT NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    window_end TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    rows_read BIGINT NOT NULL DEFAULT 0,
    rows_written BIGINT NOT NULL DEFAULT 0,
    error_message TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    CHECK (status IN ('running', 'succeeded', 'failed', 'cancelled')),
    CHECK (window_end >= window_start),
    CHECK (rows_read >= 0),
    CHECK (rows_written >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_data_rollup_runs_run_key
    ON data_rollup_runs (run_key);

CREATE INDEX IF NOT EXISTS ix_data_rollup_runs_policy_started
    ON data_rollup_runs (policy_id, started_at DESC);

CREATE INDEX IF NOT EXISTS ix_data_rollup_runs_status_started
    ON data_rollup_runs (status, started_at DESC);
