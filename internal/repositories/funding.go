package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"aggregator-services/internal/normalizers"
)

type FundingRepository struct {
	db *sql.DB
}

func NewFundingRepository(db *sql.DB) *FundingRepository {
	return &FundingRepository{db: db}
}

func (r *FundingRepository) UpsertSnapshots(ctx context.Context, items []normalizers.NormalizedFundingSnapshot) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin funding snapshot upsert: %w", err)
	}
	defer tx.Rollback()

	count := 0
	for _, item := range items {
		if err := normalizers.ValidateFundingSnapshot(item); err != nil {
			return 0, err
		}

		result, err := tx.ExecContext(ctx, `
			INSERT INTO funding_rate_snapshots (
			    symbol_id, exchange, market_type, source_symbol, snapshot_time,
			    funding_rate, next_funding_time, mark_price, index_price, raw_data
			)
			VALUES ($1, $2, '', $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (symbol_id, exchange, snapshot_time) DO UPDATE SET
			    source_symbol = EXCLUDED.source_symbol,
			    funding_rate = EXCLUDED.funding_rate,
			    next_funding_time = EXCLUDED.next_funding_time,
			    mark_price = EXCLUDED.mark_price,
			    index_price = EXCLUDED.index_price,
			    raw_data = EXCLUDED.raw_data
		`,
			item.SymbolID,
			item.Exchange,
			item.SourceSymbol,
			item.SnapshotTime,
			item.FundingRate,
			nullableTime(item.NextFundingTime),
			nullableFloat(item.MarkPrice),
			nullableFloat(item.IndexPrice),
			ensureJSON(item.RawData),
		)
		if err != nil {
			return 0, fmt.Errorf("upsert funding snapshot: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		count += int(affected)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit funding snapshot upsert: %w", err)
	}

	return count, nil
}

func (r *FundingRepository) UpsertHistory(ctx context.Context, items []normalizers.NormalizedFundingHistory) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin funding history upsert: %w", err)
	}
	defer tx.Rollback()

	count := 0
	for _, item := range items {
		if err := normalizers.ValidateFundingHistory(item); err != nil {
			return 0, err
		}

		result, err := tx.ExecContext(ctx, `
			INSERT INTO funding_rate_history (
			    symbol_id, exchange, market_type, source_symbol, funding_time,
			    funding_rate, realized_rate, mark_price, raw_data
			)
			VALUES ($1, $2, '', $3, $4, $5, $6, $7, $8)
			ON CONFLICT (symbol_id, exchange, funding_time) DO UPDATE SET
			    source_symbol = EXCLUDED.source_symbol,
			    funding_rate = EXCLUDED.funding_rate,
			    realized_rate = EXCLUDED.realized_rate,
			    mark_price = EXCLUDED.mark_price,
			    raw_data = EXCLUDED.raw_data
		`,
			item.SymbolID,
			item.Exchange,
			item.SourceSymbol,
			item.FundingTime,
			item.FundingRate,
			nullableFloat(item.RealizedRate),
			nullableFloat(item.MarkPrice),
			ensureJSON(item.RawData),
		)
		if err != nil {
			return 0, fmt.Errorf("upsert funding history: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		count += int(affected)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit funding history upsert: %w", err)
	}

	return count, nil
}
