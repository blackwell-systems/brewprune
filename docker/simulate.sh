#!/usr/bin/env bash
# simulate.sh — New-user simulation for brewprune sandbox
#
# Run inside the sandbox container:
#   docker compose -f docker/docker-compose.sandbox.yml run --rm sandbox bash /home/brewuser/simulate.sh
set -euo pipefail

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

pass=0
fail=0

check() {
    local desc="$1"
    shift
    printf "${CYAN}TEST:${NC} %s ... " "$desc"
    if output=$("$@" 2>&1); then
        printf "${GREEN}PASS${NC}\n"
        pass=$((pass + 1))
    else
        printf "${RED}FAIL${NC}\n"
        echo "$output" | head -20
        fail=$((fail + 1))
    fi
}

check_contains() {
    local desc="$1"
    local needle="$2"
    shift 2
    printf "${CYAN}TEST:${NC} %s ... " "$desc"
    if output=$("$@" 2>&1) && echo "$output" | grep -qi "$needle"; then
        printf "${GREEN}PASS${NC}\n"
        pass=$((pass + 1))
    else
        printf "${RED}FAIL${NC}\n"
        echo "  Expected to find: $needle"
        echo "  Got:"
        echo "$output" | head -20 | sed 's/^/    /'
        fail=$((fail + 1))
    fi
}

echo "======================================="
echo "  brewprune new-user simulation"
echo "======================================="
echo

# ── Phase 1: First Contact ──────────────────────────────────────────
echo "-- Phase 1: First Contact --"
check "brewprune is installed" which brewprune
check "brewprune-shim is installed" which brewprune-shim
check "brew is available" which brew
check_contains "brew has packages installed" "jq" brew list --formula

# ── Phase 2: Initial Setup ──────────────────────────────────────────
echo
echo "-- Phase 2: Initial Setup --"
check "scan finds packages" brewprune scan
check_contains "status shows formulae" "formul" brewprune status

# ── Phase 3: Unused Analysis (before any usage tracking) ────────────
echo
echo "-- Phase 3: Unused Analysis (pre-tracking) --"
check "unused runs successfully" brewprune unused
check_contains "unused --all shows packages" "Package" brewprune unused --all
check_contains "unused --tier safe shows safe tier" "safe\|SAFE" brewprune unused --tier safe || true

# ── Phase 4: Usage Tracking ─────────────────────────────────────────
echo
echo "-- Phase 4: Usage Tracking --"
check "start daemon" brewprune watch --daemon
sleep 1

# Add shim directory to PATH (as a real user would)
export PATH="$HOME/.brewprune/bin:$PATH"

# Generate usage by running brew binaries through shims
echo "  Generating usage events..."
for cmd in jq rg fd bat git curl; do
    if command -v "$cmd" >/dev/null 2>&1; then
        "$cmd" --version 2>/dev/null || "$cmd" --help 2>/dev/null || true
    else
        echo "  (shim not found: $cmd)"
    fi
done
# Wait for daemon to process the usage log (processes every 30s)
echo "  Waiting for daemon to process events..."
sleep 35

# ── Phase 5: Stats ──────────────────────────────────────────────────
echo
echo "-- Phase 5: Stats --"
check "stats runs successfully" brewprune stats

# ── Phase 6: Explain ────────────────────────────────────────────────
echo
echo "-- Phase 6: Explain --"
check "explain jq works" brewprune explain jq
check "explain ripgrep works" brewprune explain ripgrep

# ── Phase 7: Doctor ─────────────────────────────────────────────────
echo
echo "-- Phase 7: Doctor --"
check "doctor runs" brewprune doctor

# ── Phase 8: Remove dry-run ─────────────────────────────────────────
echo
echo "-- Phase 8: Remove (dry-run) --"
check "remove --safe --dry-run works" brewprune remove --safe --dry-run || true

# ── Cleanup ─────────────────────────────────────────────────────────
brewprune watch --stop 2>/dev/null || true

# ── Results ─────────────────────────────────────────────────────────
echo
echo "======================================="
echo "  Results: ${pass} passed, ${fail} failed"
echo "======================================="

[ "$fail" -eq 0 ] && exit 0 || exit 1
