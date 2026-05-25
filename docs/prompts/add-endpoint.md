# Prompt: Add a New Endpoint

Use this prompt template when asking an AI to add a new REST endpoint definition for an existing exchange.

---

## Prompt Template

```
Add a new endpoint "{ENDPOINT_NAME}" for exchange "{EXCHANGE}" supporting data type "{DATA_TYPE}".

### Context
Endpoints are database-driven. They live in `exchange_api_endpoints` and are seeded by migration `000007_seed_exchange_api_endpoints.sql`. The collector resolves endpoints at runtime; no endpoint URLs are hardcoded in Go.

### Required Steps

1. **Insert endpoint into DB seeder**
   - Edit `migrations/000007_seed_exchange_api_endpoints.sql`
   - Add an `INSERT INTO exchange_api_endpoints (...)` row with:
     - `exchange`, `market_type`, `data_type`, `name`
     - `method`, `base_url`, `path`
     - `params_template` (JSON map, use `{{source_symbol}}`, `{{period}}`, `{{limit}}` as needed)
     - `headers_template` (if required)
     - `response_format` (e.g., `json`)
     - `rate_limit_per_second`, `rate_limit_per_minute`, `request_weight`
     - `timeout_ms`
     - `is_active = true`
   - Use `ON CONFLICT (exchange, market_type, data_type, name) DO UPDATE SET ...` for idempotency

2. **Add normalizer support** (only if the data type is new for this exchange)
   - Edit `internal/exchanges/{EXCHANGE}/adapter.go`
   - Add the data type to the guard clause (allowed list)
   - Add unmarshaling logic for the exchange's JSON response shape
   - Build the appropriate normalized model and return it in `NormalizedResult`

3. **Add repository mapping** (only if the data type is new to the system)
   - Edit `internal/collectors/core/writer.go`
   - Add dispatch case in `RepositoryWriter.Write` for the new data type
   - Add repository method in `internal/repositories/` if it does not exist

4. **Add tests**
   - Edit `internal/exchanges/{EXCHANGE}/adapter_test.go`
   - Add a test with a real sample JSON body for the new endpoint/data type
   - Assert normalized fields are correctly mapped

5. **Add or update collection policy** (if this endpoint enables a new data stream)
   - Edit `migrations/000008_seed_derivative_collection_policies.sql`
   - Add policy row for `(exchange, data_type, tier, interval_seconds)`
   - Use `ON CONFLICT DO NOTHING`

### Constraints
- Do NOT hardcode endpoint URL or path in collector Go code
- Do NOT modify existing unrelated endpoints in the seed file
- Do NOT change the request builder or BaseAdapter unless a new placeholder is genuinely required
- Do NOT modify other exchanges' adapters
- Keep changes minimal: if the data type already exists in the normalizer and writer, only add the DB seed row

### Acceptance Criteria
- [ ] Migration compiles and is idempotent (`ON CONFLICT`)
- [ ] `go test ./internal/exchanges/{EXCHANGE}/...` passes
- [ ] `go test ./...` passes overall
- [ ] Endpoint is visible via API: `GET /endpoints/{EXCHANGE}/{DATA_TYPE}` after seeding
```

## Example Fill-in

- `{EXCHANGE}`: `binance`
- `{ENDPOINT_NAME}`: `index_price`
- `{DATA_TYPE}`: `index_price`
- `{MARKET_TYPE}`: `usds-m-futures`
