package scheduler

import (
	"context"
	"testing"
	"time"

	"aggregator-services/internal/endpoints"
	"aggregator-services/internal/symbols"
)

func TestPlannerCreatesJobsWithSourceSymbol(t *testing.T) {
	now := time.Date(2026, 5, 25, 12, 3, 0, 0, time.UTC)
	jobStore := &fakeJobStore{seen: map[string]Job{}}
	planner := Planner{
		Policies: fakePolicyStore{policies: []Policy{{
			ID:              1,
			Exchange:        "okx",
			MarketType:      "swap",
			DataType:        "kline",
			Tier:            TierAll,
			Period:          "5m",
			IntervalSeconds: 300,
			Priority:        10,
			Enabled:         true,
			MaxRetry:        3,
		}}},
		Endpoints: fakeEndpointStore{items: map[string][]endpoints.Endpoint{
			"okx:kline": {{ID: 22, Name: "candles", RequestWeight: 1, TimeoutMS: 10000, RateLimitPerSecond: 2, RateLimitPerMinute: 120}},
		}},
		Symbols: fakeSymbolStore{items: map[string][]symbols.SymbolMarket{
			"all:okx:0": {{SymbolID: 123, CanonicalSymbol: "0GUSDT", Exchange: "okx", SourceSymbol: "0G-USDT-SWAP", Status: "live"}},
		}},
		Jobs: jobStore,
	}

	result, err := planner.RunAt(context.Background(), now)
	if err != nil {
		t.Fatalf("RunAt() error = %v", err)
	}

	if result.AttemptedJobs != 1 || result.InsertedJobs != 1 {
		t.Fatalf("result = %+v", result)
	}

	key := "okx:kline:123:0G-USDT-SWAP:5m:2026-05-25T12:00:00Z"
	job, ok := jobStore.seen[key]
	if !ok {
		t.Fatalf("missing job key %s", key)
	}
	if job.SourceSymbol != "0G-USDT-SWAP" {
		t.Fatalf("SourceSymbol = %q", job.SourceSymbol)
	}
}

func TestPlannerSkipsInactiveOrMissingEndpoint(t *testing.T) {
	planner := Planner{
		Policies: fakePolicyStore{policies: []Policy{{
			ID:              1,
			Exchange:        "binance",
			MarketType:      "usds-m-futures",
			DataType:        "basis",
			Tier:            TierAll,
			IntervalSeconds: 900,
			Enabled:         true,
			MaxRetry:        3,
		}}},
		Endpoints: fakeEndpointStore{items: map[string][]endpoints.Endpoint{}},
		Symbols: fakeSymbolStore{items: map[string][]symbols.SymbolMarket{
			"all:binance:0": {{SymbolID: 123, Exchange: "binance", SourceSymbol: "0GUSDT", Status: "TRADING"}},
		}},
		Jobs: &fakeJobStore{seen: map[string]Job{}},
	}

	result, err := planner.RunAt(context.Background(), time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("RunAt() error = %v", err)
	}

	if result.AttemptedJobs != 0 || result.InsertedJobs != 0 {
		t.Fatalf("result = %+v", result)
	}
	if result.Skipped["binance:usds-m-futures:basis:all:"] != 1 {
		t.Fatalf("Skipped = %+v", result.Skipped)
	}
}

func TestPlannerCreatesInternalAggregationJobsWithoutEndpoint(t *testing.T) {
	jobStore := &fakeJobStore{seen: map[string]Job{}}
	planner := Planner{
		Policies: fakePolicyStore{policies: []Policy{{
			ID:              2,
			Exchange:        "mexc",
			MarketType:      "usdt-futures",
			DataType:        "aggregated_snapshot",
			Tier:            TierTop100,
			IntervalSeconds: 60,
			Enabled:         true,
			MaxRetry:        3,
		}}},
		Endpoints: fakeEndpointStore{items: map[string][]endpoints.Endpoint{}},
		Symbols: fakeSymbolStore{items: map[string][]symbols.SymbolMarket{
			"top100:mexc:100": {{SymbolID: 123, Exchange: "mexc", SourceSymbol: "0G_USDT", Status: "active"}},
		}},
		Jobs: jobStore,
	}

	result, err := planner.RunAt(context.Background(), time.Date(2026, 5, 25, 12, 0, 1, 0, time.UTC))
	if err != nil {
		t.Fatalf("RunAt() error = %v", err)
	}

	if result.InsertedJobs != 1 {
		t.Fatalf("InsertedJobs = %d, want 1", result.InsertedJobs)
	}
}

type fakePolicyStore struct {
	policies []Policy
}

func (s fakePolicyStore) ListActivePolicies(context.Context) ([]Policy, error) {
	return s.policies, nil
}

type fakeEndpointStore struct {
	items map[string][]endpoints.Endpoint
}

func (s fakeEndpointStore) ListActiveByExchangeDataType(_ context.Context, exchange string, dataType string) ([]endpoints.Endpoint, error) {
	return s.items[exchange+":"+dataType], nil
}

type fakeSymbolStore struct {
	items map[string][]symbols.SymbolMarket
}

func (s fakeSymbolStore) ListSymbolMarketsByTier(_ context.Context, tier string, exchange string, limit int) ([]symbols.SymbolMarket, error) {
	return s.items[tier+":"+exchange+":"+itoa(limit)], nil
}

type fakeJobStore struct {
	seen map[string]Job
}

func (s *fakeJobStore) InsertJobs(_ context.Context, jobs []Job) (int, error) {
	inserted := 0
	for _, job := range jobs {
		if _, ok := s.seen[job.IdempotencyKey]; ok {
			continue
		}
		s.seen[job.IdempotencyKey] = job
		inserted++
	}
	return inserted, nil
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}

	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}

	return string(digits)
}
