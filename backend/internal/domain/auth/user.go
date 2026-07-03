// Package auth is the authentication and identity bounded context. It owns the
// Organization and User aggregates, credential verification, and JWT/refresh
// token lifecycle. It depends only on repository interfaces defined here — the
// Postgres implementation lives in internal/adapter/postgres — keeping the
// domain free of infrastructure concerns.
package auth

import (
	"time"

	"github.com/google/uuid"
)

// Role is a coarse-grained authorization role. Fine-grained RBAC (teams,
// permissions) builds on top of this in a later module.
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

// Valid reports whether r is a known role.
func (r Role) Valid() bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleMember, RoleViewer:
		return true
	default:
		return false
	}
}

// CanWrite reports whether the role may create/update/delete resources.
func (r Role) CanWrite() bool {
	return r == RoleOwner || r == RoleAdmin || r == RoleMember
}

// CanAdminister reports whether the role may manage users, roles and settings.
func (r Role) CanAdminister() bool {
	return r == RoleOwner || r == RoleAdmin
}

// Organization is the top-level tenant.
type Organization struct {
	ID        uuid.UUID
	Name      string
	Slug      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// User is a member of an organization.
type User struct {
	ID           uuid.UUID
	OrgID        uuid.UUID
	Email        string
	PasswordHash string
	Name         string
	Role         Role
	IsActive     bool
	TwoFAEnabled bool
	LastLoginAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// RefreshToken is a persisted, hashed refresh credential. The plaintext value is
// only ever held in memory and returned to the client once.
type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	UserAgent string
	IP        string
	CreatedAt time.Time
}

// IsUsable reports whether the token is neither expired nor revoked at time t.
func (t *RefreshToken) IsUsable(now time.Time) bool {
	return t.RevokedAt == nil && now.Before(t.ExpiresAt)
}
