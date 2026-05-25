package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"aggregator-services/internal/normalizers"
)

type AdvancedRepository struct {
	db                *sql.DB
	LiquidationMinUSD float64
}

func NewAdvancedRepository(db *sql.DB, liquidationMinUSD float64) *AdvancedRepository {
	if liquidationMinUSD <= 0 {
		liquidationMinUSD = 1000
	}

	return &AdvancedRepository{db: db, LiquidationMinUSD: liquidationMinUSD}
}

func (r *AdvancedRepository) UpsertLongShortRatios(ctx context.Context, items []normalizers.NormalizedLongShortRatio) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin long/short upsert: %w", err)
	}
	defer tx.Rollback()

	count := 0
	for _, item := range items {
		if err := normalizers.ValidateLongShortRatio(item); err != nil {
			return 0, err
		}

		result, err := tx.ExecContext(ctx, `
			INSERT INTO long_short_ratio_snapshots (
			    symbol_id, exchange, market_type, source_symbol, period, snapshot_time,
			    long_account_ratio, short_account_ratio, long_short_ratio,
			    long_ratio, short_ratio, long_position_ratio, short_position_ratio,
			    top_trader_long_ratio, top_trader_short_ratio, raw_data
			)
			VALUES ($1, $2, '', $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
			ON CONFLICT (symbol_id, exchange, period, snapshot_time) DO UPDATE SET
			    source_symbol = EXCLUDED.source_symbol,
			    long_account_ratio = EXCLUDED.long_account_ratio,
			    short_account_ratio = EXCLUDED.short_account_ratio,
			    long_short_ratio = EXCLUDED.long_short_ratio,
			    long_ratio = EXCLUDED.long_ratio,
			    short_ratio = EXCLUDED.short_ratio,
			    long_position_ratio = EXCLUDED.long_position_ratio,
			    short_position_ratio = EXCLUDED.short_position_ratio,
			    top_trader_long_ratio = EXCLUDED.top_trader_long_ratio,
			    top_trader_short_ratio = EXCLUDED.top_trader_short_ratio,
			    raw_data = EXCLUDED.raw_data
		`,
			item.SymbolID,
			item.Exchange,
			item.SourceSymbol,
			item.Period,
			item.SnapshotTime,
			nullableFloat(item.LongAccountRatio),
			nullableFloat(item.ShortAccountRatio),
			nullableFloat(item.LongShortRatio),
			nullableFloat(item.LongRatio),
			nullableFloat(item.ShortRatio),
			nullableFloat(item.LongPositionRatio),
			nullableFloat(item.ShortPositionRatio),
			nullableFloat(item.TopTraderLongRatio),
			nullableFloat(item.TopTraderShortRatio),
			ensureJSON(item.RawData),
		)
		if err != nil {
			return 0, fmt.Errorf("upsert long/short ratio: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		count += int(affected)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit long/short upsert: %w", err)
	}

	return count, nil
}

func (r *AdvancedRepository) UpsertTakerFlows(ctx context.Context, items []normalizers.NormalizedTakerFlow) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin taker flow upsert: %w", err)
	}
	defer tx.Rollback()

	count := 0
	for _, item := range items {
		if err := normalizers.ValidateTakerFlow(item); err != nil {
			return 0, err
		}
		item = fillTakerFlowDeltas(item)

		result, err := tx.ExecContext(ctx, `
			INSERT INTO taker_flow_snapshots (
			    symbol_id, exchange, market_type, source_symbol, period, snapshot_time,
			    buy_volume, sell_volume, buy_quote_volume, sell_quote_volume,
			    buy_sell_ratio, net_quote_flow, taker_buy_volume, taker_sell_volume,
			    taker_buy_quote_volume, taker_sell_quote_volume, buy_sell_delta,
			    buy_sell_delta_quote, raw_data
			)
			VALUES ($1, $2, '', $3, $4, $5, $6, $7, $8, $9, $10, $11, $6, $7, $8, $9, $12, $13, $14)
			ON CONFLICT (symbol_id, exchange, period, snapshot_time) DO UPDATE SET
			    source_symbol = EXCLUDED.source_symbol,
			    buy_volume = EXCLUDED.buy_volume,
			    sell_volume = EXCLUDED.sell_volume,
			    buy_quote_volume = EXCLUDED.buy_quote_volume,
			    sell_quote_volume = EXCLUDED.sell_quote_volume,
			    buy_sell_ratio = EXCLUDED.buy_sell_ratio,
			    net_quote_flow = EXCLUDED.net_quote_flow,
			    taker_buy_volume = EXCLUDED.taker_buy_volume,
			    taker_sell_volume = EXCLUDED.taker_sell_volume,
			    taker_buy_quote_volume = EXCLUDED.taker_buy_quote_volume,
			    taker_sell_quote_volume = EXCLUDED.taker_sell_quote_volume,
			    buy_sell_delta = EXCLUDED.buy_sell_delta,
			    buy_sell_delta_quote = EXCLUDED.buy_sell_delta_quote,
			    raw_data = EXCLUDED.raw_data
		`,
			item.SymbolID,
			item.Exchange,
			item.SourceSymbol,
			item.Period,
			item.SnapshotTime,
			nullableFloat(item.TakerBuyVolume),
			nullableFloat(item.TakerSellVolume),
			nullableFloat(item.TakerBuyQuoteVolume),
			nullableFloat(item.TakerSellQuoteVolume),
			nullableFloat(item.BuySellRatio),
			nullableFloat(item.BuySellDeltaQuote),
			nullableFloat(item.BuySellDelta),
			nullableFloat(item.BuySellDeltaQuote),
			ensureJSON(item.RawData),
		)
		if err != nil {
			return 0, fmt.Errorf("upsert taker flow: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		count += int(affected)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit taker flow upsert: %w", err)
	}

	return count, nil
}

