package common

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aggregator-services/internal/endpoints"
	"aggregator-services/internal/normalizers"
	"aggregator-services/internal/scheduler"
	"aggregator-services/internal/symbols"
)

type BaseAdapter struct {
	ExchangeName string
	Client       *http.Client
	Normalizer   NormalizerFunc
}

func (a BaseAdapter) Exchange() string {
	return strings.ToLower(strings.TrimSpace(a.ExchangeName))
}

func (a BaseAdapter) BuildRequest(ctx context.Context, endpoint endpoints.Endpoint, job scheduler.Job, symbol symbols.Symbol) (*http.Request, error) {
	return BuildRequest(ctx, endpoint, job, symbol)
}

func (a BaseAdapter) Execute(ctx context.Context, req *http.Request) (*ExchangeResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	timeout := TimeoutFromContext(req.Context())
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	requestCtx, cancel := context.WithTimeout(req.Context(), timeout)
	defer cancel()

	req = req.Clone(requestCtx)
	client := a.Client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute exchange request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read exchange response: %w", err)
	}

	return &ExchangeResponse{
		Exchange:         a.Exchange(),
		EndpointID:       EndpointIDFromContext(req.Context()),
		EndpointName:     EndpointNameFromContext(req.Context()),
		DataType:         DataTypeFromContext(req.Context()),
		StatusCode:       resp.StatusCode,
		Headers:          resp.Header.Clone(),
		Body:             body,
		CapturedAt:       time.Now().UTC(),
		SourceEndpointID: EndpointIDFromContext(req.Context()),
	}, nil
}

func (a BaseAdapter) Normalize(ctx context.Context, dataType string, resp *ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	if a.Normalizer == nil {
		return normalizers.NormalizedResult{}, ErrUnsupportedDataType
	}

	return a.Normalizer(ctx, dataType, resp, job, symbol)
}
