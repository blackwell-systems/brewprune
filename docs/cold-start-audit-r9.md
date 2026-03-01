# Cold-Start UX Audit — Round 9

**Audit Date:** 2026-03-01
**Tool Version:** brewprune version dev (commit: unknown, built: unknown)
**Container:** brewprune-r9
**Environment:** Linux aarch64 (Ubuntu) with Homebrew (Linuxbrew)
**Auditor:** Claude agent (claude-sonnet-4-6)

---

## Summary Table

| Severity       | Count |
|----------------|-------|
| UX-critical    | 3     |
| UX-improvement | 8     |
| UX-polish      | 7     |
| **Total**      | **18** |

---

## Findings by Area

---

### Area 1: Discovery & Help System

#### [AREA 1] `-v` as version flag may surprise users expecting verbose

- **Severity:** UX-polish
- **What happens:** `brewprune -v` prints version (`brewprune version dev (commit: unknown, built: unknown)`). The flag is documented as "show version information" in the flags list.
- **Expected:** `-v` conventionally means `--verbose` in most CLI tools (git, brew, curl, etc.). New users who type `brewprune unused -v` expecting verbose output will get the version string instead. This is already documented in the help, but the choice is still counterintuitive.
- **Repro:** `docker exec brewprune-r9 brewprune -v`
- **Note:** The `unused` subcommand does accept `-v` for verbose — but it lives under the subcommand, not globally. The root `-v` maps to version, which is the source of potential confusion.

---

#### [AREA 1] `--db` global flag has no meaningful path expansion hint

- **Severity:** UX-polish
- **What happens:** The global flag `--db string` shows `(default: ~/.brewprune/brewprune.db)` in help. This is good, but no documentation exists about when you'd need to override this.
- **Expected:** A brief note like "(advanced: override database location)" would help users understand this is an expert option, not required for normal use.
- **Repro:** `docker exec brewprune-r9 brewprune --help`

---

#### [AREA 1] `quickstart --help` mentions "30 seconds" for self-test but this detail is absent from `quickstart` live output

- **Severity:** UX-polish
- **What happens:** `quickstart --help` says "The self-test takes approximately 30 seconds." During the actual quickstart run, Step 4 runs the pipeline test without mentioning its expected duration upfront. Users see a spinner with no time estimate.
- **Expected:** Step 4/4 header should say something like "Running self-test (takes ~30 seconds)..." before the spinner starts.
- **Repro:** `docker exec brewprune-r9 brewprune quickstart --help` vs `docker exec brewprune-r9 brewprune quickstart`

---

### Area 2: Setup & Onboarding

#### [AREA 2] Quickstart PATH warning appears after "Setup complete" — creates confusing message ordering

- **Severity:** UX-improvement
- **What happens:** The quickstart output shows "Setup complete — one step remains:" followed by a "TRACKING IS NOT ACTIVE YET" warning block. The success message and the warning appear to contradict each other.

  Exact output:
  ```
  Setup complete — one step remains:
  ...
  ⚠  TRACKING IS NOT ACTIVE YET
     Your shell has not loaded the new PATH. Commands you run now
     will NOT be tracked by brewprune.
  ```

- **Expected:** Either (a) the "Setup complete" message should not appear before the critical PATH warning, or (b) the warning should be integrated before the completion message so users understand setup is conditional. A cleaner framing: "Almost done — one critical step remaining:" followed by the PATH warning.
- **Repro:** `docker exec brewprune-r9 brewprune quickstart`

---

#### [AREA 2] `status` shows "PATH configured (restart shell to activate)" for Shims but no context for what this means

- **Severity:** UX-improvement
- **What happens:** Status output:
  ```
  Shims:        inactive · 0 commands · PATH configured (restart shell to activate)
  ```
  A new user sees "Shims: inactive" which sounds like a problem, alongside "PATH configured" which sounds like it's fine.
- **Expected:** The shim status line should disambiguate: "Shims: not yet active (PATH added to profile — restart shell to enable)". The word "inactive" signals failure; "not yet active" signals pending.
- **Repro:** `docker exec brewprune-r9 brewprune status`

---

#### [AREA 2] Manual path `doctor` pipeline test failure message is actionable but the actual failure cause is structural (docker exec PATH)

