# Cold-Start UX Audit  -  Round 10

**Audit Date:** 2026-03-02
**Tool Version:** brewprune version dev (commit: unknown, built: unknown)
**Container:** brewprune-r10
**Environment:** Linux aarch64 (Ubuntu) with Homebrew (Linuxbrew)
**Auditor:** Claude Sonnet 4.6 (automated)

---

## Summary Table

| Severity       | Count |
|----------------|-------|
| UX-critical    | 3     |
| UX-improvement | 7     |
| UX-polish      | 8     |
| **Total**      | 18    |

---

## Findings by Area

---

### Area 1: Discovery & Help System

**Overall: Good.** The help system is well-structured and informative. A new user encountering brewprune for the first time gets a clear mental model from `brewprune --help`.

#### [AREA 1] Finding 1.1: `-v` as `--version` is surprising

- **Severity:** UX-polish
- **What happens:** `brewprune -v` prints the version string (`brewprune version dev ...`), same as `--version`. This is unexpected because `-v` conventionally means `--verbose` in most CLI tools.
- **Expected:** Either `-v` is `--verbose` (or an alias), or the help text explicitly warns "Note: `-v` is `--version`, not `--verbose`." The `unused` command uses `-v` as `--verbose`, creating an inconsistency within the same tool.
- **Repro:** `docker exec brewprune-r10 brewprune -v`

#### [AREA 1] Finding 1.2: No explanation of prerequisite order in help

- **Severity:** UX-polish
- **What happens:** The `--help` Quick Start shows the manual 4-step sequence but does not explain *why* each step matters or what happens if you skip one (e.g., running `unused` before `scan`).
- **Expected:** A brief note like "scan must run before other commands; watch must run before usage data accumulates."
- **Repro:** `docker exec brewprune-r10 brewprune --help`

#### [AREA 1] Finding 1.3: `brewprune` with no args shows full help (positive)

- **Severity:** (positive observation)
- **What happens:** Running bare `brewprune` outputs the full help page with exit code 0.
- **Expected behavior achieved:** New users are guided rather than hit with a cryptic error.

---

### Area 2: Setup & Onboarding

**Overall: Excellent.** Quickstart is one of the best parts of the tool. Clear step-by-step feedback, actionable warnings, and a meaningful difference from the manual path.

#### [AREA 2] Finding 2.1: Quickstart references wrong home path in PATH export example (polished correctly in quickstart output, but `scan` shows generic path)

- **Severity:** UX-polish
- **What happens:** The `brewprune scan` output shows:
  ```
  add shim directory to PATH before Homebrew:
  export PATH="/home/brewuser/.brewprune/bin":$PATH
  ```
  This is personalized correctly. However the path shown in `scan` output uses a hardcoded-looking string that changes per user, which is fine. No issue found here.

#### [AREA 2] Finding 2.2: `status` shows misleading "no events" warning immediately after daemon starts

- **Severity:** UX-improvement
- **What happens:** Immediately after starting the daemon (manual path), `status` shows:
  ```
  ⚠ Daemon running but no events logged. Shims may not be intercepting commands.
  Run 'brewprune doctor' to diagnose.
  ```
  This is technically correct but alarming to a new user who just started the daemon for the first time and hasn't run any shimmed commands yet.
- **Expected:** The warning should be suppressed or softened for the first N minutes of daemon operation, or require at least some threshold time to have passed before suggesting a problem. For example: "No events logged yet (daemon started just now  -  this is normal)."
- **Repro:** `docker exec brewprune-r10 brewprune watch --daemon` followed immediately by `docker exec brewprune-r10 brewprune status`

#### [AREA 2] Finding 2.3: Quickstart references `/home/brewuser/.profile` but audit runs as root-equivalent user in container

- **Severity:** UX-polish
- **What happens:** In the container, quickstart writes to `/home/brewuser/.profile`. The `ls -la /root/.brewprune/` command fails with "Permission denied" because the actual data is at `/home/brewuser/.brewprune/`. This is a container configuration detail, not a brewprune bug  -  the tool correctly identifies and uses the actual user's home. No issue in brewprune itself.

#### [AREA 2] Finding 2.4: `watch --daemon` start output is verbose but useful

- **Severity:** (positive observation)
- **What happens:** Starting the daemon shows PID file path, log file path, and a stop command. This is exactly what a user needs.

---

### Area 3: Core Feature  -  Unused Package Discovery

**Overall: Very good.** The table is clean, filtering is effective, sort options work correctly with footer annotations. The no-data warning banner is prominent and actionable.

