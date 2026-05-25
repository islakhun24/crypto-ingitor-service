package derivatives

import (
	"strings"
	"testing"
)

func TestOverviewSQLUsesLatestAggregateAndPagination(t *testing.T) {
	opts := ListOptions{
		Exchange:  "binance",
		Search:    "btc",
		Limit:     50,
		Page:      2,
		Sort:      "volume",
		Direction: "desc",
	}

	query, args := overviewSQL(opts, false)
	required := []string{
		"WITH latest AS",
		"DISTINCT ON (symbol_id)",
		"FROM derivative_aggregated_snapshots",
		"LEFT JOIN latest ON latest.symbol_id = s.id",
		"jsonb_array_elements(COALESCE(s.markets, '[]'::jsonb))",
		"ORDER BY latest.total_volume_24h DESC",
		"LIMIT",
		"OFFSET",
	}
	for _, fragment := range required {
		if !strings.Contains(query, fragment) {
			t.Fatalf("overview SQL missing %q:\n%s", fragment, query)
		}
	}
	if len(args) == 0 {
		t.Fatal("overview SQL args are empty")
	}
}

func TestOverviewSortWhitelist(t *testing.T) {
	if got := overviewSort("not_a_column"); got != "s.cmc_rank" {
		t.Fatalf("overviewSort unknown = %q", got)
	}
	if got := overviewSort("oi"); got != "latest.total_open_interest" {
		t.Fatalf("overviewSort oi = %q", got)
	}
}
