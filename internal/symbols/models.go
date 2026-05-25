package symbols

import "strings"

type Symbol struct {
	ID         int64           `json:"id"`
	Symbol     string          `json:"symbol"`
	BaseAsset  string          `json:"base_asset"`
	QuoteAsset string          `json:"quote_asset"`
	MarketType string          `json:"market_type"`
	CmcRank    int             `json:"cmc_rank"`
	IsActive   bool            `json:"is_active"`
	Markets    []MarketMapping `json:"markets"`
}

type MarketMapping struct {
	Exchange     string `json:"exchange"`
	MarketType   string `json:"market_type"`
	SourceSymbol string `json:"source_symbol"`
	Status       string `json:"status"`
}

type SymbolMarket struct {
	SymbolID        int64  `json:"symbol_id"`
	CanonicalSymbol string `json:"canonical_symbol"`
	Exchange        string `json:"exchange"`
	MarketType      string `json:"market_type"`
	SourceSymbol    string `json:"source_symbol"`
	Status          string `json:"status"`
}

func (m MarketMapping) IsActive() bool {
	switch strings.ToLower(strings.TrimSpace(m.Status)) {
	case "active", "live", "normal", "trading":
		return true
	default:
		return false
	}
}

func (m MarketMapping) NormalizedExchange() string {
	return strings.ToLower(strings.TrimSpace(m.Exchange))
}

func NormalizeMarketStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}
