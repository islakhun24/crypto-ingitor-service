package scheduler

import (
	"testing"
	"time"
)

func TestIdempotencyKeyUsesDeterministicFormat(t *testing.T) {
	bucket := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)

	got := IdempotencyKey("OKX", "kline", 123, "0G-USDT-SWAP", "5m", bucket)
	want := "okx:kline:123:0G-USDT-SWAP:5m:2026-05-25T12:00:00Z"

	if got != want {
		t.Fatalf("IdempotencyKey() = %q, want %q", got, want)
	}
}

func TestScheduledBucketTruncatesToInterval(t *testing.T) {
	at := time.Date(2026, 5, 25, 12, 3, 42, 0, time.UTC)

	got := ScheduledBucket(at, 300)
	want := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)

	if !got.Equal(want) {
		t.Fatalf("ScheduledBucket() = %s, want %s", got, want)
	}
}
