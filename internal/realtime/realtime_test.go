package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"aggregator-services/internal/normalizers"
	"aggregator-services/internal/scheduler"
)

func TestRedisKeysAreStable(t *testing.T) {
	if got := LatestMarketKey(" Binance ", "btcusdt"); got != "deriv:latest:market:binance:BTCUSDT" {
		t.Fatalf("LatestMarketKey = %q", got)
	}
	if got := LatestOpenInterestKey("okx", "BTC-USDT-SWAP"); got != "deriv:latest:oi:okx:BTC-USDT-SWAP" {
		t.Fatalf("LatestOpenInterestKey = %q", got)
	}
	if got := LatestFundingKey("bybit", "BTCUSDT"); got != "deriv:latest:funding:bybit:BTCUSDT" {
		t.Fatalf("LatestFundingKey = %q", got)
	}
	if got := LatestOrderbookImbalanceKey("gate", "BTC_USDT"); got != "deriv:latest:orderbook_imbalance:gate:BTC_USDT" {
		t.Fatalf("LatestOrderbookImbalanceKey = %q", got)
	}
	if got := LatestAggregateKey(42); got != "deriv:latest:aggregate:42" {
		t.Fatalf("LatestAggregateKey = %q", got)
	}
	if got := WSStateKey("MEXC", "ticker market"); got != "deriv:ws:state:mexc:ticker_market" {
		t.Fatalf("WSStateKey = %q", got)
	}
}

func TestMemoryStoreHonorsTTL(t *testing.T) {
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	store := NewMemoryStoreWithClock(func() time.Time { return now })

	if err := store.SetRaw(context.Background(), "k", []byte("v"), time.Minute); err != nil {
		t.Fatalf("SetRaw() error = %v", err)
	}
	now = now.Add(59 * time.Second)
	if value, ok, err := store.GetRaw(context.Background(), "k"); err != nil || !ok || string(value) != "v" {
		t.Fatalf("before expiry value=%q ok=%v err=%v", value, ok, err)
	}
	now = now.Add(time.Second)
	if _, ok, err := store.GetRaw(context.Background(), "k"); err != nil || ok {
		t.Fatalf("after expiry ok=%v err=%v", ok, err)
	}
}

func TestNormalizedWriterStoresLatestWithoutDatabase(t *testing.T) {
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	store := NewMemoryStoreWithClock(func() time.Time { return now })
	price := 100.0
	oi := 250.0
	funding := 0.0001

	writer := NormalizedWriter{Latest: LatestWriter{Store: store, TTL: time.Minute, Now: func() time.Time { return now }}}
	err := writer.Write(context.Background(), "ticker", normalizers.NormalizedResult{
		MarketSnapshots: []normalizers.NormalizedMarketSnapshot{{
			SourceMeta:   normalizers.SourceMeta{SymbolID: 7, Exchange: "binance", SourceSymbol: "BTCUSDT"},
			SnapshotTime: now,
			LastPrice:    &price,
			OpenInterest: &oi,
			FundingRate:  &funding,
		}},
	}, scheduler.Job{ID: 99, Exchange: "binance"})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	var market LatestEvent
	ok, err := GetJSON(context.Background(), store, LatestMarketKey("binance", "BTCUSDT"), &market)
	if err != nil || !ok {
		t.Fatalf("market latest ok=%v err=%v", ok, err)
	}
	if market.Kind != KindMarket || market.SymbolID != 7 {
		t.Fatalf("market latest = %+v", market)
	}
	if _, ok, err := store.GetRaw(context.Background(), LatestOpenInterestKey("binance", "BTCUSDT")); err != nil || !ok {
		t.Fatalf("oi latest ok=%v err=%v", ok, err)
	}
	if _, ok, err := store.GetRaw(context.Background(), LatestFundingKey("binance", "BTCUSDT")); err != nil || !ok {
		t.Fatalf("funding latest ok=%v err=%v", ok, err)
	}
	if _, ok, err := store.GetRaw(context.Background(), CollectorHealthKey("binance", "ticker")); err != nil || !ok {
		t.Fatalf("collector health ok=%v err=%v", ok, err)
	}
}

