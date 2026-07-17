// Command api runs the Beacon REST API server. It also provides a `migrate`
// subcommand for applying database migrations explicitly in production
// (`api migrate up` / `api migrate status`).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"beacon/internal/adapter/ai"
	"beacon/internal/adapter/netprobe"
	"beacon/internal/adapter/notifier"
	"beacon/internal/adapter/postgres"
	"beacon/internal/adapter/promapi"
	"beacon/internal/adapter/queue"
	stripeadapter "beacon/internal/adapter/stripe"
	"beacon/internal/config"
	"beacon/internal/domain/audit"
	"beacon/internal/domain/auth"
	"beacon/internal/domain/billing"
	"beacon/internal/domain/diagnose"
	"beacon/internal/domain/heartbeat"
	"beacon/internal/domain/insight"
	"beacon/internal/domain/maintenance"
	"beacon/internal/domain/monitor"
	"beacon/internal/domain/notification"
	"beacon/internal/domain/project"
	"beacon/internal/domain/statuspage"
	"beacon/internal/platform/cache"
	"beacon/internal/platform/crypto"
	"beacon/internal/platform/database"
	"beacon/internal/platform/logger"
	"beacon/internal/platform/metrics"
	"beacon/internal/platform/safehttp"
	"beacon/internal/platform/validate"
	"beacon/internal/transport/rest"
	"beacon/internal/transport/rest/middleware"
	"beacon/migrations"
)

// version is overridable at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logger.New(cfg.Log.Level, cfg.Log.Format)
	slog.SetDefault(log)

	// Root context cancelled on SIGINT/SIGTERM for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := connectDBWithRetry(ctx, cfg.DB, log)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Handle the migrate subcommand (uses the same config/pool) and exit.
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		return runMigrate(ctx, pool, os.Args[2:])
	}

	// Apply migrations on startup so a fresh environment is usable immediately.
	if err := applyMigrations(ctx, pool, log); err != nil {
		return err
	}

	rdb, err := cache.Connect(ctx, cfg.Redis)
	if err != nil {
		return err
	}
	defer func() { _ = rdb.Close() }()

	router, err := buildRouter(cfg, log, pool, rdb)
	if err != nil {
		return err
	}

	return serve(ctx, cfg.HTTP, log, router)
}