func (r *AdvancedRepository) UpsertCVDFromTakerFlows(ctx context.Context, flows []normalizers.NormalizedTakerFlow) (int, error) {
	count := 0
	for _, flow := range flows {
		flow = fillTakerFlowDeltas(flow)
		change := firstFloat(flow.BuySellDeltaQuote, flow.BuySellDelta)
		if change == nil {
			continue
		}

		previous, err := r.previousCVD(ctx, flow.SymbolID, flow.Exchange, flow.Period, flow.SnapshotTime)
		if err != nil {
			return 0, err
		}
		current := previous + *change
		changePercent := 0.0
		if previous != 0 {
			changePercent = (*change / previous) * 100
		}

		cvd := normalizers.NormalizedCVD{
			SourceMeta:       flow.SourceMeta,
			Period:           flow.Period,
			SnapshotTime:     flow.SnapshotTime,
			CVDValue:         &current,
			CVDDelta:         change,
			CVDChange:        change,
			CVDChangePercent: &changePercent,
			BuyVolume:        flow.TakerBuyVolume,
			SellVolume:       flow.TakerSellVolume,
		}

		inserted, err := r.UpsertCVD(ctx, []normalizers.NormalizedCVD{cvd})
		if err != nil {
			return 0, err
		}
		count += inserted
	}

	return count, nil
}

func (r *AdvancedRepository) UpsertCVD(ctx context.Context, items []normalizers.NormalizedCVD) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin cvd upsert: %w", err)
	}
	defer tx.Rollback()

	count := 0
	for _, item := range items {
		if err := normalizers.ValidateCVD(item); err != nil {
			return 0, err
		}

		result, err := tx.ExecContext(ctx, `
			INSERT INTO cvd_snapshots (
			    symbol_id, exchange, market_type, source_symbol, period, snapshot_time,
			    cvd_value, cvd_delta, cvd_change, cvd_change_percent,
			    buy_volume, sell_volume, raw_data
			)
			VALUES ($1, $2, '', $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			ON CONFLICT (symbol_id, exchange, period, snapshot_time) DO UPDATE SET
			    source_symbol = EXCLUDED.source_symbol,
			    cvd_value = EXCLUDED.cvd_value,
			    cvd_delta = EXCLUDED.cvd_delta,
			    cvd_change = EXCLUDED.cvd_change,
			    cvd_change_percent = EXCLUDED.cvd_change_percent,
			    buy_volume = EXCLUDED.buy_volume,
			    sell_volume = EXCLUDED.sell_volume,
			    raw_data = EXCLUDED.raw_data
		`,
			item.SymbolID,
			item.Exchange,
			item.SourceSymbol,
			item.Period,
			item.SnapshotTime,
			nullableFloat(item.CVDValue),
			nullableFloat(item.CVDDelta),
			nullableFloat(item.CVDChange),
			nullableFloat(item.CVDChangePercent),
			nullableFloat(item.BuyVolume),
			nullableFloat(item.SellVolume),
			ensureJSON(item.RawData),
		)
		if err != nil {
			return 0, fmt.Errorf("upsert cvd: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		count += int(affected)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit cvd upsert: %w", err)
	}

	return count, nil
}

