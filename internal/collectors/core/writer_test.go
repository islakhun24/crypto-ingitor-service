package core

import (
	"testing"
	"time"

	"aggregator-services/internal/normalizers"
	"aggregator-services/internal/scheduler"
)

func TestDeriveOpenInterestFromMarketSnapshot(t *testing.T) {
	value := 123.0
	items := deriveOpenInterest([]normalizers.NormalizedMarketSnapshot{{
		SourceMeta:   normalizers.SourceMeta{SymbolID: 1, Exchange: "binance", SourceSymbol: "BTCUSDT"},
		SnapshotTime: time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC),
		OpenInterest: &value,
	}})

	if len(items) != 1 || items[0].OpenInterest != 123 {
		t.Fatalf("items = %+v", items)
	}
}

func TestDeriveFundingFromMarketSnapshot(t *testing.T) {
	value := 0.0001
	items := deriveFunding([]normalizers.NormalizedMarketSnapshot{{
		SourceMeta:   normalizers.SourceMeta{SymbolID: 1, Exchange: "binance", SourceSymbol: "BTCUSDT"},
		SnapshotTime: time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC),
		FundingRate:  &value,
	}})

	if len(items) != 1 || items[0].FundingRate != 0.0001 {
		t.Fatalf("items = %+v", items)
	}
}

func TestDeriveLiquidationAggregatesUsesTierBucket(t *testing.T) {
	usd := 2500.0
	items := deriveLiquidationAggregates(scheduler.Job{Tier: scheduler.TierAll}, []normalizers.NormalizedLiquidationEvent{{
		SourceMeta: normalizers.SourceMeta{SymbolID: 1, Exchange: "binance", SourceSymbol: "BTCUSDT"},
		EventKey:   "event-1",
		EventTime:  time.Date(2026, 5, 25, 12, 3, 42, 0, time.UTC),
		Side:       "long",
		USDValue:   &usd,
	}})

	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Period != "5m" {
		t.Fatalf("Period = %q", items[0].Period)
	}
	if !items[0].BucketTime.Equal(time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("BucketTime = %s", items[0].BucketTime)
	}
	if items[0].TotalLiquidationUSD != 2500 {
		t.Fatalf("TotalLiquidationUSD = %f", items[0].TotalLiquidationUSD)
	}
}
