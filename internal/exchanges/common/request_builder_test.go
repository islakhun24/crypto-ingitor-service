package common

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"aggregator-services/internal/endpoints"
	"aggregator-services/internal/scheduler"
	"aggregator-services/internal/symbols"
)

func TestBuildRequestReplacesPathQueryAndHeaders(t *testing.T) {
	endpoint := endpoints.Endpoint{
		ID:              9,
		Exchange:        "mexc",
		MarketType:      "usdt-futures",
		DataType:        "kline",
		Name:            "kline",
		Method:          http.MethodGet,
		BaseURL:         "https://contract.mexc.com",
		Path:            "/api/v1/contract/kline/{{source_symbol}}",
		ParamsTemplate:  []byte(`{"interval":"{{period}}","start":"{{start_time}}","end":"{{end_time}}","limit":"{{limit}}"}`),
		HeadersTemplate: []byte(`{"X-Symbol":"{{source_symbol}}"}`),
		TimeoutMS:       12000,
		IsActive:        true,
	}
	job := scheduler.Job{
		SourceSymbol: "0G_USDT",
		Period:       "Min5",
		Metadata:     []byte(`{"start_time":"1710000000","end_time":"1710000300","limit":200}`),
	}
	symbol := symbols.Symbol{BaseAsset: "0G", QuoteAsset: "USDT"}

	req, err := BuildRequest(context.Background(), endpoint, job, symbol)
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}

	if req.URL.Path != "/api/v1/contract/kline/0G_USDT" {
		t.Fatalf("path = %q", req.URL.Path)
	}
	if req.URL.Query().Get("interval") != "Min5" {
		t.Fatalf("interval = %q", req.URL.Query().Get("interval"))
	}
	if req.URL.Query().Get("limit") != "200" {
		t.Fatalf("limit = %q", req.URL.Query().Get("limit"))
	}
	if req.Header.Get("X-Symbol") != "0G_USDT" {
		t.Fatalf("X-Symbol = %q", req.Header.Get("X-Symbol"))
	}
	if TimeoutFromContext(req.Context()) == 0 {
		t.Fatal("timeout context value is missing")
	}
}

func TestBuildRequestReplacesGateStylePathSourceSymbol(t *testing.T) {
	endpoint := endpoints.Endpoint{
		ID:         10,
		Exchange:   "gate",
		MarketType: "usdt-futures",
		DataType:   "contracts",
		Name:       "contract_detail",
		Method:     http.MethodGet,
		BaseURL:    "https://api.gateio.ws",
		Path:       "/api/v4/futures/usdt/contracts/{{source_symbol}}",
		TimeoutMS:  10000,
		IsActive:   true,
	}

	req, err := BuildRequest(context.Background(), endpoint, scheduler.Job{SourceSymbol: "0G_USDT"}, symbols.Symbol{})
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	if req.URL.Path != "/api/v4/futures/usdt/contracts/0G_USDT" {
		t.Fatalf("path = %q", req.URL.Path)
	}
}

func TestBuildRequestEscapesPathPlaceholder(t *testing.T) {
	endpoint := endpoints.Endpoint{
		ID:         11,
		Exchange:   "test",
		MarketType: "swap",
		DataType:   "ticker",
		Name:       "ticker",
		Method:     http.MethodGet,
		BaseURL:    "https://example.com",
		Path:       "/markets/{{source_symbol}}",
		TimeoutMS:  10000,
		IsActive:   true,
	}

	req, err := BuildRequest(context.Background(), endpoint, scheduler.Job{SourceSymbol: "ABC/USDT"}, symbols.Symbol{})
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	if req.URL.EscapedPath() != "/markets/ABC%2FUSDT" {
		t.Fatalf("escaped path = %q", req.URL.EscapedPath())
	}
}

func TestBuildRequestSkipsInactiveEndpoint(t *testing.T) {
	_, err := BuildRequest(context.Background(), endpoints.Endpoint{IsActive: false}, scheduler.Job{SourceSymbol: "0GUSDT"}, symbols.Symbol{})
	if !errors.Is(err, ErrEndpointInactive) {
		t.Fatalf("error = %v, want ErrEndpointInactive", err)
	}
}
