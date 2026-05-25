package database

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestMigrationsCreateRequiredPhase2Tables(t *testing.T) {
	sql := readMigrations(t)

	requiredTables := []string{
		"exchange_api_endpoints",
		"derivative_collection_policies",
		"symbol_collection_tiers",
		"derivative_collection_jobs",
		"derivative_market_snapshots",
		"derivative_klines",
		"open_interest_snapshots",
		"open_interest_history",
		"funding_rate_snapshots",
		"funding_rate_history",
		"long_short_ratio_snapshots",
		"taker_flow_snapshots",
		"cvd_snapshots",
		"liquidation_events",
		"liquidation_aggregates",
		"basis_premium_snapshots",
		"orderbook_imbalance_snapshots",
		"orderbook_depth_snapshots",
		"exchange_divergence_snapshots",
		"derivative_aggregated_snapshots",
		"market_structure_snapshots",
		"volatility_snapshots",
		"collector_health",
		"data_collection_runs",
		"exchange_request_logs",
		"failed_collection_jobs",
		"data_quality_issues",
		"data_gaps",
		"raw_exchange_payloads",
		"data_retention_policies",
		"data_cleanup_runs",
		"data_rollup_runs",
	}

	for _, table := range requiredTables {
		pattern := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+IF\s+NOT\s+EXISTS\s+` + table + `\s*\(`)
		if !pattern.MatchString(sql) {
			t.Fatalf("missing idempotent CREATE TABLE for %s", table)
		}
	}
}

func TestMigrationsDoNotCreateSymbolsTable(t *testing.T) {
	sql := readMigrations(t)
	pattern := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(IF\s+NOT\s+EXISTS\s+)?symbols\s*\(`)
	if pattern.MatchString(sql) {
		t.Fatal("migrations must not recreate symbols table")
	}
}

func TestMigrationsHaveRequiredUniqueIndexes(t *testing.T) {
	sql := readMigrations(t)

	requiredIndexes := []string{
		"ux_exchange_api_endpoints_identity",
		"ux_derivative_collection_jobs_idempotency_key",
		"ux_derivative_market_snapshots_symbol_exchange_time",
		"ux_derivative_klines_symbol_exchange_interval_open_time",
		"ux_open_interest_snapshots_symbol_exchange_time",
		"ux_open_interest_history_symbol_exchange_period_timestamp",
		"ux_funding_rate_snapshots_symbol_exchange_time",
		"ux_funding_rate_history_symbol_exchange_funding_time",
		"ux_long_short_ratio_snapshots_symbol_exchange_period_time",
		"ux_taker_flow_snapshots_symbol_exchange_period_time",
		"ux_cvd_snapshots_symbol_exchange_period_time",
		"ux_liquidation_events_event_key",
		"ux_liquidation_aggregates_symbol_exchange_period_bucket",
		"ux_basis_premium_snapshots_symbol_exchange_time",
		"ux_orderbook_imbalance_snapshots_symbol_exchange_time_depth",
		"ux_orderbook_depth_snapshots_symbol_exchange_time_depth",
		"ux_exchange_divergence_snapshots_identity",
		"ux_derivative_aggregated_snapshots_symbol_time",
		"ux_market_structure_snapshots_symbol_exchange_period_time",
		"ux_volatility_snapshots_symbol_exchange_period_time",
		"ux_data_collection_runs_run_key",
		"ux_failed_collection_jobs_idempotency_key",
		"ux_data_quality_issues_issue_key",
		"ux_data_gaps_gap_key",
		"ux_raw_exchange_payloads_identity",
		"ux_data_retention_policies_identity",
		"ux_data_cleanup_runs_run_key",
		"ux_data_rollup_runs_run_key",
	}

	for _, index := range requiredIndexes {
		pattern := regexp.MustCompile(`(?i)CREATE\s+UNIQUE\s+INDEX\s+IF\s+NOT\s+EXISTS\s+` + index + `\b`)
		if !pattern.MatchString(sql) {
			t.Fatalf("missing required unique index %s", index)
		}
	}
}

func TestEndpointSeedIncludesSupportedExchanges(t *testing.T) {
	sql := readMigrations(t)

	supported := []string{"binance", "okx", "bybit", "bitget", "gate", "mexc"}
	for _, exchange := range supported {
		if !strings.Contains(sql, "('"+exchange+"',") {
			t.Fatalf("endpoint seed missing exchange %s", exchange)
		}
	}
}

func TestEndpointSeedUsesSourceSymbolPlaceholder(t *testing.T) {
	sql := readMigrations(t)

	if strings.Contains(sql, "exchange_symbol") {
		t.Fatal("endpoint seed must not use exchange_symbol placeholder")
	}
	if !strings.Contains(sql, "{{source_symbol}}") {
		t.Fatal("endpoint seed must use source_symbol placeholder")
	}
}

func TestEndpointSeedIsIdempotent(t *testing.T) {
	sql := readMigrations(t)

	if !strings.Contains(sql, "ON CONFLICT (exchange, market_type, data_type, name) DO UPDATE") {
		t.Fatal("endpoint seed must upsert on endpoint identity")
	}
}

func TestEndpointSeedHasSafeRateLimitDefaults(t *testing.T) {
	sql := readMigrations(t)

	defaults := map[string]string{
		"binance": "5, 300",
		"okx":     "2, 120",
		"bybit":   "3, 180",
		"bitget":  "4, 240",
		"gate":    "2, 120",
		"mexc":    "2, 120",
	}

	for exchange, rateFragment := range defaults {
		if !strings.Contains(sql, "('"+exchange+"',") || !strings.Contains(sql, rateFragment) {
			t.Fatalf("endpoint seed missing safe rate limit default for %s", exchange)
		}
	}
}

func TestEndpointSeedKeepsUncertainEndpointsInactive(t *testing.T) {
	sql := readMigrations(t)

	requiredInactiveNotes := []string{
		"Seeded inactive until liquidation semantics",
		"Inactive: index endpoint uses index instrument id",
		"Inactive: public liquidation REST endpoint must be verified",
		"Inactive: liquidation endpoint is not part of the Phase 3 verified MEXC list",
	}

	for _, note := range requiredInactiveNotes {
		if !strings.Contains(sql, note) {
			t.Fatalf("missing inactive uncertain endpoint note: %s", note)
		}
	}
}

func TestCollectionPolicySeedIncludesDefaultTiers(t *testing.T) {
	sql := readMigrations(t)

	requiredFragments := []string{
		"('all', 'ticker', NULL, 300",
		"('all', 'kline', '5m', 300",
		"('top100', 'ticker', NULL, 60",
		"('top100', 'liquidation_aggregate', '1m', 60",
		"('watchlist', 'ticker', NULL, 30",
		"('watchlist', 'orderbook_imbalance', NULL, 30",
		"ON CONFLICT DO NOTHING",
	}

	for _, fragment := range requiredFragments {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("collection policy seed missing fragment: %s", fragment)
		}
	}
}

