package mexc

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
		SourceEndpointID: 12,
		CapturedAt:       time.Now().UTC(),
		Body: []byte(`{
			"success":true,
			"code":0,
			"data":{"symbol":"0G_USDT","lastPrice":1.23,"bid1":1.22,"ask1":1.24,"volume24":100,"amount24":123,"holdVol":55,"indexPrice":1.229,"fairPrice":1.231,"fundingRate":0.0001,"timestamp":1779710400000}
		}`),
	}

	result, err := Normalize(context.Background(), "ticker", resp, scheduler.Job{SourceSymbol: "0G_USDT"}, symbols.Symbol{ID: 123})
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	got := result.MarketSnapshots[0]
	if got.Exchange != "mexc" || got.SourceSymbol != "0G_USDT" || *got.OpenInterest != 55 {
		t.Fatalf("snapshot = %+v", got)
	}
}
