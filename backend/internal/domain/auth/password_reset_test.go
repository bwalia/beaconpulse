package auth

import (
	"context"
	"net/url"
	"testing"

	"beacon/internal/platform/apperror"
)

type fakeMailer struct{ sent []string } // links

func (m *fakeMailer) SendPasswordReset(_ context.Context, _, _, link string) error {
	m.sent = append(m.sent, link)
	return nil
}

func resetFixture(t *testing.T) (*Service, *fakeMailer) {
	t.Helper()
	m := &fakeMailer{}
	svc := newTestService().WithPasswordReset(m, "https://app.example.com/")
	svc.async = func(f func()) { f() } // run the send inline so the test can see it
	if _, err := svc.Register(context.Background(), RegisterInput{
		OrgName: "Acme", Name: "Jane", Email: "jane@example.com", Password: "oldpassword",
	}, RequestMeta{}); err != nil {
		t.Fatalf("register: %v", err)
	}
	return svc, m
}

func tokenFrom(t *testing.T, link string) string {
	t.Helper()
	u, err := url.Parse(link)
	if err != nil || u.Path != "/reset-password" {
		t.Fatalf("bad reset link %q", link)
	}
	return u.Query().Get("token")
}

func TestPasswordReset_FullFlowAndSingleUse(t *testing.T) {
	svc, m := resetFixture(t)
	ctx := context.Background()

	if err := svc.RequestPasswordReset(ctx, " JANE@example.com ", RequestMeta{}); err != nil {
		t.Fatalf("request: %v", err)
	}
	if len(m.sent) != 1 {
		t.Fatalf("want one reset email, got %d", len(m.sent))
	}
	token := tokenFrom(t, m.sent[0])

	if err := svc.ResetPassword(ctx, token, "newpassword", RequestMeta{}); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if _, err := svc.Login(ctx, "jane@example.com", "newpassword", RequestMeta{}); err != nil {
		t.Fatalf("login with new password: %v", err)
	}
	if _, err := svc.Login(ctx, "jane@example.com", "oldpassword", RequestMeta{}); err == nil {
		t.Fatal("old password must stop working")
	}
	// The password changed, so the same link is now dead.
	if err := svc.ResetPassword(ctx, token, "anotherpass", RequestMeta{}); !apperror.IsCode(err, apperror.CodeValidation) {
		t.Fatalf("a used reset link must be rejected, got %v", err)
	}
}

func TestPasswordReset_UnknownEmailIsSilent(t *testing.T) {
	svc, m := resetFixture(t)
	if err := svc.RequestPasswordReset(context.Background(), "nobody@example.com", RequestMeta{}); err != nil {
		t.Fatalf("unknown email must not error (no account enumeration), got %v", err)
	}
	if len(m.sent) != 0 {
		t.Fatal("no email may be sent for an unknown address")
	}
}

func TestPasswordReset_GarbageTokenRejected(t *testing.T) {
	svc, _ := resetFixture(t)
	if err := svc.ResetPassword(context.Background(), "not-a-token", "newpassword", RequestMeta{}); !apperror.IsCode(err, apperror.CodeValidation) {
		t.Fatalf("want validation error, got %v", err)
	}
}

func TestPasswordReset_DisabledWithoutMailer(t *testing.T) {
	err := newTestService().RequestPasswordReset(context.Background(), "jane@example.com", RequestMeta{})
	if !apperror.IsCode(err, apperror.CodeUnavailable) {
		t.Fatalf("want unavailable when email is not configured, got %v", err)
	}
}
