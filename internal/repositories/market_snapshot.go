package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"aggregator-services/internal/normalizers"
)

type MarketSnapshotRepository struct {
	db *sql.DB
}

func NewMarketSnapshotRepository(db *sql.DB) *MarketSnapshotRepository {
	return &MarketSnapshotRepository{db: db}
}

func (r *MarketSnapshotRepository) Upsert(ctx context.Context, snapshots []normalizers.NormalizedMarketSnapshot) (int, error) {
	if len(snapshots) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin market snapshot upsert: %w", err)
	}
	defer tx.Rollback()

	count := 0
	for _, snapshot := range snapshots {
		if err := normalizers.ValidateMarketSnapshot(snapshot); err != nil {
			return 0, err
		}

		result, err := tx.ExecContext(ctx, `
			INSERT INTO derivative_market_snapshots (
			    symbol_id, exchange, market_type, source_symbol, snapshot_time,
			    last_price, mark_price, index_price, bid_price, ask_price,
			    volume_24h, quote_volume_24h, price_change_percent_24h,
			    open_interest, funding_rate, raw_data
			)
			VALUES ($1, $2, '', $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
			ON CONFLICT (symbol_id, exchange, snapshot_time) DO UPDATE SET
			    source_symbol = EXCLUDED.source_symbol,
			    last_price = EXCLUDED.last_price,
			    mark_price = EXCLUDED.mark_price,
			    index_price = EXCLUDED.index_price,
			    bid_price = EXCLUDED.bid_price,
			    ask_price = EXCLUDED.ask_price,
			    volume_24h = EXCLUDED.volume_24h,
			    quote_volume_24h = EXCLUDED.quote_volume_24h,
			    price_change_percent_24h = EXCLUDED.price_change_percent_24h,
			    open_interest = EXCLUDED.open_interest,
			    funding_rate = EXCLUDED.funding_rate,
			    raw_data = EXCLUDED.raw_data
		`,
			snapshot.SymbolID,
			snapshot.Exchange,
			snapshot.SourceSymbol,
			snapshot.SnapshotTime,
			nullableFloat(snapshot.LastPrice),
			nullableFloat(snapshot.MarkPrice),
			nullableFloat(snapshot.IndexPrice),
			nullableFloat(snapshot.BidPrice),
			nullableFloat(snapshot.AskPrice),
			nullableFloat(snapshot.Volume24h),
			nullableFloat(snapshot.QuoteVolume24h),
			nullableFloat(snapshot.PriceChangePercent24h),
			nullableFloat(snapshot.OpenInterest),
			nullableFloat(snapshot.FundingRate),
			ensureJSON(snapshot.RawData),
		)
		if err != nil {
			return 0, fmt.Errorf("upsert market snapshot: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		count += int(affected)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit market snapshot upsert: %w", err)
	}

	return count, nil
}
