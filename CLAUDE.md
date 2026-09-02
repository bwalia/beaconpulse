# Beacon / SysOps 24/7 — repo guide

Multi-tenant infrastructure-monitoring platform. Same backend ships under multiple
brands (SysOps 24/7, Red Fox Signals, Beacon) — **brand is always configuration, never
hardcoded.**

Brands live in three places, all configuration: `frontend/src/brand/<brand>.ts` (web,
selected at build time by `NEXT_PUBLIC_BRAND`), `deploy/helm/beacon/values-<brand>-<env>.yaml`
+ a wslproxy vhost (domain), and `ios/Brands/<Brand>/` (app). See
`deploy/helm/beacon/DEPLOY-A-BRAND.md`.

## Layout

- `backend/` — Go API + worker (chi, pgx, Prometheus/Blackbox control plane).
  Layering: `transport/rest` (handlers) → `domain/<x>` (services + interfaces) →
  `adapter/<x>` (postgres, notifiers, verifiers). Migrations in `backend/migrations`
  auto-apply on startup.
- `frontend/` — Next.js web app.
- `ios/` — native SwiftUI iPhone/iPad app (added recently — see below).

## iOS app — current status

All merged to `main` (PRs #75–#77). White-label SwiftUI app + a server-push
(APNs) path so monitor alerts reach the phone instead of Telegram. Build plan
artifact:
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

Verification note: the app now **builds clean against the iOS SDK** (Release,
`xcodebuild` on this Mac) and exports a signed App Store IPA via the fastlane
lanes. It has still never been run on a physical device — that's the next test.

## Apple Developer account state (mostly DONE)

Apple team id `PAS2QUVJHC`. Done: App ID `com.sysops247.app` registered with
**Push Notifications + Sign in with Apple** (primary app) enabled; App Store
Connect app record **SysOps247** (app id `6806569966`) exists; APNs auth key
`7294JN7U4Y` created and stored in Vault at **`secret/beaconpulse/apns`**
(ready-made `BEACON_APNS_*` values — the `.p8` is a one-time download, treat
Vault as the source of truth); Google iOS OAuth client id set in
`ios/Brands/SysOps/brand.xcconfig`.

TestFlight internal distribution is live: beta group **Internal Testers**
(`9b816721-8916-4130-b939-3ca294f35de1`, internal, "all builds" on so every CI
upload auto-distributes) holds all 7 App Store Connect team accounts, and the
beta app description + feedback email are set.

Still manual: App Store metadata (privacy labels, screenshots, review demo
account) and a real privacy-policy page — there is no `/privacy` route on the
site yet, and App Store review requires one.

## iOS release pipeline (TestFlight on push to main)

`.github/workflows/ios-release.yml` + `ios/fastlane/` — **every** push to `main`
builds, signs, and uploads **every brand** to TestFlight (internal): SysOps 24/7
(`com.sysops247.app`) and Red Fox Signals (`com.redfoxsignals.app`). Brands build as a serialised
matrix — never in parallel, because `prepare_signing` rewrites the shared
`Beacon.xcodeproj` and signing keychain. There is
deliberately no `ios/**` path filter: the app is a client of this backend, so a
backend or config merge can change its behaviour as much as a Swift change, and
testers should always be running current `main`. `workflow_dispatch` adds a
version override and `target=app_store` to submit for review. Runs on the
self-hosted Mac Studio runner (`hh193-beacon`); signing uses the App Store Connect API key from Vault
`secret/beaconpulse/ios` and the shared team keychain (mirrors ring-promoter's
pipeline — see `ios/README.md` → Release).

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
