package logger

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestLoggerWritesStructuredJSON(t *testing.T) {
	var buffer bytes.Buffer
	log := NewWithWriter("collector-service", &buffer)

	log.Info("job completed", Fields{
		"exchange":      "binance",
		"data_type":     "ticker",
		"symbol_id":     int64(42),
		"source_symbol": "BTCUSDT",
		"job_id":        int64(99),
		"endpoint_id":   int64(7),
		"duration_ms":   123,
	})

	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buffer.Bytes()), &record); err != nil {
		t.Fatalf("log is not JSON: %v", err)
	}
	if record["service"] != "collector-service" || record["exchange"] != "binance" || record["data_type"] != "ticker" {
		t.Fatalf("record = %+v", record)
	}
	if record["message"] != "job completed" || record["level"] != "info" {
		t.Fatalf("record = %+v", record)
	}
}