#### [AREA 3] Finding 3.1: `--sort age` shows absolute install dates, not relative ages  -  header says "Installed" not "Last Used"

- **Severity:** UX-polish
- **What happens:** When using `--sort age`, the "Last Used" column header changes to "Installed" and shows absolute dates (e.g., `2026-02-28`). The sort footer says "Sorted by: install date (oldest first)." This is consistent and correct.
- **Expected:** Minor  -  consider using a relative format like "2 days ago" in the Installed column for consistency with the Last Used column when sorting by age. Currently the sort-by-age view shows inconsistent formatting between columns.
- **Repro:** `docker exec brewprune-r10 brewprune unused --sort age`

#### [AREA 3] Finding 3.2: `--tier safe --all` correctly errors but error message is redundant

- **Severity:** UX-polish
- **What happens:** `brewprune unused --tier safe --all` returns:
  ```
  Error: --all and --tier are mutually exclusive
    Use --tier safe to show only safe packages, or --all to show all tiers
  ```
  This is good. Both flag orders produce the same error, which is correct.

#### [AREA 3] Finding 3.3: `--verbose` output is overwhelming for 40 packages

- **Severity:** UX-improvement
- **What happens:** `brewprune unused --verbose` (no tier filter) renders 40 package breakdown blocks in a continuous wall of text with no pagination or summary-first view. Each block is separated only by a line of dashes.
- **Expected:** For verbose mode with many packages, either limit to a default of 10-15 and show "use --all to see more", or interleave a compact table first followed by verbose detail on demand. The current output is ~250 lines for 40 packages.
- **Repro:** `docker exec brewprune-r10 brewprune unused --verbose`

#### [AREA 3] Finding 3.4: `--casks` on Linux produces clean graceful message

- **Severity:** (positive observation)
- **What happens:** `brewprune unused --casks` correctly outputs "No casks found in the Homebrew database." with exit code 0. Clean and informative.

#### [AREA 3] Finding 3.5: Default view shows ALL tiers when no usage data (including risky)  -  documented but surprising

- **Severity:** UX-improvement
- **What happens:** The `unused --help` documents this behavior correctly: "When no --tier or --all flag is set and no usage data exists, the risky tier is shown automatically with a warning banner." However, this means a brand-new user's first run of `brewprune unused` shows openssl@3 and ncurses in a removal list, which is potentially alarming.
- **Expected:** The warning banner does mitigate this, but consider making risky packages visually very distinct (e.g., the `⚠ risky` symbol in the Status column is good, but more visual separation like a blank line between tiers would help).
- **Repro:** Fresh install + `docker exec brewprune-r10 brewprune unused`

#### [AREA 3] Finding 3.6: All packages installed on same date  -  `--sort age` produces non-deterministic order within same date

- **Severity:** UX-polish
- **What happens:** In the test environment all packages were installed on 2026-02-28, so `--sort age` (oldest first) produces a random-looking ordering within that same date. The sort is technically correct but appears arbitrary.
- **Expected:** Secondary sort by score or package name when dates are equal.
- **Repro:** `docker exec brewprune-r10 brewprune unused --sort age`

---

### Area 4: Data Collection & Tracking

**Overall: CRITICAL issue found.** The daemon starts and reads usage.log correctly (offset advances to match file size), but ZERO events are recorded in the database after the polling cycle completes. The `stats --package git` and `stats --package jq` regression from r9 is partially resolved (they no longer crash, they return data)  -  but only because the database has no usage events, not because usage events are being correctly processed.

#### [AREA 4] Finding 4.1: [CRITICAL] Daemon reads usage.log but records zero events in database

- **Severity:** UX-critical
- **What happens:** After running 5 shimmed commands (`git`, `jq`, `bat`, `fd`, `rg` via `/home/brewuser/.brewprune/bin/`), the `usage.log` file contains 5 entries:
  ```
  1772483276909243898,/home/brewuser/.brewprune/bin/git
  1772483277172292282,/home/brewuser/.brewprune/bin/jq
  1772483280279664008,/home/brewuser/.brewprune/bin/bat
  1772483280576970476,/home/brewuser/.brewprune/bin/fd
  1772483281204782243,/home/brewuser/.brewprune/bin/rg
  ```
  The `usage.offset` file correctly advances from 0 to 267 (the full file size), confirming the daemon read all entries. However, after two full 35-second polling cycles, `brewprune status` still shows `Events: 0 total`. The `stats --package git` and `stats --package jq` commands both show "Total Uses: 0" with "Frequency: never."
