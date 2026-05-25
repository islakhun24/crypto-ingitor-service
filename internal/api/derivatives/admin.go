package derivatives

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (r *Repository) ListCollectorHealth(ctx context.Context, opts ListOptions) (PagedResponse[CollectorHealthDTO], error) {
	builder := newAdminBuilder(opts)
	builder.timeRange("heartbeat_at")
	if opts.Exchange != "" {
		builder.clauses = append(builder.clauses, "exchange = "+builder.arg(opts.Exchange))
	}
	query := fmt.Sprintf(`
		SELECT COUNT(*) OVER(), service_name, instance_id, COALESCE(exchange, ''),
		       COALESCE(data_type, ''), status, heartbeat_at,
		       last_success_at, last_error_at, COALESCE(error_message, ''),
		       metrics
		FROM collector_health
		%s
		ORDER BY heartbeat_at %s
		LIMIT %s OFFSET %s
	`, builder.whereSQL(), direction(opts.Direction), builder.arg(opts.Limit), builder.arg(opts.Offset()))
	rows, err := r.db.QueryContext(ctx, query, builder.args...)
	if err != nil {
		return PagedResponse[CollectorHealthDTO]{}, fmt.Errorf("query collector health: %w", err)
	}
	defer rows.Close()

	var total int
	items := []CollectorHealthDTO{}
	for rows.Next() {
		var item CollectorHealthDTO
		var successAt, errorAt sql.NullTime
		if err := rows.Scan(&total, &item.ServiceName, &item.InstanceID, &item.Exchange, &item.DataType, &item.Status, &item.HeartbeatAt, &successAt, &errorAt, &item.ErrorMessage, &item.Metrics); err != nil {
			return PagedResponse[CollectorHealthDTO]{}, fmt.Errorf("scan collector health: %w", err)
		}
		item.LastSuccessAt = nullTimePtr(successAt)
		item.LastErrorAt = nullTimePtr(errorAt)
		item.Metrics = ensureJSON(item.Metrics)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return PagedResponse[CollectorHealthDTO]{}, fmt.Errorf("iterate collector health: %w", err)
	}
	return PagedResponse[CollectorHealthDTO]{Data: items, Meta: opts.PageMeta(total)}, nil
}

func (r *Repository) ListExchangeHealth(ctx context.Context, opts ListOptions) (PagedResponse[ExchangeHealthDTO], error) {
	builder := newAdminBuilder(opts)
	builder.timeRange("heartbeat_at")
	if opts.Exchange != "" {
		builder.clauses = append(builder.clauses, "exchange = "+builder.arg(opts.Exchange))
	}
	query := fmt.Sprintf(`
		SELECT COUNT(*) OVER(), COALESCE(exchange, 'unknown') AS exchange,
		       MAX(heartbeat_at) AS last_heartbeat_at,
		       SUM(CASE WHEN status = 'healthy' THEN 1 ELSE 0 END)::int AS healthy_count,
		       SUM(CASE WHEN status = 'degraded' THEN 1 ELSE 0 END)::int AS degraded_count,
		       SUM(CASE WHEN status IN ('unhealthy', 'stopped') THEN 1 ELSE 0 END)::int AS unhealthy_count
		FROM collector_health
		%s
		GROUP BY COALESCE(exchange, 'unknown')
		ORDER BY exchange ASC
		LIMIT %s OFFSET %s
	`, builder.whereSQL(), builder.arg(opts.Limit), builder.arg(opts.Offset()))
	rows, err := r.db.QueryContext(ctx, query, builder.args...)
	if err != nil {
		return PagedResponse[ExchangeHealthDTO]{}, fmt.Errorf("query exchange health: %w", err)
	}
	defer rows.Close()

	var total int
	items := []ExchangeHealthDTO{}
	for rows.Next() {
		var item ExchangeHealthDTO
		if err := rows.Scan(&total, &item.Exchange, &item.LastHeartbeatAt, &item.HealthyCount, &item.DegradedCount, &item.UnhealthyCount); err != nil {
			return PagedResponse[ExchangeHealthDTO]{}, fmt.Errorf("scan exchange health: %w", err)
		}
		item.Status = exchangeHealthStatus(item)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return PagedResponse[ExchangeHealthDTO]{}, fmt.Errorf("iterate exchange health: %w", err)
	}
	return PagedResponse[ExchangeHealthDTO]{Data: items, Meta: opts.PageMeta(total)}, nil
}

