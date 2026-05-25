# Normalizers Guide

## Purpose

Normalizers convert exchange-specific JSON responses into canonical Go structs. All data types funnel through `normalizers.NormalizedResult`.

## NormalizedResult

Located in `internal/normalizers/models.go`. A single container struct holding typed slices for every data type:

- `MarketSnapshots` (`[]NormalizedMarketSnapshot`)
- `Klines` (`[]NormalizedKline`)
- `OpenInterest` (`[]NormalizedOpenInterest`)
- `FundingSnapshots` / `FundingHistory`
- `LongShortRatios`
- `TakerFlows`
- `CVDSnapshots`
- `LiquidationEvents` / `LiquidationAggregates`
- `BasisPremiums`
- `OrderbookImbalances`
- `ExchangeDivergences`

## SourceMeta

Every normalized model embeds:
```go
type SourceMeta struct {
    SymbolID         int64
    Exchange         string
    SourceSymbol     string
    SourceEndpointID int64
    RawData          json.RawMessage
}
```

## Validation

`internal/normalizers/validation.go` provides validators per type:
- `ValidateMarketSnapshot`
- `ValidateKline`
- `ValidateOpenInterest`
- `ValidateFundingSnapshot` / `ValidateFundingHistory`
- `ValidateLongShortRatio`
- `ValidateTakerFlow`
- `ValidateCVD`
- `ValidateLiquidationEvent` / `ValidateLiquidationAggregate`
- `ValidateBasisPremium`
- `ValidateOrderbookImbalance`
- `ValidateExchangeDivergence`

## Hardening Filter

Before DB write, `hardening.FilterNormalizedResult` validates:
- Source metadata (symbol ID, exchange, source symbol)
- Timestamps — rejects data too far in the future (`MaxFutureSkew`, default 2 min)
- Intervals — ensures kline/flow period matches job period
- Funding rates — clamps to sanity bounds (-5% to +5%)
- Runs normalizer validators
- Generates `QualityIssue` structs for dropped records

## Parsing Helpers

`internal/exchanges/common/parse.go`:
- `ParseFloat(any) (float64, error)` — handles string, float64, int, int64, json.Number
- `FloatPtr(any) (*float64, error)`
- `MillisToTime(any) / SecondsToTime(any)`
- `OptionalFloat(string) (*float64, error)` — empty-string → nil

## Adding a New Data Type

1. Add canonical struct to `internal/normalizers/models.go`
2. Add validator to `internal/normalizers/validation.go`
3. Update every exchange normalizer (or the ones that support it) to build the new type
4. Add hardening filter logic in `internal/hardening/validation.go`
5. Add writer dispatch in `internal/collectors/core/writer.go`
6. Add repository method in `internal/repositories/` if missing
7. Add tests in each exchange's `adapter_test.go`

## Fixing a Normalizer

1. Locate the exchange package: `internal/exchanges/<exchange>/adapter.go`
2. Find the `Normalize` function
3. Adjust field mapping, timestamp extraction, or envelope unwrapping
4. Update `internal/exchanges/<exchange>/adapter_test.go` with a sample JSON body
5. Run `go test ./internal/exchanges/<exchange>/...` to verify
6. Check `internal/hardening/validation.go` if validation rules need adjustment

## Key Files

| File | Purpose |
|------|---------|
| `internal/normalizers/models.go` | Canonical structs |
| `internal/normalizers/validation.go` | Per-type validators |
| `internal/hardening/validation.go` | Filter before DB write |
| `internal/exchanges/common/parse.go` | Safe JSON parsing |
| `internal/exchanges/common/market_snapshot.go` | Common snapshot builder |
