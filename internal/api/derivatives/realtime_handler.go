package derivatives

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"aggregator-services/internal/realtime"
)

type RealtimeHandler struct {
	Store   realtime.Store
	Timeout time.Duration
}

func RegisterRealtime(mux *http.ServeMux, store realtime.Store, timeout time.Duration) {
	handler := RealtimeHandler{Store: store, Timeout: timeout}
	mux.HandleFunc("GET /api/v1/derivatives/realtime/latest/{kind}/{exchange}/{source_symbol}", handler.latestExchangeSymbol)
	mux.HandleFunc("GET /api/v1/derivatives/realtime/aggregate/{symbol_id}", handler.latestAggregate)
	mux.HandleFunc("GET /api/v1/derivatives/realtime/ws-state/{exchange}/{stream}", handler.wsState)
}

func (h RealtimeHandler) latestExchangeSymbol(w http.ResponseWriter, r *http.Request) {
	h.withRealtime(w, r, func(ctx context.Context) (any, error) {
		kind := r.PathValue("kind")
		key, err := realtime.LatestKey(kind, r.PathValue("exchange"), r.PathValue("source_symbol"), 0)
		if err != nil {
			return nil, errBadRequest(err.Error())
		}
		return h.readLatest(ctx, key)
	})
}

func (h RealtimeHandler) latestAggregate(w http.ResponseWriter, r *http.Request) {
	h.withRealtime(w, r, func(ctx context.Context) (any, error) {
		symbolID, err := strconv.ParseInt(r.PathValue("symbol_id"), 10, 64)
		if err != nil || symbolID <= 0 {
			return nil, errBadRequest("symbol_id must be a positive integer")
		}
		return h.readLatest(ctx, realtime.LatestAggregateKey(symbolID))
	})
}

func (h RealtimeHandler) wsState(w http.ResponseWriter, r *http.Request) {
	h.withRealtime(w, r, func(ctx context.Context) (any, error) {
		var state realtime.StreamState
		ok, err := realtime.GetJSON(ctx, h.Store, realtime.WSStateKey(r.PathValue("exchange"), r.PathValue("stream")), &state)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrNotFound
		}
		return state, nil
	})
}

func (h RealtimeHandler) readLatest(ctx context.Context, key string) (any, error) {
	var event realtime.LatestEvent
	ok, err := realtime.GetJSON(ctx, h.Store, key, &event)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	return event, nil
}

func (h RealtimeHandler) withRealtime(w http.ResponseWriter, r *http.Request, fn func(context.Context) (any, error)) {
	if h.Store == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "unavailable", "realtime store is not configured", nil)
		return
	}

	ctx := r.Context()
	if h.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, h.Timeout)
		defer cancel()
	}

	body, err := fn(ctx)
	if err != nil {
		if _, ok := err.(badRequestError); ok {
			writeAPIError(w, http.StatusBadRequest, "bad_request", err.Error(), nil)
			return
		}
		status, code := errorStatus(err)
		writeAPIError(w, status, code, err.Error(), nil)
		return
	}
	writeAPIJSON(w, http.StatusOK, body)
}

type badRequestError string

func (e badRequestError) Error() string {
	return string(e)
}

func errBadRequest(message string) error {
	return badRequestError(message)
}