func TestReconnectManagerTracksHeartbeatStaleAndBackoff(t *testing.T) {
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	store := NewMemoryStoreWithClock(func() time.Time { return now })
	manager := ReconnectManager{
		Store:             store,
		HeartbeatInterval: 10 * time.Second,
		StaleAfter:        30 * time.Second,
		BackoffMin:        time.Second,
		BackoffMax:        8 * time.Second,
		Now:               func() time.Time { return now },
	}

	state, err := manager.Connected(context.Background(), "binance", "ticker", []Subscription{{Stream: "ticker"}})
	if err != nil {
		t.Fatalf("Connected() error = %v", err)
	}
	now = now.Add(9 * time.Second)
	if manager.ShouldHeartbeat(state) {
		t.Fatal("ShouldHeartbeat before interval = true")
	}
	now = now.Add(time.Second)
	if !manager.ShouldHeartbeat(state) {
		t.Fatal("ShouldHeartbeat after interval = false")
	}

	state, err = manager.RecordMessage(context.Background(), state, now.Add(-2*time.Second))
	if err != nil {
		t.Fatalf("RecordMessage() error = %v", err)
	}
	if state.LagSeconds != 2 {
		t.Fatalf("LagSeconds = %d", state.LagSeconds)
	}
	now = now.Add(31 * time.Second)
	if !manager.IsStale(state) {
		t.Fatal("IsStale = false")
	}
	if got := manager.BackoffDelay(4); got != 8*time.Second {
		t.Fatalf("BackoffDelay = %s", got)
	}
}

func TestFallbackResolverUsesRESTWhenRedisIsStale(t *testing.T) {
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	store := NewMemoryStoreWithClock(func() time.Time { return now })
	stale := LatestEvent{
		Kind:         KindMarket,
		Exchange:     "binance",
		SourceSymbol: "BTCUSDT",
		EventTime:    now.Add(-time.Minute),
		ReceivedAt:   now.Add(-time.Minute),
		Payload:      json.RawMessage(`{"price":1}`),
	}
	if err := (LatestWriter{Store: store, TTL: time.Hour, Now: func() time.Time { return now }}).Write(context.Background(), stale); err != nil {
		t.Fatalf("seed stale latest: %v", err)
	}

	resolver := FallbackResolver{
		Store:  store,
		Writer: LatestWriter{Store: store, TTL: time.Hour, Now: func() time.Time { return now }},
		MaxAge: 10 * time.Second,
		Now:    func() time.Time { return now },
		RESTFallback: func(ctx context.Context) (LatestEvent, error) {
			return LatestEvent{
				Kind:         KindMarket,
				Exchange:     "binance",
				SourceSymbol: "BTCUSDT",
				EventTime:    now,
				Payload:      json.RawMessage(`{"price":2}`),
			}, nil
		},
	}
	resolved, err := resolver.LatestOrFallback(context.Background(), KindMarket, "binance", "BTCUSDT", 0)
	if err != nil {
		t.Fatalf("LatestOrFallback() error = %v", err)
	}
	if resolved.Source != "rest_fallback" {
		t.Fatalf("Source = %q", resolved.Source)
	}
	if string(resolved.Event.Payload) != `{"price":2}` {
		t.Fatalf("Payload = %s", resolved.Event.Payload)
	}
}

func TestSnapshotWorkerFlushesBucketedLatestOnly(t *testing.T) {
	now := time.Date(2026, 5, 25, 12, 1, 0, 0, time.UTC)
	sink := &recordingSink{}
	worker := NewSnapshotWorker(sink, time.Minute)
	worker.Now = func() time.Time { return now }

	worker.Observe(LatestEvent{Kind: KindMarket, Exchange: "binance", SourceSymbol: "BTCUSDT", EventTime: now, Payload: json.RawMessage(`{"price":1}`)})
	worker.Observe(LatestEvent{Kind: KindMarket, Exchange: "binance", SourceSymbol: "BTCUSDT", EventTime: now.Add(time.Second), Payload: json.RawMessage(`{"price":2}`)})
	count, err := worker.Flush(context.Background())
	if err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	if count != 1 || len(sink.events) != 1 {
		t.Fatalf("flushed count=%d events=%d", count, len(sink.events))
	}
	if string(sink.events[0].Payload) != `{"price":2}` {
		t.Fatalf("flushed payload = %s", sink.events[0].Payload)
	}
}

func TestFallbackStoreFallsBackOnPrimaryFailure(t *testing.T) {
	fallback := NewMemoryStore()
	store := NewFallbackStore(failingStore{}, fallback)
	if err := store.SetRaw(context.Background(), "k", []byte("v"), 0); err != nil {
		t.Fatalf("SetRaw() error = %v", err)
	}
	if value, ok, err := store.GetRaw(context.Background(), "k"); err != nil || !ok || string(value) != "v" {
		t.Fatalf("GetRaw() value=%q ok=%v err=%v", value, ok, err)
	}
}

type recordingSink struct {
	bucket time.Time
	events []LatestEvent
}

func (s *recordingSink) WriteRealtimeSnapshots(ctx context.Context, bucket time.Time, events []LatestEvent) error {
	s.bucket = bucket
	s.events = append([]LatestEvent(nil), events...)
	return nil
}

type failingStore struct{}

func (failingStore) SetRaw(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return errors.New("primary down")
}

func (failingStore) GetRaw(ctx context.Context, key string) ([]byte, bool, error) {
	return nil, false, errors.New("primary down")
}