func (r *Repository) ListJobs(ctx context.Context, opts ListOptions) (PagedResponse[JobDTO], error) {
	builder := newAdminBuilder(opts)
	builder.timeRange("scheduled_at")
	if opts.Exchange != "" {
		builder.clauses = append(builder.clauses, "exchange = "+builder.arg(opts.Exchange))
	}
	if opts.Category != "" {
		builder.clauses = append(builder.clauses, "status = "+builder.arg(opts.Category))
	}
	query := fmt.Sprintf(`
		SELECT COUNT(*) OVER(), id, exchange, data_type, tier,
		       COALESCE(symbol_id, 0), source_symbol, COALESCE(period, ''),
		       status, priority, scheduled_at, retry_count, max_retry,
		       COALESCE(last_error_type, ''), COALESCE(error_message, ''),
		       job_mode
		FROM derivative_collection_jobs
		%s
		ORDER BY scheduled_at %s, priority ASC
		LIMIT %s OFFSET %s
	`, builder.whereSQL(), direction(opts.Direction), builder.arg(opts.Limit), builder.arg(opts.Offset()))
	rows, err := r.db.QueryContext(ctx, query, builder.args...)
	if err != nil {
		return PagedResponse[JobDTO]{}, fmt.Errorf("query jobs: %w", err)
	}
	defer rows.Close()

	var total int
	items := []JobDTO{}
	for rows.Next() {
		var item JobDTO
		if err := rows.Scan(&total, &item.ID, &item.Exchange, &item.DataType, &item.Tier, &item.SymbolID, &item.SourceSymbol, &item.Period, &item.Status, &item.Priority, &item.ScheduledAt, &item.RetryCount, &item.MaxRetry, &item.LastErrorType, &item.ErrorMessage, &item.JobMode); err != nil {
			return PagedResponse[JobDTO]{}, fmt.Errorf("scan job: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return PagedResponse[JobDTO]{}, fmt.Errorf("iterate jobs: %w", err)
	}
	return PagedResponse[JobDTO]{Data: items, Meta: opts.PageMeta(total)}, nil
}

func (r *Repository) ListQualityIssues(ctx context.Context, opts ListOptions) (PagedResponse[QualityIssueDTO], error) {
	builder := newAdminBuilder(opts)
	builder.timeRange("last_seen_at")
	if opts.Exchange != "" {
		builder.clauses = append(builder.clauses, "exchange = "+builder.arg(opts.Exchange))
	}
	if opts.Category != "" {
		builder.clauses = append(builder.clauses, "status = "+builder.arg(opts.Category))
	}
	query := fmt.Sprintf(`
		SELECT COUNT(*) OVER(), issue_key, severity, COALESCE(exchange, ''),
		       data_type, COALESCE(symbol_id, 0), COALESCE(source_symbol, ''),
		       issue_type, status, last_seen_at, details
		FROM data_quality_issues
		%s
		ORDER BY last_seen_at %s
		LIMIT %s OFFSET %s
	`, builder.whereSQL(), direction(opts.Direction), builder.arg(opts.Limit), builder.arg(opts.Offset()))
	rows, err := r.db.QueryContext(ctx, query, builder.args...)
	if err != nil {
		return PagedResponse[QualityIssueDTO]{}, fmt.Errorf("query quality issues: %w", err)
	}
	defer rows.Close()

	var total int
	items := []QualityIssueDTO{}
	for rows.Next() {
		var item QualityIssueDTO
		if err := rows.Scan(&total, &item.IssueKey, &item.Severity, &item.Exchange, &item.DataType, &item.SymbolID, &item.SourceSymbol, &item.IssueType, &item.Status, &item.LastSeenAt, &item.Details); err != nil {
			return PagedResponse[QualityIssueDTO]{}, fmt.Errorf("scan quality issue: %w", err)
		}
		item.Details = ensureJSON(item.Details)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return PagedResponse[QualityIssueDTO]{}, fmt.Errorf("iterate quality issues: %w", err)
	}
	return PagedResponse[QualityIssueDTO]{Data: items, Meta: opts.PageMeta(total)}, nil
}

func (r *Repository) ListDataGaps(ctx context.Context, opts ListOptions) (PagedResponse[DataGapDTO], error) {
	builder := newAdminBuilder(opts)
	builder.timeRange("gap_start")
	if opts.Exchange != "" {
		builder.clauses = append(builder.clauses, "exchange = "+builder.arg(opts.Exchange))
	}
	if opts.Period != "" {
		builder.clauses = append(builder.clauses, "period = "+builder.arg(opts.Period))
	}
	if opts.Category != "" {
		builder.clauses = append(builder.clauses, "backfill_status = "+builder.arg(opts.Category))
	}
	query := fmt.Sprintf(`
		SELECT COUNT(*) OVER(), gap_key, symbol_id, exchange, data_type,
		       COALESCE(period, ''), gap_start, gap_end, backfill_status,
		       COALESCE(expected_interval_seconds, 0), last_observed_at,
		       metadata
		FROM data_gaps
		%s
		ORDER BY gap_start %s
		LIMIT %s OFFSET %s
	`, builder.whereSQL(), direction(opts.Direction), builder.arg(opts.Limit), builder.arg(opts.Offset()))
	rows, err := r.db.QueryContext(ctx, query, builder.args...)
	if err != nil {
		return PagedResponse[DataGapDTO]{}, fmt.Errorf("query data gaps: %w", err)
	}
	defer rows.Close()

	var total int
	items := []DataGapDTO{}
	for rows.Next() {
		var item DataGapDTO
		var observedAt sql.NullTime
		if err := rows.Scan(&total, &item.GapKey, &item.SymbolID, &item.Exchange, &item.DataType, &item.Period, &item.GapStart, &item.GapEnd, &item.BackfillStatus, &item.ExpectedIntervalSeconds, &observedAt, &item.Metadata); err != nil {
			return PagedResponse[DataGapDTO]{}, fmt.Errorf("scan data gap: %w", err)
		}
		item.LastObservedAt = nullTimePtr(observedAt)
		item.Metadata = ensureJSON(item.Metadata)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return PagedResponse[DataGapDTO]{}, fmt.Errorf("iterate data gaps: %w", err)
	}
	return PagedResponse[DataGapDTO]{Data: items, Meta: opts.PageMeta(total)}, nil
}

type adminBuilder struct {
	args    []any
	clauses []string
	opts    ListOptions
}

func newAdminBuilder(opts ListOptions) *adminBuilder {
	return &adminBuilder{opts: opts}
}

func (b *adminBuilder) arg(value any) string {
	b.args = append(b.args, value)
	return fmt.Sprintf("$%d", len(b.args))
}

func (b *adminBuilder) whereSQL() string {
	if len(b.clauses) == 0 {
		return ""
	}
	return "WHERE " + strings.Join(b.clauses, " AND ")
}

func (b *adminBuilder) timeRange(column string) {
	if b.opts.StartTime != nil {
		b.clauses = append(b.clauses, column+" >= "+b.arg(*b.opts.StartTime))
	}
	if b.opts.EndTime != nil {
		b.clauses = append(b.clauses, column+" <= "+b.arg(*b.opts.EndTime))
	}
}

func exchangeHealthStatus(item ExchangeHealthDTO) string {
	switch {
	case item.UnhealthyCount > 0:
		return "unhealthy"
	case item.DegradedCount > 0:
		return "degraded"
	default:
		return "healthy"
	}
}
