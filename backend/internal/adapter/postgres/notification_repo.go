package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"beacon/internal/domain/notification"
	"beacon/internal/platform/apperror"
)

// NotificationRepository implements notification.Repository.
type NotificationRepository struct {
	pool *pgxpool.Pool
}

// NewNotificationRepository builds a NotificationRepository.
func NewNotificationRepository(pool *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{pool: pool}
}

var _ notification.Repository = (*NotificationRepository)(nil)

const channelColumns = `id, org_id, name, type, enabled, config, secret_encrypted,
	created_by, updated_by, created_at, updated_at`

func scanChannel(row pgx.Row) (*notification.Channel, error) {
	var (
		c         notification.Channel
		typ       string
		configRaw []byte
		secret    *string
	)
	if err := row.Scan(&c.ID, &c.OrgID, &c.Name, &typ, &c.Enabled, &configRaw, &secret,
		&c.CreatedBy, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	c.Type = notification.ChannelType(typ)
	if secret != nil {
		c.SecretEncrypted = *secret
	}
	if len(configRaw) > 0 {
		if err := json.Unmarshal(configRaw, &c.Config); err != nil {
			return nil, fmt.Errorf("unmarshal channel config: %w", err)
		}
	}
	if c.Config == nil {
		c.Config = map[string]string{}
	}
	return &c, nil
}

// Create inserts a channel.
func (r *NotificationRepository) Create(ctx context.Context, c *notification.Channel) error {
	cfg, err := json.Marshal(c.Config)
	if err != nil {
		return apperror.Internal(fmt.Errorf("marshal config: %w", err))
	}
	_, err = r.pool.Exec(ctx,
		`INSERT INTO notification_channels
		 (id, org_id, name, type, enabled, config, secret_encrypted, created_by, updated_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		c.ID, c.OrgID, c.Name, string(c.Type), c.Enabled, cfg, nullString(c.SecretEncrypted),
		c.CreatedBy, c.UpdatedBy, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return apperror.Internal(fmt.Errorf("insert channel: %w", err))
	}
	return nil
}

// GetByID fetches a non-deleted channel scoped to org.
func (r *NotificationRepository) GetByID(ctx context.Context, orgID, id uuid.UUID) (*notification.Channel, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+channelColumns+` FROM notification_channels WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL`, id, orgID)
	c, err := scanChannel(row)
	if err != nil {
		if isNoRows(err) {
			return nil, apperror.NotFound("notification channel not found")
		}
		return nil, apperror.Internal(fmt.Errorf("get channel: %w", err))
	}
	return c, nil
}

// List returns the org's channels, newest first.
func (r *NotificationRepository) List(ctx context.Context, orgID uuid.UUID) ([]notification.Channel, error) {
	return r.query(ctx,
		`SELECT `+channelColumns+` FROM notification_channels
		 WHERE org_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC`, orgID)
}

// ListEnabledByOrg returns the enabled channels for one org.
func (r *NotificationRepository) ListEnabledByOrg(ctx context.Context, orgID uuid.UUID) ([]notification.Channel, error) {
	return r.query(ctx,
		`SELECT `+channelColumns+` FROM notification_channels
		 WHERE org_id=$1 AND enabled=TRUE AND deleted_at IS NULL`, orgID)
}

func (r *NotificationRepository) query(ctx context.Context, sql string, args ...any) ([]notification.Channel, error) {
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("query channels: %w", err))
	}
	defer rows.Close()
	var out []notification.Channel
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, apperror.Internal(fmt.Errorf("scan channel: %w", err))
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// Update persists mutable fields.
func (r *NotificationRepository) Update(ctx context.Context, c *notification.Channel) error {
	cfg, err := json.Marshal(c.Config)
	if err != nil {
		return apperror.Internal(fmt.Errorf("marshal config: %w", err))
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE notification_channels SET name=$3, enabled=$4, config=$5, secret_encrypted=$6, updated_by=$7
		 WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL`,
		c.ID, c.OrgID, c.Name, c.Enabled, cfg, nullString(c.SecretEncrypted), c.UpdatedBy)
	if err != nil {
		return apperror.Internal(fmt.Errorf("update channel: %w", err))
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("notification channel not found")
	}
	return nil
}

// SoftDelete marks a channel deleted.
func (r *NotificationRepository) SoftDelete(ctx context.Context, orgID, id, deletedBy uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE notification_channels SET deleted_at=now(), updated_by=$3
		 WHERE id=$1 AND org_id=$2 AND deleted_at IS NULL`, id, orgID, deletedBy)
	if err != nil {
		return apperror.Internal(fmt.Errorf("soft delete channel: %w", err))
	}
	if tag.RowsAffected() == 0 {
		return apperror.NotFound("notification channel not found")
	}
	return nil
}