func (r *AdvancedRepository) UpsertLiquidationEvents(ctx context.Context, items []normalizers.NormalizedLiquidationEvent) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin liquidation events upsert: %w", err)
	}
	defer tx.Rollback()

	count := 0
	for _, item := range items {
		if err := normalizers.ValidateLiquidationEvent(item); err != nil {
			return 0, err
		}
		usdValue := liquidationUSD(item)
		if usdValue < r.LiquidationMinUSD {
			continue
		}

		result, err := tx.ExecContext(ctx, `
			INSERT INTO liquidation_events (
			    event_key, exchange, market_type, symbol_id, source_symbol, event_time,
			    side, price, quantity, notional, usd_value, order_id, trade_id, raw_data
			)
			VALUES ($1, $2, '', $3, $4, $5, $6, $7, $8, $9, $10, NULLIF($11, ''), NULLIF($12, ''), $13)
			ON CONFLICT (event_key) DO UPDATE SET
			    price = EXCLUDED.price,
			    quantity = EXCLUDED.quantity,
			    notional = EXCLUDED.notional,
			    usd_value = EXCLUDED.usd_value,
			    raw_data = EXCLUDED.raw_data
		`,
			item.EventKey,
			item.Exchange,
			item.SymbolID,
			item.SourceSymbol,
			item.EventTime,
			item.Side,
			nullableFloat(item.Price),
			nullableFloat(item.Quantity),
			nullableFloat(item.Notional),
			usdValue,
			item.OrderID,
			item.TradeID,
			ensureJSON(item.RawData),
		)
		if err != nil {
			return 0, fmt.Errorf("upsert liquidation event: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		count += int(affected)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit liquidation events upsert: %w", err)
	}

	return count, nil
}

func (r *AdvancedRepository) UpsertLiquidationAggregates(ctx context.Context, items []normalizers.NormalizedLiquidationAggregate) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin liquidation aggregate upsert: %w", err)
	}
	defer tx.Rollback()

	count := 0
	for _, item := range items {
		if err := normalizers.ValidateLiquidationAggregate(item); err != nil {
			return 0, err
		}
		item = fillLiquidationUSD(item)

		result, err := tx.ExecContext(ctx, `
			INSERT INTO liquidation_aggregates (
			    symbol_id, exchange, market_type, source_symbol, period, bucket_time,
			    long_liquidation_count, short_liquidation_count,
			    long_liquidation_notional, short_liquidation_notional,
			    total_liquidation_notional, long_liquidation_usd,
			    short_liquidation_usd, total_liquidation_usd,
			    largest_liquidation_usd, raw_data
			)
			VALUES ($1, $2, '', $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
			ON CONFLICT (symbol_id, exchange, period, bucket_time) DO UPDATE SET
			    source_symbol = EXCLUDED.source_symbol,
			    long_liquidation_count = EXCLUDED.long_liquidation_count,
			    short_liquidation_count = EXCLUDED.short_liquidation_count,
			    long_liquidation_notional = EXCLUDED.long_liquidation_notional,
			    short_liquidation_notional = EXCLUDED.short_liquidation_notional,
			    total_liquidation_notional = EXCLUDED.total_liquidation_notional,
			    long_liquidation_usd = EXCLUDED.long_liquidation_usd,
			    short_liquidation_usd = EXCLUDED.short_liquidation_usd,
			    total_liquidation_usd = EXCLUDED.total_liquidation_usd,
			    largest_liquidation_usd = EXCLUDED.largest_liquidation_usd,
			    raw_data = EXCLUDED.raw_data
		`,
			item.SymbolID,
			item.Exchange,
			item.SourceSymbol,
			item.Period,
			item.BucketTime,
			item.LongLiquidationCount,
			item.ShortLiquidationCount,
			item.LongLiquidationNotional,
			item.ShortLiquidationNotional,
			item.TotalLiquidationNotional,
			item.LongLiquidationUSD,
			item.ShortLiquidationUSD,
			item.TotalLiquidationUSD,
			item.LargestLiquidationUSD,
			ensureJSON(item.RawData),
		)
		if err != nil {
			return 0, fmt.Errorf("upsert liquidation aggregate: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		count += int(affected)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit liquidation aggregate upsert: %w", err)
	}

	return count, nil
}

