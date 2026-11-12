# Android release build

Everything comes from `app/app.json`, `app/assets` and `app/plugins`; `app/android` is generated
and gitignored, so never hand-edit it.

## Signing

Release builds are signed with `app/keystore.properties` when it exists (`app/plugins/withReleaseSigning.js`),
otherwise with the debug keystore so CI and e2e builds keep working unsigned.

```bash
cd app
keytool -genkeypair -v -keystore release.keystore -alias plusultra -keyalg RSA -keysize 2048 -validity 10000
cp keystore.properties.example keystore.properties   # fill in the passwords; storeFile is relative to app/
```

Both files are gitignored. Back the keystore and its passwords up somewhere safe: a lost keystore means
the app can never be updated in place, only reinstalled under a new signature.

## Build

```bash
cd app
npx expo prebuild --platform android --clean
cd android
./gradlew assembleRelease   # APK for sideloading: app/build/outputs/apk/release/app-release.apk
./gradlew bundleRelease     # AAB for Play:     app/build/outputs/bundle/release/app-release.aab
```

Release builds run Hermes, R8 minification and resource shrinking (`expo-build-properties` in
`app.json`). Keep rules go in `app/android/app/proguard-rules.pro` only through a config plugin.

## Versions

`expo.version` and `expo.android.versionCode` in `app/app.json` are what local builds carry. A release
comes from a tag: pushing `vX.Y.Z` runs `.github/workflows/release.yml`, which sets the version from
the tag (versionCode `X*10000 + Y*100 + Z`, so it always grows), builds the signed APK with the keystore
stored in the repository secrets, publishes the Docker image and attaches the APK to the GitHub release.
Bump `app.json` to the same version in the release commit so dev builds match.
