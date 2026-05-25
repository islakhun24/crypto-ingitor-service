package hardening

import (
	"testing"
	"time"

	"aggregator-services/internal/normalizers"
	"aggregator-services/internal/scheduler"
)

func TestFilterNormalizedResultQuarantinesFutureAndFundingOutlier(t *testing.T) {
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	price := 100.0
	badFunding := 0.25

	valid, issues := FilterNormalizedResult("ticker", normalizers.NormalizedResult{
		MarketSnapshots: []normalizers.NormalizedMarketSnapshot{
			{
				SourceMeta:   normalizers.SourceMeta{SymbolID: 1, Exchange: "binance", SourceSymbol: "BTCUSDT"},
				SnapshotTime: now.Add(10 * time.Minute),
				LastPrice:    &price,
			},
			{
				SourceMeta:   normalizers.SourceMeta{SymbolID: 1, Exchange: "binance", SourceSymbol: "BTCUSDT"},
				SnapshotTime: now,
				LastPrice:    &price,
				FundingRate:  &badFunding,
			},
		},
	}, scheduler.Job{ID: 7, SourceSymbol: "BTCUSDT"}, now, ValidationConfig{MaxFutureSkew: time.Minute, FundingMin: -0.05, FundingMax: 0.05})

	if len(valid.MarketSnapshots) != 0 {
		t.Fatalf("valid market snapshots = %d, want 0", len(valid.MarketSnapshots))
	}
	if len(issues) != 2 {
		t.Fatalf("issues = %d, want 2", len(issues))
	}
}

func TestFilterNormalizedResultDerivesLiquidationEventKey(t *testing.T) {
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	price := 100.0
	quantity := 2.0

	valid, issues := FilterNormalizedResult("liquidation", normalizers.NormalizedResult{
		LiquidationEvents: []normalizers.NormalizedLiquidationEvent{{
			SourceMeta: normalizers.SourceMeta{SymbolID: 1, Exchange: "binance", SourceSymbol: "BTCUSDT"},
			EventTime:  now,
			Side:       "sell",
			Price:      &price,
			Quantity:   &quantity,
		}},
	}, scheduler.Job{SourceSymbol: "BTCUSDT"}, now, DefaultValidationConfig())

	if len(issues) != 0 {
		t.Fatalf("issues = %+v, want none", issues)
	}
	if len(valid.LiquidationEvents) != 1 || valid.LiquidationEvents[0].EventKey == "" {
		t.Fatalf("derived liquidation event key missing: %+v", valid.LiquidationEvents)
	}
}
