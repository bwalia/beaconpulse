package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"beacon/internal/adapter/webpush"
	"beacon/internal/domain/device"
	"beacon/internal/domain/notification"
	"beacon/internal/platform/logger"
)

// webPushSender is the slice of the web push client the notifier uses.
type webPushSender interface {
	Send(ctx context.Context, sub webpush.Subscription, payload []byte, ttl time.Duration, urgency string) (webpush.Result, error)
}

// WebPushNotifier delivers alerts to the browsers (installed PWAs or open tabs)
// an org's members have subscribed. Like APNs it has no per-channel secret: the
// VAPID key is platform config and the destinations are looked up at send time.
type WebPushNotifier struct {
	client webPushSender
	store  device.WebSubscriptionStore
}

func NewWebPushNotifier(client *webpush.Client, store device.WebSubscriptionStore) *WebPushNotifier {
	return &WebPushNotifier{client: client, store: store}
}

func (n *WebPushNotifier) Type() notification.ChannelType { return notification.TypeWebPush }

// webPushPayload is what the service worker (public/sw.js) turns into a
// notification. tag collapses repeat alerts for one monitor into one notification.
type webPushPayload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
	Tag   string `json:"tag,omitempty"`
}

// alertTTL bounds how long a push service holds an alert for an offline device;
// an outage report that arrives the next day is noise.
const alertTTL = 4 * time.Hour

// Send fans the message out to every subscription for the channel's org.
// Best-effort per subscription, like APNs: one failure never blocks the rest,
// and a subscription its push service reports gone is pruned.
func (n *WebPushNotifier) Send(ctx context.Context, ch notification.Decrypted, msg notification.Message) error {
	log := logger.FromContext(ctx)
	if ch.OrgID == uuid.Nil {
		return fmt.Errorf("webpush: channel has no org id")
	}
	subs, err := n.store.WebSubscriptionsByOrg(ctx, ch.OrgID)
	if err != nil {
		return fmt.Errorf("webpush: load subscriptions: %w", err)
	}
	if len(subs) == 0 {
		return nil
	}

	p := webPushPayload{Title: apnsTitle(msg), Body: truncateRunes(apnsBody(msg), 600), URL: "/alerts", Tag: msg.MonitorID}
	urgency := "high"
	switch {
	case msg.IsTest:
		p.URL, p.Tag, urgency = "/notifications", "test", "normal"
	case msg.Status == notification.StatusResolved:
		p.URL, urgency = "/monitors", "normal"
	}
	payload, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("webpush: marshal payload: %w", err)
	}

	var (
		delivered int
		lastErr   error
	)
	for _, s := range subs {
		// Re-checked here, not just at registration: the server POSTs to this URL.
		if !device.ValidWebPushEndpoint(s.Endpoint) {
			_ = n.store.Delete(ctx, s.Endpoint)
			continue
		}
		res, err := n.client.Send(ctx, webpush.Subscription{Endpoint: s.Endpoint, P256dh: s.P256dh, Auth: s.Auth}, payload, alertTTL, urgency)
		switch {
		case err != nil:
			lastErr = err
			log.Warn("webpush: send failed", slog.String("error", err.Error()))
		case res.OK():
			delivered++
		case res.Gone():
			if derr := n.store.Delete(ctx, s.Endpoint); derr != nil {
				log.Warn("webpush: prune subscription failed", slog.String("error", derr.Error()))
			}
		default:
			lastErr = fmt.Errorf("webpush: %d %s", res.StatusCode, res.Body)
			log.Warn("webpush: push rejected", slog.Int("status", res.StatusCode), slog.String("body", res.Body))
		}
	}
	if delivered == 0 && lastErr != nil {
		return lastErr
	}
	return nil
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
