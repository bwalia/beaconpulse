// Package device stores the push-notification tokens a user's mobile devices
// register, and turns push on for their organization the first time a device
// enrolls. The tokens are consumed by the apns notifier, which fans an org's
// alerts out to every device its members have registered.
package device

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"beacon/internal/platform/apperror"
	"beacon/internal/platform/logger"
)

// Platform identifies the push transport a token belongs to. Only iOS (APNs)
// delivers today; android is reserved so the schema and API need not change when
// an Android app is added.
type Platform string

const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
)

func (p Platform) valid() bool { return p == PlatformIOS || p == PlatformAndroid }

// maxTokenLen bounds a stored token. An APNs token is 64 hex chars today; the
// ceiling is generous so a future provider's longer token still fits while still
// rejecting an unbounded body. Mirrors the DB CHECK.
const maxTokenLen = 512

// Device is one registered push endpoint, owned by a user within an org.
type Device struct {
	ID       uuid.UUID `json:"id"`
	OrgID    uuid.UUID `json:"-"`
	UserID   uuid.UUID `json:"-"`
	Platform Platform  `json:"platform"`
	// Token is the provider's device token — a credential, never echoed back to
	// the client.
	Token      string    `json:"-"`
	LastSeenAt time.Time `json:"last_seen_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// Actor is the authenticated user registering or removing their own device.
type Actor struct {
	UserID uuid.UUID
	OrgID  uuid.UUID
}

// Repository persists device tokens.
type Repository interface {
	// Upsert registers a token, or refreshes an existing one (matched on the
	// token) in place. Idempotent: re-registering the same token is not an error.
	Upsert(ctx context.Context, d *Device) error
	// DeleteByToken removes one token for an org (sign-out on a device). Idempotent.
	DeleteByToken(ctx context.Context, orgID uuid.UUID, token string) error
}

// TokenStore reads and prunes device tokens for alert fan-out. Consumed by the
// apns notifier; implemented by the same postgres repository as Repository. Kept
// separate so the notifier depends only on what it uses.
type TokenStore interface {
	TokensByOrg(ctx context.Context, orgID uuid.UUID) ([]string, error)
	// Delete removes a token unconditionally, to prune one APNs has reported dead.
	Delete(ctx context.Context, token string) error
}

// PushActivator enables an org's Apple Push channel. Implemented by the
// notification service; an interface so this package does not depend on it.
// Optional — a nil activator simply skips auto-enable.
type PushActivator interface {
	EnsureAPNsChannel(ctx context.Context, orgID uuid.UUID) error
}

// Service registers and removes device tokens.
type Service struct {
	repo      Repository
	activator PushActivator // optional; nil disables auto-enable of the push channel
	now       func() time.Time
}

// NewService wires the device service. activator may be nil.
func NewService(repo Repository, activator PushActivator) *Service {
	return &Service{repo: repo, activator: activator, now: time.Now}
}

// RegisterInput is the validated payload for enrolling a device.
type RegisterInput struct {
	Token    string
	Platform Platform
}

// Register enrolls (or refreshes) a device token for the caller, then ensures the
// org's Apple Push channel is on so alerts start flowing without a separate setup
// step. Auto-enable is best-effort: the device is registered even if turning the
// channel on fails, and it never re-enables a channel the org has deliberately
// switched off (EnsureAPNsChannel only creates a missing one).
func (s *Service) Register(ctx context.Context, actor Actor, in RegisterInput) (*Device, error) {
	token := strings.TrimSpace(in.Token)
	if token == "" {
		return nil, apperror.Validation("a device token is required",
			apperror.FieldError{Field: "token", Message: "is required"})
	}
	if len(token) > maxTokenLen {
		return nil, apperror.Validation("device token is too long",
			apperror.FieldError{Field: "token", Message: "exceeds the maximum length"})
	}
	platform := in.Platform
	if platform == "" {
		platform = PlatformIOS
	}
	if !platform.valid() {
		return nil, apperror.Validation("unsupported device platform",
			apperror.FieldError{Field: "platform", Message: "must be ios or android"})
	}

	now := s.now().UTC()
	d := &Device{
		ID:         uuid.New(),
		OrgID:      actor.OrgID,
		UserID:     actor.UserID,
		Platform:   platform,
		Token:      token,
		LastSeenAt: now,
		CreatedAt:  now,
	}
	if err := s.repo.Upsert(ctx, d); err != nil {
		return nil, err
	}
	if s.activator != nil {
		if err := s.activator.EnsureAPNsChannel(ctx, actor.OrgID); err != nil {
			// Non-fatal: the device is registered; the org just did not get its push
			// channel auto-created this time. Logged so a persistent failure is visible.
			logger.FromContext(ctx).Warn("device: auto-enable apple push channel failed",
				"org_id", actor.OrgID.String(), "error", err.Error())
		}
	}
	return d, nil
}

// Unregister removes a device token for the caller's org (sign-out). Idempotent:
// removing a token that is already gone is success, not an error.
func (s *Service) Unregister(ctx context.Context, actor Actor, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return apperror.Validation("a device token is required",
			apperror.FieldError{Field: "token", Message: "is required"})
	}
	return s.repo.DeleteByToken(ctx, actor.OrgID, token)
}
