package realtime

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	KindMarket             = "market"
	KindOpenInterest       = "oi"
	KindFunding            = "funding"
	KindOrderbookImbalance = "orderbook_imbalance"
	KindAggregate          = "aggregate"
)

func LatestMarketKey(exchange, sourceSymbol string) string {
	return latestExchangeSymbolKey(KindMarket, exchange, sourceSymbol)
}

func LatestOpenInterestKey(exchange, sourceSymbol string) string {
	return latestExchangeSymbolKey(KindOpenInterest, exchange, sourceSymbol)
}

func LatestFundingKey(exchange, sourceSymbol string) string {
	return latestExchangeSymbolKey(KindFunding, exchange, sourceSymbol)
}

func LatestOrderbookImbalanceKey(exchange, sourceSymbol string) string {
	return latestExchangeSymbolKey(KindOrderbookImbalance, exchange, sourceSymbol)
}

func LatestAggregateKey(symbolID int64) string {
	return "deriv:latest:aggregate:" + strconv.FormatInt(symbolID, 10)
}

func WSStateKey(exchange, stream string) string {
	return fmt.Sprintf("deriv:ws:state:%s:%s", normalizeExchange(exchange), normalizeStream(stream))
}

func CollectorHealthKey(exchange, dataType string) string {
	return fmt.Sprintf("deriv:collector:health:%s:%s", normalizeExchange(exchange), normalizeStream(dataType))
}

func LatestKey(kind, exchange, sourceSymbol string, symbolID int64) (string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case KindMarket:
		return LatestMarketKey(exchange, sourceSymbol), nil
	case KindOpenInterest:
		return LatestOpenInterestKey(exchange, sourceSymbol), nil
	case KindFunding:
		return LatestFundingKey(exchange, sourceSymbol), nil
	case KindOrderbookImbalance:
		return LatestOrderbookImbalanceKey(exchange, sourceSymbol), nil
	case KindAggregate:
		if symbolID <= 0 {
			return "", fmt.Errorf("symbol_id is required for aggregate latest key")
		}
		return LatestAggregateKey(symbolID), nil
	default:
		return "", fmt.Errorf("unsupported latest kind %q", kind)
	}
}

func latestExchangeSymbolKey(kind, exchange, sourceSymbol string) string {
	return fmt.Sprintf("deriv:latest:%s:%s:%s", kind, normalizeExchange(exchange), normalizeSourceSymbol(sourceSymbol))
}

func normalizeExchange(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeSourceSymbol(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func normalizeStream(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "_")
	return value
}
