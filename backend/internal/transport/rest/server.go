package rest

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"beacon/internal/platform/apperror"
	"beacon/internal/platform/httpx"
	"beacon/internal/platform/ratelimit"
	"beacon/internal/platform/metrics"
	"beacon/internal/transport/rest/middleware"
)

// RouterDeps are the dependencies required to build the API router. Assembling
// them explicitly (rather than reaching for globals) keeps wiring visible and
// testable.
type RouterDeps struct {
	Logger        *slog.Logger
	Metrics       *metrics.Metrics
	CORSOrigins   []string
	Authenticator *middleware.Authenticator

	Health             *HealthHandler
	Auth               *AuthHandler
	Project            *ProjectHandler
	Monitor            *MonitorHandler
	Notification       *NotificationHandler
	Maintenance        *MaintenanceHandler
	Alert              *AlertHandler
	Insight            *InsightHandler
	Billing            *BillingHandler
	StatusPage         *StatusPageHandler
	Heartbeat          *HeartbeatHandler
	StatusPageSettings *StatusPageSettingsHandler
	// Diagnose may be nil when AI is not configured; the route is then not mounted.
	Diagnose           *DiagnoseHandler
	APIKey             *APIKeyHandler
	Sync               *SyncHandler
}

// NewRouter builds the fully-wired HTTP handler: middleware chain, operational
// endpoints, and the versioned /api/v1 surface.
// Rate limits, chosen from what the legitimate caller actually does rather than from
// round numbers. maxKeys bounds memory: buckets that refill fully are swept, since a
// full bucket behaves identically to a key never seen.
var (
	// Baseline for all traffic. A dashboard page can fire a dozen requests at once and
	// polls a few times a minute, so 20/s sustained with a burst of 60 is invisible to
	// a real user and still caps one source hard.
	baselineLimiter = ratelimit.New(20, 60, 100_000)

	// Registration: 5 an hour per address, bursting to 3 together.
	//
	// The tightest limit here, because it is the only endpoint where one request buys
	// permanent recurring cost — ten monitors probing every minute, forever, for as
	// long as the org exists. A real person signs up once; a team behind one office IP
	// might sign up a handful of times in a day. Anything past that is a script, and
	// the thing it is building is our bill.
	signupLimiter = ratelimit.New(5.0/3600.0, 3, 50_000)

	// Login: 1/s sustained, burst 10. Fat-fingering a password a few times is fine;
	// working through a credential list is not.
	loginLimiter = ratelimit.New(1, 10, 50_000)

	// Public status pages: 5/s per address, burst 20. Meant to survive being linked
	// from an incident report while bounding a single scraper.
	publicLimiter = ratelimit.New(5, 20, 100_000)

	// Declarative sync: 1 every 10s per ORG, burst 5.
	//
	// Keyed by org rather than address because a workflow legitimately runs from a
	// different GitHub runner IP every time — an address key would let one org loop
	// freely while limiting nobody. One request can create up to 500 monitors, and CI
	// calls it once per push, so this is generous for real use and stops a runaway
	// loop from rewriting the control plane continuously.
	syncLimiter = ratelimit.New(0.1, 5, 20_000)
)

