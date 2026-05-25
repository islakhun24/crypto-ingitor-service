package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"aggregator-services/internal/normalizers"
	"aggregator-services/internal/scheduler"
)

type LatestEvent struct {
	Kind         string          `json:"kind"`
	Exchange     string          `json:"exchange,omitempty"`
	SourceSymbol string          `json:"source_symbol,omitempty"`
	SymbolID     int64           `json:"symbol_id,omitempty"`
	EventTime    time.Time       `json:"event_time"`
	ReceivedAt   time.Time       `json:"received_at"`
	Payload      json.RawMessage `json:"payload"`
}

func (e LatestEvent) Key() (string, error) {
	return LatestKey(e.Kind, e.Exchange, e.SourceSymbol, e.SymbolID)
}

type LatestWriter struct {
	Store Store
	TTL   time.Duration
	Now   func() time.Time
}

func (w LatestWriter) Write(ctx context.Context, event LatestEvent) error {
	if w.Store == nil {
		return nil
	}
	if event.EventTime.IsZero() {
		event.EventTime = w.now()
	}
	if event.ReceivedAt.IsZero() {
		event.ReceivedAt = w.now()
	}
	if len(event.Payload) == 0 {
		event.Payload = json.RawMessage(`{}`)
	}

	key, err := event.Key()
	if err != nil {
		return err
	}
	return SetJSON(ctx, w.Store, key, event, w.TTL)
}

func (w LatestWriter) WriteCollectorHealth(ctx context.Context, exchange, dataType, status string, details map[string]any) error {
	if w.Store == nil {
		return nil
	}
	payload := map[string]any{
		"exchange":    normalizeExchange(exchange),
		"data_type":   normalizeStream(dataType),
		"status":      status,
		"observed_at": w.now(),
		"details":     details,
	}
	return SetJSON(ctx, w.Store, CollectorHealthKey(exchange, dataType), payload, w.TTL)
}

func (w LatestWriter) now() time.Time {
	if w.Now != nil {
		return w.Now().UTC()
	}
	return time.Now().UTC()
}

type NormalizedWriter struct {
	Latest LatestWriter
}

func (w NormalizedWriter) Write(ctx context.Context, dataType string, result normalizers.NormalizedResult, job scheduler.Job) error {
	for _, snapshot := range result.MarketSnapshots {
		if err := w.writePayload(ctx, KindMarket, snapshot.Exchange, snapshot.SourceSymbol, snapshot.SymbolID, snapshot.SnapshotTime, snapshot); err != nil {
			return err
		}
		if snapshot.OpenInterest != nil {
			openInterest := normalizers.NormalizedOpenInterest{
				SourceMeta:        snapshot.SourceMeta,
				SnapshotTime:      snapshot.SnapshotTime,
				OpenInterest:      *snapshot.OpenInterest,
				OpenInterestValue: nil,
			}
			if err := w.writePayload(ctx, KindOpenInterest, snapshot.Exchange, snapshot.SourceSymbol, snapshot.SymbolID, snapshot.SnapshotTime, openInterest); err != nil {
				return err
			}
		}
		if snapshot.FundingRate != nil {
			funding := normalizers.NormalizedFundingSnapshot{
				SourceMeta:   snapshot.SourceMeta,
				SnapshotTime: snapshot.SnapshotTime,
				FundingRate:  *snapshot.FundingRate,
				MarkPrice:    snapshot.MarkPrice,
				IndexPrice:   snapshot.IndexPrice,
			}
			if err := w.writePayload(ctx, KindFunding, snapshot.Exchange, snapshot.SourceSymbol, snapshot.SymbolID, snapshot.SnapshotTime, funding); err != nil {
				return err
			}
		}
	}

	for _, item := range result.OpenInterest {
		if err := w.writePayload(ctx, KindOpenInterest, item.Exchange, item.SourceSymbol, item.SymbolID, item.SnapshotTime, item); err != nil {
			return err
		}
	}
	for _, item := range result.FundingSnapshots {
		if err := w.writePayload(ctx, KindFunding, item.Exchange, item.SourceSymbol, item.SymbolID, item.SnapshotTime, item); err != nil {
			return err
		}
	}
	for _, item := range result.OrderbookImbalances {
		if err := w.writePayload(ctx, KindOrderbookImbalance, item.Exchange, item.SourceSymbol, item.SymbolID, item.SnapshotTime, item); err != nil {
			return err
		}
	}

	if job.Exchange != "" && dataType != "" {
		return w.Latest.WriteCollectorHealth(ctx, job.Exchange, dataType, "updated", map[string]any{"job_id": job.ID})
	}
	return nil
}

func (w NormalizedWriter) writePayload(ctx context.Context, kind, exchange, sourceSymbol string, symbolID int64, eventTime time.Time, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal realtime payload: %w", err)
	}
	return w.Latest.Write(ctx, LatestEvent{
		Kind:         kind,
		Exchange:     exchange,
		SourceSymbol: sourceSymbol,
		SymbolID:     symbolID,
		EventTime:    eventTime,
		Payload:      raw,
	})
}
