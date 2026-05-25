# Production Checklist

- [ ] DB migration applied with `make migrate`.
- [ ] Endpoint registry seeded with `make seed-endpoints`.
- [ ] Collection policies seeded.
- [ ] Retention dry-run executed and reviewed.
- [ ] Rate limit config checked per exchange.
- [ ] Docker logs capped with `max-size=50m` and `max-file=3`.
- [ ] `/healthz` returns OK.
- [ ] `/readyz` returns ready.
- [ ] `/metrics` is scraped by Prometheus.
- [ ] Redis is reachable and memory policy is applied.
- [ ] Collector health is visible at `/api/v1/derivatives/health/collectors`.
- [ ] Free disk space checked for Postgres, Redis, and Docker volumes.
- [ ] Ubuntu firewall allows only required published ports.
- [ ] Backups and restore procedure are tested for Postgres volumes.
