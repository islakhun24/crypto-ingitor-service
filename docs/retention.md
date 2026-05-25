# Retention Guide

## Purpose

The retention-service applies data lifecycle policies: rollups, chunked deletes, partition drops, and audits. It is a one-shot batch job — schedule it via cron or Kubernetes CronJob.

## Core Components

| File | Role |
|------|------|
| `internal/retention/engine.go` | Orchestrates full retention run |
| `internal/retention/planner.go` | Sorts policies, computes cutoff, decides partition drop vs delete |
| `internal/retention/rollup.go` | Kline rollups before deletion (1m→5m→15m→1h→4h→1d) |
| `internal/retention/store.go` | DB layer: list policies, count, delete chunks, partition ops |
| `internal/retention/sql.go` | Builds COUNT and DELETE queries |
| `internal/retention/partitions.go` | Parses monthly/weekly partition names |
| `internal/retention/specs.go` | Hard-coded `TableSpec` per table with safety minimums |
| `internal/retention/metrics.go` | Prometheus-style counters |

## Cleanup Flow

1. **Validate** policy against `TableSpec` (safety minimums)
2. **Start** `data_cleanup_runs` row
3. **Count** eligible rows
4. If `RollupBeforeDelete` → run rollup (only for `derivative_klines`)
5. If `UsePartitionDrop` → `DROP TABLE` eligible child partitions (fast)
6. Otherwise, **chunked DELETE** in loops of `ChunkSize` up to `MaxRowsPerRun`
7. **Finish** cleanup run row

## Safety Guards

- `MinRetentionDays` per table spec prevents accidentally short retention
- `DiskPressureCritical` config can stop cleanup entirely
- `DryRun` supported per policy — counts rows without deleting
- Each policy has `TimeoutSeconds`

## Configuration

Env vars:
- `RETENTION_MAX_ROWS_PER_RUN` (default 50,000)
- `RETENTION_TABLE_TIMEOUT_SECONDS` (default 120)
- `RETENTION_DISK_PRESSURE_CRITICAL` (default false)

## Tables

- `data_retention_policies` — one row per (table, interval)
- `data_cleanup_runs` — audit log
- `data_rollup_runs` — rollup audit log

## Seeded Policies

Migration `000012_phase9_retention_policy_seed.sql` seeds ~20 policies covering:
- `derivative_klines` by interval with rollup chains
- Snapshot tables (market, OI, funding, long/short, taker flow, CVD)
- Event/aggregate tables (liquidations)
- Orderbook tables
- Ops tables (request logs, raw payloads, failed jobs, quality issues)

All default to `dry_run = true`. Enable by updating the row:
```sql
UPDATE data_retention_policies SET dry_run = false WHERE table_name = 'derivative_klines';
```

## Running

```bash
make run-retention
# or
go run ./cmd/retention-service
```

## Operational Commands

See `docs/troubleshooting.md` for:
- Running retention dry-run
- Checking cleanup history
- Enabling/disabling policies