- **Expected:** After the daemon's polling cycle processes usage.log entries, usage events should appear in the database. `stats --package git` should show 1 use.
- **Impact:** This is the core feature of brewprune. The entire confidence scoring system is degraded to heuristics-only mode because usage events are never recorded, even when the pipeline appears to work mechanically (shims write to log, daemon reads log).
- **Repro:**
  ```
  docker exec brewprune-r10 brewprune scan
  docker exec brewprune-r10 brewprune watch --daemon
  docker exec brewprune-r10 /home/brewuser/.brewprune/bin/git --version
  docker exec brewprune-r10 /home/brewuser/.brewprune/bin/jq --version
  docker exec brewprune-r10 sleep 35
  docker exec brewprune-r10 brewprune status
  # Shows: Events: 0 total
  docker exec brewprune-r10 cat /home/brewuser/.brewprune/usage.log
  # Shows 5 entries
  docker exec brewprune-r10 cat /home/brewuser/.brewprune/usage.offset
  # Shows 267 (matches file size)
  ```
- **Note:** The watch.log only shows the daemon start line  -  no processing log entries. This suggests the daemon's log-processing loop either silently fails to resolve shim paths to package names or silently fails to write to the database.

#### [AREA 4] Finding 4.2: `watch.log` contains no processing entries  -  daemon is silent

- **Severity:** UX-improvement
- **What happens:** After processing the usage.log, `watch.log` contains only:
  ```
  2026-03-02T20:26:26Z brewprune-watch: daemon started (PID 2126)
  ```
  No entries like "processed 5 events" or "resolved git → git package." This makes debugging impossible from the user side.
- **Expected:** Daemon log should record processing activity: each cycle's event count, resolved package names, and any resolution failures.
- **Repro:** `docker exec brewprune-r10 cat /home/brewuser/.brewprune/watch.log`

#### [AREA 4] Finding 4.3: `stats --all` sorts by "most used first" but all packages show 0 uses  -  sort produces arbitrary order

- **Severity:** UX-polish
- **What happens:** With zero usage data, `stats --all` shows a table sorted "most used first" but since all packages have 0 uses, the order is database-insertion order. This looks random to the user.
- **Expected:** When all usage is zero, sort alphabetically or by package name as a secondary sort.
- **Repro:** `docker exec brewprune-r10 brewprune stats --all`

#### [AREA 4] Finding 4.4: `watch --stop` provides clear confirmation

- **Severity:** (positive observation)
- **What happens:** `brewprune watch --stop` shows a spinner then "Daemon stopped" with exit 0.

#### [AREA 4] Finding 4.5: `stats` zero-usage message is informative

- **Severity:** (positive observation)
- **What happens:** `brewprune stats` with no data shows: "No usage recorded yet (40 packages with 0 runs). Run 'brewprune watch --daemon' to start tracking." This is clear and actionable.

---

### Area 5: Package Explanation & Detail View

**Overall: Excellent.** The explain output is clear, well-structured, and consistent with `unused --verbose`. All regression checks for this area pass.

#### [AREA 5] Finding 5.1: `explain git` shows "Protected: YES" with correct phrasing

- **Severity:** (positive observation  -  regression PASS)
- **What happens:** The Protected line reads: `Protected: YES (core system dependency  -  kept even if unused)`
  This matches the expected fixed phrasing (not "part of 47 core dependencies").

#### [AREA 5] Finding 5.2: Safe-tier recommendation shows numbered two-step list

- **Severity:** (positive observation  -  regression PASS)
- **What happens:** `brewprune explain jq` shows:
  ```
  Recommendation: Safe to remove. This package scores high for removal confidence.
    1. Preview:  brewprune remove --safe --dry-run
    2. Remove:   brewprune remove --safe
  ```
  This is exactly the expected fix from r9.

#### [AREA 5] Finding 5.3: `explain openssl@3` correctly shows cap at 70 in verbose note

- **Severity:** (positive observation)
- **What happens:** The breakdown includes `Critical: YES - capped at 70 (core system dependency)` and the score is 40 (below the cap, as expected since its dependency score drags it down). The cap prevents inflated scores for core packages.

#### [AREA 5] Finding 5.4: `explain nonexistent-package` error mentions `brewprune scan` but not `brewprune undo`

