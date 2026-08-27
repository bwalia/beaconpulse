package notification

import (
	"context"

	"github.com/google/uuid"
)

// ListFilter narrows and paginates a channel listing.
type ListFilter struct {
	Search string
	Limit  int
	Offset int
}

// Repository persists notification channels, org-scoped.
type Repository interface {
	Create(ctx context.Context, c *Channel) error
	GetByID(ctx context.Context, orgID, id uuid.UUID) (*Channel, error)
	List(ctx context.Context, orgID uuid.UUID, f ListFilter) (items []Channel, total int, err error)
	Update(ctx context.Context, c *Channel) error
	SoftDelete(ctx context.Context, orgID, id, deletedBy uuid.UUID) error
	// ListEnabledByOrg returns the enabled channels for one org, used when
	// dispatching an alert.
	ListEnabledByOrg(ctx context.Context, orgID uuid.UUID) ([]Channel, error)
	// FindByType returns the org's channel of a given type, or nil if none
	// exists. Used to make channel activation idempotent (see
	// Service.EnsureAPNsChannel).
	FindByType(ctx context.Context, orgID uuid.UUID, t ChannelType) (*Channel, error)
}

// Decrypted is a channel with its secret decrypted, handed to a Notifier at
// send time. It lives only in memory for the duration of a send.
type Decrypted struct {
	Type ChannelType
	Name string
	// OrgID is the owning organization. Most notifiers ignore it (their
	// destination is the channel's own secret), but a fan-out channel like APNs
	// needs it to look up the org's registered device tokens at send time.
	OrgID  uuid.UUID
	Config map[string]string
	Secret string
}

// Notifier delivers a rendered Message to one channel type. Implementations live
// in the adapter layer (e.g. Telegram Bot API) and must be safe for concurrent
// use.
type Notifier interface {
	Type() ChannelType
	Send(ctx context.Context, ch Decrypted, msg Message) error
}

// ProjectLookup resolves a project's display name and environment for enriching
// alert messages. Implemented over the project repository; best-effort.
type ProjectLookup interface {
	Project(ctx context.Context, orgID, projectID uuid.UUID) (name, environment string, found bool)
}
