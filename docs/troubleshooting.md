# Troubleshooting & Operations Runbook

## Operational Commands

### Checking Job Backlog

```sql
SELECT status, COUNT(*) FROM derivative_collection_jobs GROUP BY status;
```

Or via API:
```bash
curl "http://localhost:8080/api/v1/derivatives/jobs?status=pending&limit=100"
```

### Checking Failed Jobs

```sql
SELECT * FROM failed_collection_jobs ORDER BY created_at DESC LIMIT 50;
```

Or via API:
```bash
curl "http://localhost:8080/api/v1/derivatives/jobs?status=dead_letter&limit=100"
```

### Retrying Failed Jobs

Reset dead-letter jobs back to pending:
```sql
UPDATE derivative_collection_jobs
SET status = 'pending',
    retry_count = 0,
    error_message = NULL,
    next_retry_at = NULL,
    scheduled_at = now()
WHERE status = 'dead_letter'
  AND exchange = 'binance'
  AND scheduled_at > now() - interval '24 hours';
```

Then run the collector.

### Checking Stale Data

```sql
-- Latest snapshot per exchange older than 15 minutes
SELECT exchange, MAX(snapshot_time) as latest
FROM derivative_market_snapshots
GROUP BY exchange
HAVING MAX(snapshot_time) < now() - interval '15 minutes';
```

Or check data gaps via API:
```bash
curl "http://localhost:8080/api/v1/derivatives/quality/gaps?backfill_status=pending"
```

### Checking Data Gaps

```sql
SELECT * FROM data_gaps
WHERE backfill_status = 'pending'
ORDER BY gap_start DESC
LIMIT 50;
```

### Running Retention Dry-Run

All seeded policies default to `dry_run = true`. Run retention:
```bash
make run-retention
# or
go run ./cmd/retention-service
```

Review `data_cleanup_runs`:
```sql
SELECT run_key, table_name, dry_run, rows_matched, rows_deleted, status, started_at
FROM data_cleanup_runs
ORDER BY started_at DESC
LIMIT 20;
```

### Running Cleanup (Enable Dry-Run Off)

```sql
UPDATE data_retention_policies
SET dry_run = false
WHERE table_name = 'derivative_klines' AND interval_filter_value = '1m';
```

Then run retention service.

### Seeding Endpoints

```bash
make seed-endpoints
```

This applies:
- `000007_seed_exchange_api_endpoints.sql`
- `000008_seed_derivative_collection_policies.sql`

### Pausing an Exchange

Disable all endpoints for an exchange:
```sql
UPDATE exchange_api_endpoints
SET is_active = false
WHERE exchange = 'mexc';
```

Disable collection policies:
```sql
UPDATE derivative_collection_policies
SET enabled = false
WHERE exchange = 'mexc';
```

### Disabling an Endpoint

```sql
UPDATE exchange_api_endpoints
SET is_active = false
WHERE exchange = 'binance' AND data_type = 'funding' AND name = 'funding_rate';
```

## Common Issues

### Jobs stuck in `running`

The collector auto-recovers interrupted jobs on startup after `RECOVERY_RUNNING_JOB_TIMEOUT_SECONDS` (default 900s / 15m). To force recovery:
```sql
UPDATE derivative_collection_jobs
SET status = 'pending',
    started_at = NULL
WHERE status = 'running'
  AND started_at < now() - interval '15 minutes';
```

### Circuit breaker tripped

Wait for cooldown (`CIRCUIT_BREAKER_COOLDOWN_SECONDS`, default 30s), or reset by restarting collector. Check `exchange_request_logs` for failure pattern.

### Rate limited (429)

Check `RATE_LIMIT_REQUESTS_PER_SECOND` and `RATE_LIMIT_BURST` env vars. Review `exchange_api_endpoints.rate_limit_per_second` per endpoint.

### Data quality issues spike

Check `data_quality_issues` table or API:
```bash
curl "http://localhost:8080/api/v1/derivatives/quality/issues?status=open&limit=100"
```

Common causes:
- Exchange response format changed → fix normalizer in `internal/exchanges/<exchange>/adapter.go`
- Symbol mapping wrong → check `symbols.markets` JSONB
- Future timestamp skew → check `DATA_MAX_FUTURE_SKEW_SECONDS`
- Funding rate out of bounds → check `FUNDING_SANITY_MIN` / `FUNDING_SANITY_MAX`

### Postgres connection errors

Check pool settings:
- `POSTGRES_MAX_OPEN_CONNS`
- `POSTGRES_MAX_IDLE_CONNS`
- `POSTGRES_CONN_MAX_LIFETIME_SECONDS`

Check for connection leaks (long-running transactions).

### Redis unavailable

The realtime store falls back to in-memory automatically. Check Redis with:
```bash
redis-cli ping
```

### Metrics not showing

Ensure Prometheus is scraping `/metrics`. Check `deploy/prometheus.yml` target.

## Logs

All services output structured JSON logs to stdout. Docker Compose logs:
```bash
make docker-logs
```

## Health Checks

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

Container healthcheck:
```bash
docker exec <api-container> /app/api-service --healthcheck
```
