package config

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/denisakp/ogoune/internal/scheduler"
	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	RedisUrl         string
	DBDriver         string
	DatabaseUrl      string
	SQLitePath       string
	DBLogLevel       string
	Port             string
	StaticDir        string
	WebHookUrl       string
	WebHookSignature string
	WebHookIsEnabled bool
	AuthEmail        string
	AuthPassword     string
	JWTSecret        string

	// Scheduler configuration
	SchedulerMode                  string
	SchedulerTickInterval          time.Duration
	SchedulerMaxWorkers            int
	SchedulerShutdownTimeout       time.Duration
	SchedulerNotificationQueueSize int

	// Confirmation window defaults
	ConfirmationChecks      int
	ConfirmationInterval    int
	ExpiryAlertThresholds   string
	FlapDetectionEnabled    bool
	FlapThreshold           int
	FlapWindowSeconds       int
	FlapMaxDurationMinutes  int
	ReminderIntervalMinutes int
	GroupingWindowSeconds   int
	// ICMP configuration
	EnableICMP bool

	// AllowPrivateTargets, when true, lets monitors and toolbox tools reach
	// private/loopback/link-local addresses (RFC1918 etc.). Default false keeps
	// the SSRF guard on for public/SaaS instances; self-hosters monitoring their
	// own LAN set ALLOW_PRIVATE_TARGETS=true.
	AllowPrivateTargets bool

	// Metrics configuration
	MetricsEnabled bool
	MetricsToken   string

	// Notification feed retention
	NotificationRetentionDays int

	HostMetricsRetentionDays int           // purge samples older than this (default 7)
	HostMetricsRawWindow     time.Duration // keep raw (undecimated) samples within this window (default 48h)
	HostFreshnessThreshold   time.Duration // a host is "online" while last_seen_at is within this (default 45s)

	// Agent-down alerting (spec 083). A recurring scan raises a feed alert when an
	// agent-backed host has been continuously offline past the freshness threshold.
	AgentDownAlertsEnabled    bool          // master switch (default true)
	AgentDownScanInterval     time.Duration // offline scan cadence (default 20s)
	AgentDownExternalDelivery bool          // also send offline/recovery to the oldest SMTP channel (default false)

	// Swagger configuration
	EnableSwagger bool

	// Logging configuration
	LogFormat string
	LogLevel  string

	// Environment
	AppEnv string

	// SSL provider for custom domains
	// One of: "letsencrypt" | "external" | "disabled". Default "external".
	SSLProvider string

	// Public base URL of the app, used for magic-link emails (FR-012a).
	AppBaseURL string

	// Security configuration
	CORSAllowedOrigins    []string
	RateLimitAuth         int
	RateLimitAuthWindow   time.Duration
	RateLimitGlobal       int
	RateLimitGlobalWindow time.Duration
}

