package retention

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const (
	cleanupStatusSucceeded = "succeeded"
	cleanupStatusFailed    = "failed"
)

type EngineOptions struct {
	MaxRowsPerRun        int
	TableTimeout         time.Duration
	DiskPressureCritical bool
}

type Engine struct {
	Store   *Store
	Rollups RollupEngine
	Metrics *Metrics
	Now     func() time.Time
	Options EngineOptions
}

func (e *Engine) RunOnce(ctx context.Context) (Summary, error) {
	if e.Store == nil {
		return Summary{}, fmt.Errorf("retention store is required")
	}

	startedAt := e.now()
	metrics := e.Metrics
	if metrics == nil {
		metrics = NewMetrics()
	}

	policies, err := e.Store.ListEnabledPolicies(ctx)
	if err != nil {
		return Summary{}, err
	}
	plan, err := Planner{Now: e.Now}.Plan(policies)
	if err != nil {
		return Summary{}, err
	}

	summary := Summary{
		StartedAt: startedAt,
		Metrics:   metrics,
	}
	var firstErr error
	for _, item := range plan {
		result, err := e.RunPlanItem(ctx, item)
		summary.Results = append(summary.Results, result)
		metrics.Observe(result)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	summary.FinishedAt = e.now()

	return summary, firstErr
}

func (e *Engine) RunPlanItem(ctx context.Context, item PlanItem) (CleanupResult, error) {
	policy := item.Policy
	if _, err := ValidatePolicy(policy); err != nil {
		return CleanupResult{}, err
	}
	if e.Store == nil {
		return CleanupResult{}, fmt.Errorf("retention store is required")
	}

	timeout := time.Duration(policy.TimeoutSeconds) * time.Second
	if e.Options.TableTimeout > 0 && e.Options.TableTimeout < timeout {
		timeout = e.Options.TableTimeout
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	runKey := CleanupRunKey(policy, item.CutoffTime)
	result := CleanupResult{
		PolicyID:      policy.ID,
		TableName:     policy.TableName,
		IntervalValue: policy.IntervalValue,
		DryRun:        item.DryRun,
		CutoffTime:    item.CutoffTime,
		Status:        cleanupStatusSucceeded,
	}
	metadata := cleanupMetadata(item, "started", nil)
	if err := e.Store.StartCleanupRun(ctx, runKey, policy, item.CutoffTime, item.DryRun, metadata); err != nil {
		return result, err
	}

	fail := func(err error) (CleanupResult, error) {
		result.Status = cleanupStatusFailed
		result.ErrorMessage = err.Error()
		_ = e.Store.FinishCleanupRun(ctx, runKey, cleanupStatusFailed, result.RowsMatched, result.RowsDeleted, err.Error(), cleanupMetadata(item, "failed", result))
		return result, err
	}

	if e.Options.DiskPressureCritical {
		return fail(fmt.Errorf("cleanup stopped: disk pressure is critical"))
	}

	rowsMatched, err := e.Store.CountEligible(ctx, policy, item.CutoffTime)
	if err != nil {
		return fail(err)
	}
	result.RowsMatched = rowsMatched

	if policy.RollupBeforeDelete {
		rollup, err := e.Rollups.RollupBeforeDelete(ctx, policy, item.CutoffTime, item.DryRun)
		if err != nil {
			return fail(fmt.Errorf("rollup_before_delete failed: %w", err))
		}
		result.RollupRowsRead = rollup.RowsRead
		result.RollupRowsWritten = rollup.RowsWritten
	}

	if item.UsePartitionDrop {
		partitions, err := e.Store.ListEligiblePartitions(ctx, policy, item.CutoffTime)
		if err != nil {
			return fail(err)
		}
		result.PartitionsMatched = len(partitions)
		if len(partitions) > 0 && !item.DryRun {
			for _, partition := range partitions {
				if err := e.Store.DropPartition(ctx, partition); err != nil {
					return fail(err)
				}
				result.PartitionsDropped++
			}
			if err := e.Store.FinishCleanupRun(ctx, runKey, cleanupStatusSucceeded, result.RowsMatched, result.RowsDeleted, "", cleanupMetadata(item, "succeeded", result)); err != nil {
				return result, err
			}
			return result, nil
		}
	}

	if item.DryRun {
		if err := e.Store.FinishCleanupRun(ctx, runKey, cleanupStatusSucceeded, result.RowsMatched, 0, "", cleanupMetadata(item, "dry_run", result)); err != nil {
			return result, err
		}
		return result, nil
	}

	maxRows := effectiveMaxRows(policy, e.Options.MaxRowsPerRun)
	for result.RowsDeleted < maxRows {
		chunkSize := policy.ChunkSize
		remaining := maxRows - result.RowsDeleted
		if remaining < int64(chunkSize) {
			chunkSize = int(remaining)
		}
		if chunkSize <= 0 {
			break
		}

		deleted, err := e.Store.DeleteChunk(ctx, policy, item.CutoffTime, chunkSize)
		if err != nil {
			return fail(err)
		}
		result.RowsDeleted += deleted
		if deleted == 0 || deleted < int64(chunkSize) {
			break
		}
	}

	if err := e.Store.FinishCleanupRun(ctx, runKey, cleanupStatusSucceeded, result.RowsMatched, result.RowsDeleted, "", cleanupMetadata(item, "succeeded", result)); err != nil {
		return result, err
	}

	return result, nil
}

func CleanupRunKey(policy Policy, cutoff time.Time) string {
	interval := policy.IntervalValue
	if interval == "" {
		interval = "all"
	}
	return fmt.Sprintf("cleanup:%d:%s:%s:%s", policy.ID, policy.TableName, interval, cutoff.UTC().Format("20060102T150405Z"))
}

func RollupRunKey(policy Policy, cutoff time.Time, targetInterval string) string {
	return fmt.Sprintf("rollup:%d:%s:%s:%s:%s", policy.ID, policy.TableName, policy.IntervalValue, targetInterval, cutoff.UTC().Format("20060102T150405Z"))
}

func cleanupMetadata(item PlanItem, state string, result any) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{
		"state":                  state,
		"partition_drop":         item.UsePartitionDrop,
		"rollup_target_interval": item.RollupTargetInterval,
		"dry_run":                item.DryRun,
		"result":                 result,
	})
	return raw
}

func effectiveMaxRows(policy Policy, optionMax int) int64 {
	maxRows := policy.MaxRowsPerRun
	if optionMax > 0 && optionMax < maxRows {
		maxRows = optionMax
	}
	if maxRows <= 0 {
		maxRows = policy.ChunkSize
	}
	return int64(maxRows)
}

func (e *Engine) now() time.Time {
	if e.Now != nil {
		return e.Now().UTC()
	}
	return time.Now().UTC()
}
