package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/audit"
	"beacon/internal/platform/apperror"
	"beacon/internal/platform/crypto"
)

// ---- in-memory fakes ----

type fakeUserRepo struct {
	orgsBySlug map[string]*Organization
	usersByID  map[uuid.UUID]*User
	byEmail    map[string]*User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		orgsBySlug: map[string]*Organization{},
		usersByID:  map[uuid.UUID]*User{},
		byEmail:    map[string]*User{},
	}
}

func (f *fakeUserRepo) CreateOrgAndOwner(_ context.Context, org *Organization, owner *User) error {
	if _, ok := f.byEmail[owner.Email]; ok {
		return apperror.Conflict("an account with that email already exists")
	}
	if _, ok := f.orgsBySlug[org.Slug]; ok {
		return apperror.Conflict("org exists")
	}
	f.orgsBySlug[org.Slug] = org
	f.usersByID[owner.ID] = owner
	f.byEmail[owner.Email] = owner
	return nil
}

func (f *fakeUserRepo) GetUserByID(_ context.Context, id uuid.UUID) (*User, error) {
	u, ok := f.usersByID[id]
	if !ok {
		return nil, apperror.NotFound("user not found")
	}
	return u, nil
}

func (f *fakeUserRepo) GetUserByEmail(_ context.Context, email string) (*User, error) {
	u, ok := f.byEmail[email]
	if !ok {
		return nil, apperror.NotFound("user not found")
	}
	return u, nil
}

func (f *fakeUserRepo) TouchLastLogin(_ context.Context, id uuid.UUID) error {
	if u, ok := f.usersByID[id]; ok {
		now := time.Now()
		u.LastLoginAt = &now
	}
	return nil
}

func (f *fakeUserRepo) SlugExists(_ context.Context, slug string) (bool, error) {
	_, ok := f.orgsBySlug[slug]
	return ok, nil
}

func (f *fakeUserRepo) LinkGoogleSub(_ context.Context, id uuid.UUID, googleSub string) error {
	u, ok := f.usersByID[id]
	if !ok {
		return apperror.NotFound("user not found")
	}
	u.GoogleSub = googleSub
	return nil
}

type fakeRefreshRepo struct {
	byHash map[string]*RefreshToken
	byID   map[uuid.UUID]*RefreshToken
}

func newFakeRefreshRepo() *fakeRefreshRepo {
	return &fakeRefreshRepo{byHash: map[string]*RefreshToken{}, byID: map[uuid.UUID]*RefreshToken{}}
}

func (f *fakeRefreshRepo) Create(_ context.Context, t *RefreshToken) error {
	f.byHash[t.TokenHash] = t
	f.byID[t.ID] = t
	return nil
}
func (f *fakeRefreshRepo) GetByHash(_ context.Context, hash string) (*RefreshToken, error) {
	t, ok := f.byHash[hash]
	if !ok {
		return nil, apperror.NotFound("refresh token not found")
	}
	return t, nil
}
func (f *fakeRefreshRepo) Revoke(_ context.Context, id uuid.UUID) error {
	if t, ok := f.byID[id]; ok {
		now := time.Now()
		t.RevokedAt = &now
	}
	return nil
}
func (f *fakeRefreshRepo) RevokeAllForUser(_ context.Context, _ uuid.UUID) error { return nil }
func (f *fakeRefreshRepo) DeleteExpired(_ context.Context) (int64, error)        { return 0, nil }

type noopRecorder struct{}

func (noopRecorder) Record(_ context.Context, _ audit.Entry) error { return nil }

func newTestService() *Service {
	return NewService(
		newFakeUserRepo(),
		newFakeRefreshRepo(),
		testTokenManager(),
		crypto.NewPasswordHasher(4),
		noopRecorder{},
	)
}

// ---- tests ----

func TestRegisterAndLogin(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	meta := RequestMeta{IP: "127.0.0.1", UserAgent: "test"}

	res, err := svc.Register(ctx, RegisterInput{
		OrgName: "Acme Inc", Name: "Jane", Email: "Jane@Example.com", Password: "supersecret",
	}, meta)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if res.User.Role != RoleOwner {
		t.Errorf("role = %q, want owner", res.User.Role)
	}
	if res.User.Email != "jane@example.com" {
		t.Errorf("email not normalized: %q", res.User.Email)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatal("expected tokens to be issued")
	}

	// Login with correct credentials (case-insensitive email).
	if _, err := svc.Login(ctx, "jane@example.com", "supersecret", meta); err != nil {
		t.Fatalf("Login: %v", err)
	}
	// Wrong password.
	if _, err := svc.Login(ctx, "jane@example.com", "wrong", meta); !apperror.IsCode(err, apperror.CodeUnauthorized) {
		t.Fatalf("expected unauthorized, got %v", err)
	}
	// Unknown email.
	if _, err := svc.Login(ctx, "nobody@example.com", "x", meta); !apperror.IsCode(err, apperror.CodeUnauthorized) {
		t.Fatalf("expected unauthorized for unknown email, got %v", err)
	}
}

