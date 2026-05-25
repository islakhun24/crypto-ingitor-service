package ratelimit

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

type Key struct {
	Exchange string
	Endpoint string
	DataType string
}

func (k Key) String() string {
	return strings.Join([]string{
		strings.ToLower(strings.TrimSpace(k.Exchange)),
		strings.TrimSpace(k.Endpoint),
		strings.TrimSpace(k.DataType),
	}, ":")
}

type Budget struct {
	RequestsPerSecond float64
	RequestsPerMinute float64
	Burst             int
	JitterMin         time.Duration
	JitterMax         time.Duration
}

type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	now     func() time.Time
	rand    *rand.Rand
}

func NewLimiter() *Limiter {
	return &Limiter{
		buckets: make(map[string]*bucket),
		now:     time.Now,
		rand:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (l *Limiter) Configure(exchange string, budget Budget) error {
	exchange = strings.ToLower(strings.TrimSpace(exchange))
	if exchange == "" {
		return fmt.Errorf("exchange is required")
	}
	if budget.RequestsPerSecond <= 0 {
		return fmt.Errorf("requests per second must be greater than 0")
	}
	if budget.RequestsPerMinute <= 0 {
		budget.RequestsPerMinute = budget.RequestsPerSecond * 60
	}
	if budget.Burst <= 0 {
		budget.Burst = max(1, int(budget.RequestsPerSecond))
	}
	if budget.JitterMax < budget.JitterMin {
		budget.JitterMax = budget.JitterMin
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.buckets[exchange] = newBucket(budget, l.now())
	return nil
}

func (l *Limiter) Wait(ctx context.Context, key Key, weight int) error {
	if weight <= 0 {
		weight = 1
	}

	exchange := strings.ToLower(strings.TrimSpace(key.Exchange))
	if exchange == "" {
		return fmt.Errorf("exchange is required")
	}

	for {
		delay, jitter, err := l.reserve(exchange, weight)
		if err != nil {
			return err
		}
		waitFor := delay + jitter
		if waitFor <= 0 {
			return nil
		}

		timer := time.NewTimer(waitFor)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (l *Limiter) reserve(exchange string, weight int) (time.Duration, time.Duration, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	bucket, ok := l.buckets[exchange]
	if !ok {
		return 0, 0, fmt.Errorf("rate limit budget for %s is not configured", exchange)
	}

	delay := bucket.reserve(l.now(), weight)
	jitter := time.Duration(0)
	if delay == 0 && bucket.budget.JitterMax > 0 {
		span := bucket.budget.JitterMax - bucket.budget.JitterMin
		jitter = bucket.budget.JitterMin
		if span > 0 {
			jitter += time.Duration(l.rand.Int63n(int64(span)))
		}
	}

	return delay, jitter, nil
}

type bucket struct {
	budget       Budget
	secondTokens float64
	minuteTokens float64
	lastRefill   time.Time
}

func newBucket(budget Budget, now time.Time) *bucket {
	burst := float64(budget.Burst)
	minuteBurst := maxFloat(budget.RequestsPerMinute, burst)

	return &bucket{
		budget:       budget,
		secondTokens: burst,
		minuteTokens: minuteBurst,
		lastRefill:   now,
	}
}

func (b *bucket) reserve(now time.Time, weight int) time.Duration {
	if weight > b.budget.Burst {
		b.budget.Burst = weight
	}

	b.refill(now)

	needed := float64(weight)
	if b.secondTokens >= needed && b.minuteTokens >= needed {
		b.secondTokens -= needed
		b.minuteTokens -= needed
		return 0
	}

	secondDelay := delayFor(needed-b.secondTokens, b.budget.RequestsPerSecond)
	minuteDelay := delayFor(needed-b.minuteTokens, b.budget.RequestsPerMinute/60)
	return maxDuration(secondDelay, minuteDelay)
}

func (b *bucket) refill(now time.Time) {
	if now.Before(b.lastRefill) {
		b.lastRefill = now
		return
	}

	elapsed := now.Sub(b.lastRefill).Seconds()
	if elapsed <= 0 {
		return
	}

	burst := float64(b.budget.Burst)
	minuteBurst := maxFloat(b.budget.RequestsPerMinute, burst)
	b.secondTokens = minFloat(burst, b.secondTokens+elapsed*b.budget.RequestsPerSecond)
	b.minuteTokens = minFloat(minuteBurst, b.minuteTokens+elapsed*(b.budget.RequestsPerMinute/60))
	b.lastRefill = now
}

func delayFor(missing float64, refillPerSecond float64) time.Duration {
	if missing <= 0 || refillPerSecond <= 0 {
		return 0
	}

	return time.Duration((missing / refillPerSecond) * float64(time.Second))
}

func maxDuration(a time.Duration, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func minFloat(a float64, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a float64, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
