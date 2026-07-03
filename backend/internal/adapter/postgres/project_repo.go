package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/domain/project"
	"beacon/internal/platform/apperror"
)

// ProjectRepository implements project.Repository.
type ProjectRepository struct {
	pool *pgxpool.Pool
}

// NewProjectRepository builds a ProjectRepository.
func NewProjectRepository(pool *pgxpool.Pool) *ProjectRepository {
	return &ProjectRepository{pool: pool}
}

var _ project.Repository = (*ProjectRepository)(nil)

const projectColumns = `id, org_id, name, slug, description, environment, is_active,
	created_by, updated_by, created_at, updated_at`

func scanProject(row pgx.Row) (*project.Project, error) {
	var (
		p   project.Project
		env string
	)
	if err := row.Scan(&p.ID, &p.OrgID, &p.Name, &p.Slug, &p.Description, &env, &p.IsActive,
		&p.CreatedBy, &p.UpdatedBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	p.Environment = project.Environment(env)
	return &p, nil
}

// Create inserts a project.
func (r *ProjectRepository) Create(ctx context.Context, p *project.Project) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO projects (id, org_id, name, slug, description, environment, is_active, created_by, updated_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		p.ID, p.OrgID, p.Name, p.Slug, p.Description, string(p.Environment), p.IsActive,
		p.CreatedBy, p.UpdatedBy, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		if c, ok := isUniqueViolation(err); ok && c == "ux_projects_org_slug" {
			return apperror.Conflict("a project with that name already exists")
		}
		return apperror.Internal(fmt.Errorf("insert project: %w", err))
	}
	return nil
}

// GetByID fetches a non-deleted project scoped to org.
func (r *ProjectRepository) GetByID(ctx context.Context, orgID, id uuid.UUID) (*project.Project, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+projectColumns+` FROM projects WHERE id = $1 AND org_id = $2 AND deleted_at IS NULL`, id, orgID)
	p, err := scanProject(row)
	if err != nil {
		if isNoRows(err) {
			return nil, apperror.NotFound("project not found")
		}
		return nil, apperror.Internal(fmt.Errorf("get project: %w", err))
	}
	return p, nil
}

// List returns a filtered, paginated page of projects plus the total count.
func (r *ProjectRepository) List(ctx context.Context, orgID uuid.UUID, f project.ListFilter) ([]project.Project, int, error) {
	where := []string{"org_id = $1", "deleted_at IS NULL"}
	args := []any{orgID}
	n := 1

	if f.Search != "" {
		n++
		where = append(where, fmt.Sprintf("(name ILIKE $%d OR slug ILIKE $%d)", n, n))
		args = append(args, "%"+f.Search+"%")
	}
	if f.Environment != "" {
		n++
		where = append(where, fmt.Sprintf("environment = $%d", n))
		args = append(args, f.Environment)
	}
	clause := strings.Join(where, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM projects WHERE `+clause, args...).Scan(&total); err != nil {
		return nil, 0, apperror.Internal(fmt.Errorf("count projects: %w", err))
	}

	args = append(args, f.Limit, f.Offset)
	q := fmt.Sprintf(`SELECT %s FROM projects WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		projectColumns, clause, n+1, n+2)
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, apperror.Internal(fmt.Errorf("list projects: %w", err))
	}
	defer rows.Close()

	var out []project.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, 0, apperror.Internal(fmt.Errorf("scan project: %w", err))
		}
		out = append(out, *p)
	}
	return out, total, rows.Err()
}

// Update persists mutable fields.
func (r *ProjectRepository) Update(ctx context.Context, p *project.Project) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE projects SET name=$3, description=$4, environment=$5, is_active=$6, updated_by=$7
		 WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL`,
		p.ID, p.OrgID, p.Name, p.Description, string(p.Environment), p.IsActive, p.UpdatedBy)
	if err != nil {
		return apperror.Internal(fmt.Errorf("update project: %w", err))
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("project not found")
	}
	return nil
}

// SoftDelete marks the project deleted and cascades a soft-delete to its
// monitors, all in one transaction.
func (r *ProjectRepository) SoftDelete(ctx context.Context, orgID, id, deletedBy uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return apperror.Internal(fmt.Errorf("begin tx: %w", err))
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`UPDATE monitors SET deleted_at = now(), updated_by = $3
		 WHERE project_id = $1 AND org_id = $2 AND deleted_at IS NULL`,
		id, orgID, deletedBy); err != nil {
		return apperror.Internal(fmt.Errorf("cascade delete monitors: %w", err))
	}

	tag, err := tx.Exec(ctx,
		`UPDATE projects SET deleted_at = now(), updated_by = $3
		 WHERE id = $1 AND org_id = $2 AND deleted_at IS NULL`,
		id, orgID, deletedBy)
	if err != nil {
		return apperror.Internal(fmt.Errorf("soft delete project: %w", err))
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("project not found")
	}
	if err := tx.Commit(ctx); err != nil {
		return apperror.Internal(fmt.Errorf("commit: %w", err))
	}
	return nil
}

// SlugExists reports whether a slug is taken within the org, excluding one id.
func (r *ProjectRepository) SlugExists(ctx context.Context, orgID uuid.UUID, slug string, excludeID uuid.UUID) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM projects WHERE org_id=$1 AND slug=$2 AND id<>$3 AND deleted_at IS NULL)`,
		orgID, slug, excludeID,
	).Scan(&exists); err != nil {
		return false, apperror.Internal(fmt.Errorf("project slug exists: %w", err))
	}
	return exists, nil
}
