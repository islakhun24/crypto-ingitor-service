# Database Guide

## Connection

- Driver: `github.com/lib/pq` (raw `database/sql`, no ORM)
- Connection pooling configured via env vars:
  - `POSTGRES_MAX_OPEN_CONNS` (default 20)
  - `POSTGRES_MAX_IDLE_CONNS` (default 10)
  - `POSTGRES_CONN_MAX_LIFETIME_SECONDS` (default 1800)
  - `POSTGRES_CONN_MAX_IDLE_SECONDS` (default 300)

## Migrations

Located in `migrations/`. Applied via `make migrate` (runs `psql -f` inside the Postgres container). No Go migration framework.

All DDL is idempotent (`IF NOT EXISTS`). There are 14 ordered migration files.

### Key Migration Files

| File | Purpose |
|------|---------|
| `000001_exchange_api_endpoints.sql` | Endpoint registry table |
| `000002_scheduler_tables.sql` | Collection policies, tiers, jobs |
| `000003_core_derivative_tables.sql` | Market snapshots, klines, OI, funding |
| `000004_advanced_derivative_tables.sql` | Long/short, taker flow, CVD, liquidations, basis, orderbook, divergence |
| `000005_monitoring_quality_tables.sql` | Health, request logs, failed jobs, quality issues, data gaps |
| `000006_retention_rollup_tables.sql` | Retention policies, cleanup runs, rollup runs |
| `000007_seed_exchange_api_endpoints.sql` | Seed endpoints for 6 exchanges |
| `000008_seed_derivative_collection_policies.sql` | Seed default collection policies |
| `000009_aggregate_snapshot_columns.sql` | Add aggregate columns |
| `000010_phase7_advanced_columns.sql` | Enrich advanced tables |
| `000011_phase8_analytics_layer.sql` | Analytics JSONB columns |
| `000012_phase9_retention_policy_seed.sql` | Seed retention policies (all dry_run=true) |
| `000013_phase10_production_hardening.sql` | Backfill metadata, safe payloads, partial indexes |
| `000014_phase11_api_indexes.sql` | Read-path covering indexes |

## Core Tables

### Job Queue
- `derivative_collection_policies` — what to collect, how often, for which tier
- `derivative_collection_jobs` — work queue with idempotency keys
- `failed_collection_jobs` — dead-letter queue
- `data_collection_runs` — batch run metadata

### Time-Series Data
- `derivative_market_snapshots` — latest per-exchange market state
- `derivative_klines` — OHLCV candles
- `open_interest_snapshots` / `open_interest_history`
- `funding_rate_snapshots` / `funding_rate_history`
- `long_short_ratio_snapshots`
- `taker_flow_snapshots`
- `cvd_snapshots`
- `liquidation_events` / `liquidation_aggregates`
- `basis_premium_snapshots`
- `orderbook_imbalance_snapshots` / `orderbook_depth_snapshots`
- `exchange_divergence_snapshots`
- `derivative_aggregated_snapshots` — cross-exchange aggregated view
- `market_structure_snapshots` / `volatility_snapshots`

### Observability
- `collector_health` — per-service heartbeat
- `exchange_request_logs` — every HTTP request
- `data_quality_issues` — validation anomalies
- `data_gaps` — missing data intervals

### Retention
- `data_retention_policies` — cleanup rules per table
- `data_cleanup_runs` — audit log of cleanups
- `data_rollup_runs` — audit log of rollups

### Pre-existing
- `symbols` — assumed to already exist. Referenced by FKs from nearly every table.

## Query Patterns

- Heavy use of `ON CONFLICT (...) DO UPDATE SET ...` for idempotent ingestion
- JSONB for raw payloads, metadata, metrics, anomaly flags
- `FOR UPDATE SKIP LOCKED` for job claiming
- Dynamic SQL building with `strings.Builder` and positional `$N` parameters in API repository
- `sql.NullFloat64`, `sql.NullString`, `sql.NullInt64`, `sql.NullTime` extensively used
