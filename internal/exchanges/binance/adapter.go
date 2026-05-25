package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	excommon "aggregator-services/internal/exchanges/common"
	"aggregator-services/internal/normalizers"
	"aggregator-services/internal/scheduler"
	"aggregator-services/internal/symbols"
)

type Adapter struct {
	excommon.BaseAdapter
}

func NewAdapter(client *http.Client) Adapter {
	return Adapter{BaseAdapter: excommon.BaseAdapter{
		ExchangeName: "binance",
		Client:       client,
		Normalizer:   Normalize,
	}}
}

type errorEnvelope struct {
	Code *int   `json:"code"`
	Msg  string `json:"msg"`
}

type tickerResponse struct {
	Symbol             string `json:"symbol"`
	LastPrice          string `json:"lastPrice"`
	MarkPrice          string `json:"markPrice"`
	IndexPrice         string `json:"indexPrice"`
	BidPrice           string `json:"bidPrice"`
	AskPrice           string `json:"askPrice"`
	Volume             string `json:"volume"`
	QuoteVolume        string `json:"quoteVolume"`
	PriceChangePercent string `json:"priceChangePercent"`
	OpenInterest       string `json:"openInterest"`
	FundingRate        string `json:"lastFundingRate"`
	CloseTime          int64  `json:"closeTime"`
	Time               int64  `json:"time"`
}

func Normalize(_ context.Context, dataType string, resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	if dataType != "ticker" && dataType != "mark_price" && dataType != "index_price" && dataType != "funding" {
		return normalizers.NormalizedResult{}, excommon.ErrUnsupportedDataType
	}

	var errResp errorEnvelope
	if err := json.Unmarshal(resp.Body, &errResp); err == nil && errResp.Code != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("%w: binance code=%d msg=%s", excommon.ErrExchangeResponse, *errResp.Code, errResp.Msg)
	}

	var ticker tickerResponse
	if err := json.Unmarshal(resp.Body, &ticker); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse binance ticker: %w", err)
	}

	snapshotTime := resp.CapturedAt
	if ticker.CloseTime > 0 {
		snapshotTime = excommon.MustTime(excommon.MillisToTime(ticker.CloseTime))
	}
	if ticker.Time > 0 {
		snapshotTime = excommon.MustTime(excommon.MillisToTime(ticker.Time))
	}

	snapshot, err := excommon.MarketSnapshot(excommon.MarketSnapshotInput{
		Exchange:              "binance",
		SnapshotTime:          snapshotTime,
		LastPrice:             ticker.LastPrice,
		MarkPrice:             ticker.MarkPrice,
		IndexPrice:            ticker.IndexPrice,
		BidPrice:              ticker.BidPrice,
		AskPrice:              ticker.AskPrice,
		Volume24h:             ticker.Volume,
		QuoteVolume24h:        ticker.QuoteVolume,
		PriceChangePercent24h: ticker.PriceChangePercent,
		OpenInterest:          ticker.OpenInterest,
		FundingRate:           ticker.FundingRate,
	}, resp, job, symbol)
	if err != nil {
		return normalizers.NormalizedResult{}, err
	}

	return normalizers.NormalizedResult{MarketSnapshots: []normalizers.NormalizedMarketSnapshot{snapshot}}, nil
}
