package gate

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

type orderbookResponse struct {
	ID   int64      `json:"id"`
	Asks [][]string `json:"asks"`
	Bids [][]string `json:"bids"`
}

type liquidationItem struct {
	Contract string  `json:"contract"`
	Size     float64 `json:"size"`
	Price    string  `json:"price"`
	Left     float64 `json:"left"`
	Time     int64   `json:"time"`
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
	case "orderbook", "orderbook_imbalance":
		return normalizeOrderbook(resp, job, symbol)
	case "liquidation":
		return normalizeLiquidation(resp, job, symbol)
	case "basis":
		return normalizeBasis(resp, job, symbol)
	default:
		return normalizers.NormalizedResult{}, excommon.ErrUnsupportedDataType
	}
}

func normalizeTicker(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
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

func normalizeKline(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var rows [][]any
	if err := json.Unmarshal(resp.Body, &rows); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse gate klines: %w", err)
	}

	var result []normalizers.NormalizedKline
	for _, row := range rows {
		if len(row) < 6 {
			continue
		}
		openTime := excommon.MustTime(excommon.SecondsToTime(row[0]))
		closeTime := openTime

		kline := normalizers.NormalizedKline{
			SourceMeta: normalizers.SourceMeta{
				SymbolID:         symbol.ID,
				Exchange:         "gate",
				SourceSymbol:     job.SourceSymbol,
				SourceEndpointID: resp.SourceEndpointID,
				RawData:          excommon.RawMessage(row),
			},
			Interval:   job.Period,
			OpenTime:   openTime,
			CloseTime:  closeTime,
			OpenPrice:  parseFloatSafe(row[5]),
			HighPrice:  parseFloatSafe(row[3]),
			LowPrice:   parseFloatSafe(row[4]),
			ClosePrice: parseFloatSafe(row[2]),
			Volume:     floatPtrSafe(row[1]),
			IsClosed:   true,
		}
		if err := normalizers.ValidateKline(kline); err != nil {
			continue
		}
		result = append(result, kline)
	}

	return normalizers.NormalizedResult{Klines: result}, nil
}

func normalizeOpenInterest(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var rows [][]any
	if err := json.Unmarshal(resp.Body, &rows); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse gate open interest: %w", err)
	}

	var result []normalizers.NormalizedOpenInterest
	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		snapshotTime := excommon.MustTime(excommon.SecondsToTime(row[0]))
		oiValue := parseFloatSafe(row[1])

		oi := normalizers.NormalizedOpenInterest{
			SourceMeta: normalizers.SourceMeta{
				SymbolID:         symbol.ID,
				Exchange:         "gate",
				SourceSymbol:     job.SourceSymbol,
				SourceEndpointID: resp.SourceEndpointID,
				RawData:          excommon.RawMessage(row),
			},
			Period:       job.Period,
			SnapshotTime: snapshotTime,
			OpenInterest: oiValue,
		}
		if err := normalizers.ValidateOpenInterest(oi, true); err != nil {
			continue
		}
		result = append(result, oi)
	}

	return normalizers.NormalizedResult{OpenInterest: result}, nil
}

