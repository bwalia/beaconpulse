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
	"beacon/internal/domain/plan"
	"beacon/internal/platform/apperror"
	"beacon/internal/platform/crypto"
)

// MonitorRepository implements monitor.Repository.
//
// Header values (settings.headers) are encrypted at rest with cipher: sealed on
// every write, opened on every read, so the domain and the prober config only
// ever see plaintext and the database only ever holds ciphertext.
type MonitorRepository struct {
	pool   *pgxpool.Pool
	cipher *crypto.Cipher
}

// NewMonitorRepository builds a MonitorRepository.
func NewMonitorRepository(pool *pgxpool.Pool, cipher *crypto.Cipher) *MonitorRepository {
	return &MonitorRepository{pool: pool, cipher: cipher}
}

// sealedPrefix marks an encrypted header value. A value without it is legacy
// plaintext from before encryption, read as-is until SealLegacyHeaders runs.
const sealedPrefix = "enc:"

// sealedConfig marshals settings for storage with every header value encrypted.
func (r *MonitorRepository) sealedConfig(s monitor.Settings) ([]byte, error) {
	if len(s.Headers) > 0 {
		sealed := make(map[string]string, len(s.Headers))
		for name, v := range s.Headers {
			ct, err := r.cipher.EncryptString(v)
			if err != nil {
				return nil, err
			}
			sealed[name] = sealedPrefix + ct
		}
		s.Headers = sealed
	}
	return json.Marshal(s)
}

// openHeaders decrypts sealed header values in place.
func (r *MonitorRepository) openHeaders(h map[string]string) {
	for name, v := range h {
		ct, ok := strings.CutPrefix(v, sealedPrefix)
		if !ok {
			continue
		}
		pt, err := r.cipher.DecryptString(ct)
		if err != nil {
			// Unreadable (e.g. the key changed): send the header empty so only this
			// monitor's check fails, rather than failing every read — including the
			// one that builds the probe config for all tenants.
			pt = ""
		}
		h[name] = pt
	}
}

