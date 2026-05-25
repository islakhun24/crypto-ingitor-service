package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

type FailureKind string

const (
	FailureRateLimited     FailureKind = "rate_limited"
	FailureServerError     FailureKind = "server_error"
	FailureTimeout         FailureKind = "timeout"
	FailureNetworkError    FailureKind = "network_error"
	FailureBadRequest      FailureKind = "bad_request"
	FailureUnauthorized    FailureKind = "unauthorized"
	FailureNotFound        FailureKind = "not_found"
	FailureInvalidSymbol   FailureKind = "invalid_symbol"
	FailureInvalidResponse FailureKind = "invalid_response"
	FailureNormalizerError FailureKind = "normalizer_error"
	FailureUnknown         FailureKind = "unknown"
	FailureParse           FailureKind = FailureInvalidResponse
)

type CircuitSettings struct {
	FailureThreshold  int
	Cooldown          time.Duration
	HalfOpenMaxPasses int
}

type CircuitBreaker struct {
	mu       sync.Mutex
	settings CircuitSettings
	now      func() time.Time
	states   map[string]*circuit
}

func NewCircuitBreaker(settings CircuitSettings) *CircuitBreaker {
	if settings.FailureThreshold <= 0 {
		settings.FailureThreshold = 5
	}
	if settings.Cooldown <= 0 {
		settings.Cooldown = 30 * time.Second
	}
	if settings.HalfOpenMaxPasses <= 0 {
		settings.HalfOpenMaxPasses = 1
	}

	return &CircuitBreaker{
		settings: settings,
		now:      time.Now,
		states:   make(map[string]*circuit),
	}
}

func (b *CircuitBreaker) Allow(ctx context.Context, key Key) (CircuitState, error) {
	select {
	case <-ctx.Done():
		return CircuitOpen, ctx.Err()
	default:
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	state := b.stateFor(key)
	now := b.now()

	switch state.state {
	case CircuitOpen:
		if now.Before(state.openUntil) {
			return CircuitOpen, fmt.Errorf("circuit open for %s until %s", key.String(), state.openUntil.UTC().Format(time.RFC3339))
		}
		state.state = CircuitHalfOpen
		state.halfOpenPasses = 0
		return CircuitHalfOpen, nil
	default:
		return state.state, nil
	}
}

func (b *CircuitBreaker) RecordSuccess(key Key) {
	b.mu.Lock()
	defer b.mu.Unlock()

	state := b.stateFor(key)
	if state.state == CircuitHalfOpen {
		state.halfOpenPasses++
		if state.halfOpenPasses >= b.settings.HalfOpenMaxPasses {
			state.state = CircuitClosed
			state.failures = 0
			state.lastFailure = ""
			state.openUntil = time.Time{}
		}
		return
	}

	state.failures = 0
	state.lastFailure = ""
}

func (b *CircuitBreaker) RecordFailure(key Key, kind FailureKind) {
	b.mu.Lock()
	defer b.mu.Unlock()

	state := b.stateFor(key)
	state.failures++
	state.lastFailure = kind

	if state.state == CircuitHalfOpen || state.failures >= b.settings.FailureThreshold {
		state.state = CircuitOpen
		state.openUntil = b.now().Add(b.settings.Cooldown)
		state.halfOpenPasses = 0
	}
}

func (b *CircuitBreaker) State(key Key) CircuitState {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.stateFor(key).state
}

func (b *CircuitBreaker) stateFor(key Key) *circuit {
	id := key.String()
	state := b.states[id]
	if state == nil {
		state = &circuit{state: CircuitClosed}
		b.states[id] = state
	}

	return state
}

type circuit struct {
	state          CircuitState
	failures       int
	lastFailure    FailureKind
	openUntil      time.Time
	halfOpenPasses int
}
