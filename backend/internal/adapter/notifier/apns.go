package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"beacon/internal/adapter/apns"
	"beacon/internal/domain/device"
	"beacon/internal/domain/notification"
	"beacon/internal/platform/logger"
)

// pushClient is the slice of the APNs client the notifier uses. An interface so
// the notifier can be tested without reaching Apple's servers.
type pushClient interface {
	Push(ctx context.Context, deviceToken string, payload []byte) (apns.Result, error)
}

// APNsNotifier delivers alerts to Apple devices via APNs. Unlike the other
// channels, its destination is not a single org-level secret but the set of
// device tokens the org's members have registered from the mobile app — so it
// carries no per-channel credential (the APNs signing key is platform config) and
// instead looks tokens up at send time, keyed by the org on the channel.
type APNsNotifier struct {
	client pushClient
	store  device.TokenStore
}

// NewAPNsNotifier builds the notifier from a configured APNs client and the
// device-token store.
func NewAPNsNotifier(client *apns.Client, store device.TokenStore) *APNsNotifier {
	return &APNsNotifier{client: client, store: store}
}

func (n *APNsNotifier) Type() notification.ChannelType { return notification.TypeAPNs }

// apnsPayload is the APNs message body. The aps dictionary drives the visible
// banner; the flat custom keys ride alongside so the app can deep-link the
// notification straight to the monitor without a second API round-trip.
type apnsPayload struct {
	APS       apsDictionary `json:"aps"`
	MonitorID string        `json:"monitor_id,omitempty"`
	Status    string        `json:"status,omitempty"`
}

type apsDictionary struct {
	Alert apsAlert `json:"alert"`
	Sound string   `json:"sound,omitempty"`
}

type apsAlert struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// Send fans the message out to every device token registered for the channel's
// org. Best-effort per token, matching the other channels: one dead or failing
// token never blocks the rest, and a token APNs declares permanently invalid is
// pruned so the list does not grow stale.
func (n *APNsNotifier) Send(ctx context.Context, ch notification.Decrypted, msg notification.Message) error {
	log := logger.FromContext(ctx)
	if ch.OrgID == uuid.Nil {
		return fmt.Errorf("apns: channel has no org id")
	}
	tokens, err := n.store.TokensByOrg(ctx, ch.OrgID)
	if err != nil {
		return fmt.Errorf("apns: load device tokens: %w", err)
	}
	if len(tokens) == 0 {
		return nil // nobody has registered a device; not an error
	}

	payload, err := json.Marshal(buildPayload(msg))
	if err != nil {
		return fmt.Errorf("apns: marshal payload: %w", err)
	}

	var (
		delivered int
		lastErr   error
	)
	for _, tok := range tokens {
		res, err := n.client.Push(ctx, tok, payload)
		if err != nil {
			lastErr = err
			log.Warn("apns: push failed", slog.String("error", err.Error()))
			continue
		}
		switch {
		case res.OK():
			delivered++
		case res.TokenIsDead():
			if derr := n.store.Delete(ctx, tok); derr != nil {
				log.Warn("apns: prune dead token failed", slog.String("error", derr.Error()))
			}
		default:
			lastErr = fmt.Errorf("apns: %d %s", res.StatusCode, res.Reason)
			log.Warn("apns: push rejected",
				slog.Int("status", res.StatusCode), slog.String("reason", res.Reason))
		}
	}
	// Surface an error only when nothing got through, so one bad token among many
	// still counts as a delivered alert (and the dispatcher audits the success).
	if delivered == 0 && lastErr != nil {
		return lastErr
	}
	return nil
}

// buildPayload renders a Message into an APNs payload: a one-line title carrying
// state and monitor name, a body with the detail, and the monitor id for
// deep-linking.
func buildPayload(msg notification.Message) apnsPayload {
	return apnsPayload{
		APS: apsDictionary{
			Alert: apsAlert{Title: apnsTitle(msg), Body: apnsBody(msg)},
			Sound: "default",
		},
		MonitorID: msg.MonitorID,
		Status:    string(msg.Status),
	}
}

func apnsTitle(msg notification.Message) string {
	if msg.IsTest {
		return "Test notification"
	}
	name := msg.MonitorName
	if name == "" {
		name = msg.Title
	}
	switch msg.Status {
	case notification.StatusResolved:
		return "✅ Resolved: " + name
	default:
		if strings.EqualFold(msg.Severity, "critical") {
			return "🔴 Down: " + name
		}
		return "🟠 Alert: " + name
	}
}

func apnsBody(msg notification.Message) string {
	// Prefer the human description; fall back to the target so the push is never
	// empty.
	if strings.TrimSpace(msg.Description) != "" {
		return msg.Description
	}
	if msg.Target != "" {
		return msg.Target
	}
	return msg.Title
}
