#!/usr/bin/env bash
# The whole app against the compose stack on Android: register a new user with an invite minted in
# the server container, create and activate the strength16 and recomp programs, log today's
# workout, meals and weight, API key, kill and relaunch; then, as a second user, timed and
# unilateral sets on a program that has them every day.
# Usage: E2E_SERVER_URL=http://10.0.2.2:8080 server.sh [--rebuild].
# The server must be reachable from the emulator (10.0.2.2 is the host). See docs/dev/android-emulator.md.
set -euo pipefail
cd "$(dirname "$0")/../.."

PKG=dev.plusultra.app
export ANDROID_HOME=${ANDROID_HOME:-$HOME/Android/Sdk}
export JAVA_HOME=${JAVA_HOME:-/usr/lib/jvm/java-17-openjdk-amd64}
SERVER=${E2E_SERVER_URL:-http://10.0.2.2:8080}
OUT=${OUT:-e2e/artifacts}
CONTAINER=${E2E_SERVER_CONTAINER:-plusultra-server-1}
USERNAME=e2e_android_$(date +%s)
PASSWORD="correct horse battery"
STRENGTH=../internal/engine/testdata/examples/strength16.star
MEALS=../internal/engine/testdata/examples/meals/recomp.star
TIMERS=e2e/timers.star
mkdir -p "$OUT"

booted() { [ "$(adb -e shell getprop sys.boot_completed 2>/dev/null | tr -d '\r')" = "1" ]; }
metro_up() { ss -ltn | grep -q ':8081 '; }

# try_for <seconds> <command...>: polls until the command succeeds; false on timeout.
try_for() {
  local deadline=$((SECONDS + $1))
  shift
  until "$@" 2> /dev/null; do
    ((SECONDS < deadline)) || return 1
    sleep 2
  done
}
# wait_for <seconds> <what> <command...>: the same, failing the run on timeout.
wait_for() {
  local limit=$1 what=$2
  shift 2
  try_for "$limit" "$@" || { echo "FAIL: $what not ready after ${limit}s" >&2; exit 1; }
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

shot() { agent-device screenshot --out "$OUT/android-$1.png" > /dev/null; }
seen() { agent-device snapshot -i | grep -q -- "$1"; }
# wait_text <text>: a text wait with a long budget for server round trips.
wait_text() { wait_for 60 "text \"$1\"" agent-device wait text "$1"; }
press() { agent-device press "role=button label=\"$1\"" --settle > /dev/null; }
# A tab by name: a screen heading can share it, and the bottom bar's item is the lowest.
tab() {
  local ref
  ref=$(agent-device snapshot -i --json | node -e '
    let s = "";
    process.stdin.on("data", (d) => (s += d)).on("end", () => {
      const tabs = JSON.parse(s).data.nodes.filter((n) => n.label === process.argv[1]);
      console.log("@" + tabs.sort((a, b) => b.rect.y - a.rect.y)[0].ref);
    });
  ' "$1")
  agent-device press "$ref" --settle > /dev/null
}
# A field's ref by its accessibility label. agent-device labels a field with its text or
# placeholder when it has one and keeps the accessibility label as contentDescription then.
field() {
  agent-device snapshot -i --json | node -e '
    const want = process.argv[1];
    let s = "";
    process.stdin.on("data", (d) => (s += d)).on("end", () => {
      const node = JSON.parse(s).data.nodes.find(
        (n) => n.editable && (n.contentDescription ?? n.label) === want,
      );
      if (!node) process.exit(1);
      console.log("@" + node.ref);
    });
  ' "$1"
}
fill() { agent-device fill "$(field "$1")" "$2" --settle > /dev/null; }

# Monday of this week and today's weekday number, in the emulator's zone (set to the host's).
weekday=$(date +%u)
monday=$(date -d "-$((weekday - 1)) days" +%F)

# A fresh install: no server, no session, no rows.
adb -e shell pm clear "$PKG" > /dev/null
agent-device open "$PKG" --platform android --relaunch
agent-device wait 'id="server-screen"'
fill "Server address" "$SERVER"
shot 01-server
press "Connect"
agent-device wait 'id="login-screen"'

# register <display name> <username>: from the sign-in screen to Today, with a fresh invite.
register() {
  press "Create an account"
  agent-device wait 'id="register-screen"'
  fill "Display name" "$1"
  fill "Username" "$2"
  fill "Password" "$PASSWORD"
  fill "Invite code" "$(docker exec "$CONTAINER" /plusultra invite)"
  press "Create account"
  wait_for 30 "Today after registering" agent-device wait 'id="today-screen"'
}
register "E2E Android" "$USERNAME"
shot 02-today-empty

# create_program <script path> <structure text>: editor, validation, save, activate on Monday.
create_program() {
  tab "Programs"
  agent-device wait 'id="programs-screen"'
  press "New"
  agent-device wait 'id="program-editor"'
  agent-device fill "$(field Script)" --text-stdin < "$1" > /dev/null
  # The editor follows the cursor to the end of the script; the check result sits above it, and
  # only a raw drag scrolls the nested script view back up.
  for _ in 1 2 3 4 5 6 7 8; do
    agent-device snapshot | grep -q -- "$2" && break
    agent-device swipe 540 700 540 1900 > /dev/null || true
  done
  wait_text "$2"
  press "Save"
  agent-device wait 'id="program-screen"'
  press "Activate"
  fill "Start date" "$monday"
  wait_text "Starts on a Monday, at week 1 day 1"
  press "Start program"
  wait_text "Active"
  # Pressing the current tab keeps its stack, so Back returns to the list.
  press "Back"
}
create_program "$STRENGTH" "No errors. 8 weeks"
shot 03-program-active
create_program "$MEALS" "No errors. 8 weeks"

tab "Today"
wait_text "Week 1, day $weekday"
shot 04-today-planned

# Tick the first set (the first tick asks for the notification permission and starts a timer),
# then Today's "Done as planned" completes and finishes the rest.
press "Start workout"
agent-device wait 'id="workout-screen"'
# The next set is "Complete …, next up", or "Start …, next up" on a timed day (any weekday runs).
next_up=$(agent-device snapshot -i --json | node -e '
  let s = "";
  process.stdin.on("data", (d) => (s += d)).on("end", () => {
    const node = JSON.parse(s).data.nodes.find((n) => / next up$/.test(n.label ?? ""));
    if (!node) process.exit(1);
    console.log("@" + node.ref);
  });
')
agent-device press "$next_up" --settle > /dev/null
if try_for 6 seen '"Allow"'; then press "Allow"; fi
if seen '"Stop"'; then press "Stop"; fi
if seen '"Skip"'; then press "Skip"; fi
wait_text " 1 done"
shot 05-workout-set-done
press "Back"
agent-device wait 'id="today-screen"'
press "Done as planned"
wait_text "View workout"

# The meals and check-in cards sit below the workout card.
agent-device scroll down --until 'label="Log as planned"' > /dev/null
press "Log as planned"
agent-device wait absent 'label="Log as planned"'
agent-device scroll down --until 'label="Save weight"' > /dev/null
# Whole numbers: the test keyboard's fill check trips over a decimal point.
fill "Weight (kg)" "81"
press "Save weight"
wait_text "Weight today"

# The server finalizes the record on push; the calendar then shows the day done.
tab "Calendar"
agent-device wait 'id="calendar-screen"'
wait_for 60 "today marked done on the calendar" seen "done, check-in, today"
shot 06-calendar

tab "Settings"
agent-device wait 'id="settings-screen"'
press "API keys"
agent-device wait 'id="api-keys-screen"'
press "New key"
fill "Name" "e2e"
press "Create key"
wait_text "lsk_"
shot 07-api-key
press "Done"
# The key's row opens the revoke sheet; a tap while the key sheet still closes lands on its backdrop.
open_key() {
  local ref
  ref=$(agent-device snapshot -i | grep 'button\] "e2e, lsk_' | grep -o '@e[0-9]*\(~s[0-9]*\)\?' | head -1)
  [ -n "$ref" ] && agent-device press "$ref" --settle > /dev/null && seen '"Revoke e2e"'
}
wait_for 30 "the revoke sheet" open_key
agent-device press 'label="Revoke e2e"' --settle > /dev/null
wait_text "revoked"

# Kill and relaunch: the store comes back from SQLite with the workout done and the weight saved.
agent-device open "$PKG" --platform android --relaunch
agent-device wait 'id="today-screen"'
wait_text "View workout"
agent-device scroll down --until 'label="Save weight"' > /dev/null
wait_text "Weight today"
if seen "Sync failed"; then
  echo "FAIL: sync failed after relaunch" >&2
  exit 1
fi
shot 08-today-relaunched

# Timed and unilateral sets as a second user, on a program that has them every day.
tab "Settings"
agent-device wait 'id="settings-screen"'
press "Account, E2E Android"
agent-device scroll down --until 'label="Log out"' > /dev/null
press "Log out"
agent-device wait 'id="server-screen"'
press "Connect"
agent-device wait 'id="login-screen"'
register "E2E Timers" "${USERNAME/e2e_android/e2e_timers}"
create_program "$TIMERS" "No errors. 2 weeks"
tab "Today"
wait_text "3x0:45 · rest 0:30"
press "Start workout"
agent-device wait 'id="workout-screen"'
# Play completes a plain set and starts its rest; Skip ends it.
agent-device press 'id="set-0-0-done"' --settle > /dev/null
if try_for 4 seen '"Allow"'; then press "Allow"; fi
agent-device wait 'id="rest-timer"'
agent-device press 'id="rest-timer-action"' --settle > /dev/null
# Play on a timed set starts the set bar; Stop completes the set and starts its rest.
agent-device press 'id="tile-1"' --settle > /dev/null
agent-device press 'id="set-1-0-done"' --settle > /dev/null
agent-device wait 'id="set-timer"'
wait_for 10 "the running set" seen '"Stop set 1"'
shot 09-set-timer
agent-device press 'id="set-timer-action"' --settle > /dev/null
agent-device wait 'id="rest-timer"'
wait_for 10 "the timed set done" seen '"Set 1 done, 1 reps'
agent-device press 'id="rest-timer-action"' --settle > /dev/null
# Unilateral from the exercise's gear: L and R bubbles, and play completes both sides.
agent-device press 'id="tile-3"' --settle > /dev/null
press "Bulgarian Split Squat options"
agent-device press 'id="unilateral-toggle"' --settle > /dev/null
wait_for 30 "the unilateral setting" seen '"Unilateral, Log left and right reps, On"'
adb -e shell input keyevent KEYCODE_BACK
agent-device wait absent 'id="unilateral-toggle"'
agent-device wait 'label="Left reps 8"'
agent-device press 'id="set-3-0-done"' --settle > /dev/null
wait_for 30 "both sides done" seen '"Set 1 done, L 8 R 8, tap to undo"'
shot 10-unilateral
agent-device press 'id="rest-timer-action"' --settle > /dev/null
# The last exercise's pane ends with Finish; a sideways scroll there would swipe exercises.
agent-device scroll down --until 'id="next-exercise"' > /dev/null
agent-device press 'id="next-exercise"' --settle > /dev/null
agent-device wait 'id="today-screen"'
wait_text "per side"
# The finished workout opens editable, without a Finish button.
press "Edit workout"
wait_text "Edits update the program's next sessions."
shot 11-finished
agent-device close
echo "PASS: screenshots in $OUT"
