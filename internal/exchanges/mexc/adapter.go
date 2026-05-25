package mexc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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

type klineData struct {
	Time  []int64   `json:"time"`
	Open  []float64 `json:"open"`
	High  []float64 `json:"high"`
	Low   []float64 `json:"low"`
	Close []float64 `json:"close"`
	Vol   []float64 `json:"vol"`
}

type orderbookData struct {
	Asks [][]float64 `json:"asks"`
	Bids [][]float64 `json:"bids"`
}

type liquidationItem struct {
	Symbol string  `json:"symbol"`
	Price  float64 `json:"price"`
	Qty    float64 `json:"qty"`
	Side   string  `json:"side"`
	Time   int64   `json:"time"`
}

type liquidationData struct {
	Items []liquidationItem `json:"items"`
}

func Normalize(_ context.Context, dataType string, resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	switch dataType {
	case "ticker", "mark_price", "index_price", "funding":
		return normalizeTicker(resp, job, symbol)
	case "open_interest":
		return normalizeOpenInterest(resp, job, symbol)
	case "kline":
		return normalizeKline(resp, job, symbol)
	case "orderbook", "orderbook_imbalance":
		return normalizeOrderbook(resp, job, symbol)
	case "liquidation":
		return normalizeLiquidation(resp, job, symbol)
	default:
		return normalizers.NormalizedResult{}, excommon.ErrUnsupportedDataType
	}
}

func normalizeTicker(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
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

func normalizeOpenInterest(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
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
	snapshotTime := resp.CapturedAt
	if item.Timestamp > 0 {
		snapshotTime = excommon.MustTime(excommon.MillisToTime(item.Timestamp))
	}

	oiItem := normalizers.NormalizedOpenInterest{
		SourceMeta: normalizers.SourceMeta{
			SymbolID:         symbol.ID,
			Exchange:         "mexc",
			SourceSymbol:     job.SourceSymbol,
			SourceEndpointID: resp.SourceEndpointID,
			RawData:          excommon.RawMessage(item),
		},
		SnapshotTime: snapshotTime,
		OpenInterest: item.OpenInterest,
	}
	if err := normalizers.ValidateOpenInterest(oiItem, false); err != nil {
		return normalizers.NormalizedResult{}, err
	}

	return normalizers.NormalizedResult{OpenInterest: []normalizers.NormalizedOpenInterest{oiItem}}, nil
}

func normalizeKline(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var payload envelope
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse mexc response: %w", err)
	}
	if !payload.Success {
		return normalizers.NormalizedResult{}, fmt.Errorf("%w: mexc code=%d msg=%s", excommon.ErrExchangeResponse, payload.Code, payload.Message)
	}

	var kd klineData
	if err := json.Unmarshal(payload.Data, &kd); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse mexc kline: %w", err)
	}

	var result []normalizers.NormalizedKline
	for i := range kd.Time {
		if i >= len(kd.Open) || i >= len(kd.High) || i >= len(kd.Low) || i >= len(kd.Close) || i >= len(kd.Vol) {
			continue
		}
		openTime := time.Unix(kd.Time[i], 0)
		closeTime := openTime.Add(time.Minute) // approximate; MEXC kline doesn't provide closeTime

		kline := normalizers.NormalizedKline{
			SourceMeta: normalizers.SourceMeta{
				SymbolID:         symbol.ID,
				Exchange:         "mexc",
				SourceSymbol:     job.SourceSymbol,
				SourceEndpointID: resp.SourceEndpointID,
				RawData:          excommon.RawMessage(map[string]any{"time": kd.Time[i], "open": kd.Open[i], "high": kd.High[i], "low": kd.Low[i], "close": kd.Close[i], "vol": kd.Vol[i]}),
			},
			Interval:   job.Period,
			OpenTime:   openTime,
			CloseTime:  closeTime,
			OpenPrice:  kd.Open[i],
			HighPrice:  kd.High[i],
			LowPrice:   kd.Low[i],
			ClosePrice: kd.Close[i],
			Volume:     floatPtr(kd.Vol[i]),
			IsClosed:   true,
		}
		if err := normalizers.ValidateKline(kline); err != nil {
			continue
		}
		result = append(result, kline)
	}

	return normalizers.NormalizedResult{Klines: result}, nil
}