- **Severity:** UX-improvement
- **What happens:** When running `brewprune doctor` via `docker exec`, the pipeline test always fails:
  ```
  ✗ Pipeline test: fail (35.372s)
    no usage event recorded after 35.367s (waited 35s) — shim executed git but daemon did not write to database
    Action: Shims not in active PATH — run: source ~/.profile (or restart your shell)
  ```
  The action is correct but the test takes 35 seconds before failing. This 35-second wait with a blank spinner and no progress updates is jarring.
- **Expected:** The spinning dots with no time estimate during "Running pipeline test (~30s)......" feel like a hang to a new user. The approximate time is shown in the text but not while waiting. Consider periodic progress messages like "Waiting for daemon... (15s elapsed)".
- **Repro:** `docker exec brewprune-r9 brewprune doctor` (with PATH not activated)

---

#### [AREA 2] Manual path `scan` output includes the package table but quickstart path suppresses it (via `--quiet`)

- **Severity:** UX-polish
- **What happens:** Running `brewprune scan` directly shows a full 40-row package table. Running `brewprune quickstart` (which calls scan internally) shows a compact "Scan complete: 40 packages, 352 MB". The detail level differs between the two entry points.
- **Expected:** This is arguably correct — quickstart is meant to be compact. But a user who runs manual `scan` to understand what's in their system gets a wall of "never / never / never" that adds no actionable information on first run.
- **Repro:** `docker exec brewprune-r9 brewprune scan`

---

### Area 3: Core Feature — Unused Package Discovery

#### [AREA 3] `--sort size` produces no "Sorted by:" footer annotation, but `--sort age` does

- **Severity:** UX-polish
- **What happens:** When sorting by age, the footer shows "Sorted by: install date (oldest first)". When sorting by size or score, no such annotation appears. This inconsistency makes it unclear whether the sort was applied.
- **Expected:** All sort modes should show a "Sorted by: X" footer. For size: "Sorted by: size (largest first)". For score: "Sorted by: score (highest first)".
- **Repro:** `docker exec brewprune-r9 brewprune unused --sort size` (no footer); `docker exec brewprune-r9 brewprune unused --sort age` (shows footer)

---

#### [AREA 3] Score of 80/100 ("safe") for bat/fd/jq etc. feels misleading without usage data — all packages installed "today" have identical scores

- **Severity:** UX-improvement
- **What happens:** Without usage data, five leaf packages (bat, fd, jq, ripgrep, tmux) all score exactly 80/100 and are all labeled safe. The verbose breakdown shows: Usage: 40/40, Dependencies: 30/30, Age: 0/20, Type: 10/10. Every leaf package installed today gets 80 regardless of anything the user knows about them.
- **Expected:** The warning banner correctly notes "LOW CONFIDENCE without usage tracking." However, the score of 80 signals strong confidence for removal, which contradicts the warning. Consider capping heuristic-only scores at a lower max (e.g., 70) when no usage data exists, to prevent false confidence.
- **Repro:** `docker exec brewprune-r9 brewprune unused --tier safe` (immediately after fresh scan)

---

#### [AREA 3] Warning banner shows even when `--tier safe` is explicitly passed

- **Severity:** UX-polish
- **What happens:** Every `unused` invocation — including `--tier safe` — shows the full warning banner about no usage data. When a user already knows data is heuristic-only and is explicitly filtering to safe packages, the repeated warning adds noise.
- **Expected:** The warning is appropriate on default (`brewprune unused`) but could be condensed to a single line when a specific tier is explicitly requested: "Note: heuristic scores only (no usage data recorded yet)."
- **Repro:** `docker exec brewprune-r9 brewprune unused --tier safe`

---

#### [AREA 3] `--casks` on Linux/Linuxbrew returns exit 0 with a message but the message appears after the warning banner

- **Severity:** UX-polish
- **What happens:**
  ```
  ⚠ WARNING: No usage data available
  [... 8 lines of warning ...]
  No casks found in the Homebrew database.
  Cask tracking requires cask packages to be installed (brew install --cask <name>).
  ```
  The "no casks" message is buried after the full warning banner.
