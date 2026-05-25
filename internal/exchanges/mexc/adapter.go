package mexc

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
		ExchangeName: "mexc",
		Client:       client,
		Normalizer:   Normalize,
	}}
}

type envelope struct {
	Success bool            `json:"success"`
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type tickerRecord struct {
	Symbol       string  `json:"symbol"`
	LastPrice    float64 `json:"lastPrice"`
	BidPrice     float64 `json:"bid1"`
	AskPrice     float64 `json:"ask1"`
	Volume24h    float64 `json:"volume24"`
	QuoteVolume  float64 `json:"amount24"`
	OpenInterest float64 `json:"holdVol"`
	IndexPrice   float64 `json:"indexPrice"`
	MarkPrice    float64 `json:"fairPrice"`
	FundingRate  float64 `json:"fundingRate"`
	Timestamp    int64   `json:"timestamp"`
}

func Normalize(_ context.Context, dataType string, resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	if dataType != "ticker" && dataType != "mark_price" && dataType != "index_price" && dataType != "funding" && dataType != "open_interest" {
		return normalizers.NormalizedResult{}, excommon.ErrUnsupportedDataType
	}

	var payload envelope
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse mexc response: %w", err)
	}
	if !payload.Success {
		return normalizers.NormalizedResult{}, fmt.Errorf("%w: mexc code=%d msg=%s", excommon.ErrExchangeResponse, payload.Code, payload.Message)
	}

	item, err := firstTicker(payload.Data)
	if err != nil {
		return normalizers.NormalizedResult{}, err
	}
	snapshotTime, err := excommon.MillisToTime(item.Timestamp)
	if err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse mexc timestamp: %w", err)
	}
	snapshot, err := excommon.MarketSnapshot(excommon.MarketSnapshotInput{
		Exchange:       "mexc",
		SnapshotTime:   snapshotTime,
		LastPrice:      item.LastPrice,
		MarkPrice:      item.MarkPrice,
		IndexPrice:     item.IndexPrice,
		BidPrice:       item.BidPrice,
		AskPrice:       item.AskPrice,
		Volume24h:      item.Volume24h,
		QuoteVolume24h: item.QuoteVolume,
		OpenInterest:   item.OpenInterest,
		FundingRate:    item.FundingRate,
		RawData:        excommon.RawMessage(item),
	}, resp, job, symbol)
	if err != nil {
		return normalizers.NormalizedResult{}, err
	}

	return normalizers.NormalizedResult{MarketSnapshots: []normalizers.NormalizedMarketSnapshot{snapshot}}, nil
}

func firstTicker(raw json.RawMessage) (tickerRecord, error) {
	var list []tickerRecord
	if err := json.Unmarshal(raw, &list); err == nil && len(list) > 0 {
		return list[0], nil
	}

	var item tickerRecord
	if err := json.Unmarshal(raw, &item); err != nil {
		return tickerRecord{}, fmt.Errorf("parse mexc ticker: %w", err)
	}
	if item.Symbol == "" {
		return tickerRecord{}, fmt.Errorf("mexc response data is empty")
	}

	return item, nil
}
