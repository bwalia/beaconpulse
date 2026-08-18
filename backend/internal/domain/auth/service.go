package auth

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"beacon/internal/domain/audit"
	"beacon/internal/platform/apperror"
	"beacon/internal/platform/crypto"
	"beacon/internal/platform/slug"
)

// RequestMeta carries transport-level context (client IP, user agent) that the
// service records with refresh tokens and audit entries.
type RequestMeta struct {
	IP        string
	UserAgent string
}

// RegisterInput is the validated input for creating a new organization and its
// owner in one step.
type RegisterInput struct {
	OrgName  string
	Email    string
	Password string
	Name     string
}

// AuthResult is returned by register/login/refresh: a signed access token, an
// opaque refresh token, a gateway proxy token (for the httpOnly cookie), and the
// authenticated user.
type AuthResult struct {
	AccessToken  string
	RefreshToken string
	// ProxyToken is a long-lived, org-scoped token the transport layer stores in
	// an httpOnly cookie so the gateway can authorize the raw monitoring UIs.
	ProxyToken string
	TokenType  string
	ExpiresIn  int // access-token lifetime in seconds
	User       *User
}

// Service implements the authentication use cases. It depends only on
// interfaces, making it fully unit-testable with in-memory fakes.
type Service struct {
	users    UserRepository
	tokens   RefreshTokenRepository
	tm       *TokenManager
	hasher   *crypto.PasswordHasher
	auditlog audit.Recorder
	// emailPolicy vets signup addresses. Nil disables the checks.
	emailPolicy *EmailPolicy
	// google verifies Google ID tokens. Nil disables "Sign in with Google".
	google GoogleVerifier
	now    func() time.Time
}

// WithEmailPolicy vets the address a signup is made with. See emailpolicy.go for why
// this is a cost control rather than input validation.
func (s *Service) WithEmailPolicy(p *EmailPolicy) *Service {
	s.emailPolicy = p
	return s
}

// WithGoogle enables "Sign in with Google" using the given ID-token verifier.
func (s *Service) WithGoogle(v GoogleVerifier) *Service {
	s.google = v
	return s
}

// NewService wires the auth service.
func NewService(
	users UserRepository,
	tokens RefreshTokenRepository,
	tm *TokenManager,
	hasher *crypto.PasswordHasher,
	auditlog audit.Recorder,
) *Service {
	return &Service{
		users:    users,
		tokens:   tokens,
		tm:       tm,
		hasher:   hasher,
		auditlog: auditlog,
		now:      time.Now,
	}
}

