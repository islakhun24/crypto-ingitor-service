package bybit

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
		SourceEndpointID: 9,
		CapturedAt:       time.Now().UTC(),
		Body: []byte(`{
			"retCode":0,
			"retMsg":"OK",
			"time":1779710400000,
			"result":{"list":[{"symbol":"0GUSDT","lastPrice":"1.23","bid1Price":"1.22","ask1Price":"1.24","volume24h":"100","turnover24h":"123","price24hPcnt":"0.015"}]}
		}`),
	}

	result, err := Normalize(context.Background(), "ticker", resp, scheduler.Job{SourceSymbol: "0GUSDT"}, symbols.Symbol{ID: 123})
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	got := result.MarketSnapshots[0]
	if got.Exchange != "bybit" || got.SourceSymbol != "0GUSDT" || *got.AskPrice != 1.24 {
		t.Fatalf("snapshot = %+v", got)
	}
}
