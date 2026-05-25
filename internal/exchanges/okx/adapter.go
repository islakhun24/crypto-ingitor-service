package okx

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
		ExchangeName: "okx",
		Client:       client,
		Normalizer:   Normalize,
	}}
}

type envelope struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
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

type klineItem = [9]any

type openInterestRecord struct {
	OI    string `json:"oi"`
	OICcy string `json:"oiCcy"`
	TS    string `json:"ts"`
}

type longShortRatioItem = [3]any

type takerVolumeItem = [3]any

type orderbookRecord struct {
	Asks [][]string `json:"asks"`
	Bids [][]string `json:"bids"`
	TS   string     `json:"ts"`
}

type liquidationRecord struct {
	InstID  string `json:"instId"`
	PosSide string `json:"posSide"`
	BkPx    string `json:"bkPx"`
	Sz      string `json:"sz"`
	TS      string `json:"ts"`
}

func Normalize(_ context.Context, dataType string, resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
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

	switch dataType {
	case "ticker", "mark_price", "funding":
		return normalizeTicker(payload.Data, resp, job, symbol)
	case "kline":
		return normalizeKline(payload.Data, resp, job, symbol)
	case "open_interest":
		return normalizeOpenInterest(payload.Data, resp, job, symbol)
	case "long_short_ratio":
		return normalizeLongShortRatio(payload.Data, resp, job, symbol)
	case "taker_flow":
		return normalizeTakerFlow(payload.Data, resp, job, symbol)
	case "orderbook", "orderbook_imbalance":
		return normalizeOrderbook(payload.Data, resp, job, symbol)
	case "liquidation":
		return normalizeLiquidation(payload.Data, resp, job, symbol)
	default:
		return normalizers.NormalizedResult{}, excommon.ErrUnsupportedDataType
	}
}

func normalizeTicker(data json.RawMessage, resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var records []tickerRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse okx ticker: %w", err)
	}
	if len(records) == 0 {
		return normalizers.NormalizedResult{}, fmt.Errorf("okx ticker data is empty")
	}

	item := records[0]
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

func normalizeKline(data json.RawMessage, resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var klines []klineItem
	if err := json.Unmarshal(data, &klines); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse okx klines: %w", err)
	}

	var result []normalizers.NormalizedKline
	for _, k := range klines {
		openTime := excommon.MustTime(excommon.MillisToTime(k[0]))
		closeTime := openTime

		isClosed := true
		if len(k) > 8 {
			if confirm, ok := k[8].(float64); ok {
				isClosed = confirm == 1
			} else if confirm, ok := k[8].(string); ok {
				isClosed = confirm == "1"
			} else if confirm, ok := k[8].(json.Number); ok {
				v, _ := confirm.Float64()
				isClosed = v == 1
			}
		}

		kline := normalizers.NormalizedKline{
			SourceMeta: normalizers.SourceMeta{
				SymbolID:         symbol.ID,
				Exchange:         "okx",
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
			QuoteVolume: floatPtrSafe(k[7]),
			IsClosed:    isClosed,
		}
		if err := normalizers.ValidateKline(kline); err != nil {
			continue
		}
		result = append(result, kline)
	}

	return normalizers.NormalizedResult{Klines: result}, nil
}

