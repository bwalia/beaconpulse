# Android app (Google Play)

A native Kotlin app: Jetpack Compose + Material 3, the Android twin of `ios/`. Same
screens and behaviour as the iPhone app: sign-in (email/password, Continue with Google,
sign-up), Overview with charts, Monitors (search, detail, create/edit/delete,
pause/resume), Alerts, Projects, and Settings (notification channels with Send test,
maintenance windows, push status, account deletion). Billing stays on the website, as on
iOS.

## Layout (`app/src/main/java/com/beacon/app`)

| File | What |
| --- | --- |
| `Api.kt` | `Api` (the only HTTP entry point: OkHttp + kotlinx.serialization, typed `ApiError`), the authenticated client (bearer + one coalesced refresh-and-retry on 401), `Session` (tokens + user, signs out only when the server refuses a refresh), `Vault` (session encrypted with an Android Keystore key) |
| `Models.kt` | Wire models. Monitor `settings` stays raw JSON so an edit sends back keys only the web sets |
| `App.kt` | Composition root (`Container`), push (`Push`, `PushService`: Firebase Cloud Messaging, the `alerts` channel, `/api/v1/devices`) |
| `Ui.kt` | Shared pieces: `Loader`/`LoadableContent` (loading, error + retry, pull to refresh), forms, dropdowns, chart, date-time picker |
| `MainActivity.kt`, `Shell.kt` | Theme, sign-in gate, five tabs with their own back stacks, alert-tap deep links |
| `Login.kt`, `Overview.kt`, `Monitors.kt`, `Projects.kt`, `Settings.kt` | The screens |

## Brands

One product flavor per brand in `app/build.gradle.kts` via `brand(...)`: name, API base
URL, accent colour, Google *web* OAuth client id, and Firebase ids; its launcher and
notification icons in `app/src/<brand>/res`. Add a row to
`.github/workflows/android-release.yml` for a new brand.

## Push

The backend's `fcm` notifier sends alerts through Firebase Cloud Messaging
(`BEACON_FCM_CREDENTIALS`). The app is configured in code from the flavor's
`firebaseAppId` / `firebaseApiKey` / `firebaseProjectId` / `firebaseSenderId` (Firebase
console → Project settings → the Android app), no `google-services.json`. Blank = push off;
everything else works. A tapped alert opens its monitor (`monitor_id` in the data).

## Google sign-in

Credential Manager returns an ID token issued to the brand's **web** OAuth client, which
`/api/v1/auth/google` already accepts. Google also needs an **Android** OAuth client in the
same Google Cloud project for package `com.sysops247.app` with the SHA-1 of each signing
key: the upload key, and Play's app signing key (Play Console → Test and release → App
integrity). Without it the button says Google sign-in isn't available.

## Build and test

JDK 17+ (Android Studio's bundled one is fine) and the Android SDK:

```
cd android
./gradlew testSysopsDebugUnitTest    # API client, token refresh, settings round trip
./gradlew assembleSysopsDebug        # APK for a phone or emulator
./gradlew bundleSysopsRelease        # AAB for Play (unsigned without the upload key)
```

## Release

`.github/workflows/android-release.yml` tests and builds a signed AAB on every push to
`main` and, once `PLAY_SERVICE_ACCOUNT_JSON` is set, uploads it to internal testing.

- **Upload key:** `~/.secrets/beacon-android/upload.jks` (alias `upload`, password next
  to it) on the machine that created it, plus GitHub secrets
  `ANDROID_UPLOAD_KEYSTORE_B64` / `ANDROID_UPLOAD_KEYSTORE_PASSWORD`. Google holds the real
  app signing key (Play App Signing). A lost upload key can be reset in Play Console.
- **First release:** Google requires the first AAB to be uploaded by hand. Download it
  from the workflow run's artifacts and upload it in Play Console → Test and release →
  Internal testing.
