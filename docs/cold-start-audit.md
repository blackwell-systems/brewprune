# brewprune Cold-Start UX Audit — Round 4

**Date:** 2026-02-28
**Environment:** Docker container `bp-audit4`, brewprune installed at `/home/linuxbrew/.linuxbrew/bin/brewprune`
**Packages:** 40 Homebrew formulae (acl, bat, brotli, bzip2, ca-certificates, curl, cyrus-sasl, expat, fd, gettext, git, jq, keyutils, krb5, libedit, libevent, libgit2, libidn2, libnghttp2, libnghttp3, libngtcp2, libssh2, libunistring, libxcrypt, libxml2, lz4, ncurses, oniguruma, openldap, openssl@3, pcre2, readline, ripgrep, sqlite, tmux, utf8proc, util-linux, xz, zlib-ng-compat, zstd)

**Method:** Two independent audit agents ran 50+ commands across 9 areas. Findings merged and verified.

---

## Summary Table

| Severity        | Count |
|-----------------|-------|
| UX-critical     | 7     |
| UX-improvement  | 16    |
| UX-polish       | 15    |
| **Total**       | **38**|

---

## Area 1: Discovery

### [DISCOVERY] Missing --version flag
- **Severity:** UX-critical
- **What happens:** `brewprune --version` returns "Error: unknown flag: --version" with exit code 1
- **Expected:** Standard CLI tools support `--version` to show version information. Critical discovery feature for users checking their installed version.
- **Repro:** `docker exec bp-audit4 brewprune --version`

### [DISCOVERY] Error messages print 4 times
- **Severity:** UX-critical
- **What happens:** Every error appears 4 times (2 complete duplicates of error + help hint). Example: `brewprune blorp` prints "Error: unknown command" 4 times.
- **Expected:** Each error should appear once.
- **Root cause:** Triple error handling: Cobra prints despite `SilenceErrors: true`, `Execute()` prints at `root.go:97`, and `main()` prints at `main.go:12`
- **Repro:** `docker exec bp-audit4 brewprune blorp`

### [DISCOVERY] Help display exits with code 1
- **Severity:** UX-improvement
- **What happens:** Running `brewprune` with no arguments shows help but exits with code 1, suggesting an error occurred
- **Expected:** Displaying help should exit with code 0 (success). Exit code 1 should be reserved for actual errors. This matters for scripting and CI/CD integration.
- **Repro:** `docker exec bp-audit4 sh -c 'brewprune; echo "Exit code: $?"'`

### [DISCOVERY] Doctor --fix flag documented but not implemented
- **Severity:** UX-improvement
- **What happens:** `brewprune doctor --fix` returns "Error: unknown flag: --fix", but doctor --help mentions "--fix flag is not yet implemented"
- **Expected:** Either implement the flag or don't document it. Documenting unimplemented features creates confusion. Or show: "The --fix flag is not yet available. Run 'brewprune quickstart' to fix setup issues automatically."
- **Repro:** `docker exec bp-audit4 brewprune doctor --fix`

### [DISCOVERY] Help text for --safe shortcut flag not prominent
- **Severity:** UX-polish
- **What happens:** Main help page shows example `brewprune remove --safe` but the remove command help doesn't mention --safe prominently (it's listed lower under "Tier shortcut flags")
- **Expected:** Either use `--tier safe` in examples or make shortcut flags more prominent in help text
- **Repro:** Compare `brewprune --help` vs `brewprune remove --help`

---

## Area 2: Setup / Onboarding

### [SETUP] Quickstart fails on concurrent execution
- **Severity:** UX-critical
- **What happens:** Running `brewprune quickstart` when a daemon is already running results in "database is locked (5) (SQLITE_BUSY)" error
- **Expected:** Quickstart should detect existing daemon and either reuse it or provide clear instructions
- **Repro:** Run `quickstart` twice without stopping daemon between runs

### [SETUP] Quickstart writes PATH to .profile twice
- **Severity:** UX-critical
- **What happens:** Quickstart adds the PATH export to `/home/brewuser/.profile` twice, resulting in duplicate entries ("# brewprune shims" + export line appears twice)
- **Expected:** Idempotent config modification — should detect existing entries and not duplicate them
- **Repro:** `grep brewprune ~/.profile` shows 4 lines (2 duplicate sets)

