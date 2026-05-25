package symbols

import (
	"encoding/json"
	"fmt"
	"strings"
)

func ParseMarketMappings(raw []byte) ([]MarketMapping, error) {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		return nil, nil
	}

	var mappings []MarketMapping
	if err := json.Unmarshal([]byte(value), &mappings); err != nil {
		return nil, fmt.Errorf("parse symbols.markets: %w", err)
	}

	normalized := make([]MarketMapping, 0, len(mappings))
	for _, mapping := range mappings {
		sourceSymbol := strings.TrimSpace(mapping.SourceSymbol)
		if sourceSymbol == "" {
			continue
		}

		normalized = append(normalized, MarketMapping{
			Exchange:     strings.ToLower(strings.TrimSpace(mapping.Exchange)),
			MarketType:   strings.TrimSpace(mapping.MarketType),
			SourceSymbol: sourceSymbol,
			Status:       NormalizeMarketStatus(mapping.Status),
		})
	}

	return normalized, nil
}

func SupportedExchangeSet(exchanges []string) map[string]struct{} {
	set := make(map[string]struct{}, len(exchanges))
	for _, exchange := range exchanges {
		normalized := strings.ToLower(strings.TrimSpace(exchange))
		if normalized != "" {
			set[normalized] = struct{}{}
		}
	}

	return set
}

func ActiveSymbolMarkets(symbol Symbol, supported map[string]struct{}) []SymbolMarket {
	active := make([]SymbolMarket, 0, len(symbol.Markets))

	for _, market := range symbol.Markets {
		exchange := market.NormalizedExchange()
		if _, ok := supported[exchange]; !ok {
			continue
		}
		if !market.IsActive() || strings.TrimSpace(market.SourceSymbol) == "" {
			continue
		}

		active = append(active, SymbolMarket{
			SymbolID:        symbol.ID,
			CanonicalSymbol: symbol.Symbol,
			Exchange:        exchange,
			MarketType:      strings.TrimSpace(market.MarketType),
			SourceSymbol:    strings.TrimSpace(market.SourceSymbol),
			Status:          strings.TrimSpace(market.Status),
		})
	}

	return active
}
