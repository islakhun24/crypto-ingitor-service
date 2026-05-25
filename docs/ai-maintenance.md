# AI Maintenance Guide

## Purpose

This document lets future AI sessions maintain and extend the codebase without reading every file.

---

## Project Structure Map

```
cmd/
  api-service/main.go          # HTTP server (long-running)
  scheduler-service/main.go    # Job planner (one-shot)
  collector-service/main.go    # Job worker (one-shot)
  retention-service/main.go    # Cleanup engine (one-shot)

internal/
  config/                      # Env config + validation
  database/                    # Postgres wrapper
  logger/                      # JSON logger
  symbols/                     # Symbol registry (pre-existing symbols table)
  endpoints/                   # Endpoint registry repository
  scheduler/                   # Planner, Worker, backfill, recovery
  exchanges/
    all/registry.go            # Exchange name → adapter instance
    common/                    # BaseAdapter, request builder, parse helpers
    binance/adapter.go         # Per-exchange normalizer
    okx/adapter.go
    bybit/adapter.go
    bitget/adapter.go
    gate/adapter.go
    mexc/adapter.go
  collectors/
    core/                      # Main Collector, writer, router, multi-writer
    aggregate/                 # Aggregate snapshot executor
  normalizers/                 # Canonical structs + validation
  repositories/                # Postgres writers per data type
  hardening/                   # Error classification, validation filters
  ratelimit/                   # Token bucket, circuit breaker, allocator
  realtime/                    # Redis + memory fallback store
  retention/                   # Engine, planner, rollup, store, partitions
  observability/               # Prometheus metrics repository
  quality/                     # Gap detection helpers
  api/derivatives/             # REST handlers, DTOs, repositories
  aggregation/                 # Cross-exchange aggregate models
  integration/                 # E2E tests
```

---

## File Ownership Map

| Concern | Owner Files |
|---------|-------------|
| New exchange adapter | `internal/exchanges/<name>/adapter.go`, `internal/exchanges/<name>/adapter_test.go`, `internal/exchanges/all/registry.go` |
| New endpoint definition | `migrations/000007_seed_exchange_api_endpoints.sql` (or new migration) |
| New collection policy | `migrations/000008_seed_derivative_collection_policies.sql` (or new migration) |
| New data type | `internal/normalizers/models.go`, `internal/normalizers/validation.go`, `internal/hardening/validation.go`, `internal/collectors/core/writer.go`, `internal/repositories/*.go`, `internal/exchanges/*/adapter.go` |
| Normalizer bug | `internal/exchanges/<exchange>/adapter.go`, `internal/exchanges/<exchange>/adapter_test.go` |
| Scheduler policy change | `migrations/000008_seed_derivative_collection_policies.sql` or `derivative_collection_policies` table |
| Retention policy change | `migrations/000012_phase9_retention_policy_seed.sql` or `data_retention_policies` table |
| API endpoint | `internal/api/derivatives/handler.go`, `internal/api/derivatives/repository.go`, `cmd/api-service/main.go` |
| Database schema | `migrations/*.sql` |
| Config change | `internal/config/config.go` |
| Rate limits | `internal/ratelimit/*.go`, `exchange_api_endpoints` rows |
| Data quality rules | `internal/hardening/validation.go`, `internal/hardening/errors.go` |
| Health/ops endpoints | `internal/api/derivatives/admin.go` |

---

## Where to Add New Endpoint

1. **Add DB rows**: Edit `migrations/000007_seed_exchange_api_endpoints.sql` (or create a new migration)
   - Add `INSERT INTO exchange_api_endpoints (...)` with `ON CONFLICT ... DO UPDATE`
   - Include `base_url`, `path`, `params_template`, rate limits, `is_active = true`
2. **Add normalizer support** (if data type is new): See "Where to Add New Data Type"
3. **Add repository mapping** (if data type is new): `internal/collectors/core/writer.go`
4. **Add tests**: `internal/exchanges/<exchange>/adapter_test.go` with sample JSON
5. **Do NOT** hardcode endpoint URLs in collector code — the collector reads from DB

---

## Where to Add New Exchange

1. **Create package**: `internal/exchanges/<name>/`
2. **Define structs**: Exchange-specific JSON envelope and ticker/data records
3. **Write `Normalize` function**:
   - Guard unsupported data types
   - Check error envelopes
   - Unmarshal body
   - Extract fields, build `normalizers.NormalizedResult`
4. **Write `NewAdapter(client)`**: Return `Adapter{BaseAdapter: excommon.BaseAdapter{...}}`
5. **Register**: Add to `internal/exchanges/all/registry.go`
6. **Add DB rows**: Add endpoints to `migrations/000007_seed_exchange_api_endpoints.sql`
7. **Add policies**: Add to `migrations/000008_seed_derivative_collection_policies.sql`
8. **Add tests**: `internal/exchanges/<name>/adapter_test.go`
9. **Do NOT** modify existing exchange packages or their tests

---

## Where to Add New Data Type

1. **Canonical struct**: `internal/normalizers/models.go`
2. **Validation**: `internal/normalizers/validation.go`
3. **Hardening filter**: `internal/hardening/validation.go` (add loop in `FilterNormalizedResult`)
4. **Writer dispatch**: `internal/collectors/core/writer.go` (add case in `RepositoryWriter.Write`)
5. **Repository method**: `internal/repositories/advanced.go` or new file
6. **Exchange normalizers**: Update each exchange's `adapter.go` to parse and return the new type
7. **Tests**: Add to each exchange's `adapter_test.go`

