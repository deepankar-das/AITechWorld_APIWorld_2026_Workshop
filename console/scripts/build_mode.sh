#!/bin/bash
set -euo pipefail

MODE="${1:-}"
if [[ "$MODE" != "hub" && "$MODE" != "sentinel" ]]; then
  echo "Usage: $0 <hub|sentinel>"
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_DIR="$ROOT_DIR/src/app"
STASH_DIR="$(mktemp -d "$ROOT_DIR/.app-stash-${MODE}.XXXXXX")"
OUT_DIR="$ROOT_DIR/out-${MODE}"

restore_routes() {
  for route in "$STASH_DIR"/*; do
    [[ -e "$route" ]] || continue
    base="$(basename "$route")"
    mv "$route" "$APP_DIR/$base"
  done
  rmdir "$STASH_DIR" 2>/dev/null || true
}

trap restore_routes EXIT

if [[ "$MODE" == "hub" ]]; then
  # Hub excludes only developer-personal pages.
  # Sessions, Search, Export are needed by admins for investigation.
  DISALLOWED=("developer")
else
  # Sentinel excludes admin-only and org-analytics pages.
  DISALLOWED=("approvals" "policies" "analytics")
fi

for route in "${DISALLOWED[@]}"; do
  if [[ -e "$APP_DIR/$route" ]]; then
    mv "$APP_DIR/$route" "$STASH_DIR/$route"
  fi
done

pushd "$ROOT_DIR" >/dev/null
NEXT_PUBLIC_AA_CONSOLE_MODE="$MODE" npx next build
rm -rf "$OUT_DIR"
mv "$ROOT_DIR/out" "$OUT_DIR"
if [[ "$MODE" == "sentinel" ]]; then
  cp -R "$OUT_DIR" "$ROOT_DIR/out"
fi
popd >/dev/null