func normalizeOrderbook(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var payload envelope
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse mexc response: %w", err)
	}
	if !payload.Success {
		return normalizers.NormalizedResult{}, fmt.Errorf("%w: mexc code=%d msg=%s", excommon.ErrExchangeResponse, payload.Code, payload.Message)
	}

	var ob orderbookData
	if err := json.Unmarshal(payload.Data, &ob); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse mexc orderbook: %w", err)
	}

	bidNotional := calcOBNotional(ob.Bids)
	askNotional := calcOBNotional(ob.Asks)
	var imbalance *float64
	if bidNotional+askNotional > 0 {
		im := (bidNotional - askNotional) / (bidNotional + askNotional)
		imbalance = &im
	}

	var midPrice *float64
	if len(ob.Bids) > 0 && len(ob.Asks) > 0 {
		bestBid := ob.Bids[0][0]
		bestAsk := ob.Asks[0][0]
		if bestAsk > 0 {
			m := (bestBid + bestAsk) / 2
			midPrice = &m
		}
	}

	var spreadBPS *float64
	if midPrice != nil && *midPrice > 0 {
		bestBid := ob.Bids[0][0]
		bestAsk := ob.Asks[0][0]
		s := (bestAsk - bestBid) / *midPrice * 10000
		spreadBPS = &s
	}

	obi := normalizers.NormalizedOrderbookImbalance{
		SourceMeta: normalizers.SourceMeta{
			SymbolID:         symbol.ID,
			Exchange:         "mexc",
			SourceSymbol:     job.SourceSymbol,
			SourceEndpointID: resp.SourceEndpointID,
			RawData:          excommon.RawMessage(ob),
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

func normalizeLiquidation(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var payload envelope
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse mexc response: %w", err)
	}
	if !payload.Success {
		return normalizers.NormalizedResult{}, fmt.Errorf("%w: mexc code=%d msg=%s", excommon.ErrExchangeResponse, payload.Code, payload.Message)
	}

	var ld liquidationData
	if err := json.Unmarshal(payload.Data, &ld); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse mexc liquidation: %w", err)
	}

	var events []normalizers.NormalizedLiquidationEvent
	var aggregates []normalizers.NormalizedLiquidationAggregate
	bucketMap := make(map[string]*normalizers.NormalizedLiquidationAggregate)
	bucketTime := resp.CapturedAt.Truncate(5 * time.Minute)
	bucketKey := bucketTime.Format(time.RFC3339)

	for _, item := range ld.Items {
		eventTime := time.Unix(item.Time, 0)
		price := floatPtr(item.Price)
		qty := floatPtr(item.Qty)
		var notional float64
		if price != nil && qty != nil {
			notional = *price * *qty
		}

		side := "BUY"
		if item.Side == "LONG" {
			side = "SELL"
		}

		event := normalizers.NormalizedLiquidationEvent{
			SourceMeta: normalizers.SourceMeta{
				SymbolID:         symbol.ID,
				Exchange:         "mexc",
				SourceSymbol:     job.SourceSymbol,
				SourceEndpointID: resp.SourceEndpointID,
				RawData:          excommon.RawMessage(item),
			},
			EventKey:  fmt.Sprintf("%s-%s-%d", item.Symbol, item.Side, item.Time),
			EventTime: eventTime,
			Side:      side,
			Price:     price,
			Quantity:  qty,
			Notional:  &notional,
			USDValue:  &notional,
		}
		if err := normalizers.ValidateLiquidationEvent(event); err != nil {
			continue
		}
		events = append(events, event)

		agg, ok := bucketMap[bucketKey]
		if !ok {
			agg = &normalizers.NormalizedLiquidationAggregate{
				SourceMeta: normalizers.SourceMeta{
					SymbolID:     symbol.ID,
					Exchange:     "mexc",
					SourceSymbol: job.SourceSymbol,
				},
				Period:     "5m",
				BucketTime: bucketTime,
			}
			bucketMap[bucketKey] = agg
		}
		agg.TotalLiquidationNotional += notional
		agg.TotalLiquidationUSD += notional
		if notional > agg.LargestLiquidationUSD {
			agg.LargestLiquidationUSD = notional
		}
		if side == "SELL" {
			agg.LongLiquidationCount++
			agg.LongLiquidationNotional += notional
			agg.LongLiquidationUSD += notional
		} else {
			agg.ShortLiquidationCount++
			agg.ShortLiquidationNotional += notional
			agg.ShortLiquidationUSD += notional
		}
	}

	for _, agg := range bucketMap {
		agg.SourceEndpointID = resp.SourceEndpointID
		agg.RawData = resp.Body
		aggregates = append(aggregates, *agg)
	}

	return normalizers.NormalizedResult{
		LiquidationEvents:     events,
		LiquidationAggregates: aggregates,
	}, nil
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

func floatPtr(v float64) *float64 {
	return &v
}

func calcOBNotional(levels [][]float64) float64 {
	var total float64
	for _, lvl := range levels {
		if len(lvl) < 2 {
			continue
		}
		price := lvl[0]
		size := lvl[1]
		total += price * size
	}
	return total
}
