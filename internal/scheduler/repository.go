package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aggregator-services/internal/symbols"
)

type Repository struct {
	db      *sql.DB
	symbols *symbols.Repository
}

func NewRepository(db *sql.DB, symbolRepo *symbols.Repository) *Repository {
	return &Repository{db: db, symbols: symbolRepo}
}

func (r *Repository) ListActivePolicies(ctx context.Context) ([]Policy, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, exchange, market_type, data_type, tier, period, interval_seconds,
		       batch_size, priority, enabled, max_retry, stale_after_seconds,
		       metadata, created_at, updated_at
		FROM derivative_collection_policies
		WHERE enabled = true
		ORDER BY priority ASC, interval_seconds ASC, exchange ASC, data_type ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query collection policies: %w", err)
	}
	defer rows.Close()

	var policies []Policy
	for rows.Next() {
		policy, err := scanPolicy(rows)
		if err != nil {
			return nil, err
		}
		policies = append(policies, policy)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate collection policies: %w", err)
	}

	return policies, nil
}

func (r *Repository) ListSymbolMarketsByTier(ctx context.Context, tier string, exchange string, limit int) ([]symbols.SymbolMarket, error) {
	tier = strings.ToLower(strings.TrimSpace(tier))
	exchange = strings.ToLower(strings.TrimSpace(exchange))
	if tier == "" || exchange == "" {
		return nil, fmt.Errorf("tier and exchange are required")
	}

	active, err := r.symbols.ListSymbolsByExchange(ctx, exchange)
	if err != nil {
		return nil, err
	}

	if tier == TierWatchlist {
		watchlist, err := r.watchlistSymbolIDs(ctx)
		if err != nil {
			return nil, err
		}
		active = filterSymbolsByID(active, watchlist)
	}

	if tier == TierTop100 && limit == 0 {
		limit = 100
	}

	result := make([]symbols.SymbolMarket, 0, len(active))
	for _, symbol := range active {
		for _, market := range symbol.Markets {
			if !market.IsActive() || market.NormalizedExchange() != exchange || strings.TrimSpace(market.SourceSymbol) == "" {
				continue
			}

			result = append(result, symbols.SymbolMarket{
				SymbolID:        symbol.ID,
				CanonicalSymbol: symbol.Symbol,
				Exchange:        exchange,
				MarketType:      strings.TrimSpace(market.MarketType),
				SourceSymbol:    strings.TrimSpace(market.SourceSymbol),
				Status:          strings.TrimSpace(market.Status),
			})
			break
		}
		if limit > 0 && len(result) >= limit {
			break
		}
	}

	return result, nil
}

