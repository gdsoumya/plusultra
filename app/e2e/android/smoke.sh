#!/usr/bin/env bash
# Idempotent Android smoke test. Usage: smoke.sh [--rebuild]. See docs/dev/android-emulator.md.
set -euo pipefail
cd "$(dirname "$0")/../.."

PKG=dev.plusultra.app
export ANDROID_HOME=${ANDROID_HOME:-$HOME/Android/Sdk}
export JAVA_HOME=${JAVA_HOME:-/usr/lib/jvm/java-17-openjdk-amd64}
SHOT=${SHOT:-/tmp/plusultra-android-smoke.png}

booted() { [ "$(adb -e shell getprop sys.boot_completed 2>/dev/null | tr -d '\r')" = "1" ]; }
metro_up() { ss -ltn | grep -q ':8081 '; }

# wait_for <seconds> <what> <command...>: polls until the command succeeds, fails the run on timeout.
wait_for() {
  local limit=$1 what=$2
  shift 2
  local deadline=$((SECONDS + limit))
  until "$@"; do
    if ((SECONDS >= deadline)); then
      echo "FAIL: $what not ready after ${limit}s" >&2
      exit 1
    fi
    sleep 2
  done
}

if ! booted; then
  setsid nohup emulator -avd Pixel7_API35 -no-audio -no-boot-anim -gpu auto -no-snapshot-save \
    > /tmp/emulator.log 2>&1 < /dev/null &
  wait_for 180 "emulator boot" booted
fi

if [ "${1:-}" = "--rebuild" ] || ! adb -e shell pm list packages | grep -q "package:$PKG\$"; then
  npx expo run:android --no-bundler --device emulator-5554
fi

# Never CI=1: it disables Metro's file watcher.
if ! metro_up; then
  setsid nohup npx expo start --port 8081 > /tmp/metro.log 2>&1 < /dev/null &
  wait_for 60 "Metro on :8081" metro_up
fi

today_visible() { agent-device wait text "Today" && agent-device wait 'id="today-screen"'; }

agent-device open "$PKG" --platform android --relaunch
if ! today_visible; then
  echo "RETRY: Today screen not visible, relaunching $PKG" >&2
  agent-device open "$PKG" --platform android --relaunch
  today_visible
fi
agent-device screenshot --out "$SHOT"
agent-device close
echo "PASS: screenshot at $SHOT"
