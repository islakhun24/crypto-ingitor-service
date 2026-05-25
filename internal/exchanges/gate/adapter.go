package gate

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
		ExchangeName: "gate",
		Client:       client,
		Normalizer:   Normalize,
	}}
}

type errorEnvelope struct {
	Label   string `json:"label"`
	Message string `json:"message"`
}

type tickerRecord struct {
	Contract         string `json:"contract"`
	Last             string `json:"last"`
	MarkPrice        string `json:"mark_price"`
	IndexPrice       string `json:"index_price"`
	BidPrice         string `json:"highest_bid"`
	AskPrice         string `json:"lowest_ask"`
	VolumeBase24h    string `json:"volume_24h_base"`
	VolumeQuote24h   string `json:"volume_24h_quote"`
	ChangePercentage string `json:"change_percentage"`
	OpenInterest     string `json:"total_size"`
	FundingRate      string `json:"funding_rate"`
}

func Normalize(_ context.Context, dataType string, resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	if dataType != "ticker" && dataType != "mark_price" && dataType != "index_price" && dataType != "funding" && dataType != "open_interest" {
		return normalizers.NormalizedResult{}, excommon.ErrUnsupportedDataType
	}

	var errResp errorEnvelope
	if err := json.Unmarshal(resp.Body, &errResp); err == nil && errResp.Label != "" {
		return normalizers.NormalizedResult{}, fmt.Errorf("%w: gate label=%s msg=%s", excommon.ErrExchangeResponse, errResp.Label, errResp.Message)
	}

	var items []tickerRecord
	if err := json.Unmarshal(resp.Body, &items); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse gate ticker: %w", err)
	}
	if len(items) == 0 {
		return normalizers.NormalizedResult{}, fmt.Errorf("gate response data is empty")
	}

	item := items[0]
	snapshot, err := excommon.MarketSnapshot(excommon.MarketSnapshotInput{
		Exchange:              "gate",
		SnapshotTime:          resp.CapturedAt,
		LastPrice:             item.Last,
		MarkPrice:             item.MarkPrice,
		IndexPrice:            item.IndexPrice,
		BidPrice:              item.BidPrice,
		AskPrice:              item.AskPrice,
		Volume24h:             item.VolumeBase24h,
		QuoteVolume24h:        item.VolumeQuote24h,
		PriceChangePercent24h: item.ChangePercentage,
		OpenInterest:          item.OpenInterest,
		FundingRate:           item.FundingRate,
		RawData:               excommon.RawMessage(item),
	}, resp, job, symbol)
	if err != nil {
		return normalizers.NormalizedResult{}, err
	}

	return normalizers.NormalizedResult{MarketSnapshots: []normalizers.NormalizedMarketSnapshot{snapshot}}, nil
}
