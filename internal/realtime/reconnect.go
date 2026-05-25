package realtime

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

type Subscription struct {
	Stream       string `json:"stream"`
	Exchange     string `json:"exchange"`
	SourceSymbol string `json:"source_symbol,omitempty"`
}

type SubscriptionClient interface {
	Ping(ctx context.Context) error
	Subscribe(ctx context.Context, subscriptions []Subscription) error
	Close() error
}

type StreamState struct {
	Exchange        string    `json:"exchange"`
	Stream          string    `json:"stream"`
	Status          string    `json:"status"`
	Attempt         int       `json:"attempt"`
	LastConnectedAt time.Time `json:"last_connected_at,omitempty"`
	LastMessageAt   time.Time `json:"last_message_at,omitempty"`
	LastHeartbeatAt time.Time `json:"last_heartbeat_at,omitempty"`
	LagSeconds      int64     `json:"lag_seconds"`
	LastError       string    `json:"last_error,omitempty"`
	Subscriptions   int       `json:"subscriptions"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type ReconnectManager struct {
	Store             Store
	TTL               time.Duration
	HeartbeatInterval time.Duration
	StaleAfter        time.Duration
	BackoffMin        time.Duration
	BackoffMax        time.Duration
	Now               func() time.Time
}

func (m ReconnectManager) Connected(ctx context.Context, exchange, stream string, subscriptions []Subscription) (StreamState, error) {
	now := m.now()
	state := StreamState{
		Exchange:        normalizeExchange(exchange),
		Stream:          normalizeStream(stream),
		Status:          "connected",
		LastConnectedAt: now,
		LastHeartbeatAt: now,
		Subscriptions:   len(subscriptions),
		UpdatedAt:       now,
	}
	return state, m.writeState(ctx, state)
}

func (m ReconnectManager) RecordMessage(ctx context.Context, state StreamState, eventTime time.Time) (StreamState, error) {
	now := m.now()
	if eventTime.IsZero() {
		eventTime = now
	}
	state.Status = "connected"
	state.LastMessageAt = now
	state.LagSeconds = int64(now.Sub(eventTime.UTC()).Seconds())
	if state.LagSeconds < 0 {
		state.LagSeconds = 0
	}
	state.UpdatedAt = now
	return state, m.writeState(ctx, state)
}

func (m ReconnectManager) Heartbeat(ctx context.Context, client SubscriptionClient, state StreamState) (StreamState, error) {
	if client == nil {
		return state, fmt.Errorf("subscription client is required")
	}
	if err := client.Ping(ctx); err != nil {
		state.Status = "heartbeat_failed"
		state.LastError = err.Error()
		state.UpdatedAt = m.now()
		_ = m.writeState(ctx, state)
		return state, err
	}
	state.Status = "connected"
	state.LastHeartbeatAt = m.now()
	state.UpdatedAt = state.LastHeartbeatAt
	return state, m.writeState(ctx, state)
}

func (m ReconnectManager) Resubscribe(ctx context.Context, client SubscriptionClient, subscriptions []Subscription) error {
	if client == nil {
		return fmt.Errorf("subscription client is required")
	}
	return client.Subscribe(ctx, subscriptions)
}

func (m ReconnectManager) Disconnected(ctx context.Context, state StreamState, err error) (StreamState, time.Duration, error) {
	state.Attempt++
	state.Status = "disconnected"
	state.UpdatedAt = m.now()
	if err != nil {
		state.LastError = err.Error()
	}
	delay := m.BackoffDelay(state.Attempt)
	return state, delay, m.writeState(ctx, state)
}

func (m ReconnectManager) ShouldHeartbeat(state StreamState) bool {
	interval := m.HeartbeatInterval
	if interval <= 0 {
		interval = 15 * time.Second
	}
	return state.LastHeartbeatAt.IsZero() || !m.now().Before(state.LastHeartbeatAt.Add(interval))
}

func (m ReconnectManager) IsStale(state StreamState) bool {
	staleAfter := m.StaleAfter
	if staleAfter <= 0 {
		staleAfter = 30 * time.Second
	}
	last := state.LastMessageAt
	if last.IsZero() {
		last = state.LastConnectedAt
	}
	return last.IsZero() || !m.now().Before(last.Add(staleAfter))
}

func (m ReconnectManager) BackoffDelay(attempt int) time.Duration {
	minDelay := m.BackoffMin
	if minDelay <= 0 {
		minDelay = time.Second
	}
	maxDelay := m.BackoffMax
	if maxDelay <= 0 {
		maxDelay = time.Minute
	}
	if attempt < 1 {
		attempt = 1
	}

	multiplier := math.Pow(2, float64(attempt-1))
	delay := time.Duration(float64(minDelay) * multiplier)
	if delay > maxDelay || delay < 0 {
		return maxDelay
	}
	return delay
}

func (m ReconnectManager) writeState(ctx context.Context, state StreamState) error {
	if m.Store == nil {
		return nil
	}
	state.Exchange = normalizeExchange(state.Exchange)
	state.Stream = normalizeStream(state.Stream)
	if strings.TrimSpace(state.Exchange) == "" || strings.TrimSpace(state.Stream) == "" {
		return fmt.Errorf("exchange and stream are required")
	}
	ttl := m.TTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return SetJSON(ctx, m.Store, WSStateKey(state.Exchange, state.Stream), state, ttl)
}

func (m ReconnectManager) now() time.Time {
	if m.Now != nil {
		return m.Now().UTC()
	}
	return time.Now().UTC()
}
