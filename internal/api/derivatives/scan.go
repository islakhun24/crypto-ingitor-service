package derivatives

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

type floatScanner struct {
	target **float64
}

func floatDest(target **float64) *floatScanner {
	return &floatScanner{target: target}
}

func (s *floatScanner) Scan(value any) error {
	if value == nil {
		*s.target = nil
		return nil
	}
	var parsed float64
	switch typed := value.(type) {
	case float64:
		parsed = typed
	case int64:
		parsed = float64(typed)
	case []byte:
		value, err := strconv.ParseFloat(string(typed), 64)
		if err != nil {
			return err
		}
		parsed = value
	case string:
		value, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return err
		}
		parsed = value
	default:
		return fmt.Errorf("cannot scan %T into float", value)
	}
	*s.target = &parsed
	return nil
}

type int64Scanner struct {
	target **int64
}

func int64Dest(target **int64) *int64Scanner {
	return &int64Scanner{target: target}
}

func (s *int64Scanner) Scan(value any) error {
	if value == nil {
		*s.target = nil
		return nil
	}
	var parsed int64
	switch typed := value.(type) {
	case int64:
		parsed = typed
	case int:
		parsed = int64(typed)
	case []byte:
		value, err := strconv.ParseInt(string(typed), 10, 64)
		if err != nil {
			return err
		}
		parsed = value
	case string:
		value, err := strconv.ParseInt(typed, 10, 64)
		if err != nil {
			return err
		}
		parsed = value
	default:
		return fmt.Errorf("cannot scan %T into int64", value)
	}
	*s.target = &parsed
	return nil
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func ensureJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}

func ensureJSONArray(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`[]`)
	}
	return raw
}

func firstFloat(values ...*float64) *float64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func freshness(snapshotTime time.Time) FreshnessDTO {
	if snapshotTime.IsZero() {
		return FreshnessDTO{Status: "unknown"}
	}
	age := int64(time.Since(snapshotTime).Seconds())
	status := "fresh"
	switch {
	case age < 0:
		status = "future"
	case age > int64((15 * time.Minute).Seconds()):
		status = "stale"
	case age > int64((5 * time.Minute).Seconds()):
		status = "lagging"
	}
	return FreshnessDTO{
		SnapshotTime: snapshotTime,
		AgeSeconds:   age,
		Status:       status,
	}
}