// Load reads configuration from environment variables.
// It should be called after attempting to load .env file.
func Load() Config {
	webhookUrl := GetEnv("WEBHOOK_URL", "")
	webhookIsEnabled := webhookUrl != ""

	// Parse scheduler configuration
	schedulerTickInterval := parseDuration(GetEnv("SCHEDULER_TICK_INTERVAL", "1s"))
	schedulerMaxWorkers := parseInt(GetEnv("SCHEDULER_MAX_WORKERS", "10"))
	schedulerShutdownTimeout := parseDuration(GetEnv("SCHEDULER_SHUTDOWN_TIMEOUT", "15s"))
	schedulerNotificationQueueSize := parseInt(GetEnv("SCHEDULER_NOTIFICATION_QUEUE_SIZE", "100"))
	confirmationChecks := parseInt(GetEnv("CONFIRMATION_CHECKS", "2"))
	confirmationInterval := parseInt(GetEnv("CONFIRMATION_INTERVAL", "30"))
	flapDetectionEnabled := parseBool(GetEnv("FLAP_DETECTION_ENABLED", "true"), true)
	flapThreshold := parseInt(GetEnv("FLAP_THRESHOLD", "4"))
	flapWindowSeconds := parseInt(GetEnv("FLAP_WINDOW_SECONDS", "600"))
	flapMaxDurationMinutes := parseInt(GetEnv("FLAP_MAX_DURATION_MINUTES", "30"))
	reminderIntervalMinutes := parseInt(GetEnv("REMINDER_INTERVAL_MINUTES", "0"))
	groupingWindowSeconds := parseInt(GetEnv("GROUPING_WINDOW_SECONDS", "30"))
	enableICMP := parseBool(GetEnv("ENABLE_ICMP", "false"), false)
	allowPrivateTargets := parseBool(GetEnv("ALLOW_PRIVATE_TARGETS", "false"), false)
	metricsEnabled := parseBool(GetEnv("ENABLE_METRICS", "false"), false)
	metricsToken := GetEnv("METRICS_TOKEN", "")
	notificationRetentionDays := parseInt(GetEnv("NOTIFICATION_RETENTION_DAYS", "90"))
	if notificationRetentionDays <= 0 {
		notificationRetentionDays = 90 // never 0 — would prune the entire feed
	}
	hostMetricsRetentionDays := parseInt(GetEnv("HOST_METRICS_RETENTION_DAYS", "7"))
	if hostMetricsRetentionDays <= 0 {
		hostMetricsRetentionDays = 7 // never 0 — would purge all host metrics
	}
	hostMetricsRawWindow := parseDuration(GetEnv("HOST_METRICS_RAW_WINDOW", "48h"))
	hostFreshnessThreshold := parseDuration(GetEnv("HOST_FRESHNESS_THRESHOLD", "45s"))
	agentDownAlertsEnabled := parseBool(GetEnv("AGENT_DOWN_ALERTS_ENABLED", "true"), true)
	agentDownScanInterval := parseDuration(GetEnv("AGENT_DOWN_SCAN_INTERVAL", "20s"))
	if agentDownScanInterval <= 0 {
		agentDownScanInterval = 20 * time.Second
	}
	agentDownExternalDelivery := parseBool(GetEnv("AGENT_DOWN_EXTERNAL_DELIVERY", "false"), false)
	enableSwagger := parseBool(GetEnv("ENABLE_SWAGGER", "false"), false)
	logFormat := GetEnv("LOG_FORMAT", "json")
	logLevel := GetEnv("LOG_LEVEL", "info")
	appEnv := GetEnv("APP_ENV", "development")
	sslProvider := GetEnv("SSL_PROVIDER", "external")
	appBaseURL := GetEnv("APP_BASE_URL", "http://localhost:5173")

	// Security configuration
	corsOrigins := parseCORSOrigins(GetEnv("CORS_ALLOWED_ORIGINS", ""))
	rateLimitAuthCount, rateLimitAuthWindow := parseRateLimit(GetEnv("RATE_LIMIT_AUTH", "10/1m"), 10, 1*time.Minute)
	rateLimitGlobalCount, rateLimitGlobalWindow := parseRateLimit(GetEnv("RATE_LIMIT_GLOBAL", "100/1m"), 100, 1*time.Minute)

	cfg := Config{
		RedisUrl:         GetEnv("REDIS_URL", "localhost:6379"),
		DBDriver:         GetEnv("DB_DRIVER", "sqlite"),
		DatabaseUrl:      GetEnv("DATABASE_URL", ""),
		SQLitePath:       GetEnv("SQLITE_PATH", "ogoune.db"),
		DBLogLevel:       GetEnv("DB_LOG_LEVEL", "error"),
		Port:             GetEnv("APP_PORT", "9596"),
		WebHookUrl:       webhookUrl,
		WebHookSignature: GetEnv("WEBHOOK_SIGNATURE", ""),
		WebHookIsEnabled: webhookIsEnabled,
		StaticDir:        GetEnv("STATIC_DIR", "web/dist"),
		AuthEmail:        GetEnv("AUTH_EMAIL", "zoltar@ogoune.test"),
		AuthPassword:     GetEnv("AUTH_PASSWORD", ""),
		JWTSecret:        GetEnv("JWT_SECRET", ""),

		// Scheduler defaults (mode determined separately)
		SchedulerMode:                  GetEnv("SCHEDULER_MODE", "timingwheel"),
		SchedulerTickInterval:          schedulerTickInterval,
		SchedulerMaxWorkers:            schedulerMaxWorkers,
		SchedulerShutdownTimeout:       schedulerShutdownTimeout,
		SchedulerNotificationQueueSize: schedulerNotificationQueueSize,
		ConfirmationChecks:             confirmationChecks,
		ConfirmationInterval:           confirmationInterval,
		ExpiryAlertThresholds:          GetEnv("EXPIRY_ALERT_THRESHOLDS", "30,14,7,1"),
		FlapDetectionEnabled:           flapDetectionEnabled,
		FlapThreshold:                  flapThreshold,
		FlapWindowSeconds:              flapWindowSeconds,
		FlapMaxDurationMinutes:         flapMaxDurationMinutes,
		ReminderIntervalMinutes:        reminderIntervalMinutes,
		GroupingWindowSeconds:          groupingWindowSeconds,
		LogFormat:                      logFormat,
		LogLevel:                       logLevel,
		EnableICMP:                     enableICMP,
		AllowPrivateTargets:            allowPrivateTargets,
		MetricsEnabled:                 metricsEnabled,
		MetricsToken:                   metricsToken,
		NotificationRetentionDays:      notificationRetentionDays,
		HostMetricsRetentionDays:       hostMetricsRetentionDays,
		HostMetricsRawWindow:           hostMetricsRawWindow,
		HostFreshnessThreshold:         hostFreshnessThreshold,
		AgentDownAlertsEnabled:         agentDownAlertsEnabled,
		AgentDownScanInterval:          agentDownScanInterval,
		AgentDownExternalDelivery:      agentDownExternalDelivery,
		EnableSwagger:                  enableSwagger,
		AppEnv:                         appEnv,
		SSLProvider:                    sslProvider,
		AppBaseURL:                     appBaseURL,

		CORSAllowedOrigins:    corsOrigins,
		RateLimitAuth:         rateLimitAuthCount,
		RateLimitAuthWindow:   rateLimitAuthWindow,
		RateLimitGlobal:       rateLimitGlobalCount,
		RateLimitGlobalWindow: rateLimitGlobalWindow,
	}
	return cfg
}

