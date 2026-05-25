package realtime

import (
	"context"
	"sync"
	"time"
)

type SnapshotSink interface {
	WriteRealtimeSnapshots(ctx context.Context, bucket time.Time, events []LatestEvent) error
}

type SnapshotWorker struct {
	Sink   SnapshotSink
	Bucket time.Duration
	Now    func() time.Time

	mu     sync.Mutex
	latest map[string]LatestEvent
}

func NewSnapshotWorker(sink SnapshotSink, bucket time.Duration) *SnapshotWorker {
	if bucket <= 0 {
		bucket = time.Minute
	}
	return &SnapshotWorker{
		Sink:   sink,
		Bucket: bucket,
		Now:    time.Now,
		latest: map[string]LatestEvent{},
	}
}

func (w *SnapshotWorker) Observe(event LatestEvent) {
	if event.EventTime.IsZero() {
		event.EventTime = w.now()
	}
	key, err := event.Key()
	if err != nil {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.latest == nil {
		w.latest = map[string]LatestEvent{}
	}
	w.latest[key] = event
}

func (w *SnapshotWorker) Flush(ctx context.Context) (int, error) {
	if w.Sink == nil {
		return 0, nil
	}

	w.mu.Lock()
	events := make([]LatestEvent, 0, len(w.latest))
	for _, event := range w.latest {
		events = append(events, event)
	}
	w.latest = map[string]LatestEvent{}
	w.mu.Unlock()

	if len(events) == 0 {
		return 0, nil
	}

	bucket := w.now().UTC().Truncate(w.bucket())
	if err := w.Sink.WriteRealtimeSnapshots(ctx, bucket, events); err != nil {
		w.mu.Lock()
		for _, event := range events {
			key, err := event.Key()
			if err == nil {
				w.latest[key] = event
			}
		}
		w.mu.Unlock()
		return 0, err
	}
	return len(events), nil
}

func (w *SnapshotWorker) bucket() time.Duration {
	if w.Bucket <= 0 {
		return time.Minute
	}
	return w.Bucket
}

func (w *SnapshotWorker) now() time.Time {
	if w.Now != nil {
		return w.Now().UTC()
	}
	return time.Now().UTC()
}
