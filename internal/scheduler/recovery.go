package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type RecoveryResult struct {
	RunningJobsReset          int64 `json:"running_jobs_reset"`
	CollectionRunsInterrupted int64 `json:"collection_runs_interrupted"`
}

func (r *Repository) RecoverInterrupted(ctx context.Context, runningJobTimeout time.Duration) (RecoveryResult, error) {
	if runningJobTimeout <= 0 {
		runningJobTimeout = 15 * time.Minute
	}

	cutoff := time.Now().UTC().Add(-runningJobTimeout)
	result := RecoveryResult{}

	jobResult, err := r.db.ExecContext(ctx, `
		UPDATE derivative_collection_jobs
		SET status = 'pending',
		    scheduled_at = LEAST(now(), scheduled_at),
		    retry_count = retry_count + 1,
		    error_message = 'interrupted during restart recovery',
		    last_error_type = 'network_error',
		    next_retry_at = now(),
		    metadata = metadata || $2::jsonb,
		    updated_at = now()
		WHERE status = 'running'
		  AND started_at < $1
	`, cutoff, recoveryMetadata("running_job_timeout", cutoff))
	if err != nil {
		return RecoveryResult{}, fmt.Errorf("recover running jobs: %w", err)
	}
	result.RunningJobsReset, _ = jobResult.RowsAffected()

	runResult, err := r.db.ExecContext(ctx, `
		UPDATE data_collection_runs
		SET status = 'failed',
		    finished_at = now(),
		    metadata = metadata || $2::jsonb
		WHERE status = 'running'
		  AND started_at < $1
	`, cutoff, recoveryMetadata("service_restart_interrupted", cutoff))
	if err != nil {
		return RecoveryResult{}, fmt.Errorf("recover collection runs: %w", err)
	}
	result.CollectionRunsInterrupted, _ = runResult.RowsAffected()

	return result, nil
}

func recoveryMetadata(reason string, cutoff time.Time) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{
		"recovery": map[string]any{
			"reason": reason,
			"cutoff": cutoff.UTC().Format(time.RFC3339),
		},
	})
	return raw
}
