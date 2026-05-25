package retention

import (
	"encoding/json"
	"time"
)

type Policy struct {
	ID                 int64           `json:"id"`
	TableName          string          `json:"table_name"`
	TimeColumn         string          `json:"time_column"`
	IntervalColumn     string          `json:"interval_column,omitempty"`
	IntervalValue      string          `json:"interval_value,omitempty"`
	RetentionDays      int             `json:"retention_days"`
	ChunkSize          int             `json:"chunk_size"`
	Enabled            bool            `json:"enabled"`
	DryRun             bool            `json:"dry_run"`
	RollupBeforeDelete bool            `json:"rollup_before_delete"`
	RollupTargetTable  string          `json:"rollup_target_table,omitempty"`
	Priority           int             `json:"priority"`
	MaxRowsPerRun      int             `json:"max_rows_per_run"`
	TimeoutSeconds     int             `json:"timeout_seconds"`
	PartitionStrategy  string          `json:"partition_strategy"`
	MinRetentionDays   int             `json:"min_retention_days"`
	Metadata           json.RawMessage `json:"metadata"`
}

type PlanItem struct {
	Policy               Policy
	CutoffTime           time.Time
	DryRun               bool
	UsePartitionDrop     bool
	RollupTargetInterval string
}

type CleanupResult struct {
	PolicyID          int64
	TableName         string
	IntervalValue     string
	DryRun            bool
	CutoffTime        time.Time
	RowsMatched       int64
	RowsDeleted       int64
	PartitionsMatched int
	PartitionsDropped int
	RollupRowsRead    int64
	RollupRowsWritten int64
	Status            string
	ErrorMessage      string
}

type Summary struct {
	StartedAt  time.Time
	FinishedAt time.Time
	Results    []CleanupResult
	Metrics    *Metrics
}

type RollupResult struct {
	PolicyID       int64
	SourceTable    string
	TargetTable    string
	SourceInterval string
	TargetInterval string
	WindowStart    time.Time
	WindowEnd      time.Time
	RowsRead       int64
	RowsWritten    int64
	DryRun         bool
}

type Partition struct {
	SchemaName string
	TableName  string
	RangeStart time.Time
	RangeEnd   time.Time
}
