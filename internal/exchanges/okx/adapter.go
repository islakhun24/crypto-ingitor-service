package okx

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
		ExchangeName: "okx",
		Client:       client,
		Normalizer:   Normalize,
	}}
}

type envelope struct {
	Code string         `json:"code"`
	Msg  string         `json:"msg"`
	Data []tickerRecord `json:"data"`
}

type tickerRecord struct {
	InstID       string `json:"instId"`
	Last         string `json:"last"`
	BidPx        string `json:"bidPx"`
	AskPx        string `json:"askPx"`
	Vol24h       string `json:"vol24h"`
	VolCcy24h    string `json:"volCcy24h"`
	Change24h    string `json:"change24h"`
	MarkPx       string `json:"markPx"`
	IndexPx      string `json:"idxPx"`
	FundingRate  string `json:"fundingRate"`
	OpenInterest string `json:"oi"`
	Timestamp    string `json:"ts"`
}

func Normalize(_ context.Context, dataType string, resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	if dataType != "ticker" && dataType != "mark_price" && dataType != "funding" {
		return normalizers.NormalizedResult{}, excommon.ErrUnsupportedDataType
	}

	var payload envelope
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse okx response: %w", err)
	}
	if payload.Code != "" && payload.Code != "0" {
		return normalizers.NormalizedResult{}, fmt.Errorf("%w: okx code=%s msg=%s", excommon.ErrExchangeResponse, payload.Code, payload.Msg)
	}
	if len(payload.Data) == 0 {
		return normalizers.NormalizedResult{}, fmt.Errorf("okx response data is empty")
	}

	item := payload.Data[0]
	snapshotTime, err := excommon.MillisToTime(item.Timestamp)
	if err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse okx timestamp: %w", err)
	}
	snapshot, err := excommon.MarketSnapshot(excommon.MarketSnapshotInput{
		Exchange:              "okx",
		SnapshotTime:          snapshotTime,
		LastPrice:             item.Last,
		MarkPrice:             item.MarkPx,
		IndexPrice:            item.IndexPx,
		BidPrice:              item.BidPx,
		AskPrice:              item.AskPx,
		Volume24h:             item.Vol24h,
		QuoteVolume24h:        item.VolCcy24h,
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