func (r *AdvancedRepository) UpsertBasisPremiums(ctx context.Context, items []normalizers.NormalizedBasisPremium) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin basis upsert: %w", err)
	}
	defer tx.Rollback()

	count := 0
	for _, item := range items {
		if err := normalizers.ValidateBasisPremium(item); err != nil {
			return 0, err
		}
		item = fillBasisValues(item)

		result, err := tx.ExecContext(ctx, `
			INSERT INTO basis_premium_snapshots (
			    symbol_id, exchange, market_type, source_symbol, snapshot_time,
			    mark_price, index_price, basis, basis_percent, premium_index,
			    funding_rate, futures_price, spot_price, basis_value,
			    annualized_basis_percent, raw_data
			)
			VALUES ($1, $2, '', $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
			ON CONFLICT (symbol_id, exchange, snapshot_time) DO UPDATE SET
			    source_symbol = EXCLUDED.source_symbol,
			    mark_price = EXCLUDED.mark_price,
			    index_price = EXCLUDED.index_price,
			    basis = EXCLUDED.basis,
			    basis_percent = EXCLUDED.basis_percent,
			    premium_index = EXCLUDED.premium_index,
			    funding_rate = EXCLUDED.funding_rate,
			    futures_price = EXCLUDED.futures_price,
			    spot_price = EXCLUDED.spot_price,
			    basis_value = EXCLUDED.basis_value,
			    annualized_basis_percent = EXCLUDED.annualized_basis_percent,
			    raw_data = EXCLUDED.raw_data
		`,
			item.SymbolID,
			item.Exchange,
			item.SourceSymbol,
			item.SnapshotTime,
			nullableFloat(item.MarkPrice),
			nullableFloat(item.IndexPrice),
			nullableFloat(item.Basis),
			nullableFloat(item.BasisPercent),
			nullableFloat(item.PremiumIndex),
			nullableFloat(item.FundingRate),
			nullableFloat(item.FuturesPrice),
			nullableFloat(item.SpotPrice),
			nullableFloat(item.BasisValue),
			nullableFloat(item.AnnualizedBasisPercent),
			ensureJSON(item.RawData),
		)
		if err != nil {
			return 0, fmt.Errorf("upsert basis premium: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		count += int(affected)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit basis upsert: %w", err)
	}

	return count, nil
}

func (r *AdvancedRepository) UpsertOrderbookImbalances(ctx context.Context, items []normalizers.NormalizedOrderbookImbalance) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin orderbook imbalance upsert: %w", err)
	}
	defer tx.Rollback()

	count := 0
	for _, item := range items {
		if err := normalizers.ValidateOrderbookImbalance(item); err != nil {
			return 0, err
		}

		result, err := tx.ExecContext(ctx, `
			INSERT INTO orderbook_imbalance_snapshots (
			    symbol_id, exchange, market_type, source_symbol, snapshot_time,
			    depth_levels, bid_notional, ask_notional, imbalance_ratio,
			    spread_bps, mid_price, spread_percent, bid_depth_usd,
			    ask_depth_usd, bid_depth_1pct_usd, ask_depth_1pct_usd,
			    bid_depth_2pct_usd, ask_depth_2pct_usd,
			    bid_depth_5pct_usd, ask_depth_5pct_usd,
			    imbalance_percent, raw_data
			)
			VALUES ($1, $2, '', $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
			ON CONFLICT (symbol_id, exchange, snapshot_time, depth_levels) DO UPDATE SET
			    source_symbol = EXCLUDED.source_symbol,
			    bid_notional = EXCLUDED.bid_notional,
			    ask_notional = EXCLUDED.ask_notional,
			    imbalance_ratio = EXCLUDED.imbalance_ratio,
			    spread_bps = EXCLUDED.spread_bps,
			    mid_price = EXCLUDED.mid_price,
			    spread_percent = EXCLUDED.spread_percent,
			    bid_depth_usd = EXCLUDED.bid_depth_usd,
			    ask_depth_usd = EXCLUDED.ask_depth_usd,
			    bid_depth_1pct_usd = EXCLUDED.bid_depth_1pct_usd,
			    ask_depth_1pct_usd = EXCLUDED.ask_depth_1pct_usd,
			    bid_depth_2pct_usd = EXCLUDED.bid_depth_2pct_usd,
			    ask_depth_2pct_usd = EXCLUDED.ask_depth_2pct_usd,
			    bid_depth_5pct_usd = EXCLUDED.bid_depth_5pct_usd,
			    ask_depth_5pct_usd = EXCLUDED.ask_depth_5pct_usd,
			    imbalance_percent = EXCLUDED.imbalance_percent,
			    raw_data = EXCLUDED.raw_data
		`,
			item.SymbolID,
			item.Exchange,
			item.SourceSymbol,
			item.SnapshotTime,
			item.DepthLevels,
			nullableFloat(item.BidNotional),
			nullableFloat(item.AskNotional),
			nullableFloat(item.ImbalanceRatio),
			nullableFloat(item.SpreadBPS),
			nullableFloat(item.MidPrice),
			nullableFloat(item.SpreadPercent),
			nullableFloat(item.BidDepthUSD),
			nullableFloat(item.AskDepthUSD),
			nullableFloat(item.BidDepth1PctUSD),
			nullableFloat(item.AskDepth1PctUSD),
			nullableFloat(item.BidDepth2PctUSD),
			nullableFloat(item.AskDepth2PctUSD),
			nullableFloat(item.BidDepth5PctUSD),
			nullableFloat(item.AskDepth5PctUSD),
			nullableFloat(item.ImbalancePercent),
			ensureJSON(item.RawData),
		)
		if err != nil {
			return 0, fmt.Errorf("upsert orderbook imbalance: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		count += int(affected)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit orderbook imbalance upsert: %w", err)
	}

	return count, nil
}

