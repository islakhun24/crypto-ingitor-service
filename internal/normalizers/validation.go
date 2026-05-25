package normalizers

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var periodPattern = regexp.MustCompile(`^([0-9]+(s|m|h|d|w|M)|Min[0-9]+|Hour[0-9]+|Day[0-9]+|Week[0-9]+|Month[0-9]+)$`)

func ValidateSource(meta SourceMeta) error {
	if meta.SymbolID <= 0 {
		return fmt.Errorf("missing symbol_id")
	}
	if strings.TrimSpace(meta.Exchange) == "" {
		return fmt.Errorf("missing exchange")
	}
	if strings.TrimSpace(meta.SourceSymbol) == "" {
		return fmt.Errorf("missing source_symbol")
	}

	return nil
}

func ValidateTimestamp(name string, value time.Time) error {
	if value.IsZero() {
		return fmt.Errorf("missing %s", name)
	}

	return nil
}

func ValidateNonNegative(name string, value float64) error {
	if value < 0 {
		return fmt.Errorf("%s must be non-negative", name)
	}

	return nil
}

func ValidateOptionalNonNegative(name string, value *float64) error {
	if value == nil {
		return nil
	}

	return ValidateNonNegative(name, *value)
}

func ValidateFundingRate(value float64, min float64, max float64) error {
	if value < min || value > max {
		return fmt.Errorf("funding rate %f outside bounds [%f,%f]", value, min, max)
	}

	return nil
}

func ValidatePeriod(period string) error {
	period = strings.TrimSpace(period)
	if period == "" {
		return nil
	}
	if !periodPattern.MatchString(period) {
		return fmt.Errorf("malformed period %q", period)
	}

	return nil
}

func ValidateMarketSnapshot(snapshot NormalizedMarketSnapshot) error {
	if err := ValidateSource(snapshot.SourceMeta); err != nil {
		return err
	}
	if err := ValidateTimestamp("snapshot_time", snapshot.SnapshotTime); err != nil {
		return err
	}

	checks := map[string]*float64{
		"last_price":       snapshot.LastPrice,
		"mark_price":       snapshot.MarkPrice,
		"index_price":      snapshot.IndexPrice,
		"bid_price":        snapshot.BidPrice,
		"ask_price":        snapshot.AskPrice,
		"volume_24h":       snapshot.Volume24h,
		"quote_volume_24h": snapshot.QuoteVolume24h,
		"open_interest":    snapshot.OpenInterest,
	}
	for name, value := range checks {
		if err := ValidateOptionalNonNegative(name, value); err != nil {
			return err
		}
	}

	if snapshot.FundingRate != nil {
		if err := ValidateFundingRate(*snapshot.FundingRate, -1, 1); err != nil {
			return err
		}
	}

	return nil
}

func ValidateKline(kline NormalizedKline) error {
	if err := ValidateSource(kline.SourceMeta); err != nil {
		return err
	}
	if err := ValidatePeriod(kline.Interval); err != nil {
		return err
	}
	if err := ValidateTimestamp("open_time", kline.OpenTime); err != nil {
		return err
	}
	if err := ValidateTimestamp("close_time", kline.CloseTime); err != nil {
		return err
	}

	values := map[string]float64{
		"open_price":  kline.OpenPrice,
		"high_price":  kline.HighPrice,
		"low_price":   kline.LowPrice,
		"close_price": kline.ClosePrice,
	}
	for name, value := range values {
		if err := ValidateNonNegative(name, value); err != nil {
			return err
		}
	}

	optionals := map[string]*float64{
		"volume":                 kline.Volume,
		"quote_volume":           kline.QuoteVolume,
		"taker_buy_volume":       kline.TakerBuyVolume,
		"taker_buy_quote_volume": kline.TakerBuyQuoteVolume,
	}
	for name, value := range optionals {
		if err := ValidateOptionalNonNegative(name, value); err != nil {
			return err
		}
	}

	return nil
}

func ValidateOpenInterest(item NormalizedOpenInterest, history bool) error {
	if err := ValidateSource(item.SourceMeta); err != nil {
		return err
	}
	if history {
		if err := ValidatePeriod(item.Period); err != nil {
			return err
		}
		if strings.TrimSpace(item.Period) == "" {
			return fmt.Errorf("missing period")
		}
	}
	if err := ValidateTimestamp("snapshot_time", item.SnapshotTime); err != nil {
		return err
	}
	if err := ValidateNonNegative("open_interest", item.OpenInterest); err != nil {
		return err
	}
	if err := ValidateOptionalNonNegative("open_interest_value", item.OpenInterestValue); err != nil {
		return err
	}

	return nil
}

func ValidateFundingSnapshot(item NormalizedFundingSnapshot) error {
	if err := ValidateSource(item.SourceMeta); err != nil {
		return err
	}
	if err := ValidateTimestamp("snapshot_time", item.SnapshotTime); err != nil {
		return err
	}
	if err := ValidateFundingRate(item.FundingRate, -1, 1); err != nil {
		return err
	}
	if err := ValidateOptionalNonNegative("mark_price", item.MarkPrice); err != nil {
		return err
	}
	if err := ValidateOptionalNonNegative("index_price", item.IndexPrice); err != nil {
		return err
	}

	return nil
}

func ValidateFundingHistory(item NormalizedFundingHistory) error {
	if err := ValidateSource(item.SourceMeta); err != nil {
		return err
	}
	if err := ValidateTimestamp("funding_time", item.FundingTime); err != nil {
		return err
	}
	if err := ValidateFundingRate(item.FundingRate, -1, 1); err != nil {
		return err
	}
	if err := ValidateOptionalNonNegative("mark_price", item.MarkPrice); err != nil {
		return err
	}

	return nil
}

