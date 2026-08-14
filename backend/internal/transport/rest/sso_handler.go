package rest

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"beacon/internal/adapter/oidcclient"
	"beacon/internal/domain/auth"
	"beacon/internal/platform/httpx"
	"beacon/internal/transport/rest/middleware"
)

const (
	oidcStatePrefix = "oidc:state:"
	oidcStateTTL    = 10 * time.Minute
)

// SSOHandler implements "Sign in with <provider>" via the OAuth 2.0 Authorization
// Code + PKCE flow (see internal/adapter/oidcclient). It is constructed only when
// OIDC is configured; otherwise the routes are absent and the frontend hides the
// button — the same graceful-degradation pattern as Google sign-in.
type SSOHandler struct {
	svc           *auth.Service
	client        *oidcclient.Client
	rdb           *redis.Client
	provider      string
	postLoginURL  string
	secureCookies bool
}

// NewSSOHandler builds an SSOHandler.
func NewSSOHandler(svc *auth.Service, client *oidcclient.Client, rdb *redis.Client, provider, postLoginURL string, secureCookies bool) *SSOHandler {
	return &SSOHandler{
		svc:           svc,
		client:        client,
		rdb:           rdb,
		provider:      provider,
		postLoginURL:  postLoginURL,
		secureCookies: secureCookies,
	}
}

// Routes mounts the SSO endpoints.
func (h *SSOHandler) Routes() chi.Router {
	r := chi.NewRouter()
	// /start only INITIATES the flow (mint state + redirect) — it provisions
	// nothing, and clicking "sign in" a few times is normal, so it takes the
	// login-tier limit rather than the signup one.
	r.With(middleware.RateLimit(loginLimiter, middleware.ByIP, 30*time.Second)).
		Get("/start", h.Start)
	// /callback is where a first-time email PROVISIONS an org (permanent recurring
	// cost), so it carries the tighter signup limit — same reasoning as /auth/google.
	r.With(middleware.RateLimit(signupLimiter, middleware.ByIP, time.Minute)).
		Get("/callback", h.Callback)
	return r
}

// Start begins the flow: it mints a CSRF state + PKCE verifier, stashes the
// verifier in Redis keyed by the state (single-use, short TTL), and redirects the
// browser to the provider's authorize endpoint.
func (h *SSOHandler) Start(w http.ResponseWriter, r *http.Request) {
	state, err := randToken()
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	verifier := oidcclient.GenerateVerifier()
	if err := h.rdb.Set(r.Context(), oidcStatePrefix+state, verifier, oidcStateTTL).Err(); err != nil {
		httpx.Error(w, r, err)
		return
	}
	http.Redirect(w, r, h.client.AuthCodeURL(state, verifier), http.StatusFound)
}

// Callback completes the flow: it validates state (consuming the stored verifier),
// exchanges the code for tokens, reads the user's identity from UserInfo, signs
// them in (provisioning an org on first sight), sets the gateway proxy cookie, and
// hands the session to the SPA via the URL fragment (kept out of server logs).
func (h *SSOHandler) Callback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		h.redirectError(w, r, e)
		return
	}
	code, state := q.Get("code"), q.Get("state")
	if code == "" || state == "" {
		h.redirectError(w, r, "invalid_request")
		return
	}

	// Consume the state (single-use): GetDel both validates and burns it, so a
	// captured callback URL cannot be replayed.
	verifier, err := h.rdb.GetDel(r.Context(), oidcStatePrefix+state).Result()
	if err != nil || verifier == "" {
		h.redirectError(w, r, "invalid_state")
		return
	}

	tok, err := h.client.Exchange(r.Context(), code, verifier)
	if err != nil {
		h.redirectError(w, r, "exchange_failed")
		return
	}
	id, err := h.client.UserInfo(r.Context(), tok.AccessToken)
	if err != nil {
		h.redirectError(w, r, "userinfo_failed")
		return
	}

	res, err := h.svc.LoginWithOIDC(r.Context(), id.Email, id.Name, id.EmailVerified, h.provider, requestMeta(r))
	if err != nil {
		h.redirectError(w, r, "login_failed")
		return
	}

	h.setProxyCookie(w, res.ProxyToken)

	// Deliver the session to the SPA in the URL fragment: the fragment is never
	// sent to a server, so the tokens stay out of access logs and Referer headers.
	frag := url.Values{}
	frag.Set("access_token", res.AccessToken)
	frag.Set("refresh_token", res.RefreshToken)
	frag.Set("expires_in", strconv.Itoa(res.ExpiresIn))
	http.Redirect(w, r, h.postLoginURL+"#"+frag.Encode(), http.StatusFound)
}

func (h *SSOHandler) redirectError(w http.ResponseWriter, r *http.Request, code string) {
	frag := url.Values{}
	frag.Set("error", code)
	http.Redirect(w, r, h.postLoginURL+"#"+frag.Encode(), http.StatusFound)
}

func (h *SSOHandler) setProxyCookie(w http.ResponseWriter, token string) {
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

// randToken returns 32 bytes of CSPRNG entropy, base64url-encoded.
func randToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
