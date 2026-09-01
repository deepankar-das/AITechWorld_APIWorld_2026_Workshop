#!/usr/bin/env bash
# Enforcer demo — undo whatever demo/scripts/apply-change.sh last applied.
#
#   demo/scripts/revert.sh
#
# Restores every file the overlay touched to its committed state.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STATE="$ROOT/demo/run-folder/.applied"

if [[ ! -f "$STATE" ]]; then
  echo "nothing applied."
  exit 0
fi

NAME="$(cat "$STATE")"
OVERLAY="$ROOT/demo/changes/$NAME/overlay"

TOUCHED=()
while IFS= read -r -d '' f; do
  TOUCHED+=("${f#"$OVERLAY"/}")
done < <(find "$OVERLAY" -type f -print0)

# Files that exist in the repo index → checkout; files the overlay added new → delete.
for rel in "${TOUCHED[@]}"; do
  if git -C "$ROOT" cat-file -e "HEAD:$rel" 2>/dev/null; then
    git -C "$ROOT" checkout -- "$rel"
  else
    rm -f "$ROOT/$rel"
  fi
done

rm -f "$STATE" "$ROOT/demo/run-folder/.applied-hash"
echo "reverted '$NAME' — ${#TOUCHED[@]} file(s) restored."
