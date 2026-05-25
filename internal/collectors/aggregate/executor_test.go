package aggregate

import (
	"context"
	"testing"
	"time"

	aggmodel "aggregator-services/internal/aggregation"
	"aggregator-services/internal/repositories"
	"aggregator-services/internal/scheduler"
)

func TestExecutorBuildsAndWritesAggregate(t *testing.T) {
	store := &fakeStore{snapshot: aggmodel.DerivativeAggregateSnapshot{
		SymbolID:           123,
		ExchangeCount:      2,
		SnapshotTime:       time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC),
		RawByExchange:      []byte(`{}`),
		AvailableExchanges: []byte(`["binance","okx"]`),
	}}
	executor := Executor{
		Store:  store,
		Health: &fakeAggregateHealth{},
	}

	err := executor.Execute(context.Background(), scheduler.Job{
		DataType:    "aggregated_snapshot",
		SymbolID:    123,
		ScheduledAt: time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if store.upserts != 1 {
		t.Fatalf("upserts = %d, want 1", store.upserts)
	}
}

type fakeStore struct {
	snapshot aggmodel.DerivativeAggregateSnapshot
	upserts  int
}

func (f *fakeStore) BuildLatest(context.Context, int64, time.Time) (aggmodel.DerivativeAggregateSnapshot, error) {
	return f.snapshot, nil
}

func (f *fakeStore) Upsert(context.Context, aggmodel.DerivativeAggregateSnapshot) error {
	f.upserts++
	return nil
}

type fakeAggregateHealth struct{}

func (f *fakeAggregateHealth) Upsert(context.Context, repositories.CollectorHealth) error {
	return nil
}
