package all

import (
	"fmt"
	"net/http"
	"strings"

	"aggregator-services/internal/exchanges/binance"
	"aggregator-services/internal/exchanges/bitget"
	"aggregator-services/internal/exchanges/bybit"
	excommon "aggregator-services/internal/exchanges/common"
	"aggregator-services/internal/exchanges/gate"
	"aggregator-services/internal/exchanges/mexc"
	"aggregator-services/internal/exchanges/okx"
)

type Registry struct {
	adapters map[string]excommon.ExchangeAdapter
}

func NewRegistry(client *http.Client) Registry {
	adapters := map[string]excommon.ExchangeAdapter{
		"binance": binance.NewAdapter(client),
		"okx":     okx.NewAdapter(client),
		"bybit":   bybit.NewAdapter(client),
		"bitget":  bitget.NewAdapter(client),
		"gate":    gate.NewAdapter(client),
		"mexc":    mexc.NewAdapter(client),
	}

	return Registry{adapters: adapters}
}

func (r Registry) Get(exchange string) (excommon.ExchangeAdapter, error) {
	exchange = strings.ToLower(strings.TrimSpace(exchange))
	adapter, ok := r.adapters[exchange]
	if !ok {
		return nil, fmt.Errorf("exchange adapter %q is not registered", exchange)
	}

	return adapter, nil
}

func (r Registry) Exchanges() []string {
	exchanges := make([]string, 0, len(r.adapters))
	for exchange := range r.adapters {
		exchanges = append(exchanges, exchange)
	}

	return exchanges
}
