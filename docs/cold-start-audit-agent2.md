# brewprune Cold-Start UX Audit - Agent 2

**Date:** 2026-02-28
**Environment:** Docker container `bp-audit4`
**Packages:** 40 Homebrew formulae (acl, bat, brotli, bzip2, ca-certificates, curl, cyrus-sasl, expat, fd, gettext, git, jq, keyutils, krb5, libedit, libevent, libgit2, libidn2, libnghttp2, libnghttp3, libngtcp2, libssh2, libunistring, libxcrypt, libxml2, lz4, ncurses, oniguruma, openldap, openssl@3, pcre2, readline, ripgrep, sqlite, tmux, utf8proc, util-linux, xz, zlib-ng-compat, zstd)

## Summary

| Severity | Count |
|----------|-------|
| UX-critical | 6 |
| UX-improvement | 12 |
| UX-polish | 15 |
| **Total** | **33** |

---

## 1. Discovery

### [DISCOVERY] Missing --version flag
- **Severity**: UX-critical
- **What happens**: `brewprune --version` returns "Error: unknown flag: --version" with exit code 1
- **Expected**: Standard CLI tools support `--version` to show version information. This is a critical discovery feature for users trying to understand what version they have installed.
- **Repro**: `docker exec bp-audit4 brewprune --version`

### [DISCOVERY] Error output duplicated
- **Severity**: UX-improvement
- **What happens**: All error messages appear exactly twice in the output (e.g., "Error: unknown flag: --version" appears 4 times with 2 blank lines between pairs)
- **Expected**: Each error should appear once
- **Repro**: Any error condition (invalid flag, missing command, etc.)

### [DISCOVERY] Exit code 1 for help display
- **Severity**: UX-improvement
- **What happens**: Running `brewprune` with no arguments shows help but exits with code 1, suggesting an error occurred
- **Expected**: Displaying help should exit with code 0 (success). Exit code 1 should be reserved for actual errors. This matters for scripting and CI/CD integration.
- **Repro**: `docker exec bp-audit4 sh -c 'brewprune; echo "Exit code: $?"'`

### [DISCOVERY] Help text duplicated on no-command error
- **Severity**: UX-polish
- **What happens**: The entire help text (38 lines) appears twice when running `brewprune` with no arguments
- **Expected**: Help text should appear once
- **Repro**: `docker exec bp-audit4 brewprune`

### [DISCOVERY] "no command specified" error is redundant
- **Severity**: UX-polish
- **What happens**: Help already clearly shows required command format. Adding "Error: no command specified" at the end is redundant.
- **Expected**: Either show help with exit 0, or show brief error with "run brewprune --help" suggestion
- **Repro**: `docker exec bp-audit4 brewprune`

### [DISCOVERY] doctor --fix flag not implemented
- **Severity**: UX-improvement
- **What happens**: `brewprune doctor --fix` returns "Error: unknown flag: --fix", but the doctor --help text mentions "--fix flag is not yet implemented"
- **Expected**: Either implement the flag or don't document it. Documenting unimplemented features creates confusion.
- **Repro**: `docker exec bp-audit4 brewprune doctor --fix`

---

## 2. Setup / Onboarding

### [SETUP] Quickstart fails on concurrent execution
- **Severity**: UX-critical
- **What happens**: Running `brewprune quickstart` when a daemon is already running results in "database is locked (5) (SQLITE_BUSY)" error
- **Expected**: Quickstart should detect existing daemon and either reuse it or provide clear instructions
- **Repro**: Run quickstart twice without stopping daemon between runs

### [SETUP] PATH configuration status confusing
- **Severity**: UX-improvement
- **What happens**: Status and doctor both report "PATH configured (restart shell to activate)" but also warn "⚠ Shim directory not in PATH — executions won't be intercepted". These messages contradict each other.
- **Expected**: Clear distinction between "written to shell config" vs "active in current session"
- **Repro**:
```bash
docker exec bp-audit4 brewprune quickstart
docker exec bp-audit4 brewprune status
docker exec bp-audit4 brewprune doctor
```

