package ratelimit

import "testing"

func TestBudgetAllocatorHonorsPriorityOrder(t *testing.T) {
	allocations := BudgetAllocator{CapacityByExchange: map[string]int{"binance": 5}}.Allocate([]AllocationRequest{
		{Exchange: "binance", Class: WorkHistoricalBackfill, Demand: 5},
		{Exchange: "binance", Class: WorkWatchlistRealtime, Demand: 3},
		{Exchange: "binance", Class: WorkTop100OneMinute, Demand: 3},
	})

	if len(allocations) != 2 {
		t.Fatalf("allocations = %+v, want 2 entries", allocations)
	}
	if allocations[0].Class != WorkWatchlistRealtime || allocations[0].Granted != 3 {
		t.Fatalf("first allocation = %+v", allocations[0])
	}
	if allocations[1].Class != WorkTop100OneMinute || allocations[1].Granted != 2 {
		t.Fatalf("second allocation = %+v", allocations[1])
	}
}