// SealLegacyHeaders encrypts header values stored before encryption at rest.
// Idempotent and safe to run on every start (and from several replicas): a row
// is rewritten only if it still holds plaintext and hasn't changed since read.
func (r *MonitorRepository) SealLegacyHeaders(ctx context.Context) (int, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, config FROM monitors WHERE config ? 'headers'`)
	if err != nil {
		return 0, fmt.Errorf("list monitors with headers: %w", err)
	}
	type row struct {
		id  uuid.UUID
		raw []byte
	}
	var legacy []row
	for rows.Next() {
		var rw row
		if err := rows.Scan(&rw.id, &rw.raw); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan monitor config: %w", err)
		}
		legacy = append(legacy, rw)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	sealed := 0
	for _, rw := range legacy {
		var s monitor.Settings
		if err := json.Unmarshal(rw.raw, &s); err != nil {
			return sealed, fmt.Errorf("unmarshal monitor %s settings: %w", rw.id, err)
		}
		plain := false
		for _, v := range s.Headers {
			if !strings.HasPrefix(v, sealedPrefix) {
				plain = true
			}
		}
		if !plain {
			continue
		}
		r.openHeaders(s.Headers)
		cfg, err := r.sealedConfig(s)
		if err != nil {
			return sealed, err
		}
		tag, err := r.pool.Exec(ctx, `UPDATE monitors SET config=$2 WHERE id=$1 AND config=$3::jsonb`, rw.id, cfg, rw.raw)
		if err != nil {
			return sealed, fmt.Errorf("seal monitor %s headers: %w", rw.id, err)
		}
		sealed += int(tag.RowsAffected())
	}
	return sealed, nil
}

var _ monitor.Repository = (*MonitorRepository)(nil)

const monitorColumns = `id, org_id, project_id, name, type, target, enabled, public,
	interval_seconds, timeout_seconds, config, last_status, last_checked_at,
	created_by, updated_by, created_at, updated_at,
	ping_token, last_ping_at, grace_seconds,
	COALESCE(github_token_hash, ''), COALESCE(github_token_prefix, '')`

func (r *MonitorRepository) scan(row pgx.Row) (*monitor.Monitor, error) {
	var (
		m         monitor.Monitor
		typ       string
		status    string
		configRaw []byte
	)
	// Scan order must track monitorColumns exactly — `public` sits between
	// `enabled` and `interval_seconds`.
	if err := row.Scan(&m.ID, &m.OrgID, &m.ProjectID, &m.Name, &typ, &m.Target, &m.Enabled, &m.Public,
		&m.IntervalSeconds, &m.TimeoutSeconds, &configRaw, &status, &m.LastCheckedAt,
		&m.CreatedBy, &m.UpdatedBy, &m.CreatedAt, &m.UpdatedAt,
		&m.PingToken, &m.LastPingAt, &m.GraceSeconds,
		&m.GitHubTokenHash, &m.GitHubTokenPrefix); err != nil {
		return nil, err
	}
	m.Type = monitor.Type(typ)
	m.LastStatus = monitor.Status(status)
	if len(configRaw) > 0 {
		if err := json.Unmarshal(configRaw, &m.Settings); err != nil {
			return nil, fmt.Errorf("unmarshal monitor settings: %w", err)
		}
		r.openHeaders(m.Settings.Headers)
	}
	return &m, nil
}

// Create inserts a monitor.
func (r *MonitorRepository) Create(ctx context.Context, m *monitor.Monitor) error {
	cfg, err := r.sealedConfig(m.Settings)
	if err != nil {
		return apperror.Internal(fmt.Errorf("marshal settings: %w", err))
	}
	_, err = r.pool.Exec(ctx,
		`INSERT INTO monitors
		 (id, org_id, project_id, name, type, target, enabled, public, interval_seconds, timeout_seconds, config, last_status, created_by, updated_by, created_at, updated_at, ping_token, last_ping_at, grace_seconds, github_token_hash, github_token_prefix)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`,
		m.ID, m.OrgID, m.ProjectID, m.Name, string(m.Type), m.Target, m.Enabled, m.Public,
		m.IntervalSeconds, m.TimeoutSeconds, cfg, string(m.LastStatus),
		m.CreatedBy, m.UpdatedBy, m.CreatedAt, m.UpdatedAt,
		m.PingToken, m.LastPingAt, m.GraceSeconds,
		nullIfEmpty(m.GitHubTokenHash), nullIfEmpty(m.GitHubTokenPrefix))
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
	m, err := r.scan(row)
	if err != nil {
		if isNoRows(err) {
			return nil, apperror.NotFound("monitor not found")
		}
		return nil, apperror.Internal(fmt.Errorf("get monitor: %w", err))
	}
	return m, nil
}

// TargetFor returns just what a diagnosis needs to probe, scoped to org.
//
// Narrow on purpose: the org id is part of the lookup rather than a check after it,
// so another tenant's monitor is not "found and rejected" but simply not found — and
// a diagnosis, which reports resolved addresses and certificate subjects, is the last
// place to rely on remembering to compare an owner id.
func (r *MonitorRepository) TargetFor(ctx context.Context, orgID, monitorID uuid.UUID) (string, string, error) {
	var target, monitorType string
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(target, ''), type FROM monitors
		  WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL`, monitorID, orgID).
		Scan(&target, &monitorType)
	if err != nil {
		if isNoRows(err) {
			return "", "", apperror.NotFound("monitor not found")
		}
		return "", "", apperror.Internal(fmt.Errorf("get monitor target: %w", err))
	}
	return target, monitorType, nil
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
		m, err := r.scan(rows)
		if err != nil {
			return nil, 0, apperror.Internal(fmt.Errorf("scan monitor: %w", err))
		}
		out = append(out, *m)
	}
	return out, total, rows.Err()
}