- **Severity:** UX-polish
- **What happens:** Error output is:
  ```
  Error: package not found: nonexistent-package

  Check the name with 'brew list' or 'brew search nonexistent-package'.
  If you just installed it, run 'brewprune scan' to update the index.
  If you recently ran 'brewprune undo', run 'brewprune scan' to update the index.
  ```
  The `explain` command DOES mention undo. The `remove nonexistent-package` command does NOT mention undo:
  ```
  Error: package not found: nonexistent-package

  Check the name with 'brew list' or 'brew search nonexistent-package'.
  If you just installed it, run 'brewprune scan' to update the index.
  ```
- **Expected:** Both commands should have consistent error messages mentioning the undo + scan workflow.
- **Repro:** `docker exec brewprune-r10 brewprune remove nonexistent-package`

#### [AREA 5] Finding 5.5: `explain` with no args gives a clear usage error

- **Severity:** (positive observation)
- **What happens:** `brewprune explain` returns: `Error: missing package name. Usage: brewprune explain <package>` with exit code 1.

---

### Area 6: Diagnostics

**Overall: Good.** The doctor command works across all three states (healthy, degraded, blank) and provides actionable guidance. Key regression items for PATH/pipeline check pass.

#### [AREA 6] Finding 6.1: Pipeline test shows SKIPPED (not FAIL) when PATH not active  -  regression PASS

- **Severity:** (positive observation  -  regression PASS)
- **What happens:** With shims configured but PATH not sourced, `doctor` shows:
  ```
  ⚠ Pipeline test: SKIPPED
    Shims are configured but PATH is not active yet.
    Open a new terminal to activate tracking, then re-run 'brewprune doctor'.
  ```
  This does NOT count as a critical issue. The summary says "1 warning" (not "1 critical"), confirming the regression fix.

#### [AREA 6] Finding 6.2: PATH check distinguishes "configured but not sourced" from "not configured at all"  -  regression PASS

- **Severity:** (positive observation  -  regression PASS)
- **What happens:**
  - After `quickstart` or `scan` (PATH written to profile): `⚠ PATH configured (restart shell to activate)`
  - After `rm -rf .brewprune`: `✗ Shim binary not found  -  usage tracking disabled`
  - Status command shows "not yet active · PATH configured" vs "not installed · shims not installed"
  This distinction is clear and correct.

#### [AREA 6] Finding 6.3: Aliases tip appears unconditionally in doctor output

- **Severity:** UX-polish
- **What happens:** Every `doctor` run includes:
  ```
  Tip: Create ~/.config/brewprune/aliases to declare alias mappings.
       Format: one alias per line, e.g. ll=eza or g=git
       Aliases help brewprune associate your custom commands with their packages.
  ```
  This tip appears even in the blank/error state and in the healthy state, breaking the visual flow between the check results and the summary line.
- **Expected:** The aliases tip should appear only in healthy/degraded states, and should not be inserted between diagnostic check lines. Consider placing it as a footer section after the summary.
- **Repro:** Any `doctor` invocation.

#### [AREA 6] Finding 6.4: Blank-state doctor exits with code 1 ("Error: diagnostics failed")

- **Severity:** UX-improvement
- **What happens:** With no `.brewprune` directory, `doctor` exits with code 1 and prints "Error: diagnostics failed" as the last line, after the normal warning/error summaries.
- **Expected:** The terminal "Error: diagnostics failed" message is redundant  -  the critical issue count and individual check failures already communicate the problem. A new user may be confused by an "Error" message when they're just trying to check setup status. Consider exiting with a non-zero code silently or using a summary like "Action required: run 'brewprune scan' to initialize."
- **Repro:** `docker exec brewprune-r10 rm -rf /home/brewuser/.brewprune && docker exec brewprune-r10 brewprune doctor`

#### [AREA 6] Finding 6.5: Stopped-daemon doctor shows correct pipeline skip vs not-running state

- **Severity:** (positive observation)
- **What happens:** With daemon stopped (PID file removed), doctor shows:
  ```
  ⊘ Pipeline test skipped (daemon not running)
    The pipeline test requires a running daemon to record usage events
  ```
  Using the `⊘` symbol (not a warning `⚠` or error `✗`) is a nice visual distinction.

---

### Area 7: Destructive Operations (Remove & Undo)

**Overall: Mostly good, with one critical data integrity issue.** The dry-run, confirmation, snapshot, and undo flows all work correctly. However, a stale dependency graph after undo causes dangerous incorrect safe-tier classifications.

#### [AREA 7] Finding 7.1: [CRITICAL] Stale dependency graph after undo causes incorrect safe-tier classifications

