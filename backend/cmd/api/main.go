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
	"beacon/internal/adapter/apns"
	"beacon/internal/adapter/appleauth"
	"beacon/internal/adapter/googleauth"
	"beacon/internal/adapter/netprobe"
	"beacon/internal/adapter/notifier"
	"beacon/internal/adapter/oidcclient"
	"beacon/internal/adapter/postgres"
	"beacon/internal/adapter/promapi"
	"beacon/internal/adapter/queue"
	stripeadapter "beacon/internal/adapter/stripe"
	"beacon/internal/config"
	"beacon/internal/domain/apikey"
	"beacon/internal/domain/audit"
	"beacon/internal/domain/auth"
	"beacon/internal/domain/billing"
	"beacon/internal/domain/configsync"
	"beacon/internal/domain/device"
	"beacon/internal/domain/diagnose"
	"beacon/internal/domain/heartbeat"
	"beacon/internal/domain/insight"
	"beacon/internal/domain/maintenance"
	"beacon/internal/domain/monitor"
	"beacon/internal/domain/notification"
	"beacon/internal/domain/plan"
	"beacon/internal/domain/project"
	"beacon/internal/domain/settings"
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
	settingsRepo := postgres.NewSettingsRepository(pool)
	deviceRepo := postgres.NewDeviceRepository(pool)

	// Cross-cutting.
	auditRec := audit.NewRecorder(auditRepo)

	// Platform settings: operator-tunable pricing, limits and premium access. Apply
	// the env baseline first (so a fresh install seeds the DB from it), then overlay
	// whatever has been saved into the plan package's live snapshot that enforcement
	// and the pricing UI read. A load failure is non-fatal: built-in defaults apply.
	settingsSvc := settings.NewService(settingsRepo, auditRec, cfg.PlatformAdminEmails)
	// Platform operators get Pro for their own org automatically — held separately from
	// the DB config so a settings reload never clears it.
	plan.SetAdmins(cfg.PlatformAdminEmails)
	base := plan.DefaultConfig()
	base.HoursPerDollar = cfg.Billing.MonitorHoursPerDollar
	plan.Apply(base)
	if err := settingsSvc.Reload(context.Background()); err != nil {
		log.Warn("platform settings load failed; using built-in defaults", slog.Any("err", err))
	}
	// Re-read periodically so a settings edit made on ANOTHER api replica (each only
	// reloads on its own write) reaches this one. Cheap: one row read every 30s. The
	// goroutine lives for the process; a SIGTERM exit tears it down.
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for range t.C {
			if err := settingsSvc.Reload(context.Background()); err != nil {
				log.Warn("periodic platform settings reload failed", slog.Any("err", err))
			}
		}
	}()
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
		notification.TypeSlack:    notifier.NewSlackNotifier(tenantHTTP, cfg.Notify.BrandName),
		notification.TypeEmail:    notifier.NewEmailNotifier(cfg.Notify.BrandName),
		notification.TypeWebhook:  notifier.NewWebhookNotifier(tenantHTTP),
	}
	// Apple Push (APNs): registered only when the platform signing key is set, so
	// a deployment without an iOS app behaves exactly as before. The signing key is
	// platform-wide (one first-party app); destinations are the org's device tokens.
	if cfg.Push.Enabled() {
		key, err := apns.ParseP8Key([]byte(cfg.Push.APNsKeyP8))
		if err != nil {
			return nil, fmt.Errorf("apple push: %w", err)
		}
		apnsClient, err := apns.New(apns.Config{
			PrivateKey: key,
			KeyID:      cfg.Push.APNsKeyID,
			TeamID:     cfg.Push.APNsTeamID,
			Topic:      cfg.Push.APNsTopic,
			Production: cfg.Push.APNsProduction,
		})
		if err != nil {
			return nil, fmt.Errorf("apple push: %w", err)
		}
		notifierRegistry[notification.TypeAPNs] = notifier.NewAPNsNotifier(apnsClient, deviceRepo)
		log.Info("apple push (APNs) enabled",
			slog.String("topic", cfg.Push.APNsTopic), slog.Bool("production", cfg.Push.APNsProduction))
	}
	projectLookup := postgres.NewProjectLookupAdapter(projectRepo)
	notifySvc := notification.NewService(notificationRepo, cipher, notifierRegistry, auditRec, cfg.Notify.DashboardURL)
	// Device registration auto-enables the org's Apple Push channel via notifySvc;
	// a nil activator would just skip that convenience.
	deviceSvc := device.NewService(deviceRepo, notifySvc)
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
	// Default email fallback: when an org has configured no notification channel,
	// a firing/resolved alert is emailed to its owners and admins over the platform
	// SMTP relay instead of being silently dropped. Off unless a relay is set
	// (BEACON_DEFAULT_SMTP_*), the same graceful-degradation pattern as APNs/AI. A
	// partial config (host or from, not both) is a likely mistake: warn loudly and
	// leave it off rather than half-enable it.
	var fallbackNotifier notification.FallbackNotifier
	switch de := cfg.Notify.DefaultEmail; {
	case de.Enabled():
		fallbackNotifier = notifier.NewDefaultEmailNotifier(notifier.DefaultEmailConfig{
			Host:     de.Host,
			Port:     de.Port,
			From:     de.From,
			Username: de.Username,
			Password: de.Password,
			Security: de.Security,
		}, userRepo, cfg.Notify.BrandName)
		log.Info("default email fallback enabled",
			slog.String("smtp_host", de.Host), slog.String("from", de.From))
	case de.Host != "" || de.From != "":
		log.Warn("default email fallback partially configured — set BOTH BEACON_DEFAULT_SMTP_HOST and BEACON_DEFAULT_SMTP_FROM; fallback disabled",
			slog.Bool("host_set", de.Host != ""), slog.Bool("from_set", de.From != ""))
	}
	dispatcher := notification.NewDispatcher(notificationRepo, cipher, notifierRegistry, projectLookup, auditRec, maintenanceSvc, cfg.Notify.DashboardURL, analyzer, cfg.AI.Timeout, fallbackNotifier)

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
	billingSvc := billing.NewService(billingRepo, payments, auditRec)

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
			billingRepo, // owns credit_seconds, so it owns the charge
			cfg.AI.DiagnoseCostSeconds,
		)
		log.Info("AI diagnosis enabled", slog.Bool("allow_private_targets", cfg.AI.DiagnoseAllowPrivate))
	}

	// Services.
	authSvc := auth.NewService(userRepo, refreshRepo, tokens, hasher, auditRec).
		WithEmailPolicy(auth.NewEmailPolicy(cfg.RequireReachableSignupEmail))
	// "Sign in with Google" is enabled only when at least one client id is configured;
	// otherwise the endpoint 403s and the frontend hides the button.
	if cfg.Google.Enabled() {
		authSvc = authSvc.WithGoogle(googleauth.New(cfg.Google.ClientIDs))
		log.Info("google sign-in enabled", "client_ids", len(cfg.Google.ClientIDs))
	}
	// "Sign in with Apple" (OpenID Connect). Enabled only when a bundle-id audience
	// is configured; the endpoint otherwise 403s and the app hides the button.
	if cfg.Apple.Enabled() {
		authSvc = authSvc.WithApple(appleauth.New(cfg.Apple.ClientIDs))
		log.Info("apple sign-in enabled", "client_ids", len(cfg.Apple.ClientIDs))
	}
	// "Sign in with <provider>" via generic OIDC (OpsAPI by default). Enabled only
	// when the client credentials + endpoint URLs are configured; the routes are
	// otherwise absent and the frontend hides the button.
	var ssoHandler *rest.SSOHandler
	if cfg.OIDC.Enabled() {
		oidcCli := oidcclient.New(oidcclient.Config{
			ClientID:     cfg.OIDC.ClientID,
			ClientSecret: cfg.OIDC.ClientSecret,
			AuthorizeURL: cfg.OIDC.AuthorizeURL,
			TokenURL:     cfg.OIDC.TokenURL,
			UserInfoURL:  cfg.OIDC.UserInfoURL,
			RedirectURL:  cfg.OIDC.RedirectURL,
			Scopes:       cfg.OIDC.Scopes,
		})
		ssoHandler = rest.NewSSOHandler(authSvc, oidcCli, rdb, cfg.OIDC.Provider, cfg.OIDC.PostLoginURL, cfg.IsProduction())
		log.Info("OIDC sign-in enabled", "provider", cfg.OIDC.Provider)
	}
	projectSvc := project.NewService(projectRepo, syncEnqueuer, auditRec)
	// Tenants choose monitor targets, and Blackbox probes them from inside the cluster,
	// so where a target may point is a tenant-facing security boundary. Same address
	// policy as tenant webhooks and AI diagnosis — one block list, one place.
	monitorSvc := monitor.NewService(monitorRepo, syncEnqueuer, orgPlanRepo, auditRec).
		WithTargetGuard(safehttp.NewGuard(cfg.AllowPrivateMonitorTargets))
	// Public status page: the one unauthenticated read. Takes no auditor and no
	// enqueuer — it is read-only and cannot mutate anything by construction.
	statusPageSvc := statuspage.NewService(statusPageRepo)
	heartbeatSvc := heartbeat.NewService(heartbeatRepo)
	statusPageSettingsSvc := statuspage.NewSettingsService(statusPageSettingsRepo, auditRec)

	// Transport.
	m := metrics.New()
	// API keys authenticate as the same Principal a session does, so every endpoint
	// below inherits org scoping, plan limits and billing without a second code path.
	apiKeyRepo := postgres.NewAPIKeyRepository(pool)
	apiKeySvc := apikey.NewService(apiKeyRepo, auditRec)
	authn := middleware.NewAuthenticator(tokens).WithKeys(apiKeySvc)
	syncSvc := configsync.NewService(monitorSvc, projectSvc)

	// Nil handler when diagnosis is off, so the route is absent rather than present
	// and broken.
	var diagnoseHandler *rest.DiagnoseHandler
	if diagnoseSvc != nil {
		diagnoseHandler = rest.NewDiagnoseHandler(diagnoseSvc, authn)
	}

	health := rest.NewHealthHandler(version, string(cfg.Env), time.Now(),
		rest.Checker{Name: "postgres", Check: func(ctx context.Context) error { return pool.Ping(ctx) }},
		rest.Checker{Name: "redis", Check: func(ctx context.Context) error { return rdb.Ping(ctx).Err() }},
	)

	return rest.NewRouter(rest.RouterDeps{
		Logger:             log,
		Metrics:            m,
		CORSOrigins:        cfg.HTTP.CORSOrigins,
		Authenticator:      authn,
		Health:             health,
		Auth:               rest.NewAuthHandler(authSvc, validator, cfg.IsProduction(), cfg.PlatformAdminEmails),
		SSO:                ssoHandler,
		Project:            rest.NewProjectHandler(projectSvc, validator, authn),
		Monitor:            rest.NewMonitorHandler(monitorSvc, insightSvc, maintenanceSvc, validator, authn),
		Notification:       rest.NewNotificationHandler(notifySvc, validator, authn),
		Maintenance:        rest.NewMaintenanceHandler(maintenanceSvc, validator, authn),
		Alert:              rest.NewAlertHandler(dispatcher, cfg.Notify.WebhookToken),
		Insight:            rest.NewInsightHandler(insightSvc, maintenanceSvc),
		Billing:            rest.NewBillingHandler(billingSvc, stripeWebhook, validator, authn, cfg.AI.DiagnoseCostSeconds),
		StatusPage:         rest.NewStatusPageHandler(statusPageSvc),
		Heartbeat:          rest.NewHeartbeatHandler(heartbeatSvc),
		StatusPageSettings: rest.NewStatusPageSettingsHandler(statusPageSettingsSvc, validator, authn),
		Settings:           rest.NewSettingsHandler(settingsSvc, userRepo, validator, authn),
		Diagnose:           diagnoseHandler,
		APIKey:             rest.NewAPIKeyHandler(apiKeySvc, validator, authn),
		Sync:               rest.NewSyncHandler(syncSvc, validator, authn, rest.SyncLimiter()),
		Device:             rest.NewDeviceHandler(deviceSvc, validator, authn),
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
