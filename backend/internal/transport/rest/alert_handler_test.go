package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"beacon/internal/domain/notification"
)

type fakeOperators struct{ got chan notification.Message }

func (f fakeOperators) NotifyOperators(_ context.Context, msg notification.Message) error {
	f.got <- msg
	return nil
}

// A scope=platform alert goes to the operators and never to the tenant dispatcher
// (left nil here: touching it would panic).
func TestAlertWebhook_PlatformAlertGoesToOperators(t *testing.T) {
	ops := fakeOperators{got: make(chan notification.Message, 1)}
	h := NewAlertHandler(nil, "", ops)

	body := `{"alerts":[{"status":"firing","labels":{"alertname":"BeaconAPIErrorRate","scope":"platform","severity":"warning","cluster":"beacon-sysops-prod"},"annotations":{"summary":"API error rate above 5%","description":"12% of API requests have been failing"},"startsAt":"2026-10-02T10:00:00Z"}]}`
	rec := httptest.NewRecorder()
	h.Webhook(rec, httptest.NewRequest(http.MethodPost, "/api/v1/alerts/webhook", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}

	select {
	case msg := <-ops.got:
		if msg.Title != "API error rate above 5%" || msg.Severity != "warning" || msg.Environment != "beacon-sysops-prod" || msg.Status != notification.StatusFiring {
			t.Fatalf("unexpected operator message: %+v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("platform alert never reached the operators")
	}
}
