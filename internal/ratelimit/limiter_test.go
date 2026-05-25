package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestLimiterWaitHonorsContextCancellation(t *testing.T) {
	limiter := NewLimiter()
	if err := limiter.Configure("binance", Budget{
		RequestsPerSecond: 1,
		RequestsPerMinute: 60,
		Burst:             1,
	}); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	if err := limiter.Wait(context.Background(), Key{Exchange: "binance", Endpoint: "ticker", DataType: "ticker"}, 1); err != nil {
		t.Fatalf("first Wait() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	if err := limiter.Wait(ctx, Key{Exchange: "binance", Endpoint: "ticker", DataType: "ticker"}, 1); err == nil {
		t.Fatal("second Wait() error = nil, want context deadline")
	}
}

func TestCircuitBreakerOpensAndHalfOpens(t *testing.T) {
	breaker := NewCircuitBreaker(CircuitSettings{
		FailureThreshold:  2,
		Cooldown:          10 * time.Millisecond,
		HalfOpenMaxPasses: 1,
	})
	key := Key{Exchange: "okx", Endpoint: "ticker", DataType: "ticker"}

	breaker.RecordFailure(key, FailureServerError)
	if breaker.State(key) != CircuitClosed {
		t.Fatalf("state after first failure = %s", breaker.State(key))
	}

	breaker.RecordFailure(key, FailureRateLimited)
	if breaker.State(key) != CircuitOpen {
		t.Fatalf("state after second failure = %s", breaker.State(key))
	}

	if _, err := breaker.Allow(context.Background(), key); err == nil {
		t.Fatal("Allow() error = nil while circuit is open")
	}

	time.Sleep(15 * time.Millisecond)

	state, err := breaker.Allow(context.Background(), key)
	if err != nil {
		t.Fatalf("Allow() after cooldown error = %v", err)
	}
	if state != CircuitHalfOpen {
		t.Fatalf("state after cooldown = %s", state)
	}

	breaker.RecordSuccess(key)
	if breaker.State(key) != CircuitClosed {
		t.Fatalf("state after success = %s", breaker.State(key))
	}
}
