#!/usr/bin/env bash
# Enforcer demo — apply a prepared "AI-generated PR" overlay onto the working tree.
#
#   demo/scripts/apply-change.sh <change-name>
#
# Copies demo/changes/<name>/overlay/** over the repo root. Changes stack: apply
# the seeded defect, then apply the fix on top of it (matching the Convergence
# Loop — the fix lands on the same diff and the corpus is re-run). Unwind the
# whole stack with demo/scripts/revert.sh.
set -euo pipefail

NAME="${1:-}"
if [[ -z "$NAME" ]]; then
  echo "usage: demo/scripts/apply-change.sh <change-name>" >&2
  exit 2
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OVERLAY="$ROOT/demo/changes/$NAME/overlay"
STACK="$ROOT/demo/run-folder/.applied"

[[ -d "$OVERLAY" ]] || { echo "no overlay at demo/changes/$NAME/overlay/" >&2; exit 1; }

if [[ -f "$STACK" ]] && grep -qxF "$NAME" "$STACK"; then
  echo "'$NAME' is already applied." >&2
  exit 1
fi

# Files already legitimately dirty because a stacked change touched them.
EXPECTED_DIRTY=""
if [[ -f "$STACK" ]]; then
  while IFS= read -r prev; do
    [[ -n "$prev" ]] || continue
    while IFS= read -r -d '' pf; do
      EXPECTED_DIRTY+=$'\n'"${pf#"$ROOT/demo/changes/$prev/overlay/"}"
    done < <(find "$ROOT/demo/changes/$prev/overlay" -type f -print0)
  done < "$STACK"
fi

REL_LIST=()
while IFS= read -r -d '' f; do
  REL_LIST+=("${f#"$OVERLAY"/}")
done < <(find "$OVERLAY" -type f -print0)

# Refuse to overlay a file the developer has hand-modified (that no stacked
# change is responsible for) — that would silently discard their edit.
for rel in "${REL_LIST[@]}"; do
  if ! git -C "$ROOT" diff --quiet -- "$rel" 2>/dev/null; then
    if ! grep -qxF "$rel" <<<"$EXPECTED_DIRTY"; then
      echo "refusing to overlay '$rel' — it has uncommitted changes. Commit or stash it first." >&2
      exit 1
    fi
  fi
done

TOUCHED=()
for rel in "${REL_LIST[@]}"; do
  mkdir -p "$ROOT/$(dirname "$rel")"
  cp "$OVERLAY/$rel" "$ROOT/$rel"
  TOUCHED+=("$rel")
done

mkdir -p "$ROOT/demo/run-folder"
printf '%s\n' "$NAME" >> "$STACK"
git -C "$ROOT" diff -- $(cut -d' ' -f1 <<<"${TOUCHED[*]}") | sha256sum | awk '{print $1}' > "$ROOT/demo/run-folder/.applied-hash"

echo "applied '$NAME' — ${#TOUCHED[@]} file(s):"
printf '  %s\n' "${TOUCHED[@]}"
echo "stack: $(paste -sd' → ' "$STACK")"
echo "diff hash: $(cat "$ROOT/demo/run-folder/.applied-hash")"
