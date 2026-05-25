package endpoints

import (
	"encoding/json"
	"time"
)

type Endpoint struct {
	ID                 int64           `json:"id"`
	Exchange           string          `json:"exchange"`
	MarketType         string          `json:"market_type"`
	DataType           string          `json:"data_type"`
	Name               string          `json:"name"`
	Method             string          `json:"method"`
	BaseURL            string          `json:"base_url"`
	Path               string          `json:"path"`
	ParamsTemplate     json.RawMessage `json:"params_template"`
	HeadersTemplate    json.RawMessage `json:"headers_template"`
	ResponseFormat     string          `json:"response_format"`
	IsBatchSupported   bool            `json:"is_batch_supported"`
	BatchParamName     string          `json:"batch_param_name,omitempty"`
	MaxBatchSize       int             `json:"max_batch_size"`
	RateLimitPerSecond float64         `json:"rate_limit_per_second"`
	RateLimitPerMinute int             `json:"rate_limit_per_minute"`
	RequestWeight      int             `json:"request_weight"`
	MinIntervalSeconds float64         `json:"min_interval_seconds"`
	TimeoutMS          int             `json:"timeout_ms"`
	IsActive           bool            `json:"is_active"`
	Notes              string          `json:"notes,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}
