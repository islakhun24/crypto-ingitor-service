package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"aggregator-services/internal/aggregation"
)

type AggregateRepository struct {
	db      *sql.DB
	options AggregateOptions
}

func NewAggregateRepository(db *sql.DB) *AggregateRepository {
	return NewAggregateRepositoryWithOptions(db, AggregateOptions{})
}

func NewAggregateRepositoryWithOptions(db *sql.DB, options AggregateOptions) *AggregateRepository {
	return &AggregateRepository{db: db, options: normalizeAggregateOptions(options)}
}

func (r *AggregateRepository) BuildLatest(ctx context.Context, symbolID int64, snapshotTime time.Time) (aggregation.DerivativeAggregateSnapshot, error) {
	if snapshotTime.IsZero() {
		snapshotTime = time.Now()
	}
	snapshotTime = snapshotTime.UTC()
	staleCutoff := snapshotTime.Add(-r.options.MaxSnapshotAge)
	quality := newAggregateQualityMetadata(snapshotTime, staleCutoff, r.options.MaxSnapshotAge)

	rows, err := r.db.QueryContext(ctx, `
		WITH latest AS (
		    SELECT DISTINCT ON (exchange)
		           exchange, last_price, mark_price, volume_24h, quote_volume_24h,
		           open_interest, funding_rate, raw_data, snapshot_time
		    FROM derivative_market_snapshots
		    WHERE symbol_id = $1
		      AND snapshot_time <= $2
		      AND snapshot_time >= $3
		    ORDER BY exchange, snapshot_time DESC
		)
		SELECT exchange, last_price, mark_price, volume_24h, quote_volume_24h,
		       open_interest, funding_rate, raw_data, snapshot_time
		FROM latest
	`, symbolID, snapshotTime, staleCutoff)
	if err != nil {
		return aggregation.DerivativeAggregateSnapshot{}, fmt.Errorf("query latest market snapshots: %w", err)
	}
	defer rows.Close()

	var (
		exchanges         []string
		rawByExchange     = map[string]json.RawMessage{}
		priceSum          float64
		priceCount        int
		weightedNumerator float64
		weightedDenom     float64
		totalVolume       float64
		totalQuoteVolume  float64
		totalOI           float64
		totalOIValue      float64
		hasOIValue        bool
		fundingSum        float64
		fundingCount      int
		minFunding        *float64
		maxFunding        *float64
	)

	for rows.Next() {
		var (
			exchange     string
			lastPrice    sql.NullFloat64
			markPrice    sql.NullFloat64
			volume       sql.NullFloat64
			quoteVolume  sql.NullFloat64
			openInterest sql.NullFloat64
			fundingRate  sql.NullFloat64
			rawData      json.RawMessage
			snapshotAt   time.Time
		)
		if err := rows.Scan(&exchange, &lastPrice, &markPrice, &volume, &quoteVolume, &openInterest, &fundingRate, &rawData, &snapshotAt); err != nil {
			return aggregation.DerivativeAggregateSnapshot{}, fmt.Errorf("scan latest market snapshot: %w", err)
		}

		if snapshotAt.Before(staleCutoff) {
			quality.Skip(exchange, "stale_snapshot")
			continue
		}
		if rawJSONMarkedDegraded(rawData) {
			quality.Skip(exchange, "degraded_raw_data")
			continue
		}

		exchanges = append(exchanges, exchange)
		quality.Include(exchange)
		rawByExchange[exchange] = ensureJSON(rawData)

		price := lastPrice
		if !price.Valid {
			price = markPrice
		}
		if price.Valid && price.Float64 > 0 {
			priceSum += price.Float64
			priceCount++
			if quoteVolume.Valid && quoteVolume.Float64 > 0 {
				weightedNumerator += price.Float64 * quoteVolume.Float64
				weightedDenom += quoteVolume.Float64
			}
		} else if price.Valid {
			quality.Invalid(exchange, "price")
		}
		if volume.Valid && volume.Float64 >= 0 {
			totalVolume += volume.Float64
		} else if volume.Valid {
			quality.Invalid(exchange, "volume_24h")
		}
		if quoteVolume.Valid && quoteVolume.Float64 >= 0 {
			totalQuoteVolume += quoteVolume.Float64
		} else if quoteVolume.Valid {
			quality.Invalid(exchange, "quote_volume_24h")
		}
		if openInterest.Valid && openInterest.Float64 >= 0 {
			totalOI += openInterest.Float64
			if price.Valid && price.Float64 > 0 {
				totalOIValue += openInterest.Float64 * price.Float64
				hasOIValue = true
			}
		} else if openInterest.Valid {
			quality.Invalid(exchange, "open_interest")
		}
		if fundingRate.Valid {
			value := fundingRate.Float64
			fundingSum += value
			fundingCount++
			if minFunding == nil || value < *minFunding {
				minFunding = &value
			}
			if maxFunding == nil || value > *maxFunding {
				maxFunding = &value
			}
		}
	}
	if err := rows.Err(); err != nil {
		return aggregation.DerivativeAggregateSnapshot{}, fmt.Errorf("iterate latest market snapshots: %w", err)
	}

	available, _ := json.Marshal(exchanges)
	raw, _ := json.Marshal(rawByExchange)

	result := aggregation.DerivativeAggregateSnapshot{
		SymbolID:            symbolID,
		SnapshotTime:        snapshotTime.UTC(),
		ExchangeCount:       len(exchanges),
		TotalVolume24h:      floatPtr(totalVolume),
		TotalQuoteVolume24h: floatPtr(totalQuoteVolume),
		TotalOpenInterest:   floatPtr(totalOI),
		MinFundingRate:      minFunding,
		MaxFundingRate:      maxFunding,
		AvailableExchanges:  available,
		RawByExchange:       raw,
	}
	if priceCount > 0 {
		result.PriceAvg = floatPtr(priceSum / float64(priceCount))
	}
	if weightedDenom > 0 {
		result.PriceWeighted = floatPtr(weightedNumerator / weightedDenom)
	}
	if hasOIValue {
		result.TotalOpenInterestValue = floatPtr(totalOIValue)
	}
	if fundingCount > 0 {
		result.AvgFundingRate = floatPtr(fundingSum / float64(fundingCount))
	}

	if err := r.enrichAdvanced(ctx, &result); err != nil {
		return aggregation.DerivativeAggregateSnapshot{}, err
	}
	windowMetrics, err := r.BuildWindowMetrics(ctx, symbolID, snapshotTime)
	if err != nil {
		return aggregation.DerivativeAggregateSnapshot{}, err
	}
	windowRaw, _ := json.Marshal(windowMetrics)
	metricsRaw, _ := json.Marshal(map[string]any{
		"windows": windowMetrics,
	})
	result.WindowMetrics = windowRaw
	result.Metrics = metricsRaw
	result.AnomalyFlags = buildAnomalyFlags(result, windowMetrics)
	result.QualityMetadata = quality.Marshal()

	return result, nil
}

