package bitget

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
		SourceEndpointID: 10,
		CapturedAt:       time.Now().UTC(),
		Body: []byte(`{
			"code":"00000",
			"msg":"success",
			"requestTime":1779710400000,
			"data":[{"symbol":"0GUSDT","lastPr":"1.23","bidPr":"1.22","askPr":"1.24","baseVolume":"100","quoteVolume":"123","markPrice":"1.231","indexPrice":"1.229","fundingRate":"0.0001"}]
		}`),
	}

	result, err := Normalize(context.Background(), "ticker", resp, scheduler.Job{SourceSymbol: "0GUSDT"}, symbols.Symbol{ID: 123})
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	got := result.MarketSnapshots[0]
	if got.Exchange != "bitget" || got.SourceSymbol != "0GUSDT" || *got.MarkPrice != 1.231 {
		t.Fatalf("snapshot = %+v", got)
	}
}
