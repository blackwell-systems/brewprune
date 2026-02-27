#!/bin/bash
# brewprune integration test suite
# Exercises the full pipeline: scan → shims → daemon → tracking → recommendations
set -euo pipefail

PASS=0
FAIL=0

# ── Helpers ──────────────────────────────────────────────────────────────────

ok() {
  echo "  ✓ $1"
  PASS=$((PASS+1))
}

fail() {
  echo "  ✗ $1"
  echo "    details: $2"
  FAIL=$((FAIL+1))
}

assert_contains() {
  local desc="$1" expected="$2" actual="$3"
  if echo "$actual" | grep -q "$expected"; then
    ok "$desc"
  else
    fail "$desc" "expected '$expected' in: $actual"
  fi
}

assert_file_exists() {
  local desc="$1" path="$2"
  if [[ -e "$path" ]]; then
    ok "$desc"
  else
    fail "$desc" "missing: $path"
  fi
}

assert_exit_zero() {
  local desc="$1" cmd="${@:2}"
  if $cmd >/dev/null 2>&1; then
    ok "$desc"
  else
    fail "$desc" "non-zero exit from: $cmd"
  fi
}

separator() {
  echo ""
  echo "─── $1 ───────────────────────────────────────────"
}

# ── 1. Scan ──────────────────────────────────────────────────────────────────
separator "Step 1: scan"

SCAN_OUT=$(brewprune scan 2>&1) || { echo "FATAL: brewprune scan failed:"; echo "$SCAN_OUT"; exit 1; }
assert_contains "scan reports formulae indexed"  "formulae"   "$SCAN_OUT"
assert_contains "scan reports shims created"     "shim"       "$SCAN_OUT"

# ── 2. Shim verification ─────────────────────────────────────────────────────
separator "Step 2: shim verification"

SHIMDIR="$HOME/.brewprune/bin"
assert_file_exists "shim dir exists"              "$SHIMDIR"
assert_file_exists "git shim created"             "$SHIMDIR/git"
assert_file_exists "curl shim created"            "$SHIMDIR/curl"
assert_file_exists "jq shim created"              "$SHIMDIR/jq"
assert_file_exists "ripgrep shim created"         "$SHIMDIR/rg"
assert_file_exists "brewprune-shim binary present" "$SHIMDIR/brewprune-shim"

# Verify shims are symlinks to brewprune-shim
if [[ -L "$SHIMDIR/git" ]]; then
  ok "git shim is a symlink"
else
  fail "git shim is a symlink" "expected symlink at $SHIMDIR/git"
fi

# ── 3. Status before daemon ──────────────────────────────────────────────────
separator "Step 3: status (pre-daemon)"

STATUS_OUT=$(brewprune status 2>&1)
assert_contains "status shows formulae count" "formulae"  "$STATUS_OUT"
assert_contains "status shows shim count"     "commands"  "$STATUS_OUT"

# ── 4. Start daemon ──────────────────────────────────────────────────────────
separator "Step 4: daemon startup"

brewprune watch --daemon 2>/dev/null
sleep 1  # give daemon a moment to write PID

PID_FILE="$HOME/.brewprune/daemon.pid"
assert_file_exists "daemon PID file created" "$PID_FILE"

DAEMON_PID=$(cat "$PID_FILE" 2>/dev/null || echo "")
if [[ -n "$DAEMON_PID" ]] && kill -0 "$DAEMON_PID" 2>/dev/null; then
  ok "daemon process is running (PID $DAEMON_PID)"
else
  fail "daemon process is running" "PID='$DAEMON_PID' not alive"
fi

# ── 5. Usage tracking (run shimmed commands) ──────────────────────────────────
separator "Step 5: usage tracking"

echo "  → running shimmed commands (git, curl, jq)..."

# Run through shims — these will be logged to usage.log
git --version         >/dev/null 2>&1
curl --version        >/dev/null 2>&1
jq --version          >/dev/null 2>&1
git --version         >/dev/null 2>&1  # second run for git (higher frequency)
curl --head --silent --output /dev/null http://example.com 2>/dev/null || true

USAGE_LOG="$HOME/.brewprune/usage.log"
assert_file_exists "usage.log created by shim" "$USAGE_LOG"

