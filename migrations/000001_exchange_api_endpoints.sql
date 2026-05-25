CREATE TABLE IF NOT EXISTS exchange_api_endpoints (
    id BIGSERIAL PRIMARY KEY,
    exchange TEXT NOT NULL,
    market_type TEXT NOT NULL,
    data_type TEXT NOT NULL,
    name TEXT NOT NULL,
    method TEXT NOT NULL DEFAULT 'GET',
    base_url TEXT NOT NULL,
    path TEXT NOT NULL,
    params_template JSONB NOT NULL DEFAULT '{}'::jsonb,
    headers_template JSONB NOT NULL DEFAULT '{}'::jsonb,
    response_format TEXT NOT NULL DEFAULT 'json',
    is_batch_supported BOOLEAN NOT NULL DEFAULT false,
    batch_param_name TEXT,
    max_batch_size INTEGER NOT NULL DEFAULT 1,
    rate_limit_per_second NUMERIC(12, 4),
    rate_limit_per_minute INTEGER,
    request_weight INTEGER NOT NULL DEFAULT 1,
    min_interval_seconds NUMERIC(12, 3) NOT NULL DEFAULT 0,
    timeout_ms INTEGER NOT NULL DEFAULT 10000,
    is_active BOOLEAN NOT NULL DEFAULT true,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (exchange = lower(exchange)),
    CHECK (method IN ('GET', 'POST', 'PUT', 'PATCH', 'DELETE')),
    CHECK (max_batch_size > 0),
    CHECK (request_weight > 0),
    CHECK (min_interval_seconds >= 0),
    CHECK (timeout_ms > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_exchange_api_endpoints_identity
    ON exchange_api_endpoints (exchange, market_type, data_type, name);

CREATE INDEX IF NOT EXISTS ix_exchange_api_endpoints_exchange_data_type
    ON exchange_api_endpoints (exchange, data_type);

CREATE INDEX IF NOT EXISTS ix_exchange_api_endpoints_active
    ON exchange_api_endpoints (is_active, exchange, data_type);
