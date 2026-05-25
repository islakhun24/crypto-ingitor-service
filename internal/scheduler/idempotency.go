package scheduler

import (
	"fmt"
	"strings"
	"time"
)

func IdempotencyKey(exchange string, dataType string, symbolID int64, sourceSymbol string, period string, scheduledBucket time.Time) string {
	return fmt.Sprintf(
		"%s:%s:%d:%s:%s:%s",
		strings.ToLower(strings.TrimSpace(exchange)),
		strings.TrimSpace(dataType),
		symbolID,
		strings.TrimSpace(sourceSymbol),
		strings.TrimSpace(period),
		scheduledBucket.UTC().Format(time.RFC3339),
	)
}

func ScheduledBucket(at time.Time, intervalSeconds int) time.Time {
	if intervalSeconds <= 0 {
		return at.UTC()
	}

	interval := time.Duration(intervalSeconds) * time.Second
	return at.UTC().Truncate(interval)
}
