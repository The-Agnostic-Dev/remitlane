// Package config reads Remitlane's settings from the environment. Every
// variable has a default that works for local development, so the app starts
// with no .env file at all.
package config

import (
	"fmt"
	"strings"
	"time"
)

// Config is the full set of runtime settings.
type Config struct {
	Env             string
	Port            int
	BaseURL         string
	ShutdownTimeout time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration

	DatabaseURL    string
	DBMaxConns     int32
	MigrateOnStart bool
	SeedOnStart    bool

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	LogLevel  string
	LogFormat string

	BusinessTZ string

	SessionSecret  string
	SessionTTL     time.Duration
	SecureCookies  bool
	DevTools       bool
	APIRatePerMin  int
	MaxBulkItems   int
	QuoteTTL       time.Duration
	DefaultLimitUS int64

	Rates    RatesConfig
	Swiftpay SwiftpayConfig
	MoMo     MoMoConfig
	Webhooks WebhookConfig
	Jobs     JobsConfig
	Mail     MailConfig
	Alerts   AlertsConfig
}

// RatesConfig holds the fixed mid-market rates used by the sandbox rate feed.
type RatesConfig struct {
	Provider string
	USDNGN   string
	USDKES   string
	USDGHS   string
	USDUGX   string
}

// SwiftpayConfig configures the Swiftpay bank rail.
type SwiftpayConfig struct {
	Mode            string
	BaseURL         string
	APIKey          string
	WebhookSecret   string
	APIVersion      string
	Timeout         time.Duration
	SandboxDelay    time.Duration
	SandboxHold     time.Duration
	ReviewDelay     time.Duration
	ReviewThreshold int64
}

// MoMoConfig configures the MoMoBridge mobile money rail.
type MoMoConfig struct {
	Mode          string
	BaseURL       string
	APIKey        string
	WebhookSecret string
	Timeout       time.Duration
	SandboxDelay  time.Duration
}

// WebhookConfig configures outbound partner webhooks.
type WebhookConfig struct {
	Timeout     time.Duration
	MaxAttempts int
	RetryBase   time.Duration
}

// JobsConfig configures the asynq worker and scheduler.
type JobsConfig struct {
	Concurrency     int
	DelayedCheck    time.Duration
	DispatchRetries int
	DispatchBase    time.Duration
	PollInterval    time.Duration
	PollStuckAfter  time.Duration
	ReconCron       string
}

// MailConfig configures outgoing email.
type MailConfig struct {
	Provider string
	From     string
	ReconTo  string
	OpsTo    string
}

// AlertsConfig configures balance alerts.
type AlertsConfig struct {
	LowBalanceUSD int64
}

