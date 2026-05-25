# Exchange Adapters Guide

## Supported Exchanges

| Exchange | Package | Response Style |
|----------|---------|---------------|
| binance | `internal/exchanges/binance/` | Flat object |
| okx | `internal/exchanges/okx/` | Envelope `code` + `data[]` |
| bybit | `internal/exchanges/bybit/` | Envelope `retCode` + `result.list[]` |
| bitget | `internal/exchanges/bitget/` | Envelope `code` + `data` |
| gate | `internal/exchanges/gate/` | Raw array `[]` |
| mexc | `internal/exchanges/mexc/` | Envelope `success` + `data` |

## Adapter Interface

```go
type ExchangeAdapter interface {
    Exchange() string
    BuildRequest(ctx, endpoint, job, symbol) (*http.Request, error)
    Execute(ctx, req) (*ExchangeResponse, error)
    Normalize(ctx, dataType, resp, job, symbol) (NormalizedResult, error)
}
```

## BaseAdapter Pattern

Every exchange embeds `common.BaseAdapter` and only supplies:
- `ExchangeName` (string)
- `Client` (`*http.Client`)
- `Normalizer` (`NormalizerFunc`)

`BaseAdapter` provides generic `BuildRequest`, `Execute`, and `Normalize` dispatch. Most new exchanges only need a `Normalize` function.

## Normalizer Function

```go
type NormalizerFunc func(
    ctx context.Context,
    dataType string,
    resp *ExchangeResponse,
    job scheduler.Job,
    symbol symbols.Symbol,
) (normalizers.NormalizedResult, error)
```

What each normalizer does:
1. Data-type guard — returns `ErrUnsupportedDataType` if not supported
2. Error envelope check — parses exchange-specific error JSON
3. Unmarshal — parses raw JSON body into exchange-specific structs
4. Extract first record — unwraps `data[0]` or `result.list[0]`
5. Timestamp resolution — extracts exchange timestamp or falls back to `resp.CapturedAt`
6. Build normalized model — uses `common.MarketSnapshot()` or constructs other types
7. Return `NormalizedResult`

## Registry

`internal/exchanges/all/registry.go` contains a static map:
```go
adapters := map[string]ExchangeAdapter{
    "binance": binance.NewAdapter(client),
    "okx":     okx.NewAdapter(client),
    // ...
}
```

Adding a new exchange requires registering it here.

## Adding a New Exchange

See `docs/prompts/add-exchange.md`.

Short version:
1. Create package `internal/exchanges/<name>/`
2. Define response structs
3. Write `Normalize` function
4. Write `NewAdapter(client)` constructor
5. Register in `internal/exchanges/all/registry.go`
6. Add DB endpoint rows in migration seeder
7. Add `adapter_test.go`

## Key Files

| File | Purpose |
|------|---------|
| `internal/exchanges/all/registry.go` | Static adapter registry |
| `internal/exchanges/common/types.go` | Interface definitions |
| `internal/exchanges/common/adapter.go` | BaseAdapter |
| `internal/exchanges/common/request_builder.go` | Template-based request construction |
| `internal/exchanges/common/parse.go` | Safe parsing helpers (Float, MillisToTime) |
| `internal/exchanges/common/market_snapshot.go` | Common MarketSnapshot builder |
