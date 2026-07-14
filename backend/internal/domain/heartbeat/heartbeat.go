// Package heartbeat is the bounded context for the PUSH side of heartbeat
// monitors: recording an inbound ping.
//
// It is deliberately tiny and separate from the monitor CRUD context, because it
// is reached over an UNAUTHENTICATED endpoint (the ping URL's token is the only
// credential). Keeping it minimal means the unauth surface has almost no code to
// audit: look up a monitor by its capability token, stamp the ping time. Nothing
// here can read another tenant's data or mutate anything but a single last_ping_at.
package heartbeat

import (
	"context"
	"time"

	"beacon/internal/platform/apperror"
)

// Repository records a heartbeat ping.
type Repository interface {
	// RecordPing stamps last_ping_at = at on the (non-deleted) heartbeat whose
	// ping_token matches. It returns true if a heartbeat was matched, false if the
	// token is unknown. It must be a single indexed statement — this is the hot,
	// unauthenticated path.
	RecordPing(ctx context.Context, token string, at time.Time) (matched bool, err error)
}

// Service records pings.
type Service struct {
	repo Repository
	now  func() time.Time
}

// NewService builds a heartbeat Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// Ping records a successful heartbeat for the given token.
//
// An unknown token returns NotFound — the SAME error a caller gets for any missing
// resource, so the endpoint is not an oracle for which tokens exist. (The token
// space is 256-bit, so guessing is infeasible regardless.)
func (s *Service) Ping(ctx context.Context, token string) error {
	if token == "" {
		return apperror.NotFound("unknown ping token")
	}
	matched, err := s.repo.RecordPing(ctx, token, s.now().UTC())
	if err != nil {
		return err
	}
	if !matched {
		return apperror.NotFound("unknown ping token")
	}
	return nil
}
