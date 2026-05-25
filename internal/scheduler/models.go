package scheduler

import (
	"encoding/json"
	"time"
)

const (
	TierAll       = "all"
	TierTop100    = "top100"
	TierWatchlist = "watchlist"
)

const (
	JobStatusPending    = "pending"
	JobStatusRunning    = "running"
	JobStatusSucceeded  = "succeeded"
	JobStatusFailed     = "failed"
	JobStatusDeadLetter = "dead_letter"
	JobStatusCancelled  = "cancelled"
)

type Policy struct {
	ID                int64           `json:"id"`
	Exchange          string          `json:"exchange"`
	MarketType        string          `json:"market_type"`
	DataType          string          `json:"data_type"`
	Tier              string          `json:"tier"`
	Period            string          `json:"period,omitempty"`
	IntervalSeconds   int             `json:"interval_seconds"`
	BatchSize         int             `json:"batch_size"`
	Priority          int             `json:"priority"`
	Enabled           bool            `json:"enabled"`
	MaxRetry          int             `json:"max_retry"`
	StaleAfterSeconds int             `json:"stale_after_seconds"`
	Metadata          json.RawMessage `json:"metadata"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type Job struct {
	ID                 int64           `json:"id"`
	Exchange           string          `json:"exchange"`
	DataType           string          `json:"data_type"`
	Tier               string          `json:"tier"`
	SymbolID           int64           `json:"symbol_id"`
	SourceSymbol       string          `json:"source_symbol"`
	Period             string          `json:"period,omitempty"`
	IdempotencyKey     string          `json:"idempotency_key"`
	Status             string          `json:"status"`
	Priority           int             `json:"priority"`
	ScheduledAt        time.Time       `json:"scheduled_at"`
	StartedAt          *time.Time      `json:"started_at,omitempty"`
	FinishedAt         *time.Time      `json:"finished_at,omitempty"`
	RetryCount         int             `json:"retry_count"`
	MaxRetry           int             `json:"max_retry"`
	ErrorMessage       string          `json:"error_message,omitempty"`
	Metadata           json.RawMessage `json:"metadata"`
	JobMode            string          `json:"job_mode,omitempty"`
	ParentGapID        int64           `json:"parent_gap_id,omitempty"`
	BackfillCheckpoint json.RawMessage `json:"backfill_checkpoint,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type PlanResult struct {
	AttemptedJobs int               `json:"attempted_jobs"`
	InsertedJobs  int               `json:"inserted_jobs"`
	Skipped       map[string]int    `json:"skipped"`
	Reasons       map[string]string `json:"reasons"`
}

func NewPlanResult() PlanResult {
	return PlanResult{
		Skipped: make(map[string]int),
		Reasons: make(map[string]string),
	}
}

func (r *PlanResult) Skip(key string, reason string) {
	r.Skipped[key]++
	if _, ok := r.Reasons[key]; !ok {
		r.Reasons[key] = reason
	}
}