- **Expected:** When `--casks` is passed and no casks exist, skip the warning banner entirely and just show the cask-specific message. The warning is irrelevant when there's nothing to show.
- **Repro:** `docker exec brewprune-r9 brewprune unused --casks`

---

#### [AREA 3] `--medium` and `--risky` dry-run show "N packages skipped (locked by dependents)" before the table header

- **Severity:** UX-improvement
- **What happens:** `remove --medium --dry-run` output:
  ```
  Packages to remove (medium tier):

  ⚠  31 packages skipped (locked by dependents) — run with --verbose to see details
  Package          Size     Score   ...
  ```
  The skip warning appears between the title and the table, breaking up visual flow.
- **Expected:** Move the skip count to the Summary section at the bottom, or to a footer below the table. The table should immediately follow its header.
- **Repro:** `docker exec brewprune-r9 brewprune remove --medium --dry-run`

---

### Area 4: Data Collection & Tracking

#### [AREA 4] `brewprune stats --package jq` and `brewprune explain git` both crashed (exit 139) when called immediately after removing packages and restoring via `undo`

- **Severity:** UX-critical
- **What happens:** After running `brewprune remove --safe --yes` (which removes bat, fd, jq, ripgrep, tmux) and then `brewprune undo latest --yes` (which restores them), calling `brewprune stats --package jq` and `brewprune explain git` both returned exit code 139 (segmentation fault) with no output. Commands returned successfully after a subsequent `brewprune scan`.

  The undo output itself advises: "Run 'brewprune scan' to update the package database before running 'brewprune remove'." — but does not warn that `explain` and `stats --package` also require a fresh scan.
- **Expected:** Either (a) these commands should handle the post-undo database inconsistency gracefully (not crash), or (b) the undo output warning should be expanded to cover all database-dependent commands.
- **Repro:** `docker exec brewprune-r9 brewprune remove --safe --yes` → `docker exec brewprune-r9 brewprune undo latest --yes` → `docker exec brewprune-r9 brewprune explain git` (exit 139)

---

#### [AREA 4] `status` shows "0 commands" for Shims even when shim invocations have been logged in usage.log

- **Severity:** UX-improvement
- **What happens:** After invoking 5 packages via shim paths (`/home/brewuser/.brewprune/bin/git`, etc.) and waiting for the 35-second daemon cycle, `brewprune status` shows:
  ```
  Shims:        inactive · 0 commands · PATH configured (restart shell to activate)
  Events:       3 total · 3 in last 24h
  ```
  Usage events are recorded (3 total) but the Shims line still shows "0 commands."
- **Expected:** The "0 commands" count should reflect shim invocations that were recorded — or the label should clarify what it's counting (e.g., "shims in PATH" vs "shims invoked"). The discrepancy between "0 commands" and "3 events" is confusing.
- **Repro:** Run 5 shims via full path → wait 35s → `docker exec brewprune-r9 brewprune status`

---

#### [AREA 4] `stats` shows only 3 of 5 shim-invoked packages after waiting for polling cycle

- **Severity:** UX-improvement
- **What happens:** Five packages were invoked via shims: git, jq, bat, fd, rg. After the 35-second polling cycle, `brewprune stats` shows only 3 (bat, fd, ripgrep). The usage.log shows all 5 entries:
  ```
  1772348440151702341,/home/brewuser/.brewprune/bin/git
  1772348512826581659,/home/brewuser/.brewprune/bin/git
  1772348513085009162,/home/brewuser/.brewprune/bin/jq
  1772348516006690408,/home/brewuser/.brewprune/bin/bat
  1772348516670124237,/home/brewuser/.brewprune/bin/fd
  1772348516883371499,/home/brewuser/.brewprune/bin/rg
  ```
  git and jq are missing from stats output despite being in usage.log.
- **Expected:** All 5 packages invoked via shims should appear in stats after the daemon's polling cycle. If git and jq fail to resolve from shim path to package, the failure should be logged and visible.
- **Note:** This may be a shim→package resolution issue specific to the container state.
- **Repro:** Run 5 shims, wait 35s, `docker exec brewprune-r9 brewprune stats`

---

### Area 5: Package Explanation & Detail View

