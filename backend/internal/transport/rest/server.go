package rest

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"beacon/internal/platform/apperror"
	"beacon/internal/platform/httpx"
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
	Alert              *AlertHandler
	Insight            *InsightHandler
	Billing            *BillingHandler
	StatusPage         *StatusPageHandler
	StatusPageSettings *StatusPageSettingsHandler
}

// NewRouter builds the fully-wired HTTP handler: middleware chain, operational
// endpoints, and the versioned /api/v1 surface.
func NewRouter(d RouterDeps) http.Handler {
	r := chi.NewRouter()

	// Middleware order (outermost first): identify the request, apply CORS,
	// log it, record metrics, then recover from any panic in the handler.
	r.Use(middleware.RequestID)
	r.Use(corsMiddleware(d.CORSOrigins))
	r.Use(middleware.Logging(d.Logger))
	r.Use(middleware.Metrics(d.Metrics))
	r.Use(middleware.Recover)

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
		api.Mount("/public/status", d.StatusPage.Routes())

		api.Mount("/auth", d.Auth.Routes())
		api.With(d.Authenticator.Require).Get("/me", d.Auth.Me)
		// Gateway auth_request target: validates the proxy cookie and returns the
		// tenant org id. Unauthenticated (does its own cookie check).
		api.Get("/proxy/authorize", d.Auth.Authorize)
		api.Mount("/projects", d.Project.Routes())
		api.Mount("/monitors", d.Monitor.Routes())
		api.Mount("/notification-channels", d.Notification.Routes())
		// Org-scoped active alerts (read from Prometheus, filtered by org_id).
		api.With(d.Authenticator.Require).Get("/alerts", d.Insight.ActiveAlerts)
		// Org-wide dashboard overview.
		api.With(d.Authenticator.Require).Get("/overview", d.Insight.Overview)
		api.Mount("/billing", d.Billing.Routes())
		// Owner-facing controls for the public page above (publish / rename).
		api.Mount("/status-page", d.StatusPageSettings.Routes())
		// Alertmanager webhook: no JWT (Alertmanager can't present one); guarded
		// by a shared secret inside the handler.
		api.Post("/alerts/webhook", d.Alert.Webhook)
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
