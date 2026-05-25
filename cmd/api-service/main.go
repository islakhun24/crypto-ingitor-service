package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"aggregator-services/internal/api/derivatives"
	"aggregator-services/internal/config"
	"aggregator-services/internal/database"
	"aggregator-services/internal/endpoints"
	"aggregator-services/internal/logger"
	"aggregator-services/internal/observability"
	"aggregator-services/internal/realtime"
	"aggregator-services/internal/symbols"
)

func main() {
	log := logger.New("api-service")

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatal("invalid config", err, nil)
	}

	ctx := context.Background()

	pg, err := database.OpenPostgres(ctx, cfg.Postgres.DSN(), database.Options{
		MaxOpenConns:    cfg.Postgres.MaxOpenConns,
		MaxIdleConns:    cfg.Postgres.MaxIdleConns,
		ConnMaxLifetime: cfg.Postgres.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.Postgres.ConnMaxIdleTime,
	})
	if err != nil {
		log.Fatal("failed to open postgres", err, nil)
	}
	defer pg.Close()

	db := pg.DB()

	var store realtime.Store
	redisStore := realtime.NewRedisStore(realtime.RedisOptions{
		Addr:           cfg.Redis.Addr(),
		Password:       cfg.Redis.Password,
		DB:             cfg.Redis.DB,
		MaxMemory:      cfg.Redis.MaxMemory,
		EvictionPolicy: cfg.Redis.EvictionPolicy,
		Timeout:        2 * time.Second,
	})
	if err := redisStore.ApplyMemoryPolicy(ctx); err != nil {
		log.Info("redis not available, using memory fallback", logger.Fields{"error": err.Error()})
		store = realtime.NewMemoryStore()
	} else {
		store = realtime.NewFallbackStore(redisStore, realtime.NewMemoryStore())
	}

	symbolRepo := symbols.NewRepository(db)
	endpointRepo := endpoints.NewRepository(db)
	derivativeRepo := derivatives.NewRepository(db)
	metricsRepo := observability.NewRepository(db)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := pg.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy", "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics, err := metricsRepo.Prometheus(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "# error: %v\n", err)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprint(w, metrics)
	})

	registerSymbolHandlers(mux, symbolRepo, cfg.SupportedExchanges)
	registerEndpointHandlers(mux, endpointRepo)
	derivatives.Register(mux, derivativeRepo, 30*time.Second)
	derivatives.RegisterRealtime(mux, store, 5*time.Second)

	addr := cfg.HTTPAddr
	if addr == "" {
		addr = ":8080"
	}

	log.Info("starting api service", logger.Fields{"addr": addr, "env": cfg.AppEnv})
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatal("server failed", err, nil)
	}
}

func registerSymbolHandlers(mux *http.ServeMux, repo *symbols.Repository, supportedExchanges []string) {
	mux.HandleFunc("GET /symbols", func(w http.ResponseWriter, r *http.Request) {
		active, err := repo.ListActiveSymbols(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, active)
	})

	mux.HandleFunc("GET /symbols/{id}", func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		var id int64
		if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil || id <= 0 {
			writeError(w, http.StatusBadRequest, "invalid symbol id")
			return
		}
		symbol, err := repo.GetSymbolByID(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, symbol)
	})

	mux.HandleFunc("GET /symbols/top", func(w http.ResponseWriter, r *http.Request) {
		limit := 50
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if n, err := fmt.Sscanf(raw, "%d", &limit); err != nil || n != 1 || limit < 1 {
				limit = 50
			}
		}
		top, err := repo.ListTopSymbols(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, top)
	})

	mux.HandleFunc("GET /symbols/watchlist", func(w http.ResponseWriter, r *http.Request) {
		watchlist, err := repo.ListWatchlistSymbols(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, watchlist)
	})

	mux.HandleFunc("GET /symbols/exchange/{exchange}", func(w http.ResponseWriter, r *http.Request) {
		exchange := r.PathValue("exchange")
		result, err := repo.ListSymbolsByExchange(r.Context(), exchange)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("GET /symbols/markets", func(w http.ResponseWriter, r *http.Request) {
		markets, err := repo.ListActiveSymbolMarkets(r.Context(), supportedExchanges)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, markets)
	})
}

func registerEndpointHandlers(mux *http.ServeMux, repo *endpoints.Repository) {
	mux.HandleFunc("GET /endpoints/{exchange}/{data_type}", func(w http.ResponseWriter, r *http.Request) {
		exchange := r.PathValue("exchange")
		dataType := r.PathValue("data_type")
		result, err := repo.ListActiveByExchangeDataType(r.Context(), exchange, dataType)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("GET /endpoints/{exchange}/{market_type}/{data_type}/{name}", func(w http.ResponseWriter, r *http.Request) {
		exchange := r.PathValue("exchange")
		marketType := r.PathValue("market_type")
		dataType := r.PathValue("data_type")
		name := r.PathValue("name")
		result, err := repo.ResolveActive(r.Context(), exchange, marketType, dataType, name)
		if err != nil {
			if errors.Is(err, endpoints.ErrEndpointUnavailable) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": http.StatusText(status), "message": message}})
}
