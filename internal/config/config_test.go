package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadReadsPostgresEnvironment(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("APP_PORT", "9090")
	t.Setenv("POSTGRES_HOST", "exchange-normalizer-postgres")
	t.Setenv("POSTGRES_PORT", "5432")
	t.Setenv("POSTGRES_USER", "postgres")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_DB", "crypto_ultimate")
	t.Setenv("POSTGRES_SSLMODE", "disable")
	t.Setenv("POSTGRES_MAX_OPEN_CONNS", "12")
	t.Setenv("POSTGRES_MAX_IDLE_CONNS", "6")
	t.Setenv("POSTGRES_CONN_MAX_LIFETIME_SECONDS", "60")
	t.Setenv("REDIS_HOST", "redis")
	t.Setenv("REDIS_PORT", "6379")
	t.Setenv("REDIS_DB", "2")
	t.Setenv("REDIS_MAX_MEMORY", "512mb")
	t.Setenv("REDIS_EVICTION_POLICY", "allkeys-lru")
	t.Setenv("REALTIME_LATEST_TTL_SECONDS", "90")
	t.Setenv("REALTIME_STREAM_STALE_SECONDS", "20")
	t.Setenv("REALTIME_HEARTBEAT_SECONDS", "10")
	t.Setenv("REALTIME_RECONNECT_MIN_SECONDS", "2")
	t.Setenv("REALTIME_RECONNECT_MAX_SECONDS", "30")
	t.Setenv("REALTIME_SNAPSHOT_BUCKET_SECONDS", "60")
	t.Setenv("COLLECTION_MODE", "tiered")
	t.Setenv("WORKER_CONCURRENCY", "8")
	t.Setenv("RATE_LIMIT_REQUESTS_PER_SECOND", "3")
	t.Setenv("RATE_LIMIT_BURST", "7")
	t.Setenv("LIQUIDATION_MIN_USD", "2500")
	t.Setenv("ANALYTICS_MAX_SNAPSHOT_AGE_SECONDS", "120")
	t.Setenv("RETENTION_MAX_ROWS_PER_RUN", "12345")
	t.Setenv("RETENTION_TABLE_TIMEOUT_SECONDS", "45")
	t.Setenv("RETENTION_DISK_PRESSURE_CRITICAL", "true")
	t.Setenv("DATA_MAX_FUTURE_SKEW_SECONDS", "30")
	t.Setenv("FUNDING_SANITY_MIN", "-0.02")
	t.Setenv("FUNDING_SANITY_MAX", "0.02")
	t.Setenv("RECOVERY_RUNNING_JOB_TIMEOUT_SECONDS", "300")
	t.Setenv("BACKFILL_MAX_JOBS_PER_RUN", "250")
	t.Setenv("RETRY_BACKOFF_BASE_SECONDS", "5")
	t.Setenv("RETRY_BACKOFF_MAX_SECONDS", "60")
	t.Setenv("RETRY_BACKOFF_JITTER_MAX_SECONDS", "3")
	t.Setenv("SUPPORTED_EXCHANGES", " Binance,OKX, mexc ")

	cfg := Load()

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if got := strings.Join(cfg.SupportedExchanges, ","); got != "binance,okx,mexc" {
		t.Fatalf("SupportedExchanges = %q", got)
	}
	if cfg.AppEnv != "test" {
		t.Fatalf("AppEnv = %q", cfg.AppEnv)
	}
	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.Redis.DB != 2 {
		t.Fatalf("Redis.DB = %d", cfg.Redis.DB)
	}
	if cfg.Redis.MaxMemory != "512mb" || cfg.Redis.EvictionPolicy != "allkeys-lru" {
		t.Fatalf("Redis memory policy = %q/%q", cfg.Redis.MaxMemory, cfg.Redis.EvictionPolicy)
	}
	if cfg.Redis.Addr() != "redis:6379" {
		t.Fatalf("Redis.Addr() = %q", cfg.Redis.Addr())
	}
	if cfg.Realtime.LatestTTL != 90*time.Second || cfg.Realtime.StreamStaleAfter != 20*time.Second {
		t.Fatalf("Realtime freshness = %s/%s", cfg.Realtime.LatestTTL, cfg.Realtime.StreamStaleAfter)
	}
	if cfg.Realtime.HeartbeatInterval != 10*time.Second || cfg.Realtime.ReconnectMin != 2*time.Second || cfg.Realtime.ReconnectMax != 30*time.Second {
		t.Fatalf("Realtime heartbeat/reconnect = %s/%s/%s", cfg.Realtime.HeartbeatInterval, cfg.Realtime.ReconnectMin, cfg.Realtime.ReconnectMax)
	}
	if cfg.Realtime.SnapshotBucket != time.Minute {
		t.Fatalf("Realtime.SnapshotBucket = %s", cfg.Realtime.SnapshotBucket)
	}
	if cfg.WorkerConcurrency != 8 {
		t.Fatalf("WorkerConcurrency = %d", cfg.WorkerConcurrency)
	}
	if cfg.Postgres.MaxOpenConns != 12 || cfg.Postgres.MaxIdleConns != 6 {
		t.Fatalf("Postgres pool = %d/%d", cfg.Postgres.MaxOpenConns, cfg.Postgres.MaxIdleConns)
	}
	if cfg.Postgres.ConnMaxLifetime != time.Minute {
		t.Fatalf("Postgres.ConnMaxLifetime = %s", cfg.Postgres.ConnMaxLifetime)
	}
	if cfg.RateLimit.RequestsPerSecond != 3 || cfg.RateLimit.Burst != 7 {
		t.Fatalf("RateLimit = %+v", cfg.RateLimit)
	}
	if cfg.Liquidation.MinUSD != 2500 {
		t.Fatalf("Liquidation.MinUSD = %f", cfg.Liquidation.MinUSD)
	}
	if cfg.Analytics.MaxSnapshotAge != 2*time.Minute {
		t.Fatalf("Analytics.MaxSnapshotAge = %s", cfg.Analytics.MaxSnapshotAge)
	}
	if cfg.Retention.MaxRowsPerRun != 12345 {
		t.Fatalf("Retention.MaxRowsPerRun = %d", cfg.Retention.MaxRowsPerRun)
	}
	if cfg.Retention.TableTimeout != 45*time.Second {
		t.Fatalf("Retention.TableTimeout = %s", cfg.Retention.TableTimeout)
	}
	if !cfg.Retention.DiskPressureCritical {
		t.Fatal("Retention.DiskPressureCritical = false, want true")
	}
	if cfg.Hardening.MaxFutureSkew != 30*time.Second {
		t.Fatalf("Hardening.MaxFutureSkew = %s", cfg.Hardening.MaxFutureSkew)
	}
	if cfg.Hardening.FundingMin != -0.02 || cfg.Hardening.FundingMax != 0.02 {
		t.Fatalf("Hardening funding bounds = %f/%f", cfg.Hardening.FundingMin, cfg.Hardening.FundingMax)
	}
	if cfg.Hardening.RestartJobTimeout != 5*time.Minute {
		t.Fatalf("Hardening.RestartJobTimeout = %s", cfg.Hardening.RestartJobTimeout)
	}
	if cfg.Hardening.BackfillMaxJobs != 250 {
		t.Fatalf("Hardening.BackfillMaxJobs = %d", cfg.Hardening.BackfillMaxJobs)
	}
	if cfg.Hardening.BackoffBase != 5*time.Second || cfg.Hardening.BackoffMax != time.Minute || cfg.Hardening.BackoffJitterMax != 3*time.Second {
		t.Fatalf("Hardening backoff = %s/%s/%s", cfg.Hardening.BackoffBase, cfg.Hardening.BackoffMax, cfg.Hardening.BackoffJitterMax)
	}

	dsn := cfg.Postgres.DSN()
	if !strings.Contains(dsn, "postgres://postgres:secret@exchange-normalizer-postgres:5432/crypto_ultimate") {
		t.Fatalf("DSN() = %q", dsn)
	}
	if !strings.Contains(dsn, "sslmode=disable") {
		t.Fatalf("DSN() missing sslmode: %q", dsn)
	}
}

func TestValidateReportsMissingPostgresEnvironment(t *testing.T) {
	cfg := Config{}

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil")
	}
}
