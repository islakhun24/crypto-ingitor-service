package okx

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
		SourceEndpointID: 8,
		CapturedAt:       time.Now().UTC(),
		Body: []byte(`{
			"code":"0",
			"msg":"",
			"data":[{"instId":"0G-USDT-SWAP","last":"1.23","bidPx":"1.22","askPx":"1.24","vol24h":"100","volCcy24h":"123","ts":"1779710400000"}]
		}`),
	}

	result, err := Normalize(context.Background(), "ticker", resp, scheduler.Job{SourceSymbol: "0G-USDT-SWAP"}, symbols.Symbol{ID: 123})
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	got := result.MarketSnapshots[0]
	if got.Exchange != "okx" || got.SourceSymbol != "0G-USDT-SWAP" || *got.BidPrice != 1.22 {
		t.Fatalf("snapshot = %+v", got)
	}
}
