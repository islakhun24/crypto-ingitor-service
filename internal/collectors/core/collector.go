package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"aggregator-services/internal/endpoints"
	excommon "aggregator-services/internal/exchanges/common"
	"aggregator-services/internal/hardening"
	"aggregator-services/internal/ratelimit"
	"aggregator-services/internal/repositories"
	"aggregator-services/internal/scheduler"
	"aggregator-services/internal/symbols"
)

type EndpointResolver interface {
	GetByID(ctx context.Context, id int64) (endpoints.Endpoint, error)
	ResolveActive(ctx context.Context, exchange string, marketType string, dataType string, name string) (endpoints.Endpoint, error)
}

type SymbolLoader interface {
	GetSymbolByID(ctx context.Context, id int64) (symbols.Symbol, error)
}

type AdapterRegistry interface {
	Get(exchange string) (excommon.ExchangeAdapter, error)
}

type RequestLogger interface {
	Insert(ctx context.Context, log repositories.RequestLog) error
}

type HealthReporter interface {
	Upsert(ctx context.Context, health repositories.CollectorHealth) error
}

type Collector struct {
	Endpoints EndpointResolver
	Symbols   SymbolLoader
	Adapters  AdapterRegistry
	Writer    ResultWriter
	Logs      RequestLogger
	Health    HealthReporter
	Service   string
	Instance  string
	Now       func() time.Time
}

func (c Collector) Execute(ctx context.Context, job scheduler.Job) error {
	now := c.now()
	symbol, err := c.Symbols.GetSymbolByID(ctx, job.SymbolID)
	if err != nil {
		c.reportHealth(ctx, job, "unhealthy", err.Error(), now)
		return scheduler.NewExecutionError(ratelimit.FailureParse, false, err)
	}
	if job.SourceSymbol == "" {
		err := fmt.Errorf("missing source_symbol")
		c.reportHealth(ctx, job, "unhealthy", err.Error(), now)
		return scheduler.NewExecutionError(ratelimit.FailureParse, false, err)
	}

	endpoint, err := c.endpointForJob(ctx, job)
	if err != nil {
		c.reportHealth(ctx, job, "degraded", err.Error(), now)
		return scheduler.NewExecutionError(ratelimit.FailureParse, false, err)
	}

	adapter, err := c.Adapters.Get(job.Exchange)
	if err != nil {
		c.reportHealth(ctx, job, "unhealthy", err.Error(), now)
		return scheduler.NewExecutionError(ratelimit.FailureParse, false, err)
	}

	req, err := adapter.BuildRequest(ctx, endpoint, job, symbol)
	if err != nil {
		c.reportHealth(ctx, job, "degraded", err.Error(), now)
		return classifyCollectorError(err)
	}

	start := c.now()
	resp, err := adapter.Execute(ctx, req)
	durationMS := int(c.now().Sub(start).Milliseconds())
	c.logRequest(ctx, endpoint, job, req, resp, err, durationMS)
	if err != nil {
		c.reportHealth(ctx, job, "degraded", err.Error(), c.now())
		return classifyCollectorError(err)
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		err := fmt.Errorf("exchange returned status %d", resp.StatusCode)
		c.reportHealth(ctx, job, "degraded", err.Error(), c.now())
		classification := hardening.ClassifyHTTPStatus(resp.StatusCode, resp.Body)
		return scheduler.NewExecutionError(failureKind(classification.Type), classification.Recoverable, err)
	}
	if resp.StatusCode >= 400 {
		err := fmt.Errorf("exchange returned status %d", resp.StatusCode)
		c.reportHealth(ctx, job, "unhealthy", err.Error(), c.now())
		classification := hardening.ClassifyHTTPStatus(resp.StatusCode, resp.Body)
		return scheduler.NewExecutionError(failureKind(classification.Type), classification.Recoverable, err)
	}

	result, err := adapter.Normalize(ctx, job.DataType, resp, job, symbol)
	if err != nil {
		c.reportHealth(ctx, job, "unhealthy", err.Error(), c.now())
		return classifyCollectorError(err)
	}

	if err := c.Writer.Write(ctx, job.DataType, result, job); err != nil {
		c.reportHealth(ctx, job, "unhealthy", err.Error(), c.now())
		return scheduler.NewExecutionError(ratelimit.FailureServerError, true, err)
	}

	c.reportHealth(ctx, job, "healthy", "", c.now())
	return nil
}

