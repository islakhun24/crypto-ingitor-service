package normalizers

import (
	"encoding/json"
	"time"
)

type NormalizedResult struct {
	MarketSnapshots       []NormalizedMarketSnapshot       `json:"market_snapshots,omitempty"`
	Klines                []NormalizedKline                `json:"klines,omitempty"`
	OpenInterest          []NormalizedOpenInterest         `json:"open_interest,omitempty"`
	FundingSnapshots      []NormalizedFundingSnapshot      `json:"funding_snapshots,omitempty"`
	FundingHistory        []NormalizedFundingHistory       `json:"funding_history,omitempty"`
	LongShortRatios       []NormalizedLongShortRatio       `json:"long_short_ratios,omitempty"`
	TakerFlows            []NormalizedTakerFlow            `json:"taker_flows,omitempty"`
	CVDSnapshots          []NormalizedCVD                  `json:"cvd_snapshots,omitempty"`
	LiquidationEvents     []NormalizedLiquidationEvent     `json:"liquidation_events,omitempty"`
	LiquidationAggregates []NormalizedLiquidationAggregate `json:"liquidation_aggregates,omitempty"`
	BasisPremiums         []NormalizedBasisPremium         `json:"basis_premiums,omitempty"`
	OrderbookImbalances   []NormalizedOrderbookImbalance   `json:"orderbook_imbalances,omitempty"`
	ExchangeDivergences   []NormalizedExchangeDivergence   `json:"exchange_divergences,omitempty"`
}

type SourceMeta struct {
	SymbolID         int64           `json:"symbol_id"`
	Exchange         string          `json:"exchange"`
	SourceSymbol     string          `json:"source_symbol"`
	SourceEndpointID int64           `json:"source_endpoint_id,omitempty"`
	RawData          json.RawMessage `json:"raw_data,omitempty"`
}

type NormalizedMarketSnapshot struct {
	SourceMeta
	SnapshotTime          time.Time `json:"snapshot_time"`
	LastPrice             *float64  `json:"last_price,omitempty"`
	MarkPrice             *float64  `json:"mark_price,omitempty"`
	IndexPrice            *float64  `json:"index_price,omitempty"`
	BidPrice              *float64  `json:"bid_price,omitempty"`
	AskPrice              *float64  `json:"ask_price,omitempty"`
	Volume24h             *float64  `json:"volume_24h,omitempty"`
	QuoteVolume24h        *float64  `json:"quote_volume_24h,omitempty"`
	PriceChangePercent24h *float64  `json:"price_change_percent_24h,omitempty"`
	OpenInterest          *float64  `json:"open_interest,omitempty"`
	FundingRate           *float64  `json:"funding_rate,omitempty"`
}

type NormalizedKline struct {
	SourceMeta
	Interval            string    `json:"interval"`
	OpenTime            time.Time `json:"open_time"`
	CloseTime           time.Time `json:"close_time"`
	OpenPrice           float64   `json:"open_price"`
	HighPrice           float64   `json:"high_price"`
	LowPrice            float64   `json:"low_price"`
	ClosePrice          float64   `json:"close_price"`
	Volume              *float64  `json:"volume,omitempty"`
	QuoteVolume         *float64  `json:"quote_volume,omitempty"`
	TradeCount          *int64    `json:"trade_count,omitempty"`
	TakerBuyVolume      *float64  `json:"taker_buy_volume,omitempty"`
	TakerBuyQuoteVolume *float64  `json:"taker_buy_quote_volume,omitempty"`
	IsClosed            bool      `json:"is_closed"`
}

type NormalizedOpenInterest struct {
	SourceMeta
	SnapshotTime      time.Time `json:"snapshot_time"`
	Period            string    `json:"period,omitempty"`
	OpenInterest      float64   `json:"open_interest"`
	OpenInterestValue *float64  `json:"open_interest_value,omitempty"`
}

type NormalizedFundingSnapshot struct {
	SourceMeta
	SnapshotTime    time.Time `json:"snapshot_time"`
	FundingRate     float64   `json:"funding_rate"`
	NextFundingTime time.Time `json:"next_funding_time,omitempty"`
	MarkPrice       *float64  `json:"mark_price,omitempty"`
	IndexPrice      *float64  `json:"index_price,omitempty"`
}

type NormalizedFundingHistory struct {
	SourceMeta
	FundingTime  time.Time `json:"funding_time"`
	FundingRate  float64   `json:"funding_rate"`
	RealizedRate *float64  `json:"realized_rate,omitempty"`
	MarkPrice    *float64  `json:"mark_price,omitempty"`
}

type NormalizedLongShortRatio struct {
	SourceMeta
	Period              string    `json:"period"`
	SnapshotTime        time.Time `json:"snapshot_time"`
	LongRatio           *float64  `json:"long_ratio,omitempty"`
	ShortRatio          *float64  `json:"short_ratio,omitempty"`
	LongAccountRatio    *float64  `json:"long_account_ratio,omitempty"`
	ShortAccountRatio   *float64  `json:"short_account_ratio,omitempty"`
	LongPositionRatio   *float64  `json:"long_position_ratio,omitempty"`
	ShortPositionRatio  *float64  `json:"short_position_ratio,omitempty"`
	TopTraderLongRatio  *float64  `json:"top_trader_long_ratio,omitempty"`
	TopTraderShortRatio *float64  `json:"top_trader_short_ratio,omitempty"`
	LongShortRatio      *float64  `json:"long_short_ratio,omitempty"`
}

