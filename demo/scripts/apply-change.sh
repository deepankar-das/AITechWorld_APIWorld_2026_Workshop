#!/usr/bin/env bash
# Enforcer demo — apply a prepared "AI-generated PR" overlay onto the working tree.
#
#   demo/scripts/apply-change.sh <change-name>
#
# Copies demo/changes/<name>/overlay/** over the repo root and records the
# resulting diff hash. Reverse it with demo/scripts/revert.sh.
set -euo pipefail

NAME="${1:-}"
if [[ -z "$NAME" ]]; then
  echo "usage: demo/scripts/apply-change.sh <change-name>" >&2
  exit 2
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CHANGE_DIR="$ROOT/demo/changes/$NAME"
OVERLAY="$CHANGE_DIR/overlay"
STATE="$ROOT/demo/run-folder/.applied"

if [[ ! -d "$OVERLAY" ]]; then
  echo "no overlay at demo/changes/$NAME/overlay/" >&2
  exit 1
fi

if [[ -f "$STATE" ]]; then
  echo "a change is already applied ($(cat "$STATE")). Run demo/scripts/revert.sh first." >&2
  exit 1
fi

if ! git -C "$ROOT" diff --quiet || ! git -C "$ROOT" diff --cached --quiet; then
  echo "working tree is dirty — commit or stash before applying a demo change." >&2
  exit 1
fi

# Copy overlay files, tracking what we touched.
TOUCHED=()
while IFS= read -r -d '' f; do
  rel="${f#"$OVERLAY"/}"
  mkdir -p "$ROOT/$(dirname "$rel")"
  cp "$f" "$ROOT/$rel"
  TOUCHED+=("$rel")
done < <(find "$OVERLAY" -type f -print0)

mkdir -p "$ROOT/demo/run-folder"
printf '%s\n' "$NAME" > "$STATE"
git -C "$ROOT" diff -- "${TOUCHED[@]}" | sha256sum | awk '{print $1}' > "$ROOT/demo/run-folder/.applied-hash"

echo "applied '$NAME' — ${#TOUCHED[@]} file(s):"
printf '  %s\n' "${TOUCHED[@]}"
echo "diff hash: $(cat "$ROOT/demo/run-folder/.applied-hash")"
