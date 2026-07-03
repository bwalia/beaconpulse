// Package notification is the bounded context for alert delivery. It owns
// notification channels (Telegram, Slack, …), the rendering of alerts into rich
// messages, and dispatch to the configured destinations. Channel secrets (e.g. a
// Telegram bot token) are encrypted at rest via the platform crypto.Cipher and
// never leave the server. Delivery itself is performed by per-type Notifier
// implementations in the adapter layer, so the domain stays transport-agnostic.
package notification

import (
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/auth"
)

// ChannelType identifies a delivery integration.
type ChannelType string

const (
	TypeTelegram ChannelType = "telegram"
	TypeSlack    ChannelType = "slack"
	TypeDiscord  ChannelType = "discord"
	TypeEmail    ChannelType = "email"
	TypeWebhook  ChannelType = "webhook"
	TypeTeams    ChannelType = "teams"
)

// SupportedTypes are the channel types with a working Notifier today. Others are
// permitted by the schema and reserved for later iterations.
var SupportedTypes = map[ChannelType]bool{
	TypeTelegram: true,
}

// Channel is a configured delivery destination for an organization.
type Channel struct {
	ID      uuid.UUID
	OrgID   uuid.UUID
	Name    string
	Type    ChannelType
	Enabled bool
	// Config holds non-secret settings (e.g. Telegram chat_id).
	Config map[string]string
	// SecretEncrypted is the AES-256-GCM ciphertext of the channel's credential
	// (e.g. a bot token). Empty if the channel has no secret.
	SecretEncrypted string
	CreatedBy       *uuid.UUID
	UpdatedBy       *uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// HasSecret reports whether a credential is stored (used to render a masked
// indicator in the API without exposing the value).
func (c *Channel) HasSecret() bool { return c.SecretEncrypted != "" }

// Actor is the authenticated caller performing a channel operation.
type Actor struct {
	UserID uuid.UUID
	OrgID  uuid.UUID
	Role   auth.Role
}
