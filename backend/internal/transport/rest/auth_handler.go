package rest

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"beacon/internal/domain/auth"
	"beacon/internal/platform/httpx"
	"beacon/internal/platform/validate"
)

// proxyCookieName is the httpOnly cookie holding the gateway proxy-session token.
const proxyCookieName = "beacon_proxy"

// AuthHandler exposes the authentication endpoints.
type AuthHandler struct {
	svc           *auth.Service
	validator     *validate.Validator
	secureCookies bool
}

// NewAuthHandler builds an AuthHandler. secureCookies marks the proxy cookie
// Secure (set true behind HTTPS / in production).
func NewAuthHandler(svc *auth.Service, v *validate.Validator, secureCookies bool) *AuthHandler {
	return &AuthHandler{svc: svc, validator: v, secureCookies: secureCookies}
}

// setProxyCookie stores the gateway proxy-session token as an httpOnly cookie so
// full-page navigations to the raw monitoring UIs carry the tenant identity.
func (h *AuthHandler) setProxyCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     proxyCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(h.svc.ProxyTTL().Seconds()),
	})
}

func (h *AuthHandler) clearProxyCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     proxyCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// Authorize is the endpoint nginx calls via auth_request for the raw monitoring
// UIs. It validates the proxy cookie and returns the tenant's org id in a header
// that the gateway forwards to prom-label-proxy for label enforcement.
func (h *AuthHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(proxyCookieName)
	if err != nil || cookie.Value == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	orgID, err := h.svc.ValidateProxyToken(cookie.Value)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	w.Header().Set("X-Org-Id", orgID)
	w.WriteHeader(http.StatusOK)
}

// Routes returns the public auth routes (no authentication required).
func (h *AuthHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/register", h.register)
	r.Post("/login", h.login)
	r.Post("/refresh", h.refresh)
	r.Post("/logout", h.logout)
	return r
}

// ---- DTOs ----

type registerRequest struct {
	OrgName  string `json:"org_name" validate:"required,min=1,max=200"`
	Name     string `json:"name" validate:"required,min=1,max=200"`
	Email    string `json:"email" validate:"required,email,max=254"`
	Password string `json:"password" validate:"required,min=8,max=128"`
}

type loginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type authResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	TokenType    string       `json:"token_type"`
	ExpiresIn    int          `json:"expires_in"`
	User         userResponse `json:"user"`
}

type userResponse struct {
	ID           string     `json:"id"`
	OrgID        string     `json:"org_id"`
	Email        string     `json:"email"`
	Name         string     `json:"name"`
	Role         string     `json:"role"`
	IsActive     bool       `json:"is_active"`
	TwoFAEnabled bool       `json:"twofa_enabled"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

func presentUser(u *auth.User) userResponse {
	return userResponse{
		ID:           u.ID.String(),
		OrgID:        u.OrgID.String(),
		Email:        u.Email,
		Name:         u.Name,
		Role:         string(u.Role),
		IsActive:     u.IsActive,
		TwoFAEnabled: u.TwoFAEnabled,
		LastLoginAt:  u.LastLoginAt,
		CreatedAt:    u.CreatedAt,
	}
}

func presentAuth(res *auth.AuthResult) authResponse {
	return authResponse{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		TokenType:    res.TokenType,
		ExpiresIn:    res.ExpiresIn,
		User:         presentUser(res.User),
	}
}

// ---- handlers ----

func (h *AuthHandler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := httpx.DecodeJSON(w, r, &req, maxBodyBytes); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	res, err := h.svc.Register(r.Context(), auth.RegisterInput{
		OrgName:  req.OrgName,
		Email:    req.Email,
		Password: req.Password,
		Name:     req.Name,
	}, requestMeta(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.setProxyCookie(w, res.ProxyToken)
	httpx.Created(w, presentAuth(res))
}

func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(w, r, &req, maxBodyBytes); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	res, err := h.svc.Login(r.Context(), req.Email, req.Password, requestMeta(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.setProxyCookie(w, res.ProxyToken)
	httpx.OK(w, presentAuth(res))
}

func (h *AuthHandler) refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := httpx.DecodeJSON(w, r, &req, maxBodyBytes); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	res, err := h.svc.Refresh(r.Context(), req.RefreshToken, requestMeta(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.setProxyCookie(w, res.ProxyToken)
	httpx.OK(w, presentAuth(res))
}

func (h *AuthHandler) logout(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := httpx.DecodeJSON(w, r, &req, maxBodyBytes); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.validator.Struct(req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.Logout(r.Context(), req.RefreshToken); err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.clearProxyCookie(w)
	httpx.NoContent(w)
}

// Me is mounted behind the auth middleware and returns the current user.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	p := mustPrincipal(r)
	user, err := h.svc.Me(r.Context(), p.UserID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.OK(w, presentUser(user))
}
