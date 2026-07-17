// Package config loads and validates Beacon runtime configuration from the
// environment. Configuration is read once at startup into an immutable struct
// that is injected into the rest of the application; nothing reads os.Getenv
// after Load returns. This keeps configuration explicit, testable, and free of
// hidden global state.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Environment enumerates deployment environments.
type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvStaging     Environment = "staging"
	EnvProduction  Environment = "production"
)

// Config is the fully-resolved application configuration.
type Config struct {
	Env       Environment
	HTTP      HTTP
	Log       Log
	DB        DB
	Redis     Redis
	Auth      Auth
	Crypto    Crypto
	CtrlPlane ControlPlane
	Worker    Worker
	Notify    Notify
	AI        AI
	Billing   Billing
}

// AI holds optional LLM-based alert enrichment configuration. When Enabled, a
// firing alert is sent to an Ollama-compatible endpoint that classifies its real
// severity and suggests a fix; that analysis is attached to the notification
// before it is delivered. Enrichment is always best-effort: if the model is slow
// or unreachable the alert is still delivered, just without the AI section.
type AI struct {
	// Enabled turns alert enrichment on. Off by default so existing deployments
	// behave exactly as before.
	Enabled bool
	// BaseURL is the Ollama-compatible endpoint root, e.g.
	// https://ollama.example.com (no trailing slash, no /api).
	BaseURL string
	// Model is the model tag to prompt, e.g. "llama3.1".
	Model string
	// APIKey, when set, is sent as the `x-api-key` header (for endpoints behind
	// an auth proxy). Empty means no auth header.
	APIKey string
	// Timeout bounds a single analysis call. On expiry the alert is delivered
	// without enrichment.
	Timeout time.Duration
	// DiagnoseAllowPrivate lets AI diagnosis probe private, loopback and
	// link-local addresses. OFF by default, and it must stay off anywhere the
	// monitors are not yours.
	//
	// Diagnosis reports resolved addresses, certificate subjects and connection
	// errors back to whoever asked, and the target is a value they chose. Pointed
	// at an internal range that turns Beacon into a port scanner they can drive
	// from the outside — the cloud metadata endpoint, the Kubernetes API, Postgres.
	// This exists for the single-tenant operator diagnosing their own internal
	// services, where the caller already owns the network and there is nothing to
	// disclose to them.
	DiagnoseAllowPrivate bool
	// DiagnoseCostSeconds is what one AI diagnosis costs a pay-as-you-go org, in
	// monitor-seconds. Defaults to 5 monitor-minutes (~1.7¢) — about what five
	// minutes of monitoring one domain costs, which is the point: the price is meant
	// to be explainable, not to earn. The model runs on our own hardware, so what a
	// run really costs is GPU seconds already paid for. Subscribed tiers ignore this
	// and spend a monthly allowance instead.
	DiagnoseCostSeconds int64
}

// Notify holds notification/alerting configuration.
// Billing configures Stripe payments: recurring subscriptions (Starter/Pro) and
// one-time pay-as-you-go top-ups. Empty StripeSecretKey disables billing entirely
// (the API returns a clear error instead of half-working).
type Billing struct {
	StripeSecretKey      string
	StripePublishableKey string
	// StripeWebhookSecret verifies the signature on Stripe's webhook POSTs.
	StripeWebhookSecret string
	// PriceStarter / PricePro are the Stripe Price IDs for the two subscription
	// tiers. Empty means that tier is not purchasable (Checkout is refused).
	PriceStarter string
	PricePro     string
	// MonitorHoursPerDollar is the pay-as-you-go rate: $1 buys this many
	// monitor-hours of credit. Default 5.
	MonitorHoursPerDollar int
	// SuccessURL / CancelURL are where Stripe Checkout returns the customer.
	SuccessURL string
	CancelURL  string
}

// Enabled reports whether Stripe is configured.
func (b Billing) Enabled() bool { return b.StripeSecretKey != "" }

