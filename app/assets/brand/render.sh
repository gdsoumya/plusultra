#!/usr/bin/env bash
# Rasterizes logo.svg and splash.svg into the PNGs app.json references. Needs only npx (no project dependency).
set -euo pipefail
cd "$(dirname "$0")"
out=..
resvg() { npx --yes @resvg/resvg-js-cli --no-system-font "$@"; }
white() { sed 's/#B4F23C/#FFFFFF/g'; }
# Adaptive layers shrink the mark a little so the plates clear the 66% safe circle of the mask;
# standalone icons fill more of the square.
mark() { grep -v 'id="bg"' logo.svg | sed 's/<g id="dumbbell"/<g id="dumbbell" transform="translate(512 512) scale(0.92) translate(-512 -512)"/'; }
large() { sed 's/<g id="dumbbell"/<g id="dumbbell" transform="translate(512 512) scale(1.25) translate(-512 -512)"/' logo.svg; }
bg='<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024"><rect width="1024" height="1024" fill="#0B0C0F"/></svg>'

large | resvg - "$out/icon.png"
large | resvg --fit-width 48 - "$out/favicon.png"
mark | resvg - "$out/android-icon-foreground.png"
mark | white | resvg - "$out/android-icon-monochrome.png"
mark | white | resvg --fit-width 96 - "$out/notification-icon.png"
resvg splash.svg "$out/splash-icon.png"
echo "$bg" | resvg - "$out/android-icon-background.png"
