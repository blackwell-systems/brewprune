# Brewprune Cold Start UX Audit - Agent 1

**Date**: 2026-02-28
**Environment**: Docker container `bp-audit4`
**Packages**: 40 formulae (acl, bat, brotli, bzip2, ca-certificates, curl, cyrus-sasl, expat, fd, gettext, git, jq, keyutils, krb5, libedit, libevent, libgit2, libidn2, libnghttp2, libnghttp3, libngtcp2, libssh2, libunistring, libxcrypt, libxml2, lz4, ncurses, oniguruma, openldap, openssl@3, pcre2, readline, ripgrep, sqlite, tmux, utf8proc, util-linux, xz, zlib-ng-compat, zstd)

## Summary

| Severity | Count |
|----------|-------|
| UX-critical | 4 |
| UX-improvement | 11 |
| UX-polish | 8 |
| **Total** | **23** |

---

## 1. Discovery

### [DISCOVERY] Version flag causes segmentation fault
- **Severity**: UX-critical
- **What happens**: Running `brewprune --version` results in exit code 139 (segmentation fault) with no output
- **Expected**: Should display version information like "brewprune version 0.1.0" or similar
- **Repro**: `docker exec bp-audit4 brewprune --version`

### [DISCOVERY] Explain command for jq crashes with segfault
- **Severity**: UX-critical
- **What happens**: Running `brewprune explain jq` results in exit code 139 (segmentation fault) with no output
- **Expected**: Should show scoring breakdown like it does for git and openssl@3
- **Repro**: `docker exec bp-audit4 brewprune explain jq`

### [DISCOVERY] Error messages duplicated in output
- **Severity**: UX-improvement
- **What happens**: Every error message appears twice in the output (e.g., "Error: unknown command 'blorp' for 'brewprune'" appears on two separate lines)
- **Expected**: Error messages should appear once
- **Repro**: `docker exec bp-audit4 brewprune blorp`

### [DISCOVERY] Doctor --fix flag documented but not implemented
- **Severity**: UX-improvement
- **What happens**: The help text mentions `--fix` flag is not implemented, but when used it shows "Error: unknown flag: --fix" instead of a helpful message
- **Expected**: Either implement the flag or show a message like "The --fix flag is not yet available. Run 'brewprune quickstart' to fix setup issues automatically."
- **Repro**: `docker exec bp-audit4 brewprune doctor --fix`

### [DISCOVERY] Help text shows "brewprune remove --safe" but that's not in remove help
- **Severity**: UX-polish
- **What happens**: Main help page shows example `brewprune remove --safe` but the remove command help doesn't mention --safe as a shortcut (it's listed lower under "Tier shortcut flags")
- **Expected**: Either use `--tier safe` in examples or make shortcut flags more prominent
- **Repro**: Compare `brewprune --help` vs `brewprune remove --help`

---

## 2. Setup / Onboarding

### [SETUP] Quickstart writes to .profile twice
- **Severity**: UX-improvement
- **What happens**: Quickstart adds the PATH export to `/home/brewuser/.profile` twice, resulting in duplicate entries ("# brewprune shims" + export line appears twice)
- **Expected**: Should detect existing entries and not duplicate them
- **Repro**: `grep brewprune ~/.profile` shows 4 lines (2 duplicate sets)

### [SETUP] Quickstart output shows conflicting messages
- **Severity**: UX-improvement
- **What happens**: Step 2 says "Restart your shell (or source the config file) for this to take effect" but Step 4 says tracking is verified and working
- **Expected**: Either clarify that the self-test uses a workaround, or restructure messages to be consistent
- **Repro**: `docker exec bp-audit4 brewprune quickstart` - read output carefully

### [SETUP] Docker exec doesn't reflect PATH changes
- **Severity**: UX-polish
- **What happens**: After quickstart writes to .profile, running `echo $PATH` doesn't show the new path. This is correct Docker behavior but confusing for users
- **Expected**: Could add note: "Note: In Docker/non-interactive shells, PATH changes require a new login shell"
- **Repro**: `docker exec bp-audit4 sh -c 'echo $PATH'` after quickstart

### [SETUP] Doctor warning contradicts quickstart success
- **Severity**: UX-improvement
- **What happens**: After quickstart says setup complete and tracking verified, `brewprune doctor` shows "⚠ Shim directory not in PATH" and exits with code 1 (failure)
- **Expected**: Either doctor should exit 0 with warnings, or quickstart should acknowledge the PATH limitation
- **Repro**: `brewprune quickstart` then `brewprune doctor`

### [SETUP] Status message unclear for new users
- **Severity**: UX-polish
- **What happens**: Status shows "Note: events are from setup self-test, not real shim interception." This is accurate but might confuse users about whether tracking is working
- **Expected**: Could add: "This is normal. Once your shell is restarted, you'll see real usage data."
- **Repro**: `docker exec bp-audit4 brewprune status` after quickstart

