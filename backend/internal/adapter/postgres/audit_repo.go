package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/domain/audit"
	"beacon/internal/platform/apperror"
)

// AuditRepository implements audit.Repository.
type AuditRepository struct {
	pool *pgxpool.Pool
}

// NewAuditRepository builds an AuditRepository.
func NewAuditRepository(pool *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{pool: pool}
}

var _ audit.Repository = (*AuditRepository)(nil)

// Insert appends an audit entry.
func (r *AuditRepository) Insert(ctx context.Context, e *audit.Entry) error {
	meta, err := json.Marshal(e.Metadata)
	if err != nil {
		return apperror.Internal(fmt.Errorf("marshal audit metadata: %w", err))
	}
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO audit_logs
		 (id, org_id, user_id, action, resource_type, resource_id, metadata, ip, user_agent, request_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		e.ID, e.OrgID, e.UserID, string(e.Action), e.ResourceType, nullString(e.ResourceID),
		meta, nullString(e.IP), nullString(e.UserAgent), nullString(e.RequestID), e.CreatedAt,
	); err != nil {
		return apperror.Internal(fmt.Errorf("insert audit log: %w", err))
	}
	return nil
}

// List returns a page of audit entries for an org (newest first) plus the total
// count for pagination.
func (r *AuditRepository) List(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]audit.Entry, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM audit_logs WHERE org_id = $1`, orgID,
	).Scan(&total); err != nil {
		return nil, 0, apperror.Internal(fmt.Errorf("count audit logs: %w", err))
	}

	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, user_id, action, resource_type, resource_id, metadata, ip, user_agent, request_id, created_at
		 FROM audit_logs WHERE org_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		orgID, limit, offset,
	)
	if err != nil {
		return nil, 0, apperror.Internal(fmt.Errorf("list audit logs: %w", err))
	}
	defer rows.Close()

	var out []audit.Entry
	for rows.Next() {
		var (
			e          audit.Entry
			action     string
			resID      *string
			ip, ua, rq *string
			meta       []byte
		)
		if err := rows.Scan(&e.ID, &e.OrgID, &e.UserID, &action, &e.ResourceType,
			&resID, &meta, &ip, &ua, &rq, &e.CreatedAt); err != nil {
			return nil, 0, apperror.Internal(fmt.Errorf("scan audit log: %w", err))
		}
		e.Action = audit.Action(action)
		e.ResourceID = deref(resID)
		e.IP = deref(ip)
		e.UserAgent = deref(ua)
		e.RequestID = deref(rq)
		if len(meta) > 0 {
			_ = json.Unmarshal(meta, &e.Metadata)
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