USAGE_LINES=$(wc -l < "$USAGE_LOG" 2>/dev/null || echo 0)
if [[ "$USAGE_LINES" -ge 4 ]]; then
  ok "usage.log has $USAGE_LINES entries"
else
  fail "usage.log has entries" "expected ≥4 lines, got $USAGE_LINES"
fi

# ── 6. Wait for daemon to process events ────────────────────────────────────
separator "Step 6: waiting for daemon"

echo "  → polling usage_events table (up to 30s)..."
MAX_WAIT=30
ELAPSED=0
EVENT_COUNT=0
while [[ $ELAPSED -lt $MAX_WAIT ]]; do
  EVENT_COUNT=$(sqlite3 "$HOME/.brewprune/brewprune.db" \
    "SELECT COUNT(*) FROM usage_events;" 2>/dev/null || echo 0)
  if [[ "$EVENT_COUNT" -gt 0 ]]; then
    break
  fi
  sleep 1
  ELAPSED=$((ELAPSED+1))
done

if [[ "$EVENT_COUNT" -gt 0 ]]; then
  ok "daemon inserted $EVENT_COUNT events into DB (${ELAPSED}s)"
else
  fail "daemon inserted events" "0 events after ${MAX_WAIT}s — check daemon logs"
fi

# ── 7. Stats ──────────────────────────────────────────────────────────────────
separator "Step 7: stats"

STATS_OUT=$(brewprune stats 2>&1)
assert_contains "stats shows git activity"   "git"  "$STATS_OUT"
assert_contains "stats shows curl activity"  "curl" "$STATS_OUT"

# ── 8. Unused recommendations ────────────────────────────────────────────────
separator "Step 8: unused"

UNUSED_OUT=$(brewprune unused 2>&1)
assert_contains "unused output shows packages"       "SAFE\|MEDIUM\|RISKY\|safe\|medium\|risky" "$UNUSED_OUT"
assert_contains "ripgrep appears (never used)"        "ripgrep"  "$UNUSED_OUT"
assert_contains "htop appears (never used)"           "htop"     "$UNUSED_OUT"
assert_contains "tree appears (never used)"           "tree"     "$UNUSED_OUT"
assert_contains "disclaimer present"                  "Safe = low" "$UNUSED_OUT"

# Verify git/curl/jq score lower than ripgrep/htop/tree (used vs unused)
GIT_SCORE=$(brewprune explain git 2>/dev/null | grep -oP 'Score: \K\d+' || echo 0)
RG_SCORE=$(brewprune explain ripgrep 2>/dev/null | grep -oP 'Score: \K\d+' || echo 0)
if [[ "$RG_SCORE" -gt "$GIT_SCORE" ]]; then
  ok "ripgrep (unused) scores higher than git (used): $RG_SCORE > $GIT_SCORE"
else
  fail "score ordering" "expected ripgrep($RG_SCORE) > git($GIT_SCORE)"
fi

# ── 9. Doctor self-test ───────────────────────────────────────────────────────
separator "Step 9: doctor"

DOCTOR_OUT=$(brewprune doctor 2>&1)
assert_contains "doctor runs without fatal error" "Pipeline\|pass\|fail" "$DOCTOR_OUT"

# ── 10. Remove dry-run ───────────────────────────────────────────────────────
separator "Step 10: remove --safe --dry-run"

REMOVE_OUT=$(brewprune remove --safe --dry-run 2>&1)
assert_contains "dry-run shows packages to remove" "Dry-run\|dry-run\|would" "$REMOVE_OUT"

# ── 11. scan --refresh-shims ────────────────────────────────────────────────
separator "Step 11: scan --refresh-shims"

REFRESH_OUT=$(brewprune scan --refresh-shims 2>&1)
assert_contains "refresh-shims runs cleanly" "Refreshed\|shim" "$REFRESH_OUT"

# ── 12. Cleanup ──────────────────────────────────────────────────────────────
separator "Step 12: cleanup"

if [[ -n "$DAEMON_PID" ]]; then
  kill "$DAEMON_PID" 2>/dev/null && ok "daemon stopped" || ok "daemon already stopped"
fi

# ── Summary ──────────────────────────────────────────────────────────────────
echo ""
echo "════════════════════════════════════════════"
echo "  Results: $PASS passed, $FAIL failed"
echo "════════════════════════════════════════════"
echo ""

if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
exit 0