### [SETUP] PATH status messages contradict each other
- **Severity:** UX-improvement
- **What happens:** Status and doctor both report "PATH configured (restart shell to activate)" but also warn "⚠ Shim directory not in PATH — executions won't be intercepted". These messages contradict each other.
- **Expected:** Clear distinction between "written to shell config" vs "active in current session"
- **Repro:**
```bash
docker exec bp-audit4 brewprune quickstart
docker exec bp-audit4 brewprune status
docker exec bp-audit4 brewprune doctor
```

### [SETUP] Doctor warning contradicts quickstart success
- **Severity:** UX-improvement
- **What happens:** After quickstart says setup complete and tracking verified, `brewprune doctor` shows "⚠ Shim directory not in PATH" and exits with code 1 (failure)
- **Expected:** Either doctor should exit 0 with warnings, or quickstart should acknowledge the PATH limitation
- **Repro:** `brewprune quickstart` then `brewprune doctor`

### [SETUP] Status note "events from self-test" confuses new users
- **Severity:** UX-improvement
- **What happens:** Status shows "Note: events are from setup self-test, not real shim interception. Real tracking starts when PATH is fixed and shims are in front of Homebrew."
- **Expected:** This is confusing for new users. The setup already added PATH to ~/.profile, so what's "not fixed"? Either make it actionable or don't show it. Could add: "This is normal. Once your shell is restarted, you'll see real usage data."
- **Repro:** `docker exec bp-audit4 brewprune status` (after quickstart)

