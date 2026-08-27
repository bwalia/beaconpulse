package notifier

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"beacon/internal/adapter/apns"
	"beacon/internal/domain/notification"
)

type fakePush struct {
	results map[string]apns.Result
	calls   []string
}

func (f *fakePush) Push(_ context.Context, token string, _ []byte) (apns.Result, error) {
	f.calls = append(f.calls, token)
	if r, ok := f.results[token]; ok {
		return r, nil
	}
	return apns.Result{StatusCode: 200}, nil
}

type fakeStore struct {
	tokens  []string
	deleted []string
}

func (f *fakeStore) TokensByOrg(_ context.Context, _ uuid.UUID) ([]string, error) {
	return f.tokens, nil
}

func (f *fakeStore) Delete(_ context.Context, token string) error {
	f.deleted = append(f.deleted, token)
	return nil
}

func TestAPNsSendPrunesDeadTokens(t *testing.T) {
	push := &fakePush{results: map[string]apns.Result{
		"good": {StatusCode: 200},
		"dead": {StatusCode: 410, Reason: "Unregistered"},
	}}
	store := &fakeStore{tokens: []string{"good", "dead"}}
	n := &APNsNotifier{client: push, store: store}

	err := n.Send(context.Background(),
		notification.Decrypted{Type: notification.TypeAPNs, OrgID: uuid.New()},
		notification.Message{Status: notification.StatusFiring, Severity: "critical", MonitorName: "api"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(push.calls) != 2 {
		t.Fatalf("expected 2 pushes, got %d", len(push.calls))
	}
	if len(store.deleted) != 1 || store.deleted[0] != "dead" {
		t.Fatalf("expected only the dead token pruned, got %v", store.deleted)
	}
}

func TestAPNsSendNoTokensIsNotAnError(t *testing.T) {
	n := &APNsNotifier{client: &fakePush{}, store: &fakeStore{tokens: nil}}
	err := n.Send(context.Background(),
		notification.Decrypted{Type: notification.TypeAPNs, OrgID: uuid.New()},
		notification.Message{})
	if err != nil {
		t.Fatalf("Send with no tokens should be nil, got %v", err)
	}
}

func TestAPNsSendRequiresOrgID(t *testing.T) {
	n := &APNsNotifier{client: &fakePush{}, store: &fakeStore{}}
	if err := n.Send(context.Background(), notification.Decrypted{Type: notification.TypeAPNs}, notification.Message{}); err == nil {
		t.Fatal("expected an error when the channel has no org id")
	}
}

func TestBuildPayloadTitles(t *testing.T) {
	cases := []struct {
		name      string
		msg       notification.Message
		wantTitle string
	}{
		{"critical firing", notification.Message{Status: notification.StatusFiring, Severity: "critical", MonitorName: "api"}, "🔴 Down: api"},
		{"warning firing", notification.Message{Status: notification.StatusFiring, Severity: "warning", MonitorName: "api"}, "🟠 Alert: api"},
		{"resolved", notification.Message{Status: notification.StatusResolved, MonitorName: "api"}, "✅ Resolved: api"},
		{"test", notification.Message{IsTest: true}, "Test notification"},
	}
	for _, c := range cases {
		if got := buildPayload(c.msg).APS.Alert.Title; got != c.wantTitle {
			t.Errorf("%s: title=%q want %q", c.name, got, c.wantTitle)
		}
	}
}

func TestBuildPayloadCarriesMonitorIDForDeepLink(t *testing.T) {
	raw, _ := json.Marshal(buildPayload(notification.Message{MonitorID: "abc", Description: "down"}))
	if !strings.Contains(string(raw), `"monitor_id":"abc"`) {
		t.Errorf("payload missing monitor_id for deep-link: %s", raw)
	}
}
