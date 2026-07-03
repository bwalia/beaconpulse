package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/domain/monitor"
	"beacon/internal/platform/apperror"
)

// MonitorRepository implements monitor.Repository.
type MonitorRepository struct {
	pool *pgxpool.Pool
}

// NewMonitorRepository builds a MonitorRepository.
func NewMonitorRepository(pool *pgxpool.Pool) *MonitorRepository {
	return &MonitorRepository{pool: pool}
}

var _ monitor.Repository = (*MonitorRepository)(nil)

const monitorColumns = `id, org_id, project_id, name, type, target, enabled,
	interval_seconds, timeout_seconds, config, last_status, last_checked_at,
	created_by, updated_by, created_at, updated_at`

func scanMonitor(row pgx.Row) (*monitor.Monitor, error) {
	var (
		m         monitor.Monitor
		typ       string
		status    string
		configRaw []byte
	)
	if err := row.Scan(&m.ID, &m.OrgID, &m.ProjectID, &m.Name, &typ, &m.Target, &m.Enabled,
		&m.IntervalSeconds, &m.TimeoutSeconds, &configRaw, &status, &m.LastCheckedAt,
		&m.CreatedBy, &m.UpdatedBy, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return nil, err
	}
	m.Type = monitor.Type(typ)
	m.LastStatus = monitor.Status(status)
	if len(configRaw) > 0 {
		if err := json.Unmarshal(configRaw, &m.Settings); err != nil {
			return nil, fmt.Errorf("unmarshal monitor settings: %w", err)
		}
	}
	return &m, nil
}

// Create inserts a monitor.
func (r *MonitorRepository) Create(ctx context.Context, m *monitor.Monitor) error {
	cfg, err := json.Marshal(m.Settings)
	if err != nil {
		return apperror.Internal(fmt.Errorf("marshal settings: %w", err))
	}
	_, err = r.pool.Exec(ctx,
		`INSERT INTO monitors
		 (id, org_id, project_id, name, type, target, enabled, interval_seconds, timeout_seconds, config, last_status, created_by, updated_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		m.ID, m.OrgID, m.ProjectID, m.Name, string(m.Type), m.Target, m.Enabled,
		m.IntervalSeconds, m.TimeoutSeconds, cfg, string(m.LastStatus),
		m.CreatedBy, m.UpdatedBy, m.CreatedAt, m.UpdatedAt)
	if err != nil {
		if isForeignKeyViolation(err) {
			return apperror.Validation("project not found",
				apperror.FieldError{Field: "project_id", Message: "must reference an existing project"})
		}
		return apperror.Internal(fmt.Errorf("insert monitor: %w", err))
	}
	return nil
}

// GetByID fetches a non-deleted monitor scoped to org.
func (r *MonitorRepository) GetByID(ctx context.Context, orgID, id uuid.UUID) (*monitor.Monitor, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+monitorColumns+` FROM monitors WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL`, id, orgID)
	m, err := scanMonitor(row)
	if err != nil {
		if isNoRows(err) {
			return nil, apperror.NotFound("monitor not found")
		}
		return nil, apperror.Internal(fmt.Errorf("get monitor: %w", err))
	}
	return m, nil
}

// List returns a filtered, paginated page of monitors plus the total count.
func (r *MonitorRepository) List(ctx context.Context, orgID uuid.UUID, f monitor.ListFilter) ([]monitor.Monitor, int, error) {
	where := []string{"org_id = $1", "deleted_at IS NULL"}
	args := []any{orgID}
	n := 1

	if f.ProjectID != nil {
		n++
		where = append(where, fmt.Sprintf("project_id = $%d", n))
		args = append(args, *f.ProjectID)
	}
	if f.Type != "" {
		n++
		where = append(where, fmt.Sprintf("type = $%d", n))
		args = append(args, f.Type)
	}
	if f.Status != "" {
		n++
		where = append(where, fmt.Sprintf("last_status = $%d", n))
		args = append(args, f.Status)
	}
	if f.Enabled != nil {
		n++
		where = append(where, fmt.Sprintf("enabled = $%d", n))
		args = append(args, *f.Enabled)
	}
	if f.Search != "" {
		n++
		where = append(where, fmt.Sprintf("(name ILIKE $%d OR target ILIKE $%d)", n, n))
		args = append(args, "%"+f.Search+"%")
	}
	clause := strings.Join(where, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM monitors WHERE `+clause, args...).Scan(&total); err != nil {
		return nil, 0, apperror.Internal(fmt.Errorf("count monitors: %w", err))
	}

	args = append(args, f.Limit, f.Offset)
	q := fmt.Sprintf(`SELECT %s FROM monitors WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		monitorColumns, clause, n+1, n+2)
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, apperror.Internal(fmt.Errorf("list monitors: %w", err))
	}
	defer rows.Close()

	var out []monitor.Monitor
	for rows.Next() {
		m, err := scanMonitor(rows)
		if err != nil {
			return nil, 0, apperror.Internal(fmt.Errorf("scan monitor: %w", err))
		}
		out = append(out, *m)
	}
	return out, total, rows.Err()
}

// Update persists mutable fields including settings.
func (r *MonitorRepository) Update(ctx context.Context, m *monitor.Monitor) error {
	cfg, err := json.Marshal(m.Settings)
	if err != nil {
		return apperror.Internal(fmt.Errorf("marshal settings: %w", err))
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE monitors SET name=$3, target=$4, enabled=$5, interval_seconds=$6, timeout_seconds=$7, config=$8, updated_by=$9
		 WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL`,
		m.ID, m.OrgID, m.Name, m.Target, m.Enabled, m.IntervalSeconds, m.TimeoutSeconds, cfg, m.UpdatedBy)
	if err != nil {
		return apperror.Internal(fmt.Errorf("update monitor: %w", err))
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("monitor not found")
	}
	return nil
}