func normalizeLongShortRatio(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var rows [][]any
	if err := json.Unmarshal(resp.Body, &rows); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse gate long/short ratio: %w", err)
	}

	var result []normalizers.NormalizedLongShortRatio
	for _, row := range rows {
		if len(row) < 5 {
			continue
		}
		snapshotTime := excommon.MustTime(excommon.SecondsToTime(row[0]))
		longRatio := floatPtrSafe(row[3])
		shortRatio := floatPtrSafe(row[4])

		var lsRatio *float64
		if longRatio != nil && shortRatio != nil && *shortRatio != 0 {
			r := *longRatio / *shortRatio
			lsRatio = &r
		}

		ratio := normalizers.NormalizedLongShortRatio{
			SourceMeta: normalizers.SourceMeta{
				SymbolID:         symbol.ID,
				Exchange:         "gate",
				SourceSymbol:     job.SourceSymbol,
				SourceEndpointID: resp.SourceEndpointID,
				RawData:          excommon.RawMessage(row),
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

func normalizeTakerFlow(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var rows [][]any
	if err := json.Unmarshal(resp.Body, &rows); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse gate taker flow: %w", err)
	}

	var result []normalizers.NormalizedTakerFlow
	for _, row := range rows {
		if len(row) < 7 {
			continue
		}
		snapshotTime := excommon.MustTime(excommon.SecondsToTime(row[0]))
		buyVol := parseFloatSafe(row[5])
		sellVol := parseFloatSafe(row[6])
		delta := buyVol - sellVol
		var ratio *float64
		if sellVol != 0 {
			r := buyVol / sellVol
			ratio = &r
		}

		flow := normalizers.NormalizedTakerFlow{
			SourceMeta: normalizers.SourceMeta{
				SymbolID:         symbol.ID,
				Exchange:         "gate",
				SourceSymbol:     job.SourceSymbol,
				SourceEndpointID: resp.SourceEndpointID,
				RawData:          excommon.RawMessage(row),
			},
			Period:          job.Period,
			SnapshotTime:    snapshotTime,
			TakerBuyVolume:  floatPtrSafe(row[5]),
			TakerSellVolume: floatPtrSafe(row[6]),
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

func normalizeOrderbook(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var ob orderbookResponse
	if err := json.Unmarshal(resp.Body, &ob); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse gate orderbook: %w", err)
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
			Exchange:         "gate",
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
		return normalizers.NormalizedResult{}, fmt.Errorf("parse gate liquidation: %w", err)
	}

	var events []normalizers.NormalizedLiquidationEvent
	var aggregates []normalizers.NormalizedLiquidationAggregate
	bucketMap := make(map[string]*normalizers.NormalizedLiquidationAggregate)
	bucketTime := resp.CapturedAt.Truncate(5 * time.Minute)
	bucketKey := bucketTime.Format(time.RFC3339)

	for _, item := range items {
		eventTime := time.Unix(item.Time, 0).UTC()
		price := floatPtrSafe(item.Price)

		qtyVal := item.Size
		if qtyVal < 0 {
			qtyVal = -qtyVal
		}
		qty := &qtyVal

		var notional float64
		if price != nil && qty != nil {
			notional = *price * *qty
		}

		side := "BUY"
		if item.Size > 0 {
			side = "SELL"
		}

		event := normalizers.NormalizedLiquidationEvent{
			SourceMeta: normalizers.SourceMeta{
				SymbolID:         symbol.ID,
				Exchange:         "gate",
				SourceSymbol:     job.SourceSymbol,
				SourceEndpointID: resp.SourceEndpointID,
				RawData:          excommon.RawMessage(item),
			},
			EventKey:  fmt.Sprintf("%s-%s-%d", item.Contract, side, item.Time),
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
					Exchange:     "gate",
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

func normalizeBasis(resp *excommon.ExchangeResponse, job scheduler.Job, symbol symbols.Symbol) (normalizers.NormalizedResult, error) {
	var rows [][]any
	if err := json.Unmarshal(resp.Body, &rows); err != nil {
		return normalizers.NormalizedResult{}, fmt.Errorf("parse gate basis: %w", err)
	}

	var result []normalizers.NormalizedBasisPremium
	for _, row := range rows {
		if len(row) < 3 {
			continue
		}
		snapshotTime := excommon.MustTime(excommon.SecondsToTime(row[0]))
		basis := floatPtrSafe(row[2])

		basisPrem := normalizers.NormalizedBasisPremium{
			SourceMeta: normalizers.SourceMeta{
				SymbolID:         symbol.ID,
				Exchange:         "gate",
				SourceSymbol:     job.SourceSymbol,
				SourceEndpointID: resp.SourceEndpointID,
				RawData:          excommon.RawMessage(row),
			},
			SnapshotTime: snapshotTime,
			Basis:        basis,
		}
		if err := normalizers.ValidateBasisPremium(basisPrem); err != nil {
			continue
		}
		result = append(result, basisPrem)
	}

	return normalizers.NormalizedResult{BasisPremiums: result}, nil
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
