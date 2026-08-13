# OIDC Single Sign-On ("Continue with OpsAPI")

Beacon can log users in through an external OpenID Connect / OAuth 2.0 provider
using the standard **Authorization-Code + PKCE** flow. It ships configured for
**OpsAPI**, but it is deliberately provider-agnostic: point the endpoint URLs at
any conforming provider (Keycloak, Auth0, Okta, Google) and it works — exactly
the way you would wire a generic "Sign in with X" button.

A new email signs up (an org + owner are provisioned), a returning email logs in
— the same provisioning path as "Continue with Google".

## How it works

```
Browser ──"Continue with OpsAPI"──▶ GET  /api/v1/auth/oidc/start
   beacon mints state + PKCE (verifier stashed in Redis), 302 ──▶ provider /oauth/authorize
      user authenticates + consents at the provider
   provider 302 ──▶ GET /api/v1/auth/oidc/callback?code=…&state=…
      beacon: validate+burn state → POST /oauth/token (code+PKCE) → GET /oauth/userinfo
      → find-or-create org+owner by verified email → issue beacon session
   302 ──▶ <frontend>/login/callback#access_token=…&refresh_token=…
      SPA stores tokens (URL fragment, never logged) → /dashboard
```

- **Confidential client**: beacon holds a client secret and reads identity from
  the provider's **UserInfo** endpoint over TLS — no ID-token signature / JWKS
  verification needed, so it is drop-in even for providers that sign ID tokens
  with a shared secret.
- **CSRF**: `state` is single-use (Redis `GetDel`), 10-minute TTL.
- **PKCE**: S256, always sent.
- **Email must be verified** by the provider (OpsAPI asserts this after its
  mandatory email-OTP 2FA).

## Backend configuration (env)

Enabled only when the client id/secret **and** the three endpoint URLs are all
set; otherwise the routes are absent and the frontend hides the button.

| Env var | Example |
|---|---|
| `BEACON_OIDC_PROVIDER` | `OpsAPI` (button label) |
| `BEACON_OIDC_CLIENT_ID` | `beacon` |
| `BEACON_OIDC_CLIENT_SECRET` | *(from client registration)* |
| `BEACON_OIDC_AUTHORIZE_URL` | `https://<opsapi>/oauth/authorize` |
| `BEACON_OIDC_TOKEN_URL` | `https://<opsapi>/oauth/token` |
| `BEACON_OIDC_USERINFO_URL` | `https://<opsapi>/oauth/userinfo` |
| `BEACON_OIDC_REDIRECT_URL` | `https://<beacon-api>/api/v1/auth/oidc/callback` |
| `BEACON_OIDC_POST_LOGIN_URL` | `https://<beacon-frontend>/login/callback` |
| `BEACON_OIDC_SCOPES` | `openid,profile,email,offline_access` |

## Frontend configuration (build-time)

| Env var | Example |
|---|---|
| `NEXT_PUBLIC_OIDC_ENABLED` | `true` |
| `NEXT_PUBLIC_OIDC_PROVIDER` | `OpsAPI` |

## Register beacon as a client in OpsAPI

OpsAPI clients are database rows; register one (idempotent) with `redirect_uri`
matching `BEACON_OIDC_REDIRECT_URL` exactly:

```bash
docker exec opsapi lapis exec "require('scripts.oauth-register-client').run({
  client_id     = 'beacon',
  name          = 'Beacon',
  redirect_uris = { 'https://<beacon-api-host>/api/v1/auth/oidc/callback' },
  scopes        = { 'openid', 'profile', 'email', 'offline_access' }
})"
```

Copy the printed `client_secret` into `BEACON_OIDC_CLIENT_SECRET` (shown once).

## Adding another provider

No backend code changes are needed — the client and service layers are generic.
Point the `BEACON_OIDC_*` URLs at the new provider, register beacon there, and
set the button label. Supporting *two* providers at once would only need a second
set of transport routes; the `oidcclient` adapter and `auth.LoginWithOIDC`
service method are already provider-neutral.

## Code map

| Concern | Location |
|---|---|
| Generic OAuth2 + PKCE client + UserInfo | `backend/internal/adapter/oidcclient/` |
| Config | `backend/internal/config/config.go` (`OIDC`) |
| Provisioning / session issue | `backend/internal/domain/auth/service.go` (`LoginWithOIDC`) |
| HTTP endpoints (`/auth/oidc/start`, `/callback`) | `backend/internal/transport/rest/sso_handler.go` |
| Button / callback page | `frontend/src/components/auth/opsapi-button.tsx`, `frontend/src/app/login/callback/` |
