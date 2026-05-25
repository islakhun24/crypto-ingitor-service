package common

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aggregator-services/internal/endpoints"
	"aggregator-services/internal/scheduler"
	"aggregator-services/internal/symbols"
)

type contextKey string

const (
	timeoutContextKey      contextKey = "exchange_timeout"
	endpointIDContextKey   contextKey = "exchange_endpoint_id"
	endpointNameContextKey contextKey = "exchange_endpoint_name"
	dataTypeContextKey     contextKey = "exchange_data_type"
)

type RequestPlaceholders struct {
	SourceSymbol string `json:"source_symbol"`
	BaseAsset    string `json:"base_asset"`
	QuoteAsset   string `json:"quote_asset"`
	Period       string `json:"period"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	Limit        string `json:"limit"`
}

func BuildRequest(ctx context.Context, endpoint endpoints.Endpoint, job scheduler.Job, symbol symbols.Symbol) (*http.Request, error) {
	if !endpoint.IsActive {
		return nil, fmt.Errorf("%w: %s/%s/%s/%s", ErrEndpointInactive, endpoint.Exchange, endpoint.MarketType, endpoint.DataType, endpoint.Name)
	}
	if strings.TrimSpace(job.SourceSymbol) == "" {
		return nil, fmt.Errorf("job source_symbol is required")
	}

	placeholders := placeholdersFromJob(job, symbol)
	path, err := replacePathTemplate(endpoint.Path, placeholders)
	if err != nil {
		return nil, err
	}

	baseURL, err := url.Parse(endpoint.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse endpoint base_url: %w", err)
	}
	pathURL, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("parse endpoint path: %w", err)
	}

	fullURL := baseURL.ResolveReference(pathURL)
	query, err := buildQuery(endpoint.ParamsTemplate, placeholders)
	if err != nil {
		return nil, err
	}
	fullURL.RawQuery = query.Encode()

	timeout := time.Duration(endpoint.TimeoutMS) * time.Millisecond
	requestCtx := context.WithValue(ctx, timeoutContextKey, timeout)
	requestCtx = context.WithValue(requestCtx, endpointIDContextKey, endpoint.ID)
	requestCtx = context.WithValue(requestCtx, endpointNameContextKey, endpoint.Name)
	requestCtx = context.WithValue(requestCtx, dataTypeContextKey, endpoint.DataType)

	req, err := http.NewRequestWithContext(requestCtx, endpoint.Method, fullURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	headers, err := buildHeaders(endpoint.HeadersTemplate, placeholders)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

func TimeoutFromContext(ctx context.Context) time.Duration {
	timeout, _ := ctx.Value(timeoutContextKey).(time.Duration)
	return timeout
}

func EndpointIDFromContext(ctx context.Context) int64 {
	id, _ := ctx.Value(endpointIDContextKey).(int64)
	return id
}

func EndpointNameFromContext(ctx context.Context) string {
	name, _ := ctx.Value(endpointNameContextKey).(string)
	return name
}

func DataTypeFromContext(ctx context.Context) string {
	dataType, _ := ctx.Value(dataTypeContextKey).(string)
	return dataType
}

func placeholdersFromJob(job scheduler.Job, symbol symbols.Symbol) RequestPlaceholders {
	values := RequestPlaceholders{
		SourceSymbol: strings.TrimSpace(job.SourceSymbol),
		BaseAsset:    strings.TrimSpace(symbol.BaseAsset),
		QuoteAsset:   strings.TrimSpace(symbol.QuoteAsset),
		Period:       strings.TrimSpace(job.Period),
		Limit:        "100",
	}

	var metadata map[string]any
	if len(job.Metadata) > 0 && json.Unmarshal(job.Metadata, &metadata) == nil {
		values.StartTime = stringify(metadata["start_time"])
		values.EndTime = stringify(metadata["end_time"])
		if limit := stringify(metadata["limit"]); limit != "" {
			values.Limit = limit
		}
	}

	return values
}

func buildQuery(raw json.RawMessage, placeholders RequestPlaceholders) (url.Values, error) {
	values := url.Values{}
	if len(raw) == 0 {
		return values, nil
	}

	var template map[string]string
	if err := json.Unmarshal(raw, &template); err != nil {
		return values, fmt.Errorf("parse params_template: %w", err)
	}

	for key, rawValue := range template {
		value, err := replaceTemplate(rawValue, placeholders)
		if err != nil {
			return values, err
		}
		if strings.TrimSpace(value) == "" {
			continue
		}
		values.Set(key, value)
	}

	return values, nil
}

func buildHeaders(raw json.RawMessage, placeholders RequestPlaceholders) (map[string]string, error) {
	headers := map[string]string{}
	if len(raw) == 0 {
		return headers, nil
	}

	var template map[string]string
	if err := json.Unmarshal(raw, &template); err != nil {
		return headers, fmt.Errorf("parse headers_template: %w", err)
	}

	for key, rawValue := range template {
		value, err := replaceTemplate(rawValue, placeholders)
		if err != nil {
			return headers, err
		}
		if strings.TrimSpace(value) == "" {
			continue
		}
		headers[key] = value
	}

	return headers, nil
}

func replaceTemplate(value string, placeholders RequestPlaceholders) (string, error) {
	replacements := map[string]string{
		"{{source_symbol}}": placeholders.SourceSymbol,
		"{{base_asset}}":    placeholders.BaseAsset,
		"{{quote_asset}}":   placeholders.QuoteAsset,
		"{{period}}":        placeholders.Period,
		"{{start_time}}":    placeholders.StartTime,
		"{{end_time}}":      placeholders.EndTime,
		"{{limit}}":         placeholders.Limit,
	}

	result := value
	for placeholder, replacement := range replacements {
		result = strings.ReplaceAll(result, placeholder, replacement)
	}
	if strings.Contains(result, "{{") || strings.Contains(result, "}}") {
		return "", fmt.Errorf("unresolved request placeholder in %q", value)
	}

	return result, nil
}

func replacePathTemplate(value string, placeholders RequestPlaceholders) (string, error) {
	escaped := RequestPlaceholders{
		SourceSymbol: url.PathEscape(placeholders.SourceSymbol),
		BaseAsset:    url.PathEscape(placeholders.BaseAsset),
		QuoteAsset:   url.PathEscape(placeholders.QuoteAsset),
		Period:       url.PathEscape(placeholders.Period),
		StartTime:    url.PathEscape(placeholders.StartTime),
		EndTime:      url.PathEscape(placeholders.EndTime),
		Limit:        url.PathEscape(placeholders.Limit),
	}

	return replaceTemplate(value, escaped)
}

func stringify(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", typed), "0"), ".")
	case int:
		return fmt.Sprintf("%d", typed)
	case int64:
		return fmt.Sprintf("%d", typed)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}
