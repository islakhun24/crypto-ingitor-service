package binance

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

type klineItem = [12]any

type openInterestResponse struct {
	Symbol       string `json:"symbol"`
	OpenInterest string `json:"openInterest"`
	Time         int64  `json:"time"`
}

type longShortRatioItem struct {
	Symbol         string `json:"symbol"`
	LongShortRatio string `json:"longShortRatio"`
	LongAccount    string `json:"longAccount"`
	ShortAccount   string `json:"shortAccount"`
	Timestamp      int64  `json:"timestamp"`
}

type takerFlowItem struct {
	Symbol       string `json:"symbol"`
	BuySellRatio string `json:"buySellRatio"`
	BuyVol       string `json:"buyVol"`
	SellVol      string `json:"sellVol"`
	Timestamp    int64  `json:"timestamp"`
}

type basisItem struct {
	Pair         string `json:"pair"`
	ContractType string `json:"contractType"`
	FuturesPrice string `json:"futuresPrice"`
	IndexPrice   string `json:"indexPrice"`
	Basis        string `json:"basis"`
	BasisRate    string `json:"basisRate"`
	Timestamp    int64  `json:"timestamp"`
}

type orderbookResponse struct {
	LastUpdateId int64      `json:"lastUpdateId"`
	Bids         [][]string `json:"bids"`
	Asks         [][]string `json:"asks"`
}

type liquidationItem struct {
	Symbol       string `json:"symbol"`
	Price        string `json:"price"`
	OrigQty      string `json:"origQty"`
	ExecutedQty  string `json:"executedQty"`
	AveragePrice string `json:"averagePrice"`
	Status       string `json:"status"`
	Time         int64  `json:"time"`
	Side         string `json:"side"`
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
	case "taker_flow":
		return normalizeTakerFlow(resp, job, symbol)
	case "basis":
		return normalizeBasis(resp, job, symbol)
	case "orderbook", "orderbook_imbalance":
		return normalizeOrderbook(resp, job, symbol)
	case "liquidation":
		return normalizeLiquidation(resp, job, symbol)
	default:
		return normalizers.NormalizedResult{}, excommon.ErrUnsupportedDataType
	}
}

func normalizeTicker(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
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

func normalizeKline(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var klines []klineItem
	if err := json.Unmarshal(resp.Body, &klines); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse binance klines: %w", err)
	}

	var result []normalizers.NormalizedKline
	for _, k := range klines {
		openTime, _ := excommon.MillisToTime(k[0])
		closeTime, _ := excommon.MillisToTime(k[6])
		trades, _ := excommon.ParseFloat(k[8])
		isClosed := true
		if len(k) > 11 {
			if confirm, ok := k[11].(bool); ok {
				isClosed = confirm
			}
		}
		kline := normalizers.NormalizedKline{
			SourceMeta: normalizers.SourceMeta{
				SymbolID:         symbol.ID,
				Exchange:         "binance",
				SourceSymbol:     job.SourceSymbol,
				SourceEndpointID: resp.SourceEndpointID,
				RawData:          excommon.RawMessage(k),
			},
			Interval:            job.Period,
			OpenTime:            openTime,
			CloseTime:           closeTime,
			OpenPrice:           parseFloatSafe(k[1]),
			HighPrice:           parseFloatSafe(k[2]),
			LowPrice:            parseFloatSafe(k[3]),
			ClosePrice:          parseFloatSafe(k[4]),
			Volume:              floatPtrSafe(k[5]),
			QuoteVolume:         floatPtrSafe(k[7]),
			TradeCount:          intPtr(trades),
			TakerBuyVolume:      floatPtrSafe(k[9]),
			TakerBuyQuoteVolume: floatPtrSafe(k[10]),
			IsClosed:            isClosed,
		}
		if err := normalizers.ValidateKline(kline); err != nil {
			continue
		}
		result = append(result, kline)
	}

	return normalizers.NormalizedResult{Klines: result}, nil
}

func normalizeOpenInterest(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var oi openInterestResponse
	if err := json.Unmarshal(resp.Body, &oi); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse binance open interest: %w", err)
	}

	snapshotTime := resp.CapturedAt
	if oi.Time > 0 {
		snapshotTime = excommon.MustTime(excommon.MillisToTime(oi.Time))
	}

	oiValue := parseFloatSafe(oi.OpenInterest)
	oiItem := normalizers.NormalizedOpenInterest{
		SourceMeta: normalizers.SourceMeta{
			SymbolID:         symbol.ID,
			Exchange:         "binance",
			SourceSymbol:     job.SourceSymbol,
			SourceEndpointID: resp.SourceEndpointID,
			RawData:          resp.Body,
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
	var items []longShortRatioItem
	if err := json.Unmarshal(resp.Body, &items); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse binance long/short ratio: %w", err)
	}

	var result []normalizers.NormalizedLongShortRatio
	for _, item := range items {
		snapshotTime := excommon.MustTime(excommon.MillisToTime(item.Timestamp))
		ratio := normalizers.NormalizedLongShortRatio{
			SourceMeta: normalizers.SourceMeta{
				SymbolID:         symbol.ID,
				Exchange:         "binance",
				SourceSymbol:     job.SourceSymbol,
				SourceEndpointID: resp.SourceEndpointID,
				RawData:          excommon.RawMessage(item),
			},
			Period:            job.Period,
			SnapshotTime:      snapshotTime,
			LongAccountRatio:  floatPtrSafe(item.LongAccount),
			ShortAccountRatio: floatPtrSafe(item.ShortAccount),
			LongShortRatio:    floatPtrSafe(item.LongShortRatio),
		}
		if err := normalizers.ValidateLongShortRatio(ratio); err != nil {
			continue
		}
		result = append(result, ratio)
	}

	return normalizers.NormalizedResult{LongShortRatios: result}, nil
}

