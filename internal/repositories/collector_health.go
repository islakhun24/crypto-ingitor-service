package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type CollectorHealth struct {
	ServiceName   string
	InstanceID    string
	Exchange      string
	DataType      string
	Status        string
	HeartbeatAt   time.Time
	LastSuccessAt time.Time
	LastErrorAt   time.Time
	ErrorMessage  string
	Metrics       json.RawMessage
}

type CollectorHealthRepository struct {
	db *sql.DB
}

func NewCollectorHealthRepository(db *sql.DB) *CollectorHealthRepository {
	return &CollectorHealthRepository{db: db}
}

func (r *CollectorHealthRepository) Upsert(ctx context.Context, health CollectorHealth) error {
	if health.HeartbeatAt.IsZero() {
		health.HeartbeatAt = time.Now().UTC()
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO collector_health (
		    service_name, instance_id, exchange, data_type, status,
		    heartbeat_at, last_success_at, last_error_at, error_message, metrics
		)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, $6, $7, $8, NULLIF($9, ''), $10)
		ON CONFLICT DO NOTHING
	`,
		health.ServiceName,
		health.InstanceID,
		health.Exchange,
		health.DataType,
		health.Status,
		health.HeartbeatAt,
		nullableTime(health.LastSuccessAt),
		nullableTime(health.LastErrorAt),
		health.ErrorMessage,
		ensureJSON(health.Metrics),
	)
	if err != nil {
		return fmt.Errorf("insert collector health: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
		UPDATE collector_health
		SET status = $5,
		    heartbeat_at = $6,
		    last_success_at = COALESCE($7, last_success_at),
		    last_error_at = COALESCE($8, last_error_at),
		    error_message = NULLIF($9, ''),
		    metrics = $10,
		    updated_at = now()
		WHERE service_name = $1
		  AND instance_id = $2
		  AND COALESCE(exchange, '') = $3
		  AND COALESCE(data_type, '') = $4
	`,
		health.ServiceName,
		health.InstanceID,
		health.Exchange,
		health.DataType,
		health.Status,
		health.HeartbeatAt,
		nullableTime(health.LastSuccessAt),
		nullableTime(health.LastErrorAt),
		health.ErrorMessage,
		ensureJSON(health.Metrics),
	)
	if err != nil {
		return fmt.Errorf("update collector health: %w", err)
	}

	return nil
}
