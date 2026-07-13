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

// Get returns the org's status-page settings.
func (r *StatusPageSettingsRepository) Get(ctx context.Context, orgID uuid.UUID) (*statuspage.Settings, error) {
	var s statuspage.Settings
	err := r.pool.QueryRow(ctx,
		`SELECT slug, status_page_enabled, status_page_title, name
		   FROM organizations
		  WHERE id = $1 AND deleted_at IS NULL`, orgID).
		Scan(&s.Slug, &s.Enabled, &s.Title, &s.OrgName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("organization not found")
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("get status page settings: %w", err))
	}
	return &s, nil
}

// Update persists the org's status-page settings.
func (r *StatusPageSettingsRepository) Update(ctx context.Context, orgID uuid.UUID, s statuspage.Settings) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE organizations
		    SET status_page_enabled = $2, status_page_title = $3
		  WHERE id = $1 AND deleted_at IS NULL`, orgID, s.Enabled, s.Title)
	if err != nil {
		return apperror.Internal(fmt.Errorf("update status page settings: %w", err))
	}
	if ct.RowsAffected() == 0 {
		return apperror.NotFound("organization not found")
	}
	return nil
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