---

## Where to Fix Normalizer

1. Locate exchange: `internal/exchanges/<exchange>/adapter.go`
2. Find `Normalize` function
3. Fix field mapping, timestamp extraction, or envelope unwrapping
4. Update `internal/exchanges/<exchange>/adapter_test.go` with sample JSON
5. Run: `go test ./internal/exchanges/<exchange>/...`
6. If validation rules need change: `internal/hardening/validation.go` or `internal/normalizers/validation.go`

---

## Where to Update Scheduler Policy

1. **DB seed**: Edit `migrations/000008_seed_derivative_collection_policies.sql`
2. **Runtime update**: `UPDATE derivative_collection_policies SET ...`
3. Key columns: `interval_seconds`, `priority`, `max_retry`, `stale_after_seconds`, `enabled`

---

## Where to Update Retention Policy

1. **DB seed**: Edit `migrations/000012_phase9_retention_policy_seed.sql`
2. **Runtime update**: `UPDATE data_retention_policies SET ...`
3. Key columns: `retention_days`, `chunk_size`, `enabled`, `dry_run`, `rollup_before_delete`, `max_rows_per_run`

---

## Common Bugs and Exact Files to Inspect

### Bug: Data shows wrong values for an exchange
- **Inspect**: `internal/exchanges/<exchange>/adapter.go` — field mapping in normalizer
- **Test**: `internal/exchanges/<exchange>/adapter_test.go`

### Bug: Jobs failing with "endpoint unavailable"
- **Inspect**: `migrations/000007_seed_exchange_api_endpoints.sql` — endpoint missing or inactive
- **Inspect**: `internal/endpoints/repository.go` — resolution logic
- **Fix**: Seed the endpoint or set `is_active = true`

### Bug: All data dropped as quality issues
- **Inspect**: `internal/hardening/validation.go` — `FilterNormalizedResult` rules
- **Inspect**: `internal/normalizers/validation.go` — per-type validators
- **Check**: Future skew, funding bounds, interval mismatch

### Bug: Jobs stuck in `running`
- **Inspect**: `internal/scheduler/recovery.go` — recovery logic
- **Fix**: Collector auto-recovers after timeout; or manually reset in DB

### Bug: Rate limiting / 429 errors
- **Inspect**: `internal/ratelimit/limiter.go`
- **Inspect**: `exchange_api_endpoints` rows — `rate_limit_per_second`, `request_weight`
- **Fix**: Adjust env vars or DB endpoint config

### Bug: Circuit breaker always open
- **Inspect**: `internal/ratelimit/circuit_breaker.go`
- **Inspect**: `internal/hardening/errors.go` — error classification
- **Fix**: Check if errors are wrongly classified as non-recoverable

### Bug: Retention not deleting data
- **Inspect**: `data_retention_policies` — `enabled` and `dry_run` columns
- **Inspect**: `internal/retention/specs.go` — `MinRetentionDays` may block short policies

### Bug: API returns stale aggregates
- **Inspect**: `internal/repositories/aggregate.go` — `MaxSnapshotAge` check
- **Inspect**: `internal/api/derivatives/scan.go` — freshness logic
- **Fix**: Check if source snapshots are stale in `derivative_market_snapshots`

### Bug: Redis realtime missing
- **Inspect**: `internal/realtime/redis.go` — connection
- **Inspect**: `internal/realtime/fallback.go` — fallback to memory is automatic
- **Fix**: Check Redis connectivity; fallback is safe

---

## Rules: Do Not Modify Unrelated Files

1. **Adding an exchange**: Do NOT change existing exchange adapters, existing endpoint seeds, or collector core logic.
2. **Adding an endpoint**: Do NOT hardcode URLs in Go code. Use DB seed only.
3. **Fixing a normalizer**: Do NOT refactor the request builder, BaseAdapter, or writer. Fix only the normalizer function and its test.
4. **Changing scheduler policy**: Do NOT change job claiming SQL, worker concurrency, or rate limiter.
5. **Changing retention policy**: Do NOT change the engine, store, or partition logic unless the bug is there.
6. **Adding a data type**: Do NOT change unrelated normalizers. Only update the ones that will support the new type.
7. **Bugfix**: Modify the smallest scope possible. Add a regression test. Avoid refactoring unless explicitly asked.

---

## Quick Reference: Edit These Files For...

| Task | Files |
|------|-------|
| Add exchange | `internal/exchanges/<name>/adapter.go`, `adapter_test.go`, `internal/exchanges/all/registry.go`, `migrations/000007_*.sql`, `migrations/000008_*.sql` |
| Add endpoint | `migrations/000007_*.sql` (or new migration) |
| Add data type | `internal/normalizers/models.go`, `validation.go`, `internal/hardening/validation.go`, `internal/collectors/core/writer.go`, `internal/repositories/*.go`, `internal/exchanges/*/adapter.go` |
| Fix normalizer | `internal/exchanges/<exchange>/adapter.go`, `adapter_test.go` |
| Update scheduler | `migrations/000008_*.sql` or `derivative_collection_policies` table |
| Update retention | `migrations/000012_*.sql` or `data_retention_policies` table |
| Fix config | `internal/config/config.go` |
| Fix API | `internal/api/derivatives/handler.go`, `repository.go` |
| Fix DB issue | `migrations/*.sql` |
