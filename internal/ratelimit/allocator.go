package ratelimit

import (
	"sort"
	"strings"
)

type WorkClass string

const (
	WorkWatchlistRealtime  WorkClass = "watchlist_realtime"
	WorkTop100OneMinute    WorkClass = "top100_1m"
	WorkAllCoreFiveMinute  WorkClass = "all_5m_core"
	WorkHistoricalBackfill WorkClass = "historical_backfill"
	WorkDebugRetry         WorkClass = "debug_retry"
)

type AllocationRequest struct {
	Exchange string
	Class    WorkClass
	Demand   int
}

type Allocation struct {
	Exchange string    `json:"exchange"`
	Class    WorkClass `json:"class"`
	Granted  int       `json:"granted"`
}

type BudgetAllocator struct {
	CapacityByExchange map[string]int
}

func (a BudgetAllocator) Allocate(requests []AllocationRequest) []Allocation {
	grouped := map[string][]AllocationRequest{}
	for _, req := range requests {
		if req.Demand <= 0 {
			continue
		}
		exchange := strings.ToLower(strings.TrimSpace(req.Exchange))
		if exchange == "" {
			continue
		}
		req.Exchange = exchange
		grouped[exchange] = append(grouped[exchange], req)
	}

	exchanges := make([]string, 0, len(grouped))
	for exchange := range grouped {
		exchanges = append(exchanges, exchange)
	}
	sort.Strings(exchanges)

	var allocations []Allocation
	for _, exchange := range exchanges {
		capacity := a.CapacityByExchange[exchange]
		if capacity <= 0 {
			continue
		}
		requests := grouped[exchange]
		sort.SliceStable(requests, func(i, j int) bool {
			return workClassPriority(requests[i].Class) < workClassPriority(requests[j].Class)
		})
		for _, req := range requests {
			if capacity <= 0 {
				break
			}
			granted := req.Demand
			if granted > capacity {
				granted = capacity
			}
			capacity -= granted
			allocations = append(allocations, Allocation{
				Exchange: exchange,
				Class:    req.Class,
				Granted:  granted,
			})
		}
	}

	return allocations
}

func workClassPriority(class WorkClass) int {
	switch class {
	case WorkWatchlistRealtime:
		return 1
	case WorkTop100OneMinute:
		return 2
	case WorkAllCoreFiveMinute:
		return 3
	case WorkHistoricalBackfill:
		return 4
	case WorkDebugRetry:
		return 5
	default:
		return 99
	}
}
