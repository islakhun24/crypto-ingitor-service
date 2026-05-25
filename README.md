# Aggregator Services

Crypto derivative aggregator services for exchange-normalized derivative market data.

## Phase Status

Implemented the foundation for:

- Environment-driven PostgreSQL configuration.
- App env, app port, Redis, collection mode, worker concurrency, and conservative rate-limit defaults.
- PostgreSQL connection setup without hardcoded credentials in Go source.
- Configurable PostgreSQL pool settings and context-aware ping/close.
- Existing `symbols.markets` JSONB loader.
- Runtime filtering for Binance, OKX, Bybit, Bitget, Gate, and MEXC.
- Strict use of `markets[*].source_symbol` for exchange symbols.
- HTTP entrypoint with health checks (`/healthz`, `/readyz`) and Prometheus metrics (`/metrics`).
- Feature-based project structure for the scheduler, collector, retention, endpoint, quality, API, and observability phases.
- Phase 2 migrations for derivative tables, endpoint registry, scheduler, quality, monitoring, retention, and rollup state.
- Phase 3 endpoint seed migration for Binance, OKX, Bybit, Bitget, Gate, and MEXC.
- Endpoint repository for resolving active DB-backed endpoint definitions by exchange and data type.
- Phase 4 scheduler planner, symbol-tier loading, deterministic job idempotency keys, job claiming, rate limiter, and circuit breaker.
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

## Quick Start

### Docker Compose (Recommended)

```bash
cp .env.example .env.local
make docker-up    # Starts postgres, redis, api-service, prometheus
make migrate      # Applies all SQL migrations
make seed-endpoints  # Seeds endpoints and collection policies
```

Verify: `curl http://localhost:8080/healthz`

### Local Development

See [docs/local-development.md](docs/local-development.md) for detailed setup.

```bash
# Prerequisites: Go 1.23+, PostgreSQL 16+, Redis 7+

# 1. Setup env
export $(grep -v '^#' .env.local | xargs)

# 2. Start PostgreSQL & Redis (Docker)
docker compose up -d postgres redis

# 3. Apply migrations
make migrate

# 4. Start API server (terminal 1)
make run-api

# 5. Run scheduler + collector (terminal 2, loop)
while true; do make run-scheduler && make run-collector; sleep 60; done
```

## Services

| Service | Type | Command | Purpose |
|---------|------|---------|---------|
| `api-service` | Long-running daemon | `make run-api` | HTTP REST API server |
| `scheduler-service` | One-shot batch | `make run-scheduler` | Generates collection jobs |
| `collector-service` | One-shot batch | `make run-collector` | Executes jobs, fetches exchange data |
| `retention-service` | One-shot batch | `make run-retention` | Data cleanup and retention |

## API Endpoints

### Health & Ops

- `GET /healthz` - Health check
- `GET /readyz` - Readiness check
- `GET /metrics` - Prometheus metrics

### Derivatives API (`/api/v1/derivatives`)

- `GET /api/v1/derivatives/overview` - Market overview
- `GET /api/v1/derivatives/symbols` - Symbol list
- `GET /api/v1/derivatives/symbols/{symbol}` - Symbol detail
- `GET /api/v1/derivatives/symbols/{symbol}/market` - Market snapshots
- `GET /api/v1/derivatives/symbols/{symbol}/klines` - OHLCV data
- `GET /api/v1/derivatives/symbols/{symbol}/open-interest` - Open interest
- `GET /api/v1/derivatives/symbols/{symbol}/funding` - Funding rates
- `GET /api/v1/derivatives/symbols/{symbol}/long-short-ratio` - Long/short ratio
- `GET /api/v1/derivatives/symbols/{symbol}/taker-flow` - Taker flow
- `GET /api/v1/derivatives/symbols/{symbol}/cvd` - CVD
- `GET /api/v1/derivatives/symbols/{symbol}/liquidations` - Liquidations
- `GET /api/v1/derivatives/symbols/{symbol}/basis` - Basis/premium
- `GET /api/v1/derivatives/symbols/{symbol}/orderbook-imbalance` - Orderbook
- `GET /api/v1/derivatives/symbols/{symbol}/exchange-divergence` - Divergence
- `GET /api/v1/derivatives/realtime/latest/{kind}/{exchange}/{symbol}` - Realtime
- `GET /api/v1/derivatives/realtime/aggregate/{symbol_id}` - Aggregate
- `GET /api/v1/derivatives/health/collectors` - Collector health
- `GET /api/v1/derivatives/jobs` - Job queue
- `GET /api/v1/derivatives/quality/issues` - Quality issues
- `GET /api/v1/derivatives/quality/gaps` - Data gaps