### [SETUP] Duplicate "brewprune shims" comment in shell config
- **Severity**: UX-polish
- **What happens**: The PATH modification in ~/.profile appears twice: both lines show "# brewprune shims" followed by the export statement
- **Expected**: Idempotent config modification - only add if not present
- **Repro**:
```bash
docker exec bp-audit4 brewprune quickstart
docker exec bp-audit4 sh -c 'grep brewprune ~/.profile'
```

### [SETUP] "brew services not supported on Linux" message unclear
- **Severity**: UX-polish
- **What happens**: Quickstart output says "brew found but using daemon mode (brew services not supported on Linux)"
- **Expected**: Either omit this (user doesn't care about implementation detail) or clarify why it matters (e.g., "using background daemon instead")
- **Repro**: `docker exec bp-audit4 brewprune quickstart`

### [SETUP] Confusing note about "events are from setup self-test"
- **Severity**: UX-improvement
- **What happens**: Status shows "Note: events are from setup self-test, not real shim interception. Real tracking starts when PATH is fixed and shims are in front of Homebrew."
- **Expected**: This is confusing for new users. The setup already added PATH to ~/.profile, so what's "not fixed"? Either make it actionable or don't show it.
- **Repro**: `docker exec bp-audit4 brewprune status` (after quickstart)

---

## 3. Core Feature: Unused Package Detection

### [UNUSED] Inconsistent tier filtering behavior with --all
- **Severity**: UX-improvement
- **What happens**:
  - `brewprune unused` shows safe+medium (hides risky with "use --all")
  - `brewprune unused --tier risky` shows only risky tier
  - `brewprune unused --all` shows all tiers
  - The help text says "--tier shows only that specific tier regardless of --all"
- **Expected**: The interaction between --tier and --all is confusing. Pick one model: either --tier is always a filter, or --all overrides --tier.
- **Repro**: Compare outputs of various tier/all combinations

### [UNUSED] Hidden count in summary is misleading
- **Severity**: UX-improvement
- **What happens**: Footer says "35 packages below score threshold hidden. Risky tier also hidden (use --all to include)." But with --min-score 70, only 5 packages are shown and 35 are hidden - the number includes both score filtering and tier filtering.
- **Expected**: Separate counts for "below score threshold" vs "hidden tier" or just say "35 packages hidden"
- **Repro**: `docker exec bp-audit4 brewprune unused --min-score 70`

### [UNUSED] Empty result when combining filters is cryptic
- **Severity**: UX-polish
- **What happens**: `brewprune unused --tier safe --min-score 90` shows "No packages match the specified criteria." with no context about what criteria were applied.
- **Expected**: Show what filters were active: "No packages match: tier=safe, min-score=90"
- **Repro**: `docker exec bp-audit4 brewprune unused --tier safe --min-score 90`

### [UNUSED] Size formatting inconsistency
- **Severity**: UX-polish
- **What happens**: Sizes shown as "5 MB", "976 KB", "1000 KB", "1004 KB" - inconsistent use of KB vs MB for values near 1 MB
- **Expected**: Convert 1000+ KB to MB for consistency
- **Repro**: `docker exec bp-audit4 brewprune unused --all`

### [UNUSED] "Uses (7d)" column header unclear on first use
- **Severity**: UX-polish
- **What happens**: Column header "Uses (7d)" might not be immediately clear to new users (uses in last 7 days? over 7 days?)
- **Expected**: "Last 7d" or "Recent Uses" or add footnote explaining the time window
- **Repro**: `docker exec bp-audit4 brewprune unused`

### [UNUSED] Verbose mode output is extremely long
- **Severity**: UX-improvement
- **What happens**: `brewprune unused --verbose` outputs detailed scoring for every package with full separator lines, making it hard to scan. Output is 200+ lines for 40 packages.
- **Expected**: Consider paginating, summarizing, or suggesting to pipe to less. Or limit verbose to specific tier.
- **Repro**: `docker exec bp-audit4 brewprune unused --verbose`

---

## 4. Data / Tracking

### [TRACKING] Status note about self-test events is persistent
- **Severity**: UX-improvement
- **What happens**: Even after commands are executed (git, jq, fd), status still shows "Note: events are from setup self-test, not real shim interception"
- **Expected**: This note should disappear once real usage is detected, or clarify conditions
- **Repro**:
```bash
docker exec bp-audit4 sh -c 'git --version && jq --version && fd --version'
docker exec bp-audit4 brewprune status
```

### [TRACKING] No indication that commands weren't intercepted
- **Severity**: UX-critical
- **What happens**: After running `git --version && jq --version && fd --version` in the container, the event count didn't increase (remained at 2). This means shims aren't working, but the user gets no feedback about this critical failure.
- **Expected**: Either status should show "shims not active - no events in last 30s" warning, or doctor should fail if events aren't being logged
- **Repro**:
```bash
docker exec bp-audit4 sh -c 'git --version'
docker exec bp-audit4 brewprune status  # event count doesn't increase
```

### [TRACKING] Daemon restart message could be clearer
- **Severity**: UX-polish
- **What happens**: `brewprune watch --daemon` when daemon is already running says "Daemon already running (PID 3100). Nothing to do."
- **Expected**: Good message, but could add "use --stop to stop it first" for clarity
- **Repro**: `docker exec bp-audit4 brewprune watch --daemon` (when already running)

### [TRACKING] Scan command provides no detail
- **Severity**: UX-polish
- **What happens**: `brewprune scan` shows "✓ Database up to date (40 packages, 0 changes)" with no indication of what it actually checked or did
- **Expected**: When run with no changes, this is fine. But first run should show more detail (scanning, building deps, creating shims, etc.)
- **Repro**: `docker exec bp-audit4 brewprune scan` (after initial scan)

### [TRACKING] No progress indicator during 30-second wait
- **Severity**: UX-improvement
- **What happens**: Several operations (doctor pipeline test, quickstart self-test) wait up to 35 seconds with dots showing progress, but the dots appear slowly and there's no ETA
- **Expected**: Show "waiting up to 35s" or progress bar or seconds elapsed
- **Repro**: `docker exec bp-audit4 brewprune doctor` (takes 23-35 seconds with just dots)

---

## 5. Explanation / Detail

### [EXPLAIN] Missing package error could be more helpful
- **Severity**: UX-improvement
- **What happens**: `brewprune explain nonexistent-package` says "Error: package not found: nonexistent-package" followed by suggestions. Good error, but could offer fuzzy matching or list similar names.
- **Expected**: "Package not found: nonexistent-package. Did you mean: git, jq, gettext?"
- **Repro**: `docker exec bp-audit4 brewprune explain nonexistant-package`

### [EXPLAIN] Missing argument error is too terse
- **Severity**: UX-polish
- **What happens**: `brewprune explain` returns "Error: missing package name. Usage: brewprune explain <package>"
- **Expected**: This is fine, but could show an example: "Usage: brewprune explain <package> (e.g., brewprune explain git)"
- **Repro**: `docker exec bp-audit4 brewprune explain`

### [EXPLAIN] Color codes in explain output
- **Severity**: UX-polish
- **What happens**: Explain output contains ANSI escape codes like `[1m` for bold and `[32m` for green, which are visible in the raw output
- **Expected**: Colors should render properly in terminal. This may be a Docker exec issue, but worth noting.
- **Repro**: `docker exec bp-audit4 brewprune explain git`

### [STATS] Empty stats output is ambiguous
- **Severity**: UX-polish
- **What happens**: `brewprune stats` for packages with no usage just shows "Package: jq, Total Uses: 0, Last Used: never" etc.
- **Expected**: Consider adding "No usage recorded yet — install age: 1 day" or similar context
- **Repro**: `docker exec bp-audit4 brewprune stats --package jq`

### [STATS] --all flag shows unsorted output
- **Severity**: UX-polish
- **What happens**: `brewprune stats --all` lists all 40 packages but the order seems arbitrary (not alphabetical, not by usage, not by score)
- **Expected**: Sort by total uses (descending) or make it clear what the sort order is
- **Repro**: `docker exec bp-audit4 brewprune stats --all`

---

## 6. Diagnostics

### [DOCTOR] Exit code 1 for warnings is too strict
- **Severity**: UX-improvement
- **What happens**: `brewprune doctor` exits with code 1 when it finds warnings (PATH not active, daemon not running) even though it says "System is functional but not fully configured"
- **Expected**: Use exit code 0 for warnings, exit code 1 for critical failures only. This matters for scripting and CI/CD.
- **Repro**: `docker exec bp-audit4 sh -c 'brewprune doctor && echo "Exit code: $?"'`

### [DOCTOR] Pipeline test after stopping daemon is slow and redundant
- **Severity**: UX-improvement
- **What happens**: After killing the daemon, `brewprune doctor` reports "⚠ Daemon not running" but then still runs a 35-second pipeline test that predictably fails
- **Expected**: If daemon is not running, skip the pipeline test or make it much faster (5s timeout)
- **Repro**: `docker exec bp-audit4 sh -c 'pkill -f "brewprune watch" && sleep 1 && brewprune doctor'`

### [DOCTOR] Pipeline test failure message is too technical
- **Severity**: UX-polish
- **What happens**: Error says "no usage event recorded after 35.322s (waited 35s) — shim executed git but daemon did not write to database"
- **Expected**: Simplify: "Pipeline test failed: shim logged event but daemon didn't process it (timeout after 35s). Try: brewprune watch --daemon"
- **Repro**: `docker exec bp-audit4 brewprune doctor` (with daemon stopped)

---

## 7. Destructive / Write Operations

### [REMOVE] --safe, --medium, --risky flags vs --tier flag
- **Severity**: UX-polish
- **What happens**: Help text explains that `--safe` is equivalent to `--tier safe`, but having both options might confuse users
- **Expected**: Pick one pattern. The shortcut flags (--safe, --medium, --risky) are more intuitive than --tier.
- **Repro**: `docker exec bp-audit4 brewprune remove --help`

### [REMOVE] No-tier error could suggest dry-run workflow
- **Severity**: UX-improvement
- **What happens**: `brewprune remove` with no args says "Error: no tier specified; use --safe, --medium, or --risky (add --dry-run to preview changes first)"
- **Expected**: This is actually a good error! But could be even better: show the command they should run: "Try: brewprune remove --safe --dry-run"
- **Repro**: `docker exec bp-audit4 brewprune remove`

### [REMOVE] Dry-run output is excellent
- **Severity**: N/A (positive note)
- **What happens**: `brewprune remove --tier safe --dry-run` shows clear table, summary, and "Dry-run mode: no packages will be removed"
- **Expected**: N/A - this is done well
- **Repro**: `docker exec bp-audit4 brewprune remove --tier safe --dry-run`

### [UNDO] List empty state is helpful
- **Severity**: N/A (positive note)
- **What happens**: `brewprune undo --list` with no snapshots shows clear explanation
- **Expected**: N/A - this is done well
- **Repro**: `docker exec bp-audit4 brewprune undo --list`

### [UNDO] Missing argument shows usage
- **Severity**: UX-polish
- **What happens**: `brewprune undo` with no argument shows brief usage and suggests --list
- **Expected**: Good, but could exit with code 1 (it exits with 0, suggesting success)
- **Repro**: `docker exec bp-audit4 sh -c 'brewprune undo; echo "Exit code: $?"'`

---

## 8. Edge Cases

### [EDGE] Nonexistent database path gives misleading message
- **Severity**: UX-critical
- **What happens**: `brewprune --db /nonexistent/path.db status` shows "brewprune is not set up — run 'brewprune scan' to get started." This is misleading because the issue is the wrong path, not lack of setup.
- **Expected**: "Error: database not found at /nonexistent/path.db. Check --db path or run quickstart."
- **Repro**: `docker exec bp-audit4 brewprune --db /nonexistent/path.db status`

### [EDGE] Invalid tier error is clear
- **Severity**: N/A (positive note)
- **What happens**: `brewprune unused --tier invalid` shows "Error: invalid --tier value "invalid": must be one of: safe, medium, risky"
- **Expected**: N/A - this is done well
- **Repro**: `docker exec bp-audit4 brewprune unused --tier invalid`

### [EDGE] Missing flag argument error is clear
- **Severity**: N/A (positive note)
- **What happens**: `brewprune stats --package` (missing value) shows "Error: flag needs an argument: --package"
- **Expected**: N/A - this is done well
- **Repro**: `docker exec bp-audit4 brewprune stats --package`

---

## 9. Output Review

### Positives
- **Table alignment**: Excellent. Columns line up perfectly in all commands (unused, stats, remove --dry-run)
- **Headers/footers**: Summaries are consistently helpful (reclaimable space, counts by tier, confidence level)
- **Status symbols**: ✓ (safe), ~ (review), ⚠ (risky) are semantically clear
- **Error messages**: Generally actionable with next steps suggested

### Issues

### [OUTPUT] Confidence tip is repetitive
- **Severity**: UX-polish
- **What happens**: Every unused/stats output ends with "Confidence: MEDIUM (2 events, tracking for 0 days)" followed by "Tip: 1-2 weeks of data provides more reliable recommendations"
- **Expected**: Show this once during onboarding or when confidence is LOW. Don't repeat on every command.
- **Repro**: Any `brewprune unused` or `brewprune stats` command

### [OUTPUT] Reclaimable space summary format
- **Severity**: UX-polish
- **What happens**: Footer shows "Reclaimable: 39 MB (safe) · 180 MB (medium) · 134 MB (risky, hidden)"
- **Expected**: Consider cumulative format: "Reclaimable: 39 MB safe, 219 MB if medium included, 353 MB total"
- **Repro**: `docker exec bp-audit4 brewprune unused`

### [OUTPUT] "Last Used" column shows "just now" vs precise timestamp
- **Severity**: UX-polish
- **What happens**: Git shows "Last Used: just now" in table but explain shows precise timestamp "Last Used: 2026-02-28 08:49:35"
- **Expected**: Be consistent. Either always show relative time or always show timestamps. Or show both: "just now (2026-02-28 08:49)"
- **Repro**: Compare `brewprune unused --all` and `brewprune stats --package git`

### [OUTPUT] Data quality in status is vague
- **Severity**: UX-polish
- **What happens**: Status shows "Data quality: COLLECTING (0 of 14 days)"
- **Expected**: Good, but could explain what "14 days" means (minimum recommended) or what happens after 14 days (changes to "GOOD"?)
- **Repro**: `docker exec bp-audit4 brewprune status`

---

## Critical Path Summary

For a new user's first experience:

1. **Discovery is broken**: No --version flag, duplicate error output
2. **Quickstart conflicts**: Fails if daemon already running (database locked)
3. **PATH confusion**: Messages contradict each other about whether PATH is configured
4. **Tracking silently fails**: Shims aren't intercepting commands but no warning shown
5. **Diagnostics are slow**: 35-second pipeline test even when daemon is obviously down

## Recommended Priority Fixes

**Must-fix (UX-critical):**
1. Add --version flag
2. Fix quickstart database lock on concurrent execution
3. Fix duplicate error output
4. Detect and warn when shims aren't intercepting commands
5. Better error for nonexistent database path
6. Change help display (no args) to exit 0 instead of 1

**Should-fix (UX-improvement):**
7. Resolve PATH status message contradiction
8. Make doctor exit 0 for warnings, 1 for failures only
9. Skip or speed up pipeline test when daemon is not running
10. Clarify tier filtering behavior with --all flag
11. Shorten or paginate verbose mode output
12. Remove unimplemented --fix flag from doctor

**Nice-to-have (UX-polish):**
13. Idempotent shell config modification (don't duplicate PATH export)
14. Consistent size formatting (KB vs MB threshold)
15. Sort stats --all output
16. Add fuzzy matching for package name errors
17. Reduce repetitive confidence tip
18. Better empty state messages with context

---

## Methodology Notes

- All commands executed via `docker exec bp-audit4 <command>` as specified
- Exit codes captured using `sh -c 'command; echo "Exit code: $?"'`
- Timing observed using sleep commands and doctor pipeline tests
- Full output examined for all ~50 commands across 9 audit areas
- Acted as first-time user: noted confusion, surprises, and unclear behavior
