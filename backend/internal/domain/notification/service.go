package notification

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/audit"
	"beacon/internal/platform/apperror"
	"beacon/internal/platform/crypto"
)

// CreateInput is the validated payload for creating a channel.
type CreateInput struct {
	Name    string
	Type    ChannelType
	Config  map[string]string
	Secret  string
	Enabled *bool
}

// UpdateInput is a partial update. A non-nil, non-empty Secret replaces the
// stored credential; a nil Secret leaves it unchanged.
type UpdateInput struct {
	Name    *string
	Enabled *bool
	Config  map[string]string
	Secret  *string
}

// Service implements channel CRUD and the "send test" use case.
type Service struct {
	repo     Repository
	cipher   *crypto.Cipher
	registry map[ChannelType]Notifier
	auditlog audit.Recorder
	dashURL  string
	now      func() time.Time
}

// NewService wires the notification service.
func NewService(repo Repository, cipher *crypto.Cipher, registry map[ChannelType]Notifier, auditlog audit.Recorder, dashboardURL string) *Service {
	return &Service{
		repo:     repo,
		cipher:   cipher,
		registry: registry,
		auditlog: auditlog,
		dashURL:  strings.TrimRight(dashboardURL, "/"),
		now:      time.Now,
	}
}

// Create adds a channel, encrypting its secret.
func (s *Service) Create(ctx context.Context, actor Actor, in CreateInput) (*Channel, error) {
	if !actor.Role.CanWrite() {
		return nil, apperror.Forbidden("your role does not permit creating channels")
	}
	if !SupportedTypes[in.Type] {
		return nil, apperror.Validation("channel type is not available yet",
			apperror.FieldError{Field: "type", Message: "unsupported channel type"})
	}
	if err := validateChannel(in.Type, in.Config, in.Secret); err != nil {
		return nil, err
	}

	enc, err := s.encryptSecret(in.Secret)
	if err != nil {
		return nil, err
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	now := s.now().UTC()
	ch := &Channel{
		ID:              uuid.New(),
		OrgID:           actor.OrgID,
		Name:            strings.TrimSpace(in.Name),
		Type:            in.Type,
		Enabled:         enabled,
		Config:          in.Config,
		SecretEncrypted: enc,
		CreatedBy:       &actor.UserID,
		UpdatedBy:       &actor.UserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.repo.Create(ctx, ch); err != nil {
		return nil, err
	}
	s.audit(ctx, actor, "notification.channel.created", ch.ID, map[string]any{"type": string(ch.Type), "name": ch.Name})
	return ch, nil
}

// List returns a paginated page of the org's channels plus the total count.
func (s *Service) List(ctx context.Context, actor Actor, f ListFilter) ([]Channel, int, error) {
	return s.repo.List(ctx, actor.OrgID, f)
}

// Get returns one channel.
func (s *Service) Get(ctx context.Context, actor Actor, id uuid.UUID) (*Channel, error) {
	return s.repo.GetByID(ctx, actor.OrgID, id)
}

// Update applies a partial update.
func (s *Service) Update(ctx context.Context, actor Actor, id uuid.UUID, in UpdateInput) (*Channel, error) {
	if !actor.Role.CanWrite() {
		return nil, apperror.Forbidden("your role does not permit updating channels")
	}
	ch, err := s.repo.GetByID(ctx, actor.OrgID, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return nil, apperror.Validation("name must not be empty",
				apperror.FieldError{Field: "name", Message: "is required"})
		}
		ch.Name = name
	}
	if in.Enabled != nil {
		ch.Enabled = *in.Enabled
	}
	if in.Config != nil {
		ch.Config = in.Config
	}
	if in.Secret != nil && *in.Secret != "" {
		enc, err := s.encryptSecret(*in.Secret)
		if err != nil {
			return nil, err
		}
		ch.SecretEncrypted = enc
	}
	if err := validateChannel(ch.Type, ch.Config, secretOr(ch, in.Secret)); err != nil {
		return nil, err
	}
	ch.UpdatedBy = &actor.UserID
	if err := s.repo.Update(ctx, ch); err != nil {
		return nil, err
	}
	s.audit(ctx, actor, "notification.channel.updated", ch.ID, nil)
	return ch, nil
}

// Delete soft-deletes a channel.
func (s *Service) Delete(ctx context.Context, actor Actor, id uuid.UUID) error {
	if !actor.Role.CanWrite() {
		return apperror.Forbidden("your role does not permit deleting channels")
	}
	if _, err := s.repo.GetByID(ctx, actor.OrgID, id); err != nil {
		return err
	}
	if err := s.repo.SoftDelete(ctx, actor.OrgID, id, actor.UserID); err != nil {
		return err
	}
	s.audit(ctx, actor, "notification.channel.deleted", id, nil)
	return nil
}

// SendTest delivers a sample message to a channel so users can verify their
// configuration immediately.
func (s *Service) SendTest(ctx context.Context, actor Actor, id uuid.UUID) error {
	ch, err := s.repo.GetByID(ctx, actor.OrgID, id)
	if err != nil {
		return err
	}
	notifier, ok := s.registry[ch.Type]
	if !ok {
		return apperror.Validation("no notifier available for this channel type")
	}
	dec, err := s.decrypt(ch)
	if err != nil {
		return err
	}
	msg := Message{
		Status:       StatusResolved,
		Severity:     "info",
		Title:        "Beacon test notification",
		Description:  "If you can read this, your channel is configured correctly. 🎉",
		Timestamp:    s.now().UTC(),
		DashboardURL: s.dashURL,
		IsTest:       true,
	}
	if err := notifier.Send(ctx, dec, msg); err != nil {
		return apperror.Newf(apperror.CodeValidation, "test delivery failed: %v", err)
	}
	s.audit(ctx, actor, "notification.test_sent", ch.ID, nil)
	return nil
}

// ---- helpers ----

func (s *Service) encryptSecret(secret string) (string, error) {
	if secret == "" {
		return "", nil
	}
	enc, err := s.cipher.EncryptString(secret)
	if err != nil {
		return "", apperror.Internal(err)
	}
	return enc, nil
}

func (s *Service) decrypt(ch *Channel) (Decrypted, error) {
	dec, err := decryptChannel(s.cipher, ch)
	if err != nil {
		return Decrypted{}, apperror.Internal(err)
	}
	return dec, nil
}

func (s *Service) audit(ctx context.Context, actor Actor, action audit.Action, resourceID uuid.UUID, md map[string]any) {
	org := actor.OrgID
	uid := actor.UserID
	_ = s.auditlog.Record(ctx, audit.Entry{
		OrgID: &org, UserID: &uid, Action: action,
		ResourceType: "notification_channel", ResourceID: resourceID.String(), Metadata: md,
	})
}

func secretOr(ch *Channel, provided *string) string {
	if provided != nil {
		return *provided
	}
	if ch.HasSecret() {
		return "present" // non-empty placeholder so validation passes for an existing secret
	}
	return ""
}

// validateChannel checks a channel's config at CREATE/UPDATE time so a
// misconfiguration surfaces immediately, in the form, rather than as a silent
// non-delivery during a 3 a.m. outage. It validates shape only — reachability
// (a live Slack URL, a working SMTP login) is proven separately by SendTest.
func validateChannel(t ChannelType, config map[string]string, secret string) error {
	req := func(field, msg string) error {
		return apperror.Validation(msg, apperror.FieldError{Field: field, Message: "is required"})
	}
	switch t {
	case TypeTelegram:
		if strings.TrimSpace(config["chat_id"]) == "" {
			return req("config.chat_id", "Telegram requires a chat_id")
		}
		if strings.TrimSpace(secret) == "" {
			return req("secret", "Telegram requires a bot token")
		}

	case TypeSlack:
		// The Slack webhook URL is the secret. Validate it is an https Slack
		// hooks URL so a typo (or a webhook URL pasted into config) fails now.
		u, err := url.Parse(strings.TrimSpace(secret))
		if err != nil || u.Scheme != "https" {
			return apperror.Validation("Slack requires an https incoming-webhook URL",
				apperror.FieldError{Field: "secret", Message: "must be an https Slack webhook URL"})
		}
		if !strings.HasSuffix(u.Host, "slack.com") {
			return apperror.Validation("that does not look like a Slack webhook URL",
				apperror.FieldError{Field: "secret", Message: "host must be hooks.slack.com"})
		}

	case TypeWebhook:
		u, err := url.Parse(strings.TrimSpace(config["url"]))
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return apperror.Validation("Webhook requires a valid URL",
				apperror.FieldError{Field: "config.url", Message: "must be a valid http(s) URL"})
		}
		if m := strings.ToUpper(strings.TrimSpace(config["method"])); m != "" && m != "POST" && m != "PUT" {
			return apperror.Validation("Webhook method must be POST or PUT",
				apperror.FieldError{Field: "config.method", Message: "must be POST or PUT"})
		}

	case TypeEmail:
		if strings.TrimSpace(config["host"]) == "" {
			return req("config.host", "Email requires an SMTP host")
		}
		if strings.TrimSpace(config["from"]) == "" {
			return req("config.from", "Email requires a from address")
		}
		if strings.TrimSpace(config["to"]) == "" {
			return req("config.to", "Email requires at least one recipient")
		}
		if p := strings.TrimSpace(config["port"]); p != "" {
			if n, err := strconv.Atoi(p); err != nil || n < 1 || n > 65535 {
				return apperror.Validation("SMTP port must be a number between 1 and 65535",
					apperror.FieldError{Field: "config.port", Message: "invalid port"})
			}
		}
		if sec := strings.ToLower(strings.TrimSpace(config["security"])); sec != "" &&
			sec != "starttls" && sec != "tls" && sec != "none" {
			return apperror.Validation("SMTP security must be starttls, tls or none",
				apperror.FieldError{Field: "config.security", Message: "invalid value"})
		}
	}
	return nil
}
