#!/bin/bash
# Enforcer — Test Runner with Live Monitoring Dashboard
# Author: Deepankar Das
#
# 1. Launches dashboard server first (waits for it to settle)
# 2. Opens browser to dashboard on port 7000
# 3. Runs tests — dashboard polls for live results
# 4. Dashboard shows final summary + per-file breakdown
#
# Usage:
#   ./scripts/run_tests.sh                          # Run all tests + dashboard on port 7000
#   ./scripts/run_tests.sh --no-dashboard           # Run tests only
#   ./scripts/run_tests.sh --port 7001              # Custom dashboard port
#   ./scripts/run_tests.sh --filter policy           # Filter for Vitest files
#   ./scripts/run_tests.sh --unit-only              # Skip integration tests
#   ./scripts/run_tests.sh --integration-only       # Skip unit tests

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

DASHBOARD_PORT="${TEST_DASHBOARD_PORT:-7000}"
NO_DASHBOARD=false
FILTER=""
RUN_UNIT=true
RUN_INTEGRATION=true
RESULTS_DIR="$PROJECT_ROOT/.test-results"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

banner() {
  local phase="$1"
  echo ""
  echo -e "${CYAN}${BOLD}╔══════════════════════════════════════════════════════════════╗${NC}"
  printf "${CYAN}${BOLD}║${NC} %-60s ${CYAN}${BOLD}║${NC}\n" ""
  printf "${CYAN}${BOLD}║${NC} %-60s ${CYAN}${BOLD}║${NC}\n" "   _   _     ___ ___ ___ _____      ___   _    _"
  printf "${CYAN}${BOLD}║${NC} %-60s ${CYAN}${BOLD}║${NC}\n" "  /_\\ /_\\   | __|_ _| _ \\ __\\ \\    / /_\\ | |  | |"
  printf "${CYAN}${BOLD}║${NC} %-60s ${CYAN}${BOLD}║${NC}\n" " / _ \\ _ \\  | _| | ||   / _| \\ \\/\\/ / _ \\| |__| |__"
  printf "${CYAN}${BOLD}║${NC} %-60s ${CYAN}${BOLD}║${NC}\n" " /_/ \\_\\/\\_\\ |_| |___|_|_\\___|  \\_/\\_/_/ \\_\\____|____|"
  printf "${CYAN}${BOLD}║${NC} %-60s ${CYAN}${BOLD}║${NC}\n" ""
  printf "${CYAN}${BOLD}║${NC} %-60s ${CYAN}${BOLD}║${NC}\n" "$phase"
  echo -e "${CYAN}${BOLD}╚══════════════════════════════════════════════════════════════╝${NC}"
  echo ""
}

load_env_file() {
  if [[ -f "$PROJECT_ROOT/.env" ]]; then
    set -a
    source "$PROJECT_ROOT/.env"
    set +a
  fi
}

while [[ $# -gt 0 ]]; do
  case $1 in
    --no-dashboard) NO_DASHBOARD=true; shift ;;
    --port) DASHBOARD_PORT="$2"; shift 2 ;;
    --filter) FILTER="$2"; shift 2 ;;
    --unit-only) RUN_INTEGRATION=false; shift ;;
    --integration-only) RUN_UNIT=false; shift ;;
    --help|-h)
      echo "Usage: $0 [--no-dashboard] [--port PORT] [--filter PATTERN] [--unit-only|--integration-only]"
      echo ""
      echo "Options:"
      echo "  --no-dashboard      Run tests without the live monitoring dashboard"
      echo "  --port PORT         Dashboard port (default: 7000)"
      echo "  --filter PATTERN    Filter test files by name pattern"
      echo "  --unit-only         Run only unit tests (skip integration)"
      echo "  --integration-only  Run only integration tests (skip unit)"
      exit 0
      ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

if [[ "$RUN_UNIT" == "false" && "$RUN_INTEGRATION" == "false" ]]; then
  echo "Nothing to run: both unit and integration suites are disabled."
  exit 1
fi

load_env_file

mkdir -p "$RESULTS_DIR"

# ── Collect system metrics ───────────────────────────────────────────────────

