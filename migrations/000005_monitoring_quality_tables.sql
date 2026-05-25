CREATE TABLE IF NOT EXISTS collector_health (
    id BIGSERIAL PRIMARY KEY,
    service_name TEXT NOT NULL,
    instance_id TEXT NOT NULL,
    exchange TEXT,
    data_type TEXT,
    status TEXT NOT NULL,
    heartbeat_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_success_at TIMESTAMPTZ,
    last_error_at TIMESTAMPTZ,
    error_message TEXT,
    metrics JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (exchange IS NULL OR exchange = lower(exchange)),
    CHECK (status IN ('starting', 'healthy', 'degraded', 'unhealthy', 'stopped'))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_collector_health_instance_scope
    ON collector_health (service_name, instance_id, COALESCE(exchange, ''), COALESCE(data_type, ''));

CREATE INDEX IF NOT EXISTS ix_collector_health_status_heartbeat
    ON collector_health (status, heartbeat_at DESC);

CREATE TABLE IF NOT EXISTS data_collection_runs (
    id BIGSERIAL PRIMARY KEY,
    run_key TEXT NOT NULL,
    service_name TEXT NOT NULL,
    exchange TEXT,
    data_type TEXT,
    status TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    symbols_planned INTEGER NOT NULL DEFAULT 0,
    symbols_succeeded INTEGER NOT NULL DEFAULT 0,
    symbols_failed INTEGER NOT NULL DEFAULT 0,
    jobs_created INTEGER NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (exchange IS NULL OR exchange = lower(exchange)),
    CHECK (status IN ('running', 'succeeded', 'failed', 'cancelled')),
    CHECK (symbols_planned >= 0),
    CHECK (symbols_succeeded >= 0),
    CHECK (symbols_failed >= 0),
    CHECK (jobs_created >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_data_collection_runs_run_key
    ON data_collection_runs (run_key);

CREATE INDEX IF NOT EXISTS ix_data_collection_runs_exchange_data_type
    ON data_collection_runs (exchange, data_type);

CREATE INDEX IF NOT EXISTS ix_data_collection_runs_status_started
    ON data_collection_runs (status, started_at DESC);

CREATE TABLE IF NOT EXISTS exchange_request_logs (
    id BIGSERIAL PRIMARY KEY,
    exchange TEXT NOT NULL,
    endpoint_id BIGINT REFERENCES exchange_api_endpoints(id) ON DELETE SET NULL,
    data_type TEXT NOT NULL,
    source_symbol TEXT,
    request_url TEXT,
    request_path TEXT,
    status_code INTEGER,
    error_type TEXT,
    duration_ms INTEGER,
    retry_count INTEGER NOT NULL DEFAULT 0,
    rate_limited BOOLEAN NOT NULL DEFAULT false,
    captured_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    CHECK (exchange = lower(exchange)),
    CHECK (duration_ms IS NULL OR duration_ms >= 0),
    CHECK (retry_count >= 0)
);

CREATE INDEX IF NOT EXISTS ix_exchange_request_logs_exchange_data_type
    ON exchange_request_logs (exchange, data_type);

CREATE INDEX IF NOT EXISTS ix_exchange_request_logs_endpoint_captured
    ON exchange_request_logs (endpoint_id, captured_at DESC);

CREATE INDEX IF NOT EXISTS ix_exchange_request_logs_symbol_captured
    ON exchange_request_logs (source_symbol, captured_at DESC);

CREATE INDEX IF NOT EXISTS ix_exchange_request_logs_rate_limited
    ON exchange_request_logs (rate_limited, captured_at DESC);

CREATE TABLE IF NOT EXISTS failed_collection_jobs (
    id BIGSERIAL PRIMARY KEY,
    job_id BIGINT REFERENCES derivative_collection_jobs(id) ON DELETE SET NULL,
    idempotency_key TEXT,
    exchange TEXT NOT NULL,
    data_type TEXT NOT NULL,
    tier TEXT,
    symbol_id BIGINT REFERENCES symbols(id) ON DELETE SET NULL,
    source_symbol TEXT,
    period TEXT,
    failed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    retry_count INTEGER NOT NULL DEFAULT 0,
    error_type TEXT,
    error_message TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    resolved BOOLEAN NOT NULL DEFAULT false,
    resolved_at TIMESTAMPTZ,
    CHECK (exchange = lower(exchange)),
    CHECK (retry_count >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_failed_collection_jobs_job_id
    ON failed_collection_jobs (job_id)
    WHERE job_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_failed_collection_jobs_idempotency_key
    ON failed_collection_jobs (idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_failed_collection_jobs_exchange_data_type
    ON failed_collection_jobs (exchange, data_type);

CREATE INDEX IF NOT EXISTS ix_failed_collection_jobs_failed_at
    ON failed_collection_jobs (failed_at DESC);

CREATE TABLE IF NOT EXISTS data_quality_issues (
    id BIGSERIAL PRIMARY KEY,
    issue_key TEXT NOT NULL,
    severity TEXT NOT NULL,
    exchange TEXT,
    data_type TEXT NOT NULL,
    symbol_id BIGINT REFERENCES symbols(id) ON DELETE SET NULL,
    source_symbol TEXT,
    issue_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    CHECK (exchange IS NULL OR exchange = lower(exchange)),
    CHECK (severity IN ('info', 'warning', 'error', 'critical')),
    CHECK (status IN ('open', 'investigating', 'resolved', 'ignored'))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_data_quality_issues_issue_key
    ON data_quality_issues (issue_key);

CREATE INDEX IF NOT EXISTS ix_data_quality_issues_symbol_seen
    ON data_quality_issues (symbol_id, last_seen_at DESC);

CREATE INDEX IF NOT EXISTS ix_data_quality_issues_exchange_data_type
    ON data_quality_issues (exchange, data_type);

CREATE INDEX IF NOT EXISTS ix_data_quality_issues_status_severity
    ON data_quality_issues (status, severity);

CREATE TABLE IF NOT EXISTS data_gaps (
    id BIGSERIAL PRIMARY KEY,
    gap_key TEXT NOT NULL,
    symbol_id BIGINT NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    exchange TEXT NOT NULL,
    data_type TEXT NOT NULL,
    period TEXT,
    gap_start TIMESTAMPTZ NOT NULL,
    gap_end TIMESTAMPTZ NOT NULL,
    detected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    backfill_status TEXT NOT NULL DEFAULT 'pending',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    CHECK (exchange = lower(exchange)),
    CHECK (gap_end >= gap_start),
    CHECK (backfill_status IN ('pending', 'queued', 'running', 'completed', 'failed', 'ignored'))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_data_gaps_gap_key
    ON data_gaps (gap_key);

CREATE INDEX IF NOT EXISTS ix_data_gaps_symbol_gap_start
    ON data_gaps (symbol_id, gap_start DESC);

CREATE INDEX IF NOT EXISTS ix_data_gaps_exchange_data_type
    ON data_gaps (exchange, data_type);

CREATE INDEX IF NOT EXISTS ix_data_gaps_backfill_status
    ON data_gaps (backfill_status, detected_at DESC);

CREATE TABLE IF NOT EXISTS raw_exchange_payloads (
    id BIGSERIAL PRIMARY KEY,
    exchange TEXT NOT NULL,
    endpoint_id BIGINT REFERENCES exchange_api_endpoints(id) ON DELETE SET NULL,
    data_type TEXT NOT NULL,
    source_symbol TEXT,
    payload_time TIMESTAMPTZ NOT NULL,
    payload_hash TEXT NOT NULL,
    capture_reason TEXT NOT NULL DEFAULT 'debug',
    payload JSONB NOT NULL,
    captured_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    retention_expires_at TIMESTAMPTZ,
    CHECK (exchange = lower(exchange)),
    CHECK (capture_reason IN ('debug', 'failed_job', 'quality_issue', 'audit'))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_raw_exchange_payloads_identity
    ON raw_exchange_payloads (exchange, data_type, COALESCE(source_symbol, ''), payload_time, payload_hash);

CREATE INDEX IF NOT EXISTS ix_raw_exchange_payloads_symbol_time
    ON raw_exchange_payloads (source_symbol, payload_time DESC);

CREATE INDEX IF NOT EXISTS ix_raw_exchange_payloads_exchange_data_type
    ON raw_exchange_payloads (exchange, data_type);

CREATE INDEX IF NOT EXISTS ix_raw_exchange_payloads_retention_expires
    ON raw_exchange_payloads (retention_expires_at)
    WHERE retention_expires_at IS NOT NULL;
