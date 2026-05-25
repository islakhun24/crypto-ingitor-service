# Architecture Overview

## What This Project Is

A crypto derivative data aggregator written in Go. It collects, normalizes, stores, and serves exchange-normalized derivative market data from major exchanges (Binance, OKX, Bybit, Bitget, Gate, MEXC).

## Services

Four standalone binaries in `cmd/`:

| Service | Type | Purpose |
|---------|------|---------|
| `api-service` | Long-running daemon | HTTP REST API server exposing derivative data, health checks, Prometheus metrics, symbol/endpoint lookup, and realtime endpoints. |
| `scheduler-service` | One-shot batch | Reads collection policies and symbols, generates idempotent jobs into `derivative_collection_jobs`. Run periodically via cron/K8s CronJob. |
| `collector-service` | One-shot batch | Claims pending jobs, executes HTTP requests against exchange APIs, normalizes responses, writes to Postgres + Redis. Run periodically. |
| `retention-service` | One-shot batch | Applies retention policies: rollups, chunked deletes, partition drops, and audits. Run periodically. |

## Data Flow

```
scheduler-service → derivative_collection_jobs (Postgres)
                         ↓
collector-service → exchange APIs → normalize → Postgres + Redis
                         ↓
     api-service ← Postgres + Redis
                         ↓
retention-service → cleanup/rollup old data
```

1. **Scheduler** reads `collection_policies` + `symbols` → inserts rows into `derivative_collection_jobs`.
2. **Collector** claims pending jobs → uses `endpoints` registry + exchange adapters → performs HTTP calls → normalizes JSON into `normalizers.NormalizedResult` → writes via `repositories` into Postgres tables and `realtime` Redis/memory store.
3. **API Service** reads from Postgres (via `api/derivatives` repository) and Redis (via `realtime.Store`) to serve REST requests.
4. **Retention Service** reads `retention_policies` → performs rollups/deletes/partition drops → logs results in `data_cleanup_runs`.

## Key Design Patterns

- **Job Scheduler + Idempotent Worker**: Jobs have deterministic idempotency keys. Workers claim batches with `FOR UPDATE SKIP LOCKED`.
- **Exchange Adapter Registry**: Central `all.Registry` maps exchange names to adapter instances. Adding a new exchange = new package + register.
- **Normalized Data Model**: All exchange responses parse into canonical `normalizers.NormalizedResult` with strongly typed slices.
- **Multi-Writer**: Collector writes to both Postgres (persistent) and Redis/Memory (realtime latest).
- **Hardening & Quality Gates**: `hardening.FilterNormalizedResult` validates every row before persistence. Invalid rows become `QualityIssue` records.
- **Circuit Breaker + Rate Limiting**: Per-exchange token-bucket limiter and circuit breaker.
- **Retention with Rollups**: Kline rollups before deletion, partition dropping as fast path.
- **Minimal Dependencies**: Only `github.com/lib/pq`. No frameworks (no Gin, no GORM). Uses stdlib `net/http`, `database/sql`, `encoding/json`.

## Directory Map

```
cmd/
  api-service/main.go          # HTTP server entrypoint
  scheduler-service/main.go    # Job planner entrypoint
  collector-service/main.go    # Job worker entrypoint
  retention-service/main.go    # Cleanup engine entrypoint

internal/
  config/                      # Env-driven config loading + validation
  database/                    # Thin postgres wrapper (lib/pq)
  logger/                      # Structured JSON logger
  symbols/                     # Symbol registry repository
  endpoints/                   # Endpoint registry repository
  scheduler/                   # Planner, Worker, job models, backfill, recovery
  exchanges/
    all/registry.go            # Static adapter registry
    common/                    # BaseAdapter, request builder, parsing helpers
    {binance,okx,bybit,bitget,gate,mexc}/  # Per-exchange normalizers
  collectors/
    core/                      # Main Collector orchestrator, writer, router
    aggregate/                 # Aggregate snapshot executor
  normalizers/                 # Canonical structs + validation
  repositories/                # Postgres writers per data type
  hardening/                   # Error classification, validation, filtering
  ratelimit/                   # Token bucket, circuit breaker, budget allocator
  realtime/                    # Redis + in-memory fallback store
  retention/                   # Engine, planner, rollup, store, partitions
  observability/               # Prometheus metrics repository
  quality/                     # Gap/issue detection helpers
  api/derivatives/             # HTTP handlers, DTOs, repositories for REST API
  aggregation/                 # Cross-exchange aggregate models
  integration/                 # End-to-end flow tests
```
