package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"aggregator-services/internal/aggregation"
)

type marketAggregatePoint struct {
	Price         *float64
	Volume        *float64
	OpenInterest  *float64
	ExchangeCount int
}

type aggregateCandle struct {
	OpenTime time.Time
	Open     float64
	High     float64
	Low      float64
	Close    float64
}

type exchangeMetric struct {
	Exchange     string   `json:"exchange"`
	Price        *float64 `json:"price,omitempty"`
	Volume       *float64 `json:"volume_24h,omitempty"`
	OpenInterest *float64 `json:"open_interest,omitempty"`
	FundingRate  *float64 `json:"funding_rate,omitempty"`
}

func (r *AggregateRepository) BuildWindowMetrics(ctx context.Context, symbolID int64, snapshotTime time.Time) ([]aggregation.AnalyticsWindowMetric, error) {
	metrics := make([]aggregation.AnalyticsWindowMetric, 0, len(r.options.Windows))
	current, err := r.marketAggregateAt(ctx, symbolID, snapshotTime)
	if err != nil {
		return nil, err
	}
	cvdNow, err := r.cvdAggregateAt(ctx, symbolID, snapshotTime)
	if err != nil {
		return nil, err
	}

	for _, window := range r.options.Windows {
		start := snapshotTime.Add(-window.Duration)
		metric := aggregation.AnalyticsWindowMetric{
			Window:    window.Label,
			StartTime: start,
			EndTime:   snapshotTime,
		}

		previous, err := r.marketAggregateAt(ctx, symbolID, start)
		if err != nil {
			return nil, err
		}
		if current.Price != nil && previous.Price != nil {
			change := *current.Price - *previous.Price
			metric.PriceChange = &change
			if *previous.Price != 0 {
				pct := (change / *previous.Price) * 100
				metric.PriceChangePct = &pct
			}
		}
		if current.Volume != nil && previous.Volume != nil {
			change := *current.Volume - *previous.Volume
			metric.VolumeChange = &change
		}
		if current.OpenInterest != nil && previous.OpenInterest != nil {
			change := *current.OpenInterest - *previous.OpenInterest
			metric.OIChange = &change
		}

		fundingAvg, fundingMin, fundingMax, err := r.windowFundingStats(ctx, symbolID, start, snapshotTime)
		if err != nil {
			return nil, err
		}
		metric.FundingAvg = fundingAvg
		metric.FundingMin = fundingMin
		metric.FundingMax = fundingMax

		takerDelta, err := r.windowTakerDelta(ctx, symbolID, start, snapshotTime)
		if err != nil {
			return nil, err
		}
		metric.TakerDelta = takerDelta

		cvdStart, err := r.cvdAggregateAt(ctx, symbolID, start)
		if err != nil {
			return nil, err
		}
		if cvdNow != nil && cvdStart != nil {
			change := *cvdNow - *cvdStart
			metric.CVDChange = &change
		}

		liquidationSum, err := r.windowLiquidationSum(ctx, symbolID, start, snapshotTime)
		if err != nil {
			return nil, err
		}
		metric.LiquidationSumUSD = liquidationSum

		basisAvg, basisMin, basisMax, err := r.windowBasisStats(ctx, symbolID, start, snapshotTime)
		if err != nil {
			return nil, err
		}
		metric.BasisAvg = basisAvg
		metric.BasisMin = basisMin
		metric.BasisMax = basisMax

		quality, _ := json.Marshal(map[string]any{
			"current_exchange_count":   current.ExchangeCount,
			"previous_exchange_count":  previous.ExchangeCount,
			"max_snapshot_age_seconds": int64(r.options.MaxSnapshotAge.Seconds()),
		})
		metric.Quality = quality
		metrics = append(metrics, metric)
	}

	return metrics, nil
}

func (r *AggregateRepository) BuildAnalytics(ctx context.Context, symbolID int64, snapshotTime time.Time) (aggregation.AnalyticsSnapshotSet, error) {
	structures, err := r.BuildMarketStructureSnapshots(ctx, symbolID, snapshotTime)
	if err != nil {
		return aggregation.AnalyticsSnapshotSet{}, err
	}
	volatility, err := r.BuildVolatilitySnapshots(ctx, symbolID, snapshotTime)
	if err != nil {
		return aggregation.AnalyticsSnapshotSet{}, err
	}
	divergence, err := r.BuildExchangeDivergenceSnapshot(ctx, symbolID, snapshotTime)
	if err != nil {
		return aggregation.AnalyticsSnapshotSet{}, err
	}

	return aggregation.AnalyticsSnapshotSet{
		MarketStructures: structures,
		Volatility:       volatility,
		Divergence:       divergence,
	}, nil
}

