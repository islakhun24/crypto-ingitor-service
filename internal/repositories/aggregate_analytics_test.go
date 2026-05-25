package repositories

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"
	"time"

	"aggregator-services/internal/aggregation"
)

func TestComputeMarketStructureBuildsNonSignalState(t *testing.T) {
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	candles := []aggregateCandle{
		{OpenTime: now.Add(-15 * time.Minute), Open: 100, High: 101, Low: 99, Close: 100},
		{OpenTime: now.Add(-10 * time.Minute), Open: 100, High: 105, Low: 100, Close: 104},
		{OpenTime: now.Add(-5 * time.Minute), Open: 104, High: 109, Low: 103, Close: 108},
	}

	snapshot := computeMarketStructure(42, "15m", now, candles)

	if snapshot.TrendDirection != "bullish" {
		t.Fatalf("TrendDirection = %q, want bullish", snapshot.TrendDirection)
	}
	if snapshot.StructureState != "expansion" {
		t.Fatalf("StructureState = %q, want expansion", snapshot.StructureState)
	}
	if snapshot.LastSwingHigh == nil || *snapshot.LastSwingHigh != 109 {
		t.Fatalf("LastSwingHigh = %v, want 109", snapshot.LastSwingHigh)
	}
	if snapshot.PricePosition == nil || *snapshot.PricePosition <= 0 {
		t.Fatalf("PricePosition = %v, want positive", snapshot.PricePosition)
	}

	var metadata map[string]any
	if err := json.Unmarshal(snapshot.Metadata, &metadata); err != nil {
		t.Fatalf("metadata json: %v", err)
	}
	if metadata["non_signal"] != true {
		t.Fatalf("metadata non_signal = %v, want true", metadata["non_signal"])
	}
}

func TestComputeVolatilityCalculatesATRAndRanges(t *testing.T) {
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	candles := []aggregateCandle{
		{OpenTime: now.Add(-15 * time.Minute), Open: 100, High: 105, Low: 95, Close: 100},
		{OpenTime: now.Add(-10 * time.Minute), Open: 100, High: 110, Low: 98, Close: 108},
		{OpenTime: now.Add(-5 * time.Minute), Open: 108, High: 115, Low: 105, Close: 112},
	}

	snapshot := computeVolatility(42, "15m", now, candles, candles, candles)

	if snapshot.ATR == nil {
		t.Fatal("ATR = nil")
	}
	wantATR := (10.0 + 12.0 + 10.0) / 3.0
	if math.Abs(*snapshot.ATR-wantATR) > 0.000001 {
		t.Fatalf("ATR = %f, want %f", *snapshot.ATR, wantATR)
	}
	if snapshot.ATRPercent == nil || *snapshot.ATRPercent <= 0 {
		t.Fatalf("ATRPercent = %v, want positive", snapshot.ATRPercent)
	}
	if snapshot.RealizedVolatilityPercent == nil || *snapshot.RealizedVolatilityPercent <= 0 {
		t.Fatalf("RealizedVolatilityPercent = %v, want positive", snapshot.RealizedVolatilityPercent)
	}
	if snapshot.RangePercent24h == nil || snapshot.RangePercent7d == nil {
		t.Fatal("range percentages must be populated")
	}
}

func TestBuildAnomalyFlagsKeepsFlagsInJSONOnly(t *testing.T) {
	imbalance := 55.0
	liquidation := 2_000_000.0
	basis := 1.25
	funding := 0.002

	raw := buildAnomalyFlags(aggregation.DerivativeAggregateSnapshot{
		AvgOrderbookImbalancePercent: &imbalance,
	}, []aggregation.AnalyticsWindowMetric{
		{
			Window:            "5m",
			LiquidationSumUSD: &liquidation,
			BasisAvg:          &basis,
			FundingMax:        &funding,
		},
	})

	if !json.Valid(raw) {
		t.Fatalf("anomaly flags are not valid json: %s", string(raw))
	}
	if bytes.Contains(raw, []byte("signal_score")) || bytes.Contains(raw, []byte("recommendation")) {
		t.Fatalf("anomaly flags must not contain scoring or recommendation fields: %s", string(raw))
	}

	var flags []map[string]any
	if err := json.Unmarshal(raw, &flags); err != nil {
		t.Fatalf("decode flags: %v", err)
	}
	if len(flags) == 0 {
		t.Fatal("flags = 0, want at least one")
	}
}

func TestRawJSONMarkedDegradedFindsNestedQualityStatus(t *testing.T) {
	raw := json.RawMessage(`{"payload":{"quality_status":"degraded"}}`)
	if !rawJSONMarkedDegraded(raw) {
		t.Fatal("rawJSONMarkedDegraded() = false, want true")
	}
}
