package derivatives

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseListOptionsSupportsPaginationFiltersAndAliases(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/derivatives/overview?exchange=BINANCE&timeframe=5m&limit=999&page=2&direction=asc&min_volume=100&rank_min=1&start_time=2026-05-25T12:00:00Z", nil)

	opts, err := ParseListOptions(req)
	if err != nil {
		t.Fatalf("ParseListOptions() error = %v", err)
	}
	if opts.Exchange != "binance" {
		t.Fatalf("Exchange = %q", opts.Exchange)
	}
	if opts.Interval != "5m" || opts.Period != "5m" {
		t.Fatalf("interval/period = %q/%q", opts.Interval, opts.Period)
	}
	if opts.Limit != maxLimit || opts.Page != 2 {
		t.Fatalf("limit/page = %d/%d", opts.Limit, opts.Page)
	}
	if opts.Offset() != maxLimit {
		t.Fatalf("Offset = %d", opts.Offset())
	}
	if opts.MinVolume == nil || *opts.MinVolume != 100 {
		t.Fatalf("MinVolume = %v", opts.MinVolume)
	}
	if opts.RankMin == nil || *opts.RankMin != 1 {
		t.Fatalf("RankMin = %v", opts.RankMin)
	}
	if opts.StartTime == nil || !opts.StartTime.Equal(time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("StartTime = %v", opts.StartTime)
	}
}

func TestParseListOptionsRejectsBadDirection(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/derivatives/overview?direction=sideways", nil)

	if _, err := ParseListOptions(req); err == nil {
		t.Fatal("ParseListOptions() error = nil, want direction error")
	}
}

func TestPageMeta(t *testing.T) {
	meta := (ListOptions{Page: 2, Limit: 25}).PageMeta(60)
	if meta.TotalPages != 3 || !meta.HasNext {
		t.Fatalf("PageMeta = %+v", meta)
	}
}
