package realtime

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

type Store interface {
	SetRaw(ctx context.Context, key string, value []byte, ttl time.Duration) error
	GetRaw(ctx context.Context, key string) ([]byte, bool, error)
}

func SetJSON(ctx context.Context, store Store, key string, value any, ttl time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return store.SetRaw(ctx, key, raw, ttl)
}

func GetJSON(ctx context.Context, store Store, key string, target any) (bool, error) {
	raw, ok, err := store.GetRaw(ctx, key)
	if err != nil || !ok {
		return ok, err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return false, err
	}
	return true, nil
}

type MemoryStore struct {
	mu    sync.Mutex
	items map[string]memoryItem
	now   func() time.Time
}

type memoryItem struct {
	value     []byte
	expiresAt time.Time
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		items: map[string]memoryItem{},
		now:   time.Now,
	}
}

func NewMemoryStoreWithClock(now func() time.Time) *MemoryStore {
	store := NewMemoryStore()
	if now != nil {
		store.now = now
	}
	return store
}

func (s *MemoryStore) SetRaw(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	copied := append([]byte(nil), value...)
	item := memoryItem{value: copied}
	if ttl > 0 {
		item.expiresAt = s.now().UTC().Add(ttl)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = item
	return nil
}

func (s *MemoryStore) GetRaw(ctx context.Context, key string) ([]byte, bool, error) {
	select {
	case <-ctx.Done():
		return nil, false, ctx.Err()
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[key]
	if !ok {
		return nil, false, nil
	}
	if !item.expiresAt.IsZero() && !s.now().UTC().Before(item.expiresAt) {
		delete(s.items, key)
		return nil, false, nil
	}

	return append([]byte(nil), item.value...), true, nil
}

type FallbackStore struct {
	Primary  Store
	Fallback Store
}

func NewFallbackStore(primary Store, fallback Store) Store {
	if primary == nil {
		return fallback
	}
	if fallback == nil {
		return primary
	}
	return FallbackStore{Primary: primary, Fallback: fallback}
}

func (s FallbackStore) SetRaw(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	var primaryErr error
	if s.Primary != nil {
		if err := s.Primary.SetRaw(ctx, key, value, ttl); err == nil {
			return nil
		} else {
			primaryErr = err
		}
	}
	if s.Fallback == nil {
		return primaryErr
	}
	return s.Fallback.SetRaw(ctx, key, value, ttl)
}

func (s FallbackStore) GetRaw(ctx context.Context, key string) ([]byte, bool, error) {
	if s.Primary != nil {
		value, ok, err := s.Primary.GetRaw(ctx, key)
		if err == nil && ok {
			return value, true, nil
		}
		if err == nil && s.Fallback == nil {
			return nil, false, nil
		}
	}
	if s.Fallback == nil {
		return nil, false, nil
	}
	return s.Fallback.GetRaw(ctx, key)
}
