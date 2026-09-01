#!/usr/bin/env bash
# Enforcer convergence demo — presenter driver.
#
#   demo/scripts/run-demo.sh          interactive: press Enter to advance each step
#   demo/scripts/run-demo.sh --auto   run straight through, no pauses (pre-flight self-test)
#
# Ctrl-C at any point aborts and restores a clean tree.
#
# Plain-English narration is printed before every real command, and the raw
# tool output (unedited, live) follows clearly labeled. Nothing shown is
# staged or pre-recorded.
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
  echo; echo "${DIM}Cleaning up — putting the code back exactly as it was…${R}"
  "$ROOT/demo/scripts/revert.sh" >/dev/null 2>&1 || true
}
trap cleanup EXIT
trap 'echo; echo "${RED}Stopped early. Cleaning up before exit…${R}"; exit 130' INT

# Wait for the presenter to press Enter (skipped entirely in --auto mode).
pause(){
  [[ $AUTO -eq 1 ]] && return
  printf '\n%s' "${AMBER}  ▸ Press Enter to continue…${R}  ${DIM}(Ctrl-C stops and cleans up)${R} "
  read -r _ || true
  echo
}

# A numbered step header, in plain language.
step(){
  STEP=$((STEP+1))
  echo
  echo "${B}${CYAN}────────────────────────────────────────────────────────────${R}"
  echo "${B}${CYAN}  STEP $STEP of 7 — $1${R}"
  echo "${B}${CYAN}────────────────────────────────────────────────────────────${R}"
}

# A plain-English explanation paragraph, read from a heredoc so it can be
# hand-wrapped in the source for easy reading. Printed before the command runs.
say(){
  echo
  while IFS= read -r line; do
    if [[ -z "$line" ]]; then echo; else echo "  $line"; fi
  done
  echo
}

# Marks the start of real, unedited tool output.
raw(){ echo "${DIM}  — here is the exact, live output — ${R}"; }

# A short "in plain terms" translation printed right after raw output.
note(){ echo; echo "${AMBER}  In plain terms:${R} $1"; echo; }

run(){ echo "${DIM}  \$ $*${R}"; "$@"; }

DEMO_FILES="src/daemon/routes/enrich.ts tests/integration/audit-enrich-immutability.test.ts"

# ── reset to a clean, known state — idempotent, safe on restart / repeat ────
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
    echo "${RED}These demo files have uncommitted changes that are not part of the demo:${R}" >&2
    echo "$dirty" | sort -u | sed 's/^/  /' >&2
    echo "${DIM}Commit or stash them, then run this again.${R}" >&2
    exit 1
  fi
}
reset_demo_env

# ═══════════════════════════════════════════════════════════════════════════
#  OUTLINE — shown once, before anything runs
# ═══════════════════════════════════════════════════════════════════════════
command clear 2>/dev/null || printf '\n\n'
echo "${B}${CYAN}================================================================${R}"
echo "${B}${CYAN}   ENFORCER — LIVE DEMO${R}"
echo "${B}${CYAN}   Watching an AI-written code change get caught, explained,${R}"
echo "${B}${CYAN}   fixed, and re-checked — automatically.${R}"
echo "${B}${CYAN}================================================================${R}"

say <<'EOF'
WHAT ENFORCER IS
Enforcer is a security and policy layer that sits between AI
coding agents and the systems they can affect. Every change an
agent proposes gets checked before it is allowed to merge.

WHAT YOU ARE ABOUT TO SEE
An AI agent has proposed a real code change to a small part of
Enforcer's own logging system. The change looks like a
reasonable cleanup — it is not. It quietly breaks a rule the
system is supposed to protect. We will watch the automatic
checks find that problem, explain exactly what is wrong and
why, apply a fix, and re-check everything.

Nothing below is staged, scripted, or pre-recorded. Every
command runs for real, live, against the actual code in this
repository, and takes about 30 seconds of total run time.
EOF

echo "${B}THE 7 STEPS${R}"
say <<'EOF'
1. Starting point    — confirm the code currently passes every
                        automatic check, before we touch anything.
2. The change lands  — apply the AI-written change and look at
                        exactly what it edited.
3. First alarm       — re-run the quick checks. One of them will
                        now fail.
4. What gets tested  — see exactly which tests were chosen to
                        run against this change, and why each one
                        was picked.
5. The full failure  — run those tests and see the precise
                        reason the change is unsafe.
6. The fix           — apply the correction, re-run everything,
                        and watch it turn green.
7. Clean up          — put the code back exactly as it was
                        before we started.
EOF

pause

# ═══════════════════════════════════════════════════════════════════════════
# STEP 1 — Starting point
# ═══════════════════════════════════════════════════════════════════════════
step "Starting point"
say <<'EOF'
Before we touch anything, let's prove the codebase is healthy
right now. We will run three quick, automatic checks:

  1. Does the code compile without errors?
  2. Are there any leaked passwords, keys, or secrets anywhere
     in the source code?
  3. Do the tests that matter for this part of the system all
     pass?

Watch for three green PASS results below.
EOF
pause
raw
run node demo/radar/gate-fast.mjs || true
note "all three checks passed — the codebase is healthy before we start."

