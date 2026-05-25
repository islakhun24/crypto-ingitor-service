package symbols

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrSymbolNotFound = errors.New("symbol not found")

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListActiveSymbols(ctx context.Context) ([]Symbol, error) {
	return r.querySymbols(ctx, `
		SELECT id, symbol, base_asset, quote_asset, market_type, cmc_rank, is_active, markets
		FROM symbols
		WHERE is_active = true
		ORDER BY cmc_rank ASC NULLS LAST, symbol ASC
	`)
}

func (r *Repository) ListSymbolsByExchange(ctx context.Context, exchange string) ([]Symbol, error) {
	exchange = strings.ToLower(strings.TrimSpace(exchange))
	if exchange == "" {
		return nil, fmt.Errorf("exchange is required")
	}

	active, err := r.ListActiveSymbols(ctx)
	if err != nil {
		return nil, err
	}

	filtered := make([]Symbol, 0, len(active))
	for _, symbol := range active {
		markets := make([]MarketMapping, 0, len(symbol.Markets))
		for _, market := range symbol.Markets {
			if market.NormalizedExchange() == exchange && market.IsActive() {
				markets = append(markets, market)
			}
		}
		if len(markets) > 0 {
			symbol.Markets = markets
			filtered = append(filtered, symbol)
		}
	}

	return filtered, nil
}

func (r *Repository) ListTopSymbols(ctx context.Context, limit int) ([]Symbol, error) {
	if limit < 1 {
		return nil, fmt.Errorf("limit must be greater than 0")
	}

	return r.querySymbols(ctx, `
		SELECT id, symbol, base_asset, quote_asset, market_type, cmc_rank, is_active, markets
		FROM symbols
		WHERE is_active = true
		ORDER BY cmc_rank ASC NULLS LAST, symbol ASC
		LIMIT $1
	`, limit)
}

func (r *Repository) ListWatchlistSymbols(ctx context.Context) ([]Symbol, error) {
	exists, err := r.columnExists(ctx, "symbols", "is_watchlist")
	if err != nil {
		return nil, err
	}
	if !exists {
		return []Symbol{}, nil
	}

	return r.querySymbols(ctx, `
		SELECT id, symbol, base_asset, quote_asset, market_type, cmc_rank, is_active, markets
		FROM symbols
		WHERE is_active = true
		  AND is_watchlist = true
		ORDER BY cmc_rank ASC NULLS LAST, symbol ASC
	`)
}

func (r *Repository) GetSymbolByID(ctx context.Context, id int64) (Symbol, error) {
	symbols, err := r.querySymbols(ctx, `
		SELECT id, symbol, base_asset, quote_asset, market_type, cmc_rank, is_active, markets
		FROM symbols
		WHERE id = $1
		LIMIT 1
	`, id)
	if err != nil {
		return Symbol{}, err
	}
	if len(symbols) == 0 {
		return Symbol{}, ErrSymbolNotFound
	}

	return symbols[0], nil
}

func (r *Repository) ListActiveSymbolMarkets(ctx context.Context, supportedExchanges []string) ([]SymbolMarket, error) {
	symbols, err := r.ListActiveSymbols(ctx)
	if err != nil {
		return nil, err
	}

	supported := SupportedExchangeSet(supportedExchanges)
	active := make([]SymbolMarket, 0, len(symbols))
	for _, symbol := range symbols {
		active = append(active, ActiveSymbolMarkets(symbol, supported)...)
	}

	return active, nil
}

func (r *Repository) querySymbols(ctx context.Context, query string, args ...any) ([]Symbol, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query symbols: %w", err)
	}
	defer rows.Close()

	var symbols []Symbol
	for rows.Next() {
		symbol, err := scanSymbol(rows)
		if err != nil {
			return nil, err
		}
		symbols = append(symbols, symbol)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate symbols: %w", err)
	}

	return symbols, nil
}

func scanSymbol(rows *sql.Rows) (Symbol, error) {
	var (
		symbol     Symbol
		baseAsset  sql.NullString
		quoteAsset sql.NullString
		marketType sql.NullString
		cmcRank    sql.NullInt64
		marketsRaw []byte
	)

	if err := rows.Scan(
		&symbol.ID,
		&symbol.Symbol,
		&baseAsset,
		&quoteAsset,
		&marketType,
		&cmcRank,
		&symbol.IsActive,
		&marketsRaw,
	); err != nil {
		return Symbol{}, fmt.Errorf("scan symbol: %w", err)
	}

	markets, err := ParseMarketMappings(marketsRaw)
	if err != nil {
		return Symbol{}, err
	}

	symbol.BaseAsset = baseAsset.String
	symbol.QuoteAsset = quoteAsset.String
	symbol.MarketType = marketType.String
	symbol.CmcRank = int(cmcRank.Int64)
	symbol.Markets = markets

	return symbol, nil
}

func (r *Repository) columnExists(ctx context.Context, tableName string, columnName string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = $1
			  AND column_name = $2
		)
	`, tableName, columnName).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check column %s.%s: %w", tableName, columnName, err)
	}

	return exists, nil
}