collect_metrics() {
  local cpu_cores cpu_usage total_mem used_mem free_mem mem_pct node_ver
  cpu_cores=$(sysctl -n hw.ncpu 2>/dev/null || nproc 2>/dev/null || echo 0)
  node_ver=$(node --version 2>/dev/null || echo "unknown")

  if [[ "$(uname)" == "Darwin" ]]; then
    cpu_usage=$(ps -A -o %cpu | awk '{s+=$1} END {printf "%.1f", s}' 2>/dev/null || echo 0)
    total_mem=$(( $(sysctl -n hw.memsize 2>/dev/null || echo 0) / 1024 / 1024 ))
    local page_size free_pages
    page_size=$(sysctl -n hw.pagesize 2>/dev/null || echo 4096)
    free_pages=$(vm_stat 2>/dev/null | grep "Pages free" | awk '{print $3}' | tr -d '.' || echo 0)
    free_mem=$(( (${free_pages:-0} * page_size) / 1024 / 1024 ))
    used_mem=$((total_mem - free_mem))
    mem_pct=$(( total_mem > 0 ? (used_mem * 100) / total_mem : 0 ))
  else
    total_mem=$(free -m 2>/dev/null | awk '/Mem:/ {print $2}' || echo 0)
    used_mem=$(free -m 2>/dev/null | awk '/Mem:/ {print $3}' || echo 0)
    free_mem=$(free -m 2>/dev/null | awk '/Mem:/ {print $4}' || echo 0)
    mem_pct=$(free 2>/dev/null | awk '/Mem:/ {printf "%.0f", $3/$2*100}' || echo 0)
    cpu_usage="0"
  fi

  cat << EOF
{"cpuCores":$cpu_cores,"cpuUsage":$cpu_usage,"memTotalMb":$total_mem,"memUsedMb":$used_mem,"memFreeMb":$free_mem,"memPct":$mem_pct,"nodeVersion":"$node_ver"}
EOF
}

# ── Count test files ─────────────────────────────────────────────────────────

count_test_files() {
  local unit_files integration_files all_files
  unit_files=$(find "$PROJECT_ROOT/tests" -maxdepth 1 \( -name "*.test.ts" -o -name "*.test.tsx" \) 2>/dev/null | wc -l | tr -d ' ')
  integration_files=$(find "$PROJECT_ROOT/tests/integration" \( -name "*.test.ts" -o -name "*.test.tsx" \) 2>/dev/null | wc -l | tr -d ' ')
  all_files=$((unit_files + integration_files))
  echo "{\"totalFiles\":$all_files,\"unitFiles\":$unit_files,\"integrationFiles\":$integration_files}"
}

# ── Write initial state ──────────────────────────────────────────────────────

METRICS=$(collect_metrics)
FILE_COUNTS=$(count_test_files)

cat > "$RESULTS_DIR/state.json" << EOF
{
  "phase": "waiting",
  "startedAt": "$(date -u '+%Y-%m-%dT%H:%M:%SZ')",
  "metrics": $METRICS,
  "fileCounts": $FILE_COUNTS,
  "results": null,
  "durationMs": 0,
  "exitCode": null
}
EOF

# ── Generate dashboard HTML ──────────────────────────────────────────────────

