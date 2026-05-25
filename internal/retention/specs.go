package retention

import (
	"fmt"
	"strings"
)

type TableSpec struct {
	Name               string
	IDColumn           string
	TimeColumns        map[string]struct{}
	IntervalColumns    map[string]struct{}
	MinRetentionDays   int
	PartitionPreferred bool
}

var tableSpecs = map[string]TableSpec{
	"derivative_klines": {
		Name:               "derivative_klines",
		IDColumn:           "id",
		TimeColumns:        setOf("open_time"),
		IntervalColumns:    setOf("interval"),
		MinRetentionDays:   7,
		PartitionPreferred: true,
	},
	"derivative_market_snapshots": {
		Name:               "derivative_market_snapshots",
		IDColumn:           "id",
		TimeColumns:        setOf("snapshot_time"),
		MinRetentionDays:   7,
		PartitionPreferred: true,
	},
	"open_interest_snapshots": {
		Name:               "open_interest_snapshots",
		IDColumn:           "id",
		TimeColumns:        setOf("snapshot_time"),
		MinRetentionDays:   14,
		PartitionPreferred: true,
	},
	"funding_rate_snapshots": {
		Name:             "funding_rate_snapshots",
		IDColumn:         "id",
		TimeColumns:      setOf("snapshot_time"),
		MinRetentionDays: 30,
	},
	"funding_rate_history": {
		Name:             "funding_rate_history",
		IDColumn:         "id",
		TimeColumns:      setOf("funding_time"),
		MinRetentionDays: 365,
	},
	"long_short_ratio_snapshots": {
		Name:             "long_short_ratio_snapshots",
		IDColumn:         "id",
		TimeColumns:      setOf("snapshot_time"),
		MinRetentionDays: 90,
	},
	"taker_flow_snapshots": {
		Name:               "taker_flow_snapshots",
		IDColumn:           "id",
		TimeColumns:        setOf("snapshot_time"),
		MinRetentionDays:   30,
		PartitionPreferred: true,
	},
	"cvd_snapshots": {
		Name:               "cvd_snapshots",
		IDColumn:           "id",
		TimeColumns:        setOf("snapshot_time"),
		MinRetentionDays:   30,
		PartitionPreferred: true,
	},
	"derivative_aggregated_snapshots": {
		Name:               "derivative_aggregated_snapshots",
		IDColumn:           "id",
		TimeColumns:        setOf("snapshot_time"),
		MinRetentionDays:   14,
		PartitionPreferred: true,
	},
	"liquidation_events": {
		Name:               "liquidation_events",
		IDColumn:           "id",
		TimeColumns:        setOf("event_time"),
		MinRetentionDays:   7,
		PartitionPreferred: true,
	},
	"liquidation_aggregates": {
		Name:             "liquidation_aggregates",
		IDColumn:         "id",
		TimeColumns:      setOf("bucket_time"),
		IntervalColumns:  setOf("period"),
		MinRetentionDays: 30,
	},
	"orderbook_imbalance_snapshots": {
		Name:               "orderbook_imbalance_snapshots",
		IDColumn:           "id",
		TimeColumns:        setOf("snapshot_time"),
		MinRetentionDays:   7,
		PartitionPreferred: true,
	},
	"orderbook_depth_snapshots": {
		Name:             "orderbook_depth_snapshots",
		IDColumn:         "id",
		TimeColumns:      setOf("snapshot_time"),
		MinRetentionDays: 1,
	},
	"exchange_request_logs": {
		Name:             "exchange_request_logs",
		IDColumn:         "id",
		TimeColumns:      setOf("captured_at"),
		MinRetentionDays: 7,
	},
	"raw_exchange_payloads": {
		Name:             "raw_exchange_payloads",
		IDColumn:         "id",
		TimeColumns:      setOf("captured_at", "retention_expires_at"),
		MinRetentionDays: 1,
	},
	"failed_collection_jobs": {
		Name:             "failed_collection_jobs",
		IDColumn:         "id",
		TimeColumns:      setOf("failed_at"),
		MinRetentionDays: 30,
	},
	"data_collection_runs": {
		Name:             "data_collection_runs",
		IDColumn:         "id",
		TimeColumns:      setOf("started_at"),
		MinRetentionDays: 30,
	},
	"data_quality_issues": {
		Name:             "data_quality_issues",
		IDColumn:         "id",
		TimeColumns:      setOf("last_seen_at"),
		MinRetentionDays: 90,
	},
}

func setOf(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func SpecFor(tableName string) (TableSpec, bool) {
	spec, ok := tableSpecs[strings.ToLower(strings.TrimSpace(tableName))]
	return spec, ok
}

func ValidatePolicy(policy Policy) (TableSpec, error) {
	tableName := strings.ToLower(strings.TrimSpace(policy.TableName))
	spec, ok := SpecFor(tableName)
	if !ok {
		return TableSpec{}, fmt.Errorf("retention policy table %q is not supported", policy.TableName)
	}
	if _, ok := spec.TimeColumns[policy.TimeColumn]; !ok {
		return TableSpec{}, fmt.Errorf("retention policy table %s cannot use time column %q", tableName, policy.TimeColumn)
	}
	if policy.IntervalColumn != "" {
		if _, ok := spec.IntervalColumns[policy.IntervalColumn]; !ok {
			return TableSpec{}, fmt.Errorf("retention policy table %s cannot use interval column %q", tableName, policy.IntervalColumn)
		}
		if strings.TrimSpace(policy.IntervalValue) == "" {
			return TableSpec{}, fmt.Errorf("retention policy table %s interval value is required", tableName)
		}
	}
	if policy.RetentionDays <= 0 {
		return TableSpec{}, fmt.Errorf("retention_days must be greater than 0 for %s", tableName)
	}

	minDays := spec.MinRetentionDays
	if policy.MinRetentionDays > minDays {
		minDays = policy.MinRetentionDays
	}
	if policy.RetentionDays < minDays {
		return TableSpec{}, fmt.Errorf("retention_days %d is below safety minimum %d for %s", policy.RetentionDays, minDays, tableName)
	}
	if policy.ChunkSize <= 0 {
		return TableSpec{}, fmt.Errorf("chunk_size must be greater than 0 for %s", tableName)
	}
	if policy.MaxRowsPerRun <= 0 {
		return TableSpec{}, fmt.Errorf("max_rows_per_run must be greater than 0 for %s", tableName)
	}
	if policy.TimeoutSeconds <= 0 {
		return TableSpec{}, fmt.Errorf("timeout_seconds must be greater than 0 for %s", tableName)
	}
	if policy.PartitionStrategy == "" {
		policy.PartitionStrategy = "auto"
	}

	return spec, nil
}
