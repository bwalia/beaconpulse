# Beacon iOS

A native SwiftUI iPhone/iPad client for the Beacon monitoring platform. One
codebase ships any brand (SysOps 24/7, Red Fox Signals, Beacon, …) — branding is **pure
configuration**, not code.

This is **Phase 1**: app foundation + the alert loop (auth, monitors list/detail,
push registration and deep-link). Later phases add the overview dashboard,
full CRUD, and App Store polish.

## Requirements

- Xcode 15+ (iOS 17 SDK), a physical device for push testing
- [XcodeGen](https://github.com/yonaskolb/XcodeGen): `brew install xcodegen`
- A paid Apple Developer account (for push, Sign in with Apple, and distribution)

## Setup

```sh
cd ios
xcodegen generate        # builds Beacon.xcodeproj from project.yml
open Beacon.xcodeproj
```

Then in Xcode:
1. Pick the **SysOps**, **RedFox** or **Beacon** scheme.
2. Signing & Capabilities → select your Team. The **Push Notifications** and
   **Sign in with Apple** capabilities come from the generated entitlements.
3. Run on a device (push does not work in the simulator).

Before Google sign-in or a real API works, fill in the brand config (below).

## Brands = configuration

Every brand is a folder under `Brands/<Brand>/` plus a two-line target entry in
`project.yml`. **No Swift changes.** All brand/environment values live in the
brand's `brand.xcconfig`, are injected into `Info.plist`, and are read once by
`AppConfig` — nothing is hardcoded in feature code.

| Setting | Where | Notes |
|---|---|---|
| `PRODUCT_BUNDLE_IDENTIFIER` | `brand.xcconfig` | Must match the App ID and the backend `BEACON_APNS_TOPIC` |
| `API_BASE_URL` | `brand.xcconfig` | e.g. `https://api.sysops247.com` (the `/$()/` keeps xcconfig from eating the `//`) |
| `GOOGLE_CLIENT_ID` / `GOOGLE_REVERSED_CLIENT_ID` | `brand.xcconfig` | iOS OAuth client from Google Cloud; leave the `REPLACE_WITH…` placeholder to disable Google. The release workflow then strips `CFBundleURLTypes` before archiving — altool rejects an upload whose URL scheme contains underscores (error 90158), and the app hides the Google button for a placeholder anyway |
| `BRAND_DISPLAY_NAME` / `BRAND_ACCENT_HEX` | `brand.xcconfig` | App name + tint |

**Add a brand:** copy `Brands/SysOps/` to `Brands/<New>/`, edit its values, render a
1024pt icon with `swift Tools/make-brand-icon.swift --hex <RRGGBB> --out
Brands/<New>/Assets.xcassets/AppIcon.appiconset/icon-1024.png`, add a
target in `project.yml` with `templateAttributes: { brand: <New> }`, add a row to the
`BRANDS` tables in `fastlane/Fastfile` **and** `fastlane/Appfile`, add it to the
`prepare` matrix in `.github/workflows/ios-release.yml`, then re-run
`xcodegen generate`, drop in an `Assets.xcassets` with the app icon.

## Architecture

- **SwiftUI + `@Observable`** stores (MVVM). Views are thin; state lives in stores.
- **`APIClient`** — the single HTTP entry point. async/await, bearer auth via an
  injected `AuthProviding`, one transparent refresh-and-retry on `401`, typed
  `APIError`, snake_case/RFC3339 decoding. No view builds URLs.
- **`SessionStore`** — source of truth for auth; tokens in the **Keychain**;
  coalesced token refresh; conforms to `AuthProviding`.
- **`PushManager` + `AppDelegate`** — request permission, register with APNs, send
  the token to `POST /api/v1/devices`, drop it on sign-out, deep-link a tapped
  alert to its monitor via the `monitor_id` in the payload.
- **`AppContainer`** — composition root; builds the graph once.

## Push notifications

The device side registers; the **server** pushes (Beacon's Go `apns` notifier,
Phase 0). For pushes to arrive:
- Backend has `BEACON_APNS_*` set and `BEACON_APNS_TOPIC` == this brand's bundle id.
- The build's `aps-environment` entitlement is `development` here. **TestFlight and
  App Store builds use the _production_ APNs environment** — set `aps-environment`
  to `production` for distribution (a Release-config entitlements override), and
  `BEACON_APNS_PRODUCTION=true` on the backend.

## Sign in with Apple

Apple's App Store rule 4.8 means offering Google requires offering Sign in with
Apple. The app calls **`POST /api/v1/auth/apple`** with the identity token; the
backend verifies it against Apple's JWKS (set `BEACON_APPLE_CLIENT_ID` to the
bundle id). The Sign in with Apple capability is enabled on the
`com.sysops247.app` App ID (as a primary app).

## Release (TestFlight / App Store)

`.github/workflows/ios-release.yml` builds, signs, and uploads **every brand**
(SysOps 24/7 → `com.sysops247.app`, Red Fox Signals → `com.redfoxsignals.app`) on every push to
`main` (TestFlight, internal testers). Brands build as a **serialised** matrix, never
in parallel: `prepare_signing` rewrites the shared `Beacon.xcodeproj` and the shared
signing keychain, so two concurrent brands would sign each other's target. A manual
run can pick a single brand with the `brand` input. There is no
`ios/**` path filter on purpose — the app is a client of this backend, so a
backend or config merge can change its behaviour as much as a Swift change, and
testers should always be running current `main`. `workflow_dispatch` offers a
marketing-version override and a `target=app_store` option that submits the last
build for review.

Builds land in the **Internal Testers** beta group, which has "all builds"
enabled, so each upload distributes to the team automatically — no per-release
TestFlight admin.

It runs on the self-hosted Mac Studio runner and mirrors the Ring Promoter
pipeline: signing material (App Store Connect API key, team id) comes from
HashiCorp Vault at `secret/beaconpulse/ios` via `ci/load-ios-vault-secrets.sh`;
fastlane `cert`/`sigh` reuse the team's Apple Distribution certificate from a
persistent shared keychain and force-regenerate the App Store profile. The
workflow regenerates the project with XcodeGen and flips `aps-environment` to
`production` (TestFlight and the App Store deliver over production APNs) before
archiving. Build numbers come from TestFlight (`latest + 1`); the marketing
version is `MARKETING_VERSION` in `project.yml`.

Local rehearsal (uses your Vault token; never commits secrets):

```sh
cd ios && bundle install
eval "$(VAULT_TOKEN_FILE=$HOME/.secrets/acc-vault/login-token.json ./ci/load-ios-vault-secrets.sh)"
bundle exec fastlane ios ci_build_number   # needs VERSION_NAME set
```

## Tests

`Cmd-U` under the **SysOps** scheme. `Tests/APIClientTests.swift` covers decoding,
the error envelope, rate-limit parsing, and the 401→refresh→retry path via a
mock `URLProtocol` (no network).

## Not yet (later phases)

Overview dashboard + charts, active alerts, projects/maintenance/status, full
monitor CRUD, iPad split-view polish, and App Store metadata.
