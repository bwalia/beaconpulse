// Command worker runs Beacon's background processor. It performs two jobs:
//  1. Consumes control-plane sync jobs from the Redis queue and reconciles
//     Prometheus/Blackbox with the database (regenerate config + hot reload).
//  2. Runs periodic maintenance: a safety-net full resync and expired
//     refresh-token cleanup.
//
// The worker is crash-resilient: unacknowledged jobs are re-delivered, and the
// periodic resync guarantees the control plane converges even if a job is lost.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/adapter/controlplane"
	"beacon/internal/adapter/postgres"
	"beacon/internal/adapter/promapi"
	"beacon/internal/adapter/queue"
	"beacon/internal/config"
	"beacon/internal/platform/cache"
	"beacon/internal/platform/database"
	"beacon/internal/platform/logger"
	"beacon/internal/worker"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker fatal", slog.String("error", err.Error()))
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := connectDBWithRetry(ctx, cfg.DB, log)
	if err != nil {
		return err
	}
	defer pool.Close()

	rdb, err := cache.Connect(ctx, cfg.Redis)
	if err != nil {
		return err
	}
	defer func() { _ = rdb.Close() }()

	// Control-plane syncer: reads monitors, regenerates config, reloads services.
	monitorRepo := postgres.NewMonitorRepository(pool)
	refreshRepo := postgres.NewRefreshTokenRepository(pool)
	reloader := controlplane.NewReloader(cfg.CtrlPlane.PromReloadURL, cfg.CtrlPlane.BlackboxReloadURL)
	syncer := controlplane.NewSyncer(
		monitorRepo,
		controlplane.GeneratorConfig{BlackboxAddr: cfg.CtrlPlane.BlackboxAddr, DNSResolver: cfg.CtrlPlane.DNSResolver},
		controlplane.Paths{
			BlackboxConfigFile: cfg.CtrlPlane.BlackboxConfigFile,
			ScrapeFile:         cfg.CtrlPlane.PromScrapeFile,
			RulesFile:          cfg.CtrlPlane.PromRulesFile,
		},
		reloader,
	)

	// Job consumer: reconcile on demand when the API enqueues a sync.
	hostname, _ := os.Hostname()
	consumer := queue.NewConsumer(rdb, queue.ConsumerConfig{
		Stream:     queue.DefaultStream,
		Group:      queue.DefaultGroup,
		Consumer:   "worker-" + hostname,
		MaxRetries: cfg.Worker.MaxRetries,
	}, log)
	consumer.Register(queue.JobControlPlaneSync, func(ctx context.Context, _ queue.Job) error {
		log.Info("reconciling control plane (queued sync)")
		return syncer.Sync(ctx)
	})

	// Status feedback loop: read probe results from Prometheus back into the DB.
	statusSync := worker.NewStatusSync(promapi.New(cfg.CtrlPlane.PromQueryURL), monitorRepo)

	// Periodic maintenance tasks.
	scheduler := worker.NewScheduler(log,
		worker.Task{
			Name:       "controlplane-resync",
			Interval:   2 * time.Minute,
			RunAtStart: true, // converge the control plane immediately on boot
			Run:        syncer.Sync,
		},
		worker.Task{
			Name:     "monitor-status-sync",
			Interval: 30 * time.Second,
			Run:      statusSync.Run,
		},
		worker.Task{
			Name:     "expired-token-cleanup",
			Interval: time.Hour,
			Run: func(ctx context.Context) error {
				n, err := refreshRepo.DeleteExpired(ctx)
				if err == nil && n > 0 {
					log.Info("cleaned up expired refresh tokens", slog.Int64("count", n))
				}
				return err
			},
		},
	)

	log.Info("worker starting",
		slog.Int("max_retries", cfg.Worker.MaxRetries),
		slog.String("blackbox_config", cfg.CtrlPlane.BlackboxConfigFile))

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = consumer.Run(ctx) }()
	go func() { defer wg.Done(); scheduler.Run(ctx) }()
	wg.Wait()

	log.Info("worker stopped cleanly")
	return nil
}

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
	return nil, lastErr
}