func (r *AggregateRepository) Upsert(ctx context.Context, snapshot aggregation.DerivativeAggregateSnapshot) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO derivative_aggregated_snapshots (
		    symbol_id, snapshot_time, exchange_count, weighted_price,
		    price_avg, price_weighted, total_volume_24h, total_quote_volume_24h,
		    total_open_interest, total_open_interest_value, avg_funding_rate,
		    min_funding_rate, max_funding_rate, available_exchanges,
		    raw_by_exchange, raw_data, total_taker_buy_volume,
		    total_taker_sell_volume, total_buy_sell_delta, total_cvd,
		    total_long_liquidation_usd, total_short_liquidation_usd,
		    total_liquidation_usd, avg_basis_percent,
		    avg_orderbook_imbalance_percent, total_bid_depth_usd,
		    total_ask_depth_usd, window_metrics, metrics,
		    quality_metadata, anomaly_flags
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31)
		ON CONFLICT (symbol_id, snapshot_time) DO UPDATE SET
		    exchange_count = EXCLUDED.exchange_count,
		    weighted_price = EXCLUDED.weighted_price,
		    price_avg = EXCLUDED.price_avg,
		    price_weighted = EXCLUDED.price_weighted,
		    total_volume_24h = EXCLUDED.total_volume_24h,
		    total_quote_volume_24h = EXCLUDED.total_quote_volume_24h,
		    total_open_interest = EXCLUDED.total_open_interest,
		    total_open_interest_value = EXCLUDED.total_open_interest_value,
		    avg_funding_rate = EXCLUDED.avg_funding_rate,
		    min_funding_rate = EXCLUDED.min_funding_rate,
		    max_funding_rate = EXCLUDED.max_funding_rate,
		    available_exchanges = EXCLUDED.available_exchanges,
		    raw_by_exchange = EXCLUDED.raw_by_exchange,
		    raw_data = EXCLUDED.raw_data,
		    total_taker_buy_volume = EXCLUDED.total_taker_buy_volume,
		    total_taker_sell_volume = EXCLUDED.total_taker_sell_volume,
		    total_buy_sell_delta = EXCLUDED.total_buy_sell_delta,
		    total_cvd = EXCLUDED.total_cvd,
		    total_long_liquidation_usd = EXCLUDED.total_long_liquidation_usd,
		    total_short_liquidation_usd = EXCLUDED.total_short_liquidation_usd,
		    total_liquidation_usd = EXCLUDED.total_liquidation_usd,
		    avg_basis_percent = EXCLUDED.avg_basis_percent,
		    avg_orderbook_imbalance_percent = EXCLUDED.avg_orderbook_imbalance_percent,
		    total_bid_depth_usd = EXCLUDED.total_bid_depth_usd,
		    total_ask_depth_usd = EXCLUDED.total_ask_depth_usd,
		    window_metrics = EXCLUDED.window_metrics,
		    metrics = EXCLUDED.metrics,
		    quality_metadata = EXCLUDED.quality_metadata,
		    anomaly_flags = EXCLUDED.anomaly_flags
	`,
		snapshot.SymbolID,
		snapshot.SnapshotTime,
		snapshot.ExchangeCount,
		nullableFloat(snapshot.PriceWeighted),
		nullableFloat(snapshot.PriceAvg),
		nullableFloat(snapshot.PriceWeighted),
		nullableFloat(snapshot.TotalVolume24h),
		nullableFloat(snapshot.TotalQuoteVolume24h),
		nullableFloat(snapshot.TotalOpenInterest),
		nullableFloat(snapshot.TotalOpenInterestValue),
		nullableFloat(snapshot.AvgFundingRate),
		nullableFloat(snapshot.MinFundingRate),
		nullableFloat(snapshot.MaxFundingRate),
		ensureJSON(snapshot.AvailableExchanges),
		ensureJSON(snapshot.RawByExchange),
		ensureJSON(snapshot.RawByExchange),
		nullableFloat(snapshot.TotalTakerBuyVolume),
		nullableFloat(snapshot.TotalTakerSellVolume),
		nullableFloat(snapshot.TotalBuySellDelta),
		nullableFloat(snapshot.TotalCVD),
		nullableFloat(snapshot.TotalLongLiquidationUSD),
		nullableFloat(snapshot.TotalShortLiquidationUSD),
		nullableFloat(snapshot.TotalLiquidationUSD),
		nullableFloat(snapshot.AvgBasisPercent),
		nullableFloat(snapshot.AvgOrderbookImbalancePercent),
		nullableFloat(snapshot.TotalBidDepthUSD),
		nullableFloat(snapshot.TotalAskDepthUSD),
		ensureJSON(snapshot.WindowMetrics),
		ensureJSON(snapshot.Metrics),
		ensureJSON(snapshot.QualityMetadata),
		ensureJSON(snapshot.AnomalyFlags),
	)
	if err != nil {
		return fmt.Errorf("upsert aggregate snapshot: %w", err)
	}

	return nil
}

func (r *AggregateRepository) enrichAdvanced(ctx context.Context, snapshot *aggregation.DerivativeAggregateSnapshot) error {
	staleCutoff := snapshot.SnapshotTime.Add(-r.options.MaxSnapshotAge)

	var takerBuy, takerSell, takerDelta sql.NullFloat64
	if err := r.db.QueryRowContext(ctx, `
		WITH latest AS (
		    SELECT DISTINCT ON (exchange)
		           taker_buy_volume, taker_sell_volume, buy_sell_delta
		    FROM taker_flow_snapshots
		    WHERE symbol_id = $1
		      AND snapshot_time <= $2
		      AND snapshot_time >= $3
		    ORDER BY exchange, snapshot_time DESC
		)
		SELECT COALESCE(SUM(CASE WHEN taker_buy_volume >= 0 THEN taker_buy_volume ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN taker_sell_volume >= 0 THEN taker_sell_volume ELSE 0 END), 0),
		       COALESCE(SUM(buy_sell_delta), 0)
		FROM latest
	`, snapshot.SymbolID, snapshot.SnapshotTime, staleCutoff).Scan(&takerBuy, &takerSell, &takerDelta); err != nil {
		return fmt.Errorf("query aggregate taker flow: %w", err)
	}
	snapshot.TotalTakerBuyVolume = ptrFromNull(takerBuy)
	snapshot.TotalTakerSellVolume = ptrFromNull(takerSell)
	snapshot.TotalBuySellDelta = ptrFromNull(takerDelta)

	var totalCVD sql.NullFloat64
	if err := r.db.QueryRowContext(ctx, `
		WITH latest AS (
		    SELECT DISTINCT ON (exchange)
		           cvd_value
		    FROM cvd_snapshots
		    WHERE symbol_id = $1
		      AND snapshot_time <= $2
		      AND snapshot_time >= $3
		    ORDER BY exchange, snapshot_time DESC
		)
		SELECT COALESCE(SUM(cvd_value), 0)
		FROM latest
	`, snapshot.SymbolID, snapshot.SnapshotTime, staleCutoff).Scan(&totalCVD); err != nil {
		return fmt.Errorf("query aggregate cvd: %w", err)
	}
	snapshot.TotalCVD = ptrFromNull(totalCVD)

	var longLiq, shortLiq, totalLiq sql.NullFloat64
	if err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(CASE WHEN long_liquidation_usd >= 0 THEN long_liquidation_usd ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN short_liquidation_usd >= 0 THEN short_liquidation_usd ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN total_liquidation_usd >= 0 THEN total_liquidation_usd ELSE 0 END), 0)
		FROM liquidation_aggregates
		WHERE symbol_id = $1
		  AND bucket_time >= $2 - interval '5 minutes'
		  AND bucket_time <= $2
	`, snapshot.SymbolID, snapshot.SnapshotTime).Scan(&longLiq, &shortLiq, &totalLiq); err != nil {
		return fmt.Errorf("query aggregate liquidation: %w", err)
	}
	snapshot.TotalLongLiquidationUSD = ptrFromNull(longLiq)
	snapshot.TotalShortLiquidationUSD = ptrFromNull(shortLiq)
	snapshot.TotalLiquidationUSD = ptrFromNull(totalLiq)

	var avgBasis sql.NullFloat64
	if err := r.db.QueryRowContext(ctx, `
		WITH latest AS (
		    SELECT DISTINCT ON (exchange)
		           basis_percent
		    FROM basis_premium_snapshots
		    WHERE symbol_id = $1
		      AND snapshot_time <= $2
		      AND snapshot_time >= $3
		    ORDER BY exchange, snapshot_time DESC
		)
		SELECT AVG(basis_percent)
		FROM latest
	`, snapshot.SymbolID, snapshot.SnapshotTime, staleCutoff).Scan(&avgBasis); err != nil {
		return fmt.Errorf("query aggregate basis: %w", err)
	}
	snapshot.AvgBasisPercent = ptrFromNull(avgBasis)

	var avgImbalance, bidDepth, askDepth sql.NullFloat64
	if err := r.db.QueryRowContext(ctx, `
		WITH latest AS (
		    SELECT DISTINCT ON (exchange)
		           imbalance_percent, bid_depth_usd, ask_depth_usd
		    FROM orderbook_imbalance_snapshots
		    WHERE symbol_id = $1
		      AND snapshot_time <= $2
		      AND snapshot_time >= $3
		    ORDER BY exchange, snapshot_time DESC
		)
		SELECT AVG(imbalance_percent),
		       COALESCE(SUM(CASE WHEN bid_depth_usd >= 0 THEN bid_depth_usd ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN ask_depth_usd >= 0 THEN ask_depth_usd ELSE 0 END), 0)
		FROM latest
	`, snapshot.SymbolID, snapshot.SnapshotTime, staleCutoff).Scan(&avgImbalance, &bidDepth, &askDepth); err != nil {
		return fmt.Errorf("query aggregate orderbook: %w", err)
	}
	snapshot.AvgOrderbookImbalancePercent = ptrFromNull(avgImbalance)
	snapshot.TotalBidDepthUSD = ptrFromNull(bidDepth)
	snapshot.TotalAskDepthUSD = ptrFromNull(askDepth)

	return nil
}

func ptrFromNull(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	return floatPtr(value.Float64)
}

func floatPtr(value float64) *float64 {
	return &value
}
