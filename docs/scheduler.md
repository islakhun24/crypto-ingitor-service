# Scheduler & Collector Guide

## Scheduler-Service

### Purpose
One-shot planner that generates collection jobs. Run it periodically (e.g., every 30s or 1m) via cron or Kubernetes CronJob.

### Flow
1. Load active policies from `derivative_collection_policies`
2. Resolve target symbols by tier (`all`, `top100`, `watchlist`)
3. Look up active endpoint for `(exchange, data_type)`
4. Compute time bucket (`ScheduledBucket`)
5. Generate idempotency key and create `Job` structs
6. Bulk-insert into `derivative_collection_jobs` with `ON CONFLICT (idempotency_key) DO NOTHING`

### Job Model
Key fields:
- `exchange`, `data_type`, `tier`, `symbol_id`, `source_symbol`, `period`
- `idempotency_key` — deterministic, prevents duplicates
- `status`: `pending` → `running` → `succeeded` | `failed` | `dead_letter`
- `priority` — lower number = higher priority
- `scheduled_at` — the time bucket the job targets
- `retry_count`, `max_retry`
- `job_mode`: `realtime` or `backfill`
- `parent_gap_id` — links backfill jobs to `data_gaps`

### Tier Resolution
- `all`: Every active symbol with a market mapping for the exchange
- `top100`: First 100 active symbols ordered by `cmc_rank ASC`
- `watchlist`: Symbols in `symbol_collection_tiers` with `tier = 'watchlist'`

### Running
```bash
make run-scheduler
# or
go run ./cmd/scheduler-service
```

## Collector-Service

### Purpose
One-shot worker that claims and executes jobs. Run it periodically.

### Flow
1. **Recover** interrupted jobs older than `RECOVERY_RUNNING_JOB_TIMEOUT` (default 15m)
2. **Claim** batch of `pending` jobs with `FOR UPDATE SKIP LOCKED`
3. For each job: check circuit breaker + rate limiter
4. Resolve endpoint from registry
5. Look up exchange adapter
6. `BuildRequest` → `Execute` → `Normalize`
7. Write normalized result via `ResultWriter`
8. Mark job `succeeded`, or retry/fail

### Error Classification
| Type | Recoverable? | Action |
|------|-------------|--------|
| `rate_limited` | Yes | Retry with backoff |
| `server_error` | Yes | Retry with backoff |
| `timeout` | Yes | Retry with backoff |
| `network_error` | Yes | Retry with backoff |
| `bad_request` | No | Dead letter |
| `unauthorized` | No | Dead letter |
| `not_found` | No | Dead letter |
| `invalid_symbol` | No | Dead letter |
| `invalid_response` | No | Dead letter |
| `normalizer_error` | No | Dead letter |

### Retry Policy
- Exponential backoff, base 30s, max 15m
- Deterministic SHA1-based jitter (0–15s)
- Retry while `recoverable && retryCount < maxRetry`

### Running
```bash
make run-collector
# or
go run ./cmd/collector-service
```

## Backfill

`internal/scheduler/backfill.go` provides `PlanBackfillJobs()`:
- Takes `BackfillRequest` with `start`, `end`, `interval_seconds`
- Generates jobs across the time range with `job_mode = "backfill"`
- Respects `MaxJobs` limit (default 1000) and `RealtimeLagPauseAfter`

## Recovery on Startup

The collector calls `RecoverInterrupted()`:
- Resets `running` jobs older than timeout back to `pending`
- Marks interrupted `data_collection_runs` as `failed`

## Key Files

| File | Responsibility |
|------|---------------|
| `cmd/scheduler-service/main.go` | Entrypoint |
| `cmd/collector-service/main.go` | Entrypoint |
| `internal/scheduler/planner.go` | Job generation |
| `internal/scheduler/worker.go` | Job execution |
| `internal/scheduler/repository.go` | Job DB operations |
| `internal/scheduler/backfill.go` | Backfill planning |
| `internal/scheduler/recovery.go` | Crash recovery |
| `internal/collectors/core/collector.go` | Main orchestrator |
| `internal/collectors/core/writer.go` | DB + realtime writes |
| `internal/ratelimit/limiter.go` | Token bucket |
| `internal/ratelimit/circuit_breaker.go` | Circuit breaker |
