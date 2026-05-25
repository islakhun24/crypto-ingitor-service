package core

import (
	"context"
	"strings"
	"time"

	"aggregator-services/internal/hardening"
	"aggregator-services/internal/normalizers"
	"aggregator-services/internal/repositories"
	"aggregator-services/internal/scheduler"
)

type ResultWriter interface {
	Write(ctx context.Context, dataType string, result normalizers.NormalizedResult, job scheduler.Job) error
}

type QualityReporter interface {
	UpsertIssues(ctx context.Context, issues []hardening.QualityIssue) (int, error)
}

type RepositoryWriter struct {
	MarketSnapshot *repositories.MarketSnapshotRepository
	Kline          *repositories.KlineRepository
	OpenInterest   *repositories.OpenInterestRepository
	Funding        *repositories.FundingRepository
	Advanced       *repositories.AdvancedRepository
	Quality        QualityReporter
	Validation     hardening.ValidationConfig
	Now            func() time.Time
}

func (w RepositoryWriter) Write(ctx context.Context, dataType string, result normalizers.NormalizedResult, job scheduler.Job) error {
	result, issues := hardening.FilterNormalizedResult(dataType, result, job, w.now(), w.validation())
	if len(issues) > 0 && w.Quality != nil {
		if _, err := w.Quality.UpsertIssues(ctx, issues); err != nil {
			return err
		}
	}

	if w.MarketSnapshot != nil && len(result.MarketSnapshots) > 0 {
		if _, err := w.MarketSnapshot.Upsert(ctx, result.MarketSnapshots); err != nil {
			return err
		}
	}
	if w.Kline != nil && len(result.Klines) > 0 {
		if _, err := w.Kline.Upsert(ctx, result.Klines); err != nil {
			return err
		}
	}
	if w.OpenInterest != nil {
		if len(result.OpenInterest) > 0 {
			if dataType == "open_interest_history" {
				if _, err := w.OpenInterest.UpsertHistory(ctx, result.OpenInterest); err != nil {
					return err
				}
			} else {
				if _, err := w.OpenInterest.UpsertSnapshots(ctx, result.OpenInterest); err != nil {
					return err
				}
			}
		}
		if dataType == "open_interest" {
			derived := deriveOpenInterest(result.MarketSnapshots)
			if len(derived) > 0 {
				if _, err := w.OpenInterest.UpsertSnapshots(ctx, derived); err != nil {
					return err
				}
			}
		}
	}
	if w.Funding != nil {
		if len(result.FundingSnapshots) > 0 {
			if _, err := w.Funding.UpsertSnapshots(ctx, result.FundingSnapshots); err != nil {
				return err
			}
		}
		if len(result.FundingHistory) > 0 {
			if _, err := w.Funding.UpsertHistory(ctx, result.FundingHistory); err != nil {
				return err
			}
		}
		if dataType == "funding" {
			derived := deriveFunding(result.MarketSnapshots)
			if len(derived) > 0 {
				if _, err := w.Funding.UpsertSnapshots(ctx, derived); err != nil {
					return err
				}
			}
		}
	}
	if w.Advanced != nil {
		if len(result.LongShortRatios) > 0 {
			if _, err := w.Advanced.UpsertLongShortRatios(ctx, result.LongShortRatios); err != nil {
				return err
			}
		}
		if len(result.TakerFlows) > 0 {
			if _, err := w.Advanced.UpsertTakerFlows(ctx, result.TakerFlows); err != nil {
				return err
			}
			if _, err := w.Advanced.UpsertCVDFromTakerFlows(ctx, result.TakerFlows); err != nil {
				return err
			}
		}
		if len(result.CVDSnapshots) > 0 {
			if _, err := w.Advanced.UpsertCVD(ctx, result.CVDSnapshots); err != nil {
				return err
			}
		}
		if len(result.LiquidationEvents) > 0 {
			if _, err := w.Advanced.UpsertLiquidationEvents(ctx, result.LiquidationEvents); err != nil {
				return err
			}
			derived := deriveLiquidationAggregates(job, result.LiquidationEvents)
			if len(derived) > 0 {
				if _, err := w.Advanced.UpsertLiquidationAggregates(ctx, derived); err != nil {
					return err
				}
			}
		}
		if len(result.LiquidationAggregates) > 0 {
			if _, err := w.Advanced.UpsertLiquidationAggregates(ctx, result.LiquidationAggregates); err != nil {
				return err
			}
		}
		if len(result.BasisPremiums) > 0 {
			if _, err := w.Advanced.UpsertBasisPremiums(ctx, result.BasisPremiums); err != nil {
				return err
			}
		}
		if len(result.OrderbookImbalances) > 0 {
			if _, err := w.Advanced.UpsertOrderbookImbalances(ctx, result.OrderbookImbalances); err != nil {
				return err
			}
		}
		if len(result.ExchangeDivergences) > 0 {
			if _, err := w.Advanced.UpsertExchangeDivergences(ctx, result.ExchangeDivergences); err != nil {
				return err
			}
		}
	}

	return nil
}