---

## 3. Core Feature: Unused Package Detection

### [UNUSED] No packages message is too terse
- **Severity**: UX-polish
- **What happens**: When using `--tier safe --min-score 90`, output shows "No packages match the specified criteria." with no other context
- **Expected**: Could show what criteria were applied, e.g., "No packages found with tier=safe AND score>=90. Try lowering --min-score or use --all to see other tiers."
- **Repro**: `docker exec bp-audit4 brewprune unused --tier safe --min-score 90`

### [UNUSED] Confusing interaction between --tier and --min-score
- **Severity**: UX-improvement
- **What happens**: Using `--min-score 70` shows only safe tier packages, but the header still says "RISKY: 4 (hidden, use --all)". Unclear if risky packages are hidden because of score filter or tier filter
- **Expected**: Footer should clarify: "35 packages below score threshold hidden. Risky tier also hidden (use --all to include)."
- **Repro**: `docker exec bp-audit4 brewprune unused --min-score 70`
  - **Note**: This actually works correctly - the footer does show the clarification. Moving to polish.

### [UNUSED] Reclaimable space calculation unclear with filters
- **Severity**: UX-polish
- **What happens**: When filtering with --min-score, the "Reclaimable" footer still shows totals for all tiers including hidden ones
- **Expected**: Could show "Reclaimable (matching filters): 39 MB" and optionally "Total (all packages): 353 MB"
- **Repro**: `docker exec bp-audit4 brewprune unused --min-score 70`

---

## 4. Data / Tracking

### [TRACKING] Commands executed don't register usage
- **Severity**: UX-critical (expected in this context)
- **What happens**: Running `git --version && jq --version && fd --version` and waiting 35+ seconds doesn't increment usage events beyond the initial self-test
- **Expected**: This is correct behavior since PATH isn't configured in non-interactive shells, but demonstrates the PATH issue is real
- **Repro**: Run commands, wait, check status - shows same event count
- **Note**: This is not a bug but validates that the PATH warning is legitimate

### [TRACKING] Daemon already running message could be friendlier
- **Severity**: UX-polish
- **What happens**: Running `brewprune watch --daemon` when daemon is running shows: "Daemon already running (PID 3100). Nothing to do."
- **Expected**: Could add helpful context: "Daemon already running (PID 3100). Use 'brewprune status' to check tracking status or 'brewprune watch --stop' to stop it."
- **Repro**: `docker exec bp-audit4 brewprune watch --daemon` twice

### [TRACKING] No indication that events are from self-test vs real usage
- **Severity**: UX-improvement
- **What happens**: The git usage shown in stats/explain is from the self-test, but there's no way to distinguish this from real user-initiated usage
- **Expected**: Could tag events with source or add metadata to indicate test vs tracked usage
- **Repro**: `brewprune stats --package git` shows usage but it's from setup, not real tracking

---

## 5. Explanation / Detail

### [EXPLAIN] Nonexistent package error suggests wrong action
- **Severity**: UX-improvement
- **What happens**: Error says "If you recently installed it, run 'brewprune scan'" but user is asking about a package that doesn't exist at all
- **Expected**: Could say "Package not found. Check the name with 'brew list' or 'brew search <name>'. If you just installed it, run 'brewprune scan'."
- **Repro**: `docker exec bp-audit4 brewprune explain nonexistent-package`

### [EXPLAIN] Missing package error doesn't match help format
- **Severity**: UX-polish
- **What happens**: Error says "Usage: brewprune explain <package>" but help says "brewprune explain [package]"
- **Expected**: Use consistent format - either both `<package>` or both `[package]`
- **Repro**: Compare `brewprune explain` error vs `brewprune explain --help`

### [STATS] Package detail output could link to explain
- **Severity**: UX-polish
- **What happens**: Stats for specific package shows "Tip: Run 'brewprune explain git' for removal recommendation and scoring detail" (good!) but only shows this for packages with usage. Packages with zero usage say "Tip: Run 'brewprune explain jq' for removal recommendation."
- **Expected**: Consistent messaging - both should say "and scoring detail"
- **Repro**: Compare `brewprune stats --package git` vs `brewprune stats --package jq`

---

## 6. Diagnostics

### [DOCTOR] Exit code 1 even when system is functional
- **Severity**: UX-improvement
- **What happens**: Doctor exits with code 1 when there are only warnings (no critical issues), making it hard to use in scripts
- **Expected**: Exit 0 for warnings only, exit 1 for critical issues, exit 2 for errors
- **Repro**: `docker exec bp-audit4 sh -c 'brewprune doctor && echo "Exit code: $?"'` shows exit code 1 despite "System is functional but not fully configured"

### [DOCTOR] Pipeline test takes 17+ seconds with no progress indicator
- **Severity**: UX-improvement
- **What happens**: After "Running pipeline test" appears, there's a 17-second wait with only dots appearing, no context about what's happening
- **Expected**: Either show percentage/countdown, or explain: "Running pipeline test (may take up to 35s)......"
- **Repro**: `docker exec bp-audit4 brewprune doctor` - watch the timing

