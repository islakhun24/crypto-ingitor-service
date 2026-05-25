package aggregation

import (
	"encoding/json"
	"time"
)

type DerivativeAggregateSnapshot struct {
	SymbolID                     int64           `json:"symbol_id"`
	SnapshotTime                 time.Time       `json:"snapshot_time"`
	ExchangeCount                int             `json:"exchange_count"`
	PriceAvg                     *float64        `json:"price_avg,omitempty"`
	PriceWeighted                *float64        `json:"price_weighted,omitempty"`
	TotalVolume24h               *float64        `json:"total_volume_24h,omitempty"`
	TotalQuoteVolume24h          *float64        `json:"total_quote_volume_24h,omitempty"`
	TotalOpenInterest            *float64        `json:"total_open_interest,omitempty"`
	TotalOpenInterestValue       *float64        `json:"total_open_interest_value,omitempty"`
	AvgFundingRate               *float64        `json:"avg_funding_rate,omitempty"`
	MinFundingRate               *float64        `json:"min_funding_rate,omitempty"`
	MaxFundingRate               *float64        `json:"max_funding_rate,omitempty"`
	TotalTakerBuyVolume          *float64        `json:"total_taker_buy_volume,omitempty"`
	TotalTakerSellVolume         *float64        `json:"total_taker_sell_volume,omitempty"`
	TotalBuySellDelta            *float64        `json:"total_buy_sell_delta,omitempty"`
	TotalCVD                     *float64        `json:"total_cvd,omitempty"`
	TotalLongLiquidationUSD      *float64        `json:"total_long_liquidation_usd,omitempty"`
	TotalShortLiquidationUSD     *float64        `json:"total_short_liquidation_usd,omitempty"`
	TotalLiquidationUSD          *float64        `json:"total_liquidation_usd,omitempty"`
	AvgBasisPercent              *float64        `json:"avg_basis_percent,omitempty"`
	AvgOrderbookImbalancePercent *float64        `json:"avg_orderbook_imbalance_percent,omitempty"`
	TotalBidDepthUSD             *float64        `json:"total_bid_depth_usd,omitempty"`
	TotalAskDepthUSD             *float64        `json:"total_ask_depth_usd,omitempty"`
	AvailableExchanges           json.RawMessage `json:"available_exchanges"`
	RawByExchange                json.RawMessage `json:"raw_by_exchange"`
	WindowMetrics                json.RawMessage `json:"window_metrics"`
	Metrics                      json.RawMessage `json:"metrics"`
	QualityMetadata              json.RawMessage `json:"quality_metadata"`
	AnomalyFlags                 json.RawMessage `json:"anomaly_flags"`
}

type AnalyticsWindowMetric struct {
	Window            string          `json:"window"`
	StartTime         time.Time       `json:"start_time"`
	EndTime           time.Time       `json:"end_time"`
	PriceChange       *float64        `json:"price_change,omitempty"`
	PriceChangePct    *float64        `json:"price_change_percent,omitempty"`
	VolumeChange      *float64        `json:"volume_change,omitempty"`
	OIChange          *float64        `json:"oi_change,omitempty"`
	FundingAvg        *float64        `json:"funding_avg,omitempty"`
	FundingMin        *float64        `json:"funding_min,omitempty"`
	FundingMax        *float64        `json:"funding_max,omitempty"`
	TakerDelta        *float64        `json:"taker_delta,omitempty"`
	CVDChange         *float64        `json:"cvd_change,omitempty"`
	LiquidationSumUSD *float64        `json:"liquidation_sum_usd,omitempty"`
	BasisAvg          *float64        `json:"basis_avg,omitempty"`
	BasisMin          *float64        `json:"basis_min,omitempty"`
	BasisMax          *float64        `json:"basis_max,omitempty"`
	Quality           json.RawMessage `json:"quality,omitempty"`
}

