package repositories

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type DataGap struct {
	SymbolID                int64
	Exchange                string
	DataType                string
	Period                  string
	GapStart                time.Time
	GapEnd                  time.Time
	ExpectedIntervalSeconds int
	LastObservedAt          time.Time
	Metadata                json.RawMessage
}

type DataGapRepository struct {
	db interface {
		ExecContext(context.Context, string, ...any) (sql.Result, error)
	}
}

func NewDataGapRepository(db interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}) *DataGapRepository {
	return &DataGapRepository{db: db}
}

func (r *DataGapRepository) Upsert(ctx context.Context, gap DataGap) error {
	gapKey := DataGapKey(gap)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO data_gaps (
		    gap_key, symbol_id, exchange, data_type, period, gap_start,
		    gap_end, expected_interval_seconds, last_observed_at, metadata
		)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7, NULLIF($8, 0), $9, $10)
		ON CONFLICT (gap_key) DO UPDATE SET
		    gap_end = EXCLUDED.gap_end,
		    detected_at = now(),
		    expected_interval_seconds = EXCLUDED.expected_interval_seconds,
		    last_observed_at = EXCLUDED.last_observed_at,
		    metadata = EXCLUDED.metadata
	`, gapKey, gap.SymbolID, gap.Exchange, gap.DataType, gap.Period, gap.GapStart, gap.GapEnd, gap.ExpectedIntervalSeconds, nullableTime(gap.LastObservedAt), ensureJSON(gap.Metadata))
	if err != nil {
		return fmt.Errorf("upsert data gap: %w", err)
	}

	return nil
}

func (r *DataGapRepository) InsertKlineGaps(ctx context.Context, interval string, expectedInterval time.Duration, lookback time.Duration, now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `
		WITH ordered AS (
		    SELECT symbol_id, exchange, source_symbol, open_time,
		           LAG(open_time) OVER (PARTITION BY symbol_id, exchange ORDER BY open_time) AS previous_time
		    FROM derivative_klines
		    WHERE "interval" = $1
		      AND open_time >= $2
		      AND open_time <= $3
		),
		gaps AS (
		    SELECT symbol_id, exchange, source_symbol, previous_time, open_time
		    FROM ordered
		    WHERE previous_time IS NOT NULL
		      AND open_time - previous_time > ($4::int * interval '1 second' * 2)
		)
		INSERT INTO data_gaps (
		    gap_key, symbol_id, exchange, data_type, period, gap_start,
		    gap_end, expected_interval_seconds, last_observed_at, metadata
		)
		SELECT md5(symbol_id::text || exchange || 'kline' || $1 || previous_time::text || open_time::text),
		       symbol_id, exchange, 'kline', $1, previous_time, open_time,
		       $4, previous_time,
		       jsonb_build_object('source_symbol', source_symbol, 'detector', 'kline_gap')
		FROM gaps
		ON CONFLICT (gap_key) DO UPDATE SET
		    gap_end = EXCLUDED.gap_end,
		    detected_at = now(),
		    expected_interval_seconds = EXCLUDED.expected_interval_seconds,
		    last_observed_at = EXCLUDED.last_observed_at,
		    metadata = EXCLUDED.metadata
	`, interval, now.Add(-lookback), now, int(expectedInterval.Seconds()))
	if err != nil {
		return fmt.Errorf("insert kline gaps: %w", err)
	}

	return nil
}

func (r *DataGapRepository) InsertStaleLatestGaps(ctx context.Context, dataType string, expectedInterval time.Duration, now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	spec, ok := staleSpec(dataType)
	if !ok {
		return fmt.Errorf("unsupported stale data type %q", dataType)
	}
	staleBefore := now.Add(-2 * expectedInterval)
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(`
		WITH latest AS (
		    SELECT DISTINCT ON (symbol_id, exchange)
		           symbol_id, exchange, source_symbol, %s AS observed_at
		    FROM %s
		    ORDER BY symbol_id, exchange, %s DESC
		),
		stale AS (
		    SELECT *
		    FROM latest
		    WHERE observed_at < $1
		)
		INSERT INTO data_gaps (
		    gap_key, symbol_id, exchange, data_type, period, gap_start,
		    gap_end, expected_interval_seconds, last_observed_at, metadata
		)
		SELECT md5(symbol_id::text || exchange || $2 || observed_at::text || $3::text),
		       symbol_id, exchange, $2, NULL, observed_at, $3,
		       $4, observed_at,
		       jsonb_build_object('source_symbol', source_symbol, 'detector', 'stale_latest')
		FROM stale
		ON CONFLICT (gap_key) DO UPDATE SET
		    gap_end = EXCLUDED.gap_end,
		    detected_at = now(),
		    expected_interval_seconds = EXCLUDED.expected_interval_seconds,
		    last_observed_at = EXCLUDED.last_observed_at,
		    metadata = EXCLUDED.metadata
	`, spec.TimeColumn, spec.TableName, spec.TimeColumn), staleBefore, dataType, now, int(expectedInterval.Seconds()))
	if err != nil {
		return fmt.Errorf("insert stale latest gaps: %w", err)
	}

	return nil
}

func DataGapKey(gap DataGap) string {
	seed := fmt.Sprintf("%d|%s|%s|%s|%s|%s", gap.SymbolID, gap.Exchange, gap.DataType, gap.Period, gap.GapStart.UTC().Format(time.RFC3339), gap.GapEnd.UTC().Format(time.RFC3339))
	sum := sha1.Sum([]byte(seed))
	return hex.EncodeToString(sum[:])
}

type latestStaleSpec struct {
	TableName  string
	TimeColumn string
}

func staleSpec(dataType string) (latestStaleSpec, bool) {
	switch dataType {
	case "ticker", "market_snapshot":
		return latestStaleSpec{TableName: "derivative_market_snapshots", TimeColumn: "snapshot_time"}, true
	case "open_interest":
		return latestStaleSpec{TableName: "open_interest_snapshots", TimeColumn: "snapshot_time"}, true
	case "funding":
		return latestStaleSpec{TableName: "funding_rate_snapshots", TimeColumn: "snapshot_time"}, true
	case "aggregated_snapshot":
		return latestStaleSpec{TableName: "derivative_aggregated_snapshots", TimeColumn: "snapshot_time"}, true
	default:
		return latestStaleSpec{}, false
	}
}