// GetEnv retrieves an environment variable or returns a default value if not set.
func GetEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// GetSchedulerConfig returns the resolved scheduler configuration based on environment.
func (c *Config) GetSchedulerConfig() (*scheduler.Config, error) {
	// Detect mode based on database driver and explicit configuration
	mode := scheduler.DetectMode(c.SchedulerMode, c.DBDriver)

	// Validate mode compatibility with Redis
	if err := scheduler.ValidateSelector(mode, c.RedisUrl); err != nil {
		return nil, err
	}

	return &scheduler.Config{
		Mode: mode,
		TimingWheel: scheduler.TimingWheelConfig{
			TickInterval:          c.SchedulerTickInterval,
			MaxWorkers:            c.SchedulerMaxWorkers,
			ShutdownTimeout:       c.SchedulerShutdownTimeout,
			NotificationQueueSize: c.SchedulerNotificationQueueSize,
		},
		Asynq: scheduler.AsynqConfig{
			RedisURL: c.RedisUrl,
		},
	}, nil
}

// parseDuration parses a duration string or returns default on error.
func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 1 * time.Second // Default fallback
	}
	return d
}

// parseInt parses an integer or returns default on error.
func parseInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0 // Default fallback
	}
	return i
}

// parseCORSOrigins splits a comma-separated list of origins, trimming whitespace.
// Returns nil if the input is empty.
func parseCORSOrigins(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	if len(origins) == 0 {
		return nil
	}
	return origins
}

