package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aggregator-services/internal/endpoints"
	"aggregator-services/internal/symbols"
)

type PolicyStore interface {
	ListActivePolicies(ctx context.Context) ([]Policy, error)
}

type EndpointStore interface {
	ListActiveByExchangeDataType(ctx context.Context, exchange string, dataType string) ([]endpoints.Endpoint, error)
}

type SymbolTierStore interface {
	ListSymbolMarketsByTier(ctx context.Context, tier string, exchange string, limit int) ([]symbols.SymbolMarket, error)
}

type JobStore interface {
	InsertJobs(ctx context.Context, jobs []Job) (int, error)
}

type Planner struct {
	Policies  PolicyStore
	Endpoints EndpointStore
	Symbols   SymbolTierStore
	Jobs      JobStore
	Now       func() time.Time
}

func (p Planner) Run(ctx context.Context) (PlanResult, error) {
	now := time.Now().UTC()
	if p.Now != nil {
		now = p.Now().UTC()
	}

	return p.RunAt(ctx, now)
}

func (p Planner) RunAt(ctx context.Context, now time.Time) (PlanResult, error) {
	result := NewPlanResult()

	policies, err := p.Policies.ListActivePolicies(ctx)
	if err != nil {
		return result, err
	}

	var jobs []Job
	endpointCache := make(map[string][]endpoints.Endpoint)
	symbolCache := make(map[string][]symbols.SymbolMarket)

	for _, policy := range policies {
		if !policy.Enabled || policy.IntervalSeconds <= 0 {
			result.Skip(policyKey(policy), "policy disabled or interval invalid")
			continue
		}

		endpointDataType, requiresEndpoint := EndpointDataType(policy.DataType)
		activeEndpoints := []endpoints.Endpoint{{Name: "internal", RequestWeight: 1, TimeoutMS: 10000}}
		if requiresEndpoint {
			cacheKey := policy.Exchange + ":" + endpointDataType
			activeEndpoints = endpointCache[cacheKey]
			if activeEndpoints == nil {
				activeEndpoints, err = p.Endpoints.ListActiveByExchangeDataType(ctx, policy.Exchange, endpointDataType)
				if err != nil {
					return result, err
				}
				endpointCache[cacheKey] = activeEndpoints
			}
			if len(activeEndpoints) == 0 {
				result.Skip(policyKey(policy), "no active endpoint for data type")
				continue
			}
		}

		symbolLimit := SymbolLimitForTier(policy.Tier)
		symbolCacheKey := fmt.Sprintf("%s:%s:%d", policy.Tier, policy.Exchange, symbolLimit)
		symbolMarkets := symbolCache[symbolCacheKey]
		if symbolMarkets == nil {
			symbolMarkets, err = p.Symbols.ListSymbolMarketsByTier(ctx, policy.Tier, policy.Exchange, symbolLimit)
			if err != nil {
				return result, err
			}
			symbolCache[symbolCacheKey] = symbolMarkets
		}
		if len(symbolMarkets) == 0 {
			result.Skip(policyKey(policy), "no symbols in tier for exchange")
			continue
		}

		bucket := ScheduledBucket(now, policy.IntervalSeconds)
		for _, symbolMarket := range symbolMarkets {
			if strings.TrimSpace(symbolMarket.SourceSymbol) == "" {
				result.Skip(policyKey(policy), "symbol missing source_symbol")
				continue
			}

			endpoint := activeEndpoints[0]
			metadata := plannerMetadata(policy, endpoint, endpointDataType, requiresEndpoint)
			jobs = append(jobs, Job{
				Exchange:       policy.Exchange,
				DataType:       policy.DataType,
				Tier:           policy.Tier,
				SymbolID:       symbolMarket.SymbolID,
				SourceSymbol:   symbolMarket.SourceSymbol,
				Period:         policy.Period,
				IdempotencyKey: IdempotencyKey(policy.Exchange, policy.DataType, symbolMarket.SymbolID, symbolMarket.SourceSymbol, policy.Period, bucket),
				Status:         JobStatusPending,
				Priority:       policy.Priority,
				ScheduledAt:    bucket,
				RetryCount:     0,
				MaxRetry:       policy.MaxRetry,
				Metadata:       metadata,
			})
		}
	}

	result.AttemptedJobs = len(jobs)
	if len(jobs) == 0 {
		return result, nil
	}

	inserted, err := p.Jobs.InsertJobs(ctx, jobs)
	if err != nil {
		return result, err
	}
	result.InsertedJobs = inserted

	return result, nil
}

func policyKey(policy Policy) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s", policy.Exchange, policy.MarketType, policy.DataType, policy.Tier, policy.Period)
}

func plannerMetadata(policy Policy, endpoint endpoints.Endpoint, endpointDataType string, requiresEndpoint bool) json.RawMessage {
	metadata := map[string]any{
		"policy_id":         policy.ID,
		"market_type":       policy.MarketType,
		"requires_endpoint": requiresEndpoint,
	}

	if requiresEndpoint {
		metadata["endpoint_id"] = endpoint.ID
		metadata["endpoint_name"] = endpoint.Name
		metadata["endpoint_data_type"] = endpointDataType
		metadata["request_weight"] = endpoint.RequestWeight
		metadata["timeout_ms"] = endpoint.TimeoutMS
		metadata["rate_limit_per_second"] = endpoint.RateLimitPerSecond
		metadata["rate_limit_per_minute"] = endpoint.RateLimitPerMinute
	}

	raw, err := json.Marshal(metadata)
	if err != nil {
		return []byte(`{}`)
	}

	return raw
}
