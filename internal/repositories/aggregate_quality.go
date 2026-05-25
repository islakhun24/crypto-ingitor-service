package repositories

import (
	"encoding/json"
	"strings"
	"time"
)

type aggregateQualityMetadata struct {
	MaxSnapshotAgeSeconds int64               `json:"max_snapshot_age_seconds"`
	SnapshotTime          time.Time           `json:"snapshot_time"`
	StaleCutoff           time.Time           `json:"stale_cutoff"`
	IncludedExchanges     []string            `json:"included_exchanges"`
	SkippedExchanges      map[string][]string `json:"skipped_exchanges,omitempty"`
	InvalidFields         map[string][]string `json:"invalid_fields,omitempty"`
}

func newAggregateQualityMetadata(snapshotTime time.Time, staleCutoff time.Time, maxAge time.Duration) *aggregateQualityMetadata {
	return &aggregateQualityMetadata{
		MaxSnapshotAgeSeconds: int64(maxAge.Seconds()),
		SnapshotTime:          snapshotTime.UTC(),
		StaleCutoff:           staleCutoff.UTC(),
		SkippedExchanges:      map[string][]string{},
		InvalidFields:         map[string][]string{},
	}
}

func (q *aggregateQualityMetadata) Include(exchange string) {
	q.IncludedExchanges = append(q.IncludedExchanges, exchange)
}

func (q *aggregateQualityMetadata) Skip(exchange string, reason string) {
	q.SkippedExchanges[exchange] = append(q.SkippedExchanges[exchange], reason)
}

func (q *aggregateQualityMetadata) Invalid(exchange string, field string) {
	q.InvalidFields[exchange] = append(q.InvalidFields[exchange], field)
}

func (q *aggregateQualityMetadata) Marshal() json.RawMessage {
	if len(q.SkippedExchanges) == 0 {
		q.SkippedExchanges = nil
	}
	if len(q.InvalidFields) == 0 {
		q.InvalidFields = nil
	}
	raw, _ := json.Marshal(q)
	return raw
}

func rawJSONMarkedDegraded(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return false
	}

	return anyMarkedDegraded(value)
}

func anyMarkedDegraded(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			normalizedKey := strings.ToLower(strings.TrimSpace(key))
			switch normalizedKey {
			case "quality_status", "data_quality", "normalized_status", "status", "health":
				if degradedStatus(item) {
					return true
				}
			case "degraded", "failed":
				if boolValue(item) {
					return true
				}
			}
			if anyMarkedDegraded(item) {
				return true
			}
		}
	case []any:
		for _, item := range typed {
			if anyMarkedDegraded(item) {
				return true
			}
		}
	}

	return false
}

func degradedStatus(value any) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(text)) {
	case "degraded", "failed", "invalid", "unhealthy":
		return true
	default:
		return false
	}
}

func boolValue(value any) bool {
	typed, ok := value.(bool)
	return ok && typed
}
