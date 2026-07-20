package apikey

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/audit"
	"beacon/internal/domain/auth"
	"beacon/internal/platform/apperror"
)

type fakeRepo struct {
	byHash  map[string]*Key
	created []*Key
	touched int
}

func newRepo() *fakeRepo { return &fakeRepo{byHash: map[string]*Key{}} }

func (f *fakeRepo) Create(_ context.Context, k *Key, hash string) error {
	f.byHash[hash] = k
	f.created = append(f.created, k)
	return nil
}
func (f *fakeRepo) ByHash(_ context.Context, hash string) (*Key, error) { return f.byHash[hash], nil }
func (f *fakeRepo) List(context.Context, uuid.UUID) ([]Key, error)      { return nil, nil }
func (f *fakeRepo) Revoke(_ context.Context, _, id uuid.UUID, at time.Time) error {
	for _, k := range f.byHash {
		if k.ID == id {
			k.RevokedAt = &at
		}
	}
	return nil
}
func (f *fakeRepo) TouchLastUsed(context.Context, uuid.UUID, time.Time) error {
	f.touched++
	return nil
}

type noopRecorder struct{}

func (noopRecorder) Record(context.Context, audit.Entry) error { return nil }

func svc(repo Repository) *Service { return NewService(repo, noopRecorder{}) }

func owner() Actor {
	return Actor{UserID: uuid.New(), OrgID: uuid.New(), Role: auth.RoleOwner}
}

// TestGenerateProducesAnUnguessableKey — the entropy is what justifies hashing with
// SHA-256 instead of bcrypt, so it is worth pinning rather than assuming.
func TestGenerateProducesAnUnguessableKey(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		secret, hash, prefix, err := Generate()
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if !strings.HasPrefix(secret, Prefix) {
			t.Fatalf("key %q lacks the scannable prefix", secret)
		}
		if seen[secret] {
			t.Fatal("Generate returned a duplicate key")
		}
		seen[secret] = true

		if len(secret) < 40 {
			t.Fatalf("key is only %d chars — too little entropy for a fast hash", len(secret))
		}
		if strings.Contains(hash, secret) || hash == secret {
			t.Fatal("the stored hash contains the secret")
		}
		if !strings.HasPrefix(secret, prefix) {
			t.Fatal("the display prefix is not a prefix of the key")
		}
	}
}

// TestVerifyAcceptsAValidKeyAndResolvesTheOrg is the core contract: a key resolves to
// an ORGANIZATION, which is what makes every downstream plan and billing rule apply
// without the key carrying any of that itself.
func TestVerifyAcceptsAValidKeyAndResolvesTheOrg(t *testing.T) {
	repo := newRepo()
	s := svc(repo)
	a := owner()

	_, secret, err := s.Issue(context.Background(), a, CreateInput{Name: "github-actions"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	v, err := s.Verify(context.Background(), secret)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if v.OrgID != a.OrgID {
		t.Fatalf("resolved org %s, want %s", v.OrgID, a.OrgID)
	}
	if v.Role != auth.RoleOwner {
		t.Fatalf("role = %s, want owner", v.Role)
	}
	if repo.touched == 0 {
		t.Error("last-used was not recorded")
	}
}

// TestVerifyRefusesRevokedAndExpiredKeys — revocation has to actually revoke, and an
// expiry has to actually expire. These are the two promises the UI makes.
func TestVerifyRefusesRevokedAndExpiredKeys(t *testing.T) {
	t.Run("revoked", func(t *testing.T) {
		repo := newRepo()
		s := svc(repo)
		a := owner()
		k, secret, _ := s.Issue(context.Background(), a, CreateInput{Name: "leaked"})

		if err := s.Revoke(context.Background(), a, k.ID); err != nil {
			t.Fatalf("Revoke: %v", err)
		}
		if _, err := s.Verify(context.Background(), secret); err == nil {
			t.Fatal("a revoked key still authenticated")
		}
	})

	t.Run("expired", func(t *testing.T) {
		repo := newRepo()
		s := svc(repo)
		a := owner()
		exp := time.Now().Add(time.Hour)
		_, secret, err := s.Issue(context.Background(), a, CreateInput{Name: "temp", ExpiresAt: &exp})
		if err != nil {
			t.Fatalf("Issue: %v", err)
		}
		// Walk past the expiry.
		s.now = func() time.Time { return exp.Add(time.Minute) }
		if _, err := s.Verify(context.Background(), secret); err == nil {
			t.Fatal("an expired key still authenticated")
		}
	})
}

// TestVerifyFailsIdenticallyForEveryReason — an attacker holding a random string, or a
// revoked key, must not be able to tell which it is. Distinguishing them says "that
// was real once", which is a hint worth denying.
func TestVerifyFailsIdenticallyForEveryReason(t *testing.T) {
	repo := newRepo()
	s := svc(repo)
	a := owner()
	k, secret, _ := s.Issue(context.Background(), a, CreateInput{Name: "k"})
	_ = s.Revoke(context.Background(), a, k.ID)

	_, revoked := s.Verify(context.Background(), secret)
	_, unknown := s.Verify(context.Background(), Prefix+"totallymadeupkeyvaluehere000000000000000")
	_, garbage := s.Verify(context.Background(), "not-even-our-format")

	if revoked == nil || unknown == nil || garbage == nil {
		t.Fatal("expected all three to fail")
	}
	if revoked.Error() != unknown.Error() || unknown.Error() != garbage.Error() {
		t.Fatalf("failures are distinguishable and leak which key was real:\n  revoked=%q\n  unknown=%q\n  garbage=%q",
			revoked, unknown, garbage)
	}
}

// TestIssueCapsRoleAtTheCreators — otherwise an admin mints an owner key and has
// quietly promoted themselves.
func TestIssueCapsRoleAtTheCreators(t *testing.T) {
	s := svc(newRepo())
	admin := Actor{UserID: uuid.New(), OrgID: uuid.New(), Role: auth.RoleAdmin}

	if _, _, err := s.Issue(context.Background(), admin, CreateInput{Name: "escalate", Role: auth.RoleOwner}); err == nil {
		t.Fatal("an admin minted an owner key — that is privilege escalation")
	}
	// A lesser role is fine, and is the common case: a read-only key for a dashboard.
	if _, _, err := s.Issue(context.Background(), admin, CreateInput{Name: "readonly", Role: auth.RoleViewer}); err != nil {
		t.Fatalf("admin should be able to mint a viewer key: %v", err)
	}
}

// TestIssueRequiresAdmin — a member with write access to monitors must not be able to
// mint a credential that outlives their own session.
func TestIssueRequiresAdmin(t *testing.T) {
	s := svc(newRepo())
	member := Actor{UserID: uuid.New(), OrgID: uuid.New(), Role: auth.RoleMember}

	_, _, err := s.Issue(context.Background(), member, CreateInput{Name: "nope"})
	if !apperror.IsCode(err, apperror.CodeForbidden) {
		t.Fatalf("expected forbidden for a member, got %v", err)
	}
}

// TestIssueRejectsAnAlreadyExpiredKey — a key that cannot ever work is a support
// ticket, not a credential.
func TestIssueRejectsAnAlreadyExpiredKey(t *testing.T) {
	s := svc(newRepo())
	past := time.Now().Add(-time.Hour)
	_, _, err := s.Issue(context.Background(), owner(), CreateInput{Name: "stale", ExpiresAt: &past})
	if !apperror.IsCode(err, apperror.CodeValidation) {
		t.Fatalf("expected validation, got %v", err)
	}
}