func (r *AggregateRepository) UpsertAnalytics(ctx context.Context, set aggregation.AnalyticsSnapshotSet) error {
	for _, item := range set.MarketStructures {
		if err := r.upsertMarketStructure(ctx, item); err != nil {
			return err
		}
	}
	for _, item := range set.Volatility {
		if err := r.upsertVolatility(ctx, item); err != nil {
			return err
		}
	}
	if set.Divergence != nil {
		if err := r.upsertExchangeDivergence(ctx, *set.Divergence); err != nil {
			return err
		}
	}

	return nil
}

func (r *AggregateRepository) BuildMarketStructureSnapshots(ctx context.Context, symbolID int64, snapshotTime time.Time) ([]aggregation.MarketStructureSnapshot, error) {
	candles, err := r.loadAggregateCandles(ctx, symbolID, snapshotTime.Add(-24*time.Hour), snapshotTime)
	if err != nil {
		return nil, err
	}

	snapshots := make([]aggregation.MarketStructureSnapshot, 0, len(r.options.Windows))
	for _, window := range r.options.Windows {
		start := snapshotTime.Add(-window.Duration)
		snapshot := computeMarketStructure(symbolID, window.Label, snapshotTime, filterCandles(candles, start, snapshotTime))
		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
}

func (r *AggregateRepository) BuildVolatilitySnapshots(ctx context.Context, symbolID int64, snapshotTime time.Time) ([]aggregation.VolatilitySnapshot, error) {
	candles7d, err := r.loadAggregateCandles(ctx, symbolID, snapshotTime.Add(-7*24*time.Hour), snapshotTime)
	if err != nil {
		return nil, err
	}
	candles24h := filterCandles(candles7d, snapshotTime.Add(-24*time.Hour), snapshotTime)

	snapshots := make([]aggregation.VolatilitySnapshot, 0, len(r.options.Windows))
	for _, window := range r.options.Windows {
		start := snapshotTime.Add(-window.Duration)
		snapshot := computeVolatility(symbolID, window.Label, snapshotTime, filterCandles(candles7d, start, snapshotTime), candles24h, candles7d)
		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
}

func (r *AggregateRepository) BuildExchangeDivergenceSnapshot(ctx context.Context, symbolID int64, snapshotTime time.Time) (*aggregation.ExchangeDivergenceSnapshot, error) {
	metrics, err := r.latestExchangeMetrics(ctx, symbolID, snapshotTime)
	if err != nil {
		return nil, err
	}
	if len(metrics) < 2 {
		return nil, nil
	}

	priceMin, priceMax, weakest, strongest := minMaxMetric(metrics, func(item exchangeMetric) *float64 { return item.Price })
	oiMin, oiMax, _, _ := minMaxMetric(metrics, func(item exchangeMetric) *float64 { return item.OpenInterest })
	fundingMin, fundingMax, _, _ := minMaxMetric(metrics, func(item exchangeMetric) *float64 { return item.FundingRate })
	volumeMin, volumeMax, _, _ := minMaxMetric(metrics, func(item exchangeMetric) *float64 { return item.Volume })

	var referenceValue, comparedValue, divergenceAbs, divergenceBPS *float64
	priceSpreadPct := spreadPercent(priceMin, priceMax)
	if priceMin != nil && priceMax != nil {
		minValue := *priceMin
		maxValue := *priceMax
		diff := maxValue - minValue
		bps := 0.0
		if minValue != 0 {
			bps = (diff / minValue) * 10000
		}
		referenceValue = &minValue
		comparedValue = &maxValue
		divergenceAbs = &diff
		divergenceBPS = &bps
	}

	rawByExchange := map[string]exchangeMetric{}
	for _, item := range metrics {
		rawByExchange[item.Exchange] = item
	}
	raw, _ := json.Marshal(rawByExchange)
	metadata, _ := json.Marshal(map[string]any{
		"exchange_count":             len(metrics),
		"max_snapshot_age_seconds":   int64(r.options.MaxSnapshotAge.Seconds()),
		"quality_filtering_applied":  true,
		"non_signal_analytics_layer": true,
	})

	return &aggregation.ExchangeDivergenceSnapshot{
		SymbolID:            symbolID,
		DataType:            "market_summary",
		SnapshotTime:        snapshotTime,
		ReferenceExchange:   "aggregate",
		ComparedExchange:    "all",
		ReferenceValue:      referenceValue,
		ComparedValue:       comparedValue,
		DivergenceAbs:       divergenceAbs,
		DivergenceBPS:       divergenceBPS,
		PriceMin:            priceMin,
		PriceMax:            priceMax,
		PriceSpreadPercent:  priceSpreadPct,
		OIMin:               oiMin,
		OIMax:               oiMax,
		OISpreadPercent:     spreadPercent(oiMin, oiMax),
		FundingMin:          fundingMin,
		FundingMax:          fundingMax,
		FundingSpread:       simpleSpread(fundingMin, fundingMax),
		VolumeMin:           volumeMin,
		VolumeMax:           volumeMax,
		VolumeSpreadPercent: spreadPercent(volumeMin, volumeMax),
		StrongestExchange:   strongest,
		WeakestExchange:     weakest,
		RawByExchange:       raw,
		Metadata:            metadata,
	}, nil
}

func (r *AggregateRepository) marketAggregateAt(ctx context.Context, symbolID int64, at time.Time) (marketAggregatePoint, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH latest AS (
		    SELECT DISTINCT ON (exchange)
		           exchange, last_price, mark_price, volume_24h,
		           open_interest, raw_data
		    FROM derivative_market_snapshots
		    WHERE symbol_id = $1
		      AND snapshot_time <= $2
		      AND snapshot_time >= $3
		    ORDER BY exchange, snapshot_time DESC
		)
		SELECT exchange, last_price, mark_price, volume_24h, open_interest, raw_data
		FROM latest
	`, symbolID, at, at.Add(-r.options.MaxSnapshotAge))
	if err != nil {
		return marketAggregatePoint{}, fmt.Errorf("query aggregate market point: %w", err)
	}
	defer rows.Close()

	var priceSum float64
	var priceCount int
	var totalVolume float64
	var hasVolume bool
	var totalOI float64
	var hasOI bool
	var exchangeCount int

	for rows.Next() {
		var exchange string
		var lastPrice, markPrice, volume, openInterest sql.NullFloat64
		var rawData json.RawMessage
		if err := rows.Scan(&exchange, &lastPrice, &markPrice, &volume, &openInterest, &rawData); err != nil {
			return marketAggregatePoint{}, fmt.Errorf("scan aggregate market point: %w", err)
		}
		if rawJSONMarkedDegraded(rawData) {
			continue
		}

		exchangeCount++
		price := lastPrice
		if !price.Valid {
			price = markPrice
		}
		if price.Valid && price.Float64 > 0 {
			priceSum += price.Float64
			priceCount++
		}
		if volume.Valid && volume.Float64 >= 0 {
			totalVolume += volume.Float64
			hasVolume = true
		}
		if openInterest.Valid && openInterest.Float64 >= 0 {
			totalOI += openInterest.Float64
			hasOI = true
		}
	}
	if err := rows.Err(); err != nil {
		return marketAggregatePoint{}, fmt.Errorf("iterate aggregate market point: %w", err)
	}

	point := marketAggregatePoint{ExchangeCount: exchangeCount}
	if priceCount > 0 {
		point.Price = floatPtr(priceSum / float64(priceCount))
	}
	if hasVolume {
		point.Volume = floatPtr(totalVolume)
	}
	if hasOI {
		point.OpenInterest = floatPtr(totalOI)
	}

	return point, nil
}

func (r *AggregateRepository) windowFundingStats(ctx context.Context, symbolID int64, start time.Time, end time.Time) (*float64, *float64, *float64, error) {
	var avg, min, max sql.NullFloat64
	if err := r.db.QueryRowContext(ctx, `
		SELECT AVG(funding_rate), MIN(funding_rate), MAX(funding_rate)
		FROM funding_rate_snapshots
		WHERE symbol_id = $1
		  AND snapshot_time > $2
		  AND snapshot_time <= $3
		  AND lower(COALESCE(raw_data->>'quality_status', raw_data->>'status', raw_data->>'normalized_status', 'ok')) NOT IN ('degraded', 'failed', 'invalid', 'unhealthy')
	`, symbolID, start, end).Scan(&avg, &min, &max); err != nil {
		return nil, nil, nil, fmt.Errorf("query funding window stats: %w", err)
	}

	return ptrFromNull(avg), ptrFromNull(min), ptrFromNull(max), nil
}

func (r *AggregateRepository) windowTakerDelta(ctx context.Context, symbolID int64, start time.Time, end time.Time) (*float64, error) {
	var value sql.NullFloat64
	if err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(
		    CASE
		        WHEN buy_sell_delta_quote IS NOT NULL THEN buy_sell_delta_quote
		        WHEN buy_sell_delta IS NOT NULL THEN buy_sell_delta
		        WHEN taker_buy_quote_volume IS NOT NULL AND taker_sell_quote_volume IS NOT NULL THEN taker_buy_quote_volume - taker_sell_quote_volume
		        WHEN taker_buy_volume IS NOT NULL AND taker_sell_volume IS NOT NULL THEN taker_buy_volume - taker_sell_volume
		        ELSE 0
		    END
		), 0)
		FROM taker_flow_snapshots
		WHERE symbol_id = $1
		  AND snapshot_time > $2
		  AND snapshot_time <= $3
		  AND lower(COALESCE(raw_data->>'quality_status', raw_data->>'status', raw_data->>'normalized_status', 'ok')) NOT IN ('degraded', 'failed', 'invalid', 'unhealthy')
	`, symbolID, start, end).Scan(&value); err != nil {
		return nil, fmt.Errorf("query taker delta window: %w", err)
	}

	return ptrFromNull(value), nil
}

func (r *AggregateRepository) cvdAggregateAt(ctx context.Context, symbolID int64, at time.Time) (*float64, error) {
	var value sql.NullFloat64
	if err := r.db.QueryRowContext(ctx, `
		WITH latest AS (
		    SELECT DISTINCT ON (exchange)
		           cvd_value
		    FROM cvd_snapshots
		    WHERE symbol_id = $1
		      AND snapshot_time <= $2
		      AND snapshot_time >= $3
		      AND lower(COALESCE(raw_data->>'quality_status', raw_data->>'status', raw_data->>'normalized_status', 'ok')) NOT IN ('degraded', 'failed', 'invalid', 'unhealthy')
		    ORDER BY exchange, snapshot_time DESC
		)
		SELECT COALESCE(SUM(cvd_value), 0)
		FROM latest
	`, symbolID, at, at.Add(-r.options.MaxSnapshotAge)).Scan(&value); err != nil {
		return nil, fmt.Errorf("query cvd aggregate point: %w", err)
	}

	return ptrFromNull(value), nil
}

func (r *AggregateRepository) windowLiquidationSum(ctx context.Context, symbolID int64, start time.Time, end time.Time) (*float64, error) {
	var value sql.NullFloat64
	if err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(CASE WHEN total_liquidation_usd >= 0 THEN total_liquidation_usd ELSE 0 END), 0)
		FROM liquidation_aggregates
		WHERE symbol_id = $1
		  AND bucket_time > $2
		  AND bucket_time <= $3
		  AND lower(COALESCE(raw_data->>'quality_status', raw_data->>'status', raw_data->>'normalized_status', 'ok')) NOT IN ('degraded', 'failed', 'invalid', 'unhealthy')
	`, symbolID, start, end).Scan(&value); err != nil {
		return nil, fmt.Errorf("query liquidation window: %w", err)
	}

	return ptrFromNull(value), nil
}

func (r *AggregateRepository) windowBasisStats(ctx context.Context, symbolID int64, start time.Time, end time.Time) (*float64, *float64, *float64, error) {
	var avg, min, max sql.NullFloat64
	if err := r.db.QueryRowContext(ctx, `
		SELECT AVG(basis_percent), MIN(basis_percent), MAX(basis_percent)
		FROM basis_premium_snapshots
		WHERE symbol_id = $1
		  AND snapshot_time > $2
		  AND snapshot_time <= $3
		  AND lower(COALESCE(raw_data->>'quality_status', raw_data->>'status', raw_data->>'normalized_status', 'ok')) NOT IN ('degraded', 'failed', 'invalid', 'unhealthy')
	`, symbolID, start, end).Scan(&avg, &min, &max); err != nil {
		return nil, nil, nil, fmt.Errorf("query basis window stats: %w", err)
	}

	return ptrFromNull(avg), ptrFromNull(min), ptrFromNull(max), nil
}

func (r *AggregateRepository) loadAggregateCandles(ctx context.Context, symbolID int64, start time.Time, end time.Time) ([]aggregateCandle, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT open_time,
		       AVG(open_price)::float8,
		       MAX(high_price)::float8,
		       MIN(low_price)::float8,
		       AVG(close_price)::float8
		FROM derivative_klines
		WHERE symbol_id = $1
		  AND "interval" = '5m'
		  AND open_time >= $2
		  AND open_time <= $3
		  AND is_closed = true
		  AND open_price > 0
		  AND high_price > 0
		  AND low_price > 0
		  AND close_price > 0
		  AND lower(COALESCE(raw_data->>'quality_status', raw_data->>'status', raw_data->>'normalized_status', 'ok')) NOT IN ('degraded', 'failed', 'invalid', 'unhealthy')
		GROUP BY open_time
		ORDER BY open_time ASC
	`, symbolID, start, end)
	if err != nil {
		return nil, fmt.Errorf("query aggregate candles: %w", err)
	}
	defer rows.Close()

	var candles []aggregateCandle
	for rows.Next() {
		var candle aggregateCandle
		if err := rows.Scan(&candle.OpenTime, &candle.Open, &candle.High, &candle.Low, &candle.Close); err != nil {
			return nil, fmt.Errorf("scan aggregate candle: %w", err)
		}
		candles = append(candles, candle)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate aggregate candles: %w", err)
	}

	return candles, nil
}

