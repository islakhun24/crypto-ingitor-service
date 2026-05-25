package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"aggregator-services/internal/normalizers"
)

type KlineRepository struct {
	db *sql.DB
}

func NewKlineRepository(db *sql.DB) *KlineRepository {
	return &KlineRepository{db: db}
}

func (r *KlineRepository) Upsert(ctx context.Context, klines []normalizers.NormalizedKline) (int, error) {
	if len(klines) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin kline upsert: %w", err)
	}
	defer tx.Rollback()

	count := 0
	for _, kline := range klines {
		if err := normalizers.ValidateKline(kline); err != nil {
			return 0, err
		}

		result, err := tx.ExecContext(ctx, `
			INSERT INTO derivative_klines (
			    symbol_id, exchange, market_type, source_symbol, "interval",
			    open_time, close_time, open_price, high_price, low_price,
			    close_price, volume, quote_volume, trade_count,
			    taker_buy_volume, taker_buy_quote_volume, is_closed, raw_data
			)
			VALUES ($1, $2, '', $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
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
			    is_closed = EXCLUDED.is_closed,
			    raw_data = EXCLUDED.raw_data
		`,
			kline.SymbolID,
			kline.Exchange,
			kline.SourceSymbol,
			kline.Interval,
			kline.OpenTime,
			kline.CloseTime,
			kline.OpenPrice,
			kline.HighPrice,
			kline.LowPrice,
			kline.ClosePrice,
			nullableFloat(kline.Volume),
			nullableFloat(kline.QuoteVolume),
			nullableInt64(kline.TradeCount),
			nullableFloat(kline.TakerBuyVolume),
			nullableFloat(kline.TakerBuyQuoteVolume),
			kline.IsClosed,
			ensureJSON(kline.RawData),
		)
		if err != nil {
			return 0, fmt.Errorf("upsert kline: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		count += int(affected)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit kline upsert: %w", err)
	}

	return count, nil
}
