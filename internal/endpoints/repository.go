package endpoints

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrEndpointUnavailable = errors.New("endpoint unavailable")

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListActiveByExchangeDataType(ctx context.Context, exchange string, dataType string) ([]Endpoint, error) {
	exchange = strings.ToLower(strings.TrimSpace(exchange))
	dataType = strings.TrimSpace(dataType)
	if exchange == "" || dataType == "" {
		return nil, fmt.Errorf("exchange and data_type are required")
	}

	return r.query(ctx, `
		SELECT id, exchange, market_type, data_type, name, method, base_url, path,
		       params_template, headers_template, response_format, is_batch_supported,
		       batch_param_name, max_batch_size, rate_limit_per_second, rate_limit_per_minute,
		       request_weight, min_interval_seconds, timeout_ms, is_active, notes,
		       created_at, updated_at
		FROM exchange_api_endpoints
		WHERE exchange = $1
		  AND data_type = $2
		  AND is_active = true
		ORDER BY market_type ASC, name ASC
	`, exchange, dataType)
}

func (r *Repository) ResolveActive(ctx context.Context, exchange string, marketType string, dataType string, name string) (Endpoint, error) {
	exchange = strings.ToLower(strings.TrimSpace(exchange))
	marketType = strings.TrimSpace(marketType)
	dataType = strings.TrimSpace(dataType)
	name = strings.TrimSpace(name)
	if exchange == "" || marketType == "" || dataType == "" || name == "" {
		return Endpoint{}, fmt.Errorf("exchange, market_type, data_type, and name are required")
	}

	endpoint, found, err := r.get(ctx, exchange, marketType, dataType, name)
	if err != nil {
		return Endpoint{}, err
	}
	if !found {
		return Endpoint{}, fmt.Errorf("%w: %s/%s/%s/%s is not seeded", ErrEndpointUnavailable, exchange, marketType, dataType, name)
	}
	if !endpoint.IsActive {
		return Endpoint{}, fmt.Errorf("%w: %s/%s/%s/%s is inactive: %s", ErrEndpointUnavailable, exchange, marketType, dataType, name, endpoint.Notes)
	}

	return endpoint, nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (Endpoint, error) {
	rows, err := r.query(ctx, `
		SELECT id, exchange, market_type, data_type, name, method, base_url, path,
		       params_template, headers_template, response_format, is_batch_supported,
		       batch_param_name, max_batch_size, rate_limit_per_second, rate_limit_per_minute,
		       request_weight, min_interval_seconds, timeout_ms, is_active, notes,
		       created_at, updated_at
		FROM exchange_api_endpoints
		WHERE id = $1
		LIMIT 1
	`, id)
	if err != nil {
		return Endpoint{}, err
	}
	if len(rows) == 0 {
		return Endpoint{}, fmt.Errorf("%w: endpoint id %d is not seeded", ErrEndpointUnavailable, id)
	}
	if !rows[0].IsActive {
		return Endpoint{}, fmt.Errorf("%w: endpoint id %d is inactive: %s", ErrEndpointUnavailable, id, rows[0].Notes)
	}

	return rows[0], nil
}

func (r *Repository) get(ctx context.Context, exchange string, marketType string, dataType string, name string) (Endpoint, bool, error) {
	rows, err := r.query(ctx, `
		SELECT id, exchange, market_type, data_type, name, method, base_url, path,
		       params_template, headers_template, response_format, is_batch_supported,
		       batch_param_name, max_batch_size, rate_limit_per_second, rate_limit_per_minute,
		       request_weight, min_interval_seconds, timeout_ms, is_active, notes,
		       created_at, updated_at
		FROM exchange_api_endpoints
		WHERE exchange = $1
		  AND market_type = $2
		  AND data_type = $3
		  AND name = $4
		LIMIT 1
	`, exchange, marketType, dataType, name)
	if err != nil {
		return Endpoint{}, false, err
	}
	if len(rows) == 0 {
		return Endpoint{}, false, nil
	}

	return rows[0], true, nil
}

func (r *Repository) query(ctx context.Context, query string, args ...any) ([]Endpoint, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query endpoints: %w", err)
	}
	defer rows.Close()

	var result []Endpoint
	for rows.Next() {
		endpoint, err := scanEndpoint(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, endpoint)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate endpoints: %w", err)
	}

	return result, nil
}

func scanEndpoint(rows *sql.Rows) (Endpoint, error) {
	var (
		endpoint           Endpoint
		batchParamName     sql.NullString
		rateLimitPerSecond sql.NullFloat64
		rateLimitPerMinute sql.NullInt64
		notes              sql.NullString
	)

	if err := rows.Scan(
		&endpoint.ID,
		&endpoint.Exchange,
		&endpoint.MarketType,
		&endpoint.DataType,
		&endpoint.Name,
		&endpoint.Method,
		&endpoint.BaseURL,
		&endpoint.Path,
		&endpoint.ParamsTemplate,
		&endpoint.HeadersTemplate,
		&endpoint.ResponseFormat,
		&endpoint.IsBatchSupported,
		&batchParamName,
		&endpoint.MaxBatchSize,
		&rateLimitPerSecond,
		&rateLimitPerMinute,
		&endpoint.RequestWeight,
		&endpoint.MinIntervalSeconds,
		&endpoint.TimeoutMS,
		&endpoint.IsActive,
		&notes,
		&endpoint.CreatedAt,
		&endpoint.UpdatedAt,
	); err != nil {
		return Endpoint{}, fmt.Errorf("scan endpoint: %w", err)
	}

	endpoint.BatchParamName = batchParamName.String
	endpoint.RateLimitPerSecond = rateLimitPerSecond.Float64
	endpoint.RateLimitPerMinute = int(rateLimitPerMinute.Int64)
	endpoint.Notes = notes.String

	return endpoint, nil
}
