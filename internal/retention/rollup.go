package retention

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type RollupEngine struct {
	Store *Store
	Now   func() time.Time
}

type rollupSpec struct {
	SourceInterval string
	TargetInterval string
	BucketSeconds  int
}

var klineRollups = map[string]rollupSpec{
	"1m":  {SourceInterval: "1m", TargetInterval: "5m", BucketSeconds: 5 * 60},
	"5m":  {SourceInterval: "5m", TargetInterval: "15m", BucketSeconds: 15 * 60},
	"15m": {SourceInterval: "15m", TargetInterval: "1h", BucketSeconds: 60 * 60},
	"1h":  {SourceInterval: "1h", TargetInterval: "4h", BucketSeconds: 4 * 60 * 60},
	"4h":  {SourceInterval: "4h", TargetInterval: "1d", BucketSeconds: 24 * 60 * 60},
}

func (r RollupEngine) RollupBeforeDelete(ctx context.Context, policy Policy, cutoff time.Time, dryRun bool) (RollupResult, error) {
	if policy.TableName != "derivative_klines" {
		return RollupResult{}, fmt.Errorf("rollup_before_delete is only supported for derivative_klines, got %s", policy.TableName)
	}
	spec, ok := klineRollups[policy.IntervalValue]
	if !ok {
		return RollupResult{}, fmt.Errorf("no kline rollup target for interval %q", policy.IntervalValue)
	}

	result := RollupResult{
		PolicyID:       policy.ID,
		SourceTable:    policy.TableName,
		TargetTable:    "derivative_klines",
		SourceInterval: spec.SourceInterval,
		TargetInterval: spec.TargetInterval,
		WindowEnd:      cutoff,
		DryRun:         dryRun,
	}

	if r.Store == nil {
		return result, fmt.Errorf("retention rollup store is required")
	}

	windowStart, rowsRead, err := r.Store.KlineRollupWindow(ctx, spec.SourceInterval, cutoff)
	if err != nil {
		return result, err
	}
	result.WindowStart = windowStart
	result.RowsRead = rowsRead
	if rowsRead == 0 || dryRun {
		return result, nil
	}

	runKey := RollupRunKey(policy, cutoff, spec.TargetInterval)
	metadata, _ := json.Marshal(map[string]any{
		"source_interval": spec.SourceInterval,
		"target_interval": spec.TargetInterval,
		"bucket_seconds":  spec.BucketSeconds,
	})
	if err := r.Store.StartRollupRun(ctx, runKey, policy, result, metadata); err != nil {
		return result, err
	}

	rowsWritten, err := r.Store.UpsertKlineRollup(ctx, spec, cutoff)
	if err != nil {
		_ = r.Store.FinishRollupRun(ctx, runKey, "failed", rowsRead, 0, err.Error(), metadata)
		return result, err
	}
	result.RowsWritten = rowsWritten

	if err := r.Store.FinishRollupRun(ctx, runKey, "succeeded", rowsRead, rowsWritten, "", metadata); err != nil {
		return result, err
	}

	return result, nil
}