// Register creates a new organization with an owner user and returns tokens so
// the caller is immediately signed in.
func (s *Service) Register(ctx context.Context, in RegisterInput, meta RequestMeta) (*AuthResult, error) {
	in.Email = normalizeEmail(in.Email)
	// Before the bcrypt, which is the expensive part of this handler: a rejected
	// signup should cost a DNS lookup, not a key derivation.
	if s.emailPolicy != nil {
		if err := s.emailPolicy.Check(ctx, in.Email); err != nil {
			return nil, err
		}
	}
	if len(in.Password) < 8 {
		return nil, apperror.Validation("password must be at least 8 characters",
			apperror.FieldError{Field: "password", Message: "must be at least 8 characters"})
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	orgSlug, err := s.uniqueOrgSlug(ctx, in.OrgName)
	if err != nil {
		return nil, err
	}

	now := s.now().UTC()
	org := &Organization{
		ID:        uuid.New(),
		Name:      strings.TrimSpace(in.OrgName),
		Slug:      orgSlug,
		CreatedAt: now,
		UpdatedAt: now,
	}
	owner := &User{
		ID:           uuid.New(),
		OrgID:        org.ID,
		Email:        in.Email,
		PasswordHash: hash,
		Name:         strings.TrimSpace(in.Name),
		Role:         RoleOwner,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.users.CreateOrgAndOwner(ctx, org, owner); err != nil {
		return nil, err // repository returns a conflict apperror on duplicate email
	}

	result, err := s.issueTokens(ctx, owner, meta)
	if err != nil {
		return nil, err
	}

	s.audit(ctx, owner, audit.ActionUserRegistered, "user", owner.ID.String(), meta, map[string]any{
		"org_id": org.ID.String(),
		"email":  owner.Email,
	})
	return result, nil
}

// Login verifies credentials and returns tokens. Failures are deliberately
// indistinguishable ("invalid email or password") to avoid user enumeration.
func (s *Service) Login(ctx context.Context, email, password string, meta RequestMeta) (*AuthResult, error) {
	email = normalizeEmail(email)
	user, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		if apperror.IsCode(err, apperror.CodeNotFound) {
			return nil, apperror.Unauthorized("invalid email or password")
		}
		return nil, err
	}
	// A federated-only account (Google, no password) can never satisfy a password
	// login. Say so plainly rather than "invalid password" — the user is on the login
	// form and the Google button is right there.
	if !user.HasPassword() {
		return nil, apperror.Unauthorized("this account uses Google sign-in — continue with Google")
	}
	ok, err := s.hasher.Verify(user.PasswordHash, password)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if !ok {
		return nil, apperror.Unauthorized("invalid email or password")
	}
	if !user.IsActive {
		return nil, apperror.Forbidden("this account has been deactivated")
	}

	result, err := s.issueTokens(ctx, user, meta)
	if err != nil {
		return nil, err
	}
	if err := s.users.TouchLastLogin(ctx, user.ID); err != nil {
		// Non-fatal: the login succeeded. Record but do not fail.
		s.audit(ctx, user, audit.ActionUserLogin, "user", user.ID.String(), meta, map[string]any{
			"touch_last_login_error": err.Error(),
		})
		return result, nil
	}
	s.audit(ctx, user, audit.ActionUserLogin, "user", user.ID.String(), meta, nil)
	return result, nil
}

// LoginWithGoogle verifies a Google ID token and signs the user in. On first sight of
// an email it creates a new organization + owner (open signup, mirroring Register); an
// existing account is signed in and linked to the Google identity. Google's
// email_verified stands in for the app's own email verification.
func (s *Service) LoginWithGoogle(ctx context.Context, idToken string, meta RequestMeta) (*AuthResult, error) {
	if s.google == nil {
		return nil, apperror.Forbidden("Google sign-in is not enabled")
	}
	id, err := s.google.Verify(ctx, idToken)
	if err != nil {
		return nil, apperror.Unauthorized("invalid Google sign-in")
	}
	// Guards against signing in as an address the Google account has not confirmed it
	// owns — Google only sets this for verified mailboxes.
	if !id.EmailVerified {
		return nil, apperror.Forbidden("your Google account's email is not verified")
	}
	email := normalizeEmail(id.Email)
	if email == "" {
		return nil, apperror.Unauthorized("Google did not return an email address")
	}

	user, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		if apperror.IsCode(err, apperror.CodeNotFound) {
			return s.registerGoogleUser(ctx, id, email, meta)
		}
		return nil, err
	}

	if !user.IsActive {
		return nil, apperror.Forbidden("this account has been deactivated")
	}
	// Link the Google identity the first time an existing (password) account uses it.
	if user.GoogleSub == "" {
		if err := s.users.LinkGoogleSub(ctx, user.ID, id.Subject); err != nil {
			return nil, err
		}
		user.GoogleSub = id.Subject
	}

	result, err := s.issueTokens(ctx, user, meta)
	if err != nil {
		return nil, err
	}
	_ = s.users.TouchLastLogin(ctx, user.ID) // non-fatal
	s.audit(ctx, user, audit.ActionUserLogin, "user", user.ID.String(), meta, map[string]any{"via": "google"})
	return result, nil
}

