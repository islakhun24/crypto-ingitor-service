package retention

import (
	"strings"
	"testing"
	"time"
)

func TestValidatePolicyRejectsUnsupportedTable(t *testing.T) {
	_, err := ValidatePolicy(Policy{
		TableName:      "symbols",
		TimeColumn:     "created_at",
		RetentionDays:  30,
		ChunkSize:      1000,
		MaxRowsPerRun:  1000,
		TimeoutSeconds: 30,
	})
	if err == nil {
		t.Fatal("ValidatePolicy() error = nil, want unsupported table error")
	}
}

func TestValidatePolicyRejectsTooSmallRetention(t *testing.T) {
	_, err := ValidatePolicy(Policy{
		TableName:        "derivative_klines",
		TimeColumn:       "open_time",
		IntervalColumn:   "interval",
		IntervalValue:    "1m",
		RetentionDays:    1,
		ChunkSize:        1000,
		MaxRowsPerRun:    1000,
		TimeoutSeconds:   30,
		MinRetentionDays: 7,
	})
	if err == nil {
		t.Fatal("ValidatePolicy() error = nil, want minimum retention error")
	}
}

func TestBuildChunkDeleteSQLUsesBoundedSubquery(t *testing.T) {
	sql, err := BuildChunkDeleteSQL(Policy{
		TableName:        "derivative_klines",
		TimeColumn:       "open_time",
		IntervalColumn:   "interval",
		IntervalValue:    "1m",
		RetentionDays:    14,
		ChunkSize:        1000,
		MaxRowsPerRun:    5000,
		TimeoutSeconds:   30,
		MinRetentionDays: 7,
	})
	if err != nil {
		t.Fatalf("BuildChunkDeleteSQL() error = %v", err)
	}

	required := []string{
		"DELETE FROM \"derivative_klines\"",
		"WHERE \"id\" IN",
		"\"open_time\" < $1",
		"\"interval\" = $2",
		"ORDER BY \"open_time\" ASC",
		"LIMIT $3",
	}
	for _, fragment := range required {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("delete sql missing %q: %s", fragment, sql)
		}
	}
}

func TestPlannerBuildsKlineRollupPlan(t *testing.T) {
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	plan, err := Planner{Now: func() time.Time { return now }}.Plan([]Policy{
		{
			ID:                 1,
			TableName:          "derivative_klines",
			TimeColumn:         "open_time",
			IntervalColumn:     "interval",
			IntervalValue:      "5m",
			RetentionDays:      180,
			ChunkSize:          1000,
			Enabled:            true,
			DryRun:             true,
			RollupBeforeDelete: true,
			Priority:           10,
			MaxRowsPerRun:      5000,
			TimeoutSeconds:     30,
			PartitionStrategy:  "auto",
			MinRetentionDays:   30,
		},
	})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(plan) != 1 {
		t.Fatalf("plan length = %d, want 1", len(plan))
	}
	if plan[0].RollupTargetInterval != "15m" {
		t.Fatalf("RollupTargetInterval = %q, want 15m", plan[0].RollupTargetInterval)
	}
	if !plan[0].DryRun {
		t.Fatal("DryRun = false, want true")
	}
	if !plan[0].UsePartitionDrop {
		t.Fatal("UsePartitionDrop = false, want true for partition-preferred table")
	}
}

func TestKlineRollupTargetMapping(t *testing.T) {
	tests := map[string]string{
		"1m":  "5m",
		"5m":  "15m",
		"15m": "1h",
		"1h":  "4h",
		"4h":  "1d",
	}

	for source, want := range tests {
		if got := klineRollupTarget(source); got != want {
			t.Fatalf("klineRollupTarget(%q) = %q, want %q", source, got, want)
		}
	}
}

func TestParsePartitionRangeSupportsMonthlyAndWeeklyNames(t *testing.T) {
	start, end, ok := ParsePartitionRange("derivative_klines", "derivative_klines_2026_05")
	if !ok {
		t.Fatal("monthly partition was not parsed")
	}
	if start.Format("2006-01-02") != "2026-05-01" || end.Format("2006-01-02") != "2026-06-01" {
		t.Fatalf("monthly range = %s - %s", start, end)
	}

	start, end, ok = ParsePartitionRange("liquidation_events", "liquidation_events_2026_w02")
	if !ok {
		t.Fatal("weekly partition was not parsed")
	}
	if start.Weekday() != time.Monday || end.Sub(start) != 7*24*time.Hour {
		t.Fatalf("weekly range = %s - %s", start, end)
	}
}

func TestMetricsRenderPrometheusCounters(t *testing.T) {
	metrics := NewMetrics()
	metrics.Observe(CleanupResult{
		DryRun:            true,
		Status:            "succeeded",
		RowsMatched:       10,
		RowsDeleted:       0,
		PartitionsDropped: 0,
	})

	text := metrics.Prometheus()
	for _, fragment := range []string{
		`retention_cleanup_runs_total{dry_run="true",status="succeeded"} 1`,
		"retention_cleanup_rows_matched_total 10",
		"retention_cleanup_dry_run_total 1",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("metrics missing %q in:\n%s", fragment, text)
		}
	}
}
