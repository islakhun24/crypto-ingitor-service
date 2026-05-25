package retention

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lib/pq"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) ListEnabledPolicies(ctx context.Context) ([]Policy, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, table_name, time_column, interval_filter_column,
		       interval_filter_value, retention_days, chunk_size, enabled,
		       dry_run, rollup_before_delete, rollup_target_table, priority,
		       max_rows_per_run, timeout_seconds, partition_strategy,
		       min_retention_days, metadata
		FROM data_retention_policies
		WHERE enabled = true
		ORDER BY priority ASC, table_name ASC, COALESCE(interval_filter_value, '') ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query retention policies: %w", err)
	}
	defer rows.Close()

	var policies []Policy
	for rows.Next() {
		policy, err := scanRetentionPolicy(rows)
		if err != nil {
			return nil, err
		}
		policies = append(policies, policy)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate retention policies: %w", err)
	}

	return policies, nil
}

func (s *Store) CountEligible(ctx context.Context, policy Policy, cutoff time.Time) (int64, error) {
	query, err := BuildCountSQL(policy)
	if err != nil {
		return 0, err
	}

	args := retentionArgs(policy, cutoff)
	var count int64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count eligible rows for %s: %w", policy.TableName, err)
	}

	return count, nil
}

func (s *Store) DeleteChunk(ctx context.Context, policy Policy, cutoff time.Time, chunkSize int) (int64, error) {
	query, err := BuildChunkDeleteSQL(policy)
	if err != nil {
		return 0, err
	}

	args := retentionArgs(policy, cutoff)
	args = append(args, chunkSize)
	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("delete chunk for %s: %w", policy.TableName, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete rows affected for %s: %w", policy.TableName, err)
	}

	return rows, nil
}

func (s *Store) ListEligiblePartitions(ctx context.Context, policy Policy, cutoff time.Time) ([]Partition, error) {
	if _, err := ValidatePolicy(policy); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT child_ns.nspname, child.relname
		FROM pg_inherits
		JOIN pg_class parent ON pg_inherits.inhparent = parent.oid
		JOIN pg_namespace parent_ns ON parent.relnamespace = parent_ns.oid
		JOIN pg_class child ON pg_inherits.inhrelid = child.oid
		JOIN pg_namespace child_ns ON child.relnamespace = child_ns.oid
		WHERE parent.relname = $1
		ORDER BY child.relname ASC
	`, policy.TableName)
	if err != nil {
		return nil, fmt.Errorf("query partitions for %s: %w", policy.TableName, err)
	}
	defer rows.Close()

	var partitions []Partition
	for rows.Next() {
		var schemaName, tableName string
		if err := rows.Scan(&schemaName, &tableName); err != nil {
			return nil, fmt.Errorf("scan partition for %s: %w", policy.TableName, err)
		}
		start, end, ok := ParsePartitionRange(policy.TableName, tableName)
		if !ok || end.After(cutoff) {
			continue
		}
		partitions = append(partitions, Partition{
			SchemaName: schemaName,
			TableName:  tableName,
			RangeStart: start,
			RangeEnd:   end,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate partitions for %s: %w", policy.TableName, err)
	}

	return partitions, nil
}

func (s *Store) DropPartition(ctx context.Context, partition Partition) error {
	query := fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", pq.QuoteIdentifier(partition.SchemaName), pq.QuoteIdentifier(partition.TableName))
	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("drop partition %s: %w", qualifiedPartitionName(partition), err)
	}
	return nil
}

func (s *Store) StartCleanupRun(ctx context.Context, runKey string, policy Policy, cutoff time.Time, dryRun bool, metadata json.RawMessage) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO data_cleanup_runs (
		    policy_id, run_key, table_name, status, dry_run,
		    cutoff_time, rows_matched, rows_deleted, metadata
		)
		VALUES ($1, $2, $3, 'running', $4, $5, 0, 0, $6)
		ON CONFLICT (run_key) DO UPDATE SET
		    status = 'running',
		    dry_run = EXCLUDED.dry_run,
		    cutoff_time = EXCLUDED.cutoff_time,
		    started_at = now(),
		    finished_at = NULL,
		    error_message = NULL,
		    metadata = EXCLUDED.metadata
	`, policy.ID, runKey, policy.TableName, dryRun, cutoff, ensureJSON(metadata))
	if err != nil {
		return fmt.Errorf("start cleanup run: %w", err)
	}

	return nil
}

func (s *Store) FinishCleanupRun(ctx context.Context, runKey string, status string, rowsMatched int64, rowsDeleted int64, errorMessage string, metadata json.RawMessage) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE data_cleanup_runs
		SET status = $2,
		    finished_at = now(),
		    rows_matched = $3,
		    rows_deleted = $4,
		    error_message = NULLIF($5, ''),
		    metadata = $6
		WHERE run_key = $1
	`, runKey, status, rowsMatched, rowsDeleted, errorMessage, ensureJSON(metadata))
	if err != nil {
		return fmt.Errorf("finish cleanup run: %w", err)
	}

	return nil
}

func scanRetentionPolicy(rows *sql.Rows) (Policy, error) {
	var (
		policy            Policy
		intervalColumn    sql.NullString
		intervalValue     sql.NullString
		rollupTargetTable sql.NullString
		metadata          json.RawMessage
	)

	if err := rows.Scan(
		&policy.ID,
		&policy.TableName,
		&policy.TimeColumn,
		&intervalColumn,
		&intervalValue,
		&policy.RetentionDays,
		&policy.ChunkSize,
		&policy.Enabled,
		&policy.DryRun,
		&policy.RollupBeforeDelete,
		&rollupTargetTable,
		&policy.Priority,
		&policy.MaxRowsPerRun,
		&policy.TimeoutSeconds,
		&policy.PartitionStrategy,
		&policy.MinRetentionDays,
		&metadata,
	); err != nil {
		return Policy{}, fmt.Errorf("scan retention policy: %w", err)
	}

	policy.IntervalColumn = intervalColumn.String
	policy.IntervalValue = intervalValue.String
	policy.RollupTargetTable = rollupTargetTable.String
	policy.Metadata = ensureJSON(metadata)

	return policy, nil
}

func retentionArgs(policy Policy, cutoff time.Time) []any {
	args := []any{cutoff}
	if policy.IntervalColumn != "" {
		args = append(args, policy.IntervalValue)
	}
	return args
}

func ensureJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}