func normalizeOpenInterest(data json.RawMessage, resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var records []openInterestRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse okx open interest: %w", err)
	}
	if len(records) == 0 {
		return normalizers.NormalizedResult{}, fmt.Errorf("okx open interest data is empty")
	}

	item := records[0]
	snapshotTime := excommon.MustTime(excommon.MillisToTime(item.TS))
	oiValue := parseFloatSafe(item.OI)
	oiItem := normalizers.NormalizedOpenInterest{
		SourceMeta: normalizers.SourceMeta{
			SymbolID:         symbol.ID,
			Exchange:         "okx",
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

func normalizeLongShortRatio(data json.RawMessage, resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var items []longShortRatioItem
	if err := json.Unmarshal(data, &items); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse okx long/short ratio: %w", err)
	}

	var result []normalizers.NormalizedLongShortRatio
	for _, item := range items {
		snapshotTime := excommon.MustTime(excommon.MillisToTime(item[0]))
		longRatio := floatPtrSafe(item[1])
		shortRatio := floatPtrSafe(item[2])
		var lsRatio *float64
		if longRatio != nil && shortRatio != nil && *shortRatio != 0 {
			r := *longRatio / *shortRatio
			lsRatio = &r
		}
		ratio := normalizers.NormalizedLongShortRatio{
			SourceMeta: normalizers.SourceMeta{
				SymbolID:         symbol.ID,
				Exchange:         "okx",
				SourceSymbol:     job.SourceSymbol,
				SourceEndpointID: resp.SourceEndpointID,
				RawData:          excommon.RawMessage(item),
			},
			Period:         job.Period,
			SnapshotTime:   snapshotTime,
			LongRatio:      longRatio,
			ShortRatio:     shortRatio,
			LongShortRatio: lsRatio,
		}
		if err := normalizers.ValidateLongShortRatio(ratio); err != nil {
			continue
		}
		result = append(result, ratio)
	}

	return normalizers.NormalizedResult{LongShortRatios: result}, nil
}

func normalizeTakerFlow(data json.RawMessage, resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var items []takerVolumeItem
	if err := json.Unmarshal(data, &items); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse okx taker flow: %w", err)
	}

	var result []normalizers.NormalizedTakerFlow
	for _, item := range items {
		snapshotTime := excommon.MustTime(excommon.MillisToTime(item[0]))
		sellVol, _ := excommon.ParseFloat(item[1])
		buyVol, _ := excommon.ParseFloat(item[2])
		delta := buyVol - sellVol
		var ratio *float64
		if sellVol != 0 {
			r := buyVol / sellVol
			ratio = &r
		}
		flow := normalizers.NormalizedTakerFlow{
			SourceMeta: normalizers.SourceMeta{
				SymbolID:         symbol.ID,
				Exchange:         "okx",
				SourceSymbol:     job.SourceSymbol,
				SourceEndpointID: resp.SourceEndpointID,
				RawData:          excommon.RawMessage(item),
			},
			Period:          job.Period,
			SnapshotTime:    snapshotTime,
			TakerBuyVolume:  floatPtrSafe(item[2]),
			TakerSellVolume: floatPtrSafe(item[1]),
			BuySellDelta:    &delta,
			BuySellRatio:    ratio,
		}
		if err := normalizers.ValidateTakerFlow(flow); err != nil {
			continue
		}
		result = append(result, flow)
	}

	return normalizers.NormalizedResult{TakerFlows: result}, nil
}

func normalizeOrderbook(data json.RawMessage, resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var records []orderbookRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse okx orderbook: %w", err)
	}
	if len(records) == 0 {
		return normalizers.NormalizedResult{}, fmt.Errorf("okx orderbook data is empty")
	}

	ob := records[0]
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

	snapshotTime := resp.CapturedAt
	if ob.TS != "" {
		snapshotTime = excommon.MustTime(excommon.MillisToTime(ob.TS))
	}

	obi := normalizers.NormalizedOrderbookImbalance{
		SourceMeta: normalizers.SourceMeta{
			SymbolID:         symbol.ID,
			Exchange:         "okx",
			SourceSymbol:     job.SourceSymbol,
			SourceEndpointID: resp.SourceEndpointID,
			RawData:          resp.Body,
		},
		SnapshotTime:   snapshotTime,
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

func normalizeLiquidation(data json.RawMessage, resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var items []liquidationRecord
	if err := json.Unmarshal(data, &items); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse okx liquidation: %w", err)
	}

	var events []normalizers.NormalizedLiquidationEvent
	var aggregates []normalizers.NormalizedLiquidationAggregate
	bucketMap := make(map[string]*normalizers.NormalizedLiquidationAggregate)
	bucketTime := resp.CapturedAt.Truncate(5 * time.Minute)
	bucketKey := bucketTime.Format(time.RFC3339)

	for _, item := range items {
		eventTime := excommon.MustTime(excommon.MillisToTime(item.TS))
		price := floatPtrSafe(item.BkPx)
		qty := floatPtrSafe(item.Sz)
		var notional float64
		if price != nil && qty != nil {
			notional = *price * *qty
		}

		side := "BUY"
		if item.PosSide == "long" {
			side = "SELL"
		}

		event := normalizers.NormalizedLiquidationEvent{
			SourceMeta: normalizers.SourceMeta{
				SymbolID:         symbol.ID,
				Exchange:         "okx",
				SourceSymbol:     job.SourceSymbol,
				SourceEndpointID: resp.SourceEndpointID,
				RawData:          excommon.RawMessage(item),
			},
			EventKey:  fmt.Sprintf("%s-%s-%s", item.InstID, side, item.TS),
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
					Exchange:     "okx",
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
		if err := normalizers.ValidateLiquidationAggregate(*agg); err != nil {
			continue
		}
		aggregates = append(aggregates, *agg)
	}

	return normalizers.NormalizedResult{
		LiquidationEvents:     events,
		LiquidationAggregates: aggregates,
	}, nil
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
