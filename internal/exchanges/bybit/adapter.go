package bybit

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
		ExchangeName: "bybit",
		Client:       client,
		Normalizer:   Normalize,
	}}
}

type envelope struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		List []tickerRecord `json:"list"`
	} `json:"result"`
	Time int64 `json:"time"`
}

type tickerRecord struct {
	Symbol          string `json:"symbol"`
	LastPrice       string `json:"lastPrice"`
	MarkPrice       string `json:"markPrice"`
	IndexPrice      string `json:"indexPrice"`
	Bid1Price       string `json:"bid1Price"`
	Ask1Price       string `json:"ask1Price"`
	Volume24h       string `json:"volume24h"`
	Turnover24h     string `json:"turnover24h"`
	Price24hPcnt    string `json:"price24hPcnt"`
	OpenInterest    string `json:"openInterest"`
	FundingRate     string `json:"fundingRate"`
	NextFundingTime string `json:"nextFundingTime"`
}

func Normalize(_ context.Context, dataType string, resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	if dataType != "ticker" && dataType != "mark_price" && dataType != "index_price" && dataType != "funding" {
		return normalizers.NormalizedResult{}, excommon.ErrUnsupportedDataType
	}

	var payload envelope
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse bybit response: %w", err)
	}
	if payload.RetCode != 0 {
		return normalizers.NormalizedResult{}, fmt.Errorf("%w: bybit code=%d msg=%s", excommon.ErrExchangeResponse, payload.RetCode, payload.RetMsg)
	}
	if len(payload.Result.List) == 0 {
		return normalizers.NormalizedResult{}, fmt.Errorf("bybit response list is empty")
	}

	item := payload.Result.List[0]
	snapshotTime, err := excommon.MillisToTime(payload.Time)
	if err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse bybit timestamp: %w", err)
	}
	snapshot, err := excommon.MarketSnapshot(excommon.MarketSnapshotInput{
		Exchange:              "bybit",
		SnapshotTime:          snapshotTime,
		LastPrice:             item.LastPrice,
		MarkPrice:             item.MarkPrice,
		IndexPrice:            item.IndexPrice,
		BidPrice:              item.Bid1Price,
		AskPrice:              item.Ask1Price,
		Volume24h:             item.Volume24h,
		QuoteVolume24h:        item.Turnover24h,
		PriceChangePercent24h: item.Price24hPcnt,
		OpenInterest:          item.OpenInterest,
		FundingRate:           item.FundingRate,
		RawData:               excommon.RawMessage(item),
	}, resp, job, symbol)
	if err != nil {
		return normalizers.NormalizedResult{}, err
	}

	return normalizers.NormalizedResult{MarketSnapshots: []normalizers.NormalizedMarketSnapshot{snapshot}}, nil
}
