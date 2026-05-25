package derivatives

import (
	"encoding/json"
	"time"
)

type APIError struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

type PageMeta struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	Total      int  `json:"total"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
}

type PagedResponse[T any] struct {
	Data []T      `json:"data"`
	Meta PageMeta `json:"meta"`
}

type SymbolDTO struct {
	ID         int64           `json:"id"`
	Symbol     string          `json:"symbol"`
	BaseAsset  string          `json:"base_asset"`
	QuoteAsset string          `json:"quote_asset"`
	MarketType string          `json:"market_type,omitempty"`
	Category   string          `json:"category,omitempty"`
	Icon       string          `json:"icon,omitempty"`
	CmcRank    int             `json:"cmc_rank,omitempty"`
	IsActive   bool            `json:"is_active"`
	Markets    json.RawMessage `json:"markets,omitempty"`
}

type FreshnessDTO struct {
	SnapshotTime time.Time `json:"snapshot_time"`
	AgeSeconds   int64     `json:"age_seconds"`
	Status       string    `json:"status"`
}

type AggregateDTO struct {
	SnapshotTime                 time.Time       `json:"snapshot_time"`
	Price                        *float64        `json:"price,omitempty"`
	PriceAvg                     *float64        `json:"price_avg,omitempty"`
	PriceWeighted                *float64        `json:"price_weighted,omitempty"`
	PriceChange                  *float64        `json:"price_change,omitempty"`
	PriceChangePercent           *float64        `json:"price_change_percent,omitempty"`
	TotalVolume24h               *float64        `json:"total_volume_24h,omitempty"`
	TotalQuoteVolume24h          *float64        `json:"total_quote_volume_24h,omitempty"`
	TotalOpenInterest            *float64        `json:"total_open_interest,omitempty"`
	TotalOpenInterestValue       *float64        `json:"total_open_interest_value,omitempty"`
	AvgFundingRate               *float64        `json:"avg_funding_rate,omitempty"`
	TotalCVD                     *float64        `json:"total_cvd,omitempty"`
	TotalLongLiquidationUSD      *float64        `json:"total_long_liquidation_usd,omitempty"`
	TotalShortLiquidationUSD     *float64        `json:"total_short_liquidation_usd,omitempty"`
	TotalLiquidationUSD          *float64        `json:"total_liquidation_usd,omitempty"`
	AvgBasisPercent              *float64        `json:"avg_basis_percent,omitempty"`
	AvgOrderbookImbalancePercent *float64        `json:"avg_orderbook_imbalance_percent,omitempty"`
	ExchangeCount                int             `json:"exchange_count"`
	AvailableExchanges           json.RawMessage `json:"available_exchanges,omitempty"`
	RawByExchange                json.RawMessage `json:"raw_by_exchange,omitempty"`
	WindowMetrics                json.RawMessage `json:"window_metrics,omitempty"`
	AnomalyFlags                 json.RawMessage `json:"anomaly_flags,omitempty"`
	Quality                      json.RawMessage `json:"quality,omitempty"`
	Freshness                    FreshnessDTO    `json:"freshness"`
}

type OverviewItem struct {
	Symbol SymbolDTO    `json:"symbol"`
	Market AggregateDTO `json:"market"`
}

type SymbolDetail struct {
	Symbol             SymbolDTO                 `json:"symbol"`
	Market             *AggregateDTO             `json:"market,omitempty"`
	PerExchange        json.RawMessage           `json:"per_exchange,omitempty"`
	Klines             []KlineDTO                `json:"klines,omitempty"`
	OpenInterest       []OpenInterestDTO         `json:"open_interest,omitempty"`
	Funding            []FundingDTO              `json:"funding,omitempty"`
	LongShortRatio     []LongShortRatioDTO       `json:"long_short_ratio,omitempty"`
	TakerFlow          []TakerFlowDTO            `json:"taker_flow,omitempty"`
	CVD                []CVDDTO                  `json:"cvd,omitempty"`
	Liquidations       []LiquidationAggregateDTO `json:"liquidations,omitempty"`
	Basis              []BasisDTO                `json:"basis,omitempty"`
	OrderbookImbalance []OrderbookImbalanceDTO   `json:"orderbook_imbalance,omitempty"`
	ExchangeDivergence []ExchangeDivergenceDTO   `json:"exchange_divergence,omitempty"`
}

type MarketSnapshotDTO struct {
	Exchange       string          `json:"exchange"`
	SourceSymbol   string          `json:"source_symbol"`
	SnapshotTime   time.Time       `json:"snapshot_time"`
	LastPrice      *float64        `json:"last_price,omitempty"`
	MarkPrice      *float64        `json:"mark_price,omitempty"`
	IndexPrice     *float64        `json:"index_price,omitempty"`
	BidPrice       *float64        `json:"bid_price,omitempty"`
	AskPrice       *float64        `json:"ask_price,omitempty"`
	Volume24h      *float64        `json:"volume_24h,omitempty"`
	QuoteVolume24h *float64        `json:"quote_volume_24h,omitempty"`
	OpenInterest   *float64        `json:"open_interest,omitempty"`
	FundingRate    *float64        `json:"funding_rate,omitempty"`
	Raw            json.RawMessage `json:"raw,omitempty"`
}

type KlineDTO struct {
	Exchange    string    `json:"exchange"`
	Interval    string    `json:"interval"`
	OpenTime    time.Time `json:"open_time"`
	CloseTime   time.Time `json:"close_time"`
	Open        float64   `json:"open"`
	High        float64   `json:"high"`
	Low         float64   `json:"low"`
	Close       float64   `json:"close"`
	Volume      *float64  `json:"volume,omitempty"`
	QuoteVolume *float64  `json:"quote_volume,omitempty"`
	TradeCount  *int64    `json:"trade_count,omitempty"`
	IsClosed    bool      `json:"is_closed"`
}

type OpenInterestDTO struct {
	Exchange          string    `json:"exchange"`
	Period            string    `json:"period,omitempty"`
	Timestamp         time.Time `json:"timestamp"`
	OpenInterest      float64   `json:"open_interest"`
	OpenInterestValue *float64  `json:"open_interest_value,omitempty"`
}

type FundingDTO struct {
	Exchange        string     `json:"exchange"`
	Timestamp       time.Time  `json:"timestamp"`
	FundingRate     float64    `json:"funding_rate"`
	RealizedRate    *float64   `json:"realized_rate,omitempty"`
	NextFundingTime *time.Time `json:"next_funding_time,omitempty"`
	MarkPrice       *float64   `json:"mark_price,omitempty"`
}

type LongShortRatioDTO struct {
	Exchange            string    `json:"exchange"`
	Period              string    `json:"period"`
	SnapshotTime        time.Time `json:"snapshot_time"`
	LongAccountRatio    *float64  `json:"long_account_ratio,omitempty"`
	ShortAccountRatio   *float64  `json:"short_account_ratio,omitempty"`
	LongShortRatio      *float64  `json:"long_short_ratio,omitempty"`
	TopTraderLongRatio  *float64  `json:"top_trader_long_ratio,omitempty"`
	TopTraderShortRatio *float64  `json:"top_trader_short_ratio,omitempty"`
}

type TakerFlowDTO struct {
	Exchange             string    `json:"exchange"`
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

type CVDDTO struct {
	Exchange         string    `json:"exchange"`
	Period           string    `json:"period"`
	SnapshotTime     time.Time `json:"snapshot_time"`
	CVDValue         *float64  `json:"cvd_value,omitempty"`
	CVDDelta         *float64  `json:"cvd_delta,omitempty"`
	CVDChange        *float64  `json:"cvd_change,omitempty"`
	CVDChangePercent *float64  `json:"cvd_change_percent,omitempty"`
}

type LiquidationAggregateDTO struct {
	Exchange              string    `json:"exchange"`
	Period                string    `json:"period"`
	BucketTime            time.Time `json:"bucket_time"`
	LongCount             int64     `json:"long_count"`
	ShortCount            int64     `json:"short_count"`
	LongLiquidationUSD    float64   `json:"long_liquidation_usd"`
	ShortLiquidationUSD   float64   `json:"short_liquidation_usd"`
	TotalLiquidationUSD   float64   `json:"total_liquidation_usd"`
	LargestLiquidationUSD float64   `json:"largest_liquidation_usd"`
}

type BasisDTO struct {
	Exchange               string    `json:"exchange"`
	SnapshotTime           time.Time `json:"snapshot_time"`
	FuturesPrice           *float64  `json:"futures_price,omitempty"`
	SpotPrice              *float64  `json:"spot_price,omitempty"`
	MarkPrice              *float64  `json:"mark_price,omitempty"`
	IndexPrice             *float64  `json:"index_price,omitempty"`
	BasisValue             *float64  `json:"basis_value,omitempty"`
	BasisPercent           *float64  `json:"basis_percent,omitempty"`
	AnnualizedBasisPercent *float64  `json:"annualized_basis_percent,omitempty"`
	FundingRate            *float64  `json:"funding_rate,omitempty"`
}

type OrderbookImbalanceDTO struct {
	Exchange         string    `json:"exchange"`
	SnapshotTime     time.Time `json:"snapshot_time"`
	DepthLevels      int       `json:"depth_levels"`
	MidPrice         *float64  `json:"mid_price,omitempty"`
	SpreadPercent    *float64  `json:"spread_percent,omitempty"`
	BidDepthUSD      *float64  `json:"bid_depth_usd,omitempty"`
	AskDepthUSD      *float64  `json:"ask_depth_usd,omitempty"`
	ImbalancePercent *float64  `json:"imbalance_percent,omitempty"`
}

type ExchangeDivergenceDTO struct {
	DataType            string          `json:"data_type"`
	SnapshotTime        time.Time       `json:"snapshot_time"`
	PriceSpreadPercent  *float64        `json:"price_spread_percent,omitempty"`
	OISpreadPercent     *float64        `json:"oi_spread_percent,omitempty"`
	FundingSpread       *float64        `json:"funding_spread,omitempty"`
	VolumeSpreadPercent *float64        `json:"volume_spread_percent,omitempty"`
	StrongestExchange   string          `json:"strongest_exchange,omitempty"`
	WeakestExchange     string          `json:"weakest_exchange,omitempty"`
	RawByExchange       json.RawMessage `json:"raw_by_exchange,omitempty"`
	Metadata            json.RawMessage `json:"metadata,omitempty"`
}

type CollectorHealthDTO struct {
	ServiceName   string          `json:"service_name"`
	InstanceID    string          `json:"instance_id"`
	Exchange      string          `json:"exchange,omitempty"`
	DataType      string          `json:"data_type,omitempty"`
	Status        string          `json:"status"`
	HeartbeatAt   time.Time       `json:"heartbeat_at"`
	LastSuccessAt *time.Time      `json:"last_success_at,omitempty"`
	LastErrorAt   *time.Time      `json:"last_error_at,omitempty"`
	ErrorMessage  string          `json:"error_message,omitempty"`
	Metrics       json.RawMessage `json:"metrics,omitempty"`
}

type ExchangeHealthDTO struct {
	Exchange        string    `json:"exchange"`
	Status          string    `json:"status"`
	LastHeartbeatAt time.Time `json:"last_heartbeat_at"`
	HealthyCount    int       `json:"healthy_count"`
	DegradedCount   int       `json:"degraded_count"`
	UnhealthyCount  int       `json:"unhealthy_count"`
}

type JobDTO struct {
	ID            int64     `json:"id"`
	Exchange      string    `json:"exchange"`
	DataType      string    `json:"data_type"`
	Tier          string    `json:"tier"`
	SymbolID      int64     `json:"symbol_id,omitempty"`
	SourceSymbol  string    `json:"source_symbol,omitempty"`
	Period        string    `json:"period,omitempty"`
	Status        string    `json:"status"`
	Priority      int       `json:"priority"`
	ScheduledAt   time.Time `json:"scheduled_at"`
	RetryCount    int       `json:"retry_count"`
	MaxRetry      int       `json:"max_retry"`
	LastErrorType string    `json:"last_error_type,omitempty"`
	ErrorMessage  string    `json:"error_message,omitempty"`
	JobMode       string    `json:"job_mode"`
}

type QualityIssueDTO struct {
	IssueKey     string          `json:"issue_key"`
	Severity     string          `json:"severity"`
	Exchange     string          `json:"exchange,omitempty"`
	DataType     string          `json:"data_type"`
	SymbolID     int64           `json:"symbol_id,omitempty"`
	SourceSymbol string          `json:"source_symbol,omitempty"`
	IssueType    string          `json:"issue_type"`
	Status       string          `json:"status"`
	LastSeenAt   time.Time       `json:"last_seen_at"`
	Details      json.RawMessage `json:"details,omitempty"`
}

type DataGapDTO struct {
	GapKey                  string          `json:"gap_key"`
	SymbolID                int64           `json:"symbol_id"`
	Exchange                string          `json:"exchange"`
	DataType                string          `json:"data_type"`
	Period                  string          `json:"period,omitempty"`
	GapStart                time.Time       `json:"gap_start"`
	GapEnd                  time.Time       `json:"gap_end"`
	BackfillStatus          string          `json:"backfill_status"`
	ExpectedIntervalSeconds int             `json:"expected_interval_seconds,omitempty"`
	LastObservedAt          *time.Time      `json:"last_observed_at,omitempty"`
	Metadata                json.RawMessage `json:"metadata,omitempty"`
}