func (w RepositoryWriter) now() time.Time {
	if w.Now != nil {
		return w.Now().UTC()
	}
	return time.Now().UTC()
}

func (w RepositoryWriter) validation() hardening.ValidationConfig {
	cfg := w.Validation
	defaults := hardening.DefaultValidationConfig()
	if cfg.MaxFutureSkew <= 0 {
		cfg.MaxFutureSkew = defaults.MaxFutureSkew
	}
	if cfg.FundingMin == 0 && cfg.FundingMax == 0 {
		cfg.FundingMin = defaults.FundingMin
		cfg.FundingMax = defaults.FundingMax
	}
	return cfg
}

func deriveOpenInterest(snapshots []normalizers.NormalizedMarketSnapshot) []normalizers.NormalizedOpenInterest {
	result := make([]normalizers.NormalizedOpenInterest, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot.OpenInterest == nil {
			continue
		}

		result = append(result, normalizers.NormalizedOpenInterest{
			SourceMeta:        snapshot.SourceMeta,
			SnapshotTime:      snapshot.SnapshotTime,
			OpenInterest:      *snapshot.OpenInterest,
			OpenInterestValue: nil,
		})
	}
	return result
}

func deriveFunding(snapshots []normalizers.NormalizedMarketSnapshot) []normalizers.NormalizedFundingSnapshot {
	result := make([]normalizers.NormalizedFundingSnapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot.FundingRate == nil {
			continue
		}

		result = append(result, normalizers.NormalizedFundingSnapshot{
			SourceMeta:   snapshot.SourceMeta,
			SnapshotTime: snapshot.SnapshotTime,
			FundingRate:  *snapshot.FundingRate,
			MarkPrice:    snapshot.MarkPrice,
			IndexPrice:   snapshot.IndexPrice,
		})
	}
	return result
}

func deriveLiquidationAggregates(job scheduler.Job, events []normalizers.NormalizedLiquidationEvent) []normalizers.NormalizedLiquidationAggregate {
	period := liquidationPeriod(job)
	bucketSize := liquidationBucketSize(period)
	buckets := map[time.Time]*normalizers.NormalizedLiquidationAggregate{}

	for _, event := range events {
		if event.EventTime.IsZero() || event.SymbolID == 0 {
			continue
		}

		bucketTime := event.EventTime.UTC().Truncate(bucketSize)
		aggregate := buckets[bucketTime]
		if aggregate == nil {
			aggregate = &normalizers.NormalizedLiquidationAggregate{
				SourceMeta: normalizers.SourceMeta{
					SymbolID:         event.SymbolID,
					Exchange:         event.Exchange,
					SourceSymbol:     event.SourceSymbol,
					SourceEndpointID: event.SourceEndpointID,
					RawData:          event.RawData,
				},
				Period:     period,
				BucketTime: bucketTime,
			}
			buckets[bucketTime] = aggregate
		}

		usd := eventUSD(event)
		side := strings.ToLower(strings.TrimSpace(event.Side))
		switch side {
		case "sell", "long":
			aggregate.LongLiquidationCount++
			aggregate.LongLiquidationUSD += usd
			aggregate.LongLiquidationNotional += usd
		case "buy", "short":
			aggregate.ShortLiquidationCount++
			aggregate.ShortLiquidationUSD += usd
			aggregate.ShortLiquidationNotional += usd
		default:
			aggregate.ShortLiquidationCount++
			aggregate.ShortLiquidationUSD += usd
			aggregate.ShortLiquidationNotional += usd
		}
		aggregate.TotalLiquidationUSD += usd
		aggregate.TotalLiquidationNotional += usd
		if usd > aggregate.LargestLiquidationUSD {
			aggregate.LargestLiquidationUSD = usd
		}
	}

	result := make([]normalizers.NormalizedLiquidationAggregate, 0, len(buckets))
	for _, aggregate := range buckets {
		result = append(result, *aggregate)
	}

	return result
}

func liquidationPeriod(job scheduler.Job) string {
	if strings.TrimSpace(job.Period) != "" {
		return strings.TrimSpace(job.Period)
	}
	if strings.EqualFold(job.Tier, scheduler.TierAll) {
		return "5m"
	}
	return "1m"
}

func liquidationBucketSize(period string) time.Duration {
	switch strings.ToLower(strings.TrimSpace(period)) {
	case "5m":
		return 5 * time.Minute
	default:
		return time.Minute
	}
}

func eventUSD(event normalizers.NormalizedLiquidationEvent) float64 {
	if event.USDValue != nil {
		return *event.USDValue
	}
	if event.Notional != nil {
		return *event.Notional
	}
	if event.Price != nil && event.Quantity != nil {
		return *event.Price * *event.Quantity
	}
	return 0
}
