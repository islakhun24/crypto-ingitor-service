WITH exchange_market(exchange, market_type) AS (
    VALUES
        ('binance', 'usds-m-futures'),
        ('okx', 'swap'),
        ('bybit', 'linear'),
        ('bitget', 'usdt-futures'),
        ('gate', 'usdt-futures'),
        ('mexc', 'usdt-futures')
),
policy_seed(tier, data_type, period, interval_seconds, priority, notes) AS (
    VALUES
        -- All active symbols
        ('all', 'ticker', NULL, 300, 100, 'All symbols ticker every 5m.'),
        ('all', 'mark_price', NULL, 300, 100, 'All symbols mark price every 5m.'),
        ('all', 'open_interest', NULL, 300, 100, 'All symbols open interest every 5m.'),
        ('all', 'funding', NULL, 900, 110, 'All symbols current funding every 15m.'),
        ('all', 'funding_history', NULL, 3600, 120, 'All symbols funding history every 1h.'),
        ('all', 'kline', '5m', 300, 100, 'All symbols 5m klines.'),
        ('all', 'kline', '15m', 900, 105, 'All symbols 15m klines.'),
        ('all', 'kline', '1h', 3600, 110, 'All symbols 1h klines.'),
        ('all', 'kline', '4h', 14400, 120, 'All symbols 4h klines.'),
        ('all', 'kline', '1d', 86400, 130, 'All symbols 1d klines.'),
        ('all', 'long_short_ratio', '15m', 900, 120, 'All symbols long/short ratio every 15m.'),
        ('all', 'long_short_ratio', '1h', 3600, 130, 'All symbols long/short ratio every 1h.'),
        ('all', 'taker_flow', '5m', 300, 115, 'All symbols taker flow every 5m.'),
        ('all', 'basis', '15m', 900, 130, 'All symbols basis every 15m where supported.'),
        ('all', 'orderbook_imbalance', NULL, 300, 125, 'All symbols orderbook imbalance every 5m where supported.'),
        ('all', 'aggregated_snapshot', NULL, 300, 100, 'All symbols aggregated snapshot every 5m.'),

        -- Top 100 symbols
        ('top100', 'ticker', NULL, 60, 50, 'Top 100 ticker every 1m.'),
        ('top100', 'mark_price', NULL, 60, 50, 'Top 100 mark price every 1m.'),
        ('top100', 'open_interest', NULL, 60, 50, 'Top 100 open interest every 1m.'),
        ('top100', 'kline', '1m', 60, 50, 'Top 100 1m klines.'),
        ('top100', 'kline', '5m', 300, 60, 'Top 100 5m klines.'),
        ('top100', 'kline', '15m', 900, 70, 'Top 100 15m klines.'),
        ('top100', 'taker_flow', '1m', 60, 70, 'Top 100 taker flow every 1m.'),
        ('top100', 'taker_flow', '5m', 300, 80, 'Top 100 taker flow every 5m.'),
        ('top100', 'liquidation_aggregate', '1m', 60, 60, 'Top 100 liquidation aggregate every 1m where supported.'),
        ('top100', 'orderbook_imbalance', NULL, 60, 60, 'Top 100 orderbook imbalance every 1m.'),
        ('top100', 'aggregated_snapshot', NULL, 60, 50, 'Top 100 aggregated snapshot every 1m.'),

        -- Watchlist symbols
        ('watchlist', 'ticker', NULL, 30, 10, 'Watchlist ticker every 30s.'),
        ('watchlist', 'open_interest', NULL, 60, 10, 'Watchlist open interest every 1m.'),
        ('watchlist', 'funding', NULL, 300, 20, 'Watchlist funding every 5m.'),
        ('watchlist', 'kline', '1m', 60, 10, 'Watchlist 1m klines.'),
        ('watchlist', 'kline', '5m', 300, 20, 'Watchlist 5m klines.'),
        ('watchlist', 'kline', '15m', 900, 30, 'Watchlist 15m klines.'),
        ('watchlist', 'orderbook_imbalance', NULL, 30, 10, 'Watchlist orderbook imbalance every 30s.')
)
INSERT INTO derivative_collection_policies (
    exchange,
    market_type,
    data_type,
    tier,
    period,
    interval_seconds,
    batch_size,
    priority,
    enabled,
    max_retry,
    stale_after_seconds,
    metadata
)
SELECT
    em.exchange,
    em.market_type,
    ps.data_type,
    ps.tier,
    ps.period,
    ps.interval_seconds,
    1,
    ps.priority,
    true,
    3,
    ps.interval_seconds * 3,
    jsonb_build_object('notes', ps.notes, 'seed_phase', 'phase4')
FROM exchange_market em
CROSS JOIN policy_seed ps
ON CONFLICT DO NOTHING;
