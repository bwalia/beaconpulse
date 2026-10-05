package notifier

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"beacon/internal/adapter/webpush"
	"beacon/internal/domain/device"
	"beacon/internal/domain/notification"
)

type fakeWebSender struct {
	status   map[string]int
	sent     []string
	payloads [][]byte
}

func (f *fakeWebSender) Send(_ context.Context, s webpush.Subscription, p []byte, _ time.Duration, _ string) (webpush.Result, error) {
	f.sent = append(f.sent, s.Endpoint)
	f.payloads = append(f.payloads, p)
	return webpush.Result{StatusCode: f.status[s.Endpoint]}, nil
}

type fakeWebStore struct {
	subs    []device.WebSubscription
	deleted []string
}

func (f *fakeWebStore) WebSubscriptionsByOrg(context.Context, uuid.UUID) ([]device.WebSubscription, error) {
	return f.subs, nil
}
func (f *fakeWebStore) Delete(_ context.Context, token string) error {
	f.deleted = append(f.deleted, token)
	return nil
}

// TestWebPushFansOutAndPrunes: every subscription gets the alert, a gone one is
// pruned, and an endpoint off the push-service allowlist is pruned unsent.
func TestWebPushFansOutAndPrunes(t *testing.T) {
	const ok, gone, bad = "https://fcm.googleapis.com/fcm/send/a", "https://updates.push.services.mozilla.com/wpush/v2/b", "https://10.0.0.5/x"
	sender := &fakeWebSender{status: map[string]int{ok: 201, gone: 410}}
	store := &fakeWebStore{subs: []device.WebSubscription{{Endpoint: ok}, {Endpoint: gone}, {Endpoint: bad}}}
	n := &WebPushNotifier{client: sender, store: store}

	err := n.Send(context.Background(), notification.Decrypted{OrgID: uuid.New()}, notification.Message{
		Status: notification.StatusFiring, Severity: "critical", MonitorName: "Checkout", MonitorID: "m1", Description: "HTTP 503",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sender.sent) != 2 {
		t.Errorf("sent to %v, want the two allowlisted endpoints", sender.sent)
	}
	if len(store.deleted) != 2 || store.deleted[0] != gone || store.deleted[1] != bad {
		t.Errorf("pruned %v, want [%s %s]", store.deleted, gone, bad)
	}
	var p webPushPayload
	if err := json.Unmarshal(sender.payloads[0], &p); err != nil {
		t.Fatal(err)
	}
	if p.Title != "🔴 Down: Checkout" || p.URL != "/alerts" || p.Tag != "m1" {
		t.Errorf("payload = %+v", p)
	}
}
