# Local Development Guide

Panduan lengkap untuk menjalankan dan mengembangkan Aggregator Services di lingkungan lokal.

## Prerequisites

| Dependency | Versi | Install |
|-----------|-------|---------|
| Go | 1.23+ | [go.dev](https://go.dev/dl/) |
| PostgreSQL | 16+ | `docker` atau native |
| Redis | 7+ | `docker` atau native |
| Make | - | `apt install make` / `brew install make` |
| Docker & Docker Compose | - | [docker.com](https://docker.com) |

## Quick Start (Docker Compose)

Cara paling cepat untuk menjalankan stack lengkap:

```bash
# 1. Clone repository
git clone https://github.com/islakhun24/crypto-ingitor-service.git
cd crypto-ingitor-service

# 2. Copy environment
cp .env.example .env.local

# 3. Start PostgreSQL, Redis, API Service, Prometheus
make docker-up

# 4. Apply database migrations
make migrate

# 5. Seed exchange endpoints dan collection policies
make seed-endpoints

# 6. Verifikasi API berjalan
curl http://localhost:8080/healthz
# Output: {"status":"ok"}

curl http://localhost:8080/readyz
# Output: {"status":"ready"}
```

## Menjalankan Services Secara Individual

Setelah `make docker-up` dan `make migrate`, jalankan services lain secara manual:

### 1. Scheduler (Job Planner)

```bash
# Generate jobs dari collection policies ke queue
make run-scheduler

# Atau langsung:
go run ./cmd/scheduler-service
```

Scheduler membaca `derivative_collection_policies` dan membuat jobs di `derivative_collection_jobs`.

### 2. Collector (Job Worker)

```bash
# Ambil dan eksekusi pending jobs
make run-collector

# Atau langsung:
go run ./cmd/collector-service
```

Collector:
1. Claims pending jobs dari queue
2. Hit exchange APIs (Binance, OKX, Bybit, dll)
3. Normalize response ke format standar
4. Write ke PostgreSQL + Redis

### 3. API Server

```bash
# Jalankan REST API server
make run-api

# Atau langsung:
go run ./cmd/api-service
```

Server berjalan di port 8080 (default).

### 4. Retention (Cleanup)

```bash
# Jalankan data cleanup (default dry-run)
make run-retention

# Atau langsung:
go run ./cmd/retention-service
```

## Menjalankan Semua Services (Loop)

Untuk development, jalankan services dalam loop:

```bash
#!/bin/bash
# run-dev.sh - Development loop

export $(cat .env.local | grep -v '^#' | xargs)

while true; do
    echo "=== Running Scheduler ==="
    go run ./cmd/scheduler-service

    echo "=== Running Collector ==="
    go run ./cmd/collector-service

    echo "=== Sleeping 60s ==="
    sleep 60
done
```

Jalankan API server di terminal terpisah:
```bash
make run-api
```

## Environment Variables

Copy `.env.example` ke `.env.local` dan sesuaikan:

### Wajib

```env
# PostgreSQL
POSTGRES_HOST=localhost          # atau 'postgres' jika pakai Docker
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=crypto_ultimate
POSTGRES_SSLMODE=disable

# Exchanges
SUPPORTED_EXCHANGES=binance,okx,bybit,bitget,gate,mexc
```

### Optional (sudah punya default)

```env
# App
APP_PORT=8080
HTTP_ADDR=:8080

# Redis (optional - fallback ke memory)
REDIS_HOST=localhost
REDIS_PORT=6379

# Rate Limiting
RATE_LIMIT_REQUESTS_PER_SECOND=5
RATE_LIMIT_BURST=10
CIRCUIT_BREAKER_FAILURE_LIMIT=5
CIRCUIT_BREAKER_COOLDOWN_SECONDS=30

# Retention
RETENTION_MAX_ROWS_PER_RUN=50000
RETENTION_TABLE_TIMEOUT_SECONDS=120

# Worker
WORKER_CONCURRENCY=4
COLLECTION_MODE=tiered
```

## Setup Database Manual (Tanpa Docker)

Jika PostgreSQL sudah terinstall secara native:

```bash
# 1. Buat database
psql -U postgres -c "CREATE DATABASE crypto_ultimate;"

# 2. Set env vars
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_USER=postgres
export POSTGRES_PASSWORD=your_password
export POSTGRES_DB=crypto_ultimate
export POSTGRES_SSLMODE=disable

# 3. Apply migrations
for f in migrations/*.sql; do
    psql -U postgres -d crypto_ultimate -f "$f"
done

# 4. Jalankan API
make run-api
```

## Verifikasi Endpoints

Setelah semua berjalan, test endpoints:

```bash
# Health
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz

# Metrics
curl http://localhost:8080/metrics

# Symbols
curl http://localhost:8080/symbols
curl http://localhost:8080/symbols/top?limit=10

# Derivatives Overview
curl http://localhost:8080/api/v1/derivatives/overview
curl http://localhost:8080/api/v1/derivatives/symbols

# Symbol Detail (contoh: BTC)
curl http://localhost:8080/api/v1/derivatives/symbols/BTC

# Jobs Queue
curl http://localhost:8080/api/v1/derivatives/jobs

# Health Status
curl http://localhost:8080/api/v1/derivatives/health/collectors
```

## Build Binary

```bash
# Build semua services
make build

# Output:
# ./api-service
# ./scheduler-service
# ./collector-service
# ./retention-service

# Run binary langsung
./api-service
```

## Testing

```bash
# Run all tests
make test

# Atau:
go test ./...

# Run specific package
go test ./internal/scheduler/...
go test ./internal/collectors/...
go test ./internal/api/derivatives/...
```

## Struktur Proyek

```
crypto-ingitor-service/
├── cmd/                          # Entry points
│   ├── api-service/main.go       # REST API server
│   ├── scheduler-service/main.go # Job planner
│   ├── collector-service/main.go # Job worker
│   └── retention-service/main.go # Cleanup engine
├── internal/
│   ├── api/derivatives/          # REST API handlers
│   ├── collectors/               # Data collection
│   ├── exchanges/                # Exchange adapters
│   ├── scheduler/                # Job scheduling
│   ├── retention/                # Data cleanup
│   ├── realtime/                 # Redis/memory store
│   ├── repositories/             # DB writers
│   ├── hardening/                # Validation & quality
│   └── ratelimit/                # Rate limiter & circuit breaker
├── migrations/                   # SQL migrations
├── deploy/                       # Deployment configs
├── docs/                         # Documentation
├── Makefile                      # Build commands
├── docker-compose.yml            # Local stack
└── README.md
```

## Troubleshooting Lokal

### Port 8080 sudah digunakan

```bash
# Ganti port di .env.local
APP_PORT=8081
HTTP_ADDR=:8081
```

### PostgreSQL connection refused

```bash
# Cek PostgreSQL berjalan
pg_isready -h localhost -p 5432

# Jika pakai Docker
docker ps | grep postgres

# Cek env vars sudah di-export
env | grep POSTGRES
```

### Redis tidak tersedia

Service otomatis fallback ke in-memory store. Tidak wajib untuk development.

### Go version mismatch

```bash
# Cek versi
go version

# Jika terlalu lama, download versi terbaru:
# https://go.dev/dl/
```

### Module download timeout

```bash
# Gunakan proxy China (jika di China)
go env -w GOPROXY=https://goproxy.cn,direct

# Atau proxy umum
go env -w GOPROXY=https://proxy.golang.org,direct
```

## Tips Development

1. **Jalankan API server terus-menerus** di satu terminal
2. **Jalankan scheduler + collector** di loop di terminal lain
3. **Gunakan `curl` atau Postman** untuk test endpoints
4. **Monitor logs** - semua service output JSON ke stdout
5. **Check metrics** di `http://localhost:8080/metrics`
6. **Database UI** - gunakan pgAdmin, DBeaver, atau psql
