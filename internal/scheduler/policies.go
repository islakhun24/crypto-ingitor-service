package scheduler

import "strings"

func SymbolLimitForTier(tier string) int {
	switch strings.ToLower(strings.TrimSpace(tier)) {
	case TierTop100:
		return 100
	default:
		return 0
	}
}

func EndpointDataType(dataType string) (string, bool) {
	switch strings.TrimSpace(dataType) {
	case "orderbook_imbalance":
		return "orderbook", true
	case "liquidation_aggregate":
		return "liquidation", true
	case "aggregated_snapshot":
		return "", false
	default:
		return dataType, true
	}
}
