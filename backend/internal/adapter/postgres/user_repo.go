package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/domain/auth"
	"beacon/internal/platform/apperror"
)

// UserRepository implements auth.UserRepository against Postgres.
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository builds a UserRepository.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

var _ auth.UserRepository = (*UserRepository)(nil)

// CreateOrgAndOwner inserts an organization and its owner in a single
// transaction. A duplicate email surfaces as a conflict apperror.
func (r *UserRepository) CreateOrgAndOwner(ctx context.Context, org *auth.Organization, owner *auth.User) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return apperror.Internal(fmt.Errorf("begin tx: %w", err))
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`INSERT INTO organizations (id, name, slug, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		org.ID, org.Name, org.Slug, org.CreatedAt, org.UpdatedAt,
	); err != nil {
		if c, ok := isUniqueViolation(err); ok && c == "ux_organizations_slug" {
			return apperror.Conflict("an organization with that name already exists")
		}
		return apperror.Internal(fmt.Errorf("insert organization: %w", err))
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO users (id, org_id, email, password_hash, name, role, is_active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		owner.ID, owner.OrgID, owner.Email, owner.PasswordHash, owner.Name,
		string(owner.Role), owner.IsActive, owner.CreatedAt, owner.UpdatedAt,
	); err != nil {
		if c, ok := isUniqueViolation(err); ok && c == "ux_users_email" {
			return apperror.Conflict("an account with that email already exists")
		}
		return apperror.Internal(fmt.Errorf("insert owner: %w", err))
	}

	if err := tx.Commit(ctx); err != nil {
		return apperror.Internal(fmt.Errorf("commit: %w", err))
	}
	return nil
}

const userColumns = `id, org_id, email, password_hash, name, role, is_active,
	twofa_enabled, last_login_at, created_at, updated_at`

func scanUser(row pgx.Row) (*auth.User, error) {
	var u auth.User
	var role string
	if err := row.Scan(
		&u.ID, &u.OrgID, &u.Email, &u.PasswordHash, &u.Name, &role, &u.IsActive,
		&u.TwoFAEnabled, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
	); err != nil {
		return nil, err
	}
	u.Role = auth.Role(role)
	return &u, nil
}

// GetUserByID fetches a non-deleted user by id.
func (r *UserRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*auth.User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1 AND deleted_at IS NULL`, id)
	u, err := scanUser(row)
	if err != nil {
		if isNoRows(err) {
			return nil, apperror.NotFound("user not found")
		}
		return nil, apperror.Internal(fmt.Errorf("get user by id: %w", err))
	}
	return u, nil
}

// GetUserByEmail fetches a non-deleted user by lowercased email.
func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*auth.User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE lower(email) = lower($1) AND deleted_at IS NULL`, email)
	u, err := scanUser(row)
	if err != nil {
		if isNoRows(err) {
			return nil, apperror.NotFound("user not found")
		}
		return nil, apperror.Internal(fmt.Errorf("get user by email: %w", err))
	}
	return u, nil
}

// TouchLastLogin updates the user's last_login_at to now.
func (r *UserRepository) TouchLastLogin(ctx context.Context, userID uuid.UUID) error {
	if _, err := r.pool.Exec(ctx,
		`UPDATE users SET last_login_at = now() WHERE id = $1`, userID); err != nil {
		return apperror.Internal(fmt.Errorf("touch last login: %w", err))
	}
	return nil
}

// SlugExists reports whether a non-deleted organization already uses slug.
func (r *UserRepository) SlugExists(ctx context.Context, slug string) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx,
		// Guard the whole public slug namespace: a new org must not take a slug that
		// another org already serves its status page at (org slug OR custom slug).
		`SELECT EXISTS(SELECT 1 FROM organizations
		   WHERE deleted_at IS NULL AND ($1 = slug OR $1 = status_page_slug))`, slug,
	).Scan(&exists); err != nil {
		return false, apperror.Internal(fmt.Errorf("slug exists: %w", err))
	}
	return exists, nil
}
