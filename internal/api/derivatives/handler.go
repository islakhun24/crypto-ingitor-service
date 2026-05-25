package derivatives

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
)

type Handler struct {
	Repository *Repository
	Timeout    time.Duration
}

func Register(mux *http.ServeMux, repository *Repository, timeout time.Duration) {
	handler := Handler{Repository: repository, Timeout: timeout}

	mux.HandleFunc("GET /api/v1/derivatives/overview", handler.overview)
	mux.HandleFunc("GET /api/v1/derivatives/symbols", handler.symbols)
	mux.HandleFunc("GET /api/v1/derivatives/symbols/{symbol}", handler.symbolDetail)
	mux.HandleFunc("GET /api/v1/derivatives/symbols/{symbol}/market", handler.market)
	mux.HandleFunc("GET /api/v1/derivatives/symbols/{symbol}/klines", handler.klines)
	mux.HandleFunc("GET /api/v1/derivatives/symbols/{symbol}/open-interest", handler.openInterest)
	mux.HandleFunc("GET /api/v1/derivatives/symbols/{symbol}/funding", handler.funding)
	mux.HandleFunc("GET /api/v1/derivatives/symbols/{symbol}/long-short-ratio", handler.longShortRatio)
	mux.HandleFunc("GET /api/v1/derivatives/symbols/{symbol}/taker-flow", handler.takerFlow)
	mux.HandleFunc("GET /api/v1/derivatives/symbols/{symbol}/cvd", handler.cvd)
	mux.HandleFunc("GET /api/v1/derivatives/symbols/{symbol}/liquidations", handler.liquidations)
	mux.HandleFunc("GET /api/v1/derivatives/symbols/{symbol}/basis", handler.basis)
	mux.HandleFunc("GET /api/v1/derivatives/symbols/{symbol}/orderbook-imbalance", handler.orderbookImbalance)
	mux.HandleFunc("GET /api/v1/derivatives/symbols/{symbol}/exchange-divergence", handler.exchangeDivergence)
	mux.HandleFunc("GET /api/v1/derivatives/health/collectors", handler.collectorHealth)
	mux.HandleFunc("GET /api/v1/derivatives/health/exchanges", handler.exchangeHealth)
	mux.HandleFunc("GET /api/v1/derivatives/jobs", handler.jobs)
	mux.HandleFunc("GET /api/v1/derivatives/quality/issues", handler.qualityIssues)
	mux.HandleFunc("GET /api/v1/derivatives/quality/gaps", handler.dataGaps)
}

func (h Handler) overview(w http.ResponseWriter, r *http.Request) {
	h.withOptions(w, r, func(ctx context.Context, opts ListOptions) (any, error) {
		return h.Repository.ListOverview(ctx, opts)
	})
}

func (h Handler) symbols(w http.ResponseWriter, r *http.Request) {
	h.withOptions(w, r, func(ctx context.Context, opts ListOptions) (any, error) {
		return h.Repository.ListSymbols(ctx, opts)
	})
}

func (h Handler) symbolDetail(w http.ResponseWriter, r *http.Request) {
	h.withOptions(w, r, func(ctx context.Context, opts ListOptions) (any, error) {
		return h.Repository.GetSymbolDetail(ctx, r.PathValue("symbol"), opts)
	})
}

func (h Handler) market(w http.ResponseWriter, r *http.Request) {
	h.withSymbol(w, r, func(ctx context.Context, symbolID int64, opts ListOptions) (any, error) {
		return h.Repository.ListMarket(ctx, symbolID, opts)
	})
}

func (h Handler) klines(w http.ResponseWriter, r *http.Request) {
	h.withSymbol(w, r, func(ctx context.Context, symbolID int64, opts ListOptions) (any, error) {
		return h.Repository.ListKlinesPage(ctx, symbolID, opts)
	})
}

func (h Handler) openInterest(w http.ResponseWriter, r *http.Request) {
	h.withSymbol(w, r, func(ctx context.Context, symbolID int64, opts ListOptions) (any, error) {
		return h.Repository.ListOpenInterestPage(ctx, symbolID, opts)
	})
}

func (h Handler) funding(w http.ResponseWriter, r *http.Request) {
	h.withSymbol(w, r, func(ctx context.Context, symbolID int64, opts ListOptions) (any, error) {
		return h.Repository.ListFundingPage(ctx, symbolID, opts)
	})
}

func (h Handler) longShortRatio(w http.ResponseWriter, r *http.Request) {
	h.withSymbol(w, r, func(ctx context.Context, symbolID int64, opts ListOptions) (any, error) {
		return h.Repository.ListLongShortRatioPage(ctx, symbolID, opts)
	})
}