// registerGoogleUser creates a fresh organization + owner for a first-time Google
// sign-in (no password). The org is auto-named from the person's name; it can be
// renamed later.
func (s *Service) registerGoogleUser(ctx context.Context, id *GoogleIdentity, email string, meta RequestMeta) (*AuthResult, error) {
	name := strings.TrimSpace(id.Name)
	if name == "" {
		name = localPart(email)
	}
	orgName := deriveOrgName(name)

	orgSlug, err := s.uniqueOrgSlug(ctx, orgName)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	org := &Organization{ID: uuid.New(), Name: orgName, Slug: orgSlug, CreatedAt: now, UpdatedAt: now}
	owner := &User{
		ID:        uuid.New(),
		OrgID:     org.ID,
		Email:     email,
		GoogleSub: id.Subject,
		Name:      name,
		Role:      RoleOwner,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.users.CreateOrgAndOwner(ctx, org, owner); err != nil {
		return nil, err // conflict apperror on a racing duplicate email
	}
	result, err := s.issueTokens(ctx, owner, meta)
	if err != nil {
		return nil, err
	}
	s.audit(ctx, owner, audit.ActionUserRegistered, "user", owner.ID.String(), meta, map[string]any{
		"org_id": org.ID.String(),
		"email":  owner.Email,
		"via":    "google",
	})
	return result, nil
}

// LoginWithOIDC signs a user in from an external OIDC identity that the transport
// layer already obtained via the Authorization-Code flow + UserInfo (see
// internal/adapter/oidcclient). It creates an org + owner on first sight of the
// email — the same provisioning as Google sign-in, minus the provider-specific
// subject link (the trusted provider asserts a verified email, which is the join
// key). `provider` is a display/audit label ("OpsAPI").
//
// It is intentionally provider-agnostic so a second SSO provider needs no new
// service code — only new transport wiring.
func (s *Service) LoginWithOIDC(ctx context.Context, subject, email, name string, emailVerified bool, provider string, meta RequestMeta) (*AuthResult, error) {
	if !emailVerified {
		return nil, apperror.Forbidden("your " + provider + " account's email is not verified")
	}
	email = normalizeEmail(email)
	if email == "" {
		return nil, apperror.Unauthorized(provider + " did not return an email address")
	}

	user, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		if apperror.IsCode(err, apperror.CodeNotFound) {
			return s.registerOIDCUser(ctx, subject, email, name, provider, meta)
		}
		return nil, err
	}
	if !user.IsActive {
		return nil, apperror.Forbidden("this account has been deactivated")
	}

	result, err := s.issueTokens(ctx, user, meta)
	if err != nil {
		return nil, err
	}
	_ = s.users.TouchLastLogin(ctx, user.ID) // non-fatal
	s.audit(ctx, user, audit.ActionUserLogin, "user", user.ID.String(), meta, map[string]any{"via": provider})
	return result, nil
}

// registerOIDCUser creates a fresh organization + owner for a first-time SSO
// sign-in (no password), auto-naming the org from the person's name.
func (s *Service) registerOIDCUser(ctx context.Context, subject, email, name, provider string, meta RequestMeta) (*AuthResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = localPart(email)
	}
	orgName := deriveOrgName(name)

	orgSlug, err := s.uniqueOrgSlug(ctx, orgName)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	org := &Organization{ID: uuid.New(), Name: orgName, Slug: orgSlug, CreatedAt: now, UpdatedAt: now}
	owner := &User{
		ID:    uuid.New(),
		OrgID: org.ID,
		Email: email,
		// The provider subject is this account's credential — no password, no
		// Google. Satisfies ck_users_auth_method and links the identity.
		OidcSub:   subject,
		Name:      name,
		Role:      RoleOwner,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.users.CreateOrgAndOwner(ctx, org, owner); err != nil {
		return nil, err // conflict apperror on a racing duplicate email
	}
	result, err := s.issueTokens(ctx, owner, meta)
	if err != nil {
		return nil, err
	}
	s.audit(ctx, owner, audit.ActionUserRegistered, "user", owner.ID.String(), meta, map[string]any{
		"org_id": org.ID.String(),
		"email":  owner.Email,
		"via":    provider,
	})
	return result, nil
}

// Refresh rotates a refresh token: the presented token is revoked and a new
// access/refresh pair is issued. Reusing a revoked or expired token fails.
func (s *Service) Refresh(ctx context.Context, refreshPlaintext string, meta RequestMeta) (*AuthResult, error) {
	hash := s.tm.HashRefreshToken(refreshPlaintext)
	stored, err := s.tokens.GetByHash(ctx, hash)
	if err != nil {
		if apperror.IsCode(err, apperror.CodeNotFound) {
			return nil, apperror.Unauthorized("invalid or expired refresh token")
		}
		return nil, err
	}
	if !stored.IsUsable(s.now()) {
		return nil, apperror.Unauthorized("invalid or expired refresh token")
	}

	user, err := s.users.GetUserByID(ctx, stored.UserID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, apperror.Forbidden("this account has been deactivated")
	}

	// Rotate: revoke the presented token before issuing a replacement.
	if err := s.tokens.Revoke(ctx, stored.ID); err != nil {
		return nil, err
	}
	result, err := s.issueTokens(ctx, user, meta)
	if err != nil {
		return nil, err
	}
	s.audit(ctx, user, audit.ActionTokenRefreshed, "user", user.ID.String(), meta, nil)
	return result, nil
}