func normalizeTakerFlow(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var items []takerFlowItem
	if err := json.Unmarshal(resp.Body, &items); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse binance taker flow: %w", err)
	}

	var result []normalizers.NormalizedTakerFlow
	for _, item := range items {
		snapshotTime := excommon.MustTime(excommon.MillisToTime(item.Timestamp))
		buyVol, _ := excommon.ParseFloat(item.BuyVol)
		sellVol, _ := excommon.ParseFloat(item.SellVol)
		delta := buyVol - sellVol
		var ratio *float64
		if sellVol != 0 {
			r := buyVol / sellVol
			ratio = &r
		}
		flow := normalizers.NormalizedTakerFlow{
			SourceMeta: normalizers.SourceMeta{
				SymbolID:         symbol.ID,
				Exchange:         "binance",
				SourceSymbol:     job.SourceSymbol,
				SourceEndpointID: resp.SourceEndpointID,
				RawData:          excommon.RawMessage(item),
			},
			Period:          job.Period,
			SnapshotTime:    snapshotTime,
			TakerBuyVolume:  floatPtrSafe(item.BuyVol),
			TakerSellVolume: floatPtrSafe(item.SellVol),
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

func normalizeBasis(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var items []basisItem
	if err := json.Unmarshal(resp.Body, &items); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse binance basis: %w", err)
	}

	var result []normalizers.NormalizedBasisPremium
	for _, item := range items {
		snapshotTime := excommon.MustTime(excommon.MillisToTime(item.Timestamp))
		futuresPrice := floatPtrSafe(item.FuturesPrice)
		indexPrice := floatPtrSafe(item.IndexPrice)
		basis := floatPtrSafe(item.Basis)
		basisRate := floatPtrSafe(item.BasisRate)
		var annualized *float64
		if basisRate != nil {
			a := *basisRate * 365 * 100
			annualized = &a
		}
		basisPrem := normalizers.NormalizedBasisPremium{
			SourceMeta: normalizers.SourceMeta{
				SymbolID:         symbol.ID,
				Exchange:         "binance",
				SourceSymbol:     job.SourceSymbol,
				SourceEndpointID: resp.SourceEndpointID,
				RawData:          excommon.RawMessage(item),
			},
			SnapshotTime:           snapshotTime,
			FuturesPrice:           futuresPrice,
			IndexPrice:             indexPrice,
			Basis:                  basis,
			BasisPercent:           basisRate,
			AnnualizedBasisPercent: annualized,
		}
		if err := normalizers.ValidateBasisPremium(basisPrem); err != nil {
			continue
		}
		result = append(result, basisPrem)
	}

	return normalizers.NormalizedResult{BasisPremiums: result}, nil
}

func normalizeOrderbook(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var ob orderbookResponse
	if err := json.Unmarshal(resp.Body, &ob); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse binance orderbook: %w", err)
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
			Exchange:         "binance",
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

func normalizeLiquidation(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var items []liquidationItem
	if err := json.Unmarshal(resp.Body, &items); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse binance liquidation: %w", err)
	}

	var events []normalizers.NormalizedLiquidationEvent
	var aggregates []normalizers.NormalizedLiquidationAggregate
	bucketMap := make(map[string]*normalizers.NormalizedLiquidationAggregate)
	bucketTime := resp.CapturedAt.Truncate(5 * time.Minute)
	bucketKey := bucketTime.Format(time.RFC3339)

	for _, item := range items {
		eventTime := excommon.MustTime(excommon.MillisToTime(item.Time))
		price := floatPtrSafe(item.Price)
		qty := floatPtrSafe(item.OrigQty)
		var notional float64
		if price != nil && qty != nil {
			notional = *price * *qty
		}

		event := normalizers.NormalizedLiquidationEvent{
			SourceMeta: normalizers.SourceMeta{
				SymbolID:         symbol.ID,
				Exchange:         "binance",
				SourceSymbol:     job.SourceSymbol,
				SourceEndpointID: resp.SourceEndpointID,
				RawData:          excommon.RawMessage(item),
			},
			EventKey:  fmt.Sprintf("%s-%s-%d", item.Symbol, item.Side, item.Time),
			EventTime: eventTime,
			Side:      item.Side,
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
					Exchange:     "binance",
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
		if item.Side == "SELL" || item.Side == "LONG" {
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
