package repositories

import "time"

type AggregateOptions struct {
	MaxSnapshotAge time.Duration
	Windows        []AggregationWindow
}

type AggregationWindow struct {
	Label    string
	Duration time.Duration
}

var defaultAggregationWindows = []AggregationWindow{
	{Label: "5m", Duration: 5 * time.Minute},
	{Label: "15m", Duration: 15 * time.Minute},
	{Label: "1h", Duration: time.Hour},
	{Label: "4h", Duration: 4 * time.Hour},
	{Label: "24h", Duration: 24 * time.Hour},
}

func normalizeAggregateOptions(options AggregateOptions) AggregateOptions {
	if options.MaxSnapshotAge <= 0 {
		options.MaxSnapshotAge = 10 * time.Minute
	}
	if len(options.Windows) == 0 {
		options.Windows = defaultAggregationWindows
	}

	return options
}