// Logout revokes the presented refresh token. It is idempotent: an unknown
// token is treated as already logged out.
func (s *Service) Logout(ctx context.Context, refreshPlaintext string) error {
	hash := s.tm.HashRefreshToken(refreshPlaintext)
	stored, err := s.tokens.GetByHash(ctx, hash)
	if err != nil {
		if apperror.IsCode(err, apperror.CodeNotFound) {
			return nil
		}
		return err
	}
	return s.tokens.Revoke(ctx, stored.ID)
}

// Me returns the current user.
func (s *Service) Me(ctx context.Context, userID uuid.UUID) (*User, error) {
	return s.users.GetUserByID(ctx, userID)
}

// ---- internals ----

func (s *Service) issueTokens(ctx context.Context, user *User, meta RequestMeta) (*AuthResult, error) {
	access, err := s.tm.IssueAccessToken(user)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	proxyToken, err := s.tm.IssueProxyToken(user)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	gen, err := s.tm.GenerateRefreshToken()
	if err != nil {
		return nil, apperror.Internal(err)
	}
	rt := &RefreshToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: gen.Hash,
		ExpiresAt: gen.ExpiresAt,
		UserAgent: meta.UserAgent,
		IP:        meta.IP,
		CreatedAt: s.now().UTC(),
	}
	if err := s.tokens.Create(ctx, rt); err != nil {
		return nil, err
	}
	return &AuthResult{
		AccessToken:  access,
		RefreshToken: gen.Plaintext,
		ProxyToken:   proxyToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.tm.AccessTTL().Seconds()),
		User:         user,
	}, nil
}

// ProxyTTL exposes the proxy-cookie lifetime for setting Max-Age.
func (s *Service) ProxyTTL() time.Duration { return s.tm.RefreshTTL() }

// ValidateProxyToken validates a gateway proxy token and returns the org id.
func (s *Service) ValidateProxyToken(raw string) (orgID string, err error) {
	claims, err := s.tm.ParseProxyToken(raw)
	if err != nil {
		return "", apperror.Unauthorized("invalid proxy session")
	}
	return claims.OrgID, nil
}

func (s *Service) uniqueOrgSlug(ctx context.Context, name string) (string, error) {
	base := slug.Make(name, "org")
	candidate := base
	for attempt := 0; attempt < 5; attempt++ {
		exists, err := s.users.SlugExists(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		suffix := uuid.NewString()[:6]
		candidate = truncateSlug(base, 63-len(suffix)-1) + "-" + suffix
	}
	return "", apperror.Conflict("could not allocate a unique organization slug; please choose a different name")
}

func (s *Service) audit(ctx context.Context, user *User, action audit.Action, resType, resID string, meta RequestMeta, md map[string]any) {
	orgID := user.OrgID
	uid := user.ID
	_ = s.auditlog.Record(ctx, audit.Entry{
		OrgID:        &orgID,
		UserID:       &uid,
		Action:       action,
		ResourceType: resType,
		ResourceID:   resID,
		Metadata:     md,
		IP:           meta.IP,
		UserAgent:    meta.UserAgent,
	})
}

func normalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

// localPart returns the portion of an email before the "@", used as a fallback display
// name when Google returns no name.
func localPart(email string) string {
	if i := strings.IndexByte(email, '@'); i > 0 {
		return email[:i]
	}
	return email
}

// deriveOrgName builds a sensible default organization name from a person's name for a
// first-time Google signup — the given name plus a possessive, e.g. "Jane's team".
func deriveOrgName(name string) string {
	first := strings.TrimSpace(name)
	if i := strings.IndexByte(first, ' '); i > 0 {
		first = first[:i]
	}
	if first == "" {
		return "My team"
	}
	return first + "'s team"
}

func truncateSlug(s string, max int) string {
	if max < 1 {
		return s
	}
	if len(s) > max {
		return strings.Trim(s[:max], "-")
	}
	return s
}