- **Severity:** UX-critical
- **What happens:** After `brewprune undo` restores packages, running `brewprune scan` shows that some packages that are library dependencies (libevent, libgit2, oniguruma) appear with score 80/SAFE and "no dependents" even though they have dependents in the actual brew dependency graph. This causes a subsequent `brewprune remove --safe --yes` to attempt to remove these packages, resulting in brew uninstall failures:
  ```
  ⚠️  3 failures:
    - libevent: brew uninstall libevent failed: exit status 1 (output: Error: No such keg: ...)
    - libgit2: brew uninstall libgit2 failed: exit status 1 (output: Error: No such keg: ...)
    - oniguruma: brew uninstall oniguruma failed: exit status 1 (output: Error: No such keg: ...)
  ```
  Additionally, `brewprune remove --safe --yes` exits with code 0 even when all 3 removals failed (reports "✓ Removed 0 packages, freed 0 B" followed by a failure block).
- **Expected:** Either (a) scan correctly rebuilds the dependency graph with accurate dependency counts, or (b) if scan can't resolve dependencies, it should not classify packages as "safe" until fully validated.
- **Repro:**
  ```
  docker exec brewprune-r10 brewprune scan
  docker exec brewprune-r10 brewprune remove --safe --yes   # removes bat/fd/jq/ripgrep/tmux
  docker exec brewprune-r10 brewprune undo latest --yes     # restores them
  docker exec brewprune-r10 brewprune scan                  # shows libevent/libgit2/oniguruma as safe
  docker exec brewprune-r10 brewprune remove --safe --yes   # attempts to remove them, fails
  ```

#### [AREA 7] Finding 7.2: [CRITICAL] `remove --safe --yes` exits 0 when all removals fail

- **Severity:** UX-critical
- **What happens:** The remove command reports "✓ Removed 0 packages, freed 0 B" and exits with code 0 when all brew uninstall commands fail. The snapshot is created (ID 2), the failure block is shown, but the exit code does not reflect the failure.
- **Expected:** Exit code 1 when removals fail. The undo hint ("Undo with: brewprune undo 2") appearing after all failures is also confusing  -  there is nothing to undo.
- **Repro:** Same as Finding 7.1

#### [AREA 7] Finding 7.3: `remove --risky --dry-run` layout: warning appears mid-table

- **Severity:** UX-polish
- **What happens:** The `--risky` dry-run output shows:
  ```
  Packages to remove (risky tier):


  ⚠  30 packages skipped (locked by dependents)  -  run with --verbose to see details
  Package          Size     Score   ...
  ```
  There are two blank lines between the header and the warning, and then the warning appears between the section header and the column header row. This is visually inconsistent with other dry-run outputs which show the table immediately.
- **Expected:** Place the "packages skipped" warning after the table (as a footer note), or integrate it into the summary section.
- **Repro:** `docker exec brewprune-r10 brewprune remove --risky --dry-run`

#### [AREA 7] Finding 7.4: Post-undo warning correctly mentions all four commands  -  regression PASS

- **Severity:** (positive observation  -  regression PASS)
- **What happens:** After `undo latest --yes`:
  ```
  ⚠  Run 'brewprune scan' to update the package database.
     Commands that need a fresh scan: remove, unused, explain, stats --package
  ```
  All four commands are listed. The r9 regression is fixed.

#### [AREA 7] Finding 7.5: Undo output is clean  -  no redundant progress bar  -  regression PASS

- **Severity:** (positive observation  -  regression PASS)
- **What happens:** Undo shows a spinner plus per-package "Restored X" lines. There is no redundant 0%→100% progress bar.

#### [AREA 7] Finding 7.6: `remove nonexistent-package` lacks undo mention in error  -  regression PARTIAL FAIL

- **Severity:** UX-polish
- **What happens:** `remove nonexistent-package` shows:
  ```
  Error: package not found: nonexistent-package

  Check the name with 'brew list' or 'brew search nonexistent-package'.
  If you just installed it, run 'brewprune scan' to update the index.
  ```
  The r9 regression check requires this to mention undo. `explain nonexistent-package` DOES include the undo line. `remove` does not.
- **Expected:** The error for `remove nonexistent-package` should include: "If you recently ran 'brewprune undo', run 'brewprune scan' to update the index."
- **Repro:** `docker exec brewprune-r10 brewprune remove nonexistent-package`

#### [AREA 7] Finding 7.7: `stats --package jq` after undo (before scan) fails with "package jq not found"

- **Severity:** UX-improvement
- **What happens:** After `undo latest --yes` restores jq but before running `brewprune scan`, `stats --package jq` fails:
  ```
  Error: failed to get stats for jq: failed to get package: package jq not found
  ```
  The post-undo warning message does mention "stats --package" in the list of commands needing a scan, so this is documented  -  but the error message itself doesn't remind the user to scan.
