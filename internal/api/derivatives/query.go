package derivatives

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultLimit = 50
	maxLimit     = 500
)

type ListOptions struct {
	Exchange  string
	Search    string
	Category  string
	Interval  string
	Period    string
	StartTime *time.Time
	EndTime   *time.Time
	Limit     int
	Page      int
	Sort      string
	Direction string
	MinVolume *float64
	MinOI     *float64
	RankMin   *int
	RankMax   *int
}

func ParseListOptions(r *http.Request) (ListOptions, error) {
	query := r.URL.Query()
	opts := ListOptions{
		Exchange:  cleanLower(query.Get("exchange")),
		Search:    strings.TrimSpace(query.Get("search")),
		Category:  strings.TrimSpace(query.Get("category")),
		Interval:  strings.TrimSpace(firstNonEmpty(query.Get("interval"), query.Get("timeframe"))),
		Period:    strings.TrimSpace(query.Get("period")),
		Limit:     defaultLimit,
		Page:      1,
		Sort:      cleanLower(query.Get("sort")),
		Direction: cleanLower(query.Get("direction")),
	}
	if opts.Period == "" {
		opts.Period = opts.Interval
	}
	if opts.Direction == "" {
		opts.Direction = "desc"
	}
	if opts.Direction != "asc" && opts.Direction != "desc" {
		return ListOptions{}, fmt.Errorf("direction must be asc or desc")
	}

	var err error
	if raw := strings.TrimSpace(query.Get("limit")); raw != "" {
		opts.Limit, err = strconv.Atoi(raw)
		if err != nil || opts.Limit < 1 {
			return ListOptions{}, fmt.Errorf("limit must be a positive integer")
		}
		if opts.Limit > maxLimit {
			opts.Limit = maxLimit
		}
	}
	if raw := strings.TrimSpace(query.Get("page")); raw != "" {
		opts.Page, err = strconv.Atoi(raw)
		if err != nil || opts.Page < 1 {
			return ListOptions{}, fmt.Errorf("page must be a positive integer")
		}
	}
	if opts.StartTime, err = parseOptionalTime(query.Get("start_time")); err != nil {
		return ListOptions{}, fmt.Errorf("start_time must be RFC3339 or unix seconds")
	}
	if opts.EndTime, err = parseOptionalTime(query.Get("end_time")); err != nil {
		return ListOptions{}, fmt.Errorf("end_time must be RFC3339 or unix seconds")
	}
	if opts.StartTime != nil && opts.EndTime != nil && opts.StartTime.After(*opts.EndTime) {
		return ListOptions{}, fmt.Errorf("start_time must be before end_time")
	}
	if opts.MinVolume, err = parseOptionalFloat(query.Get("min_volume")); err != nil {
		return ListOptions{}, fmt.Errorf("min_volume must be a number")
	}
	if opts.MinOI, err = parseOptionalFloat(query.Get("min_oi")); err != nil {
		return ListOptions{}, fmt.Errorf("min_oi must be a number")
	}
	if opts.RankMin, err = parseOptionalInt(query.Get("rank_min")); err != nil {
		return ListOptions{}, fmt.Errorf("rank_min must be an integer")
	}
	if opts.RankMax, err = parseOptionalInt(query.Get("rank_max")); err != nil {
		return ListOptions{}, fmt.Errorf("rank_max must be an integer")
	}

	return opts, nil
}

func (o ListOptions) Offset() int {
	return (o.Page - 1) * o.Limit
}

func (o ListOptions) PageMeta(total int) PageMeta {
	totalPages := 0
	if o.Limit > 0 {
		totalPages = (total + o.Limit - 1) / o.Limit
	}
	return PageMeta{
		Page:       o.Page,
		Limit:      o.Limit,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    o.Page < totalPages,
	}
}

func parseOptionalTime(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		parsed = parsed.UTC()
		return &parsed, nil
	}
	seconds, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, err
	}
	parsed := time.Unix(seconds, 0).UTC()
	return &parsed, nil
}

func parseOptionalFloat(raw string) (*float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func parseOptionalInt(raw string) (*int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func cleanLower(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
