#!/bin/bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SET="$(mktemp -d)/LightbarEditor.iconset"
mkdir -p "$SET"
trap 'rm -rf "$(dirname "$SET")"' EXIT
for SIZE in 16 32 128 256 512; do
  sips -z "$SIZE" "$SIZE" "$ROOT/macOS/AppIcon.png" --out "$SET/icon_${SIZE}x${SIZE}.png" >/dev/null
  DOUBLE=$((SIZE * 2))
  sips -z "$DOUBLE" "$DOUBLE" "$ROOT/macOS/AppIcon.png" --out "$SET/icon_${SIZE}x${SIZE}@2x.png" >/dev/null
done
iconutil -c icns "$SET" -o "$ROOT/macOS/LightbarEditor.icns"
