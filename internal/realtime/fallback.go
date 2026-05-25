package realtime

import (
	"context"
	"fmt"
	"time"
)

type RESTFallback func(ctx context.Context) (LatestEvent, error)

type FallbackResolver struct {
	Store        Store
	Writer       LatestWriter
	MaxAge       time.Duration
	Now          func() time.Time
	RESTFallback RESTFallback
}

type ResolvedLatest struct {
	Event  LatestEvent `json:"event"`
	Source string      `json:"source"`
}

func (r FallbackResolver) LatestOrFallback(ctx context.Context, kind, exchange, sourceSymbol string, symbolID int64) (ResolvedLatest, error) {
	key, err := LatestKey(kind, exchange, sourceSymbol, symbolID)
	if err != nil {
		return ResolvedLatest{}, err
	}

	if r.Store != nil {
		var event LatestEvent
		ok, err := GetJSON(ctx, r.Store, key, &event)
		if err != nil {
			return ResolvedLatest{}, err
		}
		if ok && r.isFresh(event) {
			return ResolvedLatest{Event: event, Source: "redis"}, nil
		}
	}

	if r.RESTFallback == nil {
		return ResolvedLatest{}, fmt.Errorf("latest data is stale or missing")
	}

	event, err := r.RESTFallback(ctx)
	if err != nil {
		return ResolvedLatest{}, err
	}
	if event.Kind == "" {
		event.Kind = kind
	}
	if event.Exchange == "" {
		event.Exchange = exchange
	}
	if event.SourceSymbol == "" {
		event.SourceSymbol = sourceSymbol
	}
	if event.SymbolID == 0 {
		event.SymbolID = symbolID
	}
	if err := r.Writer.Write(ctx, event); err != nil {
		return ResolvedLatest{}, err
	}

	return ResolvedLatest{Event: event, Source: "rest_fallback"}, nil
}

func (r FallbackResolver) isFresh(event LatestEvent) bool {
	maxAge := r.MaxAge
	if maxAge <= 0 {
		maxAge = 30 * time.Second
	}
	anchor := event.ReceivedAt
	if anchor.IsZero() {
		anchor = event.EventTime
	}
	return !anchor.IsZero() && r.now().Sub(anchor.UTC()) <= maxAge
}

func (r FallbackResolver) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}