func (r *AdvancedRepository) UpsertExchangeDivergences(ctx context.Context, items []normalizers.NormalizedExchangeDivergence) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin exchange divergence upsert: %w", err)
	}
	defer tx.Rollback()

	count := 0
	for _, item := range items {
		if err := normalizers.ValidateExchangeDivergence(item); err != nil {
			return 0, err
		}

		result, err := tx.ExecContext(ctx, `
			INSERT INTO exchange_divergence_snapshots (
			    symbol_id, data_type, snapshot_time, reference_exchange,
			    compared_exchange, reference_value, compared_value,
			    divergence_abs, divergence_bps, price_min, price_max,
			    price_spread_percent, oi_min, oi_max, oi_spread_percent,
			    funding_min, funding_max, funding_spread, volume_min,
			    volume_max, volume_spread_percent, strongest_exchange,
			    weakest_exchange, raw_by_exchange, metadata
			)
			VALUES ($1, $2, $3, COALESCE(NULLIF($4, ''), 'aggregate'), COALESCE(NULLIF($5, ''), 'aggregate'), $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, NULLIF($22, ''), NULLIF($23, ''), $24, $25)
			ON CONFLICT (symbol_id, data_type, reference_exchange, compared_exchange, snapshot_time) DO UPDATE SET
			    reference_value = EXCLUDED.reference_value,
			    compared_value = EXCLUDED.compared_value,
			    divergence_abs = EXCLUDED.divergence_abs,
			    divergence_bps = EXCLUDED.divergence_bps,
			    price_min = EXCLUDED.price_min,
			    price_max = EXCLUDED.price_max,
			    price_spread_percent = EXCLUDED.price_spread_percent,
			    oi_min = EXCLUDED.oi_min,
			    oi_max = EXCLUDED.oi_max,
			    oi_spread_percent = EXCLUDED.oi_spread_percent,
			    funding_min = EXCLUDED.funding_min,
			    funding_max = EXCLUDED.funding_max,
			    funding_spread = EXCLUDED.funding_spread,
			    volume_min = EXCLUDED.volume_min,
			    volume_max = EXCLUDED.volume_max,
			    volume_spread_percent = EXCLUDED.volume_spread_percent,
			    strongest_exchange = EXCLUDED.strongest_exchange,
			    weakest_exchange = EXCLUDED.weakest_exchange,
			    raw_by_exchange = EXCLUDED.raw_by_exchange,
			    metadata = EXCLUDED.metadata
		`,
			item.SymbolID,
			item.DataType,
			item.SnapshotTime,
			item.ReferenceExchange,
			item.ComparedExchange,
			nullableFloat(item.ReferenceValue),
			nullableFloat(item.ComparedValue),
			nullableFloat(item.DivergenceAbs),
			nullableFloat(item.DivergenceBPS),
			nullableFloat(item.PriceMin),
			nullableFloat(item.PriceMax),
			nullableFloat(item.PriceSpreadPercent),
			nullableFloat(item.OIMin),
			nullableFloat(item.OIMax),
			nullableFloat(item.OISpreadPercent),
			nullableFloat(item.FundingMin),
			nullableFloat(item.FundingMax),
			nullableFloat(item.FundingSpread),
			nullableFloat(item.VolumeMin),
			nullableFloat(item.VolumeMax),
			nullableFloat(item.VolumeSpreadPercent),
			item.StrongestExchange,
			item.WeakestExchange,
			ensureJSON(item.RawByExchange),
			ensureJSON(item.RawData),
		)
		if err != nil {
			return 0, fmt.Errorf("upsert exchange divergence: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		count += int(affected)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit exchange divergence upsert: %w", err)
	}

	return count, nil
}