// Load reads the environment.
func Load() (Config, error) {
	e := &env{}
	c := Config{
		Env:             e.str("APP_ENV", "development"),
		Port:            e.int("PORT", 5121),
		ShutdownTimeout: e.dur("SHUTDOWN_TIMEOUT", 10*time.Second),
		ReadTimeout:     e.dur("HTTP_READ_TIMEOUT", 15*time.Second),
		WriteTimeout:    e.dur("HTTP_WRITE_TIMEOUT", 30*time.Second),

		DatabaseURL:    e.str("DATABASE_URL", "postgres://postgres@localhost:55421/remitlane?sslmode=disable"),
		DBMaxConns:     int32(e.int("DB_MAX_CONNS", 10)),
		MigrateOnStart: e.bool("MIGRATE_ON_START", true),
		SeedOnStart:    e.bool("SEED_ON_START", true),

		RedisAddr:     e.str("REDIS_ADDR", "localhost:56421"),
		RedisPassword: e.str("REDIS_PASSWORD", ""),
		RedisDB:       e.int("REDIS_DB", 0),

		LogLevel:  e.str("LOG_LEVEL", "info"),
		LogFormat: e.str("LOG_FORMAT", "text"),

		BusinessTZ: e.str("BUSINESS_TZ", "Africa/Lagos"),

		SessionSecret:  e.str("SESSION_SECRET", "dev-only-session-secret-change-me"),
		SessionTTL:     e.dur("SESSION_TTL", 12*time.Hour),
		SecureCookies:  e.bool("CONSOLE_SECURE_COOKIES", false),
		DevTools:       e.bool("DEV_TOOLS_ENABLED", true),
		APIRatePerMin:  e.int("API_RATE_LIMIT_PER_MIN", 600),
		MaxBulkItems:   e.int("BULK_MAX_ITEMS", 500),
		QuoteTTL:       e.dur("QUOTE_TTL", 15*time.Minute),
		DefaultLimitUS: int64(e.int("DEFAULT_DAILY_LIMIT_USD", 25000)),
	}
	c.BaseURL = strings.TrimRight(e.str("APP_BASE_URL", fmt.Sprintf("http://localhost:%d", c.Port)), "/")

	c.Rates = RatesConfig{
		Provider: e.str("RATES_PROVIDER", "fixed"),
		USDNGN:   e.str("RATES_USD_NGN", "1540.00"),
		USDKES:   e.str("RATES_USD_KES", "129.37"),
		USDGHS:   e.str("RATES_USD_GHS", "15.82"),
		USDUGX:   e.str("RATES_USD_UGX", "3698.40"),
	}
	c.Swiftpay = SwiftpayConfig{
		Mode:            e.str("SWIFTPAY_MODE", "sandbox"),
		BaseURL:         e.str("SWIFTPAY_BASE_URL", c.BaseURL+"/_sandbox/swiftpay"),
		APIKey:          e.str("SWIFTPAY_API_KEY", "sp_sandbox_key_7c1e0b"),
		WebhookSecret:   e.str("SWIFTPAY_WEBHOOK_SECRET", "sp_sandbox_whsec_3f9d22"),
		APIVersion:      e.str("SWIFTPAY_API_VERSION", "v2"),
		Timeout:         e.dur("SWIFTPAY_TIMEOUT", 10*time.Second),
		SandboxDelay:    e.dur("SWIFTPAY_SANDBOX_DELAY", 1500*time.Millisecond),
		SandboxHold:     e.dur("SWIFTPAY_SANDBOX_HOLD_DELAY", 4*time.Second),
		ReviewDelay:     e.dur("SWIFTPAY_SANDBOX_REVIEW_DELAY", 8*time.Second),
		ReviewThreshold: int64(e.int("SWIFTPAY_SANDBOX_REVIEW_THRESHOLD_NGN", 1000000)),
	}
	c.MoMo = MoMoConfig{
		Mode:          e.str("MOMOBRIDGE_MODE", "sandbox"),
		BaseURL:       e.str("MOMOBRIDGE_BASE_URL", c.BaseURL+"/_sandbox/momobridge"),
		APIKey:        e.str("MOMOBRIDGE_API_KEY", "mmb_sandbox_key_51aa90"),
		WebhookSecret: e.str("MOMOBRIDGE_WEBHOOK_SECRET", "mmb_sandbox_whsec_8e27c4"),
		Timeout:       e.dur("MOMOBRIDGE_TIMEOUT", 10*time.Second),
		SandboxDelay:  e.dur("MOMOBRIDGE_SANDBOX_DELAY", 1200*time.Millisecond),
	}
	c.Webhooks = WebhookConfig{
		Timeout:     e.dur("WEBHOOK_TIMEOUT", 5*time.Second),
		MaxAttempts: e.int("WEBHOOK_MAX_ATTEMPTS", 5),
		RetryBase:   e.dur("WEBHOOK_RETRY_BASE", 2*time.Second),
	}
	c.Jobs = JobsConfig{
		Concurrency:     e.int("WORKER_CONCURRENCY", 10),
		DelayedCheck:    e.dur("ASYNQ_DELAYED_CHECK_INTERVAL", time.Second),
		DispatchRetries: e.int("DISPATCH_MAX_RETRY", 3),
		DispatchBase:    e.dur("DISPATCH_RETRY_BASE", 2*time.Second),
		PollInterval:    e.dur("POLL_INTERVAL", 30*time.Second),
		PollStuckAfter:  e.dur("POLL_STUCK_AFTER", 30*time.Minute),
		ReconCron:       e.str("RECON_CRON", "0 1 * * *"),
	}
	c.Mail = MailConfig{
		Provider: e.str("MAIL_PROVIDER", "outbox"),
		From:     e.str("MAIL_FROM", "Remitlane <no-reply@remitlane.com>"),
		ReconTo:  e.str("RECON_EMAIL_TO", "finance@remitlane.com"),
		OpsTo:    e.str("OPS_ALERT_EMAIL", "ops@remitlane.com"),
	}
	c.Alerts = AlertsConfig{LowBalanceUSD: int64(e.int("LOW_BALANCE_ALERT_USD", 5000))}

	if err := e.err(); err != nil {
		return Config{}, err
	}
	return c, nil
}

// IsProduction reports whether the app runs in production.
func (c Config) IsProduction() bool { return c.Env == "production" }
