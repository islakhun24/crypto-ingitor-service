ALTER TABLE derivative_collection_jobs
    ADD COLUMN IF NOT EXISTS job_mode TEXT NOT NULL DEFAULT 'realtime',
    ADD COLUMN IF NOT EXISTS parent_gap_id BIGINT REFERENCES data_gaps(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS backfill_checkpoint JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS last_error_type TEXT,
    ADD COLUMN IF NOT EXISTS next_retry_at TIMESTAMPTZ;

ALTER TABLE failed_collection_jobs
    ADD COLUMN IF NOT EXISTS endpoint_id BIGINT REFERENCES exchange_api_endpoints(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS last_status_code INTEGER,
    ADD COLUMN IF NOT EXISTS safe_payload_sample JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE data_quality_issues
    ADD COLUMN IF NOT EXISTS job_id BIGINT REFERENCES derivative_collection_jobs(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS endpoint_id BIGINT REFERENCES exchange_api_endpoints(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS observed_at TIMESTAMPTZ NOT NULL DEFAULT now();

ALTER TABLE data_gaps
    ADD COLUMN IF NOT EXISTS expected_interval_seconds INTEGER,
    ADD COLUMN IF NOT EXISTS last_observed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS backfill_job_id BIGINT REFERENCES derivative_collection_jobs(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS ix_derivative_collection_jobs_mode_status_scheduled
    ON derivative_collection_jobs (job_mode, status, scheduled_at, priority);

CREATE INDEX IF NOT EXISTS ix_derivative_collection_jobs_running_stale
    ON derivative_collection_jobs (status, started_at)
    WHERE status = 'running';

CREATE INDEX IF NOT EXISTS ix_derivative_collection_jobs_parent_gap
    ON derivative_collection_jobs (parent_gap_id)
    WHERE parent_gap_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_failed_collection_jobs_endpoint_failed
    ON failed_collection_jobs (endpoint_id, failed_at DESC)
    WHERE endpoint_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_data_quality_issues_job_observed
    ON data_quality_issues (job_id, observed_at DESC)
    WHERE job_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_data_gaps_backfill_status_detected
    ON data_gaps (backfill_status, detected_at DESC);