type NormalizedTakerFlow struct {
	SourceMeta
	Period               string    `json:"period"`
	SnapshotTime         time.Time `json:"snapshot_time"`
	TakerBuyVolume       *float64  `json:"taker_buy_volume,omitempty"`
	TakerSellVolume      *float64  `json:"taker_sell_volume,omitempty"`
	TakerBuyQuoteVolume  *float64  `json:"taker_buy_quote_volume,omitempty"`
	TakerSellQuoteVolume *float64  `json:"taker_sell_quote_volume,omitempty"`
	BuySellDelta         *float64  `json:"buy_sell_delta,omitempty"`
	BuySellDeltaQuote    *float64  `json:"buy_sell_delta_quote,omitempty"`
	BuySellRatio         *float64  `json:"buy_sell_ratio,omitempty"`
}

type NormalizedCVD struct {
	SourceMeta
	Period           string    `json:"period"`
	SnapshotTime     time.Time `json:"snapshot_time"`
	CVDValue         *float64  `json:"cvd_value,omitempty"`
	CVDDelta         *float64  `json:"cvd_delta,omitempty"`
	CVDChange        *float64  `json:"cvd_change,omitempty"`
	CVDChangePercent *float64  `json:"cvd_change_percent,omitempty"`
	BuyVolume        *float64  `json:"buy_volume,omitempty"`
	SellVolume       *float64  `json:"sell_volume,omitempty"`
}

type NormalizedLiquidationEvent struct {
	SourceMeta
	EventKey  string    `json:"event_key"`
	EventTime time.Time `json:"event_time"`
	Side      string    `json:"side"`
	Price     *float64  `json:"price,omitempty"`
	Quantity  *float64  `json:"quantity,omitempty"`
	Notional  *float64  `json:"notional,omitempty"`
	USDValue  *float64  `json:"usd_value,omitempty"`
	OrderID   string    `json:"order_id,omitempty"`
	TradeID   string    `json:"trade_id,omitempty"`
}

type NormalizedLiquidationAggregate struct {
	SourceMeta
	Period                   string    `json:"period"`
	BucketTime               time.Time `json:"bucket_time"`
	LongLiquidationCount     int64     `json:"long_liquidation_count"`
	ShortLiquidationCount    int64     `json:"short_liquidation_count"`
	LongLiquidationNotional  float64   `json:"long_liquidation_notional"`
	ShortLiquidationNotional float64   `json:"short_liquidation_notional"`
	TotalLiquidationNotional float64   `json:"total_liquidation_notional"`
	LongLiquidationUSD       float64   `json:"long_liquidation_usd"`
	ShortLiquidationUSD      float64   `json:"short_liquidation_usd"`
	TotalLiquidationUSD      float64   `json:"total_liquidation_usd"`
	LargestLiquidationUSD    float64   `json:"largest_liquidation_usd"`
}

type NormalizedBasisPremium struct {
	SourceMeta
	SnapshotTime           time.Time `json:"snapshot_time"`
	FuturesPrice           *float64  `json:"futures_price,omitempty"`
	MarkPrice              *float64  `json:"mark_price,omitempty"`
	IndexPrice             *float64  `json:"index_price,omitempty"`
	SpotPrice              *float64  `json:"spot_price,omitempty"`
	Basis                  *float64  `json:"basis,omitempty"`
	BasisValue             *float64  `json:"basis_value,omitempty"`
	BasisPercent           *float64  `json:"basis_percent,omitempty"`
	AnnualizedBasisPercent *float64  `json:"annualized_basis_percent,omitempty"`
	PremiumIndex           *float64  `json:"premium_index,omitempty"`
	FundingRate            *float64  `json:"funding_rate,omitempty"`
}

type NormalizedOrderbookImbalance struct {
	SourceMeta
	SnapshotTime     time.Time `json:"snapshot_time"`
	DepthLevels      int       `json:"depth_levels"`
	MidPrice         *float64  `json:"mid_price,omitempty"`
	SpreadBPS        *float64  `json:"spread_bps,omitempty"`
	SpreadPercent    *float64  `json:"spread_percent,omitempty"`
	BidNotional      *float64  `json:"bid_notional,omitempty"`
	AskNotional      *float64  `json:"ask_notional,omitempty"`
	BidDepthUSD      *float64  `json:"bid_depth_usd,omitempty"`
	AskDepthUSD      *float64  `json:"ask_depth_usd,omitempty"`
	BidDepth1PctUSD  *float64  `json:"bid_depth_1pct_usd,omitempty"`
	AskDepth1PctUSD  *float64  `json:"ask_depth_1pct_usd,omitempty"`
	BidDepth2PctUSD  *float64  `json:"bid_depth_2pct_usd,omitempty"`
	AskDepth2PctUSD  *float64  `json:"ask_depth_2pct_usd,omitempty"`
	BidDepth5PctUSD  *float64  `json:"bid_depth_5pct_usd,omitempty"`
	AskDepth5PctUSD  *float64  `json:"ask_depth_5pct_usd,omitempty"`
	ImbalanceRatio   *float64  `json:"imbalance_ratio,omitempty"`
	ImbalancePercent *float64  `json:"imbalance_percent,omitempty"`
}

type NormalizedExchangeDivergence struct {
	SourceMeta
	DataType            string          `json:"data_type"`
	SnapshotTime        time.Time       `json:"snapshot_time"`
	ReferenceExchange   string          `json:"reference_exchange,omitempty"`
	ComparedExchange    string          `json:"compared_exchange,omitempty"`
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
	RawByExchange       json.RawMessage `json:"raw_by_exchange,omitempty"`
}
