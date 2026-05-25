package main

import (
	"context"
	"os"

	"aggregator-services/internal/config"
	"aggregator-services/internal/database"
	"aggregator-services/internal/logger"
	"aggregator-services/internal/retention"
)

func main() {
	log := logger.New("retention-service")

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
	store := retention.NewStore(db)

	engine := retention.Engine{
		Store: store,
		Rollups: retention.RollupEngine{
			Store: store,
		},
		Options: retention.EngineOptions{
			MaxRowsPerRun:        cfg.Retention.MaxRowsPerRun,
			TableTimeout:         cfg.Retention.TableTimeout,
			DiskPressureCritical: cfg.Retention.DiskPressureCritical,
		},
	}

	summary, err := engine.RunOnce(ctx)
	if err != nil {
		log.Error("retention engine finished with error", err, logger.Fields{
			"results": len(summary.Results),
			"started": summary.StartedAt,
		})
		os.Exit(1)
	}

	fields := logger.Fields{
		"results":  len(summary.Results),
		"started":  summary.StartedAt,
		"finished": summary.FinishedAt,
	}
	if summary.Metrics != nil {
		fields["total_rows_matched"] = summary.Metrics.TotalRowsMatched
		fields["total_rows_deleted"] = summary.Metrics.TotalRowsDeleted
		fields["total_partitions_dropped"] = summary.Metrics.TotalPartitionsDropped
	}
	log.Info("retention completed", fields)

	os.Exit(0)
}
