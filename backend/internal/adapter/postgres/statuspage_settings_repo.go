package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/domain/statuspage"
	"beacon/internal/platform/apperror"
)

// StatusPageSettingsRepository implements statuspage.SettingsRepository — the
// AUTHENTICATED side of the feature: an owner reading or changing whether their
// org publishes a page.
//
// Kept separate from StatusPageRepository (the public read) so the two never get
// confused. One is reachable without a token; the other must never be.
type StatusPageSettingsRepository struct {
	pool *pgxpool.Pool
}

// NewStatusPageSettingsRepository builds a StatusPageSettingsRepository.
func NewStatusPageSettingsRepository(pool *pgxpool.Pool) *StatusPageSettingsRepository {
	return &StatusPageSettingsRepository{pool: pool}
}

var _ statuspage.SettingsRepository = (*StatusPageSettingsRepository)(nil)

// Get returns the org's status-page settings, resolving the effective public slug
// (the custom slug when set, otherwise the org slug).
func (r *StatusPageSettingsRepository) Get(ctx context.Context, orgID uuid.UUID) (*statuspage.Settings, error) {
	var (
		s      statuspage.Settings
		custom *string
	)
	err := r.pool.QueryRow(ctx,
		`SELECT slug, status_page_slug, status_page_enabled, status_page_title, name
		   FROM organizations
		  WHERE id = $1 AND deleted_at IS NULL`, orgID).
		Scan(&s.OrgSlug, &custom, &s.Enabled, &s.Title, &s.OrgName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("organization not found")
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("get status page settings: %w", err))
	}
	if custom != nil {
		s.CustomSlug = *custom
	}
	s.Slug = s.OrgSlug
	if s.CustomSlug != "" {
		s.Slug = s.CustomSlug
	}
	return &s, nil
}

// Update persists the org's status-page settings, including the custom slug
// (NULL when empty). A unique-violation on the slug maps to a clean conflict.
func (r *StatusPageSettingsRepository) Update(ctx context.Context, orgID uuid.UUID, s statuspage.Settings) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE organizations
		    SET status_page_enabled = $2, status_page_title = $3, status_page_slug = NULLIF($4, '')
		  WHERE id = $1 AND deleted_at IS NULL`, orgID, s.Enabled, s.Title, s.CustomSlug)
	if err != nil {
		if _, ok := isUniqueViolation(err); ok {
			return apperror.Conflict("that status page URL is already taken")
		}
		return apperror.Internal(fmt.Errorf("update status page settings: %w", err))
	}
	if ct.RowsAffected() == 0 {
		return apperror.NotFound("organization not found")
	}
	return nil
}

// SlugAvailable reports whether slug is free for orgID to claim — no other live
// org uses it as its org slug or its custom slug.
func (r *StatusPageSettingsRepository) SlugAvailable(ctx context.Context, orgID uuid.UUID, slug string) (bool, error) {
	var taken bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM organizations
			 WHERE deleted_at IS NULL AND id <> $1
			   AND ($2 = slug OR $2 = status_page_slug))`, orgID, slug).Scan(&taken)
	if err != nil {
		return false, apperror.Internal(fmt.Errorf("check status page slug: %w", err))
	}
	return !taken, nil
}

// PublishedCount reports how many monitors are currently published.
//
// The UI uses this to warn that a page is enabled but empty — a live status page
// showing nothing is worse than no page at all.
func (r *StatusPageSettingsRepository) PublishedCount(ctx context.Context, orgID uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM monitors
		  WHERE org_id = $1 AND public AND enabled AND deleted_at IS NULL`, orgID).Scan(&n)
	if err != nil {
		return 0, apperror.Internal(fmt.Errorf("count published monitors: %w", err))
	}
	return n, nil
}
