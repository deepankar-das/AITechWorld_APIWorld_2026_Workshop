#!/usr/bin/env bash
# Enforcer demo — unwind every change demo/scripts/apply-change.sh has stacked.
#
#   demo/scripts/revert.sh
#
# Restores every file any applied overlay touched to its committed state.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STACK="$ROOT/demo/run-folder/.applied"

if [[ ! -f "$STACK" ]]; then
  echo "nothing applied."
  exit 0
fi

mapfile -t NAMES < "$STACK"

# Union of every path touched across the whole stack.
declare -A TOUCHED=()
for name in "${NAMES[@]}"; do
  OVERLAY="$ROOT/demo/changes/$name/overlay"
  while IFS= read -r -d '' f; do
    TOUCHED["${f#"$OVERLAY"/}"]=1
  done < <(find "$OVERLAY" -type f -print0)
done

for rel in "${!TOUCHED[@]}"; do
  if git -C "$ROOT" cat-file -e "HEAD:$rel" 2>/dev/null; then
    git -C "$ROOT" checkout -- "$rel"
  else
    rm -f "$ROOT/$rel"
  fi
done

rm -f "$STACK" "$ROOT/demo/run-folder/.applied-hash"
echo "reverted ${#NAMES[@]} change(s): ${NAMES[*]} — ${#TOUCHED[@]} file(s) restored."