### [SETUP] "brew services not supported on Linux" message unclear
- **Severity:** UX-polish
- **What happens:** Quickstart output says "brew found but using daemon mode (brew services not supported on Linux)"
- **Expected:** Either omit this (user doesn't care about implementation detail) or clarify why it matters (e.g., "using background daemon instead")
- **Repro:** `docker exec bp-audit4 brewprune quickstart`

---

## Area 3: Core Feature — Unused Package Detection

### [UNUSED] Verbose mode output is extremely long
- **Severity:** UX-improvement
- **What happens:** `brewprune unused --verbose` outputs detailed scoring for every package with full separator lines, making it hard to scan. Output is 200+ lines for 40 packages.
- **Expected:** Consider paginating, summarizing, or suggesting to pipe to less. Or limit verbose to specific tier.
- **Repro:** `docker exec bp-audit4 brewprune unused --verbose`

### [UNUSED] Inconsistent tier filtering behavior with --all
- **Severity:** UX-improvement
- **What happens:**
  - `brewprune unused` shows safe+medium (hides risky with "use --all")
  - `brewprune unused --tier risky` shows only risky tier
  - `brewprune unused --all` shows all tiers
  - The help text says "--tier shows only that specific tier regardless of --all"
- **Expected:** The interaction between --tier and --all is confusing. Pick one model: either --tier is always a filter, or --all overrides --tier.
- **Repro:** Compare outputs of various tier/all combinations

### [UNUSED] Hidden count in summary mixes two filters
- **Severity:** UX-improvement
- **What happens:** Footer says "35 packages below score threshold hidden. Risky tier also hidden (use --all to include)." But with --min-score 70, the 35 includes both score filtering and tier filtering.
- **Expected:** Separate counts for "below score threshold" vs "hidden tier" or just say "35 packages hidden (score/tier filters)"
- **Repro:** `docker exec bp-audit4 brewprune unused --min-score 70`

### [UNUSED] Empty result message too terse
- **Severity:** UX-polish
- **What happens:** When using `--tier safe --min-score 90`, output shows "No packages match the specified criteria." with no context
- **Expected:** Show what filters were active: "No packages match: tier=safe, min-score=90. Try lowering --min-score or use --all."
- **Repro:** `docker exec bp-audit4 brewprune unused --tier safe --min-score 90`

### [UNUSED] Size formatting inconsistency
- **Severity:** UX-polish
- **What happens:** Sizes shown as "5 MB", "976 KB", "1000 KB", "1004 KB" - inconsistent use of KB vs MB for values near 1 MB
- **Expected:** Convert 1000+ KB to MB for consistency
- **Repro:** `docker exec bp-audit4 brewprune unused --all`

### [UNUSED] "Uses (7d)" column header unclear
- **Severity:** UX-polish
- **What happens:** Column header "Uses (7d)" might not be immediately clear to new users (uses in last 7 days? over 7 days?)
- **Expected:** "Last 7d" or "Recent Uses" or add footnote explaining the time window
- **Repro:** `docker exec bp-audit4 brewprune unused`

---

## Area 4: Data / Tracking

### [TRACKING] Silent tracking failure — no warning when shims don't intercept
- **Severity:** UX-critical
- **What happens:** After running `git --version && jq --version && fd --version` in the container, the event count didn't increase (remained at 2). This means shims aren't working, but the user gets no feedback about this critical failure.
- **Expected:** Either status should show "⚠ shims not active - no events in last 30s" warning, or doctor should fail if events aren't being logged
- **Repro:**
```bash
docker exec bp-audit4 sh -c 'git --version'
docker exec bp-audit4 brewprune status  # event count doesn't increase
```

### [TRACKING] Status note about self-test events persists
- **Severity:** UX-improvement
- **What happens:** Even after commands are executed (git, jq, fd), status still shows "Note: events are from setup self-test, not real shim interception"
- **Expected:** This note should disappear once real usage is detected, or clarify conditions under which it will change
- **Repro:**
```bash
docker exec bp-audit4 sh -c 'git --version && jq --version && fd --version'
docker exec bp-audit4 brewprune status
```

### [TRACKING] No indication that events source is test vs real
- **Severity:** UX-improvement
- **What happens:** The git usage shown in stats/explain is from the self-test, but there's no way to distinguish this from real user-initiated usage
- **Expected:** Could tag events with source or add metadata to indicate test vs tracked usage
- **Repro:** `brewprune stats --package git` shows usage but it's from setup, not real tracking

### [TRACKING] Progress indicators lack time estimates
- **Severity:** UX-improvement
- **What happens:** Several operations (doctor pipeline test, quickstart self-test) wait up to 35 seconds with dots showing progress, but the dots appear slowly and there's no ETA
- **Expected:** Show "waiting up to 35s" or progress bar or seconds elapsed
- **Repro:** `docker exec bp-audit4 brewprune doctor` (takes 23-35 seconds with just dots)

### [TRACKING] Daemon restart message could be clearer
- **Severity:** UX-polish
- **What happens:** `brewprune watch --daemon` when daemon is already running says "Daemon already running (PID 3100). Nothing to do."
- **Expected:** Good message, but could add "use --stop to stop it first" or "use 'brewprune status' to check tracking"
- **Repro:** `docker exec bp-audit4 brewprune watch --daemon` (when already running)

### [TRACKING] Scan provides no detail on re-run
- **Severity:** UX-polish
- **What happens:** `brewprune scan` shows "✓ Database up to date (40 packages, 0 changes)" with no indication of what it checked
- **Expected:** When run with no changes, this is fine. But first run should show more detail (scanning, building deps, creating shims).
- **Repro:** `docker exec bp-audit4 brewprune scan` (after initial scan)

---

## Area 5: Explanation / Detail

### [EXPLAIN] Nonexistent package error suggests wrong action
- **Severity:** UX-improvement
- **What happens:** Error says "If you recently installed it, run 'brewprune scan'" but user is asking about a package that doesn't exist at all
- **Expected:** "Package not found: nonexistent-package. Check the name with 'brew list' or 'brew search <name>'. If you just installed it, run 'brewprune scan'."
- **Repro:** `docker exec bp-audit4 brewprune explain nonexistent-package`

### [EXPLAIN] Missing argument format inconsistent
- **Severity:** UX-polish
- **What happens:** Error says "Usage: brewprune explain <package>" but help says "brewprune explain [package]"
- **Expected:** Use consistent format - either both `<package>` or both `[package]`
- **Repro:** Compare `brewprune explain` error vs `brewprune explain --help`

### [STATS] Tip message inconsistency
- **Severity:** UX-polish
- **What happens:** Stats for packages with usage show "Tip: Run 'brewprune explain git' for removal recommendation and scoring detail." Packages with zero usage say "Tip: Run 'brewprune explain jq' for removal recommendation." (missing "and scoring detail")
- **Expected:** Consistent messaging - both should say "and scoring detail"
- **Repro:** Compare `brewprune stats --package git` vs `brewprune stats --package jq`

### [STATS] --all flag shows unsorted output
- **Severity:** UX-polish
- **What happens:** `brewprune stats --all` lists all 40 packages but the order seems arbitrary (not alphabetical, not by usage, not by score)
- **Expected:** Sort by total uses (descending) or make it clear what the sort order is
- **Repro:** `docker exec bp-audit4 brewprune stats --all`

---

## Area 6: Diagnostics

### [DOCTOR] Exit code 1 for warnings breaks scripting
- **Severity:** UX-improvement
- **What happens:** Doctor exits with code 1 when there are only warnings (no critical issues), making it hard to use in scripts
- **Expected:** Exit 0 for warnings only, exit 1 for critical issues, exit 2 for errors. This matters for scripting and CI/CD integration.
- **Repro:** `docker exec bp-audit4 sh -c 'brewprune doctor && echo "Exit code: $?"'` shows exit code 1 despite "System is functional but not fully configured"

### [DOCTOR] Pipeline test takes 17-35 seconds with minimal feedback
- **Severity:** UX-improvement
- **What happens:** After "Running pipeline test" appears, there's a 17-35 second wait with only dots appearing, no context
- **Expected:** Either show percentage/countdown, or explain: "Running pipeline test (may take up to 35s)......"
- **Repro:** `docker exec bp-audit4 brewprune doctor` - watch the timing

### [DOCTOR] Pipeline test runs even when daemon is stopped
- **Severity:** UX-improvement
- **What happens:** After killing the daemon, `brewprune doctor` reports "⚠ Daemon not running" but then still runs a 35-second pipeline test that predictably fails
- **Expected:** If daemon is not running, skip the pipeline test or make it much faster (5s timeout)
- **Repro:** `docker exec bp-audit4 sh -c 'pkill -f "brewprune watch" && sleep 1 && brewprune doctor'`

### [DOCTOR] Incorrect fix suggestion when daemon stopped
- **Severity:** UX-improvement
- **What happens:** When daemon is stopped and pipeline test fails, it says "Action: Run 'brewprune scan' to rebuild shims and restart the daemon" but that won't restart the daemon
- **Expected:** Should say "Action: Run 'brewprune watch --daemon' to restart the daemon"
- **Repro:** Kill daemon with `pkill -f "brewprune watch"` then run `brewprune doctor`

### [DOCTOR] Pipeline test failure message too technical
- **Severity:** UX-polish
- **What happens:** Error says "no usage event recorded after 35.322s (waited 35s) — shim executed git but daemon did not write to database"
- **Expected:** Simplify: "Pipeline test failed: shim logged event but daemon didn't process it (timeout after 35s). Try: brewprune watch --daemon"
- **Repro:** `docker exec bp-audit4 brewprune doctor` (with daemon stopped)

---

## Area 7: Destructive / Write Operations

### [REMOVE] No-argument error could suggest workflow
- **Severity:** UX-improvement
- **What happens:** Running `brewprune remove` with no packages or tier shows: "Error: no tier specified; use --safe, --medium, or --risky (add --dry-run to preview changes first)"
- **Expected:** This is actually good! But could show exact command: "Try: brewprune remove --safe --dry-run"
- **Repro:** `docker exec bp-audit4 brewprune remove`

### [REMOVE] --safe/--medium/--risky vs --tier confusion
- **Severity:** UX-polish
- **What happens:** Help text explains that `--safe` is equivalent to `--tier safe`, but having both options might confuse users
- **Expected:** Pick one pattern. The shortcut flags (--safe, --medium, --risky) are more intuitive than --tier.
- **Repro:** `docker exec bp-audit4 brewprune remove --help`

### [UNDO] Missing argument shows usage but exits 0
- **Severity:** UX-polish
- **What happens:** `brewprune undo` with no argument shows brief usage and suggests --list, but exits with code 0
- **Expected:** Should exit with code 1 since no action was taken (or explicitly say "no action taken" to justify exit 0)
- **Repro:** `docker exec bp-audit4 sh -c 'brewprune undo; echo "Exit code: $?"'`

---

## Area 8: Edge Cases

### [EDGE] Nonexistent database path gives misleading message
- **Severity:** UX-critical
- **What happens:** `brewprune --db /nonexistent/path.db status` shows "brewprune is not set up — run 'brewprune scan' to get started." This is misleading because the issue is the wrong path, not lack of setup.
- **Expected:** "Error: database not found at /nonexistent/path.db. Check --db path or run quickstart."
- **Repro:** `docker exec bp-audit4 brewprune --db /nonexistent/path.db status`

### [EDGE] Invalid flag error doesn't suggest correction
- **Severity:** UX-polish
- **What happens:** `brewprune --invalid-flag` shows "Error: unknown flag: --invalid-flag" with no suggestion
- **Expected:** Could suggest: "Run 'brewprune --help' to see available flags"
- **Repro:** `docker exec bp-audit4 brewprune --invalid-flag`

---

## Area 9: Output Review

### [OUTPUT] Confidence tip is repetitive
- **Severity:** UX-polish
- **What happens:** Every unused/stats output ends with "Confidence: MEDIUM (2 events, tracking for 0 days)" followed by "Tip: 1-2 weeks of data provides more reliable recommendations"
- **Expected:** Show this once during onboarding or when confidence is LOW. Don't repeat on every command.
- **Repro:** Any `brewprune unused` or `brewprune stats` command

### [OUTPUT] "Last Used" column shows relative vs absolute time
- **Severity:** UX-polish
- **What happens:** Git shows "Last Used: just now" in table but explain shows precise timestamp "Last Used: 2026-02-28 08:49:35"
- **Expected:** Be consistent. Either always show relative time or always show timestamps. Or show both: "just now (2026-02-28 08:49)"
- **Repro:** Compare `brewprune unused --all` and `brewprune stats --package git`

### [OUTPUT] Data quality description vague
- **Severity:** UX-polish
- **What happens:** Status shows "Data quality: COLLECTING (0 of 14 days)"
- **Expected:** Good, but could explain what "14 days" means (minimum recommended) or what happens after 14 days (changes to "GOOD"?)
- **Repro:** `docker exec bp-audit4 brewprune status`

---

## Positive Notes

**What works well:**
- Table alignment is excellent - columns line up perfectly across all commands
- Headers and footers provide helpful context (reclaimable space, tier counts)
- Status symbols are semantic: ✓ (safe), ~ (review), ⚠ (risky)
- Error messages generally actionable with suggested next steps
- Empty states handled gracefully with guidance
- Quickstart workflow is intuitive and well-designed
- Dry-run mode output is clear and informative
- Color usage appears semantic (green=good, yellow=caution, red=warning)
- Size formatting is human-readable (MB, KB)
- Help text is comprehensive and well-structured

---

## Priority Recommendations

### P0 — Must Fix (UX-Critical)

1. **Add --version flag** - Standard CLI feature
2. **Fix error message 4x duplication** - Remove duplicate handlers in Execute() or main()
3. **Fix quickstart PATH duplication** - Make shell config modification idempotent
4. **Fix database lock on concurrent quickstart** - Detect existing daemon
5. **Warn when shims don't intercept** - Silent tracking failure is critical
6. **Fix misleading "not set up" for wrong DB path** - Distinguish path error from setup error
7. **Fix help display exit code** - Should exit 0, not 1

### P1 — Should Fix (UX-Improvement)

8. Resolve PATH status message contradictions (configured vs active)
9. Make doctor exit 0 for warnings, 1 for failures only
10. Skip or speed up pipeline test when daemon is not running
11. Fix doctor's incorrect action suggestion (suggests scan instead of watch --daemon)
12. Improve nonexistent package error messaging
13. Clarify tier filtering behavior with --all flag
14. Shorten or paginate verbose mode output
15. Remove unimplemented --fix flag from doctor or implement it
16. Add progress indicators with time estimates for long operations
17. Distinguish self-test events from real usage events

### P2 — Nice to Have (UX-Polish)

18. Consistent size formatting (KB vs MB threshold at 1024)
19. Sort stats --all output meaningfully
20. Reduce repetitive confidence tip
21. Consistent --tier vs --safe flag documentation
22. Better empty state messages with filter context
23. Add fuzzy matching for package name errors
24. Various minor messaging improvements

---

## Environmental Notes

- Container environment makes PATH changes hard to test in real-world way
- Non-interactive shell means sourcing .profile isn't automatic
- The tool handles these limitations reasonably well with status messages
- Self-test mechanism is clever and validates the pipeline despite PATH issues

---

## Overall Assessment

Brewprune is impressively polished for a pre-1.0 tool. The CLI interface is intuitive, help text is comprehensive, and error handling is generally good. Round 3 fixes addressed many issues, but introduced a few regressions (error duplication, PATH duplication).

The most critical issues are:
- Missing --version flag
- 4x error duplication (triple error handling in code)
- Silent tracking failure (no warning when shims don't work)
- Database locking on quickstart re-run
- PATH status message contradictions

Most issues are polish-level concerns that would elevate an already solid foundation. The tool successfully guides users through a complex setup (shims, daemon, PATH modification) with clear feedback. The "quickstart" command is particularly well-designed.
