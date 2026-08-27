package device

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type fakeRepo struct {
	upserted *Device
	deleted  string
}

func (f *fakeRepo) Upsert(_ context.Context, d *Device) error { f.upserted = d; return nil }
func (f *fakeRepo) DeleteByToken(_ context.Context, _ uuid.UUID, token string) error {
	f.deleted = token
	return nil
}

type fakeActivator struct {
	called bool
	err    error
}

func (f *fakeActivator) EnsureAPNsChannel(_ context.Context, _ uuid.UUID) error {
	f.called = true
	return f.err
}

func actor() Actor { return Actor{UserID: uuid.New(), OrgID: uuid.New()} }

func TestRegisterDefaultsPlatformAndActivates(t *testing.T) {
	repo := &fakeRepo{}
	act := &fakeActivator{}
	svc := NewService(repo, act)

	d, err := svc.Register(context.Background(), actor(), RegisterInput{Token: "  abc123  "})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if d.Platform != PlatformIOS {
		t.Errorf("platform=%q want ios", d.Platform)
	}
	if repo.upserted == nil || repo.upserted.Token != "abc123" {
		t.Errorf("token not trimmed/stored: %+v", repo.upserted)
	}
	if !act.called {
		t.Error("expected the push channel to be auto-enabled")
	}
}

func TestRegisterRejectsEmptyToken(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, &fakeActivator{})
	if _, err := svc.Register(context.Background(), actor(), RegisterInput{Token: "   "}); err == nil {
		t.Fatal("expected a validation error for an empty token")
	}
	if repo.upserted != nil {
		t.Error("must not persist on validation failure")
	}
}

func TestRegisterRejectsUnknownPlatform(t *testing.T) {
	svc := NewService(&fakeRepo{}, &fakeActivator{})
	if _, err := svc.Register(context.Background(), actor(), RegisterInput{Token: "abc", Platform: "windows"}); err == nil {
		t.Fatal("expected a validation error for an unsupported platform")
	}
}

func TestRegisterSucceedsWhenAutoEnableFails(t *testing.T) {
	repo := &fakeRepo{}
	act := &fakeActivator{err: errors.New("db down")}
	svc := NewService(repo, act)

	// Auto-enable is best-effort: a failure there must not fail registration.
	if _, err := svc.Register(context.Background(), actor(), RegisterInput{Token: "abc"}); err != nil {
		t.Fatalf("Register should succeed despite auto-enable failure: %v", err)
	}
	if repo.upserted == nil {
		t.Error("device should still be registered")
	}
}

func TestRegisterWithoutActivator(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, nil) // nil activator must not panic
	if _, err := svc.Register(context.Background(), actor(), RegisterInput{Token: "abc"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
}

func TestUnregisterRejectsEmptyToken(t *testing.T) {
	svc := NewService(&fakeRepo{}, nil)
	if err := svc.Unregister(context.Background(), actor(), "  "); err == nil {
		t.Fatal("expected a validation error for an empty token")
	}
}
