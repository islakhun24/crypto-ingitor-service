package observability

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Prometheus(ctx context.Context) (string, error) {
	var builder strings.Builder

	if err := r.writeRequestMetrics(ctx, &builder); err != nil {
		return "", err
	}
	if err := r.writeJobMetrics(ctx, &builder); err != nil {
		return "", err
	}
	if err := r.writeQualityMetrics(ctx, &builder); err != nil {
		return "", err
	}
	if err := r.writeCleanupMetrics(ctx, &builder); err != nil {
		return "", err
	}

	return builder.String(), nil
}

func (r *Repository) writeRequestMetrics(ctx context.Context, builder *strings.Builder) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT exchange, data_type,
		       COALESCE(NULLIF(error_type, ''), COALESCE(status_code::text, 'unknown')) AS status,
		       COUNT(*) AS total,
		       COALESCE(AVG(duration_ms), 0) AS avg_duration_ms,
		       COALESCE(SUM(CASE WHEN rate_limited THEN 1 ELSE 0 END), 0) AS rate_limited_total
		FROM exchange_request_logs
		WHERE captured_at >= now() - interval '24 hours'
		GROUP BY exchange, data_type, COALESCE(NULLIF(error_type, ''), COALESCE(status_code::text, 'unknown'))
	`)
	if err != nil {
		return fmt.Errorf("query request metrics: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var exchange, dataType, status string
		var total, rateLimited int64
		var avgDuration float64
		if err := rows.Scan(&exchange, &dataType, &status, &total, &avgDuration, &rateLimited); err != nil {
			return fmt.Errorf("scan request metrics: %w", err)
		}
		writeMetric(builder, "requests_total", map[string]string{"exchange": exchange, "data_type": dataType, "status": status}, float64(total))
		writeMetric(builder, "request_duration_ms", map[string]string{"exchange": exchange, "data_type": dataType, "status": status}, avgDuration)
		writeMetric(builder, "rate_limited_total", map[string]string{"exchange": exchange, "data_type": dataType}, float64(rateLimited))
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate request metrics: %w", err)
	}

	return nil
}

func (r *Repository) writeJobMetrics(ctx context.Context, builder *strings.Builder) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT status, COUNT(*)
		FROM derivative_collection_jobs
		GROUP BY status
	`)
	if err != nil {
		return fmt.Errorf("query job metrics: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return fmt.Errorf("scan job metrics: %w", err)
		}
		writeMetric(builder, "jobs_"+status, nil, float64(count))
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate job metrics: %w", err)
	}

	return nil
}

func (r *Repository) writeQualityMetrics(ctx context.Context, builder *strings.Builder) error {
	var qualityIssues, gaps int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM data_quality_issues WHERE status <> 'resolved'`).Scan(&qualityIssues); err != nil {
		return fmt.Errorf("query quality issue metrics: %w", err)
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM data_gaps WHERE backfill_status <> 'completed'`).Scan(&gaps); err != nil {
		return fmt.Errorf("query data gap metrics: %w", err)
	}
	writeMetric(builder, "data_quality_issues_total", nil, float64(qualityIssues))
	writeMetric(builder, "data_gaps_total", nil, float64(gaps))

	return nil
}

func (r *Repository) writeCleanupMetrics(ctx context.Context, builder *strings.Builder) error {
	var rowsDeleted int64
	if err := r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(rows_deleted), 0) FROM data_cleanup_runs`).Scan(&rowsDeleted); err != nil {
		return fmt.Errorf("query cleanup metrics: %w", err)
	}
	writeMetric(builder, "cleanup_rows_deleted_total", nil, float64(rowsDeleted))
	writeMetric(builder, "rows_inserted_total", map[string]string{"source": "collection_runs"}, 0)
	writeMetric(builder, "circuit_breaker_state", map[string]string{"state": "in_memory"}, 0)

	return nil
}

func writeMetric(builder *strings.Builder, name string, labels map[string]string, value float64) {
	builder.WriteString(name)
	if len(labels) > 0 {
		builder.WriteByte('{')
		keys := make([]string, 0, len(labels))
		for key := range labels {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for index, key := range keys {
			if index > 0 {
				builder.WriteByte(',')
			}
			builder.WriteString(key)
			builder.WriteString(`="`)
			builder.WriteString(strings.ReplaceAll(labels[key], `"`, `\"`))
			builder.WriteByte('"')
		}
		builder.WriteByte('}')
	}
	builder.WriteByte(' ')
	builder.WriteString(fmt.Sprintf("%.3f", value))
	builder.WriteByte('\n')
}
