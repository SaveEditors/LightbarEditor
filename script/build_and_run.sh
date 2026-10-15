#!/bin/bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
GO_BIN="${GO_BIN:-$ROOT/../go-toolchain/go/bin/go}"
if [[ ! -x "$GO_BIN" ]]; then GO_BIN="$(command -v go)"; fi
DIST="${DIST_DIR:-$ROOT/dist}"
APP="$DIST/Lightbar Editor.app"
pkill -x LightbarEditor 2>/dev/null || true
pkill -x LightbarBackend 2>/dev/null || true
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources" "$DIST/build"
(cd source && GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 "$GO_BIN" build -trimpath -ldflags '-s -w' -o "$APP/Contents/MacOS/LightbarBackend" .)
xcrun swiftc macOS/main.swift -O -target arm64-apple-macos13.0 -sdk "$(xcrun --show-sdk-path)" -framework AppKit -framework WebKit -o "$APP/Contents/MacOS/LightbarEditor"
[[ "$(lipo -archs "$APP/Contents/MacOS/LightbarBackend")" == "arm64" ]]
[[ "$(lipo -archs "$APP/Contents/MacOS/LightbarEditor")" == "arm64" ]]
cp macOS/Info.plist "$APP/Contents/Info.plist"
cp LICENSE THIRD_PARTY_LICENSES.txt "$APP/Contents/Resources/"
cp source/ui/fonts/OFL.txt "$APP/Contents/Resources/FONT_LICENSE.txt"
if [[ -f macOS/LightbarEditor.icns ]]; then cp macOS/LightbarEditor.icns "$APP/Contents/Resources/"; fi
xattr -cr "$APP"
IDENTITY="${SIGN_IDENTITY:--}"
SIGN_ARGS=(--force --sign "$IDENTITY")
if [[ "$IDENTITY" != "-" ]]; then SIGN_ARGS+=(--options runtime --timestamp); fi
codesign "${SIGN_ARGS[@]}" "$APP/Contents/MacOS/LightbarBackend"
codesign "${SIGN_ARGS[@]}" "$APP"
if [[ "${1:-}" == "--build-only" ]]; then exit 0; fi
open -n "$APP"
if [[ "${1:-}" == "--verify" ]]; then sleep 2; pgrep -x LightbarEditor >/dev/null; fi
