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
}

// RefreshTokenRepository persists hashed refresh tokens.
type RefreshTokenRepository interface {
	Create(ctx context.Context, t *RefreshToken) error
	GetByHash(ctx context.Context, hash string) (*RefreshToken, error)
	Revoke(ctx context.Context, id uuid.UUID) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) (int64, error)
}