func (r *Repository) InsertJobs(ctx context.Context, jobs []Job) (int, error) {
	if len(jobs) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin insert jobs: %w", err)
	}
	defer tx.Rollback()

	inserted := 0
	for _, job := range jobs {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO derivative_collection_jobs (
			    exchange, data_type, tier, symbol_id, source_symbol, period,
			    idempotency_key, status, priority, scheduled_at, retry_count,
			    max_retry, metadata, job_mode, parent_gap_id, backfill_checkpoint
			)
			VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7, $8, $9, $10, $11, $12, $13, COALESCE(NULLIF($14, ''), 'realtime'), NULLIF($15, 0), $16)
			ON CONFLICT (idempotency_key) DO NOTHING
		`,
			job.Exchange,
			job.DataType,
			job.Tier,
			job.SymbolID,
			job.SourceSymbol,
			job.Period,
			job.IdempotencyKey,
			JobStatusPending,
			job.Priority,
			job.ScheduledAt,
			job.RetryCount,
			job.MaxRetry,
			ensureJSON(job.Metadata),
			job.JobMode,
			job.ParentGapID,
			ensureJSON(job.BackfillCheckpoint),
		)
		if err != nil {
			return 0, fmt.Errorf("insert job %s: %w", job.IdempotencyKey, err)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("job rows affected: %w", err)
		}
		inserted += int(rowsAffected)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit insert jobs: %w", err)
	}

	return inserted, nil
}

func (r *Repository) ClaimPendingJobs(ctx context.Context, limit int) ([]Job, error) {
	if limit < 1 {
		return nil, fmt.Errorf("limit must be greater than 0")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin claim jobs: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		WITH selected AS (
		    SELECT id
		    FROM derivative_collection_jobs
		    WHERE status = 'pending'
		      AND scheduled_at <= now()
		    ORDER BY priority ASC, scheduled_at ASC, id ASC
		    LIMIT $1
		    FOR UPDATE SKIP LOCKED
		)
		UPDATE derivative_collection_jobs j
		SET status = 'running',
		    started_at = now(),
		    updated_at = now()
		FROM selected
		WHERE j.id = selected.id
		RETURNING j.id, j.exchange, j.data_type, j.tier, j.symbol_id, j.source_symbol,
		          j.period, j.idempotency_key, j.status, j.priority, j.scheduled_at,
		          j.started_at, j.finished_at, j.retry_count, j.max_retry, j.error_message,
		          j.metadata, j.job_mode, j.parent_gap_id, j.backfill_checkpoint,
		          j.created_at, j.updated_at
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("claim jobs: %w", err)
	}
	defer rows.Close()

	jobs, err := scanJobs(rows)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit claim jobs: %w", err)
	}

	return jobs, nil
}

func (r *Repository) MarkJobSucceeded(ctx context.Context, jobID int64) error {
	return r.updateJobTerminal(ctx, jobID, JobStatusSucceeded, "")
}

func (r *Repository) MarkJobFailed(ctx context.Context, job Job, message string, deadLetter bool) error {
	return r.MarkJobFailedDetailed(ctx, job, JobFailure{
		Kind:       "",
		Message:    message,
		EndpointID: RuntimeMetadataFromJob(job).EndpointID,
		Payload:    ensureJSON(job.Metadata),
	}, deadLetter)
}

func (r *Repository) MarkJobFailedDetailed(ctx context.Context, job Job, failure JobFailure, deadLetter bool) error {
	status := JobStatusFailed
	if deadLetter {
		status = JobStatusDeadLetter
	}
	if failure.Message == "" {
		failure.Message = string(failure.Kind)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin mark job failed: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		UPDATE derivative_collection_jobs
		SET status = $2,
		    finished_at = now(),
		    retry_count = retry_count + 1,
		    error_message = $3,
		    last_error_type = NULLIF($4, ''),
		    updated_at = now()
		WHERE id = $1
	`, job.ID, status, failure.Message, string(failure.Kind)); err != nil {
		return fmt.Errorf("mark job failed: %w", err)
	}

	if deadLetter {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO failed_collection_jobs (
			    job_id, idempotency_key, exchange, data_type, tier, symbol_id,
			    source_symbol, period, retry_count, error_type, error_message,
			    endpoint_id, payload, safe_payload_sample
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9, $10, $11, NULLIF($12, 0), $13, $13)
			ON CONFLICT DO NOTHING
		`,
			job.ID,
			job.IdempotencyKey,
			job.Exchange,
			job.DataType,
			job.Tier,
			job.SymbolID,
			job.SourceSymbol,
			job.Period,
			job.RetryCount+1,
			defaultFailureKind(failure.Kind),
			failure.Message,
			failure.EndpointID,
			ensureJSON(failure.Payload),
		); err != nil {
			return fmt.Errorf("dead-letter job: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit mark job failed: %w", err)
	}

	return nil
}

func (r *Repository) RetryJobLater(ctx context.Context, job Job, nextRun time.Time, message string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE derivative_collection_jobs
		SET status = 'pending',
		    scheduled_at = $2,
		    next_retry_at = $2,
		    finished_at = now(),
		    retry_count = retry_count + 1,
		    error_message = $3,
		    last_error_type = COALESCE(NULLIF(last_error_type, ''), 'unknown'),
		    updated_at = now()
		WHERE id = $1
	`, job.ID, nextRun, message)
	if err != nil {
		return fmt.Errorf("retry job later: %w", err)
	}

	return nil
}

