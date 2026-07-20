// Package apikey issues and verifies the credentials machines use to call Beacon.
//
// The whole design rests on one decision: a key is an OPAQUE SECRET that resolves to
// an organization, and nothing more. It does not carry the holder's plan, credit or
// limits, because those are not facts about a key — they are facts about an org at a
// moment, and pay-as-you-go credit changes every minute. A key that claimed "pro" or
// "$5 left" would be a snapshot that starts lying immediately, would keep granting a
// tier after a downgrade, and could not be revoked without a blocklist — which is a
// lookup, which is the thing embedding was supposed to avoid.
//
// So authentication resolves org_id, and every existing rule then applies unchanged:
// the monitor service already reads the org's effective plan on create, billing
// already meters per org. A key inherits all of it without a line of duplicated logic,
// which is also why an API key and a dashboard session are indistinguishable
// downstream — they produce the same principal.
package apikey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/audit"
	"beacon/internal/domain/auth"
	"beacon/internal/platform/apperror"
)

// Prefix marks a Beacon API key. It is deliberately recognisable: a distinctive,
// greppable prefix is what lets secret scanners (GitHub's included) spot one committed
// to a repository and what lets a user identify a stray string in a log as ours.
const Prefix = "bp_"

// secretBytes is the entropy behind each key. 32 bytes is 256 bits — far past any
// brute-force, which is what justifies a fast hash on the verify path.
const secretBytes = 32

// displayPrefixLen is how much of the key is kept in clear for the UI. Enough to tell
// four keys apart in a list, nowhere near enough to be useful to anyone who reads it.
const displayPrefixLen = 12

