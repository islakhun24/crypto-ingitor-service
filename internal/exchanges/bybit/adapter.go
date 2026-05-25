package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
		ExchangeName: "bybit",
		Client:       client,
		Normalizer:   Normalize,
	}}
}

type baseEnvelope struct {
	RetCode int             `json:"retCode"`
	RetMsg  string          `json:"retMsg"`
	Result  json.RawMessage `json:"result"`
	Time    int64           `json:"time"`
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

type tickerResult struct {
	List []tickerRecord `json:"list"`
}

type klineResult struct {
	List [][]string `json:"list"`
}

type oiRecord struct {
	OpenInterest string `json:"openInterest"`
	Timestamp    string `json:"timestamp"`
}

type oiResult struct {
	List []oiRecord `json:"list"`
}

type lsRatioRecord struct {
	Symbol    string `json:"symbol"`
	BuyRatio  string `json:"buyRatio"`
	SellRatio string `json:"sellRatio"`
	Timestamp string `json:"timestamp"`
}

type lsRatioResult struct {
	List []lsRatioRecord `json:"list"`
}

type obResult struct {
	Asks [][]string `json:"a"`
	Bids [][]string `json:"b"`
	Ts   string     `json:"ts"`
}

type liquidationRecord struct {
	Symbol    string `json:"symbol"`
	Side      string `json:"side"`
	Price     string `json:"price"`
	Size      string `json:"size"`
	Timestamp string `json:"timestamp"`
}

type liquidationResult struct {
	List []liquidationRecord `json:"list"`
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
	case "liquidation":
		return normalizeLiquidation(resp, job, symbol)
	default:
		return normalizers.NormalizedResult{}, excommon.ErrUnsupportedDataType
	}
}

func parseBaseEnvelope(body []byte) (baseEnvelope, error) {
	var env baseEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return baseEnvelope{}, fmt.Errorf("parse bybit response: %w", err)
	}
	if env.RetCode != 0 {
		return baseEnvelope{}, fmt.Errorf("%w: bybit code=%d msg=%s", excommon.ErrExchangeResponse, env.RetCode, env.RetMsg)
	}
	return env, nil
}

func normalizeTicker(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	env, err := parseBaseEnvelope(resp.Body)
	if err != nil {
		return normalizers.NormalizedResult{}, err
	}

	var result tickerResult
	if err := json.Unmarshal(env.Result, &result); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse bybit ticker: %w", err)
	}
	if len(result.List) == 0 {
		return normalizers.NormalizedResult{}, fmt.Errorf("bybit ticker list is empty")
	}

	item := result.List[0]
	snapshotTime, err := excommon.MillisToTime(env.Time)
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

func normalizeKline(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	env, err := parseBaseEnvelope(resp.Body)
	if err != nil {
		return normalizers.NormalizedResult{}, err
	}

	var result klineResult
	if err := json.Unmarshal(env.Result, &result); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse bybit klines: %w", err)
	}

	var items []normalizers.NormalizedKline
	for _, k := range result.List {
		if len(k) < 7 {
			continue
		}
		openTime, _ := excommon.MillisToTime(k[0])
		kline := normalizers.NormalizedKline{
			SourceMeta: normalizers.SourceMeta{
				SymbolID:         symbol.ID,
				Exchange:         "bybit",
				SourceSymbol:     job.SourceSymbol,
				SourceEndpointID: resp.SourceEndpointID,
				RawData:          excommon.RawMessage(k),
			},
			Interval:   job.Period,
			OpenTime:   openTime,
			CloseTime:  openTime,
			OpenPrice:  parseFloatSafe(k[1]),
			HighPrice:  parseFloatSafe(k[2]),
			LowPrice:   parseFloatSafe(k[3]),
			ClosePrice: parseFloatSafe(k[4]),
			Volume:     floatPtrSafe(k[5]),
			QuoteVolume: floatPtrSafe(k[6]),
			IsClosed:   true,
		}
		if err := normalizers.ValidateKline(kline); err != nil {
			continue
		}
		items = append(items, kline)
	}

	return normalizers.NormalizedResult{Klines: items}, nil
}

