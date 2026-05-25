package common

import (
	"fmt"
	"time"

	"aggregator-services/internal/normalizers"
	"aggregator-services/internal/scheduler"
	"aggregator-services/internal/symbols"
)

type MarketSnapshotInput struct {
	Exchange              string
	SnapshotTime          time.Time
	LastPrice             any
	MarkPrice             any
	IndexPrice            any
	BidPrice              any
	AskPrice              any
	Volume24h             any
	QuoteVolume24h        any
	PriceChangePercent24h any
	OpenInterest          any
	FundingRate           any
	RawData               []byte
}

func MarketSnapshot(input MarketSnapshotInput, resp *ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedMarketSnapshot, error) {
	if input.SnapshotTime.IsZero() {
		return normalizers.NormalizedMarketSnapshot{}, fmt.Errorf("missing snapshot_time")
	}

	lastPrice, err := FloatPtr(input.LastPrice)
	if err != nil {
		return normalizers.NormalizedMarketSnapshot{}, fmt.Errorf("last_price: %w", err)
	}
	markPrice, err := FloatPtr(input.MarkPrice)
	if err != nil {
		return normalizers.NormalizedMarketSnapshot{}, fmt.Errorf("mark_price: %w", err)
	}
	indexPrice, err := FloatPtr(input.IndexPrice)
	if err != nil {
		return normalizers.NormalizedMarketSnapshot{}, fmt.Errorf("index_price: %w", err)
	}
	bidPrice, err := FloatPtr(input.BidPrice)
	if err != nil {
		return normalizers.NormalizedMarketSnapshot{}, fmt.Errorf("bid_price: %w", err)
	}
	askPrice, err := FloatPtr(input.AskPrice)
	if err != nil {
		return normalizers.NormalizedMarketSnapshot{}, fmt.Errorf("ask_price: %w", err)
	}
	volume, err := FloatPtr(input.Volume24h)
	if err != nil {
		return normalizers.NormalizedMarketSnapshot{}, fmt.Errorf("volume_24h: %w", err)
	}
	quoteVolume, err := FloatPtr(input.QuoteVolume24h)
	if err != nil {
		return normalizers.NormalizedMarketSnapshot{}, fmt.Errorf("quote_volume_24h: %w", err)
	}
	priceChange, err := FloatPtr(input.PriceChangePercent24h)
	if err != nil {
		return normalizers.NormalizedMarketSnapshot{}, fmt.Errorf("price_change_percent_24h: %w", err)
	}
	openInterest, err := FloatPtr(input.OpenInterest)
	if err != nil {
		return normalizers.NormalizedMarketSnapshot{}, fmt.Errorf("open_interest: %w", err)
	}
	fundingRate, err := FloatPtr(input.FundingRate)
	if err != nil {
		return normalizers.NormalizedMarketSnapshot{}, fmt.Errorf("funding_rate: %w", err)
	}

	rawData := input.RawData
	if len(rawData) == 0 {
		rawData = resp.Body
	}

	snapshot := normalizers.NormalizedMarketSnapshot{
		SourceMeta: normalizers.SourceMeta{
			SymbolID:         symbol.ID,
			Exchange:         input.Exchange,
			SourceSymbol:     job.SourceSymbol,
			SourceEndpointID: resp.SourceEndpointID,
			RawData:          rawData,
		},
		SnapshotTime:          input.SnapshotTime,
		LastPrice:             lastPrice,
		MarkPrice:             markPrice,
		IndexPrice:            indexPrice,
		BidPrice:              bidPrice,
		AskPrice:              askPrice,
		Volume24h:             volume,
		QuoteVolume24h:        quoteVolume,
		PriceChangePercent24h: priceChange,
		OpenInterest:          openInterest,
		FundingRate:           fundingRate,
	}
	if err := normalizers.ValidateMarketSnapshot(snapshot); err != nil {
		return normalizers.NormalizedMarketSnapshot{}, err
	}

	return snapshot, nil
}
