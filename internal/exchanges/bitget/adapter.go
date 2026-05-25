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

type klineItem = [7]any

type openInterestRecord struct {
	OpenInterest string `json:"openInterest"`
	Symbol       string `json:"symbol"`
	Ts           string `json:"ts"`
}

type longShortRatioRecord struct {
	LongRatio  string `json:"longRatio"`
	ShortRatio string `json:"shortRatio"`
	Ts         string `json:"ts"`
}

type orderbookRecord struct {
	Asks [][]string `json:"asks"`
	Bids [][]string `json:"bids"`
}

func Normalize(_ context.Context, dataType string, resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	switch dataType {
	case "ticker", "mark_price", "index_price", "funding":
		return normalizeTicker(resp, job, symbol)
	case "kline":
		return normalizeKline(resp, job, symbol)
	case "open_interest":
		return normalizeOpenInterest(resp, job, symbol)
	case "long_short_ratio":
		return normalizeLongShortRatio(resp, job, symbol)
	case "orderbook", "orderbook_imbalance":
		return normalizeOrderbook(resp, job, symbol)
	default:
		return normalizers.NormalizedResult{}, excommon.ErrUnsupportedDataType
	}
}

func normalizeTicker(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
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

func normalizeKline(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var payload envelope
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse bitget response: %w", err)
	}
	if payload.Code != "" && payload.Code != "00000" {
		return normalizers.NormalizedResult{}, fmt.Errorf("%w: bitget code=%s msg=%s", excommon.ErrExchangeResponse, payload.Code, payload.Msg)
	}

	var klines []klineItem
	if err := json.Unmarshal(payload.Data, &klines); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse bitget klines: %w", err)
	}

	var result []normalizers.NormalizedKline
	for _, k := range klines {
		openTime, _ := excommon.MillisToTime(k[0])
		closeTime, _ := excommon.MillisToTime(k[0])
		kline := normalizers.NormalizedKline{
			SourceMeta: normalizers.SourceMeta{
				SymbolID:         symbol.ID,
				Exchange:         "bitget",
				SourceSymbol:     job.SourceSymbol,
				SourceEndpointID: resp.SourceEndpointID,
				RawData:          excommon.RawMessage(k),
			},
			Interval:    job.Period,
			OpenTime:    openTime,
			CloseTime:   closeTime,
			OpenPrice:   parseFloatSafe(k[1]),
			HighPrice:   parseFloatSafe(k[2]),
			LowPrice:    parseFloatSafe(k[3]),
			ClosePrice:  parseFloatSafe(k[4]),
			Volume:      floatPtrSafe(k[5]),
			QuoteVolume: floatPtrSafe(k[6]),
			IsClosed:    true,
		}
		if err := normalizers.ValidateKline(kline); err != nil {
			continue
		}
		result = append(result, kline)
	}

	return normalizers.NormalizedResult{Klines: result}, nil
}

func normalizeOpenInterest(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var payload envelope
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse bitget response: %w", err)
	}
	if payload.Code != "" && payload.Code != "00000" {
		return normalizers.NormalizedResult{}, fmt.Errorf("%w: bitget code=%s msg=%s", excommon.ErrExchangeResponse, payload.Code, payload.Msg)
	}

	var items []openInterestRecord
	if err := json.Unmarshal(payload.Data, &items); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse bitget open interest: %w", err)
	}
	if len(items) == 0 {
		return normalizers.NormalizedResult{}, fmt.Errorf("bitget open interest data is empty")
	}

	item := items[0]
	snapshotTime := resp.CapturedAt
	if item.Ts != "" {
		ts, err := excommon.MillisToTime(item.Ts)
		if err == nil {
			snapshotTime = ts
		}
	}

	oiValue := parseFloatSafe(item.OpenInterest)
	oiItem := normalizers.NormalizedOpenInterest{
		SourceMeta: normalizers.SourceMeta{
			SymbolID:         symbol.ID,
			Exchange:         "bitget",
			SourceSymbol:     job.SourceSymbol,
			SourceEndpointID: resp.SourceEndpointID,
			RawData:          excommon.RawMessage(item),
		},
		SnapshotTime: snapshotTime,
		OpenInterest: oiValue,
	}
	if err := normalizers.ValidateOpenInterest(oiItem, false); err != nil {
		return normalizers.NormalizedResult{}, err
	}

	return normalizers.NormalizedResult{OpenInterest: []normalizers.NormalizedOpenInterest{oiItem}}, nil
}

