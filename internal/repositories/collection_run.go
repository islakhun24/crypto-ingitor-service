package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type CollectionRun struct {
	RunKey           string
	ServiceName      string
	Exchange         string
	DataType         string
	Status           string
	StartedAt        time.Time
	FinishedAt       time.Time
	SymbolsPlanned   int
	SymbolsSucceeded int
	SymbolsFailed    int
	JobsCreated      int
	Metadata         json.RawMessage
}

type CollectionRunRepository struct {
	db *sql.DB
}

func NewCollectionRunRepository(db *sql.DB) *CollectionRunRepository {
	return &CollectionRunRepository{db: db}
}

func (r *CollectionRunRepository) Start(ctx context.Context, run CollectionRun) error {
	if run.StartedAt.IsZero() {
		run.StartedAt = time.Now().UTC()
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO data_collection_runs (
		    run_key, service_name, exchange, data_type, status, started_at,
		    symbols_planned, symbols_succeeded, symbols_failed, jobs_created, metadata
		)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (run_key) DO UPDATE SET
		    status = EXCLUDED.status,
		    started_at = EXCLUDED.started_at,
		    metadata = EXCLUDED.metadata
	`,
		run.RunKey,
		run.ServiceName,
		run.Exchange,
		run.DataType,
		run.Status,
		run.StartedAt,
		run.SymbolsPlanned,
		run.SymbolsSucceeded,
		run.SymbolsFailed,
		run.JobsCreated,
		ensureJSON(run.Metadata),
	)
	if err != nil {
		return fmt.Errorf("start collection run: %w", err)
	}

	return nil
}

func (r *CollectionRunRepository) Finish(ctx context.Context, runKey string, status string, metadata json.RawMessage) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE data_collection_runs
		SET status = $2,
		    finished_at = now(),
		    metadata = $3
		WHERE run_key = $1
	`, runKey, status, ensureJSON(metadata))
	if err != nil {
		return fmt.Errorf("finish collection run: %w", err)
	}

	return nil
}
