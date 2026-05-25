# Endpoint Registry Guide

## Philosophy

Endpoints are **database-driven**, not hardcoded in Go (except seed data). The collector resolves the exact HTTP endpoint at runtime from `exchange_api_endpoints`.

## Table: `exchange_api_endpoints`

Columns:
- `id`, `exchange`, `market_type`, `data_type`, `name`
- `method`, `base_url`, `path`
- `params_template`, `headers_template` — JSON templates
- `response_format`
- `is_batch_supported`, `batch_param_name`, `max_batch_size`
- `rate_limit_per_second`, `rate_limit_per_minute`, `request_weight`
- `min_interval_seconds`, `timeout_ms`
- `is_active`, `notes`

## Template Placeholders

The request builder (`internal/exchanges/common/request_builder.go`) substitutes:
- `{{source_symbol}}`
- `{{base_asset}}`
- `{{quote_asset}}`
- `{{period}}`
- `{{start_time}}`
- `{{end_time}}`
- `{{limit}}`

## Repository Methods

In `internal/endpoints/repository.go`:
- `ListActiveByExchangeDataType(exchange, dataType)` — used by planner
- `ResolveActive(exchange, marketType, dataType, name)` — used by collector
- `GetByID(id)` — fallback from job metadata

## Endpoint → Job Binding

During planning, endpoint metadata is stored in the job's JSON `metadata` field:
```json
{
  "endpoint_id": 42,
  "endpoint_name": "ticker_24hr",
  "endpoint_data_type": "ticker",
  "requires_endpoint": true,
  "request_weight": 1,
  "timeout_ms": 10000,
  "rate_limit_per_second": 5.0,
  "rate_limit_per_minute": 300
}
```

The collector deserializes this as `RuntimeMetadata` to know which endpoint to hit.

## Seeding

Migration `000007_seed_exchange_api_endpoints.sql` seeds ~70 endpoints across 6 exchanges.

To re-seed or update endpoints:
```bash
make seed-endpoints
```

This applies:
- `000007_seed_exchange_api_endpoints.sql` (upsert)
- `000008_seed_derivative_collection_policies.sql`

## Adding a New Endpoint

See `docs/prompts/add-endpoint.md`.

Short version:
1. Add rows to `migrations/000007_seed_exchange_api_endpoints.sql` (or a new migration)
2. Add normalizer support in the exchange adapter if the data type is new
3. Add repository mapping in `internal/collectors/core/writer.go` if needed
4. Add tests

## API Lookup

The API service exposes:
- `GET /endpoints/{exchange}/{data_type}` — list active endpoints
- `GET /endpoints/{exchange}/{market_type}/{data_type}/{name}` — resolve specific endpoint
