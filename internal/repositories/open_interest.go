package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"aggregator-services/internal/normalizers"
)

type OpenInterestRepository struct {
	db *sql.DB
}

func NewOpenInterestRepository(db *sql.DB) *OpenInterestRepository {
	return &OpenInterestRepository{db: db}
}

func (r *OpenInterestRepository) UpsertSnapshots(ctx context.Context, items []normalizers.NormalizedOpenInterest) (int, error) {
	return r.upsert(ctx, items, false)
}

func (r *OpenInterestRepository) UpsertHistory(ctx context.Context, items []normalizers.NormalizedOpenInterest) (int, error) {
	return r.upsert(ctx, items, true)
}

func (r *OpenInterestRepository) upsert(ctx context.Context, items []normalizers.NormalizedOpenInterest, history bool) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin open interest upsert: %w", err)
	}
	defer tx.Rollback()

	count := 0
	for _, item := range items {
		if err := normalizers.ValidateOpenInterest(item, history); err != nil {
			return 0, err
		}

		var result sql.Result
		if history {
			result, err = tx.ExecContext(ctx, `
				INSERT INTO open_interest_history (
				    symbol_id, exchange, market_type, source_symbol, period,
				    "timestamp", open_interest, open_interest_value, raw_data
				)
				VALUES ($1, $2, '', $3, $4, $5, $6, $7, $8)
				ON CONFLICT (symbol_id, exchange, period, "timestamp") DO UPDATE SET
				    source_symbol = EXCLUDED.source_symbol,
				    open_interest = EXCLUDED.open_interest,
				    open_interest_value = EXCLUDED.open_interest_value,
				    raw_data = EXCLUDED.raw_data
			`,
				item.SymbolID,
				item.Exchange,
				item.SourceSymbol,
				item.Period,
				item.SnapshotTime,
				item.OpenInterest,
				nullableFloat(item.OpenInterestValue),
				ensureJSON(item.RawData),
			)
		} else {
			result, err = tx.ExecContext(ctx, `
				INSERT INTO open_interest_snapshots (
				    symbol_id, exchange, market_type, source_symbol,
				    snapshot_time, open_interest, open_interest_value, raw_data
				)
				VALUES ($1, $2, '', $3, $4, $5, $6, $7)
				ON CONFLICT (symbol_id, exchange, snapshot_time) DO UPDATE SET
				    source_symbol = EXCLUDED.source_symbol,
				    open_interest = EXCLUDED.open_interest,
				    open_interest_value = EXCLUDED.open_interest_value,
				    raw_data = EXCLUDED.raw_data
			`,
				item.SymbolID,
				item.Exchange,
				item.SourceSymbol,
				item.SnapshotTime,
				item.OpenInterest,
				nullableFloat(item.OpenInterestValue),
				ensureJSON(item.RawData),
			)
		}
		if err != nil {
			return 0, fmt.Errorf("upsert open interest: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		count += int(affected)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit open interest upsert: %w", err)
	}

	return count, nil
}
