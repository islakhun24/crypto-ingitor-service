CREATE TABLE IF NOT EXISTS derivative_collection_policies (
    id BIGSERIAL PRIMARY KEY,
    exchange TEXT NOT NULL,
    market_type TEXT NOT NULL,
    data_type TEXT NOT NULL,
    tier TEXT NOT NULL,
    period TEXT,
    interval_seconds INTEGER NOT NULL,
    batch_size INTEGER NOT NULL DEFAULT 1,
    priority INTEGER NOT NULL DEFAULT 100,
    enabled BOOLEAN NOT NULL DEFAULT true,
    max_retry INTEGER NOT NULL DEFAULT 3,
    stale_after_seconds INTEGER,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (exchange = lower(exchange)),
    CHECK (interval_seconds > 0),
    CHECK (batch_size > 0),
    CHECK (max_retry >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_derivative_collection_policies_identity
    ON derivative_collection_policies (exchange, market_type, data_type, tier, COALESCE(period, ''));

CREATE INDEX IF NOT EXISTS ix_derivative_collection_policies_enabled
    ON derivative_collection_policies (enabled, exchange, data_type);

CREATE TABLE IF NOT EXISTS symbol_collection_tiers (
    id BIGSERIAL PRIMARY KEY,
    symbol_id BIGINT NOT NULL REFERENCES symbols(id) ON DELETE CASCADE,
    tier TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 100,
    reason TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_symbol_collection_tiers_symbol_tier
    ON symbol_collection_tiers (symbol_id, tier);

CREATE INDEX IF NOT EXISTS ix_symbol_collection_tiers_active
    ON symbol_collection_tiers (is_active, tier, priority);

CREATE TABLE IF NOT EXISTS derivative_collection_jobs (
    id BIGSERIAL PRIMARY KEY,
    exchange TEXT NOT NULL,
    data_type TEXT NOT NULL,
    tier TEXT NOT NULL,
    symbol_id BIGINT REFERENCES symbols(id) ON DELETE SET NULL,
    source_symbol TEXT NOT NULL,
    period TEXT,
    idempotency_key TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    priority INTEGER NOT NULL DEFAULT 100,
    scheduled_at TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retry INTEGER NOT NULL DEFAULT 3,
    error_message TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (exchange = lower(exchange)),
    CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'dead_letter', 'cancelled')),
    CHECK (retry_count >= 0),
    CHECK (max_retry >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_derivative_collection_jobs_idempotency_key
    ON derivative_collection_jobs (idempotency_key);

CREATE INDEX IF NOT EXISTS ix_derivative_collection_jobs_status_scheduled_at
    ON derivative_collection_jobs (status, scheduled_at, priority);

CREATE INDEX IF NOT EXISTS ix_derivative_collection_jobs_exchange_data_type
    ON derivative_collection_jobs (exchange, data_type);

CREATE INDEX IF NOT EXISTS ix_derivative_collection_jobs_symbol_scheduled_at
    ON derivative_collection_jobs (symbol_id, scheduled_at DESC);
