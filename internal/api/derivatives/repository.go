package derivatives

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrNotFound = errors.New("not found")

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListOverview(ctx context.Context, opts ListOptions) (PagedResponse[OverviewItem], error) {
	query, args := overviewSQL(opts, false)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return PagedResponse[OverviewItem]{}, fmt.Errorf("query derivative overview: %w", err)
	}
	defer rows.Close()

	var total int
	items := []OverviewItem{}
	for rows.Next() {
		var item OverviewItem
		if err := scanOverview(rows, &total, &item); err != nil {
			return PagedResponse[OverviewItem]{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return PagedResponse[OverviewItem]{}, fmt.Errorf("iterate derivative overview: %w", err)
	}

	return PagedResponse[OverviewItem]{Data: items, Meta: opts.PageMeta(total)}, nil
}

func (r *Repository) ListSymbols(ctx context.Context, opts ListOptions) (PagedResponse[OverviewItem], error) {
	query, args := overviewSQL(opts, true)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return PagedResponse[OverviewItem]{}, fmt.Errorf("query derivative symbols: %w", err)
	}
	defer rows.Close()

	var total int
	items := []OverviewItem{}
	for rows.Next() {
		var item OverviewItem
		if err := scanOverview(rows, &total, &item); err != nil {
			return PagedResponse[OverviewItem]{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return PagedResponse[OverviewItem]{}, fmt.Errorf("iterate derivative symbols: %w", err)
	}

	return PagedResponse[OverviewItem]{Data: items, Meta: opts.PageMeta(total)}, nil
}

func (r *Repository) GetSymbolDetail(ctx context.Context, symbol string, opts ListOptions) (SymbolDetail, error) {
	symbolDTO, symbolID, err := r.ResolveSymbol(ctx, symbol)
	if err != nil {
		return SymbolDetail{}, err
	}
	aggregate, err := r.LatestAggregate(ctx, symbolID)
	if err != nil {
		return SymbolDetail{}, err
	}

	detail := SymbolDetail{Symbol: symbolDTO, Market: aggregate}
	if aggregate != nil {
		detail.PerExchange = aggregate.RawByExchange
	}

	seriesOpts := opts
	seriesOpts.Limit = minPositive(opts.Limit, 100)
	seriesOpts.Page = 1
	detail.Klines, _ = r.ListKlines(ctx, symbolID, seriesOpts)
	detail.OpenInterest, _ = r.ListOpenInterest(ctx, symbolID, seriesOpts)
	detail.Funding, _ = r.ListFunding(ctx, symbolID, seriesOpts)
	if page, err := r.ListLongShortRatioPage(ctx, symbolID, seriesOpts); err == nil {
		detail.LongShortRatio = page.Data
	}
	detail.TakerFlow, _ = r.ListTakerFlow(ctx, symbolID, seriesOpts)
	detail.CVD, _ = r.ListCVD(ctx, symbolID, seriesOpts)
	detail.Liquidations, _ = r.ListLiquidations(ctx, symbolID, seriesOpts)
	if page, err := r.ListBasisPage(ctx, symbolID, seriesOpts); err == nil {
		detail.Basis = page.Data
	}
	if page, err := r.ListOrderbookImbalancePage(ctx, symbolID, seriesOpts); err == nil {
		detail.OrderbookImbalance = page.Data
	}
	detail.ExchangeDivergence, _ = r.ListExchangeDivergence(ctx, symbolID, seriesOpts)

	return detail, nil
}

func (r *Repository) ResolveSymbol(ctx context.Context, symbol string) (SymbolDTO, int64, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return SymbolDTO{}, 0, ErrNotFound
	}

	var (
		dto        SymbolDTO
		marketsRaw []byte
		marketType sql.NullString
		baseAsset  sql.NullString
		quoteAsset sql.NullString
		cmcRank    sql.NullInt64
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT id, symbol, base_asset, quote_asset, market_type, cmc_rank, is_active,
		       COALESCE(markets, '[]'::jsonb)
		FROM symbols
		WHERE lower(symbol) = lower($1)
		   OR lower(base_asset) = lower($1)
		ORDER BY is_active DESC, cmc_rank ASC NULLS LAST, symbol ASC
		LIMIT 1
	`, symbol).Scan(&dto.ID, &dto.Symbol, &baseAsset, &quoteAsset, &marketType, &cmcRank, &dto.IsActive, &marketsRaw)
	if err == sql.ErrNoRows {
		return SymbolDTO{}, 0, ErrNotFound
	}
	if err != nil {
		return SymbolDTO{}, 0, fmt.Errorf("resolve symbol: %w", err)
	}

	dto.BaseAsset = baseAsset.String
	dto.QuoteAsset = quoteAsset.String
	dto.MarketType = marketType.String
	dto.Category = marketType.String
	dto.CmcRank = int(cmcRank.Int64)
	dto.Markets = ensureJSONArray(marketsRaw)

	return dto, dto.ID, nil
}

func (r *Repository) LatestAggregate(ctx context.Context, symbolID int64) (*AggregateDTO, error) {
	var dto AggregateDTO
	err := r.db.QueryRowContext(ctx, `
		SELECT snapshot_time, price_avg, price_weighted, total_volume_24h,
		       total_quote_volume_24h, total_open_interest,
		       total_open_interest_value, avg_funding_rate, total_cvd,
		       total_long_liquidation_usd, total_short_liquidation_usd,
		       total_liquidation_usd, avg_basis_percent,
		       avg_orderbook_imbalance_percent, exchange_count,
		       available_exchanges, raw_by_exchange, window_metrics,
		       anomaly_flags, quality_metadata
		FROM derivative_aggregated_snapshots
		WHERE symbol_id = $1
		ORDER BY snapshot_time DESC
		LIMIT 1
	`, symbolID).Scan(
		&dto.SnapshotTime,
		floatDest(&dto.PriceAvg),
		floatDest(&dto.PriceWeighted),
		floatDest(&dto.TotalVolume24h),
		floatDest(&dto.TotalQuoteVolume24h),
		floatDest(&dto.TotalOpenInterest),
		floatDest(&dto.TotalOpenInterestValue),
		floatDest(&dto.AvgFundingRate),
		floatDest(&dto.TotalCVD),
		floatDest(&dto.TotalLongLiquidationUSD),
		floatDest(&dto.TotalShortLiquidationUSD),
		floatDest(&dto.TotalLiquidationUSD),
		floatDest(&dto.AvgBasisPercent),
		floatDest(&dto.AvgOrderbookImbalancePercent),
		&dto.ExchangeCount,
		&dto.AvailableExchanges,
		&dto.RawByExchange,
		&dto.WindowMetrics,
		&dto.AnomalyFlags,
		&dto.Quality,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query latest aggregate: %w", err)
	}
	dto.Price = firstFloat(dto.PriceWeighted, dto.PriceAvg)
	dto.Freshness = freshness(dto.SnapshotTime)
	dto.AvailableExchanges = ensureJSON(dto.AvailableExchanges)
	dto.RawByExchange = ensureJSON(dto.RawByExchange)
	dto.WindowMetrics = ensureJSON(dto.WindowMetrics)
	dto.AnomalyFlags = ensureJSONArray(dto.AnomalyFlags)
	dto.Quality = ensureJSON(dto.Quality)

	return &dto, nil
}

func overviewSQL(opts ListOptions, symbolsOnly bool) (string, []any) {
	builder := strings.Builder{}
	builder.WriteString(`
		WITH latest AS (
		    SELECT DISTINCT ON (symbol_id)
		           symbol_id, snapshot_time, price_avg, price_weighted,
		           total_volume_24h, total_quote_volume_24h,
		           total_open_interest, total_open_interest_value,
		           avg_funding_rate, total_cvd,
		           total_long_liquidation_usd, total_short_liquidation_usd,
		           total_liquidation_usd, avg_basis_percent,
		           avg_orderbook_imbalance_percent, exchange_count,
		           available_exchanges, raw_by_exchange, window_metrics,
		           anomaly_flags, quality_metadata
		    FROM derivative_aggregated_snapshots
		    ORDER BY symbol_id, snapshot_time DESC
		)
		SELECT COUNT(*) OVER(),
		       s.id, s.symbol, s.base_asset, s.quote_asset, s.market_type,
		       s.cmc_rank, s.is_active, COALESCE(s.markets, '[]'::jsonb),
		       latest.snapshot_time, latest.price_avg, latest.price_weighted,
		       latest.total_volume_24h, latest.total_quote_volume_24h,
		       latest.total_open_interest, latest.total_open_interest_value,
		       latest.avg_funding_rate, latest.total_cvd,
		       latest.total_long_liquidation_usd,
		       latest.total_short_liquidation_usd,
		       latest.total_liquidation_usd, latest.avg_basis_percent,
		       latest.avg_orderbook_imbalance_percent,
		       COALESCE(latest.exchange_count, 0),
		       COALESCE(latest.available_exchanges, '[]'::jsonb),
		       COALESCE(latest.raw_by_exchange, '{}'::jsonb),
		       COALESCE(latest.window_metrics, '{}'::jsonb),
		       COALESCE(latest.anomaly_flags, '[]'::jsonb),
		       COALESCE(latest.quality_metadata, '{}'::jsonb)
		FROM symbols s
		LEFT JOIN latest ON latest.symbol_id = s.id
		WHERE s.is_active = true
	`)
	args := []any{}
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if opts.Search != "" {
		placeholder := addArg("%" + opts.Search + "%")
		builder.WriteString(" AND (s.symbol ILIKE " + placeholder + " OR s.base_asset ILIKE " + placeholder + " OR s.quote_asset ILIKE " + placeholder + ")")
	}
	if opts.Category != "" {
		builder.WriteString(" AND s.market_type = " + addArg(opts.Category))
	}
	if opts.Exchange != "" {
		builder.WriteString(" AND EXISTS (SELECT 1 FROM jsonb_array_elements(COALESCE(s.markets, '[]'::jsonb)) market WHERE lower(market->>'exchange') = " + addArg(opts.Exchange) + ")")
	}
	if opts.MinVolume != nil {
		builder.WriteString(" AND latest.total_volume_24h >= " + addArg(*opts.MinVolume))
	}
	if opts.MinOI != nil {
		builder.WriteString(" AND latest.total_open_interest >= " + addArg(*opts.MinOI))
	}
	if opts.RankMin != nil {
		builder.WriteString(" AND s.cmc_rank >= " + addArg(*opts.RankMin))
	}
	if opts.RankMax != nil {
		builder.WriteString(" AND s.cmc_rank <= " + addArg(*opts.RankMax))
	}
	builder.WriteString(" ORDER BY " + overviewSort(opts.Sort) + " " + strings.ToUpper(opts.Direction) + " NULLS LAST, s.symbol ASC")
	builder.WriteString(" LIMIT " + addArg(opts.Limit) + " OFFSET " + addArg(opts.Offset()))

	return builder.String(), args
}

func scanOverview(rows *sql.Rows, total *int, item *OverviewItem) error {
	var (
		symbol     SymbolDTO
		aggregate  AggregateDTO
		marketsRaw []byte
		baseAsset  sql.NullString
		quoteAsset sql.NullString
		marketType sql.NullString
		cmcRank    sql.NullInt64
		snapshotAt sql.NullTime
	)
	if err := rows.Scan(
		total,
		&symbol.ID,
		&symbol.Symbol,
		&baseAsset,
		&quoteAsset,
		&marketType,
		&cmcRank,
		&symbol.IsActive,
		&marketsRaw,
		&snapshotAt,
		floatDest(&aggregate.PriceAvg),
		floatDest(&aggregate.PriceWeighted),
		floatDest(&aggregate.TotalVolume24h),
		floatDest(&aggregate.TotalQuoteVolume24h),
		floatDest(&aggregate.TotalOpenInterest),
		floatDest(&aggregate.TotalOpenInterestValue),
		floatDest(&aggregate.AvgFundingRate),
		floatDest(&aggregate.TotalCVD),
		floatDest(&aggregate.TotalLongLiquidationUSD),
		floatDest(&aggregate.TotalShortLiquidationUSD),
		floatDest(&aggregate.TotalLiquidationUSD),
		floatDest(&aggregate.AvgBasisPercent),
		floatDest(&aggregate.AvgOrderbookImbalancePercent),
		&aggregate.ExchangeCount,
		&aggregate.AvailableExchanges,
		&aggregate.RawByExchange,
		&aggregate.WindowMetrics,
		&aggregate.AnomalyFlags,
		&aggregate.Quality,
	); err != nil {
		return fmt.Errorf("scan overview: %w", err)
	}
	symbol.BaseAsset = baseAsset.String
	symbol.QuoteAsset = quoteAsset.String
	symbol.MarketType = marketType.String
	symbol.Category = marketType.String
	symbol.CmcRank = int(cmcRank.Int64)
	symbol.Markets = ensureJSONArray(marketsRaw)

	if snapshotAt.Valid {
		aggregate.SnapshotTime = snapshotAt.Time
		aggregate.Price = firstFloat(aggregate.PriceWeighted, aggregate.PriceAvg)
	}
	aggregate.Freshness = freshness(aggregate.SnapshotTime)
	aggregate.AvailableExchanges = ensureJSONArray(aggregate.AvailableExchanges)
	aggregate.RawByExchange = ensureJSON(aggregate.RawByExchange)
	aggregate.WindowMetrics = ensureJSON(aggregate.WindowMetrics)
	aggregate.AnomalyFlags = ensureJSONArray(aggregate.AnomalyFlags)
	aggregate.Quality = ensureJSON(aggregate.Quality)

	item.Symbol = symbol
	item.Market = aggregate
	return nil
}

func overviewSort(sort string) string {
	switch sort {
	case "symbol":
		return "s.symbol"
	case "rank", "cmc_rank":
		return "s.cmc_rank"
	case "volume":
		return "latest.total_volume_24h"
	case "oi", "open_interest":
		return "latest.total_open_interest"
	case "funding":
		return "latest.avg_funding_rate"
	case "price":
		return "COALESCE(latest.price_weighted, latest.price_avg)"
	case "freshness":
		return "latest.snapshot_time"
	default:
		return "s.cmc_rank"
	}
}

func minPositive(value int, fallback int) int {
	if value <= 0 {
		return fallback
	}
	if value > fallback {
		return fallback
	}
	return value
}
