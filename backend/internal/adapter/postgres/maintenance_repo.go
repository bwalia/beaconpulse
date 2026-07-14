package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/domain/maintenance"
	"beacon/internal/platform/apperror"
)

// MaintenanceRepository implements maintenance.Repository.
type MaintenanceRepository struct {
	pool *pgxpool.Pool
}

// NewMaintenanceRepository builds a MaintenanceRepository.
func NewMaintenanceRepository(pool *pgxpool.Pool) *MaintenanceRepository {
	return &MaintenanceRepository{pool: pool}
}

var _ maintenance.Repository = (*MaintenanceRepository)(nil)

// scope_ids is a uuid[] column. This project registers no pgx uuid codec (scalar
// uuid works only via google/uuid's database/sql fallback, which does not extend
// to array elements), so we move the array across the wire as text[] and cast on
// each side — writes bind text[] and cast `$n::uuid[]`, reads select `::text[]`.
const windowColumns = `id, org_id, title, description, starts_at, ends_at, scope,
	scope_ids::text[], created_by, updated_by, created_at, updated_at`

func scanWindow(row pgx.Row) (*maintenance.Window, error) {
	var (
		w      maintenance.Window
		scope  string
		idStrs []string
	)
	if err := row.Scan(&w.ID, &w.OrgID, &w.Title, &w.Description, &w.StartsAt, &w.EndsAt,
		&scope, &idStrs, &w.CreatedBy, &w.UpdatedBy, &w.CreatedAt, &w.UpdatedAt); err != nil {
		return nil, err
	}
	w.Scope = maintenance.Scope(scope)
	ids, err := stringsToUUIDs(idStrs)
	if err != nil {
		return nil, fmt.Errorf("parse scope_ids: %w", err)
	}
	w.ScopeIDs = ids
	return &w, nil
}

// Create inserts a window.
func (r *MaintenanceRepository) Create(ctx context.Context, w *maintenance.Window) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO maintenance_windows
		 (id, org_id, title, description, starts_at, ends_at, scope, scope_ids, created_by, updated_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8::uuid[],$9,$10,$11,$12)`,
		w.ID, w.OrgID, w.Title, w.Description, w.StartsAt, w.EndsAt, string(w.Scope),
		uuidsToStrings(w.ScopeIDs), w.CreatedBy, w.UpdatedBy, w.CreatedAt, w.UpdatedAt)
	if err != nil {
		return apperror.Internal(fmt.Errorf("insert maintenance window: %w", err))
	}
	return nil
}

// GetByID fetches a non-deleted window scoped to org.
func (r *MaintenanceRepository) GetByID(ctx context.Context, orgID, id uuid.UUID) (*maintenance.Window, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+windowColumns+` FROM maintenance_windows WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL`, id, orgID)
	w, err := scanWindow(row)
	if err != nil {
		if isNoRows(err) {
			return nil, apperror.NotFound("maintenance window not found")
		}
		return nil, apperror.Internal(fmt.Errorf("get maintenance window: %w", err))
	}
	return w, nil
}

// List returns a paginated page of the org's windows plus the total count,
// newest-starting first.
func (r *MaintenanceRepository) List(ctx context.Context, orgID uuid.UUID, f maintenance.ListFilter) ([]maintenance.Window, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM maintenance_windows WHERE org_id=$1 AND deleted_at IS NULL`, orgID).Scan(&total); err != nil {
		return nil, 0, apperror.Internal(fmt.Errorf("count maintenance windows: %w", err))
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+windowColumns+` FROM maintenance_windows
		 WHERE org_id=$1 AND deleted_at IS NULL
		 ORDER BY starts_at DESC LIMIT $2 OFFSET $3`, orgID, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, apperror.Internal(fmt.Errorf("list maintenance windows: %w", err))
	}
	defer rows.Close()
	var out []maintenance.Window
	for rows.Next() {
		w, err := scanWindow(rows)
		if err != nil {
			return nil, 0, apperror.Internal(fmt.Errorf("scan maintenance window: %w", err))
		}
		out = append(out, *w)
	}
	return out, total, rows.Err()
}

// Update persists mutable fields. updated_at is set by the row trigger.
func (r *MaintenanceRepository) Update(ctx context.Context, w *maintenance.Window) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE maintenance_windows
		 SET title=$3, description=$4, starts_at=$5, ends_at=$6, scope=$7, scope_ids=$8::uuid[], updated_by=$9
		 WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL`,
		w.ID, w.OrgID, w.Title, w.Description, w.StartsAt, w.EndsAt, string(w.Scope),
		uuidsToStrings(w.ScopeIDs), w.UpdatedBy)
	if err != nil {
		return apperror.Internal(fmt.Errorf("update maintenance window: %w", err))
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("maintenance window not found")
	}
	return nil
}

// SoftDelete marks a window deleted.
func (r *MaintenanceRepository) SoftDelete(ctx context.Context, orgID, id, deletedBy uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE maintenance_windows SET deleted_at=now(), updated_by=$3
		 WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL`, id, orgID, deletedBy)
	if err != nil {
		return apperror.Internal(fmt.Errorf("soft delete maintenance window: %w", err))
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("maintenance window not found")
	}
	return nil
}

// ActiveForMonitor reports whether an active window at `at` covers the monitor.
// The monitor's project is resolved by the join, so scope='project' works without
// the caller supplying the project id. One indexed EXISTS — the suppression hot path.
func (r *MaintenanceRepository) ActiveForMonitor(ctx context.Context, orgID, monitorID uuid.UUID, at time.Time) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1
			  FROM maintenance_windows w
			  JOIN monitors m ON m.id = $2 AND m.org_id = $1 AND m.deleted_at IS NULL
			 WHERE w.org_id = $1
			   AND w.deleted_at IS NULL
			   AND w.starts_at <= $3 AND w.ends_at > $3
			   AND (
			       w.scope = 'org'
			       OR (w.scope = 'project' AND m.project_id = ANY(w.scope_ids))
			       OR (w.scope = 'monitor' AND m.id = ANY(w.scope_ids))
			   )
		)`, orgID, monitorID, at.UTC()).Scan(&exists)
	if err != nil {
		return false, apperror.Internal(fmt.Errorf("check maintenance suppression: %w", err))
	}
	return exists, nil
}

func uuidsToStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

func stringsToUUIDs(ss []string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(ss))
	for _, s := range ss {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}
