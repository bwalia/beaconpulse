// Package worker contains the background-processing building blocks used by the
// worker process: a periodic task scheduler and the job handlers. Keeping this
// logic here (rather than in cmd/worker) makes it unit-testable and reusable.
package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Task is a named unit of periodic work.
type Task struct {
	Name     string
	Interval time.Duration
	// RunAtStart runs the task once immediately when the scheduler starts,
	// before waiting for the first tick.
	RunAtStart bool
	Run        func(ctx context.Context) error
}

// Scheduler runs tasks on independent tickers until its context is cancelled.
// A task that returns an error is logged and retried on its next tick; one
// failing task never blocks the others.
type Scheduler struct {
	log   *slog.Logger
	tasks []Task
}

// NewScheduler builds a Scheduler.
func NewScheduler(log *slog.Logger, tasks ...Task) *Scheduler {
	return &Scheduler{log: log, tasks: tasks}
}

// Run starts every task and blocks until ctx is cancelled, after which it waits
// for in-flight task runs to finish.
func (s *Scheduler) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, t := range s.tasks {
		wg.Add(1)
		go func(t Task) {
			defer wg.Done()
			s.runTask(ctx, t)
		}(t)
	}
	wg.Wait()
}

func (s *Scheduler) runTask(ctx context.Context, t Task) {
	if t.RunAtStart {
		s.execute(ctx, t)
	}
	ticker := time.NewTicker(t.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.execute(ctx, t)
		}
	}
}

func (s *Scheduler) execute(ctx context.Context, t Task) {
	start := time.Now()
	if err := t.Run(ctx); err != nil {
		s.log.Error("scheduled task failed",
			slog.String("task", t.Name),
			slog.String("error", err.Error()),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()))
		return
	}
	s.log.Debug("scheduled task completed",
		slog.String("task", t.Name),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()))
}