### [DOCTOR] Critical failure doesn't explain what to do clearly
- **Severity**: UX-improvement
- **What happens**: When daemon is stopped and pipeline test fails, it says "Action: Run 'brewprune scan' to rebuild shims and restart the daemon" but that won't restart the daemon
- **Expected**: Should say "Action: Run 'brewprune watch --daemon' to restart the daemon"
- **Repro**: Kill daemon with `pkill -f "brewprune watch"` then run `brewprune doctor`

---

## 7. Destructive / Write Operations

### [REMOVE] Remove with no arguments shows confusing error
- **Severity**: UX-improvement
- **What happens**: Running `brewprune remove` with no packages or tier shows: "Error: no tier specified; use --safe, --medium, or --risky (add --dry-run to preview changes first)"
- **Expected**: This is actually good! But could also mention: "Or provide package names: brewprune remove <package1> <package2>"
- **Repro**: `docker exec bp-audit4 brewprune remove`

### [UNDO] Undo with no arguments is helpful but inconsistent
- **Severity**: UX-polish
- **What happens**: Running `brewprune undo` with no arguments shows helpful usage instead of an error
- **Expected**: This is good behavior! But other commands error out. Consider making this consistent.
- **Repro**: `docker exec bp-audit4 brewprune undo`

---

## 8. Edge Cases

### [EDGE] Invalid flag error doesn't suggest correction
- **Severity**: UX-polish
- **What happens**: `brewprune --invalid-flag` shows "Error: unknown flag: --invalid-flag" with no suggestion
- **Expected**: Could suggest: "Run 'brewprune --help' to see available flags"
- **Repro**: `docker exec bp-audit4 brewprune --invalid-flag`

### [EDGE] Nonexistent database path shows gentle error
- **Severity**: UX-polish (positive note)
- **What happens**: Using `--db /nonexistent/path.db` shows friendly message: "brewprune is not set up — run 'brewprune scan' to get started."
- **Expected**: This is excellent! This is the right tone for error messages.
- **Repro**: `docker exec bp-audit4 brewprune --db /nonexistent/path.db status`

---

## 9. Output Review

### Visual Quality (Positive Notes)

**What works well:**
- Table alignment is excellent - columns line up perfectly
- Headers are clear with good use of visual separators (─ characters)
- Color usage appears semantic (though I see ANSI codes in text: `[1m` for bold, `[31m` for red)
- Score/tier indicators use appropriate symbols: ✓ for safe, ~ for review, ⚠ for risky
- Size formatting is human-readable (5 MB, 976 KB, etc.)
- Confidence footer provides helpful context
- Empty states are handled well (e.g., "No snapshots available" with helpful guidance)

**Color Usage Observed (from ANSI codes):**
- Package names: bold white (`[1m`)
- Risky scores: red (`[31m`)
- Status messages: green ✓, yellow ~, red ⚠
- Table separators: dim gray

### Terminology Consistency

**Consistent terms:**
- "Safe/Medium/Risky" tiers used consistently
- "Confidence score" terminology clear
- "Daemon" vs "watch" used correctly

**Inconsistencies noted above:**
- `<package>` vs `[package]` in help/errors
- "shortcut flags" hidden in help text

---

## Recommendations Priority

### P0 (Must Fix - Critical)
1. Fix `--version` segfault
2. Fix `brewprune explain jq` segfault
3. Fix duplicate error messages
4. Fix quickstart writing duplicate PATH entries

### P1 (Should Fix - Significant UX Issues)
5. Make doctor exit code 0 for warnings-only state
6. Fix doctor's incorrect action suggestion when daemon is stopped
7. Improve nonexistent package error messaging
8. Stop --fix flag from showing generic "unknown flag" error

### P2 (Nice to Have - Polish)
9. Add progress indication or time estimate to doctor pipeline test
10. Make tier shortcut flags more discoverable in remove help
11. Provide clearer guidance about PATH in Docker/non-interactive contexts
12. Add more context to empty/filtered results

---

## Environmental Notes

- Container environment makes PATH changes hard to test in real-world way
- Non-interactive shell means sourcing .profile isn't automatic
- The tool handles these limitations reasonably well with status messages
- Self-test mechanism is clever and validates the pipeline despite PATH issues

---

## Overall Assessment

Brewprune is impressively polished for a pre-1.0 tool. The CLI interface is intuitive, help text is comprehensive, and error handling is generally good. The two segfaults are the most critical issues. The UX improvements around messaging consistency and exit codes would significantly improve the experience for new users and script authors.

The tool successfully guides users through a complex setup (shims, daemon, PATH modification) with clear feedback. The "quickstart" command is particularly well-designed. Most issues are polish-level concerns that would elevate an already solid foundation.
