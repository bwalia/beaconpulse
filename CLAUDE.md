# Beacon / SysOps 24/7 — repo guide

Multi-tenant infrastructure-monitoring platform. Same backend ships under multiple
brands (SysOps 24/7, Beacon) — **brand is always configuration, never hardcoded.**

## Layout

- `backend/` — Go API + worker (chi, pgx, Prometheus/Blackbox control plane).
  Layering: `transport/rest` (handlers) → `domain/<x>` (services + interfaces) →
  `adapter/<x>` (postgres, notifiers, verifiers). Migrations in `backend/migrations`
  auto-apply on startup.
- `frontend/` — Next.js web app.
- `ios/` — native SwiftUI iPhone/iPad app (added recently — see below).

## iOS app — current status

Work is on branch **`feat/ios-apns-push`** (committed, not yet PR'd/merged).
White-label SwiftUI app + a server-push (APNs) path so monitor alerts reach the
phone instead of Telegram. Build plan artifact:
https://claude.ai/code/artifact/46a5bfe8-19eb-4bc7-a2ef-0091bf3d3c0d

Done:
- **Phase 0 — backend push (Go):** `device_tokens` table + `POST/DELETE /api/v1/devices`
  (session-only) + an `apns` notification channel/notifier that fans an org's alert to
  its device tokens over APNs HTTP/2 and prunes dead tokens. Registering a device
  auto-enables the org's Apple Push channel (never re-enables a muted one).
- **Phase 1 — app scaffold:** `APIClient` (async/await, bearer, transparent 401
  refresh+retry, typed `APIError`), `SessionStore` (Keychain tokens, coalesced
  refresh), sign-in (email/password + Google + Apple), `PushManager` (APNs → /devices,
  tap deep-links to the monitor), monitors list/detail.
- **Phase 2 — read parity:** tabbed shell (Overview/Monitors/Alerts/Projects/Settings),
  Overview dashboard with Swift Charts, active alerts, projects (drill into filtered
  monitors), response-time chart on monitor detail.
- **Phase 3 — write parity:** monitor create/edit/delete + pause/resume (edit
  round-trips the full `settings` object so web-set advanced config isn't clobbered),
  notification-channel CRUD + "send test", Settings tab.
- **Sign in with Apple backend:** `POST /api/v1/auth/apple` (verifies Apple's identity
  token via JWKS; reuses OIDC provisioning; `BEACON_APPLE_CLIENT_ID` = bundle id).

Next: **Phase 4 — ship** (iPad split-view polish, empty/error/loading states,
accessibility, TestFlight → App Store). Deferred (lower value): maintenance-window
CRUD, platform settings screen. Billing stays web-only (Apple IAP rules).

Verification note: the app's SwiftUI/Charts/GoogleSignIn code has **only been
type-checked/parsed against the macOS SDK here — it has never been built in Xcode.**
The first device build is the real test.

## ⚠️ Tasks that need the Apple Developer account (do these on this Mac)

These were blocked on the machine without the account. Do them here, then set the
backend secrets below.

1. **App ID** — register bundle id `com.sysops247.app` (Identifiers → +). Enable
   **Push Notifications** and **Sign in with Apple** capabilities.
2. **APNs Auth Key (.p8)** — Keys → +, tick *Apple Push Notifications service*. Download
   the `.p8` (one time!) and note the **Key ID**. The **Team ID** is under Membership.
   → these become the backend `BEACON_APNS_*` secrets.
3. **Sign in with Apple** — needs *no separate key* for a native app: the capability on
   the App ID + the entitlement (already in `ios/project.yml`) is enough. The backend
   only needs the bundle id as the token audience (`BEACON_APPLE_CLIENT_ID`).
4. **Google iOS OAuth client id** (Google Cloud Console, *not* Apple) — create an iOS
   OAuth client, put the client id + its reversed form in
   `ios/Brands/SysOps/brand.xcconfig` (`GOOGLE_CLIENT_ID`, `GOOGLE_REVERSED_CLIENT_ID`).
   Leave the `REPLACE_WITH…` placeholder to keep Google sign-in disabled.
5. **Xcode signing** — open the project (below), select your Team on each app target.
6. **App Store Connect** — create the app record, then TestFlight (internal testers),
   later App Store (privacy labels, screenshots, a review demo account).

## Backend env / secrets to set (sealed secrets per deployment)

Push and third-party sign-in all degrade off when unset — safe to deploy before they're
configured.

- `BEACON_APNS_KEY_P8` — contents of the `.p8` file (not a path)
- `BEACON_APNS_KEY_ID`, `BEACON_APNS_TEAM_ID`
- `BEACON_APNS_TOPIC` — the bundle id, e.g. `com.sysops247.app`
- `BEACON_APNS_PRODUCTION` — `true` for TestFlight/App Store builds (**TestFlight uses
  the *production* APNs environment**; also set `aps-environment: production` in the
  release entitlement); `false` only for an Xcode debug build on a device
- `BEACON_APPLE_CLIENT_ID` — the bundle id(s), comma-separated per brand
- `BEACON_GOOGLE_CLIENT_ID` — existing web client id(s), comma-separated

## Build, run, test

Backend:
```
cd backend
go build ./...
go test ./internal/adapter/... ./internal/domain/... ./internal/config/...
```
Full `go test ./...` may hit a macOS dyld issue on some packages — run targeted package
tests, or use a Linux container for the whole suite.

iOS (needs Xcode 15+, an iOS 17 device for push):
```
brew install xcodegen
cd ios && xcodegen generate && open Beacon.xcodeproj
# pick the SysOps scheme → set your Team → run on a device
```
`Beacon.xcodeproj` is generated from `ios/project.yml` (git-ignored). `Cmd-U` runs the
unit tests under the SysOps scheme. See `ios/README.md` for the white-label brand model.

## Conventions

- **Brand = configuration.** Web/API: `BEACON_*` env per deployment. iOS: per-brand
  `ios/Brands/<Brand>/brand.xcconfig`. Never hardcode a brand, URL, id, or secret.
- **Backend:** handler → domain service → repository interface → postgres adapter (no
  handler-talks-to-DB shortcut). Verifiers/notifiers are adapters injected in
  `cmd/api/main.go`. New feature flags follow the graceful-degradation `Enabled()`
  pattern (see Google/Apple/Stripe/AI/Push in `internal/config`).
- **iOS:** SwiftUI + `@Observable` MVVM. `APIClient` is the only HTTP entry point
  (bearer + one transparent 401 refresh). Tokens live in the Keychain. All screens go
  through a shared `Loadable<T>` for loading/error/retry.
- Commit style: `type(scope): summary`, and co-author trailer on commits.

## References

- `ios/README.md` — iOS setup, brand model, architecture.
- Auth-provider pattern to copy: `internal/adapter/googleauth` + `appleauth`,
  wired in `internal/domain/auth/service.go` and `cmd/api/main.go`.
