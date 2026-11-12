package notifier

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"beacon/internal/adapter/fcm"
	"beacon/internal/domain/device"
	"beacon/internal/domain/notification"
)

// fakeFCM answers per registration token and records each request body.
type fakeFCM struct {
	results map[string]fcm.Result
	errs    map[string]error
	bodies  []map[string]fcmMessage
}

func (f *fakeFCM) Send(_ context.Context, body []byte) (fcm.Result, error) {
	var req map[string]fcmMessage
	if err := json.Unmarshal(body, &req); err != nil {
		return fcm.Result{}, err
	}
	f.bodies = append(f.bodies, req)
	tok := req["message"].Token
	if err, ok := f.errs[tok]; ok {
		return fcm.Result{}, err
	}
	if r, ok := f.results[tok]; ok {
		return r, nil
	}
	return fcm.Result{StatusCode: 200}, nil
}

func firing() notification.Message {
	return notification.Message{Status: notification.StatusFiring, Severity: "critical",
		MonitorName: "api", MonitorID: "mon-1", Description: "connection refused"}
}

func TestFCMSendFansOutAndPrunesOnlyDeadTokens(t *testing.T) {
	client := &fakeFCM{
		results: map[string]fcm.Result{
			"unregistered": {StatusCode: 404, Code: "UNREGISTERED"},
			"bad-token":    {StatusCode: 400, Code: "INVALID_ARGUMENT", Field: "message.token"},
			"quota":        {StatusCode: 429, Code: "QUOTA_EXCEEDED"},
			"auth":         {StatusCode: 401, Code: "UNAUTHENTICATED"},
		},
		errs: map[string]error{"network": errors.New("timeout")},
	}
	store := &fakeStore{tokens: []string{"good", "unregistered", "bad-token", "quota", "auth", "network"}}
	n := &FCMNotifier{client: client, store: store}

	if err := n.Send(context.Background(), notification.Decrypted{Type: notification.TypeFCM, OrgID: uuid.New()}, firing()); err != nil {
		t.Fatalf("Send: %v (one token delivered, so no error)", err)
	}
	if store.platform != device.PlatformAndroid {
		t.Errorf("FCM must read android tokens, read %q", store.platform)
	}
	if len(client.bodies) != 6 {
		t.Fatalf("expected 6 sends, got %d", len(client.bodies))
	}
	for i, tok := range store.tokens {
		if got := client.bodies[i]["message"].Token; got != tok {
			t.Errorf("send %d went to %q, want %q", i, got, tok)
		}
	}
	if len(store.deleted) != 2 || store.deleted[0] != "unregistered" || store.deleted[1] != "bad-token" {
		t.Fatalf("expected only unregistered + bad-token pruned, got %v", store.deleted)
	}
}

func TestFCMSendErrorsWhenNothingDelivered(t *testing.T) {
	client := &fakeFCM{results: map[string]fcm.Result{"a": {StatusCode: 503, Code: "UNAVAILABLE"}}}
	n := &FCMNotifier{client: client, store: &fakeStore{tokens: []string{"a"}}}
	if err := n.Send(context.Background(), notification.Decrypted{OrgID: uuid.New()}, firing()); err == nil {
		t.Fatal("expected an error when no token was delivered")
	}
}

func TestFCMSendNoTokensAndNoOrg(t *testing.T) {
	n := &FCMNotifier{client: &fakeFCM{}, store: &fakeStore{}}
	if err := n.Send(context.Background(), notification.Decrypted{OrgID: uuid.New()}, firing()); err != nil {
		t.Fatalf("no tokens should be nil, got %v", err)
	}
	if err := n.Send(context.Background(), notification.Decrypted{}, firing()); err == nil {
		t.Fatal("expected an error when the channel has no org id")
	}
}

// TestFCMPayloadShape pins the v1 wire format the Android app relies on.
func TestFCMPayloadShape(t *testing.T) {
	client := &fakeFCM{}
	n := &FCMNotifier{client: client, store: &fakeStore{tokens: []string{"tok"}}}
	_ = n.Send(context.Background(), notification.Decrypted{OrgID: uuid.New()}, firing())

	raw, _ := json.Marshal(client.bodies[0])
	var got map[string]any
	_ = json.Unmarshal(raw, &got)
	want := map[string]any{"message": map[string]any{
		"token":        "tok",
		"notification": map[string]any{"title": "🔴 Down: api", "body": "connection refused"},
		"data":         map[string]any{"monitor_id": "mon-1", "status": "firing"},
		"android": map[string]any{
			"priority":     "high",
			"notification": map[string]any{"channel_id": "alerts"},
		},
	}}
	wantRaw, _ := json.Marshal(want)
	gotRaw, _ := json.Marshal(got)
	if string(gotRaw) != string(wantRaw) {
		t.Errorf("payload\n got %s\nwant %s", gotRaw, wantRaw)
	}
}
