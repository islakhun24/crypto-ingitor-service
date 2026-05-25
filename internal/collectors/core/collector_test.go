package core

import (
	"context"
	"net/http"
	"testing"
	"time"

	"aggregator-services/internal/endpoints"
	excommon "aggregator-services/internal/exchanges/common"
	"aggregator-services/internal/normalizers"
	"aggregator-services/internal/repositories"
	"aggregator-services/internal/scheduler"
	"aggregator-services/internal/symbols"
)

func TestCollectorExecutesAndWritesNormalizedResult(t *testing.T) {
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	writer := &fakeWriter{}
	logs := &fakeRequestLogs{}
	health := &fakeHealth{}
	collector := Collector{
		Endpoints: &fakeEndpoints{endpoint: endpoints.Endpoint{ID: 10, Exchange: "binance", DataType: "ticker", Name: "ticker", IsActive: true}},
		Symbols:   fakeSymbols{symbol: symbols.Symbol{ID: 123, Symbol: "0GUSDT", BaseAsset: "0G", QuoteAsset: "USDT"}},
		Adapters:  fakeRegistry{adapter: fakeAdapter{now: now}},
		Writer:    writer,
		Logs:      logs,
		Health:    health,
		Now:       func() time.Time { return now },
	}
	job := scheduler.Job{
		Exchange:     "binance",
		DataType:     "ticker",
		SymbolID:     123,
		SourceSymbol: "0GUSDT",
		Metadata:     []byte(`{"endpoint_id":10,"endpoint_name":"ticker","endpoint_data_type":"ticker","market_type":"usds-m-futures"}`),
	}

	if err := collector.Execute(context.Background(), job); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if writer.writes != 1 {
		t.Fatalf("writes = %d, want 1", writer.writes)
	}
	if len(logs.logs) != 1 || logs.logs[0].StatusCode == nil || *logs.logs[0].StatusCode != 200 {
		t.Fatalf("logs = %+v", logs.logs)
	}
	if health.last.Status != "healthy" {
		t.Fatalf("health = %+v", health.last)
	}
}

func TestCollectorLogsRecoverableHTTPFailure(t *testing.T) {
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	logs := &fakeRequestLogs{}
	collector := Collector{
		Endpoints: &fakeEndpoints{endpoint: endpoints.Endpoint{ID: 10, Exchange: "binance", DataType: "ticker", Name: "ticker", IsActive: true}},
		Symbols:   fakeSymbols{symbol: symbols.Symbol{ID: 123}},
		Adapters:  fakeRegistry{adapter: fakeAdapter{now: now, statusCode: 429}},
		Writer:    &fakeWriter{},
		Logs:      logs,
		Health:    &fakeHealth{},
		Now:       func() time.Time { return now },
	}

	err := collector.Execute(context.Background(), scheduler.Job{
		Exchange:     "binance",
		DataType:     "ticker",
		SymbolID:     123,
		SourceSymbol: "0GUSDT",
		Metadata:     []byte(`{"endpoint_id":10}`),
	})
	if err == nil {
		t.Fatal("Execute() error = nil")
	}
	if len(logs.logs) != 1 || logs.logs[0].StatusCode == nil || *logs.logs[0].StatusCode != 429 {
		t.Fatalf("logs = %+v", logs.logs)
	}
}

type fakeEndpoints struct {
	endpoint endpoints.Endpoint
}

func (f *fakeEndpoints) GetByID(context.Context, int64) (endpoints.Endpoint, error) {
	return f.endpoint, nil
}

func (f *fakeEndpoints) ResolveActive(context.Context, string, string, string, string) (endpoints.Endpoint, error) {
	return f.endpoint, nil
}

type fakeSymbols struct {
	symbol symbols.Symbol
}

func (f fakeSymbols) GetSymbolByID(context.Context, int64) (symbols.Symbol, error) {
	return f.symbol, nil
}

type fakeRegistry struct {
	adapter excommon.ExchangeAdapter
}

func (f fakeRegistry) Get(string) (excommon.ExchangeAdapter, error) {
	return f.adapter, nil
}

type fakeAdapter struct {
	now        time.Time
	statusCode int
}

func (f fakeAdapter) Exchange() string {
	return "binance"
}

func (f fakeAdapter) BuildRequest(ctx context.Context, _ endpoints.Endpoint, _ scheduler.Job, _ symbols.Symbol) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.test/ticker", nil)
	if err != nil {
		return nil, err
	}
	return req, nil
}

func (f fakeAdapter) Execute(context.Context, *http.Request) (*excommon.ExchangeResponse, error) {
	status := f.statusCode
	if status == 0 {
		status = 200
	}
	return &excommon.ExchangeResponse{StatusCode: status, Body: []byte(`{}`), CapturedAt: f.now}, nil
}

func (f fakeAdapter) Normalize(context.Context, string, *excommon.ExchangeResponse, scheduler.Job, symbols.Symbol) (normalizers.NormalizedResult, error) {
	price := 1.23
	return normalizers.NormalizedResult{MarketSnapshots: []normalizers.NormalizedMarketSnapshot{{
		SourceMeta:   normalizers.SourceMeta{SymbolID: 123, Exchange: "binance", SourceSymbol: "0GUSDT"},
		SnapshotTime: f.now,
		LastPrice:    &price,
	}}}, nil
}

type fakeWriter struct {
	writes int
}

func (f *fakeWriter) Write(context.Context, string, normalizers.NormalizedResult, scheduler.Job) error {
	f.writes++
	return nil
}

type fakeRequestLogs struct {
	logs []repositories.RequestLog
}

func (f *fakeRequestLogs) Insert(_ context.Context, log repositories.RequestLog) error {
	f.logs = append(f.logs, log)
	return nil
}

type fakeHealth struct {
	last repositories.CollectorHealth
}

func (f *fakeHealth) Upsert(_ context.Context, health repositories.CollectorHealth) error {
	f.last = health
	return nil
}
