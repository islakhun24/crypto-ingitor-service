package symbols

import "testing"

func TestParseMarketMappingsUsesSourceSymbol(t *testing.T) {
	raw := []byte(`[
		{"exchange":"binance","market_type":"usds-m-futures","source_symbol":"0GUSDT","status":"TRADING"},
		{"exchange":"okx","market_type":"swap","source_symbol":"0G-USDT-SWAP","status":"live"},
		{"exchange":"bybit","market_type":"linear","source_symbol":"0GUSDT","status":"Trading"},
		{"exchange":"bitget","market_type":"usdt-futures","source_symbol":"0GUSDT","status":"normal"},
		{"exchange":"gate","market_type":"usdt-futures","source_symbol":"0G_USDT","status":"active"},
		{"exchange":"mexc","market_type":"usdt-futures","source_symbol":"0G_USDT","status":"active"},
		{"exchange":"mexc","market_type":"usdt-futures","exchange_symbol":"WRONG","status":"active"}
	]`)

	mappings, err := ParseMarketMappings(raw)
	if err != nil {
		t.Fatalf("ParseMarketMappings() error = %v", err)
	}

	symbol := Symbol{ID: 1, Symbol: "0GUSDT", Markets: mappings}
	active := ActiveSymbolMarkets(symbol, SupportedExchangeSet([]string{"binance", "okx", "bybit", "bitget", "gate", "mexc"}))

	if len(active) != 6 {
		t.Fatalf("len(active) = %d, want 6", len(active))
	}
	want := map[string]string{
		"binance": "0GUSDT",
		"okx":     "0G-USDT-SWAP",
		"bybit":   "0GUSDT",
		"bitget":  "0GUSDT",
		"gate":    "0G_USDT",
		"mexc":    "0G_USDT",
	}
	for _, market := range active {
		if market.SourceSymbol != want[market.Exchange] {
			t.Fatalf("%s source symbol = %q", market.Exchange, market.SourceSymbol)
		}
	}
}

func TestActiveSymbolMarketsFiltersInactiveAndUnsupported(t *testing.T) {
	symbol := Symbol{
		ID:     2,
		Symbol: "BTCUSDT",
		Markets: []MarketMapping{
			{Exchange: "binance", MarketType: "usds-m-futures", SourceSymbol: "BTCUSDT", Status: "TRADING"},
			{Exchange: "bybit", MarketType: "linear", SourceSymbol: "BTCUSDT", Status: "delisted"},
			{Exchange: "gate", MarketType: "usdt-futures", SourceSymbol: "", Status: "active"},
			{Exchange: "kucoin", MarketType: "swap", SourceSymbol: "BTCUSDTM", Status: "active"},
		},
	}

	active := ActiveSymbolMarkets(symbol, SupportedExchangeSet([]string{"binance", "bybit", "gate"}))

	if len(active) != 1 {
		t.Fatalf("len(active) = %d, want 1", len(active))
	}
	if active[0].Exchange != "binance" {
		t.Fatalf("active[0].Exchange = %q", active[0].Exchange)
	}
}
