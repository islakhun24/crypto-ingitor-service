package derivatives

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aggregator-services/internal/realtime"
)

func TestRealtimeLatestEndpointReadsStore(t *testing.T) {
	store := realtime.NewMemoryStore()
	event := realtime.LatestEvent{
		Kind:         realtime.KindMarket,
		Exchange:     "binance",
		SourceSymbol: "BTCUSDT",
		EventTime:    time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC),
		Payload:      json.RawMessage(`{"last_price":100}`),
	}
	if err := (realtime.LatestWriter{Store: store, TTL: time.Minute}).Write(context.Background(), event); err != nil {
		t.Fatalf("seed latest: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRealtime(mux, store, time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/derivatives/realtime/latest/market/binance/BTCUSDT", nil)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
	}
	if !json.Valid(resp.Body.Bytes()) || !strings.Contains(resp.Body.String(), `"kind":"market"`) {
		t.Fatalf("body = %s", resp.Body.String())
	}
}

func TestRealtimeLatestEndpointReturnsNotFound(t *testing.T) {
	mux := http.NewServeMux()
	RegisterRealtime(mux, realtime.NewMemoryStore(), time.Second)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/derivatives/realtime/latest/market/binance/ETHUSDT", nil)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"code":"not_found"`) {
		t.Fatalf("body = %s", resp.Body.String())
	}
}