func normalizeOpenInterest(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	env, err := parseBaseEnvelope(resp.Body)
	if err != nil {
		return normalizers.NormalizedResult{}, err
	}

	var result oiResult
	if err := json.Unmarshal(env.Result, &result); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse bybit open interest: %w", err)
	}
	if len(result.List) == 0 {
		return normalizers.NormalizedResult{OpenInterest: []normalizers.NormalizedOpenInterest{}}, nil
	}

	item := result.List[0]
	snapshotTime := resp.CapturedAt
	if item.Timestamp != "" {
		snapshotTime = excommon.MustTime(excommon.MillisToTime(item.Timestamp))
	}

	oiItem := normalizers.NormalizedOpenInterest{
		SourceMeta: normalizers.SourceMeta{
			SymbolID:         symbol.ID,
			Exchange:         "bybit",
			SourceSymbol:     job.SourceSymbol,
			SourceEndpointID: resp.SourceEndpointID,
			RawData:          excommon.RawMessage(item),
		},
		SnapshotTime: snapshotTime,
		OpenInterest: parseFloatSafe(item.OpenInterest),
	}
	if err := normalizers.ValidateOpenInterest(oiItem, false); err != nil {
		return normalizers.NormalizedResult{}, err
	}

	return normalizers.NormalizedResult{OpenInterest: []normalizers.NormalizedOpenInterest{oiItem}}, nil
}

func normalizeLongShortRatio(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	env, err := parseBaseEnvelope(resp.Body)
	if err != nil {
		return normalizers.NormalizedResult{}, err
	}

	var result lsRatioResult
	if err := json.Unmarshal(env.Result, &result); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse bybit long/short ratio: %w", err)
	}

	var items []normalizers.NormalizedLongShortRatio
	for _, item := range result.List {
		snapshotTime := excommon.MustTime(excommon.MillisToTime(item.Timestamp))
		buyRatio := floatPtrSafe(item.BuyRatio)
		sellRatio := floatPtrSafe(item.SellRatio)
		var lsRatio *float64
		if sellRatio != nil && *sellRatio != 0 {
			r := *buyRatio / *sellRatio
			lsRatio = &r
		}

		ratio := normalizers.NormalizedLongShortRatio{
			SourceMeta: normalizers.SourceMeta{
				SymbolID:         symbol.ID,
				Exchange:         "bybit",
				SourceSymbol:     job.SourceSymbol,
				SourceEndpointID: resp.SourceEndpointID,
				RawData:          excommon.RawMessage(item),
			},
			Period:            job.Period,
			SnapshotTime:      snapshotTime,
			LongAccountRatio:  buyRatio,
			ShortAccountRatio: sellRatio,
			LongShortRatio:    lsRatio,
		}
		if err := normalizers.ValidateLongShortRatio(ratio); err != nil {
			continue
		}
		items = append(items, ratio)
	}

	return normalizers.NormalizedResult{LongShortRatios: items}, nil
}

func normalizeOrderbook(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	env, err := parseBaseEnvelope(resp.Body)
	if err != nil {
		return normalizers.NormalizedResult{}, err
	}

	var ob obResult
	if err := json.Unmarshal(env.Result, &ob); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse bybit orderbook: %w", err)
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
			Exchange:         "bybit",
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
	env, err := parseBaseEnvelope(resp.Body)
	if err != nil {
		return normalizers.NormalizedResult{}, err
	}

	var result liquidationResult
	if err := json.Unmarshal(env.Result, &result); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse bybit liquidation: %w", err)
	}

	var events []normalizers.NormalizedLiquidationEvent
	var aggregates []normalizers.NormalizedLiquidationAggregate
	bucketMap := make(map[string]*normalizers.NormalizedLiquidationAggregate)
	bucketTime := resp.CapturedAt.Truncate(5 * time.Minute)
	bucketKey := bucketTime.Format(time.RFC3339)

	for _, item := range result.List {
		eventTime := excommon.MustTime(excommon.MillisToTime(item.Timestamp))
		price := floatPtrSafe(item.Price)
		size := floatPtrSafe(item.Size)
		var notional float64
		if price != nil && size != nil {
			notional = *price * *size
		}
		sideUpper := strings.ToUpper(item.Side)

		event := normalizers.NormalizedLiquidationEvent{
			SourceMeta: normalizers.SourceMeta{
				SymbolID:         symbol.ID,
				Exchange:         "bybit",
				SourceSymbol:     job.SourceSymbol,
				SourceEndpointID: resp.SourceEndpointID,
				RawData:          excommon.RawMessage(item),
			},
			EventKey:  fmt.Sprintf("%s-%s-%s", item.Symbol, sideUpper, item.Timestamp),
			EventTime: eventTime,
			Side:      sideUpper,
			Price:     price,
			Quantity:  size,
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
					Exchange:     "bybit",
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
		if sideUpper == "SELL" || sideUpper == "LONG" {
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
