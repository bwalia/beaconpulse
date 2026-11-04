# Android app (Google Play)

The Android app is the website in a **Trusted Web Activity** (TWA): a small shell, built
on Google's `androidbrowserhelper`, that opens `https://<brand host>/dashboard` full
screen in Chrome. Everything comes from the site: screens, sign-in (email, Google), and
alerts (browser push, shown as the app's notifications). No Kotlin or Java code; every
web deploy updates the app.

## What ties the app to the site

- The app declares its site in `app/build.gradle.kts` (`site("sysops247.com")`).
- The site serves `/.well-known/assetlinks.json` (route in
  `frontend/src/app/.well-known/assetlinks.json`), built from `brand.android` in
  `frontend/src/brand/<brand>.ts`: the package name and the SHA-256 fingerprints of the
  signing keys. If the two don't match, Android shows a browser address bar over the app.
  After Play's first upload, add the **app signing key** fingerprint from Play Console →
  Test and release → App integrity to `brand.android.sha256`.
- The app opens `?app=android`. The site then hides billing for that tab (`isPlayApp()`
  in `frontend/src/lib/pwa.ts`): Play allows only Google Play Billing for in-app
  purchases, so plans are sold on the web only, as in the iOS app.

## Brands

One product flavor per brand in `app/build.gradle.kts` (application id, name, site), and
its icon and splash colour in `app/src/<brand>/res` (vectors, redrawn from the iOS icon in
`ios/Brands/<Brand>`). A new brand also needs `android` in its web brand file and a row in
`.github/workflows/android-release.yml`.

## Build

JDK 17+ and the Android SDK (`ANDROID_HOME`):

```
cd android
./gradlew assembleSysopsDebug     # APK for a phone or emulator
./gradlew bundleSysopsRelease     # AAB for Play (unsigned without the upload key)
```

A debug build is signed with your local debug key, which isn't in `assetlinks.json`, so it
shows the address bar. That's expected.

## Release

`.github/workflows/android-release.yml` builds a signed AAB on every push to `main` and,
once `PLAY_SERVICE_ACCOUNT_JSON` is set, uploads it to the internal testing track.

- **Upload key:** `~/.secrets/beacon-android/upload.jks` (alias `upload`, password next
  to it) on the machine that created it, plus GitHub secrets
  `ANDROID_UPLOAD_KEYSTORE_B64` / `ANDROID_UPLOAD_KEYSTORE_PASSWORD`. Google holds the real
  app signing key (Play App Signing). A lost upload key can be reset in Play Console.
- **First release:** Google requires the first AAB to be uploaded by hand. Download it
  from the workflow run's artifacts and upload it in Play Console → Test and release →
  Internal testing.
