package gate

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
		SourceEndpointID: 11,
		CapturedAt:       time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC),
		Body: []byte(`[
			{"contract":"0G_USDT","last":"1.23","highest_bid":"1.22","lowest_ask":"1.24","volume_24h_base":"100","volume_24h_quote":"123","mark_price":"1.231","index_price":"1.229","funding_rate":"0.0001"}
		]`),
	}

	result, err := Normalize(context.Background(), "ticker", resp, scheduler.Job{SourceSymbol: "0G_USDT"}, symbols.Symbol{ID: 123})
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	got := result.MarketSnapshots[0]
	if got.Exchange != "gate" || got.SourceSymbol != "0G_USDT" || *got.IndexPrice != 1.229 {
		t.Fatalf("snapshot = %+v", got)
	}
}