func (r *AdvancedRepository) previousCVD(ctx context.Context, symbolID int64, exchange string, period string, before time.Time) (float64, error) {
	var value sql.NullFloat64
	err := r.db.QueryRowContext(ctx, `
		SELECT cvd_value
		FROM cvd_snapshots
		WHERE symbol_id = $1
		  AND exchange = $2
		  AND period = $3
		  AND snapshot_time < $4
		ORDER BY snapshot_time DESC
		LIMIT 1
	`, symbolID, exchange, period, before).Scan(&value)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("query previous cvd: %w", err)
	}
	if !value.Valid {
		return 0, nil
	}

	return value.Float64, nil
}

func fillTakerFlowDeltas(item normalizers.NormalizedTakerFlow) normalizers.NormalizedTakerFlow {
	if item.BuySellDelta == nil && item.TakerBuyVolume != nil && item.TakerSellVolume != nil {
		value := *item.TakerBuyVolume - *item.TakerSellVolume
		item.BuySellDelta = &value
	}
	if item.BuySellDeltaQuote == nil && item.TakerBuyQuoteVolume != nil && item.TakerSellQuoteVolume != nil {
		value := *item.TakerBuyQuoteVolume - *item.TakerSellQuoteVolume
		item.BuySellDeltaQuote = &value
	}
	if item.BuySellRatio == nil && item.TakerSellVolume != nil && *item.TakerSellVolume != 0 && item.TakerBuyVolume != nil {
		value := *item.TakerBuyVolume / *item.TakerSellVolume
		item.BuySellRatio = &value
	}

	return item
}

func fillLiquidationUSD(item normalizers.NormalizedLiquidationAggregate) normalizers.NormalizedLiquidationAggregate {
	if item.LongLiquidationUSD == 0 {
		item.LongLiquidationUSD = item.LongLiquidationNotional
	}
	if item.ShortLiquidationUSD == 0 {
		item.ShortLiquidationUSD = item.ShortLiquidationNotional
	}
	if item.TotalLiquidationUSD == 0 {
		item.TotalLiquidationUSD = item.LongLiquidationUSD + item.ShortLiquidationUSD
	}
	if item.TotalLiquidationNotional == 0 {
		item.TotalLiquidationNotional = item.LongLiquidationNotional + item.ShortLiquidationNotional
	}

	return item
}

func fillBasisValues(item normalizers.NormalizedBasisPremium) normalizers.NormalizedBasisPremium {
	if item.BasisValue == nil && item.Basis != nil {
		item.BasisValue = item.Basis
	}
	if item.Basis == nil && item.BasisValue != nil {
		item.Basis = item.BasisValue
	}
	if item.BasisValue == nil && item.FuturesPrice != nil && item.IndexPrice != nil {
		value := *item.FuturesPrice - *item.IndexPrice
		item.BasisValue = &value
		item.Basis = &value
	}
	if item.BasisPercent == nil && item.BasisValue != nil && item.IndexPrice != nil && *item.IndexPrice != 0 {
		value := (*item.BasisValue / *item.IndexPrice) * 100
		item.BasisPercent = &value
	}

	return item
}

func liquidationUSD(item normalizers.NormalizedLiquidationEvent) float64 {
	if item.USDValue != nil {
		return *item.USDValue
	}
	if item.Notional != nil {
		return *item.Notional
	}
	if item.Price != nil && item.Quantity != nil {
		return *item.Price * *item.Quantity
	}
	return 0
}

func firstFloat(values ...*float64) *float64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