// parseRateLimit parses a rate limit string in "count/duration" format (e.g., "10/1m").
// Returns the provided defaults on any parse error.
func parseRateLimit(s string, defaultCount int, defaultWindow time.Duration) (int, time.Duration) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		slog.Warn("invalid rate limit format, using defaults", "value", s)
		return defaultCount, defaultWindow
	}
	count, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || count <= 0 {
		slog.Warn("invalid rate limit count, using defaults", "value", parts[0])
		return defaultCount, defaultWindow
	}
	window, err := time.ParseDuration(strings.TrimSpace(parts[1]))
	if err != nil || window <= 0 {
		slog.Warn("invalid rate limit window, using defaults", "value", parts[1])
		return defaultCount, defaultWindow
	}
	return count, window
}

func parseBool(s string, defaultValue bool) bool {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return defaultValue
	}
	return v
}

// MustInit loads configuration from .env file (if present) and environment variables.
// It attempts to load a .env file first, falls back to system environment variables,
// and panics if critical configuration is missing.
func MustInit() Config {
	// Attempt to load .env file - ignore error if file doesn't exist
	if err := godotenv.Load(); err != nil {
		slog.Info(".env file not found, falling back to system environment variables")
	} else {
		slog.Info("loaded configuration from .env file")
	}

	cfg := Load()

	// Validate critical configuration for the selected driver.
	if cfg.DBDriver != "sqlite" && cfg.DatabaseUrl == "" {
		slog.Error("DATABASE_URL environment variable is required when DB_DRIVER is postgres")
		os.Exit(1)
	}

	// Validate SSL_PROVIDER. Allowed: letsencrypt|external|disabled.
	switch cfg.SSLProvider {
	case "letsencrypt", "external", "disabled":
		// ok
	default:
		slog.Error("SSL_PROVIDER invalid", "got", cfg.SSLProvider, "want", "letsencrypt|external|disabled")
		os.Exit(1)
	}

	// Resolve JWT secret: env → persisted file → auto-generate
	cfg.JWTSecret = resolveJWTSecret(cfg.SQLitePath)

	// Resolve auth password: env → auto-generate and log
	cfg.AuthPassword = resolveAuthPassword()

	slog.Info("configuration loaded", "port", cfg.Port, "db_driver", cfg.DBDriver)

	// Log scheduler mode determination
	mode := scheduler.DetectMode(cfg.SchedulerMode, cfg.DBDriver)
	slog.Info("scheduler mode resolved", "mode", mode)

	return cfg
}

// resolveJWTSecret returns the JWT secret from env var, a persisted file, or generates a new one.
// The generated secret is written to <data_dir>/.jwt_secret so it survives restarts.
func resolveJWTSecret(sqlitePath string) string {
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		return secret
	}

	dataDir := filepath.Dir(sqlitePath)
	secretFile := filepath.Join(dataDir, ".jwt_secret")

	if data, err := os.ReadFile(secretFile); err == nil {
		if secret := strings.TrimSpace(string(data)); secret != "" {
			return secret
		}
	}

	secret := generateRandomHex(32)
	if err := os.MkdirAll(dataDir, 0750); err == nil {
		if err := os.WriteFile(secretFile, []byte(secret), 0600); err == nil {
			slog.Warn("JWT_SECRET not set — auto-generated and saved. Copy this value to your .env to make it explicit.",
				"file", secretFile,
				"JWT_SECRET", secret,
			)
			return secret
		}
	}

	slog.Warn("JWT_SECRET not set and could not persist — sessions will be lost on restart. Set JWT_SECRET in your .env.",
		"JWT_SECRET", secret,
	)
	return secret
}

// resolveAuthPassword returns the admin password from env var or generates a random one.
// A generated password is logged once so the user can retrieve it.
func resolveAuthPassword() string {
	if password := os.Getenv("AUTH_PASSWORD"); password != "" {
		return password
	}
	password := generateRandomHex(8) // 16-char hex = readable enough
	slog.Warn("AUTH_PASSWORD not set — generated a temporary admin password. Set AUTH_PASSWORD in your .env to make it permanent.",
		"AUTH_PASSWORD", password,
	)
	return password
}

// generateRandomHex returns a cryptographically random hex string of n bytes (2n chars).
func generateRandomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// Extremely unlikely; fall back to a timestamp-based value rather than crashing
		return hex.EncodeToString([]byte("fallback-change-me"))
	}
	return hex.EncodeToString(b)
}