// buildRouter performs dependency injection: it constructs adapters, domain
// services and handlers, then assembles the HTTP router.
func buildRouter(cfg config.Config, log *slog.Logger, pool *pgxpool.Pool, rdb *redis.Client) (http.Handler, error) {
	cipher, err := crypto.NewCipher(cfg.Crypto.EncryptionKey)
	if err != nil {
		return nil, err
	}

	hasher := crypto.NewPasswordHasher(crypto.DefaultBcryptCost)
	tokens := auth.NewTokenManager(cfg.Auth.AccessSecret, cfg.Auth.RefreshSecret, cfg.Auth.AccessTTL, cfg.Auth.RefreshTTL)
	validator := validate.New()

	// Repositories.
	userRepo := postgres.NewUserRepository(pool)
	refreshRepo := postgres.NewRefreshTokenRepository(pool)
	auditRepo := postgres.NewAuditRepository(pool)
	projectRepo := postgres.NewProjectRepository(pool)
	monitorRepo := postgres.NewMonitorRepository(pool)
	orgPlanRepo := postgres.NewOrgPlanRepository(pool)
	notificationRepo := postgres.NewNotificationRepository(pool)
	maintenanceRepo := postgres.NewMaintenanceRepository(pool)
	statusPageRepo := postgres.NewStatusPageRepository(pool)
	heartbeatRepo := postgres.NewHeartbeatRepository(pool)
	statusPageSettingsRepo := postgres.NewStatusPageSettingsRepository(pool)

	// Cross-cutting.
	auditRec := audit.NewRecorder(auditRepo)
	// The API enqueues control-plane syncs; the worker performs them.
	syncEnqueuer := queue.NewSyncEnqueuer(queue.NewQueue(rdb, queue.DefaultStream))

	// Notification wiring: a registry of per-type notifiers, the CRUD service,
	// and the dispatcher used by the Alertmanager webhook.
	//
	// Slack and Webhook fetch a TENANT-supplied URL, so they share one SSRF-guarded
	// HTTP client (safehttp) that refuses internal/loopback/metadata addresses.
	// AllowPrivate/AllowHTTP are off by default and only flipped by a single-tenant
	// operator who deliberately wants internal webhooks.
	tenantHTTP := safehttp.New(safehttp.Config{
		AllowPrivate: cfg.Notify.WebhookAllowPrivate,
		AllowHTTP:    cfg.Notify.WebhookAllowHTTP,
	})
	notifierRegistry := map[notification.ChannelType]notification.Notifier{
		notification.TypeTelegram: notifier.NewTelegramNotifier(),
		notification.TypeSlack:    notifier.NewSlackNotifier(tenantHTTP),
		notification.TypeEmail:    notifier.NewEmailNotifier(),
		notification.TypeWebhook:  notifier.NewWebhookNotifier(tenantHTTP),
	}
	projectLookup := postgres.NewProjectLookupAdapter(projectRepo)
	notifySvc := notification.NewService(notificationRepo, cipher, notifierRegistry, auditRec, cfg.Notify.DashboardURL)
	// Maintenance windows both CRUD (below) and suppress alerts: the same service
	// is the Dispatcher's suppression checker, so planned downtime never pages.
	maintenanceSvc := maintenance.NewService(maintenanceRepo, auditRec)

	// Optional AI alert enrichment: when enabled, firing alerts are triaged by an
	// LLM (assessed severity + likely cause + suggested fix) before delivery.
	var analyzer notification.Analyzer
	if cfg.AI.Enabled {
		analyzer = ai.NewOllamaAnalyzer(cfg.AI.BaseURL, cfg.AI.Model, cfg.AI.APIKey, cfg.AI.Timeout)
		log.Info("AI alert enrichment enabled",
			slog.String("endpoint", cfg.AI.BaseURL), slog.String("model", cfg.AI.Model))
	}
	dispatcher := notification.NewDispatcher(notificationRepo, cipher, notifierRegistry, projectLookup, auditRec, maintenanceSvc, cfg.Notify.DashboardURL, analyzer, cfg.AI.Timeout)

	// Tenant-scoped insight reads over Prometheus.
	insightQuerier := promapi.NewInsightQuerier(promapi.New(cfg.CtrlPlane.PromQueryURL))
	insightSvc := insight.NewService(insightQuerier, monitorRepo)
	// Billing: Stripe-backed subscriptions + pay-as-you-go credit. When Stripe is
	// unconfigured the service still serves the read-only overview; checkout is
	// refused with a clear error.
	billingRepo := postgres.NewBillingRepository(pool)
	var payments billing.Payments
	var stripeWebhook rest.StripeWebhook
	if cfg.Billing.Enabled() {
		stripeClient := stripeadapter.New(stripeadapter.Config{
			SecretKey:     cfg.Billing.StripeSecretKey,
			PriceStarter:  cfg.Billing.PriceStarter,
			PricePro:      cfg.Billing.PricePro,
			SuccessURL:    cfg.Billing.SuccessURL,
			CancelURL:     cfg.Billing.CancelURL,
			WebhookSecret: cfg.Billing.StripeWebhookSecret,
		})
		payments = stripeClient
		stripeWebhook = stripeClient
		log.Info("Stripe billing enabled")
	}
	billingSvc := billing.NewService(billingRepo, payments, auditRec, cfg.Billing.MonitorHoursPerDollar)

	// AI diagnosis. The prober is what actually measures anything, so it is built
	// whenever the feature is on; the explainer is optional and a nil one degrades to
	// returning the measurements without prose.
	var diagnoseSvc *diagnose.Service
	if cfg.AI.Enabled {
		var explainer diagnose.Explainer
		if cfg.AI.BaseURL != "" && cfg.AI.Model != "" {
			explainer = ai.NewOllamaAnalyzer(cfg.AI.BaseURL, cfg.AI.Model, cfg.AI.APIKey, cfg.AI.Timeout)
		}
		diagnoseSvc = diagnose.NewService(
			monitorRepo,
			orgPlanRepo,
			netprobe.New(cfg.AI.DiagnoseAllowPrivate),
			explainer,
		)
		log.Info("AI diagnosis enabled", slog.Bool("allow_private_targets", cfg.AI.DiagnoseAllowPrivate))
	}

	// Services.
	authSvc := auth.NewService(userRepo, refreshRepo, tokens, hasher, auditRec)
	projectSvc := project.NewService(projectRepo, syncEnqueuer, auditRec)
	monitorSvc := monitor.NewService(monitorRepo, syncEnqueuer, orgPlanRepo, auditRec)
	// Public status page: the one unauthenticated read. Takes no auditor and no
	// enqueuer — it is read-only and cannot mutate anything by construction.
	statusPageSvc := statuspage.NewService(statusPageRepo)
	heartbeatSvc := heartbeat.NewService(heartbeatRepo)
	statusPageSettingsSvc := statuspage.NewSettingsService(statusPageSettingsRepo, auditRec)

	// Transport.
	m := metrics.New()
	authn := middleware.NewAuthenticator(tokens)

	// Nil handler when diagnosis is off, so the route is absent rather than present
	// and broken.
	var diagnoseHandler *rest.DiagnoseHandler
	if diagnoseSvc != nil {
		diagnoseHandler = rest.NewDiagnoseHandler(diagnoseSvc, authn)
	}

	health := rest.NewHealthHandler(version, time.Now(),
		rest.Checker{Name: "postgres", Check: func(ctx context.Context) error { return pool.Ping(ctx) }},
		rest.Checker{Name: "redis", Check: func(ctx context.Context) error { return rdb.Ping(ctx).Err() }},
	)

	return rest.NewRouter(rest.RouterDeps{
		Logger:             log,
		Metrics:            m,
		CORSOrigins:        cfg.HTTP.CORSOrigins,
		Authenticator:      authn,
		Health:             health,
		Auth:               rest.NewAuthHandler(authSvc, validator, cfg.IsProduction()),
		Project:            rest.NewProjectHandler(projectSvc, validator, authn),
		Monitor:            rest.NewMonitorHandler(monitorSvc, insightSvc, maintenanceSvc, validator, authn),
		Notification:       rest.NewNotificationHandler(notifySvc, validator, authn),
		Maintenance:        rest.NewMaintenanceHandler(maintenanceSvc, validator, authn),
		Alert:              rest.NewAlertHandler(dispatcher, cfg.Notify.WebhookToken),
		Insight:            rest.NewInsightHandler(insightSvc, maintenanceSvc),
		Billing:            rest.NewBillingHandler(billingSvc, stripeWebhook, validator, authn),
		StatusPage:         rest.NewStatusPageHandler(statusPageSvc),
		Heartbeat:          rest.NewHeartbeatHandler(heartbeatSvc),
		StatusPageSettings: rest.NewStatusPageSettingsHandler(statusPageSettingsSvc, validator, authn),
		Diagnose:           diagnoseHandler,
	}), nil
}

