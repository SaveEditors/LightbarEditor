#!/bin/bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST="${DIST_DIR:-$ROOT/dist}"
APP="$DIST/Lightbar Editor.app"
DMG="$DIST/LightbarEditor-1.7.0-macOS-arm64.dmg"
if [[ ! -d "$APP" ]]; then "$ROOT/script/build_and_run.sh" --build-only; fi
codesign --verify --deep --strict "$APP"
STAGE="$(mktemp -d "$DIST/dmg-stage.XXXXXX")"
trap 'rm -rf "$STAGE"' EXIT
ditto "$APP" "$STAGE/Lightbar Editor.app"
ln -s /Applications "$STAGE/Applications"
cp "$ROOT/LICENSE" "$STAGE/LICENSE.txt"
cp "$ROOT/THIRD_PARTY_LICENSES.txt" "$STAGE/THIRD_PARTY_LICENSES.txt"
cat > "$STAGE/Install.txt" <<'TXT'
Drag Lightbar Editor.app to Applications.
Requires macOS 13 or newer; Apple Silicon (arm64) only.
No PS5 assets are bundled. Open your own shutdown_lightbar.fbxd and shutdown_ramp.gnf files.

This preview is not notarized. A macOS security warning may appear.
Only proceed after checking the source and published SHA-256 checksum.
Upstream: https://github.com/SaveEditors/LightbarEditor
TXT
hdiutil create -ov -volname 'Lightbar Editor' -srcfolder "$STAGE" -format UDZO "$DMG"
if [[ -n "${SIGN_IDENTITY:-}" ]]; then codesign --force --sign "$SIGN_IDENTITY" --timestamp "$DMG"; fi
if [[ -n "${NOTARY_PROFILE:-}" ]]; then
  xcrun notarytool submit "$DMG" --keychain-profile "$NOTARY_PROFILE" --wait
  xcrun stapler staple "$DMG"
  xcrun stapler validate "$DMG"
fi
hdiutil verify "$DMG"
(cd "$DIST" && shasum -a 256 "$(basename "$DMG")" > SHA256SUMS)
echo "$DMG"