type Notify struct {
	// WebhookToken is the shared secret Alertmanager presents (as a Bearer
	// token) when POSTing alerts to Beacon's webhook. Empty disables the check
	// (development only).
	WebhookToken string
	// DashboardURL is the base URL used to build "open dashboard" links in
	// notification messages.
	DashboardURL string
	// WebhookAllowPrivate lets tenant webhook/Slack channels reach private,
	// loopback and link-local addresses. This RE-OPENS the SSRF hole and exists
	// only for single-tenant on-prem operators who deliberately want an internal
	// webhook. NEVER enable in a multi-tenant deployment. Default false.
	WebhookAllowPrivate bool
	// WebhookAllowHTTP permits plain-http webhook targets (default false: https
	// only). For local development against an http test receiver.
	WebhookAllowHTTP bool
}

// HTTP holds the API server configuration.
type HTTP struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	CORSOrigins     []string
}

// Log holds structured-logging configuration.
type Log struct {
	Level  string // debug|info|warn|error
	Format string // json|text
}

// DB holds the Postgres connection pool configuration.
type DB struct {
	DSN             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
}

// Redis holds the cache/queue connection configuration.
type Redis struct {
	Addr     string
	Password string
	DB       int
}

// Auth holds JWT configuration for access and refresh tokens.
type Auth struct {
	AccessSecret  []byte
	RefreshSecret []byte
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
}

// Crypto holds symmetric-encryption configuration used to protect secrets at
// rest (e.g. notification credentials).
type Crypto struct {
	EncryptionKey []byte // exactly 32 bytes (AES-256)
}

// ControlPlane holds configuration for managing Prometheus and Blackbox. Beacon
// regenerates these files from the database and hot-reloads the services.
type ControlPlane struct {
	// PromScrapeFile is written with generated scrape_configs (referenced by the
	// main prometheus.yml via scrape_config_files).
	PromScrapeFile string
	// PromRulesFile is written with generated alerting rules (referenced via
	// rule_files).
	PromRulesFile string
	// PromReloadURL is Prometheus's POST /-/reload endpoint.
	PromReloadURL string
	// PromQueryURL is Prometheus's base URL for the query API (/api/v1/query),
	// used by the worker to read probe results back into monitor status.
	PromQueryURL string
	// BlackboxConfigFile is the full Blackbox config Beacon owns and rewrites.
	BlackboxConfigFile string
	// BlackboxReloadURL is Blackbox's POST /-/reload endpoint.
	BlackboxReloadURL string
	// BlackboxAddr is the host:port Prometheus uses to reach Blackbox's /probe.
	BlackboxAddr string
	// DNSResolver is the default resolver DNS monitors query (host:port).
	DNSResolver string
}

// Worker holds background-worker tuning.
type Worker struct {
	Concurrency int
	MaxRetries  int
	// MetricsAddr is where the worker serves /metrics. The worker (single-instance)
	// exports the heartbeat gauge here for Prometheus to scrape as job
	// beacon-worker. Distinct from the API's :8080/metrics.
	MetricsAddr string
}