### Reference Data

- `GET /symbols` - Active symbols
- `GET /symbols/{id}` - Symbol by ID
- `GET /symbols/top?limit=N` - Top symbols
- `GET /endpoints/{exchange}/{data_type}` - Active endpoints

## Documentation

| Document | Description |
|----------|-------------|
| [docs/local-development.md](docs/local-development.md) | Panduan development lokal |
| [docs/ssh-deploy.md](docs/ssh-deploy.md) | Deploy ke VPS via SSH |
| [docs/deployment.md](docs/deployment.md) | Deployment guide (Docker, K8s, systemd) |
| [docs/architecture.md](docs/architecture.md) | Architecture overview |
| [docs/api.md](docs/api.md) | API documentation |
| [docs/scheduler.md](docs/scheduler.md) | Scheduler documentation |
| [docs/endpoint-registry.md](docs/endpoint-registry.md) | Endpoint registry guide |
| [docs/exchange-adapters.md](docs/exchange-adapters.md) | Exchange adapter guide |
| [docs/normalizers.md](docs/normalizers.md) | Normalizer documentation |
| [docs/retention.md](docs/retention.md) | Retention policies |
| [docs/troubleshooting.md](docs/troubleshooting.md) | Troubleshooting runbook |
| [docs/github-actions-runner.md](docs/github-actions-runner.md) | Setup GitHub Actions self-hosted runner |
| [deploy/production-checklist.md](deploy/production-checklist.md) | Production checklist |

## CI/CD (GitHub Actions)

| Workflow | File | Trigger | Runner |
|----------|------|---------|--------|
| **CI** | `.github/workflows/ci.yml` | Push/PR ke `main`/`develop` | GitHub-hosted (`ubuntu-latest`) |
| **Deploy** | `.github/workflows/deploy-local.yml` | Push ke `main`, manual dispatch | Self-hosted runner |
| **Jobs** | `.github/workflows/jobs-local.yml` | Cron setiap 2 menit, manual | Self-hosted runner |

### Setup Self-Hosted Runner

Lihat [docs/github-actions-runner.md](docs/github-actions-runner.md) untuk panduan lengkap.

Quick setup:
```bash
# 1. Register runner di GitHub (Settings → Actions → Runners → New)

# 2. Install di server
mkdir -p ~/actions-runner && cd ~/actions-runner
curl -o actions-runner-linux-x64.tar.gz -L https://github.com/actions/runner/releases/download/v2.311.0/actions-runner-linux-x64-2.311.0.tar.gz
tar xzf actions-runner-linux-x64.tar.gz
./config.sh --url https://github.com/islakhun24/crypto-ingitor-service --token YOUR_TOKEN
sudo ./svc.sh install && sudo ./svc.sh start

# 3. Tambah repository secrets (POSTGRES_HOST, POSTGRES_PASSWORD, dll)

# 4. Workflow otomatis trigger saat push ke main
```

### Manual Deploy via GitHub

1. Buka repository → Actions tab
2. Pilih **Deploy to Local Runner**
3. Klik **Run workflow** → pilih branch

## Build Commands

- `make build` - Build all 4 binaries
- `make test` - Run tests
- `make migrate` - Apply migrations
- `make seed-endpoints` - Seed endpoints
- `make run-api` - Run API server
- `make run-scheduler` - Run scheduler
- `make run-collector` - Run collector
- `make run-retention` - Run retention
- `make docker-up` - Start Docker stack
- `make docker-down` - Stop Docker stack
- `make docker-logs` - View logs

## Tech Stack

- **Language**: Go 1.26+ (stdlib `net/http`, `database/sql`)
- **Database**: PostgreSQL 16+
- **Cache**: Redis 7+ (optional, fallback ke memory)
- **Container**: Docker & Docker Compose
- **Metrics**: Prometheus (text format)
- **Exchanges**: Binance, OKX, Bybit, Bitget, Gate, MEXC