// SetEnabled toggles the enabled flag. When pausing, last_status is set to
// 'paused'; when enabling it is reset to 'unknown' until the next probe.
func (r *MonitorRepository) SetEnabled(ctx context.Context, orgID, id uuid.UUID, enabled bool, updatedBy uuid.UUID) error {
	status := string(monitor.StatusPaused)
	if enabled {
		status = string(monitor.StatusUnknown)
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE monitors SET enabled=$3, last_status=$4, updated_by=$5
		 WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL`,
		id, orgID, enabled, status, updatedBy)
	if err != nil {
		return apperror.Internal(fmt.Errorf("set monitor enabled: %w", err))
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("monitor not found")
	}
	return nil
}

// SoftDelete marks a monitor deleted.
func (r *MonitorRepository) SoftDelete(ctx context.Context, orgID, id, deletedBy uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE monitors SET deleted_at = now(), updated_by = $3
		 WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL`,
		id, orgID, deletedBy)
	if err != nil {
		return apperror.Internal(fmt.Errorf("soft delete monitor: %w", err))
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("monitor not found")
	}
	return nil
}

// CountByOrg returns the number of non-deleted monitors in an org.
func (r *MonitorRepository) CountByOrg(ctx context.Context, orgID uuid.UUID) (int, error) {
	var n int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM monitors WHERE org_id = $1 AND deleted_at IS NULL`, orgID,
	).Scan(&n); err != nil {
		return 0, apperror.Internal(fmt.Errorf("count monitors by org: %w", err))
	}
	return n, nil
}

// ProjectExists verifies a project belongs to the org and is not deleted.
func (r *MonitorRepository) ProjectExists(ctx context.Context, orgID, projectID uuid.UUID) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL)`,
		projectID, orgID,
	).Scan(&exists); err != nil {
		return false, apperror.Internal(fmt.Errorf("project exists: %w", err))
	}
	return exists, nil
}

// ListAllEnabled returns every enabled, non-deleted monitor across all orgs,
// joined with project/org labels the control plane attaches to metrics.
func (r *MonitorRepository) ListAllEnabled(ctx context.Context) ([]monitor.Monitor, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+monitorColumns+` FROM monitors WHERE enabled = TRUE AND deleted_at IS NULL ORDER BY id`)
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("list all enabled monitors: %w", err))
	}
	defer rows.Close()

	var out []monitor.Monitor
	for rows.Next() {
		m, err := scanMonitor(rows)
		if err != nil {
			return nil, apperror.Internal(fmt.Errorf("scan monitor: %w", err))
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

// ApplyStatusUpdates writes observed statuses back in a single transaction. It
// never overwrites a paused monitor (a monitor may have been paused between the
// Prometheus read and this write).
func (r *MonitorRepository) ApplyStatusUpdates(ctx context.Context, updates []monitor.StatusUpdate) (int64, error) {
	if len(updates) == 0 {
		return 0, nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, apperror.Internal(fmt.Errorf("begin tx: %w", err))
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var affected int64
	for _, u := range updates {
		tag, err := tx.Exec(ctx,
			`UPDATE monitors SET last_status = $2, last_checked_at = $3
			 WHERE id = $1 AND deleted_at IS NULL AND enabled = TRUE AND last_status <> 'paused'`,
			u.MonitorID, string(u.Status), u.CheckedAt)
		if err != nil {
			return 0, apperror.Internal(fmt.Errorf("update monitor status: %w", err))
		}
		affected += tag.RowsAffected()
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, apperror.Internal(fmt.Errorf("commit status updates: %w", err))
	}
	return affected, nil
}
