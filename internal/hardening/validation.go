package hardening

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aggregator-services/internal/normalizers"
	"aggregator-services/internal/scheduler"
)

type ValidationConfig struct {
	MaxFutureSkew time.Duration
	FundingMin    float64
	FundingMax    float64
}

func DefaultValidationConfig() ValidationConfig {
	return ValidationConfig{
		MaxFutureSkew: 2 * time.Minute,
		FundingMin:    -0.05,
		FundingMax:    0.05,
	}
}

type QualityIssue struct {
	IssueKey     string          `json:"issue_key"`
	Severity     string          `json:"severity"`
	Exchange     string          `json:"exchange,omitempty"`
	DataType     string          `json:"data_type"`
	SymbolID     int64           `json:"symbol_id,omitempty"`
	SourceSymbol string          `json:"source_symbol,omitempty"`
	IssueType    string          `json:"issue_type"`
	Details      json.RawMessage `json:"details"`
	ObservedAt   time.Time       `json:"observed_at"`
	JobID        int64           `json:"job_id,omitempty"`
	EndpointID   int64           `json:"endpoint_id,omitempty"`
}

func FilterNormalizedResult(dataType string, result normalizers.NormalizedResult, job scheduler.Job, now time.Time, cfg ValidationConfig) (normalizers.NormalizedResult, []QualityIssue) {
	if cfg.MaxFutureSkew <= 0 {
		cfg.MaxFutureSkew = DefaultValidationConfig().MaxFutureSkew
	}
	if cfg.FundingMin == 0 && cfg.FundingMax == 0 {
		cfg.FundingMin = DefaultValidationConfig().FundingMin
		cfg.FundingMax = DefaultValidationConfig().FundingMax
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()

	var issues []QualityIssue
	valid := normalizers.NormalizedResult{}

	for _, item := range result.MarketSnapshots {
		if err := validateSourceAndTime(item.SourceMeta, item.SnapshotTime, job, now, cfg); err != nil {
			issues = append(issues, qualityIssue(dataType, "market_snapshot_invalid", err, item.SourceMeta, job, now))
			continue
		}
		if err := validateFundingOptional(item.FundingRate, cfg); err != nil {
			issues = append(issues, qualityIssue(dataType, "funding_out_of_range", err, item.SourceMeta, job, now))
			continue
		}
		if err := normalizers.ValidateMarketSnapshot(item); err != nil {
			issues = append(issues, qualityIssue(dataType, "market_snapshot_invalid", err, item.SourceMeta, job, now))
			continue
		}
		valid.MarketSnapshots = append(valid.MarketSnapshots, item)
	}

	for _, item := range result.Klines {
		if err := validateSourceAndTime(item.SourceMeta, item.OpenTime, job, now, cfg); err != nil {
			issues = append(issues, qualityIssue(dataType, "kline_invalid", err, item.SourceMeta, job, now))
			continue
		}
		if err := validateInterval(item.Interval, job); err != nil {
			issues = append(issues, qualityIssue(dataType, "interval_mismatch", err, item.SourceMeta, job, now))
			continue
		}
		if err := normalizers.ValidateKline(item); err != nil {
			issues = append(issues, qualityIssue(dataType, "kline_invalid", err, item.SourceMeta, job, now))
			continue
		}
		valid.Klines = append(valid.Klines, item)
	}

	for _, item := range result.OpenInterest {
		if err := validateSourceAndTime(item.SourceMeta, item.SnapshotTime, job, now, cfg); err != nil {
			issues = append(issues, qualityIssue(dataType, "open_interest_invalid", err, item.SourceMeta, job, now))
			continue
		}
		if err := normalizers.ValidateOpenInterest(item, dataType == "open_interest_history"); err != nil {
			issues = append(issues, qualityIssue(dataType, "open_interest_invalid", err, item.SourceMeta, job, now))
			continue
		}
		valid.OpenInterest = append(valid.OpenInterest, item)
	}

	for _, item := range result.FundingSnapshots {
		if err := validateSourceAndTime(item.SourceMeta, item.SnapshotTime, job, now, cfg); err != nil {
			issues = append(issues, qualityIssue(dataType, "funding_invalid", err, item.SourceMeta, job, now))
			continue
		}
		if err := validateFunding(item.FundingRate, cfg); err != nil {
			issues = append(issues, qualityIssue(dataType, "funding_out_of_range", err, item.SourceMeta, job, now))
			continue
		}
		if err := normalizers.ValidateFundingSnapshot(item); err != nil {
			issues = append(issues, qualityIssue(dataType, "funding_invalid", err, item.SourceMeta, job, now))
			continue
		}
		valid.FundingSnapshots = append(valid.FundingSnapshots, item)
	}

	for _, item := range result.FundingHistory {
		if err := validateSourceAndTime(item.SourceMeta, item.FundingTime, job, now, cfg); err != nil {
			issues = append(issues, qualityIssue(dataType, "funding_history_invalid", err, item.SourceMeta, job, now))
			continue
		}
		if err := validateFunding(item.FundingRate, cfg); err != nil {
			issues = append(issues, qualityIssue(dataType, "funding_out_of_range", err, item.SourceMeta, job, now))
			continue
		}
		if err := normalizers.ValidateFundingHistory(item); err != nil {
			issues = append(issues, qualityIssue(dataType, "funding_history_invalid", err, item.SourceMeta, job, now))
			continue
		}
		valid.FundingHistory = append(valid.FundingHistory, item)
	}

	for _, item := range result.TakerFlows {
		if err := validateSourceAndTime(item.SourceMeta, item.SnapshotTime, job, now, cfg); err != nil {
			issues = append(issues, qualityIssue(dataType, "taker_flow_invalid", err, item.SourceMeta, job, now))
			continue
		}
		if err := validateInterval(item.Period, job); err != nil {
			issues = append(issues, qualityIssue(dataType, "interval_mismatch", err, item.SourceMeta, job, now))
			continue
		}
		if err := normalizers.ValidateTakerFlow(item); err != nil {
			issues = append(issues, qualityIssue(dataType, "taker_flow_invalid", err, item.SourceMeta, job, now))
			continue
		}
		valid.TakerFlows = append(valid.TakerFlows, item)
	}

	for _, item := range result.CVDSnapshots {
		if err := validateSourceAndTime(item.SourceMeta, item.SnapshotTime, job, now, cfg); err != nil {
			issues = append(issues, qualityIssue(dataType, "cvd_invalid", err, item.SourceMeta, job, now))
			continue
		}
		if err := validateInterval(item.Period, job); err != nil {
			issues = append(issues, qualityIssue(dataType, "interval_mismatch", err, item.SourceMeta, job, now))
			continue
		}
		if err := normalizers.ValidateCVD(item); err != nil {
			issues = append(issues, qualityIssue(dataType, "cvd_invalid", err, item.SourceMeta, job, now))
			continue
		}
		valid.CVDSnapshots = append(valid.CVDSnapshots, item)
	}

	for _, item := range result.LiquidationEvents {
		if strings.TrimSpace(item.EventKey) == "" {
			item.EventKey = LiquidationEventKey(item)
		}
		if err := validateSourceAndTime(item.SourceMeta, item.EventTime, job, now, cfg); err != nil {
			issues = append(issues, qualityIssue(dataType, "liquidation_event_invalid", err, item.SourceMeta, job, now))
			continue
		}
		if err := normalizers.ValidateLiquidationEvent(item); err != nil {
			issues = append(issues, qualityIssue(dataType, "liquidation_event_invalid", err, item.SourceMeta, job, now))
			continue
		}
		valid.LiquidationEvents = append(valid.LiquidationEvents, item)
	}

	for _, item := range result.LiquidationAggregates {
		if err := validateSourceAndTime(item.SourceMeta, item.BucketTime, job, now, cfg); err != nil {
			issues = append(issues, qualityIssue(dataType, "liquidation_aggregate_invalid", err, item.SourceMeta, job, now))
			continue
		}
		if err := validateInterval(item.Period, job); err != nil {
			issues = append(issues, qualityIssue(dataType, "interval_mismatch", err, item.SourceMeta, job, now))
			continue
		}
		if err := normalizers.ValidateLiquidationAggregate(item); err != nil {
			issues = append(issues, qualityIssue(dataType, "liquidation_aggregate_invalid", err, item.SourceMeta, job, now))
			continue
		}
		valid.LiquidationAggregates = append(valid.LiquidationAggregates, item)
	}

	for _, item := range result.LongShortRatios {
		if err := validateSourceAndTime(item.SourceMeta, item.SnapshotTime, job, now, cfg); err != nil {
			issues = append(issues, qualityIssue(dataType, "long_short_ratio_invalid", err, item.SourceMeta, job, now))
			continue
		}
		if err := validateInterval(item.Period, job); err != nil {
			issues = append(issues, qualityIssue(dataType, "interval_mismatch", err, item.SourceMeta, job, now))
			continue
		}
		if err := normalizers.ValidateLongShortRatio(item); err != nil {
			issues = append(issues, qualityIssue(dataType, "long_short_ratio_invalid", err, item.SourceMeta, job, now))
			continue
		}
		valid.LongShortRatios = append(valid.LongShortRatios, item)
	}

	for _, item := range result.BasisPremiums {
		if err := validateSourceAndTime(item.SourceMeta, item.SnapshotTime, job, now, cfg); err != nil {
			issues = append(issues, qualityIssue(dataType, "basis_invalid", err, item.SourceMeta, job, now))
			continue
		}
		if err := validateFundingOptional(item.FundingRate, cfg); err != nil {
			issues = append(issues, qualityIssue(dataType, "funding_out_of_range", err, item.SourceMeta, job, now))
			continue
		}
		if err := normalizers.ValidateBasisPremium(item); err != nil {
			issues = append(issues, qualityIssue(dataType, "basis_invalid", err, item.SourceMeta, job, now))
			continue
		}
		valid.BasisPremiums = append(valid.BasisPremiums, item)
	}

	for _, item := range result.OrderbookImbalances {
		if err := validateSourceAndTime(item.SourceMeta, item.SnapshotTime, job, now, cfg); err != nil {
			issues = append(issues, qualityIssue(dataType, "orderbook_imbalance_invalid", err, item.SourceMeta, job, now))
			continue
		}
		if err := normalizers.ValidateOrderbookImbalance(item); err != nil {
			issues = append(issues, qualityIssue(dataType, "orderbook_imbalance_invalid", err, item.SourceMeta, job, now))
			continue
		}
		valid.OrderbookImbalances = append(valid.OrderbookImbalances, item)
	}

	for _, item := range result.ExchangeDivergences {
		if err := validateSourceAndTime(item.SourceMeta, item.SnapshotTime, job, now, cfg); err != nil {
			issues = append(issues, qualityIssue(dataType, "exchange_divergence_invalid", err, item.SourceMeta, job, now))
			continue
		}
		if err := normalizers.ValidateExchangeDivergence(item); err != nil {
			issues = append(issues, qualityIssue(dataType, "exchange_divergence_invalid", err, item.SourceMeta, job, now))
			continue
		}
		valid.ExchangeDivergences = append(valid.ExchangeDivergences, item)
	}

	return valid, issues
}

func LiquidationEventKey(item normalizers.NormalizedLiquidationEvent) string {
	parts := []string{
		strings.ToLower(strings.TrimSpace(item.Exchange)),
		strings.TrimSpace(item.SourceSymbol),
		strings.ToLower(strings.TrimSpace(item.Side)),
		floatToken(item.Price),
		floatToken(item.Quantity),
		item.EventTime.UTC().Format(time.RFC3339Nano),
	}
	sum := sha1.Sum([]byte(strings.Join(parts, "|")))
	return "liq:" + hex.EncodeToString(sum[:])
}

func validateSourceAndTime(meta normalizers.SourceMeta, at time.Time, job scheduler.Job, now time.Time, cfg ValidationConfig) error {
	if err := normalizers.ValidateSource(meta); err != nil {
		return err
	}
	if strings.TrimSpace(job.SourceSymbol) != "" && strings.TrimSpace(meta.SourceSymbol) != strings.TrimSpace(job.SourceSymbol) {
		return fmt.Errorf("source_symbol %q does not match job source_symbol %q", meta.SourceSymbol, job.SourceSymbol)
	}
	if at.IsZero() {
		return fmt.Errorf("missing timestamp")
	}
	if at.After(now.Add(cfg.MaxFutureSkew)) {
		return fmt.Errorf("timestamp %s is too far in the future", at.UTC().Format(time.RFC3339))
	}
	return nil
}

func validateInterval(interval string, job scheduler.Job) error {
	if strings.TrimSpace(job.Period) == "" {
		return nil
	}
	if strings.TrimSpace(interval) != strings.TrimSpace(job.Period) {
		return fmt.Errorf("interval %q does not match job period %q", interval, job.Period)
	}
	return nil
}

func validateFundingOptional(value *float64, cfg ValidationConfig) error {
	if value == nil {
		return nil
	}
	return validateFunding(*value, cfg)
}

func validateFunding(value float64, cfg ValidationConfig) error {
	if value < cfg.FundingMin || value > cfg.FundingMax {
		return fmt.Errorf("funding rate %f outside sanity bounds [%f,%f]", value, cfg.FundingMin, cfg.FundingMax)
	}
	return nil
}

func qualityIssue(dataType string, issueType string, err error, meta normalizers.SourceMeta, job scheduler.Job, now time.Time) QualityIssue {
	details, _ := json.Marshal(map[string]any{
		"error":           err.Error(),
		"job_id":          job.ID,
		"idempotency_key": job.IdempotencyKey,
	})
	seed := fmt.Sprintf("%s|%s|%d|%s|%s|%s", issueType, meta.Exchange, meta.SymbolID, meta.SourceSymbol, dataType, err.Error())
	sum := sha1.Sum([]byte(seed))
	return QualityIssue{
		IssueKey:     hex.EncodeToString(sum[:]),
		Severity:     "warning",
		Exchange:     strings.ToLower(strings.TrimSpace(meta.Exchange)),
		DataType:     dataType,
		SymbolID:     meta.SymbolID,
		SourceSymbol: strings.TrimSpace(meta.SourceSymbol),
		IssueType:    issueType,
		Details:      details,
		ObservedAt:   now,
		JobID:        job.ID,
		EndpointID:   scheduler.RuntimeMetadataFromJob(job).EndpointID,
	}
}

func floatToken(value *float64) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%.18f", *value)
}