func (s *Store) KlineRollupWindow(ctx context.Context, sourceInterval string, cutoff time.Time) (time.Time, int64, error) {
	var (
		windowStart time.Time
		rowsRead    int64
	)
	if err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(MIN(open_time), $2), COUNT(*)
		FROM derivative_klines
		WHERE "interval" = $1
		  AND open_time < $2
		  AND is_closed = true
	`, sourceInterval, cutoff).Scan(&windowStart, &rowsRead); err != nil {
		return time.Time{}, 0, fmt.Errorf("query kline rollup window: %w", err)
	}

	return windowStart, rowsRead, nil
}

func (s *Store) UpsertKlineRollup(ctx context.Context, spec rollupSpec, cutoff time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
		WITH eligible AS (
		    SELECT symbol_id, exchange, market_type, source_symbol,
		           to_timestamp(floor(extract(epoch FROM open_time) / $1) * $1) AS bucket_time,
		           open_time, close_time, open_price, high_price, low_price,
		           close_price, volume, quote_volume, trade_count,
		           taker_buy_volume, taker_buy_quote_volume
		    FROM derivative_klines
		    WHERE "interval" = $2
		      AND open_time < $3
		      AND is_closed = true
		),
		grouped AS (
		    SELECT symbol_id, exchange, market_type, source_symbol, bucket_time,
		           MAX(close_time) AS close_time,
		           (array_agg(open_price ORDER BY open_time ASC))[1] AS open_price,
		           MAX(high_price) AS high_price,
		           MIN(low_price) AS low_price,
		           (array_agg(close_price ORDER BY open_time DESC))[1] AS close_price,
		           SUM(volume) AS volume,
		           SUM(quote_volume) AS quote_volume,
		           SUM(trade_count) AS trade_count,
		           SUM(taker_buy_volume) AS taker_buy_volume,
		           SUM(taker_buy_quote_volume) AS taker_buy_quote_volume,
		           COUNT(*) AS source_rows
		    FROM eligible
		    GROUP BY symbol_id, exchange, market_type, source_symbol, bucket_time
		)
		INSERT INTO derivative_klines (
		    symbol_id, exchange, market_type, source_symbol, "interval",
		    open_time, close_time, open_price, high_price, low_price,
		    close_price, volume, quote_volume, trade_count,
		    taker_buy_volume, taker_buy_quote_volume, is_closed, raw_data
		)
		SELECT symbol_id, exchange, market_type, source_symbol, $4,
		       bucket_time, close_time, open_price, high_price, low_price,
		       close_price, volume, quote_volume, trade_count,
		       taker_buy_volume, taker_buy_quote_volume, true,
		       jsonb_build_object(
		           'rollup', true,
		           'source_interval', $2,
		           'target_interval', $4,
		           'source_rows', source_rows
		       )
		FROM grouped
		ON CONFLICT (symbol_id, exchange, "interval", open_time) DO UPDATE SET
		    close_time = EXCLUDED.close_time,
		    high_price = EXCLUDED.high_price,
		    low_price = EXCLUDED.low_price,
		    close_price = EXCLUDED.close_price,
		    volume = EXCLUDED.volume,
		    quote_volume = EXCLUDED.quote_volume,
		    trade_count = EXCLUDED.trade_count,
		    taker_buy_volume = EXCLUDED.taker_buy_volume,
		    taker_buy_quote_volume = EXCLUDED.taker_buy_quote_volume,
		    is_closed = true,
		    raw_data = EXCLUDED.raw_data
	`, spec.BucketSeconds, spec.SourceInterval, cutoff, spec.TargetInterval)
	if err != nil {
		return 0, fmt.Errorf("upsert kline rollup %s to %s: %w", spec.SourceInterval, spec.TargetInterval, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("kline rollup rows affected: %w", err)
	}

	return rows, nil
}

func (s *Store) StartRollupRun(ctx context.Context, runKey string, policy Policy, result RollupResult, metadata json.RawMessage) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO data_rollup_runs (
		    policy_id, run_key, source_table, target_table, period,
		    status, window_start, window_end, rows_read, rows_written, metadata
		)
		VALUES ($1, $2, $3, $4, $5, 'running', $6, $7, 0, 0, $8)
		ON CONFLICT (run_key) DO UPDATE SET
		    status = 'running',
		    window_start = EXCLUDED.window_start,
		    window_end = EXCLUDED.window_end,
		    started_at = now(),
		    finished_at = NULL,
		    error_message = NULL,
		    metadata = EXCLUDED.metadata
	`, policy.ID, runKey, result.SourceTable, result.TargetTable, result.SourceInterval, result.WindowStart, result.WindowEnd, ensureJSON(metadata))
	if err != nil {
		return fmt.Errorf("start rollup run: %w", err)
	}

	return nil
}

func (s *Store) FinishRollupRun(ctx context.Context, runKey string, status string, rowsRead int64, rowsWritten int64, errorMessage string, metadata json.RawMessage) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE data_rollup_runs
		SET status = $2,
		    finished_at = now(),
		    rows_read = $3,
		    rows_written = $4,
		    error_message = NULLIF($5, ''),
		    metadata = $6
		WHERE run_key = $1
	`, runKey, status, rowsRead, rowsWritten, errorMessage, ensureJSON(metadata))
	if err != nil {
		return fmt.Errorf("finish rollup run: %w", err)
	}

	return nil
}

func klineRollupTarget(sourceInterval string) string {
	spec, ok := klineRollups[sourceInterval]
	if !ok {
		return ""
	}
	return spec.TargetInterval
}
