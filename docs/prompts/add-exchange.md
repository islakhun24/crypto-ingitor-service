# Prompt: Add a New Exchange

Use this prompt template when asking an AI to add support for a new crypto exchange.

---

## Prompt Template

```
Add exchange support for {EXCHANGE_NAME} to the aggregator.

### Context
This is a Go crypto derivative data aggregator. Exchanges are implemented as adapter packages under `internal/exchanges/`. Each adapter embeds `common.BaseAdapter` and only supplies a `NormalizerFunc`.

### Required Steps

1. **Create adapter package** at `internal/exchanges/{EXCHANGE_NAME}/`
   - Create `adapter.go` with:
     - Exchange-specific response structs for the JSON envelope
     - `Normalize` function with signature:
       ```go
       func Normalize(ctx context.Context, dataType string, resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error)
       ```
     - `NewAdapter(client *http.Client)` constructor returning the adapter
   - The normalizer MUST:
     - Guard unsupported data types (return `ErrUnsupportedDataType`)
     - Check error envelopes specific to {EXCHANGE_NAME}
     - Unmarshal raw JSON body
     - Extract fields and build `normalizers.NormalizedResult`
     - Use `excommon.MarketSnapshot(...)` for ticker/market data
   - Create `adapter_test.go` with at least one happy-path test per supported data type using raw JSON samples

2. **Register adapter** in `internal/exchanges/all/registry.go`
   - Add `"{EXCHANGE_NAME}": {EXCHANGE_NAME}.NewAdapter(client),`

3. **Add endpoint registry rows** in `migrations/000007_seed_exchange_api_endpoints.sql`
   - Add endpoints for each supported data type
   - Use `ON CONFLICT (exchange, market_type, data_type, name) DO UPDATE SET ...`
   - Include `base_url`, `path`, `params_template` (use `{{source_symbol}}` placeholder)
   - Set conservative rate limits (`rate_limit_per_second`, `rate_limit_per_minute`)
   - Set `is_active = true`

4. **Add collection policies** in `migrations/000008_seed_derivative_collection_policies.sql`
   - Cross-join exchange×market_type with existing policy tuples
   - Use `ON CONFLICT DO NOTHING`

5. **Update `SUPPORTED_EXCHANGES`** env var documentation if needed (do not change defaults unless asked)

### Constraints
- Do NOT modify any existing exchange adapter packages or their tests
- Do NOT modify `internal/exchanges/common/` unless the new exchange requires a genuinely new common helper
- Do NOT hardcode endpoint URLs in collector code
- Do NOT change existing migrations for other exchanges
- Keep the normalizer pure: no HTTP calls, only transform `resp.Body` into structs

### Acceptance Criteria
- [ ] `go test ./internal/exchanges/{EXCHANGE_NAME}/...` passes
- [ ] `go test ./internal/exchanges/all/...` passes (registry compiles)
- [ ] `go test ./...` passes overall
- [ ] Migration syntax is valid SQL with idempotent `ON CONFLICT`
```

## Example Fill-in

Replace `{EXCHANGE_NAME}` with the actual exchange (e.g., `kucoin`, `hyperliquid`).

Replace `supported data type` with the subset the exchange will initially support (e.g., `ticker`, `funding`, `open_interest`).