- **Expected:** The error message for package-not-found in `stats --package` should say: "Package 'jq' not found. If you recently ran 'brewprune undo', run 'brewprune scan' to update the index."
- **Repro:** `docker exec brewprune-r10 brewprune undo latest --yes` → `docker exec brewprune-r10 brewprune stats --package jq`

#### [AREA 7] Finding 7.8: `explain git` after undo (before scan) works correctly

- **Severity:** (positive observation  -  regression PASS)
- **What happens:** `brewprune explain git` after undo (no intermediate scan) works without crashing and shows correct data. The r9 regression about crashing is fixed.

---

### Area 8: Edge Cases & Error Handling

**Overall: Excellent.** Validation is thorough and messages are specific. This area saw major improvements.

#### [AREA 8] Finding 8.1: Unknown subcommands list valid commands  -  very helpful

- **Severity:** (positive observation)
- **What happens:**
  ```
  Error: unknown command "blorp" for "brewprune"
  Valid commands: scan, unused, remove, undo, status, stats, explain, doctor, quickstart, watch, completion
  Run 'brewprune --help' for usage.
  ```
  This is excellent. The valid commands list is exactly what a new user needs.

#### [AREA 8] Finding 8.2: `unused --tier invalid` shows valid tier values

- **Severity:** (positive observation)
- **What happens:** `Error: invalid --tier value "invalid": must be one of: safe, medium, risky`

#### [AREA 8] Finding 8.3: `stats --days -1` and `stats --days abc` both give correct validation

- **Severity:** (positive observation)
- **What happens:** Both produce `Error: --days must be a positive integer`

#### [AREA 8] Finding 8.4: `remove --safe --medium --risky` gives correct multi-flag error

- **Severity:** (positive observation)
- **What happens:** `Error: only one tier flag can be specified at a time (got --safe, --medium, and --risky)`

#### [AREA 8] Finding 8.5: `watch --daemon --stop` conflict detected correctly

- **Severity:** (positive observation)
- **What happens:** `Error: --daemon and --stop are mutually exclusive: use one or the other`

#### [AREA 8] Finding 8.6: `unused` with no DB still has "failed to list packages:" chain prefix

- **Severity:** UX-improvement
- **What happens:** `brewprune unused` with no DB shows:
  ```
  Error: failed to list packages: database not initialized  -  run 'brewprune scan' to create the database
  ```
  The "failed to list packages:" prefix is a code-level error chain leaking into the user message.
- **Expected:** `Error: database not initialized  -  run 'brewprune scan' to create the database`
  (Note: `brewprune remove --safe` with no DB correctly shows just the terminal message without the prefix chain  -  so this is an inconsistency between commands.)
- **Repro:** `docker exec brewprune-r10 rm -rf /home/brewuser/.brewprune && docker exec brewprune-r10 brewprune unused`

#### [AREA 8] Finding 8.7: `remove` with no args gives clear usage error with `--dry-run` suggestion

- **Severity:** (positive observation)
- **What happens:**
  ```
  Error: no tier specified

  Try:
    brewprune remove --safe --dry-run

  Or use --medium or --risky for more aggressive removal
  ```
  This is excellent  -  the suggestion starts with the safest option (`--safe`) and `--dry-run`.

---

### Area 9: Output Quality & Visual Design

**Overall: Consistent and clean.** The visual design is coherent. Tables are well-aligned. The tier system uses consistent symbols (✓ safe, ~ medium, ⚠ risky). Several regression items pass.

#### [AREA 9] Finding 9.1: `status` correctly shows "not yet active"  -  regression PASS

- **Severity:** (positive observation  -  regression PASS)
- **What happens:** `status` shows: `Shims:        not yet active · PATH configured (restart shell to activate)`
  Not "inactive · 0 commands" as the r9 regression described.

#### [AREA 9] Finding 9.2: `status` shows "shims" terminology  -  regression PASS

- **Severity:** (positive observation  -  regression PASS)
- **What happens:** The status line says "Shims:" (not "Commands:"). When shims are not installed it says "not installed · shims not installed."

#### [AREA 9] Finding 9.3: Terminology is consistent across commands

- **Severity:** (positive observation)
- **What happens:** "daemon" is used consistently (not "service" or "background process"). "snapshot" is consistent (not "backup"). "tier" is consistent (not "level" or "category"). "score" is used consistently (not "confidence score").