type MarketStructureSnapshot struct {
	SymbolID         int64           `json:"symbol_id"`
	Exchange         string          `json:"exchange"`
	MarketType       string          `json:"market_type,omitempty"`
	SourceSymbol     string          `json:"source_symbol,omitempty"`
	Period           string          `json:"period"`
	SnapshotTime     time.Time       `json:"snapshot_time"`
	TrendDirection   string          `json:"trend_direction"`
	StructureState   string          `json:"structure_state"`
	LastSwingHigh    *float64        `json:"last_swing_high,omitempty"`
	LastSwingLow     *float64        `json:"last_swing_low,omitempty"`
	SupportLevels    json.RawMessage `json:"support_levels"`
	ResistanceLevels json.RawMessage `json:"resistance_levels"`
	PricePosition    *float64        `json:"price_position,omitempty"`
	Metadata         json.RawMessage `json:"metadata"`
}

type VolatilitySnapshot struct {
	SymbolID                  int64           `json:"symbol_id"`
	Exchange                  string          `json:"exchange"`
	MarketType                string          `json:"market_type,omitempty"`
	SourceSymbol              string          `json:"source_symbol,omitempty"`
	Period                    string          `json:"period"`
	SnapshotTime              time.Time       `json:"snapshot_time"`
	RealizedVolatility        *float64        `json:"realized_volatility,omitempty"`
	RealizedVolatilityPercent *float64        `json:"realized_volatility_percent,omitempty"`
	ATR                       *float64        `json:"atr,omitempty"`
	ATRPercent                *float64        `json:"atr_percent,omitempty"`
	HighPrice                 *float64        `json:"high_price,omitempty"`
	LowPrice                  *float64        `json:"low_price,omitempty"`
	ClosePrice                *float64        `json:"close_price,omitempty"`
	RangePercent24h           *float64        `json:"range_percent_24h,omitempty"`
	RangePercent7d            *float64        `json:"range_percent_7d,omitempty"`
	RawData                   json.RawMessage `json:"raw_data"`
}

type ExchangeDivergenceSnapshot struct {
	SymbolID            int64           `json:"symbol_id"`
	DataType            string          `json:"data_type"`
	SnapshotTime        time.Time       `json:"snapshot_time"`
	ReferenceExchange   string          `json:"reference_exchange"`
	ComparedExchange    string          `json:"compared_exchange"`
	ReferenceValue      *float64        `json:"reference_value,omitempty"`
	ComparedValue       *float64        `json:"compared_value,omitempty"`
	DivergenceAbs       *float64        `json:"divergence_abs,omitempty"`
	DivergenceBPS       *float64        `json:"divergence_bps,omitempty"`
	PriceMin            *float64        `json:"price_min,omitempty"`
	PriceMax            *float64        `json:"price_max,omitempty"`
	PriceSpreadPercent  *float64        `json:"price_spread_percent,omitempty"`
	OIMin               *float64        `json:"oi_min,omitempty"`
	OIMax               *float64        `json:"oi_max,omitempty"`
	OISpreadPercent     *float64        `json:"oi_spread_percent,omitempty"`
	FundingMin          *float64        `json:"funding_min,omitempty"`
	FundingMax          *float64        `json:"funding_max,omitempty"`
	FundingSpread       *float64        `json:"funding_spread,omitempty"`
	VolumeMin           *float64        `json:"volume_min,omitempty"`
	VolumeMax           *float64        `json:"volume_max,omitempty"`
	VolumeSpreadPercent *float64        `json:"volume_spread_percent,omitempty"`
	StrongestExchange   string          `json:"strongest_exchange,omitempty"`
	WeakestExchange     string          `json:"weakest_exchange,omitempty"`
	RawByExchange       json.RawMessage `json:"raw_by_exchange"`
	Metadata            json.RawMessage `json:"metadata"`
}

type AnalyticsSnapshotSet struct {
	MarketStructures []MarketStructureSnapshot   `json:"market_structures,omitempty"`
	Volatility       []VolatilitySnapshot        `json:"volatility,omitempty"`
	Divergence       *ExchangeDivergenceSnapshot `json:"divergence,omitempty"`
}