func normalizeLongShortRatio(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var payload envelope
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse bitget response: %w", err)
	}
	if payload.Code != "" && payload.Code != "00000" {
		return normalizers.NormalizedResult{}, fmt.Errorf("%w: bitget code=%s msg=%s", excommon.ErrExchangeResponse, payload.Code, payload.Msg)
	}

	var items []longShortRatioRecord
	if err := json.Unmarshal(payload.Data, &items); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse bitget long/short ratio: %w", err)
	}

	var result []normalizers.NormalizedLongShortRatio
	for _, item := range items {
		snapshotTime := resp.CapturedAt
		if item.Ts != "" {
			ts, err := excommon.MillisToTime(item.Ts)
			if err == nil {
				snapshotTime = ts
			}
		}

		longRatio := floatPtrSafe(item.LongRatio)
		shortRatio := floatPtrSafe(item.ShortRatio)
		var lsRatio *float64
		if shortRatio != nil && *shortRatio != 0 {
			r := *longRatio / *shortRatio
			lsRatio = &r
		}

		ratio := normalizers.NormalizedLongShortRatio{
			SourceMeta: normalizers.SourceMeta{
				SymbolID:         symbol.ID,
				Exchange:         "bitget",
				SourceSymbol:     job.SourceSymbol,
				SourceEndpointID: resp.SourceEndpointID,
				RawData:          excommon.RawMessage(item),
			},
			Period:            job.Period,
			SnapshotTime:      snapshotTime,
			LongAccountRatio:  longRatio,
			ShortAccountRatio: shortRatio,
			LongShortRatio:    lsRatio,
		}
		if err := normalizers.ValidateLongShortRatio(ratio); err != nil {
			continue
		}
		result = append(result, ratio)
	}

	return normalizers.NormalizedResult{LongShortRatios: result}, nil
}

func normalizeOrderbook(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var payload envelope
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse bitget response: %w", err)
	}
	if payload.Code != "" && payload.Code != "00000" {
		return normalizers.NormalizedResult{}, fmt.Errorf("%w: bitget code=%s msg=%s", excommon.ErrExchangeResponse, payload.Code, payload.Msg)
	}

	var items []orderbookRecord
	if err := json.Unmarshal(payload.Data, &items); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse bitget orderbook: %w", err)
	}
	if len(items) == 0 {
		return normalizers.NormalizedResult{}, fmt.Errorf("bitget orderbook data is empty")
	}

	ob := items[0]
	bidNotional := calcOBNotional(ob.Bids)
	askNotional := calcOBNotional(ob.Asks)
	var imbalance *float64
	if bidNotional+askNotional > 0 {
		im := (bidNotional - askNotional) / (bidNotional + askNotional)
		imbalance = &im
	}

	var midPrice *float64
	if len(ob.Bids) > 0 && len(ob.Asks) > 0 {
		bestBid := parseFloatSafe(ob.Bids[0][0])
		bestAsk := parseFloatSafe(ob.Asks[0][0])
		if bestAsk > 0 {
			m := (bestBid + bestAsk) / 2
			midPrice = &m
		}
	}

	var spreadBPS *float64
	if midPrice != nil && *midPrice > 0 {
		bestBid := parseFloatSafe(ob.Bids[0][0])
		bestAsk := parseFloatSafe(ob.Asks[0][0])
		s := (bestAsk - bestBid) / *midPrice * 10000
		spreadBPS = &s
	}

	obi := normalizers.NormalizedOrderbookImbalance{
		SourceMeta: normalizers.SourceMeta{
			SymbolID:         symbol.ID,
			Exchange:         "bitget",
			SourceSymbol:     job.SourceSymbol,
			SourceEndpointID: resp.SourceEndpointID,
			RawData:          resp.Body,
		},
		SnapshotTime:   resp.CapturedAt,
		DepthLevels:    10,
		MidPrice:       midPrice,
		SpreadBPS:      spreadBPS,
		BidNotional:    &bidNotional,
		AskNotional:    &askNotional,
		ImbalanceRatio: imbalance,
	}
	if err := normalizers.ValidateOrderbookImbalance(obi); err != nil {
		return normalizers.NormalizedResult{}, err
	}

	return normalizers.NormalizedResult{OrderbookImbalances: []normalizers.NormalizedOrderbookImbalance{obi}}, nil
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

func parseFloatSafe(v any) float64 {
	f, _ := excommon.ParseFloat(v)
	return f
}

func floatPtrSafe(v any) *float64 {
	f, _ := excommon.FloatPtr(v)
	return f
}

func intPtr(v float64) *int64 {
	i := int64(v)
	return &i
}

func calcOBNotional(levels [][]string) float64 {
	var total float64
	for _, lvl := range levels {
		if len(lvl) < 2 {
			continue
		}
		price := parseFloatSafe(lvl[0])
		size := parseFloatSafe(lvl[1])
		total += price * size
	}
	return total
}