#### [AREA 5] `explain bat` score changed from 80 (safe) to 40 (risky) after shim usage was recorded — score correctly reflects usage, but visual discrepancy vs `unused` table is jarring

- **Severity:** UX-improvement
- **What happens:** Before any shim invocations: `unused` table shows bat as 80/100 safe. After bat is invoked via shim and usage is recorded: `explain bat` shows Score: 40 (RISKY) with "Usage: 0/40 pts - used today (actively used — penalizes removal confidence)". The `unused` table at the same time still showed bat as 80/100 safe (this inconsistency appears immediately after usage is first recorded, before `unused` reflects the update).
- **Expected:** This score change is correct behavior — the score should drop when the package is used. However, there's no warning in `unused` output to indicate that scores may be stale relative to very recent usage. A note like "(scores update on next polling cycle)" would help.
- **Repro:** Run bat via shim → wait 35s → `brewprune explain bat` vs `brewprune unused`

---

#### [AREA 5] `explain` output for safe packages doesn't mention `brewprune remove --safe` as the direct next action — only the dry-run variant

- **Severity:** UX-polish
- **What happens:** `brewprune explain jq` ends with:
  ```
  Recommendation: Safe to remove. This package scores high for removal confidence.
  Run 'brewprune remove --safe --dry-run' to preview, then without --dry-run to remove all safe-tier packages.
  ```
  This is helpful, but the two-step "dry-run first, then without" phrasing on a single line is hard to parse.
- **Expected:** Show the two steps as a numbered list:
  ```
  1. Preview: brewprune remove --safe --dry-run
  2. Remove:  brewprune remove --safe
  ```
- **Repro:** `docker exec brewprune-r9 brewprune explain jq`

---

#### [AREA 5] `explain` for a critical package (openssl@3) shows "Protected: YES (part of 47 core dependencies)" — the number 47 is unexplained

- **Severity:** UX-polish
- **What happens:** `explain openssl@3` output:
  ```
  Protected: YES (part of 47 core dependencies)
  ```
  The number 47 appears without context. A new user doesn't know what "47 core dependencies" means or why this number matters.
- **Expected:** Either remove the count or expand it: "Protected: YES (brewprune considers this a core system dependency, along with 46 others)" or simply "Protected: YES (core system dependency — kept even if unused)".
- **Repro:** `docker exec brewprune-r9 brewprune explain openssl@3`

---

### Area 6: Diagnostics

#### [AREA 6] `doctor` always fails pipeline test in docker exec context — the failure message is accurate but the check is always-failing in this environment

- **Severity:** UX-critical
- **What happens:** In the docker exec environment, the pipeline test in `doctor` always fails with exit 1 because the shims aren't in the docker exec PATH. This happens even in the "healthy state" where the daemon is running and usage events are recorded. The output:
  ```
  Found 1 critical issue(s) and 1 warning(s).
  Error: diagnostics failed
  ```
  The exit code is 1 (failure) even when the system is actually functional (3 usage events were recorded manually via shim paths).
- **Expected:** This is a real limitation of the audit environment (docker exec doesn't inherit the user's PATH), but for real users after `source ~/.profile`, the pipeline test should pass. The issue is that `doctor` labels the system as broken when shims are in the profile but not yet in the active session. A better distinction: "Pipeline test: SKIPPED (shims not in active PATH for this session — restart shell first)" vs outright failure.
- **Repro:** Any `docker exec brewprune-r9 brewprune doctor` call where `source ~/.profile` has not been run.

---

#### [AREA 6] `doctor` in blank state (no `~/.brewprune`) skips the "no usage events" check entirely

