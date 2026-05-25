package scheduler

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"aggregator-services/internal/ratelimit"
)

type WorkerJobStore interface {
	ClaimPendingJobs(ctx context.Context, limit int) ([]Job, error)
	MarkJobSucceeded(ctx context.Context, jobID int64) error
	MarkJobFailed(ctx context.Context, job Job, message string, deadLetter bool) error
	RetryJobLater(ctx context.Context, job Job, nextRun time.Time, message string) error
}

type DetailedFailureJobStore interface {
	MarkJobFailedDetailed(ctx context.Context, job Job, failure JobFailure, deadLetter bool) error
}

type Executor interface {
	Execute(ctx context.Context, job Job) error
}

type RateLimiter interface {
	Wait(ctx context.Context, key ratelimit.Key, weight int) error
}

type Circuit interface {
	Allow(ctx context.Context, key ratelimit.Key) (ratelimit.CircuitState, error)
	RecordSuccess(key ratelimit.Key)
	RecordFailure(key ratelimit.Key, kind ratelimit.FailureKind)
}

type Worker struct {
	Jobs       WorkerJobStore
	Executor   Executor
	Limiter    RateLimiter
	Breaker    Circuit
	BatchSize  int
	Now        func() time.Time
	Backoff    func(job Job) time.Duration
	Retry      RetryPolicy
	OnProgress func(job Job, status string)
}

type ExecutionError struct {
	Kind        ratelimit.FailureKind
	Recoverable bool
	Err         error
}

type RetryPolicy struct {
	Base      time.Duration
	Max       time.Duration
	JitterMax time.Duration
}

type JobFailure struct {
	Kind       ratelimit.FailureKind `json:"kind"`
	Message    string                `json:"message"`
	EndpointID int64                 `json:"endpoint_id,omitempty"`
	Payload    json.RawMessage       `json:"payload,omitempty"`
}

func (e ExecutionError) Error() string {
	if e.Err == nil {
		return string(e.Kind)
	}

	return e.Err.Error()
}

func (e ExecutionError) Unwrap() error {
	return e.Err
}

func (w Worker) RunOnce(ctx context.Context) (int, error) {
	batchSize := w.BatchSize
	if batchSize <= 0 {
		batchSize = 10
	}

	jobs, err := w.Jobs.ClaimPendingJobs(ctx, batchSize)
	if err != nil {
		return 0, err
	}

	processed := 0
	for _, job := range jobs {
		if err := w.process(ctx, job); err != nil {
			return processed, err
		}
		processed++
	}

	return processed, nil
}

func (w Worker) process(ctx context.Context, job Job) error {
	metadata := RuntimeMetadataFromJob(job)
	key := ratelimit.Key{
		Exchange: job.Exchange,
		Endpoint: metadata.EndpointName,
		DataType: job.DataType,
	}

	if w.Breaker != nil {
		if _, err := w.Breaker.Allow(ctx, key); err != nil {
			return w.retryOrDeadLetter(ctx, job, err, ratelimit.FailureRateLimited, true)
		}
	}

	if w.Limiter != nil && metadata.RequiresEndpoint {
		if err := w.Limiter.Wait(ctx, key, metadata.RequestWeight); err != nil {
			return w.retryOrDeadLetter(ctx, job, err, ratelimit.FailureRateLimited, true)
		}
	}

	if w.OnProgress != nil {
		w.OnProgress(job, JobStatusRunning)
	}

	err := w.Executor.Execute(ctx, job)
	if err != nil {
		kind, recoverable := classifyExecutionError(err)
		if w.Breaker != nil {
			w.Breaker.RecordFailure(key, kind)
		}
		return w.retryOrDeadLetter(ctx, job, err, kind, recoverable)
	}

	if w.Breaker != nil {
		w.Breaker.RecordSuccess(key)
	}
	if err := w.Jobs.MarkJobSucceeded(ctx, job.ID); err != nil {
		return err
	}
	if w.OnProgress != nil {
		w.OnProgress(job, JobStatusSucceeded)
	}

	return nil
}

func (w Worker) retryOrDeadLetter(ctx context.Context, job Job, err error, _ ratelimit.FailureKind, recoverable bool) error {
	deadLetter := !recoverable || job.RetryCount+1 >= job.MaxRetry
	kind, _ := classifyExecutionError(err)
	failure := JobFailure{
		Kind:       kind,
		Message:    err.Error(),
		EndpointID: RuntimeMetadataFromJob(job).EndpointID,
		Payload:    safeFailurePayload(job, err, kind),
	}
	if deadLetter {
		if detailed, ok := w.Jobs.(DetailedFailureJobStore); ok {
			return detailed.MarkJobFailedDetailed(ctx, job, failure, true)
		}
		return w.Jobs.MarkJobFailed(ctx, job, err.Error(), true)
	}

	nextRun := w.now().Add(w.backoff(job))
	return w.Jobs.RetryJobLater(ctx, job, nextRun, err.Error())
}

