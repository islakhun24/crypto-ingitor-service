package main

import (
	"context"
	"os"

	"aggregator-services/internal/config"
	"aggregator-services/internal/database"
	"aggregator-services/internal/endpoints"
	"aggregator-services/internal/logger"
	"aggregator-services/internal/scheduler"
	"aggregator-services/internal/symbols"
)

func main() {
	log := logger.New("scheduler-service")

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

	result, err := schedulerRepo.RecoverInterrupted(ctx, cfg.Hardening.RestartJobTimeout)
	if err != nil {
		log.Error("failed to recover interrupted jobs", err, nil)
	} else if result.RunningJobsReset > 0 || result.CollectionRunsInterrupted > 0 {
		log.Info("recovered interrupted work", logger.Fields{
			"running_jobs_reset":            result.RunningJobsReset,
			"collection_runs_interrupted": result.CollectionRunsInterrupted,
		})
	}

	planner := scheduler.Planner{
		Policies:  schedulerRepo,
		Endpoints: endpointRepo,
		Symbols:   schedulerRepo,
		Jobs:      schedulerRepo,
	}

	planResult, err := planner.Run(ctx)
	if err != nil {
		log.Fatal("planner failed", err, nil)
	}

	fields := logger.Fields{
		"attempted": planResult.AttemptedJobs,
		"inserted":  planResult.InsertedJobs,
		"skipped":   len(planResult.Skipped),
	}
	for key, count := range planResult.Skipped {
		fields["skip_"+key] = count
	}
	log.Info("scheduler completed", fields)

	os.Exit(0)
}
