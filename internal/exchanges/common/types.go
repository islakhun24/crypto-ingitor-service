package common

import (
	"context"
	"errors"
	"net/http"
	"time"

	"aggregator-services/internal/endpoints"
	"aggregator-services/internal/normalizers"
	"aggregator-services/internal/scheduler"
	"aggregator-services/internal/symbols"
)

var (
	ErrEndpointInactive    = errors.New("endpoint is inactive")
	ErrUnsupportedDataType = errors.New("unsupported data type")
	ErrExchangeResponse    = errors.New("exchange response error")
)

type ExchangeAdapter interface {
	Exchange() string
	BuildRequest(ctx context.Context, endpoint endpoints.Endpoint, job scheduler.Job, symbol symbols.Symbol) (*http.Request, error)
	Execute(ctx context.Context, req *http.Request) (*ExchangeResponse, error)
	Normalize(ctx context.Context, dataType string, resp *ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error)
}

type ExchangeResponse struct {
	Exchange         string      `json:"exchange"`
	EndpointID       int64       `json:"endpoint_id"`
	EndpointName     string      `json:"endpoint_name"`
	DataType         string      `json:"data_type"`
	StatusCode       int         `json:"status_code"`
	Headers          http.Header `json:"headers"`
	Body             []byte      `json:"body"`
	CapturedAt       time.Time   `json:"captured_at"`
	SourceEndpointID int64       `json:"source_endpoint_id"`
}

type NormalizerFunc func(ctx context.Context, dataType string, resp *ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error)
