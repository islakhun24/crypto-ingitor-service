package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv             string
	AppPort            string
	HTTPAddr           string
	Postgres           PostgresConfig
	Redis              RedisConfig
	Realtime           RealtimeConfig
	CollectionMode     string
	WorkerConcurrency  int
	RateLimit          RateLimitConfig
	Liquidation        LiquidationConfig
	Analytics          AnalyticsConfig
	Retention          RetentionConfig
	Hardening          HardeningConfig
	SupportedExchanges []string
}

type PostgresConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

type RedisConfig struct {
	Host           string
	Port           string
	Password       string
	DB             int
	MaxMemory      string
	EvictionPolicy string
}

type RealtimeConfig struct {
	LatestTTL         time.Duration
	StreamStaleAfter  time.Duration
	HeartbeatInterval time.Duration
	ReconnectMin      time.Duration
	ReconnectMax      time.Duration
	SnapshotBucket    time.Duration
}

type RateLimitConfig struct {
	RequestsPerSecond          int
	Burst                      int
	Timeout                    time.Duration
	CircuitBreakerFailureLimit int
	CircuitBreakerCooldown     time.Duration
}

type LiquidationConfig struct {
	MinUSD float64
}

type AnalyticsConfig struct {
	MaxSnapshotAge time.Duration
}

type RetentionConfig struct {
	MaxRowsPerRun        int
	TableTimeout         time.Duration
	DiskPressureCritical bool
}

type HardeningConfig struct {
	MaxFutureSkew     time.Duration
	FundingMin        float64
	FundingMax        float64
	RestartJobTimeout time.Duration
	RealtimeLagPause  time.Duration
	BackfillMaxJobs   int
	BackoffBase       time.Duration
	BackoffMax        time.Duration
	BackoffJitterMax  time.Duration
}