func (c Collector) endpointForJob(ctx context.Context, job scheduler.Job) (endpoints.Endpoint, error) {
	metadata := scheduler.RuntimeMetadataFromJob(job)
	if metadata.EndpointID > 0 {
		return c.Endpoints.GetByID(ctx, metadata.EndpointID)
	}
	if metadata.EndpointName == "" || metadata.EndpointDataType == "" {
		return endpoints.Endpoint{}, fmt.Errorf("job metadata missing endpoint identity")
	}

	var meta map[string]any
	_ = json.Unmarshal(job.Metadata, &meta)
	marketType, _ := meta["market_type"].(string)
	if marketType == "" {
		return endpoints.Endpoint{}, fmt.Errorf("job metadata missing market_type")
	}

	return c.Endpoints.ResolveActive(ctx, job.Exchange, marketType, metadata.EndpointDataType, metadata.EndpointName)
}

func (c Collector) logRequest(ctx context.Context, endpoint endpoints.Endpoint, job scheduler.Job, req *http.Request, resp *excommon.ExchangeResponse, err error, durationMS int) {
	if c.Logs == nil {
		return
	}

	var statusCode *int
	if resp != nil {
		statusCode = &resp.StatusCode
	}

	errorType := ""
	if err != nil {
		errorType = classifyErrorType(err)
	} else if resp != nil && resp.StatusCode >= 400 {
		errorType = string(hardening.ClassifyHTTPStatus(resp.StatusCode, resp.Body).Type)
	}

	_ = c.Logs.Insert(ctx, repositories.RequestLog{
		Exchange:     job.Exchange,
		EndpointID:   endpoint.ID,
		DataType:     job.DataType,
		SourceSymbol: job.SourceSymbol,
		RequestURL:   req.URL.String(),
		RequestPath:  req.URL.Path,
		StatusCode:   statusCode,
		ErrorType:    errorType,
		DurationMS:   durationMS,
		RetryCount:   job.RetryCount,
		RateLimited:  resp != nil && resp.StatusCode == http.StatusTooManyRequests,
		CapturedAt:   c.now(),
		Metadata:     job.Metadata,
	})
}

func (c Collector) reportHealth(ctx context.Context, job scheduler.Job, status string, message string, at time.Time) {
	if c.Health == nil {
		return
	}

	health := repositories.CollectorHealth{
		ServiceName:  c.service(),
		InstanceID:   c.instance(),
		Exchange:     job.Exchange,
		DataType:     job.DataType,
		Status:       status,
		HeartbeatAt:  at,
		ErrorMessage: message,
	}
	if status == "healthy" {
		health.LastSuccessAt = at
	} else if message != "" {
		health.LastErrorAt = at
	}

	_ = c.Health.Upsert(ctx, health)
}

func (c Collector) now() time.Time {
	if c.Now != nil {
		return c.Now().UTC()
	}
	return time.Now().UTC()
}

func (c Collector) service() string {
	if c.Service == "" {
		return "collector-service"
	}
	return c.Service
}

func (c Collector) instance() string {
	if c.Instance == "" {
		return "default"
	}
	return c.Instance
}

func classifyCollectorError(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return scheduler.NewExecutionError(ratelimit.FailureTimeout, true, err)
	case errors.Is(err, endpoints.ErrEndpointUnavailable),
		errors.Is(err, excommon.ErrEndpointInactive),
		errors.Is(err, excommon.ErrUnsupportedDataType):
		return scheduler.NewExecutionError(ratelimit.FailureBadRequest, false, err)
	default:
		classification := hardening.ClassifyError(err)
		return scheduler.NewExecutionError(failureKind(classification.Type), classification.Recoverable, err)
	}
}

func classifyErrorType(err error) string {
	var execErr scheduler.ExecutionError
	if errors.As(err, &execErr) {
		return string(execErr.Kind)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return string(ratelimit.FailureTimeout)
	}
	return string(hardening.ClassifyError(err).Type)
}

func failureKind(errorType hardening.ErrorType) ratelimit.FailureKind {
	switch errorType {
	case hardening.ErrorRateLimited:
		return ratelimit.FailureRateLimited
	case hardening.ErrorServerError:
		return ratelimit.FailureServerError
	case hardening.ErrorTimeout:
		return ratelimit.FailureTimeout
	case hardening.ErrorNetwork:
		return ratelimit.FailureNetworkError
	case hardening.ErrorBadRequest:
		return ratelimit.FailureBadRequest
	case hardening.ErrorUnauthorized:
		return ratelimit.FailureUnauthorized
	case hardening.ErrorNotFound:
		return ratelimit.FailureNotFound
	case hardening.ErrorInvalidSymbol:
		return ratelimit.FailureInvalidSymbol
	case hardening.ErrorInvalidResponse:
		return ratelimit.FailureInvalidResponse
	case hardening.ErrorNormalizer:
		return ratelimit.FailureNormalizerError
	default:
		return ratelimit.FailureUnknown
	}
}