func (r *Repository) updateJobTerminal(ctx context.Context, jobID int64, status string, message string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE derivative_collection_jobs
		SET status = $2,
		    finished_at = now(),
		    next_retry_at = NULL,
		    error_message = NULLIF($3, ''),
		    updated_at = now()
		WHERE id = $1
	`, jobID, status, message)
	if err != nil {
		return fmt.Errorf("mark job %s: %w", status, err)
	}

	return nil
}

func defaultFailureKind(kind any) string {
	value := strings.TrimSpace(fmt.Sprint(kind))
	if value == "" {
		return "unknown"
	}
	return value
}

func scanPolicy(rows *sql.Rows) (Policy, error) {
	var (
		policy     Policy
		period     sql.NullString
		staleAfter sql.NullInt64
	)

	if err := rows.Scan(
		&policy.ID,
		&policy.Exchange,
		&policy.MarketType,
		&policy.DataType,
		&policy.Tier,
		&period,
		&policy.IntervalSeconds,
		&policy.BatchSize,
		&policy.Priority,
		&policy.Enabled,
		&policy.MaxRetry,
		&staleAfter,
		&policy.Metadata,
		&policy.CreatedAt,
		&policy.UpdatedAt,
	); err != nil {
		return Policy{}, fmt.Errorf("scan collection policy: %w", err)
	}

	policy.Period = period.String
	policy.StaleAfterSeconds = int(staleAfter.Int64)

	return policy, nil
}

func scanJobs(rows *sql.Rows) ([]Job, error) {
	var jobs []Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs: %w", err)
	}

	return jobs, nil
}

func scanJob(rows *sql.Rows) (Job, error) {
	var (
		job          Job
		symbolID     sql.NullInt64
		period       sql.NullString
		startedAt    sql.NullTime
		finishedAt   sql.NullTime
		errorMessage sql.NullString
		jobMode      sql.NullString
		parentGapID  sql.NullInt64
		checkpoint   json.RawMessage
	)

	if err := rows.Scan(
		&job.ID,
		&job.Exchange,
		&job.DataType,
		&job.Tier,
		&symbolID,
		&job.SourceSymbol,
		&period,
		&job.IdempotencyKey,
		&job.Status,
		&job.Priority,
		&job.ScheduledAt,
		&startedAt,
		&finishedAt,
		&job.RetryCount,
		&job.MaxRetry,
		&errorMessage,
		&job.Metadata,
		&jobMode,
		&parentGapID,
		&checkpoint,
		&job.CreatedAt,
		&job.UpdatedAt,
	); err != nil {
		return Job{}, fmt.Errorf("scan job: %w", err)
	}

	job.SymbolID = symbolID.Int64
	job.Period = period.String
	job.ErrorMessage = errorMessage.String
	job.JobMode = jobMode.String
	job.ParentGapID = parentGapID.Int64
	job.BackfillCheckpoint = ensureJSON(checkpoint)
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		job.FinishedAt = &finishedAt.Time
	}

	return job, nil
}

func (r *Repository) watchlistSymbolIDs(ctx context.Context) (map[int64]struct{}, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT symbol_id
		FROM symbol_collection_tiers
		WHERE tier = 'watchlist'
		  AND is_active = true
		ORDER BY priority ASC, symbol_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query watchlist symbols: %w", err)
	}
	defer rows.Close()

	result := make(map[int64]struct{})
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan watchlist symbol: %w", err)
		}
		result[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate watchlist symbols: %w", err)
	}

	return result, nil
}

func filterSymbolsByID(input []symbols.Symbol, ids map[int64]struct{}) []symbols.Symbol {
	if len(ids) == 0 {
		return []symbols.Symbol{}
	}

	filtered := make([]symbols.Symbol, 0, len(input))
	for _, symbol := range input {
		if _, ok := ids[symbol.ID]; ok {
			filtered = append(filtered, symbol)
		}
	}

	return filtered
}

func ensureJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}

	return raw
}