func Load() Config {
	appPort := getenv("APP_PORT", "8080")

	return Config{
		AppEnv:            getenv("APP_ENV", "local"),
		AppPort:           appPort,
		HTTPAddr:          getenv("HTTP_ADDR", ":"+appPort),
		CollectionMode:    getenv("COLLECTION_MODE", "tiered"),
		WorkerConcurrency: getenvInt("WORKER_CONCURRENCY", 4),
		Postgres: PostgresConfig{
			Host:            os.Getenv("POSTGRES_HOST"),
			Port:            os.Getenv("POSTGRES_PORT"),
			User:            os.Getenv("POSTGRES_USER"),
			Password:        os.Getenv("POSTGRES_PASSWORD"),
			Database:        os.Getenv("POSTGRES_DB"),
			SSLMode:         os.Getenv("POSTGRES_SSLMODE"),
			MaxOpenConns:    getenvInt("POSTGRES_MAX_OPEN_CONNS", 20),
			MaxIdleConns:    getenvInt("POSTGRES_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getenvDurationSeconds("POSTGRES_CONN_MAX_LIFETIME_SECONDS", 1800),
			ConnMaxIdleTime: getenvDurationSeconds("POSTGRES_CONN_MAX_IDLE_SECONDS", 300),
		},
		Redis: RedisConfig{
			Host:           getenv("REDIS_HOST", "redis"),
			Port:           getenv("REDIS_PORT", "6379"),
			Password:       os.Getenv("REDIS_PASSWORD"),
			DB:             getenvInt("REDIS_DB", 0),
			MaxMemory:      getenv("REDIS_MAX_MEMORY", ""),
			EvictionPolicy: getenv("REDIS_EVICTION_POLICY", "allkeys-lru"),
		},
		Realtime: RealtimeConfig{
			LatestTTL:         getenvDurationSeconds("REALTIME_LATEST_TTL_SECONDS", 600),
			StreamStaleAfter:  getenvDurationSeconds("REALTIME_STREAM_STALE_SECONDS", 30),
			HeartbeatInterval: getenvDurationSeconds("REALTIME_HEARTBEAT_SECONDS", 15),
			ReconnectMin:      getenvDurationSeconds("REALTIME_RECONNECT_MIN_SECONDS", 1),
			ReconnectMax:      getenvDurationSeconds("REALTIME_RECONNECT_MAX_SECONDS", 60),
			SnapshotBucket:    getenvDurationSeconds("REALTIME_SNAPSHOT_BUCKET_SECONDS", 60),
		},
		RateLimit: RateLimitConfig{
			RequestsPerSecond:          getenvInt("RATE_LIMIT_REQUESTS_PER_SECOND", 5),
			Burst:                      getenvInt("RATE_LIMIT_BURST", 10),
			Timeout:                    getenvDurationSeconds("RATE_LIMIT_TIMEOUT_SECONDS", 10),
			CircuitBreakerFailureLimit: getenvInt("CIRCUIT_BREAKER_FAILURE_LIMIT", 5),
			CircuitBreakerCooldown:     getenvDurationSeconds("CIRCUIT_BREAKER_COOLDOWN_SECONDS", 30),
		},
		Liquidation: LiquidationConfig{
			MinUSD: getenvFloat("LIQUIDATION_MIN_USD", 1000),
		},
		Analytics: AnalyticsConfig{
			MaxSnapshotAge: getenvDurationSeconds("ANALYTICS_MAX_SNAPSHOT_AGE_SECONDS", 600),
		},
		Retention: RetentionConfig{
			MaxRowsPerRun:        getenvInt("RETENTION_MAX_ROWS_PER_RUN", 50000),
			TableTimeout:         getenvDurationSeconds("RETENTION_TABLE_TIMEOUT_SECONDS", 120),
			DiskPressureCritical: getenvBool("RETENTION_DISK_PRESSURE_CRITICAL", false),
		},
		Hardening: HardeningConfig{
			MaxFutureSkew:     getenvDurationSeconds("DATA_MAX_FUTURE_SKEW_SECONDS", 120),
			FundingMin:        getenvFloat("FUNDING_SANITY_MIN", -0.05),
			FundingMax:        getenvFloat("FUNDING_SANITY_MAX", 0.05),
			RestartJobTimeout: getenvDurationSeconds("RECOVERY_RUNNING_JOB_TIMEOUT_SECONDS", 900),
			RealtimeLagPause:  getenvDurationSeconds("BACKFILL_REALTIME_LAG_PAUSE_SECONDS", 300),
			BackfillMaxJobs:   getenvInt("BACKFILL_MAX_JOBS_PER_RUN", 1000),
			BackoffBase:       getenvDurationSeconds("RETRY_BACKOFF_BASE_SECONDS", 30),
			BackoffMax:        getenvDurationSeconds("RETRY_BACKOFF_MAX_SECONDS", 900),
			BackoffJitterMax:  getenvDurationSeconds("RETRY_BACKOFF_JITTER_MAX_SECONDS", 15),
		},
		SupportedExchanges: parseCSV(getenv("SUPPORTED_EXCHANGES", "binance,okx,bybit,bitget,gate,mexc")),
	}
}

func (c Config) Validate() error {
	var missing []string

	required := map[string]string{
		"POSTGRES_HOST":     c.Postgres.Host,
		"POSTGRES_PORT":     c.Postgres.Port,
		"POSTGRES_USER":     c.Postgres.User,
		"POSTGRES_PASSWORD": c.Postgres.Password,
		"POSTGRES_DB":       c.Postgres.Database,
		"POSTGRES_SSLMODE":  c.Postgres.SSLMode,
	}

	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, key)
		}
	}

	if len(c.SupportedExchanges) == 0 {
		missing = append(missing, "SUPPORTED_EXCHANGES")
	}

	if c.WorkerConcurrency < 1 {
		return fmt.Errorf("WORKER_CONCURRENCY must be greater than 0")
	}
	if c.Postgres.MaxOpenConns < 1 {
		return fmt.Errorf("POSTGRES_MAX_OPEN_CONNS must be greater than 0")
	}
	if c.Postgres.MaxIdleConns < 0 {
		return fmt.Errorf("POSTGRES_MAX_IDLE_CONNS must be greater than or equal to 0")
	}
	if c.RateLimit.RequestsPerSecond < 1 {
		return fmt.Errorf("RATE_LIMIT_REQUESTS_PER_SECOND must be greater than 0")
	}
	if c.RateLimit.Burst < 1 {
		return fmt.Errorf("RATE_LIMIT_BURST must be greater than 0")
	}
	if c.Analytics.MaxSnapshotAge <= 0 {
		return fmt.Errorf("ANALYTICS_MAX_SNAPSHOT_AGE_SECONDS must be greater than 0")
	}
	if c.Retention.MaxRowsPerRun < 1 {
		return fmt.Errorf("RETENTION_MAX_ROWS_PER_RUN must be greater than 0")
	}
	if c.Retention.TableTimeout <= 0 {
		return fmt.Errorf("RETENTION_TABLE_TIMEOUT_SECONDS must be greater than 0")
	}
	if c.Realtime.LatestTTL <= 0 {
		return fmt.Errorf("REALTIME_LATEST_TTL_SECONDS must be greater than 0")
	}
	if c.Realtime.StreamStaleAfter <= 0 {
		return fmt.Errorf("REALTIME_STREAM_STALE_SECONDS must be greater than 0")
	}
	if c.Realtime.HeartbeatInterval <= 0 {
		return fmt.Errorf("REALTIME_HEARTBEAT_SECONDS must be greater than 0")
	}
	if c.Realtime.ReconnectMin <= 0 || c.Realtime.ReconnectMax <= 0 || c.Realtime.ReconnectMin > c.Realtime.ReconnectMax {
		return fmt.Errorf("realtime reconnect durations must be positive and ordered")
	}
	if c.Realtime.SnapshotBucket <= 0 {
		return fmt.Errorf("REALTIME_SNAPSHOT_BUCKET_SECONDS must be greater than 0")
	}
	if c.Hardening.MaxFutureSkew <= 0 {
		return fmt.Errorf("DATA_MAX_FUTURE_SKEW_SECONDS must be greater than 0")
	}
	if c.Hardening.FundingMin >= c.Hardening.FundingMax {
		return fmt.Errorf("FUNDING_SANITY_MIN must be less than FUNDING_SANITY_MAX")
	}
	if c.Hardening.RestartJobTimeout <= 0 {
		return fmt.Errorf("RECOVERY_RUNNING_JOB_TIMEOUT_SECONDS must be greater than 0")
	}
	if c.Hardening.BackfillMaxJobs < 1 {
		return fmt.Errorf("BACKFILL_MAX_JOBS_PER_RUN must be greater than 0")
	}
	if c.Hardening.BackoffBase <= 0 || c.Hardening.BackoffMax <= 0 {
		return fmt.Errorf("retry backoff durations must be greater than 0")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return nil
}

func (c PostgresConfig) DSN() string {
	user := url.User(c.User)
	if c.Password != "" {
		user = url.UserPassword(c.User, c.Password)
	}

	dsn := &url.URL{
		Scheme: "postgres",
		User:   user,
		Host:   net.JoinHostPort(c.Host, c.Port),
		Path:   c.Database,
	}

	query := dsn.Query()
	query.Set("sslmode", c.SSLMode)
	dsn.RawQuery = query.Encode()

	return dsn.String()
}

func (c RedisConfig) Addr() string {
	return net.JoinHostPort(c.Host, c.Port)
}

func getenv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func getenvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func getenvFloat(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}

	return parsed
}

func getenvDurationSeconds(key string, fallbackSeconds int) time.Duration {
	return time.Duration(getenvInt(key, fallbackSeconds)) * time.Second
}

func getenvBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}

	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func parseCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))

	for _, part := range parts {
		item := strings.ToLower(strings.TrimSpace(part))
		if item != "" {
			items = append(items, item)
		}
	}

	return items
}