func ValidateLongShortRatio(item NormalizedLongShortRatio) error {
	if err := ValidateSource(item.SourceMeta); err != nil {
		return err
	}
	if err := ValidateTimestamp("snapshot_time", item.SnapshotTime); err != nil {
		return err
	}
	if err := ValidatePeriod(item.Period); err != nil {
		return err
	}

	return nil
}

func ValidateTakerFlow(item NormalizedTakerFlow) error {
	if err := ValidateSource(item.SourceMeta); err != nil {
		return err
	}
	if err := ValidateTimestamp("snapshot_time", item.SnapshotTime); err != nil {
		return err
	}
	if err := ValidatePeriod(item.Period); err != nil {
		return err
	}

	checks := map[string]*float64{
		"taker_buy_volume":        item.TakerBuyVolume,
		"taker_sell_volume":       item.TakerSellVolume,
		"taker_buy_quote_volume":  item.TakerBuyQuoteVolume,
		"taker_sell_quote_volume": item.TakerSellQuoteVolume,
	}
	for name, value := range checks {
		if err := ValidateOptionalNonNegative(name, value); err != nil {
			return err
		}
	}

	return nil
}

func ValidateCVD(item NormalizedCVD) error {
	if err := ValidateSource(item.SourceMeta); err != nil {
		return err
	}
	if err := ValidateTimestamp("snapshot_time", item.SnapshotTime); err != nil {
		return err
	}
	if err := ValidatePeriod(item.Period); err != nil {
		return err
	}

	return nil
}

func ValidateLiquidationEvent(item NormalizedLiquidationEvent) error {
	if err := ValidateSource(item.SourceMeta); err != nil {
		return err
	}
	if strings.TrimSpace(item.EventKey) == "" {
		return fmt.Errorf("missing event_key")
	}
	if err := ValidateTimestamp("event_time", item.EventTime); err != nil {
		return err
	}
	if strings.TrimSpace(item.Side) == "" {
		return fmt.Errorf("missing liquidation side")
	}
	if err := ValidateOptionalNonNegative("price", item.Price); err != nil {
		return err
	}
	if err := ValidateOptionalNonNegative("quantity", item.Quantity); err != nil {
		return err
	}
	if err := ValidateOptionalNonNegative("notional", item.Notional); err != nil {
		return err
	}
	if err := ValidateOptionalNonNegative("usd_value", item.USDValue); err != nil {
		return err
	}

	return nil
}

func ValidateLiquidationAggregate(item NormalizedLiquidationAggregate) error {
	if err := ValidateSource(item.SourceMeta); err != nil {
		return err
	}
	if err := ValidateTimestamp("bucket_time", item.BucketTime); err != nil {
		return err
	}
	if err := ValidatePeriod(item.Period); err != nil {
		return err
	}

	values := map[string]float64{
		"long_liquidation_notional":  item.LongLiquidationNotional,
		"short_liquidation_notional": item.ShortLiquidationNotional,
		"total_liquidation_notional": item.TotalLiquidationNotional,
		"long_liquidation_usd":       item.LongLiquidationUSD,
		"short_liquidation_usd":      item.ShortLiquidationUSD,
		"total_liquidation_usd":      item.TotalLiquidationUSD,
		"largest_liquidation_usd":    item.LargestLiquidationUSD,
	}
	for name, value := range values {
		if err := ValidateNonNegative(name, value); err != nil {
			return err
		}
	}

	return nil
}

func ValidateBasisPremium(item NormalizedBasisPremium) error {
	if err := ValidateSource(item.SourceMeta); err != nil {
		return err
	}
	if err := ValidateTimestamp("snapshot_time", item.SnapshotTime); err != nil {
		return err
	}

	checks := map[string]*float64{
		"futures_price": item.FuturesPrice,
		"mark_price":    item.MarkPrice,
		"index_price":   item.IndexPrice,
		"spot_price":    item.SpotPrice,
	}
	for name, value := range checks {
		if err := ValidateOptionalNonNegative(name, value); err != nil {
			return err
		}
	}

	return nil
}

func ValidateOrderbookImbalance(item NormalizedOrderbookImbalance) error {
	if err := ValidateSource(item.SourceMeta); err != nil {
		return err
	}
	if err := ValidateTimestamp("snapshot_time", item.SnapshotTime); err != nil {
		return err
	}
	if item.DepthLevels <= 0 {
		return fmt.Errorf("depth_levels must be greater than 0")
	}

	checks := map[string]*float64{
		"mid_price":          item.MidPrice,
		"bid_notional":       item.BidNotional,
		"ask_notional":       item.AskNotional,
		"bid_depth_usd":      item.BidDepthUSD,
		"ask_depth_usd":      item.AskDepthUSD,
		"bid_depth_1pct_usd": item.BidDepth1PctUSD,
		"ask_depth_1pct_usd": item.AskDepth1PctUSD,
		"bid_depth_2pct_usd": item.BidDepth2PctUSD,
		"ask_depth_2pct_usd": item.AskDepth2PctUSD,
		"bid_depth_5pct_usd": item.BidDepth5PctUSD,
		"ask_depth_5pct_usd": item.AskDepth5PctUSD,
	}
	for name, value := range checks {
		if err := ValidateOptionalNonNegative(name, value); err != nil {
			return err
		}
	}

	return nil
}

func ValidateExchangeDivergence(item NormalizedExchangeDivergence) error {
	if err := ValidateSource(item.SourceMeta); err != nil {
		return err
	}
	if err := ValidateTimestamp("snapshot_time", item.SnapshotTime); err != nil {
		return err
	}
	if strings.TrimSpace(item.DataType) == "" {
		return fmt.Errorf("missing data_type")
	}

	return nil
}