func (r *AggregateRepository) latestExchangeMetrics(ctx context.Context, symbolID int64, snapshotTime time.Time) ([]exchangeMetric, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH latest AS (
		    SELECT DISTINCT ON (exchange)
		           exchange, last_price, mark_price, volume_24h,
		           open_interest, funding_rate, raw_data
		    FROM derivative_market_snapshots
		    WHERE symbol_id = $1
		      AND snapshot_time <= $2
		      AND snapshot_time >= $3
		    ORDER BY exchange, snapshot_time DESC
		)
		SELECT exchange, last_price, mark_price, volume_24h, open_interest, funding_rate, raw_data
		FROM latest
	`, symbolID, snapshotTime, snapshotTime.Add(-r.options.MaxSnapshotAge))
	if err != nil {
		return nil, fmt.Errorf("query latest exchange metrics: %w", err)
	}
	defer rows.Close()

	var metrics []exchangeMetric
	for rows.Next() {
		var item exchangeMetric
		var lastPrice, markPrice, volume, openInterest, fundingRate sql.NullFloat64
		var rawData json.RawMessage
		if err := rows.Scan(&item.Exchange, &lastPrice, &markPrice, &volume, &openInterest, &fundingRate, &rawData); err != nil {
			return nil, fmt.Errorf("scan latest exchange metrics: %w", err)
		}
		if rawJSONMarkedDegraded(rawData) {
			continue
		}
		price := lastPrice
		if !price.Valid {
			price = markPrice
		}
		if price.Valid && price.Float64 > 0 {
			item.Price = floatPtr(price.Float64)
		}
		if volume.Valid && volume.Float64 >= 0 {
			item.Volume = floatPtr(volume.Float64)
		}
		if openInterest.Valid && openInterest.Float64 >= 0 {
			item.OpenInterest = floatPtr(openInterest.Float64)
		}
		if fundingRate.Valid {
			item.FundingRate = floatPtr(fundingRate.Float64)
		}
		metrics = append(metrics, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate latest exchange metrics: %w", err)
	}

	return metrics, nil
}

func computeMarketStructure(symbolID int64, period string, snapshotTime time.Time, candles []aggregateCandle) aggregation.MarketStructureSnapshot {
	emptyLevels := json.RawMessage(`[]`)
	snapshot := aggregation.MarketStructureSnapshot{
		SymbolID:         symbolID,
		Exchange:         "aggregate",
		Period:           period,
		SnapshotTime:     snapshotTime,
		TrendDirection:   "unknown",
		StructureState:   "unknown",
		SupportLevels:    emptyLevels,
		ResistanceLevels: emptyLevels,
		Metadata:         json.RawMessage(`{}`),
	}
	if len(candles) < 2 {
		snapshot.Metadata = mustJSON(map[string]any{"reason": "insufficient_candles", "candles": len(candles)})
		return snapshot
	}

	first := candles[0]
	last := candles[len(candles)-1]
	high := first.High
	low := first.Low
	for _, candle := range candles {
		if candle.High > high {
			high = candle.High
		}
		if candle.Low < low {
			low = candle.Low
		}
	}

	priceChangePct := 0.0
	if first.Close != 0 {
		priceChangePct = ((last.Close - first.Close) / first.Close) * 100
	}
	switch {
	case priceChangePct > 0.2:
		snapshot.TrendDirection = "bullish"
	case priceChangePct < -0.2:
		snapshot.TrendDirection = "bearish"
	default:
		snapshot.TrendDirection = "sideways"
	}

	rangePct := 0.0
	if last.Close != 0 {
		rangePct = ((high - low) / last.Close) * 100
	}
	switch {
	case rangePct < 1:
		snapshot.StructureState = "compression"
	case rangePct > 5 || math.Abs(priceChangePct) > 3:
		snapshot.StructureState = "expansion"
	default:
		snapshot.StructureState = "ranging"
	}

	snapshot.LastSwingHigh = floatPtr(high)
	snapshot.LastSwingLow = floatPtr(low)
	supports := supportLevels(candles, 3)
	resistances := resistanceLevels(candles, 3)
	snapshot.SupportLevels = mustJSON(supports)
	snapshot.ResistanceLevels = mustJSON(resistances)
	if high > low {
		position := ((last.Close - low) / (high - low)) * 100
		snapshot.PricePosition = &position
	}
	snapshot.Metadata = mustJSON(map[string]any{
		"candles":              len(candles),
		"price_change_percent": priceChangePct,
		"range_percent":        rangePct,
		"non_signal":           true,
	})

	return snapshot
}

func computeVolatility(symbolID int64, period string, snapshotTime time.Time, candles []aggregateCandle, candles24h []aggregateCandle, candles7d []aggregateCandle) aggregation.VolatilitySnapshot {
	snapshot := aggregation.VolatilitySnapshot{
		SymbolID:     symbolID,
		Exchange:     "aggregate",
		Period:       period,
		SnapshotTime: snapshotTime,
		RawData:      mustJSON(map[string]any{"candles": len(candles), "non_signal": true}),
	}
	if len(candles) == 0 {
		return snapshot
	}

	high, low, closePrice := candleRange(candles)
	snapshot.HighPrice = &high
	snapshot.LowPrice = &low
	snapshot.ClosePrice = &closePrice

	atr := averageTrueRange(candles)
	if atr != nil {
		snapshot.ATR = atr
		if closePrice != 0 {
			atrPercent := (*atr / closePrice) * 100
			snapshot.ATRPercent = &atrPercent
		}
	}

	realized := realizedVolatility(candles)
	if realized != nil {
		snapshot.RealizedVolatility = realized
		percent := *realized * 100
		snapshot.RealizedVolatilityPercent = &percent
	}
	snapshot.RangePercent24h = rangePercent(candles24h)
	snapshot.RangePercent7d = rangePercent(candles7d)

	return snapshot
}

func filterCandles(candles []aggregateCandle, start time.Time, end time.Time) []aggregateCandle {
	filtered := make([]aggregateCandle, 0, len(candles))
	for _, candle := range candles {
		if (candle.OpenTime.Equal(start) || candle.OpenTime.After(start)) && (candle.OpenTime.Equal(end) || candle.OpenTime.Before(end)) {
			filtered = append(filtered, candle)
		}
	}
	return filtered
}

func supportLevels(candles []aggregateCandle, limit int) []float64 {
	values := make([]float64, 0, len(candles))
	for _, candle := range candles {
		values = append(values, candle.Low)
	}
	sort.Float64s(values)
	return uniqueRounded(values, limit)
}

func resistanceLevels(candles []aggregateCandle, limit int) []float64 {
	values := make([]float64, 0, len(candles))
	for _, candle := range candles {
		values = append(values, candle.High)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(values)))
	return uniqueRounded(values, limit)
}

func uniqueRounded(values []float64, limit int) []float64 {
	result := make([]float64, 0, limit)
	for _, value := range values {
		if len(result) > 0 && math.Abs(result[len(result)-1]-value) < 1e-9 {
			continue
		}
		result = append(result, value)
		if len(result) == limit {
			break
		}
	}
	return result
}

func averageTrueRange(candles []aggregateCandle) *float64 {
	if len(candles) == 0 {
		return nil
	}
	var sum float64
	for index, candle := range candles {
		trueRange := candle.High - candle.Low
		if index > 0 {
			prevClose := candles[index-1].Close
			trueRange = math.Max(trueRange, math.Abs(candle.High-prevClose))
			trueRange = math.Max(trueRange, math.Abs(candle.Low-prevClose))
		}
		sum += trueRange
	}
	value := sum / float64(len(candles))
	return &value
}

func realizedVolatility(candles []aggregateCandle) *float64 {
	if len(candles) < 2 {
		return nil
	}
	returns := make([]float64, 0, len(candles)-1)
	for index := 1; index < len(candles); index++ {
		prev := candles[index-1].Close
		current := candles[index].Close
		if prev <= 0 || current <= 0 {
			continue
		}
		returns = append(returns, math.Log(current/prev))
	}
	if len(returns) < 2 {
		return nil
	}

	mean := 0.0
	for _, value := range returns {
		mean += value
	}
	mean /= float64(len(returns))

	variance := 0.0
	for _, value := range returns {
		diff := value - mean
		variance += diff * diff
	}
	variance /= float64(len(returns) - 1)
	value := math.Sqrt(variance) * math.Sqrt(float64(len(returns)))
	return &value
}

func candleRange(candles []aggregateCandle) (float64, float64, float64) {
	high := candles[0].High
	low := candles[0].Low
	closePrice := candles[len(candles)-1].Close
	for _, candle := range candles {
		if candle.High > high {
			high = candle.High
		}
		if candle.Low < low {
			low = candle.Low
		}
	}
	return high, low, closePrice
}

func rangePercent(candles []aggregateCandle) *float64 {
	if len(candles) == 0 {
		return nil
	}
	high, low, closePrice := candleRange(candles)
	if closePrice == 0 {
		return nil
	}
	value := ((high - low) / closePrice) * 100
	return &value
}

func minMaxMetric(items []exchangeMetric, pick func(exchangeMetric) *float64) (*float64, *float64, string, string) {
	var minValue, maxValue *float64
	var minExchange, maxExchange string
	for _, item := range items {
		value := pick(item)
		if value == nil {
			continue
		}
		if minValue == nil || *value < *minValue {
			copied := *value
			minValue = &copied
			minExchange = item.Exchange
		}
		if maxValue == nil || *value > *maxValue {
			copied := *value
			maxValue = &copied
			maxExchange = item.Exchange
		}
	}
	return minValue, maxValue, minExchange, maxExchange
}

func spreadPercent(minValue *float64, maxValue *float64) *float64 {
	if minValue == nil || maxValue == nil || *minValue == 0 {
		return nil
	}
	value := ((*maxValue - *minValue) / *minValue) * 100
	return &value
}

func simpleSpread(minValue *float64, maxValue *float64) *float64 {
	if minValue == nil || maxValue == nil {
		return nil
	}
	value := *maxValue - *minValue
	return &value
}

func (r *AggregateRepository) upsertMarketStructure(ctx context.Context, item aggregation.MarketStructureSnapshot) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO market_structure_snapshots (
		    symbol_id, exchange, market_type, source_symbol, period, snapshot_time,
		    trend, support_price, resistance_price, breakout_state,
		    trend_direction, structure_state, last_swing_high, last_swing_low,
		    support_levels, resistance_levels, price_position, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		ON CONFLICT (symbol_id, exchange, period, snapshot_time) DO UPDATE SET
		    market_type = EXCLUDED.market_type,
		    source_symbol = EXCLUDED.source_symbol,
		    trend = EXCLUDED.trend,
		    support_price = EXCLUDED.support_price,
		    resistance_price = EXCLUDED.resistance_price,
		    breakout_state = EXCLUDED.breakout_state,
		    trend_direction = EXCLUDED.trend_direction,
		    structure_state = EXCLUDED.structure_state,
		    last_swing_high = EXCLUDED.last_swing_high,
		    last_swing_low = EXCLUDED.last_swing_low,
		    support_levels = EXCLUDED.support_levels,
		    resistance_levels = EXCLUDED.resistance_levels,
		    price_position = EXCLUDED.price_position,
		    metadata = EXCLUDED.metadata
	`,
		item.SymbolID,
		item.Exchange,
		nullableText(item.MarketType),
		nullableText(item.SourceSymbol),
		item.Period,
		item.SnapshotTime,
		item.TrendDirection,
		nullableFloat(item.LastSwingLow),
		nullableFloat(item.LastSwingHigh),
		item.StructureState,
		item.TrendDirection,
		item.StructureState,
		nullableFloat(item.LastSwingHigh),
		nullableFloat(item.LastSwingLow),
		ensureJSON(item.SupportLevels),
		ensureJSON(item.ResistanceLevels),
		nullableFloat(item.PricePosition),
		ensureJSON(item.Metadata),
	)
	if err != nil {
		return fmt.Errorf("upsert market structure snapshot: %w", err)
	}

	return nil
}