// Update persists mutable fields including settings.
func (r *MonitorRepository) Update(ctx context.Context, m *monitor.Monitor) error {
	cfg, err := r.sealedConfig(m.Settings)
	if err != nil {
		return apperror.Internal(fmt.Errorf("marshal settings: %w", err))
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE monitors SET name=$3, target=$4, enabled=$5, interval_seconds=$6, timeout_seconds=$7, config=$8, updated_by=$9, public=$10, grace_seconds=$11, github_token_hash=$12, github_token_prefix=$13
		 WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL`,
		m.ID, m.OrgID, m.Name, m.Target, m.Enabled, m.IntervalSeconds, m.TimeoutSeconds, cfg, m.UpdatedBy, m.Public, m.GraceSeconds,
		nullIfEmpty(m.GitHubTokenHash), nullIfEmpty(m.GitHubTokenPrefix))
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
		m, err := r.scan(rows)
		if err != nil {
			return nil, apperror.Internal(fmt.Errorf("scan monitor: %w", err))
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

// EffectivePlans returns each non-deleted org's effective plan, so the control
// plane can cap probing to that tier's monitor limit — a depleted pay-as-you-go
// org falls back to Free's 10. Limit values stay in the plan package.
func (r *MonitorRepository) EffectivePlans(ctx context.Context) (map[uuid.UUID]plan.Plan, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT o.id, o.plan, o.subscription_status, o.credit_seconds, COALESCE(ow.email, '')
		   FROM organizations o
		   LEFT JOIN LATERAL (
		       SELECT email FROM users
		        WHERE org_id = o.id AND role = 'owner' AND deleted_at IS NULL
		        ORDER BY created_at LIMIT 1
		   ) ow ON true
		  WHERE o.deleted_at IS NULL`)
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("list org plans: %w", err))
	}
	defer rows.Close()
	out := map[uuid.UUID]plan.Plan{}
	for rows.Next() {
		var (
			id         uuid.UUID
			p          string
			status     *string
			credit     int64
			ownerEmail string
		)
		if err := rows.Scan(&id, &p, &status, &credit, &ownerEmail); err != nil {
			return nil, apperror.Internal(fmt.Errorf("scan org plan: %w", err))
		}
		active := status != nil && (*status == "active" || *status == "trialing")
		out[id] = plan.Resolve(plan.Plan(p), active, credit, ownerEmail)
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

// GitHubByTokenHash resolves the non-deleted github_actions monitor whose ingest
// token hashes to hash, in one indexed lookup on the unique hash. It returns
// (nil, nil) when no monitor matches, so the domain reports one opaque NotFound
// rather than letting the endpoint become an oracle for which tokens exist.
func (r *MonitorRepository) GitHubByTokenHash(ctx context.Context, hash string) (*monitor.Monitor, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+monitorColumns+` FROM monitors WHERE github_token_hash=$1 AND deleted_at IS NULL`, hash)
	m, err := r.scan(row)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, apperror.Internal(fmt.Errorf("lookup github monitor: %w", err))
	}
	return m, nil
}

// RecordGitHubRun appends one github_actions run to the history log. Append-only:
// the row is never updated, so a later green run no longer erases an earlier red one.
func (r *MonitorRepository) RecordGitHubRun(ctx context.Context, run monitor.GitHubRun) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO github_runs
		   (monitor_id, status, conclusion, workflow, run_number, run_attempt,
		    branch, sha, actor, event_name, run_url, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		run.MonitorID, string(run.Status), run.Conclusion, run.Workflow, run.RunNumber,
		run.RunAttempt, run.Branch, run.SHA, run.Actor, run.EventName, run.RunURL, run.CreatedAt)
	if err != nil {
		return apperror.Internal(fmt.Errorf("record github run: %w", err))
	}
	return nil
}

// ListGitHubRuns returns a monitor's recent runs, newest first. The join on monitors
// enforces org scope (a monitor in another org, or a soft-deleted one, yields no rows).
func (r *MonitorRepository) ListGitHubRuns(ctx context.Context, orgID, monitorID uuid.UUID, limit int) ([]monitor.GitHubRun, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT gr.status, gr.conclusion, gr.workflow, gr.run_number, gr.run_attempt,
		        gr.branch, gr.sha, gr.actor, gr.event_name, gr.run_url, gr.created_at
		   FROM github_runs gr
		   JOIN monitors m ON m.id = gr.monitor_id
		  WHERE gr.monitor_id = $1 AND m.org_id = $2 AND m.deleted_at IS NULL
		  ORDER BY gr.created_at DESC
		  LIMIT $3`,
		monitorID, orgID, limit)
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("list github runs: %w", err))
	}
	defer rows.Close()

	out := make([]monitor.GitHubRun, 0, limit)
	for rows.Next() {
		run := monitor.GitHubRun{MonitorID: monitorID}
		var status string
		if err := rows.Scan(&status, &run.Conclusion, &run.Workflow, &run.RunNumber,
			&run.RunAttempt, &run.Branch, &run.SHA, &run.Actor, &run.EventName,
			&run.RunURL, &run.CreatedAt); err != nil {
			return nil, apperror.Internal(fmt.Errorf("scan github run: %w", err))
		}
		run.Status = monitor.Status(status)
		out = append(out, run)
	}
	if err := rows.Err(); err != nil {
		return nil, apperror.Internal(fmt.Errorf("iterate github runs: %w", err))
	}
	return out, nil
}

// nullIfEmpty maps an empty string to a SQL NULL. github_token_hash carries a
// partial UNIQUE index, so the many monitors without a token must store NULL (many
// allowed) rather than a shared empty string (which would collide on the second row).
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
