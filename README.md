# Aggregator Services

Crypto derivative aggregator services for exchange-normalized derivative market data.

## Phase 1 Status

Implemented the foundation for:

- Environment-driven PostgreSQL configuration.
- App env, app port, Redis, collection mode, worker concurrency, and conservative rate-limit defaults.
- PostgreSQL connection setup without hardcoded credentials in Go source.
- Configurable PostgreSQL pool settings and context-aware ping/close.
- Existing `symbols.markets` JSONB loader.
- Runtime filtering for Binance, OKX, Bybit, Bitget, Gate, and MEXC.
- Strict use of `markets[*].source_symbol` for exchange symbols.
- Minimal HTTP entrypoint for health checks and active market mapping inspection.
- Feature-based project structure for the later scheduler, collector, retention, endpoint, quality, API, and observability phases.
- Phase 2 migrations for derivative tables, endpoint registry, scheduler, quality, monitoring, retention, and rollup state.
- Phase 3 endpoint seed migration for Binance, OKX, Bybit, Bitget, Gate, and MEXC.
- Endpoint repository for resolving active DB-backed endpoint definitions by exchange and data type.
- Phase 4 scheduler planner, symbol-tier loading, deterministic job idempotency keys, job claiming skeleton, rate limiter, and circuit breaker.
- Phase 5 exchange adapter skeletons, DB endpoint request builder, HTTP executor, adapter registry, and common normalizer models.
- Phase 6 core collector executor, idempotent write repositories, request logging, collector health updates, and basic aggregate snapshots.
- Phase 7 advanced derivative repositories for long/short ratio, taker flow, recalculable CVD, liquidation filtering/aggregation, basis, orderbook imbalance, divergence, and aggregate enrichment.
- Phase 8 analytics layer for windowed derivative metrics, non-signal market structure, volatility snapshots, divergence summaries, quality metadata, and JSONB anomaly flags without trade scoring.
- Phase 9 retention policies, dry-run cleanup, chunked delete, kline rollups, cleanup audit runs, partition-drop support, and Prometheus-style cleanup metrics.
- Phase 10 production hardening for error classification, exponential retry, richer dead letters, invalid-row quarantine, stale/gap detection helpers, restart recovery, safe backfill planning, rate-limit budget allocation, and `/metrics`.
- Phase 11 DTO-based REST API for derivative terminal overview, symbol detail, series data, health, jobs, quality issues, and gaps under `/api/v1/derivatives`.
- Phase 12 Redis/memory realtime latest layer, Redis key policy, reconnect state helpers, REST fallback hooks, periodic snapshot buffering, and realtime read endpoints without writing raw websocket ticks to Postgres.
- Phase 13 production build/deploy packaging with Docker Compose, Prometheus config, JSON logs, `/readyz`, Make targets, local env files, and deployment checklist.

The existing `symbols` table is not recreated by this service.

## Run

Copy `.env.example` values into your runtime environment, then run:

```bash
go run ./cmd/api-service
```

Run the scheduler planner once:

```bash
go run ./cmd/scheduler-service
```

Run one collector worker batch:

```bash
go run ./cmd/collector-service
```

Run one retention/cleanup batch:

```bash
go run ./cmd/retention-service
```

Docker Compose local stack:

```bash
cp .env.example .env.local
make docker-up
make migrate
make seed-endpoints
```

If an existing `exchange-normalizer-postgres` container is already on a Docker network, set `POSTGRES_HOST=exchange-normalizer-postgres`, `AGGREGATOR_NETWORK_NAME=<network>`, and `AGGREGATOR_NETWORK_EXTERNAL=true` in `.env.local`.

Available endpoints:

- `GET /healthz`
- `GET /readyz`
- `GET /metrics`
- `GET /api/v1/derivatives/overview`
- `GET /api/v1/derivatives/symbols`
- `GET /api/v1/derivatives/symbols/{symbol}`
- `GET /api/v1/derivatives/symbols/{symbol}/market`
- `GET /api/v1/derivatives/symbols/{symbol}/klines`
- `GET /api/v1/derivatives/symbols/{symbol}/open-interest`
- `GET /api/v1/derivatives/symbols/{symbol}/funding`
- `GET /api/v1/derivatives/symbols/{symbol}/long-short-ratio`
- `GET /api/v1/derivatives/symbols/{symbol}/taker-flow`
- `GET /api/v1/derivatives/symbols/{symbol}/cvd`
- `GET /api/v1/derivatives/symbols/{symbol}/liquidations`
- `GET /api/v1/derivatives/symbols/{symbol}/basis`
- `GET /api/v1/derivatives/symbols/{symbol}/orderbook-imbalance`
- `GET /api/v1/derivatives/symbols/{symbol}/exchange-divergence`
- `GET /api/v1/derivatives/health/collectors`
- `GET /api/v1/derivatives/health/exchanges`
- `GET /api/v1/derivatives/jobs`
- `GET /api/v1/derivatives/quality/issues`
- `GET /api/v1/derivatives/quality/gaps`
- `GET /api/v1/derivatives/realtime/latest/{kind}/{exchange}/{source_symbol}`
- `GET /api/v1/derivatives/realtime/aggregate/{symbol_id}`
- `GET /api/v1/derivatives/realtime/ws-state/{exchange}/{stream}`
- `GET /symbols`
- `GET /symbols/{id}`
- `GET /symbols/top?limit=50`
- `GET /symbols/watchlist`
- `GET /symbols/exchange/{exchange}`
- `GET /symbols/markets`
- `GET /endpoints/{exchange}/{data_type}`
- `GET /endpoints/{exchange}/{market_type}/{data_type}/{name}`

## Test

```bash
go test ./...
```

## Build Commands

- `make build`
- `make test`
- `make migrate`
- `make seed-endpoints`
- `make run-api`
- `make run-scheduler`
- `make run-collector`
- `make run-retention`
- `make docker-up`
- `make docker-down`

Production checklist: [deploy/production-checklist.md](deploy/production-checklist.md).
