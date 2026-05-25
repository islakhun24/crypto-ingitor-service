package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type RequestLog struct {
	Exchange     string
	EndpointID   int64
	DataType     string
	SourceSymbol string
	RequestURL   string
	RequestPath  string
	StatusCode   *int
	ErrorType    string
	DurationMS   int
	RetryCount   int
	RateLimited  bool
	CapturedAt   time.Time
	Metadata     json.RawMessage
}

type RequestLogRepository struct {
	db *sql.DB
}

func NewRequestLogRepository(db *sql.DB) *RequestLogRepository {
	return &RequestLogRepository{db: db}
}

func (r *RequestLogRepository) Insert(ctx context.Context, log RequestLog) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO exchange_request_logs (
		    exchange, endpoint_id, data_type, source_symbol, request_url,
		    request_path, status_code, error_type, duration_ms, retry_count,
		    rate_limited, captured_at, metadata
		)
		VALUES ($1, NULLIF($2, 0), $3, NULLIF($4, ''), NULLIF($5, ''),
		        NULLIF($6, ''), $7, NULLIF($8, ''), $9, $10, $11, $12, $13)
	`,
		log.Exchange,
		log.EndpointID,
		log.DataType,
		log.SourceSymbol,
		log.RequestURL,
		log.RequestPath,
		nullableStatusCode(log.StatusCode),
		log.ErrorType,
		log.DurationMS,
		log.RetryCount,
		log.RateLimited,
		log.CapturedAt,
		ensureJSON(log.Metadata),
	)
	if err != nil {
		return fmt.Errorf("insert request log: %w", err)
	}

	return nil
}

func nullableStatusCode(value *int) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}
