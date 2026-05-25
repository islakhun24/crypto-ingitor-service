package scheduler

import (
	"encoding/json"
	"fmt"
	"time"
)

const JobModeBackfill = "backfill"

type BackfillRequest struct {
	Exchange              string
	DataType              string
	Tier                  string
	SymbolID              int64
	SourceSymbol          string
	Period                string
	Start                 time.Time
	End                   time.Time
	IntervalSeconds       int
	Priority              int
	MaxRetry              int
	EndpointMetadata      RuntimeMetadata
	ParentGapID           int64
	MaxJobs               int
	RealtimeLag           time.Duration
	RealtimeLagPauseAfter time.Duration
}

func PlanBackfillJobs(req BackfillRequest) ([]Job, error) {
	if req.RealtimeLagPauseAfter > 0 && req.RealtimeLag > req.RealtimeLagPauseAfter {
		return nil, nil
	}
	if req.IntervalSeconds <= 0 {
		return nil, fmt.Errorf("interval_seconds must be greater than 0")
	}
	if req.Start.IsZero() || req.End.IsZero() || !req.Start.Before(req.End) {
		return nil, fmt.Errorf("valid backfill start/end are required")
	}
	if req.SymbolID <= 0 || req.SourceSymbol == "" {
		return nil, fmt.Errorf("symbol_id and source_symbol are required")
	}
	if req.Priority <= 0 {
		req.Priority = 1000
	}
	if req.MaxRetry <= 0 {
		req.MaxRetry = 3
	}
	if req.MaxJobs <= 0 {
		req.MaxJobs = 1000
	}

	var jobs []Job
	step := time.Duration(req.IntervalSeconds) * time.Second
	for bucket := ScheduledBucket(req.Start, req.IntervalSeconds); bucket.Before(req.End); bucket = bucket.Add(step) {
		if bucket.Before(req.Start) {
			continue
		}
		if len(jobs) >= req.MaxJobs {
			break
		}
		checkpoint, metadata := backfillMetadata(req, bucket)
		jobs = append(jobs, Job{
			Exchange:           req.Exchange,
			DataType:           req.DataType,
			Tier:               req.Tier,
			SymbolID:           req.SymbolID,
			SourceSymbol:       req.SourceSymbol,
			Period:             req.Period,
			IdempotencyKey:     IdempotencyKey(req.Exchange, req.DataType, req.SymbolID, req.SourceSymbol, req.Period, bucket),
			Status:             JobStatusPending,
			Priority:           req.Priority,
			ScheduledAt:        bucket,
			RetryCount:         0,
			MaxRetry:           req.MaxRetry,
			Metadata:           metadata,
			JobMode:            JobModeBackfill,
			ParentGapID:        req.ParentGapID,
			BackfillCheckpoint: checkpoint,
		})
	}

	return jobs, nil
}

func backfillMetadata(req BackfillRequest, bucket time.Time) (json.RawMessage, json.RawMessage) {
	checkpointMap := map[string]any{
		"bucket":        bucket.UTC().Format(time.RFC3339),
		"start_time":    req.Start.UTC().Format(time.RFC3339),
		"end_time":      req.End.UTC().Format(time.RFC3339),
		"parent_gap_id": req.ParentGapID,
	}
	meta := map[string]any{
		"mode":               JobModeBackfill,
		"requires_endpoint":  req.EndpointMetadata.RequiresEndpoint,
		"endpoint_id":        req.EndpointMetadata.EndpointID,
		"endpoint_name":      req.EndpointMetadata.EndpointName,
		"endpoint_data_type": req.EndpointMetadata.EndpointDataType,
		"request_weight":     req.EndpointMetadata.RequestWeight,
		"timeout_ms":         req.EndpointMetadata.TimeoutMS,
		"backfill":           checkpointMap,
		"start_time":         bucket.UTC().Format(time.RFC3339),
		"end_time":           bucket.Add(time.Duration(req.IntervalSeconds) * time.Second).UTC().Format(time.RFC3339),
	}
	checkpoint, _ := json.Marshal(checkpointMap)
	metadata, _ := json.Marshal(meta)
	return checkpoint, metadata
}
