package aggregate

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	aggmodel "aggregator-services/internal/aggregation"
	"aggregator-services/internal/ratelimit"
	"aggregator-services/internal/repositories"
	"aggregator-services/internal/scheduler"
)

type Store interface {
	BuildLatest(ctx context.Context, symbolID int64, snapshotTime time.Time) (aggmodel.DerivativeAggregateSnapshot, error)
	Upsert(ctx context.Context, snapshot aggmodel.DerivativeAggregateSnapshot) error
}

type AnalyticsStore interface {
	BuildAnalytics(ctx context.Context, symbolID int64, snapshotTime time.Time) (aggmodel.AnalyticsSnapshotSet, error)
	UpsertAnalytics(ctx context.Context, set aggmodel.AnalyticsSnapshotSet) error
}

type HealthReporter interface {
	Upsert(ctx context.Context, health repositories.CollectorHealth) error
}

type Executor struct {
	Store    Store
	Health   HealthReporter
	Service  string
	Instance string
	Now      func() time.Time
}

func (e Executor) Execute(ctx context.Context, job scheduler.Job) error {
	if job.SymbolID <= 0 {
		return scheduler.NewExecutionError(ratelimit.FailureParse, false, fmt.Errorf("missing symbol_id"))
	}

	snapshotTime := job.ScheduledAt
	if snapshotTime.IsZero() {
		snapshotTime = e.now()
	}

	snapshot, err := e.Store.BuildLatest(ctx, job.SymbolID, snapshotTime)
	if err != nil {
		e.report(ctx, job, "degraded", err.Error())
		return scheduler.NewExecutionError(ratelimit.FailureServerError, true, err)
	}
	if snapshot.ExchangeCount == 0 {
		err := fmt.Errorf("no exchange snapshots available for symbol_id %d", job.SymbolID)
		e.report(ctx, job, "degraded", err.Error())
		return scheduler.NewExecutionError(ratelimit.FailureServerError, true, err)
	}
	if err := e.Store.Upsert(ctx, snapshot); err != nil {
		e.report(ctx, job, "degraded", err.Error())
		return scheduler.NewExecutionError(ratelimit.FailureServerError, true, err)
	}
	if analyticsStore, ok := e.Store.(AnalyticsStore); ok {
		analytics, err := analyticsStore.BuildAnalytics(ctx, job.SymbolID, snapshotTime)
		if err != nil {
			e.report(ctx, job, "degraded", err.Error())
			return scheduler.NewExecutionError(ratelimit.FailureServerError, true, err)
		}
		if err := analyticsStore.UpsertAnalytics(ctx, analytics); err != nil {
			e.report(ctx, job, "degraded", err.Error())
			return scheduler.NewExecutionError(ratelimit.FailureServerError, true, err)
		}
	}

	e.report(ctx, job, "healthy", "")
	return nil
}

func (e Executor) report(ctx context.Context, job scheduler.Job, status string, message string) {
	if e.Health == nil {
		return
	}

	at := e.now()
	metrics, _ := json.Marshal(map[string]any{"symbol_id": job.SymbolID})
	health := repositories.CollectorHealth{
		ServiceName:  e.service(),
		InstanceID:   e.instance(),
		Exchange:     "aggregate",
		DataType:     job.DataType,
		Status:       status,
		HeartbeatAt:  at,
		ErrorMessage: message,
		Metrics:      metrics,
	}
	if status == "healthy" {
		health.LastSuccessAt = at
	} else {
		health.LastErrorAt = at
	}
	_ = e.Health.Upsert(ctx, health)
}

func (e Executor) now() time.Time {
	if e.Now != nil {
		return e.Now().UTC()
	}
	return time.Now().UTC()
}

func (e Executor) service() string {
	if e.Service == "" {
		return "collector-service"
	}
	return e.Service
}

func (e Executor) instance() string {
	if e.Instance == "" {
		return "default"
	}
	return e.Instance
}
