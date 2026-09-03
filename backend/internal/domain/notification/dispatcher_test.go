package notification

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"beacon/internal/platform/crypto"
)

// stubAnalyzer is a controllable Analyzer for dispatcher tests.
type stubAnalyzer struct {
	result *AlertAnalysis
	err    error
	calls  int
}

func (s *stubAnalyzer) Analyze(_ context.Context, _ AlertEvent) (*AlertAnalysis, error) {
	s.calls++
	return s.result, s.err
}

// newDispatcherWith wires a dispatcher over a fake repo/notifier with one enabled
// telegram channel for orgID, returning the notifier so tests can inspect what
// was delivered.
func newDispatcherWith(t *testing.T, orgID uuid.UUID, analyzer Analyzer) (*Dispatcher, *fakeNotifier) {
	t.Helper()
	cipher, err := crypto.NewCipher(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	repo := newFakeChannelRepo()
	repo.channels[uuid.New()] = &Channel{
		ID: uuid.New(), OrgID: orgID, Name: "Ops", Type: TypeTelegram,
		Enabled: true, Config: map[string]string{"chat_id": "1"},
	}
	notif := &fakeNotifier{}
	registry := map[ChannelType]Notifier{TypeTelegram: notif}
	d := NewDispatcher(repo, cipher, registry, nil, noopRecorder{}, nil, "http://dash", analyzer, time.Second, nil)
	return d, notif
}

// fakeFallback records the last fallback delivery so tests can assert the safety
// net fired (or did not).
type fakeFallback struct {
	called bool
	orgID  uuid.UUID
	msg    Message
	err    error
}

func (f *fakeFallback) Fallback(_ context.Context, orgID uuid.UUID, msg Message) error {
	f.called = true
	f.orgID = orgID
	f.msg = msg
	return f.err
}

func firingEvent(orgID uuid.UUID) AlertEvent {
	return AlertEvent{Status: StatusFiring, OrgID: orgID, AlertName: "Down", MonitorName: "API"}
}

func TestDispatchEnrichesFiringAlert(t *testing.T) {
	org := uuid.New()
	analysis := &AlertAnalysis{Severity: AISeverityHigh, Summary: "down", SuggestedFix: "restart"}
	an := &stubAnalyzer{result: analysis}
	d, notif := newDispatcherWith(t, org, an)

	d.DispatchAlerts(context.Background(), []AlertEvent{firingEvent(org)})

	if !notif.called {
		t.Fatal("notifier was not called")
	}
	if an.calls != 1 {
		t.Errorf("analyzer called %d times, want 1", an.calls)
	}
	if notif.msg.Analysis == nil || notif.msg.Analysis.Severity != AISeverityHigh {
		t.Errorf("delivered message not enriched: %+v", notif.msg.Analysis)
	}
}

func TestDispatchDoesNotEnrichResolvedAlert(t *testing.T) {
	org := uuid.New()
	an := &stubAnalyzer{result: &AlertAnalysis{Severity: AISeverityLow}}
	d, notif := newDispatcherWith(t, org, an)

	ev := firingEvent(org)
	ev.Status = StatusResolved
	d.DispatchAlerts(context.Background(), []AlertEvent{ev})

	if an.calls != 0 {
		t.Errorf("analyzer should not run for resolved alerts, ran %d times", an.calls)
	}
	if notif.msg.Analysis != nil {
		t.Error("resolved message should have no analysis")
	}
}

func TestDispatchDeliversWhenAnalyzerFails(t *testing.T) {
	org := uuid.New()
	an := &stubAnalyzer{err: errors.New("model down")}
	d, notif := newDispatcherWith(t, org, an)

	d.DispatchAlerts(context.Background(), []AlertEvent{firingEvent(org)})

	if !notif.called {
		t.Fatal("delivery must still happen when the analyzer fails")
	}
	if notif.msg.Analysis != nil {
		t.Error("failed analysis must not attach partial data")
	}
}

func TestDispatchNilAnalyzerDelivers(t *testing.T) {
	org := uuid.New()
	d, notif := newDispatcherWith(t, org, nil)

	d.DispatchAlerts(context.Background(), []AlertEvent{firingEvent(org)})

	if !notif.called {
		t.Fatal("delivery must happen with no analyzer configured")
	}
	if notif.msg.Analysis != nil {
		t.Error("no analyzer means no analysis")
	}
}

func TestDispatchFallbackWhenNoChannels(t *testing.T) {
	cipher, err := crypto.NewCipher(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	org := uuid.New()
	repo := newFakeChannelRepo() // org has no channels
	fb := &fakeFallback{}
	d := NewDispatcher(repo, cipher, map[ChannelType]Notifier{}, nil, noopRecorder{}, nil, "http://dash", nil, time.Second, fb)

	d.DispatchAlerts(context.Background(), []AlertEvent{firingEvent(org)})

	if !fb.called {
		t.Fatal("fallback must fire when the org has no channels")
	}
	if fb.orgID != org {
		t.Errorf("fallback org = %v, want %v", fb.orgID, org)
	}
	if fb.msg.MonitorName != "API" {
		t.Errorf("fallback message not rendered: %+v", fb.msg)
	}
}

func TestDispatchNoFallbackWhenChannelExists(t *testing.T) {
	org := uuid.New()
	d, notif := newDispatcherWith(t, org, nil) // one enabled telegram channel
	fb := &fakeFallback{}
	d.fallback = fb

	d.DispatchAlerts(context.Background(), []AlertEvent{firingEvent(org)})

	if !notif.called {
		t.Fatal("configured channel should have received the alert")
	}
	if fb.called {
		t.Error("fallback must NOT fire when a channel is configured")
	}
}

func TestDispatchNoFallbackWhenUnconfigured(t *testing.T) {
	cipher, err := crypto.NewCipher(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	org := uuid.New()
	repo := newFakeChannelRepo() // no channels, and no fallback wired
	d := NewDispatcher(repo, cipher, map[ChannelType]Notifier{}, nil, noopRecorder{}, nil, "http://dash", nil, time.Second, nil)

	// Must not panic on a nil fallback — this is the previous behaviour: an org
	// with no channels simply gets no alert.
	d.DispatchAlerts(context.Background(), []AlertEvent{firingEvent(org)})
}