- **Severity:** UX-polish
- **What happens:** When `~/.brewprune` doesn't exist at all, `doctor` output:
  ```
  ✗ Database not found at: /home/brewuser/.brewprune/brewprune.db
    Action: Run 'brewprune scan' to create database
  ⚠ Daemon not running (no PID file)
    Action: Run 'brewprune watch --daemon'
  ✗ Shim binary not found — usage tracking disabled
    Action: Run 'brewprune scan' to build it

  Found 2 critical issue(s) and 1 warning(s).
  ```
  The usage events check is absent (correctly skipped as database doesn't exist). But the count "2 critical, 1 warning" mentions 3 checks with 3 shown — this is actually clean. No issue found here beyond the PATH check still being absent.
- **Expected:** Consider adding a PATH check: "✗ ~/.brewprune/bin not found in PATH — run brewprune scan, then add to PATH". This check is absent from all three doctor states (healthy, degraded, blank).
- **Repro:** `docker exec brewprune-r9 rm -rf /home/brewuser/.brewprune && docker exec brewprune-r9 brewprune doctor`

---

#### [AREA 6] `doctor` has no PATH-in-PATH check — only a "PATH configured in profile" check

- **Severity:** UX-improvement
- **What happens:** `doctor` checks whether `~/.brewprune/bin` appears in `~/.profile` (it does), but has no check for whether it's actually in the current session's `$PATH`. These are two different states that produce very different behavior. The current check only tells you whether the profile was written, not whether tracking is actually active.
- **Expected:** A dedicated check: "✗ ~/.brewprune/bin not active in current PATH" with action "Run: source ~/.profile". This would surface the #1 setup issue (PATH not yet sourced) as a distinct diagnostic, separate from "PATH configured in profile."
- **Repro:** `docker exec brewprune-r9 brewprune doctor` (after quickstart, before `source ~/.profile`)

---

### Area 7: Destructive Operations

#### [AREA 7] `undo latest --yes` output mixes two progress styles — list and progress bar run simultaneously

- **Severity:** UX-polish
- **What happens:** The undo output shows a progress bar line followed by individual "Restored X" messages:
  ```
  [=======================================>] 100% Restoring packages
  Restoring packages from snapshot......
  Restored bat
  Restored fd
  Restored jq
  Restored ripgrep
  Restored tmux
  ```
  The progress bar shows 100% and then another animation (`......`) appears and individual items are listed. It's as if two different rendering paths are running.
- **Expected:** Pick one style: either the progress bar (concise) or the item-by-item list (verbose). The current output suggests both run simultaneously.
- **Repro:** `docker exec brewprune-r9 brewprune undo latest --yes`

---

#### [AREA 7] `remove nonexistent-package` error is minimal and doesn't suggest next steps

- **Severity:** UX-polish
- **What happens:**
  ```
  Error: package "nonexistent-package" not found
  ```
  Exit 1. No suggestion to check spelling, use `brew list`, or run `brewprune scan`.
- **Expected:** Match the more helpful error from `explain nonexistent-package`:
  ```
  Error: package not found: nonexistent-package
  Check the name with 'brew list' or 'brew search nonexistent-package'.
  If you just installed it, run 'brewprune scan' to update the index.
  ```
  The `remove` command's error should be equally helpful.
- **Repro:** `docker exec brewprune-r9 brewprune remove nonexistent-package`

---

### Area 8: Edge Cases & Error Handling

#### [AREA 8] `stats` with no database shows a double-nested error path in the message

- **Severity:** UX-improvement
- **What happens:** When no database exists:
  ```
  Error: failed to get usage trends: failed to list packages: database not initialized — run 'brewprune scan' to create the database
  ```
  The error chain is exposed to the user: "failed to get usage trends: failed to list packages: database not initialized".
- **Expected:** Surface only the actionable end of the error chain:
  ```
  Error: database not initialized — run 'brewprune scan' to create the database
  ```
  Same issue applies to `remove --safe` with no database:
  ```
  Error: failed to get packages: failed to list packages: database not initialized — run 'brewprune scan' to create the database
  ```
- **Repro:** `docker exec brewprune-r9 rm -rf /home/brewuser/.brewprune && docker exec brewprune-r9 brewprune stats`

---

#### [AREA 8] `brewprune unused` after undo (without re-scan) shows stale "5 new formulae since last scan" warning with wrong package list

- **Severity:** UX-critical
- **What happens:** After `brewprune undo latest --yes` restores packages, running `brewprune unused` immediately shows:
  ```
  ⚠  5 new formulae since last scan. Run 'brewprune scan' to update shims.
  ```
  Additionally, the package list and scores appear partially stale — libevent, libgit2, and oniguruma appear in the safe tier (80/100 with no dependents) even though they have dependents in the full install. This is a consequence of the stale database state post-undo.
- **Expected:** After `undo`, the tool should either (a) automatically trigger a scan before showing recommendations, or (b) block `unused` output with a hard error: "Database is stale after restore — run 'brewprune scan' before viewing recommendations."
- **Repro:** `docker exec brewprune-r9 brewprune remove --safe --yes` → `docker exec brewprune-r9 brewprune undo latest --yes` → `docker exec brewprune-r9 brewprune unused`

---

#### [AREA 8] `status` returns exit 0 and shows "PATH configured" when `~/.brewprune` doesn't exist

- **Severity:** UX-polish
- **What happens:** After `rm -rf /home/brewuser/.brewprune`, `brewprune status` returns:
  ```
  Tracking:     stopped  (run 'brewprune watch --daemon')
  Shims:        inactive · 0 commands · PATH configured (restart shell to activate)
  Last scan:    never — run 'brewprune scan' · 0 formulae · 4 KB
  ```
  "PATH configured" appears even though the `.brewprune` directory was deleted. This means the PATH check is reading from the shell profile, not from whether the shim directory actually exists.
- **Expected:** The status line should reflect the actual state: "Shims: not installed (run 'brewprune scan' to build)" when the shim binary doesn't exist.
- **Repro:** `docker exec brewprune-r9 rm -rf /home/brewuser/.brewprune && docker exec brewprune-r9 brewprune status`

---

### Area 9: Output Quality & Visual Design

Overall visual quality is high. The tool uses consistent table formatting throughout, with well-aligned columns. The `─` separator lines and spacing create a clean, readable layout.

#### Color Usage (as best observed via terminal output text)

- Tier labels in the Status column use distinguishable symbols as proxies for color: `✓ safe`, `~ medium`, `⚠ risky`. These work in non-color contexts.
- Warning banners use `⚠` prefix consistently.
- Error messages start with `Error:` prefix consistently.
- `doctor` uses `✓`, `⚠`, and `✗` for pass, warning, and fail — this is clear and consistent.
- No evidence of color-coded tier column values in the unused table (colors can't be verified in this audit context, but the symbol-based system is solid).

#### Terminology Consistency

- **daemon vs service vs background process:** "daemon" is used consistently throughout. `watch --daemon`, "daemon started", "daemon running". No mixing with "service" or "background process". Consistent.
- **score vs confidence:** `unused --help` says "confidence score" and "confidence scores"; the table header says "Score"; `explain` says "removal confidence score". Minor mixing but acceptable.
- **snapshot vs backup vs rollback point:** "snapshot" is used exclusively. Consistent.
- **tier vs level vs category:** "tier" is used exclusively. Consistent.

#### Positive Notes on Output

- The `unused` warning banner is well-structured and actionable with three numbered steps.
- The Reclaimable summary line ("Reclaimable: 39 MB (safe) · 248 MB (medium) · 66 MB (risky)") is an excellent at-a-glance value proposition.
- `remove --safe --yes` shows a real-time progress bar: `[=======================================>] 100% Removing packages`. Clean.
- `undo --list` output is minimal and correct: ID, Created, Packages, Reason columns.
- `remove --no-snapshot --dry-run` prominently warns: "⚠  Snapshot: SKIPPED (--no-snapshot) — removal cannot be undone!" — excellent danger visibility.
- `--tier` filter indicator in the summary line is creative: `[SAFE: 5 packages (39 MB)] · MEDIUM: 32 ...` (brackets highlight the active filter). Clear and elegant.
- Conflict detection errors are specific: "only one tier flag can be specified at a time (got --safe, --medium, and --risky)".
- Unknown subcommand errors list all valid commands inline: "Valid commands: scan, unused, remove, undo, status, stats, explain, doctor, quickstart, watch, completion".

#### Spacing and Sections

- Consistent use of empty lines between sections.
- The verbose breakdown per-package in `unused --verbose` uses horizontal rules (`───...`) between packages — readable but verbose for 40 packages.
- `stats --all` lacks a header line ("Sorted by: most used first" is present, which is good) but has no "Showing N of N packages" count line, unlike the default view.

---

## Positive Observations

1. **Conflict flag detection is excellent.** Every conflicting flag combination tested (`--all --tier`, `--safe --medium`, `--daemon --stop`, `--safe --tier medium`, `--safe --medium --risky`) produces a clear, specific error naming exactly which flags conflict and why.

2. **`unused` warning banner is well-designed.** It correctly appears when no usage data exists, explains why recommendations are heuristic-only, provides exactly three actionable steps, and notes confidence level at the bottom. The full-tier display when no usage data exists (instead of the normal safe+medium default) is smart behavior.

3. **`remove` safety features all work correctly.** Dry-run is labeled clearly ("Dry-run mode: no packages will be removed."). Snapshot creation confirms the ID. `undo --list` is immediately readable. The `--no-snapshot` danger warning is prominent.

4. **Error messages for missing packages are helpful.** `explain nonexistent-package` correctly suggests `brew list`, `brew search`, and `brewprune scan`. The error is actionable.

5. **`doctor` graceful degradation across all three states** (healthy, degraded, blank) produces appropriate output without crashing. The blank state correctly skips checks that require the database.

6. **`quickstart` four-step flow is clean.** Each step shows a `✓` on completion. The self-test pass message includes context about shell restart being needed. Progress is clear throughout.

7. **Invalid enum values surface valid options inline.** `--tier invalid` → "must be one of: safe, medium, risky". `--sort invalid` → "must be score, size, or age". `--min-score 200` → "must be 0-100". All correctly validated.

8. **Score display as `80/100` rather than just `80` is excellent.** Immediately communicates scale without requiring the user to know the range.

9. **Round 9 vs prior rounds:** The PATH warning at quickstart end (`⚠  TRACKING IS NOT ACTIVE YET`) is much improved over earlier rounds where this was absent. The `undo` command now correctly restores packages and provides a rescan reminder. The `stats` command correctly hides unused packages by default with a count of hidden entries.

---

## Recommendations Summary

### High Priority (UX-critical)

1. **Fix segfault on `explain` and `stats --package` after undo + before rescan** (Area 4/5). This is a crash that can occur in normal usage flow. The undo warning should be more explicit: "Run 'brewprune scan' before using explain, stats --package, or unused."

2. **Fix stale database state after undo making `unused` show incorrect package data** (Area 8). After undo, the dependency graph is stale and packages appear in wrong tiers. Either block `unused` until rescan, or auto-trigger a scan on undo completion.

3. **Reclassify doctor pipeline failure as "WARN: session not sourced" not "CRITICAL: pipeline broken"** (Area 6). The pipeline test fails in any non-interactive shell context (docker exec, cron, CI). This should be a skippable or context-aware check, not an always-failing critical blocker.

### Medium Priority (UX-improvement)

4. **`status` Shims line: "inactive · 0 commands" is misleading** — "0 commands" doesn't reflect shim invocations recorded in usage.log. Fix the label or the count.

5. **Stats not recording all shim invocations** — git and jq shim invocations appear in usage.log but not in `stats` output. Investigate shim→package resolution failures.

6. **`remove nonexistent-package` error needs the same helpful context as `explain nonexistent-package`** — add suggestions to check `brew list` and run `brewprune scan`.

7. **Error message chaining exposed to users** — `stats` and `remove` with no database show internal error chain ("failed to get usage trends: failed to list packages: database not initialized"). Surface only the terminal message.

8. **Quickstart "Setup complete" before PATH warning** — restructure so the critical warning comes before the completion message, not after.

9. **Doctor missing active-PATH check** — add a check for whether `~/.brewprune/bin` is in `$PATH` right now (not just in the profile). This surfaces the most common post-setup issue as a diagnostic.

### Lower Priority (UX-polish)

10. **Sort mode annotation inconsistency** — `--sort size` and `--sort score` don't show "Sorted by:" footer; `--sort age` does. Add footers for all sort modes.

11. **Warning banner on `--casks` with no casks** — skip the full banner and just show the cask message.

12. **`undo` progress bar + item list running simultaneously** — pick one rendering style.

13. **"Protected: YES (part of 47 core dependencies)"** — the 47 count is unexplained and confusing.

14. **Quickstart self-test step should announce expected duration upfront** before the spinner starts.

15. **`explain` recommendation formatting** — use a numbered list for dry-run-then-remove steps.