func TestAggregateMigrationIncludesPhase6Columns(t *testing.T) {
	sql := readMigrations(t)

	requiredFragments := []string{
		"ADD COLUMN IF NOT EXISTS price_avg",
		"ADD COLUMN IF NOT EXISTS price_weighted",
		"ADD COLUMN IF NOT EXISTS total_quote_volume_24h",
		"ADD COLUMN IF NOT EXISTS total_open_interest_value",
		"ADD COLUMN IF NOT EXISTS min_funding_rate",
		"ADD COLUMN IF NOT EXISTS max_funding_rate",
		"ADD COLUMN IF NOT EXISTS available_exchanges",
		"ADD COLUMN IF NOT EXISTS raw_by_exchange",
	}

	for _, fragment := range requiredFragments {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("aggregate migration missing fragment: %s", fragment)
		}
	}
}

func TestPhase7MigrationIncludesAdvancedColumns(t *testing.T) {
	sql := readMigrations(t)

	requiredFragments := []string{
		"ADD COLUMN IF NOT EXISTS taker_buy_volume",
		"ADD COLUMN IF NOT EXISTS buy_sell_delta_quote",
		"ADD COLUMN IF NOT EXISTS cvd_change_percent",
		"ADD COLUMN IF NOT EXISTS usd_value",
		"ADD COLUMN IF NOT EXISTS largest_liquidation_usd",
		"ADD COLUMN IF NOT EXISTS annualized_basis_percent",
		"ADD COLUMN IF NOT EXISTS bid_depth_1pct_usd",
		"ADD COLUMN IF NOT EXISTS price_spread_percent",
		"ADD COLUMN IF NOT EXISTS total_cvd",
		"ADD COLUMN IF NOT EXISTS total_liquidation_usd",
	}

	for _, fragment := range requiredFragments {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("phase 7 migration missing fragment: %s", fragment)
		}
	}
}

func TestPhase8MigrationAddsAnalyticsLayerWithoutScoring(t *testing.T) {
	sql := readMigrations(t)

	requiredFragments := []string{
		"ADD COLUMN IF NOT EXISTS window_metrics",
		"ADD COLUMN IF NOT EXISTS quality_metadata",
		"ADD COLUMN IF NOT EXISTS anomaly_flags",
		"ADD COLUMN IF NOT EXISTS trend_direction",
		"ADD COLUMN IF NOT EXISTS structure_state",
		"ADD COLUMN IF NOT EXISTS support_levels",
		"ADD COLUMN IF NOT EXISTS resistance_levels",
		"ADD COLUMN IF NOT EXISTS atr_percent",
		"ADD COLUMN IF NOT EXISTS realized_volatility_percent",
		"ADD COLUMN IF NOT EXISTS range_percent_24h",
		"ix_derivative_klines_symbol_interval_open_time",
	}

	for _, fragment := range requiredFragments {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("phase 8 migration missing fragment: %s", fragment)
		}
	}
	if strings.Contains(strings.ToLower(sql), "signal_scores") {
		t.Fatal("phase 8 must not create signal_scores")
	}
}