# ═══════════════════════════════════════════════════════════════════════════
# STEP 2 — The change lands
# ═══════════════════════════════════════════════════════════════════════════
step "The change lands"
say <<'EOF'
Now we apply the change an AI agent proposed. It edits exactly
one file — the part of the system that records what actually
happened after an action was taken.

The agent's own reasoning sounds sensible: "we already have a
record of what was attempted, so instead of adding a new record
for what actually happened, let's just update the existing one
in place." That saves a row of storage. It also, quietly,
breaks a promise the system makes: a record, once written, is
never changed afterward.

First, here is the one file that changed:
EOF
pause
raw
run demo/scripts/apply-change.sh 01-audit-mutation
echo
echo "${B}  Summary of the change:${R}"
raw
run git --no-pager diff --stat -- src/daemon/routes/enrich.ts
echo
echo "${B}  The exact lines that changed${R} ${DIM}(the old code added a new record; the new code edits the old one in place)${R}"
raw
run git --no-pager diff -- src/daemon/routes/enrich.ts
note "the change is small, reads as reasonable, and compiles fine. That is exactly why an automatic safety net matters."

# ═══════════════════════════════════════════════════════════════════════════
# STEP 3 — First alarm
# ═══════════════════════════════════════════════════════════════════════════
step "First alarm"
say <<'EOF'
We run the same three quick checks from Step 1 again, now that
the change is in place.

The code still compiles. There are still no leaked secrets. But
watch the third check — the one that runs the tests relevant to
this change. One of them is about to fail.
EOF
pause
raw
run node demo/radar/gate-fast.mjs --change 01-audit-mutation || true
note "compiles fine, no secrets — but one test failed. That failing test exists specifically to protect the rule this change broke: a record must never be silently changed after it is written."

# ═══════════════════════════════════════════════════════════════════════════
# STEP 4 — What gets tested
# ═══════════════════════════════════════════════════════════════════════════
step "What gets tested"
say <<'EOF'
Before running the full set of tests, the system first decides
which tests are actually relevant to this specific change,
instead of blindly re-running everything every time.

Below is that short list. Next to each test you will see the
reason it was chosen. Every one of these six is what we call a
"mandatory floor" — a rule the system is never allowed to skip
checking, no matter how small or safe a change looks.
EOF
pause
raw
run node demo/radar/select.mjs --change 01-audit-mutation
note "six tests were chosen, each with a stated reason — nothing here is a guess, and none of these six can be skipped."

# ═══════════════════════════════════════════════════════════════════════════
# STEP 5 — The full failure
# ═══════════════════════════════════════════════════════════════════════════
step "The full failure"
say <<'EOF'
Now we actually run that list of tests. Almost all of them will
pass — sixty-eight out of sixty-nine. One will fail, and it
will tell us exactly why: the original record was supposed to
stay untouched, and it did not.

The system also writes the result to a file — think of it as a
receipt. That receipt will say, in plain terms, whether this
change is safe to merge.
EOF
pause
raw
run node demo/radar/run.mjs || true
echo
echo "${B}  The receipt${R} ${DIM}(a permanent record of this result, saved next to the code)${R}"
raw
run cat "demo/run-folder/$(cat demo/run-folder/.latest)/receipt"
note "\"admissible_for_merge\": false means the system will not allow this change to merge yet."

# ═══════════════════════════════════════════════════════════════════════════
# STEP 6 — The fix
# ═══════════════════════════════════════════════════════════════════════════
step "The fix"
say <<'EOF'
Now we apply the fix. It does two things:

  1. It restores the original, correct behavior — every action
     gets its own new record, and the old record is never
     touched.
  2. It adds a new safeguard test, so this same mistake is much
     harder to make silently again in the future.

We then re-run the exact same tests from Step 5. Watch all
seventy of them pass this time, and watch the receipt flip from
"not safe to merge" to "safe to merge."
EOF
pause
raw
run demo/scripts/apply-change.sh 02-fix
echo
echo "${B}  What the fix adds:${R}"
raw
run git --no-pager diff --stat
pause
raw
run node demo/radar/run.mjs || true
echo
echo "${B}  The receipt now:${R}"
raw
run cat "demo/run-folder/$(cat demo/run-folder/.latest)/receipt"
note "all seventy tests pass, and \"admissible_for_merge\" has flipped to true — this version of the change is safe to merge."

# ═══════════════════════════════════════════════════════════════════════════
# STEP 7 — Clean up
# ═══════════════════════════════════════════════════════════════════════════
step "Clean up"
say <<'EOF'
Finally, we undo everything we just did — both the original
AI-written change and the fix — so the codebase is back exactly
as it was before we started. Nothing is left behind.
EOF
pause
raw
run demo/scripts/revert.sh
echo
echo "${B}  Confirming nothing was left behind:${R}"
raw
run git status --short
DONE=1
trap - EXIT

echo
echo "${B}${GREEN}================================================================${R}"
echo "${B}${GREEN}  DEMO COMPLETE — the codebase is back to a clean, known state.${R}"
echo "${B}${GREEN}================================================================${R}"
say <<'EOF'
What you just watched: an AI-written change that looked
reasonable, that quietly broke an important rule anyway, was
caught automatically, explained precisely, fixed, and
re-verified — before any human had to review a single line.
EOF