// Key is a stored key. The secret itself is never in here — it exists exactly once, in
// the response to the call that created it.
type Key struct {
	ID         uuid.UUID  `json:"id"`
	OrgID      uuid.UUID  `json:"-"`
	// CreatedBy is who minted it, kept for the audit question "who added this key?".
	// Nullable because a user can be removed while their key legitimately keeps
	// working — see the FK's ON DELETE SET NULL.
	CreatedBy  *uuid.UUID `json:"-"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Role       auth.Role  `json:"role"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// Active reports whether the key may authenticate right now.
func (k Key) Active(now time.Time) bool {
	if k.RevokedAt != nil {
		return false
	}
	if k.ExpiresAt != nil && now.After(*k.ExpiresAt) {
		return false
	}
	return true
}

// Actor is the authenticated caller managing keys — always a human on a session, never
// a key. See Service.Issue.
type Actor struct {
	UserID uuid.UUID
	OrgID  uuid.UUID
	Role   auth.Role
}

// Repository persists keys.
type Repository interface {
	Create(ctx context.Context, k *Key, hash string) error
	// ByHash resolves a presented key. It must be a single indexed lookup on the
	// hash; anything that scans turns every API request into a table scan.
	ByHash(ctx context.Context, hash string) (*Key, error)
	List(ctx context.Context, orgID uuid.UUID) ([]Key, error)
	Revoke(ctx context.Context, orgID, id uuid.UUID, at time.Time) error
	// TouchLastUsed records that a key authenticated. Implementations should write
	// coarsely — see the repository for why this must not be a write per request.
	TouchLastUsed(ctx context.Context, id uuid.UUID, at time.Time) error
}

// Service issues, lists, revokes and verifies keys.
type Service struct {
	repo     Repository
	auditlog audit.Recorder
	now      func() time.Time
}

func NewService(repo Repository, auditlog audit.Recorder) *Service {
	return &Service{repo: repo, auditlog: auditlog, now: time.Now}
}

// Generate returns a new key and the hash to store for it.
//
// The returned secret is the only copy that will ever exist. It is shown to the user
// once and then deliberately unrecoverable — a key we could read back is a key an
// attacker with database access could read back too.
func Generate() (secret, hash, prefix string, err error) {
	buf := make([]byte, secretBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", "", fmt.Errorf("generate api key: %w", err)
	}
	// URL-safe and unpadded so the key is one word: no '+', '/' or '=' to be mangled
	// by a shell, a URL, or a CI secret store.
	secret = Prefix + base64.RawURLEncoding.EncodeToString(buf)
	return secret, HashOf(secret), secret[:displayPrefixLen], nil
}

// HashOf is the stored representation of a key. Also used on the verify path, so the
// two can never disagree about what "the hash of this key" means.
func HashOf(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// Looks reports whether a credential is shaped like one of ours. Used to route a
// bearer token to key verification instead of JWT parsing — a cheap prefix test, so a
// mistyped session token is not run through a database lookup.
func Looks(token string) bool { return strings.HasPrefix(token, Prefix) }

// CreateInput describes a key to issue.
type CreateInput struct {
	Name string
	// Role the key will hold. Empty means the creator's own role.
	Role auth.Role
	// ExpiresAt is optional; nil means the key does not expire.
	ExpiresAt *time.Time
}

// Issue mints a key and returns the secret, which the caller must surface exactly once.
//
// Only a human session can do this, never another API key. A key that can mint keys
// turns a single leaked credential into permanent, self-renewing access that survives
// revoking the original — so the escalation is cut off at the source rather than
// managed. The transport enforces it; this comment is why.
func (s *Service) Issue(ctx context.Context, actor Actor, in CreateInput) (*Key, string, error) {
	if !actor.Role.CanAdminister() {
		return nil, "", apperror.Forbidden("only owners and admins can create API keys")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, "", apperror.Validation("a name is required",
			apperror.FieldError{Field: "name", Message: "name the key after what will use it, e.g. \"github-actions\""})
	}

	role := in.Role
	if role == "" {
		role = actor.Role
	}
	// A key may never out-rank the person who minted it. Without this, an admin could
	// mint an owner key and quietly promote themselves — the key becomes a privilege
	// escalation dressed as a convenience.
	if rank(role) > rank(actor.Role) {
		return nil, "", apperror.Validation("a key cannot have more access than you do",
			apperror.FieldError{Field: "role", Message: "choose your own role or a lesser one"})
	}
	if in.ExpiresAt != nil && !in.ExpiresAt.After(s.now()) {
		return nil, "", apperror.Validation("expiry must be in the future",
			apperror.FieldError{Field: "expires_at", Message: "a key that has already expired cannot be used"})
	}

	secret, hash, prefix, err := Generate()
	if err != nil {
		return nil, "", apperror.Internal(err)
	}
	k := &Key{
		ID:        uuid.New(),
		OrgID:     actor.OrgID,
		CreatedBy: &actor.UserID,
		Name:      name,
		Prefix:    prefix,
		Role:      role,
		CreatedAt: s.now().UTC(),
		ExpiresAt: in.ExpiresAt,
	}
	if err := s.repo.Create(ctx, k, hash); err != nil {
		return nil, "", err
	}

	org := actor.OrgID
	_ = s.auditlog.Record(ctx, audit.Entry{
		OrgID: &org, UserID: &actor.UserID, Action: audit.ActionAPIKeyCreated,
		ResourceType: "api_key", ResourceID: k.ID.String(),
		Metadata: map[string]any{"name": name, "role": string(role), "prefix": prefix},
	})
	return k, secret, nil
}

// List returns the org's keys. Secrets are not stored, so nothing sensitive is here.
func (s *Service) List(ctx context.Context, actor Actor) ([]Key, error) {
	return s.repo.List(ctx, actor.OrgID)
}

// Revoke withdraws a key immediately.
func (s *Service) Revoke(ctx context.Context, actor Actor, id uuid.UUID) error {
	if !actor.Role.CanAdminister() {
		return apperror.Forbidden("only owners and admins can revoke API keys")
	}
	if err := s.repo.Revoke(ctx, actor.OrgID, id, s.now().UTC()); err != nil {
		return err
	}
	org := actor.OrgID
	_ = s.auditlog.Record(ctx, audit.Entry{
		OrgID: &org, UserID: &actor.UserID, Action: audit.ActionAPIKeyRevoked,
		ResourceType: "api_key", ResourceID: id.String(),
	})
	return nil
}

// Verified is what a valid key resolves to.
type Verified struct {
	KeyID uuid.UUID
	OrgID uuid.UUID
	Role  auth.Role
}

// Verify resolves a presented key, or fails.
//
// Every failure returns the same error on purpose. Distinguishing "no such key" from
// "revoked" from "expired" would let anyone holding a random string learn whether it
// was ever real, and would tell an attacker with a stolen-but-revoked key that they
// had the right format and merely need a fresher one.
func (s *Service) Verify(ctx context.Context, secret string) (*Verified, error) {
	fail := apperror.Unauthorized("invalid API key")
	if !Looks(secret) {
		return nil, fail
	}
	k, err := s.repo.ByHash(ctx, HashOf(secret))
	if err != nil || k == nil {
		return nil, fail
	}
	if !k.Active(s.now()) {
		return nil, fail
	}
	// No constant-time comparison, and that is not an oversight. Nothing is compared
	// here: the presented key is hashed and the hash is looked up on a unique index,
	// so there is no secret-dependent branch to time. Timing could at most reveal
	// whether a hash exists, and a hash is useless without its preimage — which is
	// precisely why keys are stored this way rather than compared.

	// Best-effort and deliberately not fatal: "when was this key last used" is an
	// operational nicety, and failing an otherwise-valid request because a bookkeeping
	// write failed would trade a working API for a timestamp.
	_ = s.repo.TouchLastUsed(ctx, k.ID, s.now().UTC())

	return &Verified{KeyID: k.ID, OrgID: k.OrgID, Role: k.Role}, nil
}

// rank orders roles for the capping check. Kept local rather than exported from auth:
// it exists only to answer "is this role stronger than that one", and a general
// comparison invites use where the two roles are not actually comparable.
func rank(r auth.Role) int {
	switch r {
	case auth.RoleOwner:
		return 3
	case auth.RoleAdmin:
		return 2
	case auth.RoleMember:
		return 1
	default: // viewer, and anything unrecognised
		return 0
	}
}
