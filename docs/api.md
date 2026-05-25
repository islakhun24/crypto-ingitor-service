# API Guide

## Service

`api-service` is a long-running HTTP server (`net/http`, no framework). Entrypoint: `cmd/api-service/main.go`.

## Health & Ops

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/healthz` | Postgres ping |
| GET | `/readyz` | Postgres + Redis ping |
| GET | `/metrics` | Prometheus metrics (custom text format) |

## Symbol Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/symbols` | List active symbols |
| GET | `/symbols/exchange/{exchange}` | Filter symbols by exchange |
| GET | `/symbols/top?limit=N` | Top symbols |
| GET | `/symbols/watchlist` | Watchlist symbols |
| GET | `/symbols/{id}` | Symbol by ID |
| GET | `/symbols/markets` | Active symbol markets |

## Endpoint Registry

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/endpoints/{exchange}/{data_type}` | Active endpoints |
| GET | `/endpoints/{exchange}/{market_type}/{data_type}/{name}` | Resolve specific endpoint |

## Derivatives API

Base path: `/api/v1/derivatives/`

### Market Data

| Method | Path | Data |
|--------|------|------|
| GET | `/overview` | Latest aggregated snapshot per symbol (paged) |
| GET | `/symbols` | Same as overview |
| GET | `/symbols/{symbol}` | Full composite view: aggregate + klines, OI, funding, L/S ratio, taker flow, CVD, liquidations, basis, orderbook, divergence |
| GET | `/symbols/{symbol}/market` | Per-exchange latest market snapshots |
| GET | `/symbols/{symbol}/klines` | OHLCV klines (default 5m) |
| GET | `/symbols/{symbol}/open-interest` | Open interest snapshots/history |
| GET | `/symbols/{symbol}/funding` | Funding rate history |
| GET | `/symbols/{symbol}/long-short-ratio` | Long/short ratio snapshots |
| GET | `/symbols/{symbol}/taker-flow` | Taker buy/sell flow |
| GET | `/symbols/{symbol}/cvd` | CVD snapshots |
| GET | `/symbols/{symbol}/liquidations` | Liquidation aggregates |
| GET | `/symbols/{symbol}/basis` | Basis/premium snapshots |
| GET | `/symbols/{symbol}/orderbook-imbalance` | Orderbook imbalance |
| GET | `/symbols/{symbol}/exchange-divergence` | Cross-exchange divergence |

### Realtime

| Method | Path | Data |
|--------|------|------|
| GET | `/realtime/latest/{kind}/{exchange}/{source_symbol}` | Latest event from realtime store |
| GET | `/realtime/aggregate/{symbol_id}` | Latest aggregate snapshot |
| GET | `/realtime/ws-state/{exchange}/{stream}` | WebSocket stream state |

### Admin / Operational

| Method | Path | Data |
|--------|------|------|
| GET | `/health/collectors` | Collector health rows |
| GET | `/health/exchanges` | Aggregated exchange health counts |
| GET | `/jobs` | Collection jobs queue |
| GET | `/quality/issues` | Data quality issues |
| GET | `/quality/gaps` | Data gaps |

## Query Parameters

Common across list endpoints:
- `exchange` — filter by exchange
- `search` — text search
- `category` — status filter (maps to `status` or `backfill_status`)
- `interval` / `timeframe` / `period` — data granularity
- `start_time`, `end_time` — RFC3339 or Unix seconds
- `limit` — max 500
- `page` — pagination
- `sort`, `direction` — asc/desc
- `min_volume`, `min_oi`, `rank_min`, `rank_max` — filtering

## Running

```bash
make run-api
# or
go run ./cmd/api-service
```

## Healthcheck CLI

All services support `--healthcheck` flag:
```bash
/app/api-service --healthcheck
```
Exits 0 if healthy, 1 if not. Used by Docker healthchecks.