func (r *AggregateRepository) upsertVolatility(ctx context.Context, item aggregation.VolatilitySnapshot) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO volatility_snapshots (
		    symbol_id, exchange, market_type, source_symbol, period, snapshot_time,
		    realized_volatility, realized_volatility_percent, atr, atr_percent,
		    high_price, low_price, close_price, range_percent_24h,
		    range_percent_7d, raw_data
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (symbol_id, exchange, period, snapshot_time) DO UPDATE SET
		    market_type = EXCLUDED.market_type,
		    source_symbol = EXCLUDED.source_symbol,
		    realized_volatility = EXCLUDED.realized_volatility,
		    realized_volatility_percent = EXCLUDED.realized_volatility_percent,
		    atr = EXCLUDED.atr,
		    atr_percent = EXCLUDED.atr_percent,
		    high_price = EXCLUDED.high_price,
		    low_price = EXCLUDED.low_price,
		    close_price = EXCLUDED.close_price,
		    range_percent_24h = EXCLUDED.range_percent_24h,
		    range_percent_7d = EXCLUDED.range_percent_7d,
		    raw_data = EXCLUDED.raw_data
	`,
		item.SymbolID,
		item.Exchange,
		nullableText(item.MarketType),
		nullableText(item.SourceSymbol),
		item.Period,
		item.SnapshotTime,
		nullableFloat(item.RealizedVolatility),
		nullableFloat(item.RealizedVolatilityPercent),
		nullableFloat(item.ATR),
		nullableFloat(item.ATRPercent),
		nullableFloat(item.HighPrice),
		nullableFloat(item.LowPrice),
		nullableFloat(item.ClosePrice),
		nullableFloat(item.RangePercent24h),
		nullableFloat(item.RangePercent7d),
		ensureJSON(item.RawData),
	)
	if err != nil {
		return fmt.Errorf("upsert volatility snapshot: %w", err)
	}

	return nil
}

func (r *AggregateRepository) upsertExchangeDivergence(ctx context.Context, item aggregation.ExchangeDivergenceSnapshot) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO exchange_divergence_snapshots (
		    symbol_id, data_type, snapshot_time, reference_exchange,
		    compared_exchange, reference_value, compared_value,
		    divergence_abs, divergence_bps, price_min, price_max,
		    price_spread_percent, oi_min, oi_max, oi_spread_percent,
		    funding_min, funding_max, funding_spread, volume_min,
		    volume_max, volume_spread_percent, strongest_exchange,
		    weakest_exchange, raw_by_exchange, metadata, metrics,
		    quality_metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, NULLIF($22, ''), NULLIF($23, ''), $24, $25, $26, $27)
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
		    metadata = EXCLUDED.metadata,
		    metrics = EXCLUDED.metrics,
		    quality_metadata = EXCLUDED.quality_metadata
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
		ensureJSON(item.Metadata),
		ensureJSON(item.Metadata),
		ensureJSON(item.Metadata),
	)
	if err != nil {
		return fmt.Errorf("upsert exchange divergence snapshot: %w", err)
	}

	return nil
}

func buildAnomalyFlags(snapshot aggregation.DerivativeAggregateSnapshot, windows []aggregation.AnalyticsWindowMetric) json.RawMessage {
	type anomalyFlag struct {
		Name     string  `json:"name"`
		Window   string  `json:"window,omitempty"`
		Severity string  `json:"severity"`
		Value    float64 `json:"value"`
	}

	var flags []anomalyFlag
	for _, metric := range windows {
		if metric.LiquidationSumUSD != nil && *metric.LiquidationSumUSD >= 1_000_000 {
			flags = append(flags, anomalyFlag{Name: "large_liquidation_cluster", Window: metric.Window, Severity: "info", Value: *metric.LiquidationSumUSD})
		}
		if metric.BasisAvg != nil && math.Abs(*metric.BasisAvg) >= 1 {
			flags = append(flags, anomalyFlag{Name: "elevated_basis", Window: metric.Window, Severity: "info", Value: *metric.BasisAvg})
		}
		if metric.FundingMax != nil && *metric.FundingMax >= 0.001 {
			flags = append(flags, anomalyFlag{Name: "positive_funding_extreme", Window: metric.Window, Severity: "info", Value: *metric.FundingMax})
		}
		if metric.FundingMin != nil && *metric.FundingMin <= -0.001 {
			flags = append(flags, anomalyFlag{Name: "negative_funding_extreme", Window: metric.Window, Severity: "info", Value: *metric.FundingMin})
		}
	}
	if snapshot.AvgOrderbookImbalancePercent != nil && math.Abs(*snapshot.AvgOrderbookImbalancePercent) >= 50 {
		flags = append(flags, anomalyFlag{Name: "orderbook_imbalance", Severity: "info", Value: *snapshot.AvgOrderbookImbalancePercent})
	}

	raw, _ := json.Marshal(flags)
	return raw
}

func mustJSON(value any) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}
