package bitget

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
		ExchangeName: "bitget",
		Client:       client,
		Normalizer:   Normalize,
	}}
}

type envelope struct {
	Code        string          `json:"code"`
	Msg         string          `json:"msg"`
	RequestTime int64           `json:"requestTime"`
	Data        json.RawMessage `json:"data"`
}

type tickerRecord struct {
	Symbol       string `json:"symbol"`
	LastPrice    string `json:"lastPr"`
	MarkPrice    string `json:"markPrice"`
	IndexPrice   string `json:"indexPrice"`
	BidPrice     string `json:"bidPr"`
	AskPrice     string `json:"askPr"`
	BaseVolume   string `json:"baseVolume"`
	QuoteVolume  string `json:"quoteVolume"`
	Change24h    string `json:"change24h"`
	OpenInterest string `json:"openInterest"`
	FundingRate  string `json:"fundingRate"`
}

func Normalize(_ context.Context, dataType string, resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	if dataType != "ticker" && dataType != "mark_price" && dataType != "index_price" && dataType != "funding" {
		return normalizers.NormalizedResult{}, excommon.ErrUnsupportedDataType
	}

	var payload envelope
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse bitget response: %w", err)
	}
	if payload.Code != "" && payload.Code != "00000" {
		return normalizers.NormalizedResult{}, fmt.Errorf("%w: bitget code=%s msg=%s", excommon.ErrExchangeResponse, payload.Code, payload.Msg)
	}

	item, err := firstTicker(payload.Data)
	if err != nil {
		return normalizers.NormalizedResult{}, err
	}
	snapshotTime, err := excommon.MillisToTime(payload.RequestTime)
	if err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse bitget timestamp: %w", err)
	}
	snapshot, err := excommon.MarketSnapshot(excommon.MarketSnapshotInput{
		Exchange:              "bitget",
		SnapshotTime:          snapshotTime,
		LastPrice:             item.LastPrice,
		MarkPrice:             item.MarkPrice,
		IndexPrice:            item.IndexPrice,
		BidPrice:              item.BidPrice,
		AskPrice:              item.AskPrice,
		Volume24h:             item.BaseVolume,
		QuoteVolume24h:        item.QuoteVolume,
		PriceChangePercent24h: item.Change24h,
		OpenInterest:          item.OpenInterest,
		FundingRate:           item.FundingRate,
		RawData:               excommon.RawMessage(item),
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
		return tickerRecord{}, fmt.Errorf("parse bitget ticker: %w", err)
	}
	if item.Symbol == "" {
		return tickerRecord{}, fmt.Errorf("bitget response data is empty")
	}

	return item, nil
}