cat > "$RESULTS_DIR/dashboard.html" << 'ENDHTML'
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Enforcer — Test Dashboard</title>
<style>
  :root { --bg:#0f172a; --card:#1e293b; --border:#334155; --text:#e2e8f0; --dim:#94a3b8;
    --green:#22c55e; --red:#ef4444; --yellow:#eab308; --blue:#3b82f6; --cyan:#06b6d4; --purple:#a855f7; --orange:#f97316; }
  *{margin:0;padding:0;box-sizing:border-box}
  body{font-family:'Inter',-apple-system,system-ui,sans-serif;background:var(--bg);color:var(--text);padding:20px 24px}
  .hdr{text-align:center;margin-bottom:28px}
  .hdr h1{font-size:26px;font-weight:700;color:var(--cyan);letter-spacing:-0.5px}
  .hdr .tag{font-size:12px;color:var(--dim);margin-top:2px}
  .hdr .phase{display:inline-block;margin-top:8px;padding:4px 14px;border-radius:20px;font-size:13px;font-weight:600}
  .phase-waiting{background:#1e3a5f;color:var(--blue)}
  .phase-running{background:#1a3326;color:var(--green);animation:pulse 1.5s infinite}
  .phase-done{background:#1a3326;color:var(--green)}
  .phase-failed{background:#3b1a1a;color:var(--red)}
  @keyframes pulse{0%,100%{opacity:1}50%{opacity:.6}}

  .summary{display:grid;grid-template-columns:repeat(6,1fr);gap:12px;margin-bottom:20px}
  @media(max-width:900px){.summary{grid-template-columns:repeat(3,1fr)}}
  .scard{background:var(--card);border:1px solid var(--border);border-radius:10px;padding:14px 16px;text-align:center}
  .scard .lbl{font-size:10px;text-transform:uppercase;letter-spacing:.08em;color:var(--dim)}
  .scard .val{font-size:28px;font-weight:700;margin-top:2px;line-height:1.1}
  .scard .sub{font-size:11px;color:var(--dim);margin-top:2px}

  .row{display:grid;grid-template-columns:1fr 1fr;gap:16px;margin-bottom:16px}
  @media(max-width:800px){.row{grid-template-columns:1fr}}
  .sec{background:var(--card);border:1px solid var(--border);border-radius:10px;padding:16px}
  .sec h2{font-size:13px;font-weight:600;color:var(--cyan);margin-bottom:10px;text-transform:uppercase;letter-spacing:.04em}
  .sec.full{grid-column:1/-1}

  .mi{display:flex;justify-content:space-between;padding:6px 0;border-bottom:1px solid var(--border);font-size:13px}
  .mi:last-child{border:none}
  .mi .k{color:var(--dim)}
  .mi .v{font-weight:600}

  table{width:100%;border-collapse:collapse;font-size:13px}
  th{color:var(--dim);font-weight:500;font-size:11px;text-transform:uppercase;text-align:left;padding:8px 10px;border-bottom:1px solid var(--border)}
  td{padding:8px 10px;border-bottom:1px solid var(--border)}
  tr:last-child td{border:none}
  .pass{color:var(--green);font-weight:600}
  .fail{color:var(--red);font-weight:600}

  .bar-row{display:flex;align-items:center;gap:8px;margin-bottom:8px}
  .bar-label{min-width:56px;font-size:12px;color:var(--dim)}
  .bar{flex:1;height:8px;border-radius:4px;background:var(--border);overflow:hidden}
  .bar-fill{height:100%;border-radius:4px;transition:width .4s}
  .bar-pct{min-width:48px;text-align:right;font-size:12px;color:var(--dim)}

  .ts{text-align:center;color:var(--dim);font-size:11px;margin-top:16px}
</style>
</head>
<body>
<div class="hdr">
  <h1>Enforcer Test Dashboard</h1>
  <div class="tag">Runtime security and governance for AI coding agents</div>
  <div id="phase" class="phase phase-waiting">Waiting for test run...</div>
</div>

<div class="summary" id="summary"></div>

<div class="row">
  <div class="sec" id="sys-metrics"><h2>System Metrics</h2><div id="sys-body"></div></div>
  <div class="sec" id="resource-sec"><h2>Resource Usage</h2><div id="bars"></div></div>
</div>

<div class="sec full" id="files-sec">
  <h2>Test Files</h2>
  <table><thead><tr>
    <th>File</th><th>Category</th><th>Tests</th><th>Passed</th><th>Failed</th><th>Duration</th><th>Tests/sec</th><th>Status</th>
  </tr></thead><tbody id="ftbody"></tbody></table>
</div>

<div class="ts" id="ts"></div>

<script>
const STATE_URL = '/state.json';
let lastPhase = '';

async function poll() {
  try {
    const r = await fetch(STATE_URL + '?t=' + Date.now());
    if (!r.ok) return;
    const s = await r.json();
    render(s);
  } catch {}
}

function render(s) {
  const ph = document.getElementById('phase');
  const phase = s.phase || 'waiting';
  if (phase !== lastPhase) {
    lastPhase = phase;
    ph.className = 'phase phase-' + phase;
    ph.textContent = phase === 'waiting' ? 'Waiting for test run...'
      : phase === 'running' ? 'Running tests...'
      : phase === 'done' ? 'All tests passed'
      : 'Tests failed';
  }

  const fc = s.fileCounts || {};
  const res = s.results || {};
  const suites = res.testResults || [];
  const totalTests = suites.reduce((a, f) => a + (f.assertionResults || []).length, 0);
  const passed = suites.reduce((a, f) => a + (f.assertionResults || []).filter(t => t.status === 'passed').length, 0);
  const failed = totalTests - passed;
  const dur = ((s.durationMs || 0) / 1000).toFixed(2);
  const tps = s.durationMs > 0 ? (totalTests / (s.durationMs / 1000)).toFixed(1) : '--';

  const unitTests = suites.filter(f => !f.name.includes('/integration/')).reduce((a, f) => a + (f.assertionResults || []).length, 0);
  const intTests = suites.filter(f => f.name.includes('/integration/')).reduce((a, f) => a + (f.assertionResults || []).length, 0);

  document.getElementById('summary').innerHTML = [
    card('Test Files', fc.totalFiles || suites.length || '--', 'cyan', (fc.unitFiles||0) + ' unit · ' + (fc.integrationFiles||0) + ' integration'),
    card('Total Tests', totalTests || '--', 'blue', unitTests + ' unit · ' + intTests + ' integration'),
    card('Passed', passed || (phase === 'waiting' ? '--' : '0'), 'green'),
    card('Failed', failed || (phase === 'waiting' ? '--' : '0'), failed > 0 ? 'red' : 'green'),
    card('Duration', phase === 'waiting' ? '--' : dur + 's', 'purple'),
    card('Throughput', phase === 'waiting' ? '--' : tps + '/s', 'yellow', 'tests per second'),
  ].join('');

  const m = s.metrics || {};
  document.getElementById('sys-body').innerHTML = [
    mi('CPU Cores', m.cpuCores || 'N/A'),
    mi('CPU Usage', (m.cpuUsage || 0).toFixed(1) + '%'),
    mi('Memory Total', (m.memTotalMb || 0).toLocaleString() + ' MB'),
    mi('Memory Used', (m.memUsedMb || 0).toLocaleString() + ' MB'),
    mi('Memory Free', (m.memFreeMb || 0).toLocaleString() + ' MB'),
    mi('Node.js', m.nodeVersion || 'N/A'),
    mi('Exit Code', s.exitCode === null ? '--' : s.exitCode === 0 ? '0 ✓' : s.exitCode + ' ✗'),
  ].join('');

  const cpuPct = Math.min(100, m.cpuUsage || 0);
  const memPct = m.memPct || 0;
  document.getElementById('bars').innerHTML =
    bar('CPU', cpuPct, cpuPct > 80 ? 'var(--red)' : cpuPct > 50 ? 'var(--yellow)' : 'var(--green)') +
    bar('Memory', memPct, memPct > 80 ? 'var(--red)' : memPct > 50 ? 'var(--yellow)' : 'var(--green)');

  const tbody = document.getElementById('ftbody');
  if (suites.length) {
    tbody.innerHTML = suites.map(su => {
      const nm = su.name.split('/').pop();
      const cat = su.name.includes('/integration/') ? 'Integration' : 'Unit';
      const t = (su.assertionResults || []);
      const tot = t.length, p = t.filter(x => x.status === 'passed').length, f = tot - p;
      const ms = (su.endTime || 0) - (su.startTime || 0);
      const d = (ms / 1000).toFixed(2);
      const tp = ms > 0 ? (tot / (ms / 1000)).toFixed(1) : '∞';
      return '<tr><td><code>' + nm + '</code></td><td>' + cat + '</td><td>' + tot +
        '</td><td class="pass">' + p + '</td><td class="' + (f > 0 ? 'fail' : '') + '">' + f +
        '</td><td>' + d + 's</td><td>' + tp + '/s</td><td class="' + (f > 0 ? 'fail' : 'pass') + '">' +
        (f > 0 ? 'FAIL' : 'PASS') + '</td></tr>';
    }).join('');
  } else if (phase === 'running') {
    tbody.innerHTML = '<tr><td colspan="8" style="text-align:center;color:var(--dim);padding:20px">Running tests...</td></tr>';
  }

  document.getElementById('ts').textContent = 'Last updated: ' + new Date().toLocaleString();
}

function card(lbl, val, cls, sub) {
  return '<div class="scard"><div class="lbl">' + lbl + '</div><div class="val" style="color:var(--' + cls + ')">' +
    val + '</div>' + (sub ? '<div class="sub">' + sub + '</div>' : '') + '</div>';
}
function mi(k, v) { return '<div class="mi"><span class="k">' + k + '</span><span class="v">' + v + '</span></div>'; }
function bar(lbl, pct, col) {
  return '<div class="bar-row"><span class="bar-label">' + lbl + '</span><div class="bar"><div class="bar-fill" style="width:' +
    pct + '%;background:' + col + '"></div></div><span class="bar-pct">' + pct.toFixed(1) + '%</span></div>';
}

setInterval(poll, 1000);
poll();
</script>
</body>
</html>
ENDHTML

# ── Launch dashboard server ──────────────────────────────────────────────────

if [[ "$NO_DASHBOARD" == "true" ]]; then
  echo -e "${BLUE}[TEST]${NC} Running tests (no dashboard)..."
else
  lsof -ti tcp:"${DASHBOARD_PORT}" 2>/dev/null | xargs kill -9 2>/dev/null || true
  sleep 0.5

  echo -e "${BLUE}[DASH]${NC} Starting dashboard on port $DASHBOARD_PORT..."

  node -e "
    const http = require('http');
    const fs = require('fs');
    const path = require('path');
    const dir = '${RESULTS_DIR}';
    http.createServer((req, res) => {
      const url = req.url.split('?')[0];
      if (url === '/state.json') {
        try {
          const data = fs.readFileSync(path.join(dir, 'state.json'), 'utf-8');
          res.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'no-cache' });
          res.end(data);
        } catch { res.writeHead(404); res.end('{}'); }
      } else {
        try {
          const html = fs.readFileSync(path.join(dir, 'dashboard.html'), 'utf-8');
          res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
          res.end(html);
        } catch { res.writeHead(500); res.end('Dashboard not found'); }
      }
    }).listen(${DASHBOARD_PORT}, () => {
      console.log('Dashboard: http://localhost:${DASHBOARD_PORT}');
    });
  " &
  DASHBOARD_PID=$!

  for _i in $(seq 1 10); do
    if curl -s "http://localhost:${DASHBOARD_PORT}" > /dev/null 2>&1; then
      break
    fi
    sleep 0.3
  done

  echo -e "${GREEN}[DASH]${NC} Dashboard ready at ${BOLD}http://localhost:${DASHBOARD_PORT}${NC}"

  if [[ "$(uname)" == "Darwin" ]]; then
    open "http://localhost:${DASHBOARD_PORT}" 2>/dev/null || true
  fi

  echo ""
fi

# ── Update state to "running" ────────────────────────────────────────────────

METRICS=$(collect_metrics)
FILE_COUNTS=$(count_test_files)

cat > "$RESULTS_DIR/state.json" << EOF
{
  "phase": "running",
  "startedAt": "$(date -u '+%Y-%m-%dT%H:%M:%SZ')",
  "metrics": $METRICS,
  "fileCounts": $FILE_COUNTS,
  "results": null,
  "durationMs": 0,
  "exitCode": null
}
EOF

# ── Run tests ────────────────────────────────────────────────────────────────

banner "Test Runner Start"

TEST_START_S=$(date +%s)

RESULTS_FILE="$RESULTS_DIR/results.json"
rm -f "$RESULTS_FILE"

TEST_EXIT=0

cd "$PROJECT_ROOT"

VITEST_ARGS="run --reporter=json --reporter=default --outputFile=$RESULTS_FILE"

if [[ -n "$FILTER" ]]; then
  VITEST_ARGS="$VITEST_ARGS $FILTER"
fi

if [[ "$RUN_UNIT" == "true" && "$RUN_INTEGRATION" == "false" ]]; then
  VITEST_ARGS="$VITEST_ARGS --exclude='**/integration/**'"
  echo -e "${BLUE}[UNIT]${NC} Running unit tests only..."
elif [[ "$RUN_UNIT" == "false" && "$RUN_INTEGRATION" == "true" ]]; then
  VITEST_ARGS="$VITEST_ARGS tests/integration"
  echo -e "${BLUE}[INT]${NC} Running integration tests only..."
else
  echo -e "${BLUE}[ALL]${NC} Running all tests..."
fi

npx vitest $VITEST_ARGS 2>&1 || TEST_EXIT=$?

TEST_END_S=$(date +%s)
TEST_DURATION_MS=$(( (TEST_END_S - TEST_START_S) * 1000 ))

# ── Update state to final ────────────────────────────────────────────────────

METRICS=$(collect_metrics)
RESULTS_JSON=""
if [[ -f "$RESULTS_FILE" ]]; then
  RESULTS_JSON=$(cat "$RESULTS_FILE")
fi

PHASE="done"
if [[ $TEST_EXIT -ne 0 ]]; then
  PHASE="failed"
fi

cat > "$RESULTS_DIR/state.json" << EOF
{
  "phase": "$PHASE",
  "startedAt": "$(date -u '+%Y-%m-%dT%H:%M:%SZ')",
  "metrics": $METRICS,
  "fileCounts": $FILE_COUNTS,
  "results": ${RESULTS_JSON:-null},
  "durationMs": $TEST_DURATION_MS,
  "exitCode": $TEST_EXIT
}
EOF

echo ""
if [[ $TEST_EXIT -eq 0 ]]; then
  echo -e "${GREEN}${BOLD}All tests passed.${NC}"
  banner "Test Runner Complete"
else
  echo -e "${RED}${BOLD}Tests failed (exit code: $TEST_EXIT).${NC}"
  banner "Test Runner Failed"
fi

if [[ "$NO_DASHBOARD" != "true" ]]; then
  echo ""
  echo -e "${GREEN}Dashboard:${NC} http://localhost:${DASHBOARD_PORT}"
  echo -e "${DIM}Press Ctrl+C to stop${NC}"
  trap "kill $DASHBOARD_PID 2>/dev/null; exit $TEST_EXIT" INT TERM
  wait $DASHBOARD_PID 2>/dev/null || true
fi

exit $TEST_EXIT
