// Package device stores the push-notification tokens a user's devices register
// — mobile apps (APNs) and browsers (web push) — and turns push on for their
// organization the first time a device enrolls. The apns and webpush notifiers
// fan an org's alerts out to every device its members have registered.
package device

import (
	"context"
	"encoding/base64"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"beacon/internal/platform/apperror"
	"beacon/internal/platform/logger"
)

// Platform identifies the push transport a token belongs to. iOS (APNs) and web
// (browser push) deliver; android is reserved so the schema and API need not
// change when an Android app is added.
type Platform string

const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
	// PlatformWeb is a browser push subscription: the token is its endpoint URL.
	PlatformWeb Platform = "web"
)

func (p Platform) valid() bool { return p == PlatformIOS || p == PlatformAndroid || p == PlatformWeb }

// maxTokenLen bounds a stored token. An APNs token is 64 hex chars; a web push
// endpoint URL can run to several hundred. Mirrors the DB CHECK.
const maxTokenLen = 2048

// webPushHosts are the push services browsers subscribe through (Chrome, Edge,
// Opera, Samsung → FCM; Firefox → Mozilla; Safari → Apple; legacy Edge → WNS).
// The server POSTs to a registered endpoint, so accepting any URL would let a
// user aim that request at an internal address; only these hosts are allowed.
var webPushHosts = []string{".googleapis.com", ".mozilla.com", ".push.apple.com", ".notify.windows.com"}

// ValidWebPushEndpoint reports whether endpoint is an https URL on a known push
// service. Checked at registration and again before every send.
func ValidWebPushEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Port() != "" || u.User != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, suffix := range webPushHosts {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

// Device is one registered push endpoint, owned by a user within an org.
type Device struct {
	ID       uuid.UUID `json:"id"`
	OrgID    uuid.UUID `json:"-"`
	UserID   uuid.UUID `json:"-"`
	Platform Platform  `json:"platform"`
	// Token is the provider's device token (web: the subscription endpoint) — a
	// credential, never echoed back to the client.
	Token string `json:"-"`
	// P256dh and Auth are a web subscription's encryption keys (base64url).
	P256dh     string    `json:"-"`
	Auth       string    `json:"-"`
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
	// TokensByOrg returns the org's iOS (APNs) tokens.
	TokensByOrg(ctx context.Context, orgID uuid.UUID) ([]string, error)
	// Delete removes a token unconditionally, to prune one APNs has reported dead.
	Delete(ctx context.Context, token string) error
}

// WebSubscription is one browser's push subscription, for the webpush notifier.
type WebSubscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

// WebSubscriptionStore reads and prunes browser subscriptions for alert fan-out.
type WebSubscriptionStore interface {
	WebSubscriptionsByOrg(ctx context.Context, orgID uuid.UUID) ([]WebSubscription, error)
	// Delete removes a subscription by endpoint, once its push service says it's gone.
	Delete(ctx context.Context, token string) error
}

// PushActivator enables an org's push channel for a platform the first time one
// of its devices enrolls. Implemented by the notification service; an interface
// so this package does not depend on it. Optional — nil skips auto-enable.
type PushActivator interface {
	EnsureAPNsChannel(ctx context.Context, orgID uuid.UUID) error
	EnsureWebPushChannel(ctx context.Context, orgID uuid.UUID) error
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
	// P256dh and Auth are required for PlatformWeb (the subscription's keys).
	P256dh string
	Auth   string
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
			apperror.FieldError{Field: "platform", Message: "must be ios, android or web"})
	}
	if platform == PlatformWeb {
		if err := validateWebKeys(token, in.P256dh, in.Auth); err != nil {
			return nil, err
		}
	}

	now := s.now().UTC()
	d := &Device{
		ID:         uuid.New(),
		OrgID:      actor.OrgID,
		UserID:     actor.UserID,
		Platform:   platform,
		Token:      token,
		P256dh:     strings.TrimSpace(in.P256dh),
		Auth:       strings.TrimSpace(in.Auth),
		LastSeenAt: now,
		CreatedAt:  now,
	}
	if err := s.repo.Upsert(ctx, d); err != nil {
		return nil, err
	}
	if s.activator != nil {
		ensure := s.activator.EnsureAPNsChannel
		if platform == PlatformWeb {
			ensure = s.activator.EnsureWebPushChannel
		}
		if err := ensure(ctx, actor.OrgID); err != nil {
			// Non-fatal: the device is registered; the org just did not get its push
			// channel auto-created this time. Logged so a persistent failure is visible.
			logger.FromContext(ctx).Warn("device: auto-enable push channel failed",
				"org_id", actor.OrgID.String(), "platform", string(platform), "error", err.Error())
		}
	}
	return d, nil
}

// validateWebKeys checks a browser subscription: a known push service endpoint,
// an uncompressed P-256 public key and a 16-byte auth secret.
func validateWebKeys(endpoint, p256dh, auth string) error {
	if !ValidWebPushEndpoint(endpoint) {
		return apperror.Validation("unsupported push service",
			apperror.FieldError{Field: "token", Message: "must be a browser push service endpoint"})
	}
	if k, err := decodeB64(p256dh); err != nil || len(k) != 65 || k[0] != 0x04 {
		return apperror.Validation("invalid subscription key",
			apperror.FieldError{Field: "keys.p256dh", Message: "must be an uncompressed P-256 public key"})
	}
	if a, err := decodeB64(auth); err != nil || len(a) != 16 {
		return apperror.Validation("invalid subscription key",
			apperror.FieldError{Field: "keys.auth", Message: "must be a 16-byte secret"})
	}
	return nil
}

func decodeB64(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(strings.TrimSpace(s), "="))
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