func NewRouter(d RouterDeps) http.Handler {
	r := chi.NewRouter()

	// Middleware order (outermost first): identify the request, apply CORS,
	// log it, record metrics, then recover from any panic in the handler.
	r.Use(middleware.RequestID)
	r.Use(corsMiddleware(d.CORSOrigins))
	r.Use(middleware.Logging(d.Logger))
	r.Use(middleware.Metrics(d.Metrics))
	r.Use(middleware.Recover)
	// A baseline ceiling on everything, keyed by address. Generous enough that a
	// dashboard doing a dozen parallel fetches never notices, low enough that a single
	// source cannot saturate the API. The tighter limits below sit UNDER this one, so
	// an endpoint whose abuse is cheap gets both.
	r.Use(middleware.RateLimit(baselineLimiter, middleware.ByIP, 5*time.Second))

	// Operational endpoints (unversioned, unauthenticated).
	r.Get("/healthz", d.Health.Health)
	r.Get("/livez", d.Health.Live)
	r.Get("/readyz", d.Health.Ready)
	r.Handle("/metrics", d.Metrics.Handler())

	// Versioned API.
	r.Route("/api/v1", func(api chi.Router) {
		// PUBLIC, unauthenticated: the customer-facing status page. Mounted under
		// an explicit /public prefix so that "this needs no token" is obvious from
		// the URL in logs, proxies and rate-limit rules — not something a reviewer
		// has to infer from the absence of a middleware.
		// Public and uncached at the origin: every hit is a database read that anyone
		// can trigger. The page is meant to survive being linked from an incident
		// report, so the limit is per-address and roomy — it bounds one source, not
		// legitimate traffic.
		api.With(middleware.RateLimit(publicLimiter, middleware.ByIP, 5*time.Second)).
			Mount("/public/status", d.StatusPage.Routes())
		// PUBLIC, unauthenticated: heartbeat ping ingest. The URL token is the
		// credential; rate-limited per token inside the handler.
		// Same payload as /healthz, mounted where a BROWSER can reach it: the gateway
		// routes /api to the API, and /healthz only answers to the orchestrator. Public,
		// like /healthz already is — the version it discloses is disclosed there too, and
		// the environment is legible from the hostname.
		api.Get("/system/info", d.Health.Health)
		api.Mount("/ping", d.Heartbeat.Routes())

		api.Mount("/auth", d.Auth.Routes())
		api.With(d.Authenticator.Require).Get("/me", d.Auth.Me)
		// Gateway auth_request target: validates the proxy cookie and returns the
		// tenant org id. Unauthenticated (does its own cookie check).
		api.Get("/proxy/authorize", d.Auth.Authorize)
		api.Mount("/projects", d.Project.Routes())
		api.Mount("/monitors", d.Monitor.Routes())
		// Mounted as a sibling of the monitors subrouter rather than inside it: the
		// diagnosis service is not the monitor service, and chi resolves this more
		// specific path ahead of the mount's wildcard. Nil when AI is unconfigured, so
		// the endpoint simply does not exist rather than 500ing on every call.
		if d.Diagnose != nil {
			api.Mount("/monitors/{id}/diagnose", d.Diagnose.Routes())
		}
		api.Mount("/notification-channels", d.Notification.Routes())
		api.Mount("/maintenance-windows", d.Maintenance.Routes())
		// Org-scoped active alerts (read from Prometheus, filtered by org_id).
		api.With(d.Authenticator.Require).Get("/alerts", d.Insight.ActiveAlerts)
		// Org-wide dashboard overview.
		api.With(d.Authenticator.Require).Get("/overview", d.Insight.Overview)
		api.Mount("/billing", d.Billing.Routes())
		// Owner-facing controls for the public page above (publish / rename).
		api.Mount("/status-page", d.StatusPageSettings.Routes())
		// Machine surface. /api-keys is session-only (a key must not mint keys);
		// /sync is the declarative endpoint CI calls, and accepts either credential.
		api.Mount("/api-keys", d.APIKey.Routes())
		api.Mount("/sync", d.Sync.Routes())
		// Alertmanager webhook: no JWT (Alertmanager can't present one); guarded
		// by a shared secret inside the handler.
		api.Post("/alerts/webhook", d.Alert.Webhook)
		// Stripe webhook: no JWT (Stripe can't present one); authenticity comes
		// from the signature the handler verifies against the webhook secret.
		api.Post("/billing/webhook", d.Billing.Webhook)
	})

	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		httpx.Error(w, req, apperror.NotFound("the requested resource does not exist"))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		httpx.Error(w, req, apperror.New(apperror.CodeValidation, "method not allowed"))
	})

	return r
}

// corsMiddleware configures CORS for the dashboard origin(s).
func corsMiddleware(origins []string) func(http.Handler) http.Handler {
	if len(origins) == 0 {
		origins = []string{"http://localhost:3000"}
	}
	return cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", middleware.HeaderRequestID, middleware.HeaderCorrelationID},
		ExposedHeaders:   []string{middleware.HeaderRequestID, middleware.HeaderCorrelationID},
		AllowCredentials: true,
		MaxAge:           300,
	})
}

// SyncLimiter exposes the sync limiter to main, so the rate lives beside the others
// rather than being invented at the wiring site.
func SyncLimiter() *ratelimit.KeyedLimiter { return syncLimiter }
