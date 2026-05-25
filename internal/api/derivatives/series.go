package derivatives

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (r *Repository) ListMarket(ctx context.Context, symbolID int64, opts ListOptions) (PagedResponse[MarketSnapshotDTO], error) {
	builder := newSeriesBuilder(symbolID, opts, "snapshot_time")
	builder.exchange()
	query := fmt.Sprintf(`
		WITH latest AS (
		    SELECT DISTINCT ON (exchange)
		           exchange, source_symbol, snapshot_time, last_price, mark_price,
		           index_price, bid_price, ask_price, volume_24h, quote_volume_24h,
		           open_interest, funding_rate, raw_data
		    FROM derivative_market_snapshots
		    %s
		    ORDER BY exchange, snapshot_time DESC
		)
		SELECT COUNT(*) OVER(), exchange, source_symbol, snapshot_time,
		       last_price, mark_price, index_price, bid_price, ask_price,
		       volume_24h, quote_volume_24h, open_interest, funding_rate, raw_data
		FROM latest
		ORDER BY exchange ASC
		LIMIT %s OFFSET %s
	`, builder.whereSQL(), builder.arg(opts.Limit), builder.arg(opts.Offset()))
	rows, err := r.db.QueryContext(ctx, query, builder.args...)
	if err != nil {
		return PagedResponse[MarketSnapshotDTO]{}, fmt.Errorf("query market snapshots: %w", err)
	}
	defer rows.Close()

	var total int
	items := []MarketSnapshotDTO{}
	for rows.Next() {
		var item MarketSnapshotDTO
		if err := rows.Scan(
			&total,
			&item.Exchange,
			&item.SourceSymbol,
			&item.SnapshotTime,
			floatDest(&item.LastPrice),
			floatDest(&item.MarkPrice),
			floatDest(&item.IndexPrice),
			floatDest(&item.BidPrice),
			floatDest(&item.AskPrice),
			floatDest(&item.Volume24h),
			floatDest(&item.QuoteVolume24h),
			floatDest(&item.OpenInterest),
			floatDest(&item.FundingRate),
			&item.Raw,
		); err != nil {
			return PagedResponse[MarketSnapshotDTO]{}, fmt.Errorf("scan market snapshot: %w", err)
		}
		item.Raw = ensureJSON(item.Raw)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return PagedResponse[MarketSnapshotDTO]{}, fmt.Errorf("iterate market snapshots: %w", err)
	}
	return PagedResponse[MarketSnapshotDTO]{Data: items, Meta: opts.PageMeta(total)}, nil
}

func (r *Repository) ListKlines(ctx context.Context, symbolID int64, opts ListOptions) ([]KlineDTO, error) {
	page, err := r.ListKlinesPage(ctx, symbolID, opts)
	return page.Data, err
}