func TestPhase9MigrationSeedsRetentionPolicies(t *testing.T) {
	sql := readMigrations(t)

	requiredFragments := []string{
		"ADD COLUMN IF NOT EXISTS priority",
		"ADD COLUMN IF NOT EXISTS max_rows_per_run",
		"ADD COLUMN IF NOT EXISTS timeout_seconds",
		"ADD COLUMN IF NOT EXISTS partition_strategy",
		"('derivative_klines', 'open_time', 'interval', '1m', 14",
		`"rollup_target_interval":"5m"`,
		"('derivative_klines', 'open_time', 'interval', '4h', 1825",
		`"rollup_target_interval":"1d"`,
		"('liquidation_events', 'event_time', NULL, NULL, 30",
		"('orderbook_depth_snapshots', 'snapshot_time', NULL, NULL, 7",
		"('raw_exchange_payloads', 'captured_at', NULL, NULL, 7",
		"ON CONFLICT",
		"dry_run = EXCLUDED.dry_run",
		"ix_derivative_klines_interval_open_time_retention",
	}

	for _, fragment := range requiredFragments {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("phase 9 migration missing fragment: %s", fragment)
		}
	}
}

func TestPhase10MigrationAddsHardeningColumns(t *testing.T) {
	sql := readMigrations(t)

	requiredFragments := []string{
		"ADD COLUMN IF NOT EXISTS job_mode",
		"ADD COLUMN IF NOT EXISTS parent_gap_id",
		"ADD COLUMN IF NOT EXISTS backfill_checkpoint",
		"ADD COLUMN IF NOT EXISTS last_error_type",
		"ADD COLUMN IF NOT EXISTS next_retry_at",
		"ADD COLUMN IF NOT EXISTS endpoint_id",
		"ADD COLUMN IF NOT EXISTS safe_payload_sample",
		"ADD COLUMN IF NOT EXISTS expected_interval_seconds",
		"ADD COLUMN IF NOT EXISTS backfill_job_id",
		"ix_derivative_collection_jobs_mode_status_scheduled",
		"ix_derivative_collection_jobs_running_stale",
		"ix_data_quality_issues_job_observed",
	}

	for _, fragment := range requiredFragments {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("phase 10 migration missing fragment: %s", fragment)
		}
	}
}

func TestPhase11MigrationAddsAPIIndexes(t *testing.T) {
	sql := readMigrations(t)

	requiredFragments := []string{
		"ix_derivative_aggregated_snapshots_api_latest",
		"ix_derivative_klines_api_symbol_interval_time",
		"ix_open_interest_history_api_symbol_period_time",
		"ix_funding_rate_history_api_symbol_time",
		"ix_long_short_ratio_api_symbol_period_time",
		"ix_taker_flow_api_symbol_period_time",
		"ix_cvd_api_symbol_period_time",
		"ix_liquidation_aggregates_api_symbol_period_time",
		"ix_collector_health_api_exchange_heartbeat",
		"ix_data_quality_issues_api_status_seen",
		"ix_data_gaps_api_status_start",
	}

	for _, fragment := range requiredFragments {
		if !strings.Contains(sql, fragment) {
			t.Fatalf("phase 11 migration missing fragment: %s", fragment)
		}
	}
}

func TestMigrationsUseIdempotentCreateTableAndIndex(t *testing.T) {
	sql := readMigrations(t)
	lines := strings.Split(sql, "\n")

	for _, line := range lines {
		normalized := strings.ToUpper(strings.TrimSpace(line))
		if strings.HasPrefix(normalized, "CREATE TABLE") && !strings.Contains(normalized, "IF NOT EXISTS") {
			t.Fatalf("non-idempotent table DDL: %s", line)
		}
		if strings.HasPrefix(normalized, "CREATE INDEX") && !strings.Contains(normalized, "IF NOT EXISTS") {
			t.Fatalf("non-idempotent index DDL: %s", line)
		}
		if strings.HasPrefix(normalized, "CREATE UNIQUE INDEX") && !strings.Contains(normalized, "IF NOT EXISTS") {
			t.Fatalf("non-idempotent unique index DDL: %s", line)
		}
	}
}

func readMigrations(t *testing.T) string {
	t.Helper()

	root := filepath.Join("..", "..", "migrations")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}

	var combined strings.Builder
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}

		content, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatalf("read migration %s: %v", entry.Name(), err)
		}
		combined.Write(content)
		combined.WriteByte('\n')
	}

	return combined.String()
}
