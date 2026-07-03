package audit

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"beacon/internal/platform/httpx"
	"beacon/internal/platform/logger"
)

// recorder is the default Recorder. It enriches entries with request metadata
// from the context, persists them via the repository, and logs (never returns)
// failures so audit problems can never break a user-facing operation.
type recorder struct {
	repo Repository
	now  func() time.Time
}

// NewRecorder builds the default best-effort Recorder.
func NewRecorder(repo Repository) Recorder {
	return &recorder{repo: repo, now: time.Now}
}

func (r *recorder) Record(ctx context.Context, e Entry) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = r.now().UTC()
	}
	if e.RequestID == "" {
		e.RequestID = httpx.RequestIDFromContext(ctx)
	}
	if e.Metadata == nil {
		e.Metadata = map[string]any{}
	}

	if err := r.repo.Insert(ctx, &e); err != nil {
		logger.FromContext(ctx).Error("failed to write audit log",
			slog.String("action", string(e.Action)),
			slog.String("resource_type", e.ResourceType),
			slog.String("error", err.Error()),
		)
		return err
	}
	return nil
}