#### [AREA 9] Finding 9.4: Color documentation  -  no color observed in docker exec output

- **Severity:** UX-polish
- **What happens:** The audit spec asks to evaluate color usage. In `docker exec` output piped through bash, ANSI colors are stripped. It is not possible to verify color correctness from this audit context. Based on the symbols used (`✓` for safe, `~` for medium, `⚠` for risky), the non-color fallback is well-designed. A future audit in an interactive terminal would be needed to verify actual color behavior.

#### [AREA 9] Finding 9.5: Remove output shows progress bar ("0%→100%")  -  questionable for small sets

- **Severity:** UX-polish
- **What happens:** `remove --safe --yes` shows: `[=======================================>] 100% Removing packages`. For 5 packages, this progress bar is essentially instantaneous and adds little value. It also shows 100% even when all removals fail (see Finding 7.2).
- **Expected:** For small sets (< 10 packages), a simple "Removing bat... Removing fd..." list would be more informative than a progress bar that completes in <1 second.
- **Repro:** `docker exec brewprune-r10 brewprune remove --safe --yes`

#### [AREA 9] Finding 9.6: `scan` output table has no header row separating column headers from data

- **Severity:** UX-polish
- **What happens:** The `scan` output table shows:
  ```
  Package              Size     Installed     Last Used
  ────────────────────────────────────────────────────────────
  acl                  1 MB     2 days ago    never
  ```
  The separator line is under the header, which is fine. However the column spacing is inconsistent  -  "Package" column is 20 chars wide but some package names (like "zlib-ng-compat") fit, while header names are left-padded differently from the unused table. This is a minor alignment concern.

#### [AREA 9] Finding 9.7: `stats --all` table lacks "Sorted by:" footer annotation

- **Severity:** UX-polish
- **What happens:** `stats --all` output header says "Sorted by: most used first" at the top, but uses a top-placement style while `unused` uses a bottom footer "Sorted by:" line. Inconsistent placement of sort annotation.
- **Expected:** Consistent placement of the sort annotation (either always top or always bottom) across all table outputs.
- **Repro:** `docker exec brewprune-r10 brewprune stats --all`

#### [AREA 9] Finding 9.8: `remove` partial failure exits 0 with checkmark  -  misleading visual

- **Severity:** UX-critical (same as Finding 7.2, visual aspect)
- **What happens:** After 3 brew uninstall failures:
  ```
  ✓ Removed 0 packages, freed 0 B

  ⚠️  3 failures:
  ```
  The `✓` checkmark followed by "Removed 0 packages" directly above a failure block is a contradictory signal. The checkmark implies success.
- **Expected:** When all removals fail, use `✗` or similar failure symbol. "✓ Removed 0 packages" is never meaningful  -  it should say "✗ Removal failed" or at minimum not use a success symbol.

---

## Regression Verification

| Regression Check | Status | Observed Output |
|---|---|---|
| `stats --package git` and `stats --package jq` now show usage events (r9: git/jq shim resolution was broken) | **PARTIAL PASS** | `stats --package git` and `stats --package jq` no longer crash and return data correctly after scan. However, 0 usage events are recorded in the database despite shims being invoked, so "usage events" never appear. The crash regression is fixed; the functional regression (events not flowing) is a new critical issue in r10. |
| Pipeline test shows "SKIPPED" (not "FAIL") when shims configured but PATH not active | **PASS** | `⚠ Pipeline test: SKIPPED` with "Shims are configured but PATH is not active yet." Not counted as critical issue. |
| PATH active/inactive check distinguishes "configured but not sourced" from "not configured at all" | **PASS** | "PATH configured (restart shell to activate)" vs "shims not installed (run 'brewprune scan' to build)" |
| Protected line says "core system dependency  -  kept even if unused" (not "part of 47 core dependencies") | **PASS** | `Protected: YES (core system dependency  -  kept even if unused)` |
| Safe-tier recommendation shows numbered two-step list | **PASS** | `1. Preview: brewprune remove --safe --dry-run` / `2. Remove: brewprune remove --safe` |
| `remove nonexistent-package` shows multi-line error with "brew list" and "brewprune scan" suggestions | **PARTIAL PASS** | Multi-line error with "brew list", "brew search", and "brewprune scan" suggestions shown. Does NOT include "brewprune undo" mention (unlike `explain nonexistent-package` which does include it). |
| Post-undo warning mentions ALL of: remove, unused, explain, stats --package | **PASS** | `Commands that need a fresh scan: remove, unused, explain, stats --package` |
| Undo output is spinner + per-package "Restored X" lines (no redundant 0%→100% progress bar) | **PASS** | Spinner + individual "Restored bat", "Restored fd", etc. No progress bar. |
| After `undo latest --yes`, `explain git` exits gracefully suggesting `brewprune scan` | **PASS** | `explain git` after undo (no scan) works correctly, showing data without crash. |
| `status` shows "not yet active" (not "inactive · 0 commands") when shims configured but PATH not sourced | **PASS** | `Shims: not yet active · PATH configured (restart shell to activate)` |
| `status` shows "N shims" (not "N commands") | **PASS** | Shims terminology used throughout. |
| When no DB exists, `stats` and `remove` show only terminal error message without chain prefix | **PARTIAL PASS** | `remove --safe` correctly shows `Error: database not initialized  -  run 'brewprune scan'`. `unused` still shows `Error: failed to list packages: database not initialized...` (chain prefix present). `stats` correctly shows terminal message only. |