func TestRegisterRejectsShortPassword(t *testing.T) {
	svc := newTestService()
	_, err := svc.Register(context.Background(), RegisterInput{
		OrgName: "Acme", Name: "J", Email: "a@b.com", Password: "short",
	}, RequestMeta{})
	if !apperror.IsCode(err, apperror.CodeValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	in := RegisterInput{OrgName: "Acme", Name: "J", Email: "a@b.com", Password: "supersecret"}
	if _, err := svc.Register(ctx, in, RequestMeta{}); err != nil {
		t.Fatalf("first register: %v", err)
	}
	in.OrgName = "Other"
	if _, err := svc.Register(ctx, in, RequestMeta{}); !apperror.IsCode(err, apperror.CodeConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestRefreshRotatesToken(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	res, err := svc.Register(ctx, RegisterInput{
		OrgName: "Acme", Name: "J", Email: "a@b.com", Password: "supersecret",
	}, RequestMeta{})
	if err != nil {
		t.Fatal(err)
	}

	refreshed, err := svc.Refresh(ctx, res.RefreshToken, RequestMeta{})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if refreshed.RefreshToken == res.RefreshToken {
		t.Fatal("expected a rotated (new) refresh token")
	}
	// The old token must no longer be usable.
	if _, err := svc.Refresh(ctx, res.RefreshToken, RequestMeta{}); !apperror.IsCode(err, apperror.CodeUnauthorized) {
		t.Fatalf("expected old refresh token to be revoked, got %v", err)
	}
}

// ---- Google sign-in ----

type fakeGoogleVerifier struct {
	id  *GoogleIdentity
	err error
}

func (f fakeGoogleVerifier) Verify(_ context.Context, _ string) (*GoogleIdentity, error) {
	return f.id, f.err
}

func TestLoginWithGoogleCreatesOrgForNewUser(t *testing.T) {
	svc := newTestService().WithGoogle(fakeGoogleVerifier{
		id: &GoogleIdentity{Subject: "google-123", Email: "New@Example.com", EmailVerified: true, Name: "New Person"},
	})
	res, err := svc.LoginWithGoogle(context.Background(), "tok", RequestMeta{})
	if err != nil {
		t.Fatalf("LoginWithGoogle: %v", err)
	}
	if res.User.Role != RoleOwner {
		t.Errorf("role = %s, want owner", res.User.Role)
	}
	if res.User.Email != "new@example.com" {
		t.Errorf("email not normalized: %q", res.User.Email)
	}
	if res.User.GoogleSub != "google-123" {
		t.Errorf("google_sub = %q, want google-123", res.User.GoogleSub)
	}
	if res.User.PasswordHash != "" {
		t.Errorf("google-only user should have no password hash")
	}
	if res.AccessToken == "" {
		t.Errorf("expected an access token")
	}
}

func TestLoginWithGoogleLinksExistingPasswordUser(t *testing.T) {
	svc := newTestService().WithGoogle(fakeGoogleVerifier{
		id: &GoogleIdentity{Subject: "google-xyz", Email: "jane@example.com", EmailVerified: true, Name: "Jane"},
	})
	ctx := context.Background()
	reg, err := svc.Register(ctx, RegisterInput{OrgName: "Acme", Name: "Jane", Email: "jane@example.com", Password: "supersecret"}, RequestMeta{})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	res, err := svc.LoginWithGoogle(ctx, "tok", RequestMeta{})
	if err != nil {
		t.Fatalf("LoginWithGoogle: %v", err)
	}
	if res.User.ID != reg.User.ID {
		t.Errorf("logged into a different user; expected the existing account")
	}
	if res.User.GoogleSub != "google-xyz" {
		t.Errorf("existing user not linked to google")
	}
}

func TestLoginWithGoogleRejectsUnverifiedEmail(t *testing.T) {
	svc := newTestService().WithGoogle(fakeGoogleVerifier{
		id: &GoogleIdentity{Subject: "s", Email: "x@example.com", EmailVerified: false, Name: "X"},
	})
	if _, err := svc.LoginWithGoogle(context.Background(), "tok", RequestMeta{}); err == nil {
		t.Fatal("expected rejection for an unverified Google email")
	}
}

func TestLoginWithGoogleDisabled(t *testing.T) {
	svc := newTestService() // no WithGoogle
	if _, err := svc.LoginWithGoogle(context.Background(), "tok", RequestMeta{}); err == nil {
		t.Fatal("expected error when Google sign-in is not configured")
	}
}

func TestPasswordLoginRejectedForGoogleOnlyUser(t *testing.T) {
	svc := newTestService().WithGoogle(fakeGoogleVerifier{
		id: &GoogleIdentity{Subject: "s", Email: "g@example.com", EmailVerified: true, Name: "G"},
	})
	ctx := context.Background()
	if _, err := svc.LoginWithGoogle(ctx, "tok", RequestMeta{}); err != nil {
		t.Fatalf("seed google user: %v", err)
	}
	if _, err := svc.Login(ctx, "g@example.com", "whatever", RequestMeta{}); err == nil {
		t.Fatal("expected password login to fail for a Google-only account")
	}
}
