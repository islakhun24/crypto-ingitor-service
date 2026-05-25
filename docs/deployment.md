# Deployment Guide

## Deployment Methods

| Method | Best For | Docs |
|--------|---------|------|
| **Docker Compose** | Local dev, single server | [This guide](#docker-compose-stack) |
| **SSH to VPS** | Production VPS | [docs/ssh-deploy.md](ssh-deploy.md) |
| **Kubernetes** | Scalable production | [CronJobs section](#kubernetes) |
| **systemd** | Bare metal / VM | [Timer units section](#systemd) |
| **crontab** | Simple single-server | [Cron section](#crontab) |

## Requirements

- Docker & Docker Compose (untuk Docker method)
- PostgreSQL 16+ (atau use container)
- Redis 7+ (atau use container)
- Go 1.26+ (untuk local builds)

## Environment

Copy `.env.example` ke `.env.local` dan customize:

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
- `postgres` - primary store
- `redis` - realtime cache
- `api-service` - REST API (long-running)
- `scheduler-service` - one-shot planner (`restart: "no"`)
- `collector-service` - one-shot worker (`restart: "no"`)
- `retention-service` - one-shot cleanup (`restart: "no"`)
- `prometheus` - metrics scraping
- `grafana` - dashboards (optional profile)

## Build

```bash
make build   # Builds all 4 binaries
make test    # Runs all tests
```

## Dockerfile

Multi-stage build:
1. `golang:1.26-bookworm` - compile 4 binaries with `-trimpath -ldflags="-s -w"`
2. `gcr.io/distroless/static-debian12:nonroot` - minimal runtime image

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

### Kubernetes

CronJobs for scheduler and retention; Deployment with HPA for collector.

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: aggregator-scheduler
spec:
  schedule: "* * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: scheduler
            image: aggregator-services:latest
            command: ["/app/scheduler-service"]
            envFrom:
            - configMapRef:
                name: aggregator-config
          restartPolicy: OnFailure
```

### systemd

Timer units for Linux systems.

```ini
# /etc/systemd/system/aggregator-scheduler.service
[Unit]
Description=Aggregator Scheduler

[Service]
Type=oneshot
ExecStart=/opt/aggregator/scheduler-service
Environment=POSTGRES_HOST=localhost
EnvironmentFile=/opt/aggregator/.env

# /etc/systemd/system/aggregator-scheduler.timer
[Unit]
Description=Run Aggregator Scheduler every minute

[Timer]
OnCalendar=*-*-* *:*:00
AccuracySec=1s

[Install]
WantedBy=timers.target
```

Enable:
```bash
sudo systemctl daemon-reload
sudo systemctl enable aggregator-scheduler.timer
sudo systemctl start aggregator-scheduler.timer
```

### crontab

Simple cron entries.

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

## Connecting to Existing Infrastructure via SSH

Untuk deploy ke VPS yang sudah berjalan, lihat panduan lengkap di [docs/ssh-deploy.md](ssh-deploy.md).

Quick commands:

```bash
# Setup SSH key
ssh-copy-id -i ~/.ssh/id_ed25519.pub root@YOUR_VPS_IP

# Deploy (setelah setup)
ssh root@YOUR_VPS_IP "cd /opt/crypto-ingitor-service && git pull && docker compose up -d --build"

# Verifikasi
curl http://YOUR_VPS_IP:8080/healthz
```
