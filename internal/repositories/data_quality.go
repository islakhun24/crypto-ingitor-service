package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"aggregator-services/internal/hardening"
)

type DataQualityRepository struct {
	db *sql.DB
}

func NewDataQualityRepository(db *sql.DB) *DataQualityRepository {
	return &DataQualityRepository{db: db}
}

func (r *DataQualityRepository) UpsertIssues(ctx context.Context, issues []hardening.QualityIssue) (int, error) {
	if len(issues) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin data quality upsert: %w", err)
	}
	defer tx.Rollback()

	count := 0
	for _, issue := range issues {
		if issue.ObservedAt.IsZero() {
			issue.ObservedAt = time.Now().UTC()
		}
		result, err := tx.ExecContext(ctx, `
			INSERT INTO data_quality_issues (
			    issue_key, severity, exchange, data_type, symbol_id,
			    source_symbol, issue_type, status, first_seen_at,
			    last_seen_at, observed_at, job_id, endpoint_id, details
			)
			VALUES ($1, $2, NULLIF($3, ''), $4, NULLIF($5, 0),
			        NULLIF($6, ''), $7, 'open', $8, $8, $8,
			        NULLIF($9, 0), NULLIF($10, 0), $11)
			ON CONFLICT (issue_key) DO UPDATE SET
			    severity = EXCLUDED.severity,
			    last_seen_at = EXCLUDED.last_seen_at,
			    observed_at = EXCLUDED.observed_at,
			    job_id = EXCLUDED.job_id,
			    endpoint_id = EXCLUDED.endpoint_id,
			    details = EXCLUDED.details,
			    status = CASE
			        WHEN data_quality_issues.status = 'resolved' THEN 'open'
			        ELSE data_quality_issues.status
			    END
		`,
			issue.IssueKey,
			defaultString(issue.Severity, "warning"),
			issue.Exchange,
			issue.DataType,
			issue.SymbolID,
			issue.SourceSymbol,
			issue.IssueType,
			issue.ObservedAt,
			issue.JobID,
			issue.EndpointID,
			ensureJSON(issue.Details),
		)
		if err != nil {
			return 0, fmt.Errorf("upsert data quality issue: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		count += int(affected)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit data quality upsert: %w", err)
	}

	return count, nil
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func qualityDetails(value any) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}
