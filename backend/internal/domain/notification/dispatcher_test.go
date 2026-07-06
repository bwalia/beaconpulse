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
	d := NewDispatcher(repo, cipher, registry, nil, noopRecorder{}, "http://dash", analyzer, time.Second)
	return d, notif
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
