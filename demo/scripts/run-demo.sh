#!/usr/bin/env bash
# Enforcer convergence demo — presenter driver.
#
#   demo/scripts/run-demo.sh          interactive: press Enter to advance each beat
#   demo/scripts/run-demo.sh --auto   run straight through, no pauses (pre-flight self-test)
#
# Ctrl-C at any point aborts and restores a clean tree.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

AUTO=0
[[ "${1:-}" == "--auto" ]] && AUTO=1

if [[ -t 1 ]]; then
  B=$'\e[1m'; DIM=$'\e[2m'; AMBER=$'\e[33m'; GREEN=$'\e[32m'; RED=$'\e[31m'; CYAN=$'\e[36m'; R=$'\e[0m'
else
  B=""; DIM=""; AMBER=""; GREEN=""; RED=""; CYAN=""; R=""
fi

STEP=0
DONE=0
cleanup(){
  [[ $DONE -eq 1 ]] && return
  echo; echo "${DIM}restoring a clean tree…${R}"
  "$ROOT/demo/scripts/revert.sh" >/dev/null 2>&1 || true
}
trap cleanup EXIT
trap 'echo; echo "${RED}aborted.${R}"; exit 130' INT

pause(){
  [[ $AUTO -eq 1 ]] && return
  printf '\n%s' "${AMBER}  press Enter ▸${R}  ${DIM}(Ctrl-C aborts)${R} "
  read -r _ || true
  echo
}

beat(){
  STEP=$((STEP+1))
  echo
  echo "${B}${CYAN}━━━━━━  BEAT $STEP  ·  $1  ━━━━━━${R}"
  [[ -n "${2:-}" ]] && echo "${DIM}$2${R}"
}

run(){ echo "${DIM}\$ $*${R}"; "$@"; }

DEMO_FILES="src/daemon/routes/enrich.ts tests/integration/audit-enrich-immutability.test.ts"

# ---- reset to a clean, known state (idempotent — safe on restart / repeat) ----
reset_demo_env(){
  # 1. unwind any overlay stack a previous run left applied
  "$ROOT/demo/scripts/revert.sh" >/dev/null 2>&1 || true

  # 2. force-restore a demo-target file only if it still exactly matches a known
  #    overlay (i.e. stale state from a killed run) — never touch a hand edit
  local f c ov
  for f in $DEMO_FILES; do
    git -C "$ROOT" diff --quiet -- "$f" && continue
    for c in 01-audit-mutation 02-fix; do
      ov="$ROOT/demo/changes/$c/overlay/$f"
      [[ -f "$ov" ]] && cmp -s "$ov" "$ROOT/$f" && { git -C "$ROOT" checkout -- "$f"; break; }
    done
  done

  # 3. wipe run-folder scratch (dated run dirs, temp files, state pointers)
  rm -rf "$ROOT/demo/run-folder" 2>/dev/null || true
  mkdir -p "$ROOT/demo/run-folder"

  # 4. anything still dirty in a demo-target file is a real edit — stop
  local dirty
  dirty="$(git -C "$ROOT" diff --name-only -- $DEMO_FILES; git -C "$ROOT" diff --cached --name-only -- $DEMO_FILES)"
  if [[ -n "$dirty" ]]; then
    echo "${RED}these demo-target files have uncommitted changes that aren't a demo overlay:${R}" >&2
    echo "$dirty" | sort -u | sed 's/^/  /' >&2
    echo "${DIM}commit or stash them, then re-run.${R}" >&2
    exit 1
  fi
}
reset_demo_env

command clear 2>/dev/null || printf '\n\n'
echo "${B}Enforcer convergence demo${R}"
echo "${DIM}Runs against Enforcer's real Vitest suite. No infrastructure. ~30s of command time.${R}"
echo "${DIM}Press Enter to advance each beat. Ctrl-C aborts and reverts.${R}"
pause

# ── BEAT 1 ─────────────────────────────────────────────────────────────────
beat "clean baseline" "Three fast gates on an untouched tree — all green."
pause
run node demo/radar/gate-fast.mjs || true

# ── BEAT 2 ─────────────────────────────────────────────────────────────────
beat "the AI-generated PR lands" "One file. Reads like a tidy refactor: 'the pending event already carries the action context, so just record the outcome on it instead of appending a second row.'"
pause
run demo/scripts/apply-change.sh 01-audit-mutation
echo
echo "${B}what changed${R}"
run git --no-pager diff --stat -- src/daemon/routes/enrich.ts
echo
echo "${B}the diff${R} ${DIM}— the append-only enrichment event is gone; the pending event is written in place${R}"
run git --no-pager diff -- src/daemon/routes/enrich.ts

# ── BEAT 3 ─────────────────────────────────────────────────────────────────
beat "the fast gate catches it" "It compiles. No secrets. G3 — the invariant floor 'audit-enrich-immutability' — fails."
pause
run node demo/radar/gate-fast.mjs --change 01-audit-mutation || true

# ── BEAT 4 ─────────────────────────────────────────────────────────────────
beat "RADAR selection" "Every test carries its reason and its blast-radius dimension. Six mandatory floors — the selector is forbidden from dropping them."
pause
run node demo/radar/select.mjs --change 01-audit-mutation

# ── BEAT 5 ─────────────────────────────────────────────────────────────────
beat "the run and the verdict" "68 of 69 green — one floor red is a hard stop."
pause
run node demo/radar/run.mjs || true
echo
echo "${B}the receipt${R} ${DIM}— written next to the code, hashed against this exact change${R}"
run cat "demo/run-folder/$(cat demo/run-folder/.latest)/receipt"

# ── BEAT 6 ─────────────────────────────────────────────────────────────────
beat "the fix, stacked on the diff, re-run" "The fix restores the append-only path AND ships a counter-example: it freezes the original event so any in-place write is a thrown error, and proves repeated enrichment only ever appends."
pause
run demo/scripts/apply-change.sh 02-fix
echo
echo "${B}what the fix adds${R}"
run git --no-pager diff --stat
pause
run node demo/radar/run.mjs || true
echo
echo "${B}the receipt now${R}"
run cat "demo/run-folder/$(cat demo/run-folder/.latest)/receipt"

# ── BEAT 7 ─────────────────────────────────────────────────────────────────
beat "cleanup" "Back to a clean tree. The takeaway: a tamper-evidence regression that compiled clean and passed the secret scan was stopped by a floor the selector isn't allowed to skip — before it reached review."
pause
run demo/scripts/revert.sh
echo
run git status --short
DONE=1
trap - EXIT
echo
echo "${GREEN}${B}demo complete — tree clean.${R}"
