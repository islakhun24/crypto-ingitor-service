package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"aggregator-services/internal/ratelimit"
)

func TestWorkerMarksSuccess(t *testing.T) {
	store := &fakeWorkerStore{jobs: []Job{{ID: 1, MaxRetry: 3, Metadata: []byte(`{"endpoint_name":"ticker","request_weight":1,"requires_endpoint":false}`)}}}
	worker := Worker{
		Jobs:      store,
		Executor:  fakeExecutor{},
		BatchSize: 1,
	}

	processed, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}
	if store.succeeded != 1 {
		t.Fatalf("succeeded = %d, want 1", store.succeeded)
	}
}

func TestWorkerDeadLettersNonRecoverableError(t *testing.T) {
	store := &fakeWorkerStore{jobs: []Job{{ID: 1, MaxRetry: 3, Metadata: []byte(`{"endpoint_name":"ticker","request_weight":1,"requires_endpoint":false}`)}}}
	worker := Worker{
		Jobs: store,
		Executor: fakeExecutor{err: NewExecutionError(
			ratelimit.FailureParse,
			false,
			errors.New("parse changed response"),
		)},
		BatchSize: 1,
	}

	if _, err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if store.deadLetters != 1 {
		t.Fatalf("deadLetters = %d, want 1", store.deadLetters)
	}
}

func TestWorkerBackoffUsesExponentialDelayWithJitter(t *testing.T) {
	worker := Worker{
		Retry: RetryPolicy{
			Base:      time.Second,
			Max:       time.Minute,
			JitterMax: time.Second,
		},
	}
	delay := worker.backoff(Job{ID: 1, RetryCount: 3, IdempotencyKey: "job-key"})
	if delay < 8*time.Second || delay >= 9*time.Second {
		t.Fatalf("backoff = %s, want base exponential plus bounded jitter", delay)
	}
}

type fakeWorkerStore struct {
	jobs        []Job
	succeeded   int
	deadLetters int
	retried     int
}

func (s *fakeWorkerStore) ClaimPendingJobs(context.Context, int) ([]Job, error) {
	jobs := s.jobs
	s.jobs = nil
	return jobs, nil
}

func (s *fakeWorkerStore) MarkJobSucceeded(context.Context, int64) error {
	s.succeeded++
	return nil
}

func (s *fakeWorkerStore) MarkJobFailed(_ context.Context, _ Job, _ string, deadLetter bool) error {
	if deadLetter {
		s.deadLetters++
	}
	return nil
}

func (s *fakeWorkerStore) RetryJobLater(context.Context, Job, time.Time, string) error {
	s.retried++
	return nil
}

type fakeExecutor struct {
	err error
}

func (e fakeExecutor) Execute(context.Context, Job) error {
	return e.err
}
