package binance

import (
	"context"
	"testing"
	"time"

	excommon "aggregator-services/internal/exchanges/common"
	"aggregator-services/internal/scheduler"
	"aggregator-services/internal/symbols"
)

func TestNormalizeTickerSample(t *testing.T) {
	resp := &excommon.ExchangeResponse{
		SourceEndpointID: 7,
		CapturedAt:       time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC),
		Body: []byte(`{
			"symbol":"0GUSDT",
			"lastPrice":"1.23",
			"bidPrice":"1.22",
			"askPrice":"1.24",
			"volume":"100",
			"quoteVolume":"123",
			"priceChangePercent":"1.5",
			"closeTime":1779710400000
		}`),
	}

	result, err := Normalize(context.Background(), "ticker", resp, scheduler.Job{SourceSymbol: "0GUSDT"}, symbols.Symbol{ID: 123})
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if len(result.MarketSnapshots) != 1 {
		t.Fatalf("len(MarketSnapshots) = %d", len(result.MarketSnapshots))
	}
	got := result.MarketSnapshots[0]
	if got.Exchange != "binance" || got.SourceSymbol != "0GUSDT" || *got.LastPrice != 1.23 {
		t.Fatalf("snapshot = %+v", got)
	}
}
