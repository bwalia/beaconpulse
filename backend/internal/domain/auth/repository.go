package auth

import (
	"context"

	"github.com/google/uuid"
)

// UserRepository persists organizations and users. Implementations must enforce
// email uniqueness and surface a conflict via apperror when violated.
type UserRepository interface {
	// CreateOrgAndOwner atomically inserts an organization and its first (owner)
	// user. Both succeed or neither does.
	CreateOrgAndOwner(ctx context.Context, org *Organization, owner *User) error

	GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	TouchLastLogin(ctx context.Context, userID uuid.UUID) error
	SlugExists(ctx context.Context, slug string) (bool, error)

	// LinkGoogleSub attaches a Google subject id to an existing user (first time
	// they sign in with Google using an email that already has an account). A
	// conflict apperror is returned if that subject is already linked elsewhere.
	LinkGoogleSub(ctx context.Context, userID uuid.UUID, googleSub string) error
}

// GoogleIdentity is the verified result of a Google ID token.
type GoogleIdentity struct {
	Subject       string // the stable Google user id ("sub")
	Email         string
	EmailVerified bool
	Name          string
}

// GoogleVerifier validates a Google-issued OIDC ID token (signature against
// Google's public keys, issuer, expiry, and audience) and returns the identity.
// Implemented in internal/adapter/googleauth; a nil verifier disables Google sign-in.
type GoogleVerifier interface {
	Verify(ctx context.Context, idToken string) (*GoogleIdentity, error)
}

// RefreshTokenRepository persists hashed refresh tokens.
type RefreshTokenRepository interface {
	Create(ctx context.Context, t *RefreshToken) error
	GetByHash(ctx context.Context, hash string) (*RefreshToken, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) (int64, error)
}