func (r *Repository) ListKlinesPage(ctx context.Context, symbolID int64, opts ListOptions) (PagedResponse[KlineDTO], error) {
	if opts.Interval == "" {
		opts.Interval = "5m"
	}
	builder := newSeriesBuilder(symbolID, opts, "open_time")
	builder.exchange()
	builder.interval(`"interval"`, opts.Interval)
	query := fmt.Sprintf(`
		SELECT COUNT(*) OVER(), exchange, "interval", open_time, close_time,
		       open_price, high_price, low_price, close_price, volume,
		       quote_volume, trade_count, is_closed
		FROM derivative_klines
		%s
		ORDER BY open_time %s
		LIMIT %s OFFSET %s
	`, builder.whereSQL(), direction(opts.Direction), builder.arg(opts.Limit), builder.arg(opts.Offset()))
	rows, err := r.db.QueryContext(ctx, query, builder.args...)
	if err != nil {
		return PagedResponse[KlineDTO]{}, fmt.Errorf("query klines: %w", err)
	}
	defer rows.Close()

	var total int
	items := []KlineDTO{}
	for rows.Next() {
		var item KlineDTO
		if err := rows.Scan(
			&total,
			&item.Exchange,
			&item.Interval,
			&item.OpenTime,
			&item.CloseTime,
			&item.Open,
			&item.High,
			&item.Low,
			&item.Close,
			floatDest(&item.Volume),
			floatDest(&item.QuoteVolume),
			int64Dest(&item.TradeCount),
			&item.IsClosed,
		); err != nil {
			return PagedResponse[KlineDTO]{}, fmt.Errorf("scan kline: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return PagedResponse[KlineDTO]{}, fmt.Errorf("iterate klines: %w", err)
	}
	return PagedResponse[KlineDTO]{Data: items, Meta: opts.PageMeta(total)}, nil
}

func (r *Repository) ListOpenInterest(ctx context.Context, symbolID int64, opts ListOptions) ([]OpenInterestDTO, error) {
	page, err := r.ListOpenInterestPage(ctx, symbolID, opts)
	return page.Data, err
}

func (r *Repository) ListOpenInterestPage(ctx context.Context, symbolID int64, opts ListOptions) (PagedResponse[OpenInterestDTO], error) {
	if opts.Period != "" {
		return r.listOpenInterestHistory(ctx, symbolID, opts)
	}
	builder := newSeriesBuilder(symbolID, opts, "snapshot_time")
	builder.exchange()
	query := fmt.Sprintf(`
		SELECT COUNT(*) OVER(), exchange, '' AS period, snapshot_time,
		       open_interest, open_interest_value
		FROM open_interest_snapshots
		%s
		ORDER BY snapshot_time %s
		LIMIT %s OFFSET %s
	`, builder.whereSQL(), direction(opts.Direction), builder.arg(opts.Limit), builder.arg(opts.Offset()))
	return scanOpenInterest(ctx, r.db, query, builder.args, opts)
}

func (r *Repository) listOpenInterestHistory(ctx context.Context, symbolID int64, opts ListOptions) (PagedResponse[OpenInterestDTO], error) {
	builder := newSeriesBuilder(symbolID, opts, `"timestamp"`)
	builder.exchange()
	builder.interval("period", opts.Period)
	query := fmt.Sprintf(`
		SELECT COUNT(*) OVER(), exchange, period, "timestamp",
		       open_interest, open_interest_value
		FROM open_interest_history
		%s
		ORDER BY "timestamp" %s
		LIMIT %s OFFSET %s
	`, builder.whereSQL(), direction(opts.Direction), builder.arg(opts.Limit), builder.arg(opts.Offset()))
	return scanOpenInterest(ctx, r.db, query, builder.args, opts)
}

func scanOpenInterest(ctx context.Context, db *sql.DB, query string, args []any, opts ListOptions) (PagedResponse[OpenInterestDTO], error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return PagedResponse[OpenInterestDTO]{}, fmt.Errorf("query open interest: %w", err)
	}
	defer rows.Close()

	var total int
	items := []OpenInterestDTO{}
	for rows.Next() {
		var item OpenInterestDTO
		if err := rows.Scan(&total, &item.Exchange, &item.Period, &item.Timestamp, &item.OpenInterest, floatDest(&item.OpenInterestValue)); err != nil {
			return PagedResponse[OpenInterestDTO]{}, fmt.Errorf("scan open interest: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return PagedResponse[OpenInterestDTO]{}, fmt.Errorf("iterate open interest: %w", err)
	}
	return PagedResponse[OpenInterestDTO]{Data: items, Meta: opts.PageMeta(total)}, nil
}

func (r *Repository) ListFunding(ctx context.Context, symbolID int64, opts ListOptions) ([]FundingDTO, error) {
	page, err := r.ListFundingPage(ctx, symbolID, opts)
	return page.Data, err
}

func (r *Repository) ListFundingPage(ctx context.Context, symbolID int64, opts ListOptions) (PagedResponse[FundingDTO], error) {
	builder := newSeriesBuilder(symbolID, opts, "funding_time")
	builder.exchange()
	query := fmt.Sprintf(`
		SELECT COUNT(*) OVER(), exchange, funding_time, funding_rate,
		       realized_rate, NULL::timestamptz AS next_funding_time, mark_price
		FROM funding_rate_history
		%s
		ORDER BY funding_time %s
		LIMIT %s OFFSET %s
	`, builder.whereSQL(), direction(opts.Direction), builder.arg(opts.Limit), builder.arg(opts.Offset()))
	rows, err := r.db.QueryContext(ctx, query, builder.args...)
	if err != nil {
		return PagedResponse[FundingDTO]{}, fmt.Errorf("query funding: %w", err)
	}
	defer rows.Close()

	var total int
	items := []FundingDTO{}
	for rows.Next() {
		var item FundingDTO
		var nextFunding sql.NullTime
		if err := rows.Scan(&total, &item.Exchange, &item.Timestamp, &item.FundingRate, floatDest(&item.RealizedRate), &nextFunding, floatDest(&item.MarkPrice)); err != nil {
			return PagedResponse[FundingDTO]{}, fmt.Errorf("scan funding: %w", err)
		}
		item.NextFundingTime = nullTimePtr(nextFunding)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return PagedResponse[FundingDTO]{}, fmt.Errorf("iterate funding: %w", err)
	}
	return PagedResponse[FundingDTO]{Data: items, Meta: opts.PageMeta(total)}, nil
}

func (r *Repository) ListLongShortRatioPage(ctx context.Context, symbolID int64, opts ListOptions) (PagedResponse[LongShortRatioDTO], error) {
	if opts.Period == "" {
		opts.Period = "5m"
	}
	builder := newSeriesBuilder(symbolID, opts, "snapshot_time")
	builder.exchange()
	builder.interval("period", opts.Period)
	query := fmt.Sprintf(`
		SELECT COUNT(*) OVER(), exchange, period, snapshot_time,
		       long_account_ratio, short_account_ratio, long_short_ratio,
		       top_trader_long_ratio, top_trader_short_ratio
		FROM long_short_ratio_snapshots
		%s
		ORDER BY snapshot_time %s
		LIMIT %s OFFSET %s
	`, builder.whereSQL(), direction(opts.Direction), builder.arg(opts.Limit), builder.arg(opts.Offset()))
	rows, err := r.db.QueryContext(ctx, query, builder.args...)
	if err != nil {
		return PagedResponse[LongShortRatioDTO]{}, fmt.Errorf("query long short ratio: %w", err)
	}
	defer rows.Close()

	var total int
	items := []LongShortRatioDTO{}
	for rows.Next() {
		var item LongShortRatioDTO
		if err := rows.Scan(&total, &item.Exchange, &item.Period, &item.SnapshotTime, floatDest(&item.LongAccountRatio), floatDest(&item.ShortAccountRatio), floatDest(&item.LongShortRatio), floatDest(&item.TopTraderLongRatio), floatDest(&item.TopTraderShortRatio)); err != nil {
			return PagedResponse[LongShortRatioDTO]{}, fmt.Errorf("scan long short ratio: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return PagedResponse[LongShortRatioDTO]{}, fmt.Errorf("iterate long short ratio: %w", err)
	}
	return PagedResponse[LongShortRatioDTO]{Data: items, Meta: opts.PageMeta(total)}, nil
}

func (r *Repository) ListTakerFlow(ctx context.Context, symbolID int64, opts ListOptions) ([]TakerFlowDTO, error) {
	page, err := r.ListTakerFlowPage(ctx, symbolID, opts)
	return page.Data, err
}

func (r *Repository) ListTakerFlowPage(ctx context.Context, symbolID int64, opts ListOptions) (PagedResponse[TakerFlowDTO], error) {
	if opts.Period == "" {
		opts.Period = "5m"
	}
	builder := newSeriesBuilder(symbolID, opts, "snapshot_time")
	builder.exchange()
	builder.interval("period", opts.Period)
	query := fmt.Sprintf(`
		SELECT COUNT(*) OVER(), exchange, period, snapshot_time,
		       taker_buy_volume, taker_sell_volume, taker_buy_quote_volume,
		       taker_sell_quote_volume, buy_sell_delta, buy_sell_delta_quote,
		       buy_sell_ratio
		FROM taker_flow_snapshots
		%s
		ORDER BY snapshot_time %s
		LIMIT %s OFFSET %s
	`, builder.whereSQL(), direction(opts.Direction), builder.arg(opts.Limit), builder.arg(opts.Offset()))
	rows, err := r.db.QueryContext(ctx, query, builder.args...)
	if err != nil {
		return PagedResponse[TakerFlowDTO]{}, fmt.Errorf("query taker flow: %w", err)
	}
	defer rows.Close()

	var total int
	items := []TakerFlowDTO{}
	for rows.Next() {
		var item TakerFlowDTO
		if err := rows.Scan(&total, &item.Exchange, &item.Period, &item.SnapshotTime, floatDest(&item.TakerBuyVolume), floatDest(&item.TakerSellVolume), floatDest(&item.TakerBuyQuoteVolume), floatDest(&item.TakerSellQuoteVolume), floatDest(&item.BuySellDelta), floatDest(&item.BuySellDeltaQuote), floatDest(&item.BuySellRatio)); err != nil {
			return PagedResponse[TakerFlowDTO]{}, fmt.Errorf("scan taker flow: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return PagedResponse[TakerFlowDTO]{}, fmt.Errorf("iterate taker flow: %w", err)
	}
	return PagedResponse[TakerFlowDTO]{Data: items, Meta: opts.PageMeta(total)}, nil
}

func (r *Repository) ListCVD(ctx context.Context, symbolID int64, opts ListOptions) ([]CVDDTO, error) {
	page, err := r.ListCVDPage(ctx, symbolID, opts)
	return page.Data, err
}

func (r *Repository) ListCVDPage(ctx context.Context, symbolID int64, opts ListOptions) (PagedResponse[CVDDTO], error) {
	if opts.Period == "" {
		opts.Period = "5m"
	}
	builder := newSeriesBuilder(symbolID, opts, "snapshot_time")
	builder.exchange()
	builder.interval("period", opts.Period)
	query := fmt.Sprintf(`
		SELECT COUNT(*) OVER(), exchange, period, snapshot_time,
		       cvd_value, cvd_delta, cvd_change, cvd_change_percent
		FROM cvd_snapshots
		%s
		ORDER BY snapshot_time %s
		LIMIT %s OFFSET %s
	`, builder.whereSQL(), direction(opts.Direction), builder.arg(opts.Limit), builder.arg(opts.Offset()))
	rows, err := r.db.QueryContext(ctx, query, builder.args...)
	if err != nil {
		return PagedResponse[CVDDTO]{}, fmt.Errorf("query cvd: %w", err)
	}
	defer rows.Close()

	var total int
	items := []CVDDTO{}
	for rows.Next() {
		var item CVDDTO
		if err := rows.Scan(&total, &item.Exchange, &item.Period, &item.SnapshotTime, floatDest(&item.CVDValue), floatDest(&item.CVDDelta), floatDest(&item.CVDChange), floatDest(&item.CVDChangePercent)); err != nil {
			return PagedResponse[CVDDTO]{}, fmt.Errorf("scan cvd: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return PagedResponse[CVDDTO]{}, fmt.Errorf("iterate cvd: %w", err)
	}
	return PagedResponse[CVDDTO]{Data: items, Meta: opts.PageMeta(total)}, nil
}

func (r *Repository) ListLiquidations(ctx context.Context, symbolID int64, opts ListOptions) ([]LiquidationAggregateDTO, error) {
	page, err := r.ListLiquidationsPage(ctx, symbolID, opts)
	return page.Data, err
}

func (r *Repository) ListLiquidationsPage(ctx context.Context, symbolID int64, opts ListOptions) (PagedResponse[LiquidationAggregateDTO], error) {
	if opts.Period == "" {
		opts.Period = "5m"
	}
	builder := newSeriesBuilder(symbolID, opts, "bucket_time")
	builder.exchange()
	builder.interval("period", opts.Period)
	query := fmt.Sprintf(`
		SELECT COUNT(*) OVER(), exchange, period, bucket_time,
		       long_liquidation_count, short_liquidation_count,
		       long_liquidation_usd, short_liquidation_usd,
		       total_liquidation_usd, largest_liquidation_usd
		FROM liquidation_aggregates
		%s
		ORDER BY bucket_time %s
		LIMIT %s OFFSET %s
	`, builder.whereSQL(), direction(opts.Direction), builder.arg(opts.Limit), builder.arg(opts.Offset()))
	rows, err := r.db.QueryContext(ctx, query, builder.args...)
	if err != nil {
		return PagedResponse[LiquidationAggregateDTO]{}, fmt.Errorf("query liquidations: %w", err)
	}
	defer rows.Close()

	var total int
	items := []LiquidationAggregateDTO{}
	for rows.Next() {
		var item LiquidationAggregateDTO
		if err := rows.Scan(&total, &item.Exchange, &item.Period, &item.BucketTime, &item.LongCount, &item.ShortCount, &item.LongLiquidationUSD, &item.ShortLiquidationUSD, &item.TotalLiquidationUSD, &item.LargestLiquidationUSD); err != nil {
			return PagedResponse[LiquidationAggregateDTO]{}, fmt.Errorf("scan liquidations: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return PagedResponse[LiquidationAggregateDTO]{}, fmt.Errorf("iterate liquidations: %w", err)
	}
	return PagedResponse[LiquidationAggregateDTO]{Data: items, Meta: opts.PageMeta(total)}, nil
}

func (r *Repository) ListBasisPage(ctx context.Context, symbolID int64, opts ListOptions) (PagedResponse[BasisDTO], error) {
	builder := newSeriesBuilder(symbolID, opts, "snapshot_time")
	builder.exchange()
	query := fmt.Sprintf(`
		SELECT COUNT(*) OVER(), exchange, snapshot_time, futures_price,
		       spot_price, mark_price, index_price, basis_value,
		       basis_percent, annualized_basis_percent, funding_rate
		FROM basis_premium_snapshots
		%s
		ORDER BY snapshot_time %s
		LIMIT %s OFFSET %s
	`, builder.whereSQL(), direction(opts.Direction), builder.arg(opts.Limit), builder.arg(opts.Offset()))
	rows, err := r.db.QueryContext(ctx, query, builder.args...)
	if err != nil {
		return PagedResponse[BasisDTO]{}, fmt.Errorf("query basis: %w", err)
	}
	defer rows.Close()

	var total int
	items := []BasisDTO{}
	for rows.Next() {
		var item BasisDTO
		if err := rows.Scan(&total, &item.Exchange, &item.SnapshotTime, floatDest(&item.FuturesPrice), floatDest(&item.SpotPrice), floatDest(&item.MarkPrice), floatDest(&item.IndexPrice), floatDest(&item.BasisValue), floatDest(&item.BasisPercent), floatDest(&item.AnnualizedBasisPercent), floatDest(&item.FundingRate)); err != nil {
			return PagedResponse[BasisDTO]{}, fmt.Errorf("scan basis: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return PagedResponse[BasisDTO]{}, fmt.Errorf("iterate basis: %w", err)
	}
	return PagedResponse[BasisDTO]{Data: items, Meta: opts.PageMeta(total)}, nil
}

func (r *Repository) ListOrderbookImbalancePage(ctx context.Context, symbolID int64, opts ListOptions) (PagedResponse[OrderbookImbalanceDTO], error) {
	builder := newSeriesBuilder(symbolID, opts, "snapshot_time")
	builder.exchange()
	query := fmt.Sprintf(`
		SELECT COUNT(*) OVER(), exchange, snapshot_time, depth_levels,
		       mid_price, spread_percent, bid_depth_usd, ask_depth_usd,
		       imbalance_percent
		FROM orderbook_imbalance_snapshots
		%s
		ORDER BY snapshot_time %s
		LIMIT %s OFFSET %s
	`, builder.whereSQL(), direction(opts.Direction), builder.arg(opts.Limit), builder.arg(opts.Offset()))
	rows, err := r.db.QueryContext(ctx, query, builder.args...)
	if err != nil {
		return PagedResponse[OrderbookImbalanceDTO]{}, fmt.Errorf("query orderbook imbalance: %w", err)
	}
	defer rows.Close()

	var total int
	items := []OrderbookImbalanceDTO{}
	for rows.Next() {
		var item OrderbookImbalanceDTO
		if err := rows.Scan(&total, &item.Exchange, &item.SnapshotTime, &item.DepthLevels, floatDest(&item.MidPrice), floatDest(&item.SpreadPercent), floatDest(&item.BidDepthUSD), floatDest(&item.AskDepthUSD), floatDest(&item.ImbalancePercent)); err != nil {
			return PagedResponse[OrderbookImbalanceDTO]{}, fmt.Errorf("scan orderbook imbalance: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return PagedResponse[OrderbookImbalanceDTO]{}, fmt.Errorf("iterate orderbook imbalance: %w", err)
	}
	return PagedResponse[OrderbookImbalanceDTO]{Data: items, Meta: opts.PageMeta(total)}, nil
}

func (r *Repository) ListExchangeDivergence(ctx context.Context, symbolID int64, opts ListOptions) ([]ExchangeDivergenceDTO, error) {
	page, err := r.ListExchangeDivergencePage(ctx, symbolID, opts)
	return page.Data, err
}

func (r *Repository) ListExchangeDivergencePage(ctx context.Context, symbolID int64, opts ListOptions) (PagedResponse[ExchangeDivergenceDTO], error) {
	builder := newSeriesBuilder(symbolID, opts, "snapshot_time")
	query := fmt.Sprintf(`
		SELECT COUNT(*) OVER(), data_type, snapshot_time,
		       price_spread_percent, oi_spread_percent, funding_spread,
		       volume_spread_percent, COALESCE(strongest_exchange, ''),
		       COALESCE(weakest_exchange, ''),
		       raw_by_exchange, metadata
		FROM exchange_divergence_snapshots
		%s
		ORDER BY snapshot_time %s
		LIMIT %s OFFSET %s
	`, builder.whereSQL(), direction(opts.Direction), builder.arg(opts.Limit), builder.arg(opts.Offset()))
	rows, err := r.db.QueryContext(ctx, query, builder.args...)
	if err != nil {
		return PagedResponse[ExchangeDivergenceDTO]{}, fmt.Errorf("query exchange divergence: %w", err)
	}
	defer rows.Close()

	var total int
	items := []ExchangeDivergenceDTO{}
	for rows.Next() {
		var item ExchangeDivergenceDTO
		if err := rows.Scan(&total, &item.DataType, &item.SnapshotTime, floatDest(&item.PriceSpreadPercent), floatDest(&item.OISpreadPercent), floatDest(&item.FundingSpread), floatDest(&item.VolumeSpreadPercent), &item.StrongestExchange, &item.WeakestExchange, &item.RawByExchange, &item.Metadata); err != nil {
			return PagedResponse[ExchangeDivergenceDTO]{}, fmt.Errorf("scan exchange divergence: %w", err)
		}
		item.RawByExchange = ensureJSON(item.RawByExchange)
		item.Metadata = ensureJSON(item.Metadata)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return PagedResponse[ExchangeDivergenceDTO]{}, fmt.Errorf("iterate exchange divergence: %w", err)
	}
	return PagedResponse[ExchangeDivergenceDTO]{Data: items, Meta: opts.PageMeta(total)}, nil
}

type seriesBuilder struct {
	args       []any
	clauses    []string
	opts       ListOptions
	timeColumn string
}

func newSeriesBuilder(symbolID int64, opts ListOptions, timeColumn string) *seriesBuilder {
	builder := &seriesBuilder{opts: opts, timeColumn: timeColumn}
	builder.clauses = append(builder.clauses, "symbol_id = "+builder.arg(symbolID))
	if opts.StartTime != nil {
		builder.clauses = append(builder.clauses, timeColumn+" >= "+builder.arg(*opts.StartTime))
	}
	if opts.EndTime != nil {
		builder.clauses = append(builder.clauses, timeColumn+" <= "+builder.arg(*opts.EndTime))
	}
	return builder
}

func (b *seriesBuilder) exchange() {
	if b.opts.Exchange != "" {
		b.clauses = append(b.clauses, "exchange = "+b.arg(b.opts.Exchange))
	}
}

func (b *seriesBuilder) interval(column string, value string) {
	if value != "" {
		b.clauses = append(b.clauses, column+" = "+b.arg(value))
	}
}

func (b *seriesBuilder) whereSQL() string {
	if len(b.clauses) == 0 {
		return ""
	}
	return "WHERE " + strings.Join(b.clauses, " AND ")
}

func (b *seriesBuilder) arg(value any) string {
	b.args = append(b.args, value)
	return fmt.Sprintf("$%d", len(b.args))
}

func direction(value string) string {
	if strings.EqualFold(value, "asc") {
		return "ASC"
	}
	return "DESC"
}

func defaultEndTime(opts ListOptions) time.Time {
	if opts.EndTime != nil {
		return *opts.EndTime
	}
	return time.Now().UTC()
}
