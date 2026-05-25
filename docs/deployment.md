# Deployment Guide

## Requirements

- Docker & Docker Compose
- PostgreSQL 16+ (or use provided container)
- Redis 7+ (or use provided container)
- Go 1.26+ (for local builds)

## Environment

Copy `.env.example` to `.env.local` and customize:
```bash
cp .env.example .env.local
```

Required env vars:
- `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`, `POSTGRES_SSLMODE`
- `SUPPORTED_EXCHANGES`

## Docker Compose Stack

```bash
make docker-up    # Builds image, starts postgres, redis, api-service, prometheus
make migrate      # Applies all SQL migrations
make seed-endpoints  # Seeds endpoints and collection policies
```

Services defined:
- `postgres` — primary store
- `redis` — realtime cache
- `api-service` — REST API (long-running)
- `scheduler-service` — one-shot planner (`restart: "no"`)
- `collector-service` — one-shot worker (`restart: "no"`)
- `retention-service` — one-shot cleanup (`restart: "no"`)
- `prometheus` — metrics scraping
- `grafana` — dashboards (optional profile)

## Build

```bash
make build   # Builds all 4 binaries
make test    # Runs all tests
```

## Dockerfile

Multi-stage build:
1. `golang:1.26-bookworm` — compile 4 binaries with `-trimpath -ldflags="-s -w"`
2. `gcr.io/distroless/static-debian12:nonroot` — minimal runtime image

## Production Checklist

See `deploy/production-checklist.md`:

- [ ] DB migration applied with `make migrate`
- [ ] Endpoint registry seeded with `make seed-endpoints`
- [ ] Collection policies seeded
- [ ] Retention dry-run executed and reviewed
- [ ] Rate limit config checked per exchange
- [ ] Docker logs capped with `max-size=50m` and `max-file=3`
- [ ] `/healthz` returns OK
- [ ] `/readyz` returns ready
- [ ] `/metrics` is scraped by Prometheus
- [ ] Redis is reachable and memory policy is applied
- [ ] Collector health visible at `/api/v1/derivatives/health/collectors`
- [ ] Free disk space checked for Postgres, Redis, and Docker volumes
- [ ] Ubuntu firewall allows only required published ports
- [ ] Backups and restore procedure tested for Postgres volumes

## Scheduling in Production

The scheduler, collector, and retention services are **one-shot batch jobs**. In production, trigger them externally:

- **Kubernetes**: CronJobs for scheduler and retention; Deployment with HPA for collector (or CronJob if single-worker)
- **systemd**: Timer units
- **crontab**: Simple cron entries

Example cron:
```cron
# Scheduler every minute
* * * * * /app/scheduler-service

# Collector every minute
* * * * * /app/collector-service

# Retention daily at 2 AM
0 2 * * * /app/retention-service
```

## Connecting to Existing Postgres

If an existing `exchange-normalizer-postgres` container is already on a Docker network, set in `.env.local`:
```
POSTGRES_HOST=exchange-normalizer-postgres
AGGREGATOR_NETWORK_NAME=<network>
AGGREGATOR_NETWORK_EXTERNAL=true
```
