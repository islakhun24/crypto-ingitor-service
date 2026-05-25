package normalizers

import (
	"strings"
	"testing"
	"time"
)

func TestValidateMarketSnapshotRejectsNegativePrice(t *testing.T) {
	price := -1.0
	err := ValidateMarketSnapshot(NormalizedMarketSnapshot{
		SourceMeta:   SourceMeta{SymbolID: 1, Exchange: "binance", SourceSymbol: "BTCUSDT"},
		SnapshotTime: time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC),
		LastPrice:    &price,
	})
	if err == nil || !strings.Contains(err.Error(), "last_price") {
		t.Fatalf("ValidateMarketSnapshot() error = %v", err)
	}
}

func TestValidateKlineRequiresValidPeriodAndTimestamps(t *testing.T) {
	err := ValidateKline(NormalizedKline{
		SourceMeta: SourceMeta{SymbolID: 1, Exchange: "okx", SourceSymbol: "BTC-USDT-SWAP"},
		Interval:   "bad-period",
		OpenTime:   time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC),
		CloseTime:  time.Date(2026, 5, 25, 12, 5, 0, 0, time.UTC),
		OpenPrice:  1,
		HighPrice:  2,
		LowPrice:   1,
		ClosePrice: 2,
	})
	if err == nil || !strings.Contains(err.Error(), "malformed period") {
		t.Fatalf("ValidateKline() error = %v", err)
	}
}

func TestValidateLiquidationEventRequiresEventKey(t *testing.T) {
	err := ValidateLiquidationEvent(NormalizedLiquidationEvent{
		SourceMeta: SourceMeta{SymbolID: 1, Exchange: "bybit", SourceSymbol: "BTCUSDT"},
		EventTime:  time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC),
		Side:       "long",
	})
	if err == nil || !strings.Contains(err.Error(), "event_key") {
		t.Fatalf("ValidateLiquidationEvent() error = %v", err)
	}
}

func TestValidateOrderbookImbalanceRequiresDepth(t *testing.T) {
	err := ValidateOrderbookImbalance(NormalizedOrderbookImbalance{
		SourceMeta:   SourceMeta{SymbolID: 1, Exchange: "gate", SourceSymbol: "BTC_USDT"},
		SnapshotTime: time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC),
	})
	if err == nil || !strings.Contains(err.Error(), "depth_levels") {
		t.Fatalf("ValidateOrderbookImbalance() error = %v", err)
	}
}