---

## Positive Observations

1. **Error messages are excellent.** Unknown subcommands, invalid enum values, conflicting flags  -  all handled with specific, actionable messages. A significant improvement over early rounds.

2. **Quickstart is a standout feature.** Four clear steps with per-step feedback, a meaningful self-test (even if the self-test passes when the pipeline is broken, the UX flow is excellent), and a clear post-setup state description.

3. **`doctor` with PATH-not-yet-active shows SKIPPED pipeline test.** The distinction between "can't test" and "test failed" is important and correctly implemented.

4. **`undo` flow is clean.** Snapshot creation, restoration, per-package feedback, and the post-undo warning about needing a scan are all well-executed.

5. **`remove` with no tier flag gives `--dry-run` suggestion first.** "Try: `brewprune remove --safe --dry-run`" is exactly the right default recommendation.

6. **Tier summary header in `unused` output.** "SAFE: 5 packages (39 MB) · MEDIUM: 32 (248 MB) · RISKY: 3 (66 MB)" provides excellent at-a-glance context before the table.

7. **`--casks` on Linux handles gracefully.** Clean "No casks found" message rather than an error.

8. **`remove --safe --dry-run` is clearly labeled.** "Dry-run mode: no packages will be removed." is unambiguous.

---

## Recommendations Summary

### Priority 1: Fix the usage event pipeline (blocking)

The core value proposition of brewprune is broken in r10. Shims write to usage.log correctly. The daemon reads usage.log (offset advances correctly). But zero events are recorded in the database. This means every user who installs brewprune and uses it correctly will see "LOW CONFIDENCE" heuristics forever.

Investigation path: check the daemon's shim-path-to-package resolution logic. The usage.log entries contain full paths (`/home/brewuser/.brewprune/bin/git`). The daemon needs to map these to package names. A likely regression: the mapping table (shim binary path → formula name) may not be populated, or the database write in the polling loop may be failing silently.

### Priority 2: Fix `remove` exit code on partial/total failure

`remove` should exit non-zero when any brew uninstall fails. The `✓` checkmark with "Removed 0 packages" following a total failure is misleading and potentially dangerous.

### Priority 3: Fix stale dependency graph after undo → scan

After undo + scan, some packages lose their dependency relationships and appear as safe-tier when they are not. The scan should fully rebuild the dependency graph from `brew deps --tree` output for all installed packages.

### Priority 4: Fix `unused` error chain prefix

`brewprune unused` with no database should show `Error: database not initialized  -  run 'brewprune scan'` without the `failed to list packages:` prefix. One-line fix to strip the internal error wrapping.

### Priority 5: Daemon logging

Add per-cycle log entries to watch.log: "processed N events, resolved M packages, skipped K." This would have made the r10 pipeline regression immediately visible.

### Priority 6: `remove nonexistent-package` should mention undo (parity with `explain`)

Minor consistency fix: add the "If you recently ran 'brewprune undo', run 'brewprune scan'" line to the `remove` package-not-found error.

### Priority 7: `status` no-events warning timing

Suppress or soften the "Daemon running but no events logged" warning for the first few minutes of daemon operation, or until at least one shimmed command has been run. Currently it fires immediately on `brewprune status` after daemon start.

### Pattern Summary

The r10 round resolved most of the r9 visual and UX regressions successfully. The remaining issues cluster around two themes:

1. **Silent failures in the data pipeline** (usage events not recorded, brew uninstall failures masked with exit 0). These are hard to discover without intentional testing.
2. **State consistency after destructive operations** (stale dependency data after undo, stats --package failing after undo). These suggest the database update logic needs to be more comprehensive.