// serve runs the HTTP server and shuts it down gracefully when ctx is cancelled.
func serve(ctx context.Context, cfg config.HTTP, log *slog.Logger, handler http.Handler) error {
	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("api server listening", slog.String("addr", cfg.Addr), slog.String("version", version))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutdown signal received; draining connections")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}
		log.Info("server stopped cleanly")
		return nil
	}
}

// ---- database bootstrap ----

func connectDBWithRetry(ctx context.Context, cfg config.DB, log *slog.Logger) (*pgxpool.Pool, error) {
	const maxAttempts = 15
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		pool, err := database.Connect(ctx, cfg)
		if err == nil {
			return pool, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		log.Warn("database not ready; retrying",
			slog.Int("attempt", attempt), slog.String("error", err.Error()))
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return nil, fmt.Errorf("could not connect to database after %d attempts: %w", maxAttempts, lastErr)
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) error {
	migrator, err := database.NewMigrator(pool, migrations.FS)
	if err != nil {
		return err
	}
	applied, err := migrator.Up(ctx)
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	if len(applied) > 0 {
		log.Info("applied migrations", slog.Any("versions", applied))
	} else {
		log.Info("database schema up to date")
	}
	return nil
}

func runMigrate(ctx context.Context, pool *pgxpool.Pool, args []string) error {
	migrator, err := database.NewMigrator(pool, migrations.FS)
	if err != nil {
		return err
	}
	sub := "up"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "up":
		applied, err := migrator.Up(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("applied %d migration(s): %v\n", len(applied), applied)
		return nil
	case "status":
		statuses, err := migrator.Status(ctx)
		if err != nil {
			return err
		}
		for _, s := range statuses {
			mark := "pending"
			if s.Applied {
				mark = "applied"
			}
			fmt.Printf("  %04d  %-8s  %s\n", s.Version, mark, s.Name)
		}
		return nil
	default:
		return fmt.Errorf("unknown migrate subcommand %q (want up|status)", sub)
	}
}