func (w Worker) now() time.Time {
	if w.Now != nil {
		return w.Now().UTC()
	}

	return time.Now().UTC()
}

func (w Worker) backoff(job Job) time.Duration {
	if w.Backoff != nil {
		return w.Backoff(job)
	}

	policy := w.Retry
	if policy.Base <= 0 {
		policy.Base = 30 * time.Second
	}
	if policy.Max <= 0 {
		policy.Max = 15 * time.Minute
	}
	if policy.JitterMax < 0 {
		policy.JitterMax = 0
	}

	attempt := job.RetryCount + 1
	if attempt < 1 {
		attempt = 1
	}
	exponent := math.Min(float64(attempt-1), 10)
	delay := time.Duration(float64(policy.Base) * math.Pow(2, exponent))
	if delay > policy.Max {
		delay = policy.Max
	}
	return delay + deterministicJitter(job, policy.JitterMax)
}

func classifyExecutionError(err error) (ratelimit.FailureKind, bool) {
	var execErr ExecutionError
	if errors.As(err, &execErr) {
		return execErr.Kind, execErr.Recoverable
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return ratelimit.FailureTimeout, true
	}

	return ratelimit.FailureServerError, true
}

type RuntimeMetadata struct {
	EndpointID         int64   `json:"endpoint_id"`
	EndpointName       string  `json:"endpoint_name"`
	EndpointDataType   string  `json:"endpoint_data_type"`
	RequiresEndpoint   bool    `json:"requires_endpoint"`
	RequestWeight      int     `json:"request_weight"`
	TimeoutMS          int     `json:"timeout_ms"`
	RateLimitPerSecond float64 `json:"rate_limit_per_second"`
	RateLimitPerMinute int     `json:"rate_limit_per_minute"`
}

func RuntimeMetadataFromJob(job Job) RuntimeMetadata {
	metadata := RuntimeMetadata{
		EndpointName:     "internal",
		RequiresEndpoint: true,
		RequestWeight:    1,
		TimeoutMS:        10000,
	}

	if len(job.Metadata) == 0 {
		return metadata
	}

	if err := json.Unmarshal(job.Metadata, &metadata); err != nil {
		return RuntimeMetadata{
			EndpointName:     "invalid_metadata",
			RequiresEndpoint: true,
			RequestWeight:    1,
			TimeoutMS:        10000,
		}
	}
	if metadata.EndpointName == "" {
		metadata.EndpointName = "internal"
	}
	if metadata.RequestWeight <= 0 {
		metadata.RequestWeight = 1
	}
	if metadata.TimeoutMS <= 0 {
		metadata.TimeoutMS = 10000
	}

	return metadata
}

func NewExecutionError(kind ratelimit.FailureKind, recoverable bool, err error) ExecutionError {
	if err == nil {
		err = fmt.Errorf("%s", kind)
	}

	return ExecutionError{Kind: kind, Recoverable: recoverable, Err: err}
}

func deterministicJitter(job Job, max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	seed := strings.Join([]string{job.IdempotencyKey, fmt.Sprint(job.ID), fmt.Sprint(job.RetryCount)}, "|")
	sum := sha1.Sum([]byte(seed))
	value := int64(sum[0])<<24 | int64(sum[1])<<16 | int64(sum[2])<<8 | int64(sum[3])
	if value < 0 {
		value = -value
	}
	return time.Duration(value % int64(max))
}

func safeFailurePayload(job Job, err error, kind ratelimit.FailureKind) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{
		"job": map[string]any{
			"id":              job.ID,
			"exchange":        job.Exchange,
			"data_type":       job.DataType,
			"tier":            job.Tier,
			"symbol_id":       job.SymbolID,
			"source_symbol":   job.SourceSymbol,
			"period":          job.Period,
			"idempotency_key": job.IdempotencyKey,
			"retry_count":     job.RetryCount,
			"max_retry":       job.MaxRetry,
		},
		"error": map[string]any{
			"type":    kind,
			"message": err.Error(),
		},
		"metadata": RuntimeMetadataFromJob(job),
	})
	return raw
}
