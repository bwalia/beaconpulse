package notifier

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"beacon/internal/domain/notification"
)

// recordingNotifier stands in for the real EmailNotifier so the test asserts
// what the fallback would send without opening an SMTP connection.
type recordingNotifier struct {
	sent bool
	dec  notification.Decrypted
	err  error
}

func (r *recordingNotifier) Type() notification.ChannelType { return notification.TypeEmail }
func (r *recordingNotifier) Send(_ context.Context, ch notification.Decrypted, _ notification.Message) error {
	r.sent = true
	r.dec = ch
	return r.err
}

type fakeLookup struct {
	emails []string
	err    error
}

func (f fakeLookup) AlertRecipients(context.Context, uuid.UUID) ([]string, error) {
	return f.emails, f.err
}

func newFallback(rec notification.Notifier, lookup OrgEmailLookup) *DefaultEmailNotifier {
	return &DefaultEmailNotifier{
		cfg:    DefaultEmailConfig{Host: "smtp.relay", Port: "587", From: "alerts@beacon", Password: "sekret", Security: "starttls"},
		email:  rec,
		lookup: lookup,
	}
}

func TestFallbackSendsToResolvedRecipients(t *testing.T) {
	rec := &recordingNotifier{}
	fb := newFallback(rec, fakeLookup{emails: []string{"owner@acme.com", "admin@acme.com"}})

	if err := fb.Fallback(context.Background(), uuid.New(), notification.Message{MonitorName: "API"}); err != nil {
		t.Fatalf("Fallback: %v", err)
	}
	if !rec.sent {
		t.Fatal("email notifier was not invoked")
	}
	if got := rec.dec.Config["to"]; got != "owner@acme.com,admin@acme.com" {
		t.Errorf("to = %q, want the joined recipient list", got)
	}
	if rec.dec.Config["host"] != "smtp.relay" || rec.dec.Secret != "sekret" {
		t.Errorf("relay config not passed through: %+v", rec.dec.Config)
	}
}

func TestFallbackNoRecipientsIsNoop(t *testing.T) {
	rec := &recordingNotifier{}
	fb := newFallback(rec, fakeLookup{emails: nil})

	if err := fb.Fallback(context.Background(), uuid.New(), notification.Message{}); err != nil {
		t.Fatalf("Fallback with no recipients should be a no-op, got %v", err)
	}
	if rec.sent {
		t.Error("must not send when there are no recipients")
	}
}

func TestFallbackLookupErrorSurfaces(t *testing.T) {
	rec := &recordingNotifier{}
	fb := newFallback(rec, fakeLookup{err: errors.New("db down")})

	if err := fb.Fallback(context.Background(), uuid.New(), notification.Message{}); err == nil {
		t.Fatal("a lookup error must surface")
	}
	if rec.sent {
		t.Error("must not send when recipient lookup failed")
	}
}

func TestFallbackRejectsNilOrg(t *testing.T) {
	rec := &recordingNotifier{}
	fb := newFallback(rec, fakeLookup{emails: []string{"x@y.com"}})

	if err := fb.Fallback(context.Background(), uuid.Nil, notification.Message{}); err == nil {
		t.Fatal("nil org id must be rejected")
	}
}
