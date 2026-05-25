package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"aggregator-services/internal/collectors/aggregate"
	"aggregator-services/internal/collectors/core"
	"aggregator-services/internal/config"
	"aggregator-services/internal/database"
	"aggregator-services/internal/endpoints"
	"aggregator-services/internal/exchanges/all"
	"aggregator-services/internal/hardening"
	"aggregator-services/internal/logger"
	"aggregator-services/internal/ratelimit"
	"aggregator-services/internal/repositories"
	"aggregator-services/internal/scheduler"
	"aggregator-services/internal/symbols"
)

func main() {
	log := logger.New("collector-service")

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

	symbolRepo := symbols.NewRepository(db)
	endpointRepo := endpoints.NewRepository(db)
	schedulerRepo := scheduler.NewRepository(db, symbolRepo)

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        50,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  false,
			DisableKeepAlives:   false,
			MaxIdleConnsPerHost: 10,
		},
	}
	registry := all.NewRegistry(client)

	limiter := ratelimit.NewLimiter()
	for _, exchange := range cfg.SupportedExchanges {
		if err := limiter.Configure(exchange, ratelimit.Budget{
			RequestsPerSecond: float64(cfg.RateLimit.RequestsPerSecond),
			Burst:             cfg.RateLimit.Burst,
			JitterMin:         0,
			JitterMax:         100 * time.Millisecond,
		}); err != nil {
			log.Error("failed to configure rate limiter", err, logger.Fields{"exchange": exchange})
		}
	}

	breaker := ratelimit.NewCircuitBreaker(ratelimit.CircuitSettings{
		FailureThreshold:  cfg.RateLimit.CircuitBreakerFailureLimit,
		Cooldown:          cfg.RateLimit.CircuitBreakerCooldown,
		HalfOpenMaxPasses: 1,
	})

	healthRepo := repositories.NewCollectorHealthRepository(db)
	requestLogRepo := repositories.NewRequestLogRepository(db)
	dataQualityRepo := repositories.NewDataQualityRepository(db)
	marketSnapshotRepo := repositories.NewMarketSnapshotRepository(db)
	klineRepo := repositories.NewKlineRepository(db)
	openInterestRepo := repositories.NewOpenInterestRepository(db)
	fundingRepo := repositories.NewFundingRepository(db)
	advancedRepo := repositories.NewAdvancedRepository(db, cfg.Liquidation.MinUSD)

	instanceID := os.Getenv("HOSTNAME")
	if instanceID == "" {
		instanceID = "default"
	}

	writer := core.RepositoryWriter{
		MarketSnapshot: marketSnapshotRepo,
		Kline:          klineRepo,
		OpenInterest:   openInterestRepo,
		Funding:        fundingRepo,
		Advanced:       advancedRepo,
		Quality:        dataQualityRepo,
		Validation: hardening.ValidationConfig{
			MaxFutureSkew: cfg.Hardening.MaxFutureSkew,
			FundingMin:    cfg.Hardening.FundingMin,
			FundingMax:    cfg.Hardening.FundingMax,
		},
	}

	collector := core.Collector{
		Endpoints: endpointRepo,
		Symbols:   symbolRepo,
		Adapters:  registry,
		Writer:    writer,
		Logs:      requestLogRepo,
		Health:    healthRepo,
		Service:   "collector-service",
		Instance:  instanceID,
	}

	aggExecutor := aggregate.Executor{
		Store:    repositories.NewAggregateRepository(db),
		Health:   healthRepo,
		Service:  "collector-service",
		Instance: instanceID,
	}

	router := core.DataTypeExecutor{
		"*":                   &collector,
		"aggregated_snapshot": &aggExecutor,
	}

	worker := scheduler.Worker{
		Jobs:      schedulerRepo,
		Executor:  router,
		Limiter:   limiter,
		Breaker:   breaker,
		BatchSize: cfg.WorkerConcurrency,
		Retry: scheduler.RetryPolicy{
			Base:      cfg.Hardening.BackoffBase,
			Max:       cfg.Hardening.BackoffMax,
			JitterMax: cfg.Hardening.BackoffJitterMax,
		},
	}

	processed, err := worker.RunOnce(ctx)
	if err != nil {
		log.Fatal("worker failed", err, logger.Fields{"processed": processed})
	}

	log.Info("collector completed", logger.Fields{"processed": processed})

	os.Exit(0)
}
