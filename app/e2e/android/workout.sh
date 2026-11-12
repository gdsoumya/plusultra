#!/usr/bin/env bash
# Logs today's demo workout, relaunches the app and checks the sets stayed done, then takes a dark
# mode screenshot. Usage: workout.sh [--rebuild]. The app must be signed in to the demo server
# (mock://demo) and synced once. See docs/dev/android-emulator.md.
set -euo pipefail
cd "$(dirname "$0")/../.."

PKG=dev.plusultra.app
export ANDROID_HOME=${ANDROID_HOME:-$HOME/Android/Sdk}
export JAVA_HOME=${JAVA_HOME:-/usr/lib/jvm/java-17-openjdk-amd64}
SHOT=${SHOT:-/tmp/plusultra-android-workout-dark.png}
# Today's record in the demo fixtures.
RECORD="workout-$(date +%F)"

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

# The deep link opens the same route Today's "Start workout" does.
open_workout() {
  adb -e shell am start -W -a android.intent.action.VIEW -d "plusultra://workout/$RECORD" "$PKG" > /dev/null
  agent-device wait 'id="workout-screen"'
}

# One exercise shows at a time; the carousel tiles (tile-0, tile-1, ...) bring each one up.
# Warmups are rows like any other set.
tiles() {
  agent-device snapshot -i | grep -o 'tile-[0-9]*' | sort -u -t- -k2n
}
# Each set's play button is a checkbox labelled "Complete set n" ("Start set n" when the set has
# a work timer) until done; snapshot -i lists them with their testID. Prints the testIDs to press.
pending_sets() {
  agent-device snapshot -i | grep 'Complete ' | grep -o 'set-[0-9]*-[0-9]*-done' | sort -u
}
timed_sets() {
  agent-device snapshot -i | grep 'Start ' | grep -o 'set-[0-9]*-[0-9]*-done' | sort -u
}
# Completes every set of the shown exercise. A timed set starts its work timer on the first tap;
# Stop completes it and starts its rest.
complete_shown() {
  for set in $(pending_sets); do
    agent-device press "id=\"$set\"" --settle
  done
  for set in $(timed_sets); do
    agent-device press "id=\"$set\"" --settle
    agent-device wait 'id="set-timer"'
    agent-device press 'id="set-timer-action"' --settle
  done
}

restore_day() { adb -e shell cmd uimode night no > /dev/null; }
trap restore_day EXIT

agent-device open "$PKG" --platform android --relaunch
agent-device wait 'id="today-screen"'
open_workout

# The planned rest shows on the first row before anything is pressed.
if ! agent-device snapshot -i | grep -q 'rest [0-9]*:[0-9][0-9]'; then
  echo "FAIL: planned rest not shown on the set rows" >&2
  exit 1
fi
for tile in $(tiles); do
  agent-device press "id=\"$tile\"" --settle
  complete_shown
done
# The last tap started the rest timer; nudge it before skipping.
agent-device wait 'id="rest-timer"'
agent-device press 'id="rest-timer-plus"' --settle
agent-device press 'id="rest-timer-minus"' --settle
agent-device press 'id="rest-timer-action"' --settle
# Finish sits in the action row under the carousel.
agent-device scroll up --until 'id="finish-workout"' --settle
agent-device press 'id="finish-workout"' --settle
agent-device wait 'id="today-screen"'

# Kill and relaunch: the record comes back from SQLite, finished and fully done.
agent-device open "$PKG" --platform android --relaunch
agent-device wait 'id="today-screen"'
open_workout
for tile in $(tiles); do
  agent-device press "id=\"$tile\"" --settle
  if [ -n "$(pending_sets)$(timed_sets)" ]; then
    echo "FAIL: sets came back pending after relaunch" >&2
    exit 1
  fi
done
if agent-device snapshot -i | grep -q 'finish-workout'; then
  echo "FAIL: a finished workout still offers Finish" >&2
  exit 1
fi

adb -e shell cmd uimode night yes > /dev/null
agent-device wait 'id="workout-screen"'
agent-device screenshot --out "$SHOT"
agent-device close
echo "PASS: screenshot at $SHOT"
