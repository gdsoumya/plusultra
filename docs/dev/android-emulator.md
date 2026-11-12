# Android emulator dev loop (verified 2026-11-11)

Verified on this machine with a throwaway Expo SDK 58 / React Native 0.88 app: Gradle build,
install, Metro with Fast Refresh, and agent-device driving the UI.

## Machine facts
- SDK at `~/Android/Sdk` (`ANDROID_HOME` is set). Platforms 35 and 36, build-tools 35/36, NDK 27,
  emulator 37.1, one system image: `system-images;android-35;google_apis;x86_64`.
- Java 17 at `/usr/lib/jvm/java-17-openjdk-amd64` (set `JAVA_HOME` for Gradle). Node 26 via nvm.
- KVM works. `watchman` is not installed; Metro uses its Node watcher, which is fine.
- `agent-device` 0.21.36 is installed globally.
- AVD `Pixel7_API35` (Pixel 7 profile, 6 GB RAM, 6 GB data, swiftshader GPU config). 4 GB RAM
  made Android swap and killed uiautomator, so keep 6 GB.

## Commands
```bash
# boot (visible window; add -no-window for headless)
setsid nohup emulator -avd Pixel7_API35 -no-audio -no-boot-anim -gpu auto -no-snapshot-save \
  > /tmp/emulator.log 2>&1 < /dev/null &
until [ "$(adb -e shell getprop sys.boot_completed 2>/dev/null | tr -d '\r')" = "1" ]; do sleep 3; done

# native build + install (first build ~4.5 min, later ones faster)
export ANDROID_HOME=$HOME/Android/Sdk JAVA_HOME=/usr/lib/jvm/java-17-openjdk-amd64
npx expo run:android --no-bundler --device emulator-5554
# after a native config change (app.json plugins, e.g. app/plugins/*), regenerate android/ first:
npx expo prebuild --platform android --clean

# bundler: NEVER set CI=1 (it disables file watching; the app then gets stale code forever)
setsid nohup npx expo start --port 8081 > /tmp/metro.log 2>&1 < /dev/null &
# the app reaches Metro at 10.0.2.2:8081 on its own; adb reverse is not required

# drive it
agent-device open <package.id> --platform android --foreground   # or --relaunch
agent-device snapshot -i          # lists only interactive nodes; static Text is not listed
agent-device fill 'id="name-input"' "Soumya" --settle
agent-device press '@e2~s592551' --settle   # use the EXACT ref printed by the latest snapshot/diff
agent-device wait text "count=1"            # no --timeout flag exists
agent-device get text 'id="greeting"'
agent-device metro reload                   # force a reload from Metro
agent-device close                          # add --shutdown to also stop the emulator
```

## Traps hit today
- `CI=1` on `expo start` puts Metro in CI mode: no watcher, stale transforms served to the app
  even though curl shows fresh output. Symptom: edits never appear, even after relaunch.
  Fix: run Metro without CI, or `--clear` once to drop the stale cache.
- `pkill -f "<pattern>"` from a shell whose own command line contains the pattern kills that shell
  (exit 144). Kill by PID from `ps` instead, and start long processes with `setsid nohup ... &`.
- `snapshot -i` returning 0 nodes while the screen shows only static text is expected. Give
  elements `testID` and use `wait text` / `get text` for assertions.
- `press "Increment"` is invalid; targets are `@ref`, `text="..."`, `id="..."` or coordinates.
- `avdmanager create avd` prints a harmless `devices.xml` error for this system image.
- The emulator window is only visible when started without `-no-window`.