func (h Handler) takerFlow(w http.ResponseWriter, r *http.Request) {
	h.withSymbol(w, r, func(ctx context.Context, symbolID int64, opts ListOptions) (any, error) {
		return h.Repository.ListTakerFlowPage(ctx, symbolID, opts)
	})
}

func (h Handler) cvd(w http.ResponseWriter, r *http.Request) {
	h.withSymbol(w, r, func(ctx context.Context, symbolID int64, opts ListOptions) (any, error) {
		return h.Repository.ListCVDPage(ctx, symbolID, opts)
	})
}

func (h Handler) liquidations(w http.ResponseWriter, r *http.Request) {
	h.withSymbol(w, r, func(ctx context.Context, symbolID int64, opts ListOptions) (any, error) {
		return h.Repository.ListLiquidationsPage(ctx, symbolID, opts)
	})
}

func (h Handler) basis(w http.ResponseWriter, r *http.Request) {
	h.withSymbol(w, r, func(ctx context.Context, symbolID int64, opts ListOptions) (any, error) {
		return h.Repository.ListBasisPage(ctx, symbolID, opts)
	})
}

func (h Handler) orderbookImbalance(w http.ResponseWriter, r *http.Request) {
	h.withSymbol(w, r, func(ctx context.Context, symbolID int64, opts ListOptions) (any, error) {
		return h.Repository.ListOrderbookImbalancePage(ctx, symbolID, opts)
	})
}

func (h Handler) exchangeDivergence(w http.ResponseWriter, r *http.Request) {
	h.withSymbol(w, r, func(ctx context.Context, symbolID int64, opts ListOptions) (any, error) {
		return h.Repository.ListExchangeDivergencePage(ctx, symbolID, opts)
	})
}

func (h Handler) collectorHealth(w http.ResponseWriter, r *http.Request) {
	h.withOptions(w, r, func(ctx context.Context, opts ListOptions) (any, error) {
		return h.Repository.ListCollectorHealth(ctx, opts)
	})
}

func (h Handler) exchangeHealth(w http.ResponseWriter, r *http.Request) {
	h.withOptions(w, r, func(ctx context.Context, opts ListOptions) (any, error) {
		return h.Repository.ListExchangeHealth(ctx, opts)
	})
}

func (h Handler) jobs(w http.ResponseWriter, r *http.Request) {
	h.withOptions(w, r, func(ctx context.Context, opts ListOptions) (any, error) {
		return h.Repository.ListJobs(ctx, opts)
	})
}

func (h Handler) qualityIssues(w http.ResponseWriter, r *http.Request) {
	h.withOptions(w, r, func(ctx context.Context, opts ListOptions) (any, error) {
		return h.Repository.ListQualityIssues(ctx, opts)
	})
}

func (h Handler) dataGaps(w http.ResponseWriter, r *http.Request) {
	h.withOptions(w, r, func(ctx context.Context, opts ListOptions) (any, error) {
		return h.Repository.ListDataGaps(ctx, opts)
	})
}

func (h Handler) withSymbol(w http.ResponseWriter, r *http.Request, fn func(context.Context, int64, ListOptions) (any, error)) {
	h.withOptions(w, r, func(ctx context.Context, opts ListOptions) (any, error) {
		_, symbolID, err := h.Repository.ResolveSymbol(ctx, r.PathValue("symbol"))
		if err != nil {
			return nil, err
		}
		return fn(ctx, symbolID, opts)
	})
}

func (h Handler) withOptions(w http.ResponseWriter, r *http.Request, fn func(context.Context, ListOptions) (any, error)) {
	opts, err := ParseListOptions(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "bad_request", err.Error(), nil)
		return
	}

	ctx := r.Context()
	if h.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, h.Timeout)
		defer cancel()
	}

	body, err := fn(ctx, opts)
	if err != nil {
		status, code := errorStatus(err)
		writeAPIError(w, status, code, err.Error(), nil)
		return
	}
	writeAPIJSON(w, http.StatusOK, body)
}

func errorStatus(err error) (int, string) {
	if errors.Is(err, ErrNotFound) {
		return http.StatusNotFound, "not_found"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusGatewayTimeout, "timeout"
	}
	if strings.Contains(strings.ToLower(err.Error()), "not found") {
		return http.StatusNotFound, "not_found"
	}
	return http.StatusInternalServerError, "internal_error"
}

func writeAPIJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("write api response: %v", err)
	}
}

func writeAPIError(w http.ResponseWriter, status int, code string, message string, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}
	writeAPIJSON(w, status, APIError{Error: ErrorBody{Code: code, Message: message, Details: details}})
}
