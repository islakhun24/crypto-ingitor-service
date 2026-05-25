package scheduler

import (
	"testing"
	"time"
)

func TestPlanBackfillJobsCreatesLowPriorityIdempotentJobs(t *testing.T) {
	start := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	jobs, err := PlanBackfillJobs(BackfillRequest{
		Exchange:        "binance",
		DataType:        "kline",
		Tier:            TierAll,
		SymbolID:        123,
		SourceSymbol:    "BTCUSDT",
		Period:          "1m",
		Start:           start,
		End:             start.Add(3 * time.Minute),
		IntervalSeconds: 60,
		EndpointMetadata: RuntimeMetadata{
			RequiresEndpoint: true,
			EndpointID:       10,
			EndpointName:     "klines",
			EndpointDataType: "kline",
			RequestWeight:    1,
			TimeoutMS:        10000,
		},
	})
	if err != nil {
		t.Fatalf("PlanBackfillJobs() error = %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("jobs = %d, want 3", len(jobs))
	}
	if jobs[0].JobMode != JobModeBackfill || jobs[0].Priority != 1000 {
		t.Fatalf("job mode/priority = %s/%d", jobs[0].JobMode, jobs[0].Priority)
	}
	if jobs[0].IdempotencyKey != "binance:kline:123:BTCUSDT:1m:2026-05-25T12:00:00Z" {
		t.Fatalf("idempotency key = %q", jobs[0].IdempotencyKey)
	}
	if len(jobs[0].BackfillCheckpoint) == 0 {
		t.Fatal("BackfillCheckpoint is empty")
	}
}

func TestPlanBackfillJobsPausesWhenRealtimeLagIsHigh(t *testing.T) {
	start := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	jobs, err := PlanBackfillJobs(BackfillRequest{
		Exchange:              "binance",
		DataType:              "kline",
		Tier:                  TierAll,
		SymbolID:              123,
		SourceSymbol:          "BTCUSDT",
		Start:                 start,
		End:                   start.Add(time.Minute),
		IntervalSeconds:       60,
		RealtimeLag:           10 * time.Minute,
		RealtimeLagPauseAfter: 5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("PlanBackfillJobs() error = %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("jobs = %d, want paused backfill", len(jobs))
	}
}
