package notifier

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"beacon/internal/domain/notification"
)

// OrgEmailLookup resolves the fallback alert recipients for an org — the people
// to email when the org has configured no notification channel of its own
// (its active owners and admins). Implemented over the user repository.
type OrgEmailLookup interface {
	AlertRecipients(ctx context.Context, orgID uuid.UUID) ([]string, error)
}

// DefaultEmailConfig is the platform SMTP relay used for the default fallback.
// It mirrors the fields a per-channel email uses, but is operator-wide config
// (one relay for the whole deployment) rather than tenant-supplied.
type DefaultEmailConfig struct {
	Host, Port, From, Username, Password, Security string
}

// DefaultEmailNotifier is the last-resort alert channel: it emails an org's
// owners/admins over the platform SMTP relay when that org has no channel of its
// own. It delegates the actual SMTP exchange to the per-channel EmailNotifier by
// synthesising an in-memory channel, so there is exactly one place that speaks
// SMTP and the fallback email is byte-for-byte the same as a configured one.
type DefaultEmailNotifier struct {
	cfg    DefaultEmailConfig
	email  notification.Notifier // an email notifier (TypeEmail); injectable for tests
	lookup OrgEmailLookup
}

// NewDefaultEmailNotifier builds the fallback over the platform relay config and
// an org→recipients lookup, using a fresh EmailNotifier for delivery. brand is the
// product name shown in the message copy (empty falls back to "Beacon").
func NewDefaultEmailNotifier(cfg DefaultEmailConfig, lookup OrgEmailLookup, brand string) *DefaultEmailNotifier {
	return &DefaultEmailNotifier{cfg: cfg, email: NewEmailNotifier(brand), lookup: lookup}
}

// Fallback emails the alert to the org's default recipients. Returns nil (a
// no-op) when the org has no resolvable recipients, so an org whose only members
// are viewers — or an org with none — is not treated as a delivery failure.
func (n *DefaultEmailNotifier) Fallback(ctx context.Context, orgID uuid.UUID, msg notification.Message) error {
	if orgID == uuid.Nil {
		return fmt.Errorf("default email: missing org id")
	}
	recipients, err := n.lookup.AlertRecipients(ctx, orgID)
	if err != nil {
		return fmt.Errorf("default email: resolve recipients: %w", err)
	}
	if len(recipients) == 0 {
		return nil // nobody to notify; not an error
	}
	dec := notification.Decrypted{
		Type:  notification.TypeEmail,
		Name:  "Default email",
		OrgID: orgID,
		Config: map[string]string{
			"host":     n.cfg.Host,
			"port":     n.cfg.Port,
			"from":     n.cfg.From,
			"username": n.cfg.Username,
			"security": n.cfg.Security,
			"to":       strings.Join(recipients, ","),
		},
		Secret: n.cfg.Password,
	}
	return n.email.Send(ctx, dec, msg)
}