// Load reads configuration from the environment, applies defaults, and
// validates the result. It returns an error describing every problem found so
// operators can fix misconfiguration in one pass rather than one variable at a
// time.
func Load() (Config, error) {
	var errs []string
	add := func(format string, a ...any) { errs = append(errs, fmt.Sprintf(format, a...)) }

	cfg := Config{
		Env: Environment(getStr("BEACON_ENV", "development")),
		HTTP: HTTP{
			Addr:            getStr("BEACON_HTTP_ADDR", ":8080"),
			ReadTimeout:     getDur("BEACON_HTTP_READ_TIMEOUT", 15*time.Second, add),
			WriteTimeout:    getDur("BEACON_HTTP_WRITE_TIMEOUT", 30*time.Second, add),
			ShutdownTimeout: getDur("BEACON_HTTP_SHUTDOWN_TIMEOUT", 20*time.Second, add),
			CORSOrigins:     getCSV("BEACON_CORS_ORIGINS", []string{"http://localhost:3000"}),
		},
		Log: Log{
			Level:  getStr("BEACON_LOG_LEVEL", "info"),
			Format: getStr("BEACON_LOG_FORMAT", "json"),
		},
		DB: DB{
			DSN:             getStr("BEACON_DB_DSN", ""),
			MaxConns:        int32(getInt("BEACON_DB_MAX_CONNS", 20, add)),
			MinConns:        int32(getInt("BEACON_DB_MIN_CONNS", 2, add)),
			MaxConnLifetime: getDur("BEACON_DB_MAX_CONN_LIFETIME", time.Hour, add),
		},
		Redis: Redis{
			Addr:     getStr("BEACON_REDIS_ADDR", "localhost:6379"),
			Password: getStr("BEACON_REDIS_PASSWORD", ""),
			DB:       getInt("BEACON_REDIS_DB", 0, add),
		},
		Auth: Auth{
			AccessSecret:  []byte(getStr("BEACON_JWT_ACCESS_SECRET", "")),
			RefreshSecret: []byte(getStr("BEACON_JWT_REFRESH_SECRET", "")),
			AccessTTL:     getDur("BEACON_JWT_ACCESS_TTL", 15*time.Minute, add),
			RefreshTTL:    getDur("BEACON_JWT_REFRESH_TTL", 720*time.Hour, add),
		},
		Crypto: Crypto{
			EncryptionKey: decodeKey(getStr("BEACON_ENCRYPTION_KEY", ""), add),
		},
		CtrlPlane: ControlPlane{
			PromScrapeFile:     getStr("BEACON_PROM_SCRAPE_FILE", "./deploy/prometheus/generated/scrape_monitors.yml"),
			PromRulesFile:      getStr("BEACON_PROM_RULES_FILE", "./deploy/prometheus/generated/rules_monitors.yml"),
			PromReloadURL:      getStr("BEACON_PROM_RELOAD_URL", "http://localhost:9090/-/reload"),
			PromQueryURL:       getStr("BEACON_PROM_QUERY_URL", "http://localhost:9090"),
			BlackboxConfigFile: getStr("BEACON_BLACKBOX_CONFIG_FILE", "./deploy/blackbox/blackbox.yml"),
			BlackboxReloadURL:  getStr("BEACON_BLACKBOX_RELOAD_URL", "http://localhost:9115/-/reload"),
			BlackboxAddr:       getStr("BEACON_BLACKBOX_ADDR", "localhost:9115"),
			DNSResolver:        getStr("BEACON_DNS_RESOLVER", "8.8.8.8:53"),
		},
		Worker: Worker{
			Concurrency: getInt("BEACON_WORKER_CONCURRENCY", 8, add),
			MaxRetries:  getInt("BEACON_WORKER_MAX_RETRIES", 5, add),
			MetricsAddr: getStr("BEACON_WORKER_METRICS_ADDR", ":8081"),
		},
		Notify: Notify{
			WebhookToken:        getStr("BEACON_WEBHOOK_TOKEN", ""),
			DashboardURL:        getStr("BEACON_DASHBOARD_URL", "http://localhost:3000"),
			WebhookAllowPrivate: getBool("BEACON_WEBHOOK_ALLOW_PRIVATE", false, add),
			WebhookAllowHTTP:    getBool("BEACON_WEBHOOK_ALLOW_HTTP", false, add),
		},
		Billing: Billing{
			StripeSecretKey:       getStr("STRIPE_SECRET_KEY", ""),
			StripePublishableKey:  getStr("STRIPE_PUBLISHABLE_KEY", ""),
			StripeWebhookSecret:   getStr("STRIPE_WEBHOOK_SECRET", ""),
			PriceStarter:          getStr("STRIPE_PRICE_STARTER", ""),
			PricePro:              getStr("STRIPE_PRICE_PRO", ""),
			MonitorHoursPerDollar: getInt("BEACON_BILLING_MONITOR_HOURS_PER_DOLLAR", 5, add),
			SuccessURL:            getStr("BEACON_BILLING_SUCCESS_URL", "http://localhost:3000/billing?checkout=success"),
			CancelURL:             getStr("BEACON_BILLING_CANCEL_URL", "http://localhost:3000/billing?checkout=cancel"),
		},
		AI: AI{
			Enabled: getBool("BEACON_AI_ENABLED", false, add),
			BaseURL: strings.TrimRight(getStr("BEACON_AI_BASE_URL", ""), "/"),
			Model:   getStr("BEACON_AI_MODEL", ""),
			APIKey:  getStr("BEACON_AI_API_KEY", ""),
			Timeout: getDur("BEACON_AI_TIMEOUT", 20*time.Second, add),
			DiagnoseAllowPrivate: getBool("BEACON_AI_DIAGNOSE_ALLOW_PRIVATE", false, add),
			DiagnoseCostSeconds:  int64(getInt("BEACON_AI_DIAGNOSE_COST_SECONDS", 5*60, add)),
		},
	}

	// ---- validation ----
	switch cfg.Env {
	case EnvDevelopment, EnvStaging, EnvProduction:
	default:
		add("BEACON_ENV %q is invalid (want development|staging|production)", cfg.Env)
	}
	if cfg.DB.DSN == "" {
		add("BEACON_DB_DSN is required")
	}
	if len(cfg.Auth.AccessSecret) < 32 {
		add("BEACON_JWT_ACCESS_SECRET must be at least 32 bytes")
	}
	if len(cfg.Auth.RefreshSecret) < 32 {
		add("BEACON_JWT_REFRESH_SECRET must be at least 32 bytes")
	}
	if len(cfg.Crypto.EncryptionKey) != 32 {
		add("BEACON_ENCRYPTION_KEY must decode to exactly 32 bytes (got %d)", len(cfg.Crypto.EncryptionKey))
	}
	if cfg.IsProduction() {
		if strings.Contains(string(cfg.Auth.AccessSecret), "change-me") ||
			strings.Contains(string(cfg.Auth.RefreshSecret), "change-me") {
			add("refusing to start in production with default 'change-me' JWT secrets")
		}
		if cfg.Notify.WebhookToken == "" {
			add("BEACON_WEBHOOK_TOKEN is required in production to authenticate the Alertmanager webhook")
		}
	}
	if cfg.Worker.Concurrency < 1 {
		add("BEACON_WORKER_CONCURRENCY must be >= 1")
	}
	if cfg.AI.Enabled {
		if cfg.AI.BaseURL == "" {
			add("BEACON_AI_BASE_URL is required when BEACON_AI_ENABLED=true")
		}
		if cfg.AI.Model == "" {
			add("BEACON_AI_MODEL is required when BEACON_AI_ENABLED=true")
		}
	}

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("invalid configuration:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return cfg, nil
}

// IsProduction reports whether the app is running in production.
func (c Config) IsProduction() bool { return c.Env == EnvProduction }

// ---- typed env helpers ----

func getStr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getBool(key string, def bool, add func(string, ...any)) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		add("%s must be a boolean (true/false), got %q", key, v)
		return def
	}
	return b
}

func getInt(key string, def int, add func(string, ...any)) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		add("%s must be an integer, got %q", key, v)
		return def
	}
	return n
}

func getDur(key string, def time.Duration, add func(string, ...any)) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		add("%s must be a duration (e.g. 15s, 1h), got %q", key, v)
		return def
	}
	return d
}

func getCSV(key string, def []string) []string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// decodeKey accepts a hex- or base64-encoded key and returns the raw bytes. An
// empty value yields nil so validation can report the requirement.
func decodeKey(v string, add func(string, ...any)) []byte {
	if v == "" {
		return nil
	}
	if b, err := hexDecode(v); err == nil {
		return b
	}
	if b, err := base64Decode(v); err == nil {
		return b
	}
	add("BEACON_ENCRYPTION_KEY must be hex or base64 encoded")
	return nil
}

var errBadEncoding = errors.New("bad encoding")
