package common

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func RawMessage(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}

	return raw
}

func FloatPtr(value any) (*float64, error) {
	if value == nil {
		return nil, nil
	}
	if typed, ok := value.(string); ok && strings.TrimSpace(typed) == "" {
		return nil, nil
	}

	parsed, err := ParseFloat(value)
	if err != nil {
		return nil, err
	}

	return &parsed, nil
}

func OptionalFloat(value string) (*float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, err
	}

	return &parsed, nil
}

func ParseFloat(value any) (float64, error) {
	switch typed := value.(type) {
	case string:
		typed = strings.TrimSpace(typed)
		if typed == "" {
			return 0, fmt.Errorf("empty numeric string")
		}
		return strconv.ParseFloat(typed, 64)
	case float64:
		return typed, nil
	case int:
		return float64(typed), nil
	case int64:
		return float64(typed), nil
	case json.Number:
		return typed.Float64()
	default:
		return 0, fmt.Errorf("unsupported numeric type %T", value)
	}
}

func MillisToTime(value any) (time.Time, error) {
	switch typed := value.(type) {
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return time.Time{}, err
		}
		return time.UnixMilli(parsed).UTC(), nil
	case float64:
		return time.UnixMilli(int64(typed)).UTC(), nil
	case int64:
		return time.UnixMilli(typed).UTC(), nil
	case int:
		return time.UnixMilli(int64(typed)).UTC(), nil
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return time.Time{}, err
		}
		return time.UnixMilli(parsed).UTC(), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported millis type %T", value)
	}
}

func MustTime(value time.Time, err error) time.Time {
	if err != nil {
		return time.Time{}
	}

	return value
}

func SecondsToTime(value any) (time.Time, error) {
	switch typed := value.(type) {
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return time.Time{}, err
		}
		return time.Unix(parsed, 0).UTC(), nil
	case float64:
		return time.Unix(int64(typed), 0).UTC(), nil
	case int64:
		return time.Unix(typed, 0).UTC(), nil
	case int:
		return time.Unix(int64(typed), 0).UTC(), nil
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return time.Time{}, err
		}
		return time.Unix(parsed, 0).UTC(), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported seconds type %T", value)
	}
}
