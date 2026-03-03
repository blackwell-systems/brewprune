# Cold-Start UX Audit Report — Round 11

**Audit Date:** 2026-03-02 (executed 2026-03-03 UTC)
**Tool Version:** brewprune version dev (commit: unknown, built: unknown)
**Container:** brewprune-r11
**Environment:** Linux aarch64 (Ubuntu) with Homebrew (Linuxbrew), via `docker exec`
**Auditor:** Automated cold-start audit agent

---

## Summary Table

| Severity       | Count |
|----------------|-------|
| UX-critical    | 3     |
| UX-improvement | 6     |
| UX-polish      | 8     |
| **Total**      | 17    |

---

## Findings by Area

---

### Area 1: Discovery & Help System

**Commands run and results:**

`brewprune --help` (exit 0): Displays full help with Quick Start section, feature list, examples, Available Commands, and flags. Clean and readable.

`brewprune --version` (exit 0): `brewprune version dev (commit: unknown, built: unknown)`

`brewprune -v` (exit 1): `Error: unknown shorthand flag: 'v' in -v` — this is a regression; `-v` was expected to show the version string.

`brewprune help` (exit 0): Identical output to `--help`. Correct.

`brewprune` (no args, exit 0): Shows full help output. Good — new users are not stranded at a bare error.

All subcommand `--help` pages were accessible and informative. The `watch --help` page clearly explains the 30-second polling interval. The `unused --help` explains the tier system, score components, and the `--tier` vs `--all` interaction clearly. The `remove --help` explains both shortcut flags and `--tier`.

---

### [AREA 1] Finding: `-v` Fails Instead of Printing Version

- **Severity:** UX-critical
- **What happens:** `brewprune -v` prints `Error: unknown shorthand flag: 'v' in -v` and exits 1.
- **Expected:** `brewprune -v` prints `brewprune version dev (commit: unknown, built: unknown)` and exits 0, matching `--version`.
- **Repro:** `docker exec brewprune-r11 brewprune -v`

---

### [AREA 1] Finding: `--help` Flag Not Listed in Flags Section

- **Severity:** UX-polish
- **What happens:** The top-level `--help` flag appears in the Flags section (as `-h, --help`) but the `--version` flag is not shown as `-v, --version`. There is no short form for `--version`.
- **Expected:** If `-v` is intended to be the short form, it should work. If it is not, the help text need not imply it.
- **Repro:** `docker exec brewprune-r11 brewprune --help`

---

### [AREA 1] Finding: No Mention of Prerequisites Between Commands in Help

- **Severity:** UX-polish
- **What happens:** Subcommand help pages (e.g., `stats --help`, `unused --help`) do not mention that `scan` must be run first. The dependency between commands is only hinted at in the top-level Quick Start section.
- **Expected:** Subcommand help pages could include a one-line prereq note such as "Requires: brewprune scan (run first)".
- **Repro:** `docker exec brewprune-r11 brewprune stats --help` (no mention of scan requirement)

---

### Area 2: Setup & Onboarding

**Quickstart path:** `brewprune quickstart` completed all 4 steps and produced clear, numbered output. Step 1 showed package count and disk size. Step 2 confirmed PATH setup. Step 3 confirmed daemon PID and log location. Step 4 confirmed self-test passed. The PATH warning at the end was prominent and actionable.

Exit code: 0. Duration: approximately 35 seconds (self-test includes a 30-second sleep).

`brewprune status` immediately after quickstart:
```
Tracking:     running (since just now, PID 1101)
Events:       1 total · 1 in last 24h
Shims:        not yet active · PATH configured (restart shell to activate)
Last scan:    just now · 40 formulae · 72 KB
Data quality: COLLECTING (0 of 14 days)
```
Good. The self-test event is recorded. The PATH/shim warning is accurate.

`brewprune doctor` after quickstart:
- Passes: database, accessibility, 40 packages, 1 usage event, daemon running, shim binary found.
- Warns: PATH not yet active (actionable: `source ~/.profile`), pipeline test skipped (because PATH not active).
- No bare "Error:" lines.
- Exit code: 0.

**Manual path:** `rm -rf .brewprune` then `scan`, `watch --daemon`, `status`, `doctor`. All worked correctly. `scan` output was a full package table with a clear "add to PATH" instruction. `watch --daemon` confirmed PID and log location. `status` immediately after showed the grace period message (not the alarming "shim warning"). `doctor` showed actionable warnings.

---

### [AREA 2] Finding: Quickstart Writes PATH Config to Wrong User's Profile

- **Severity:** UX-improvement
- **What happens:** `quickstart` output says "Added /home/brewuser/.brewprune/bin to PATH in /home/brewuser/.profile" — the container runs as `brewuser`, not `root`. This is correct for the container but is noted because the audit prompt uses `/root/.brewprune/` paths, which do not exist. The actual data directory is at `/home/brewuser/.brewprune/`.
- **Expected:** Audit documentation should note that the user running `brewprune` in this container is `brewuser`, not `root`. All `ls -la /root/.brewprune/` commands in the audit prompt fail with "Permission denied" or "No such file or directory."
- **Repro:** `docker exec brewprune-r11 ls -la /root/.brewprune/` → `Permission denied`

Note: This is a documentation/audit-prompt issue, not a tool issue.

---

### [AREA 2] Finding: Quickstart Self-Test Takes ~35s With No Intermediate Progress

- **Severity:** UX-polish
- **What happens:** During Step 4, the output prints "Verifying shim → daemon → database pipeline..." and then pauses approximately 30 seconds before printing the result. There is no spinner or progress indication during the wait.
- **Expected:** A spinner or periodic dot output (e.g., "....") would reassure the user the command has not hung.
- **Repro:** `docker exec brewprune-r11 brewprune quickstart` — observe the pause at Step 4.

---

### Area 3: Core Feature — Unused Package Discovery

**Default (`brewprune unused`, exit 0):** Displays a warning banner (yellow text in terminal: "WARNING: No usage data available"), followed by a tier count summary ("SAFE: 5 · MEDIUM: 32 · RISKY: 3"), then a full 40-package table, then a sorted-by annotation, reclaimable disk totals, and confidence footer. Output is comprehensive.

Table columns: `Package | Size | Score | Uses (7d) | Last Used | Depended On | Status`

Status column uses: `✓ safe` (safe tier), `~ medium` (medium tier), `⚠ risky` (risky tier). Clear visual hierarchy.

**`--all`**: Identical to default output when no usage data exists (both show all tiers). The warning banner correctly omits the "All tiers shown as a result" qualifier when `--all` is explicitly set.

**`--tier safe/medium/risky`**: Correctly filters. The tier count summary highlights the active filter in brackets: `[SAFE: 5 packages (39 MB)]`. Clean.

**`--min-score 70`**: Shows "Showing 6 of 40 packages (score >= 70)" before the table. Shows "Hidden: 34 below score threshold (70)" after. Both context lines are present.

**`--sort size`**: Works. Footer says "Sorted by: size (largest first)". Packages reordered by disk size.

**`--sort age`**: Works. Changes the "Last Used" column header to "Installed" and sorts by install date oldest first. Footer says "Sorted by: install date (oldest first)". The column header change is a nice contextual touch.

**`--casks`**: Gracefully handles no casks: "No casks found in the Homebrew database." with an explanation. Exit 0. No crash.

**`--verbose`**: Shows per-package score breakdown instead of table rows. Each package gets a block with Usage/Dependencies/Age/Type breakdown and a "Reason:" line. Very informative for users who want to understand scores.

**`--tier safe --all`**: Returns error "–-all and --tier are mutually exclusive" (exit 1) with a helpful clarification. This matches the documented behavior.

---

### [AREA 3] Finding: `--risky` Dry-Run Output Has Warning Before Table Header

- **Severity:** UX-polish
- **What happens:** `brewprune remove --risky --dry-run` prints the "⚠ 34 packages skipped" warning line BEFORE the table header line. The output reads:
  ```
  Packages to remove (risky tier):

  ⚠  34 packages skipped (locked by dependents) — run with --verbose to see details
  Package          Size     Score   ...
  ```
  The warning appears between the section label and the table header, interrupting the visual flow.
- **Expected:** The "skipped" warning should appear AFTER the table (in the Summary section), not between the label and the table header.
- **Repro:** `docker exec brewprune-r11 brewprune remove --risky --dry-run`

---

### [AREA 3] Finding: `--verbose` Output Is Very Long With No Paging Hint

- **Severity:** UX-polish
- **What happens:** `brewprune unused --verbose` with 40 packages produces hundreds of lines of output with no suggested pipe to a pager, no `--limit` flag, and no count of how many entries will follow.
- **Expected:** A leading "Showing verbose output for 40 packages (pipe to less for paging)" line would help users manage the output.
- **Repro:** `docker exec brewprune-r11 brewprune unused --verbose`

---

### [AREA 3] Finding: Usage Score Inconsistency Between `unused --verbose` and `explain`

- **Severity:** UX-improvement
- **What happens:** After running 5 shim commands and the daemon processing them, `unused --verbose` shows `Usage: 40/40 pts - never observed execution` for packages that WERE run (bat, fd, jq, etc.), while `brewprune explain git` shows `Usage: 0/40 pts - used today`. The `unused` command did not pick up the fresh usage data while `explain` did.
- **Expected:** `unused --verbose` and `explain` should show the same usage score for the same package in the same state.
- **Repro:** Run shims via `~/.brewprune/bin/`, wait 35s, then compare `brewprune unused --verbose` vs `brewprune explain git`.

Note: This may be related to when `unused` queries the DB vs when `explain` queries it, or a caching issue.

---

### Area 4: Data Collection & Tracking

**Start daemon:** `brewprune watch --daemon` — clean output with PID and log file path. Exit 0.

**`status` immediately after:** Shows grace period message: "(no events yet — daemon started just now, this is normal)". Pass.

**Shim execution:** All 5 shims ran and correctly returned tool output. The shims transparently pass through to the real binaries.

**`cat usage.log`:** Shows 5 entries in `<nanosecond-timestamp>,<shim-path>` format. This is internal log format, acceptable.

**`cat watch.log`:** After the 35-second wait (first Area 4 run, with existing daemon from quickstart), showed:
```
2026-03-03T03:39:23Z brewprune-watch: daemon started (PID 1101)
2026-03-03T03:39:23Z brewprune-watch: processed 1 lines, resolved 1 packages, skipped 0
2026-03-03T03:43:46Z brewprune-watch: processed 5 lines, resolved 5 packages, skipped 0
```
The per-cycle logging is working. Format is timestamped and clear.

**`status` after 35s:** Shows `Events: 5 total · 5 in last 24h`. Correct.

**`stats`:** Shows 5 packages used with correct "Total Runs: 1", frequency: daily, trend: →. Shows "Showing 5 of 40 packages (35 with no recorded usage — use --all to see all)". Clean.

**`stats --days 1/7/90`:** All work correctly and adjust the summary line ("last 1 day", "last 7 days", "last 90 days"). No issues.

**`stats --package git/jq`:** Shows full per-package breakdown. Appends a tip: "Run 'brewprune explain git' for removal recommendation."

**`stats --all`:** Shows all 40 packages including zero-usage ones. "Sorted by: most used first" annotation appears AFTER the table. Pass.

**`watch --stop`:** Prints "Stopping daemon......" then "✓ Daemon stopped". Exit 0.

**`status` after stop:** Shows "Tracking: stopped (run 'brewprune watch --daemon')". Clear.

---

### [AREA 4] Finding: Daemon Skips Usage Events Written Before First Cycle Completes

- **Severity:** UX-critical
- **What happens:** When `brewprune watch --daemon` is started and shim commands are run immediately afterward (before the 30-second cycle fires), the daemon initializes its read offset to the current end-of-file of `usage.log`. Any events already written to the log before the first cycle completes are permanently skipped and never recorded. Status shows "Events: 0 total" indefinitely, even with 5 shim events in the log.

  Observed state: `usage.offset` = 267, `usage.log` file size = 267 bytes. Offset equals file size, meaning daemon will never read those 5 entries.

  This also affects the regression test scenario explicitly: `scan` + `daemon` + 5 shims + `sleep 35` → 0 events recorded, not 5.

  Root cause: The daemon appears to set `usage.offset` to the current file size at startup (to "skip" pre-existing log entries), but this also skips events written in the brief window between daemon start and first processing cycle.

  The first Area 4 run worked only because the daemon from `quickstart` was already established and had completed a cycle before the 5 new shims were added.

- **Expected:** The daemon should process all entries in `usage.log` that were written after the daemon's start timestamp (or start from offset 0 on first launch), not skip everything present at startup.
- **Repro:** `rm -rf ~/.brewprune && brewprune scan && brewprune watch --daemon && ~/.brewprune/bin/git --version && ~/.brewprune/bin/jq --version && ~/.brewprune/bin/bat --version && ~/.brewprune/bin/fd --version && ~/.brewprune/bin/rg --version && sleep 35 && brewprune status` — Events will show 0, not 5.

---

### [AREA 4] Finding: `stats` With No Usage Shows Confusing Package Count

- **Severity:** UX-polish
- **What happens:** After a fresh scan, `brewprune stats` (no args, no usage data) prints: "No usage recorded yet (35 packages with 0 runs)." but the scan reported 40 packages.
- **Expected:** The count should match the total scanned package count, or clarify why 5 packages are excluded ("35 packages with 0 recent runs; 5 packages have historical usage data").
- **Repro:** Fresh scan of 40 packages, run `brewprune stats` when events exist from a previous session but no recent usage.

---

### Area 5: Package Explanation & Detail View

**`explain git`** (after daemon recorded 1 use of git): Correctly shows `Usage: 0/40 pts - used today`. Score 30 (RISKY) with the cap applied. The "Critical: YES — capped at 70" line appears in the breakdown. Recommendation says "Do not remove." Protected status shown.

**`explain jq`** (after 1 recorded use): Score 40 (RISKY), usage 0/40 pts (recently used). No dependents, leaf package.

**`explain bat`**: Same pattern as jq.

**`explain openssl@3`**: Score 40 (RISKY), Usage 40/40 pts, Dependencies 0/30 pts (9 dependents), Type 0/10 pts, Critical: YES - capped at 70. The cap note appears. Recommendation correctly says "Do not remove."

**`explain curl`**: Score 50 (MEDIUM). Shows "1 used dependent" in deps calculation. Critical cap shown. Recommendation: review before removing. Offers dry-run command.

**`explain nonexistent-package`**: Error message with "check with 'brew list' or 'brew search'", plus the undo hint: "If you recently ran 'brewprune undo', run 'brewprune scan' to update the index." Exit 1.

**`explain` (no args)**: `Error: missing package name. Usage: brewprune explain <package>`. Exit 1. Clear.

---

### [AREA 5] Finding: `explain` Score Inconsistency — Verbose vs Explain

- **Severity:** UX-improvement
- **What happens:** `unused --verbose` shows `Usage: 40/40 pts` for git (treating it as never-used) while `explain git` shows `Usage: 0/40 pts` for git (correctly showing it as recently used). A user running both commands sees contradictory removal scores for the same package.
- **Expected:** Both commands should show the same score for the same package in the same database state.
- **Repro:** Run 5 shims, wait 35s for daemon cycle, then run `brewprune unused --verbose` and `brewprune explain git` and compare the Usage pts line.

---

### [AREA 5] Finding: `explain` Does Not List Which Packages Depend On This One

- **Severity:** UX-improvement
- **What happens:** The `explain` output shows "Dependencies: 0/30 pts - 9 unused dependents" (for openssl@3) but does not name the 9 dependent packages. The user must manually cross-reference to understand why a package is considered risky.
- **Expected:** List the dependent package names, e.g., "Depended on by: curl, libssh2, krb5, ..."
- **Repro:** `docker exec brewprune-r11 brewprune explain openssl@3`

---

### Area 6: Diagnostics

**Healthy state (after quickstart):** All checks pass except PATH (not yet active) and pipeline test (skipped because PATH not active). Alias tip shown. Exit 0. Output uses ✓ for passes, ⚠ for warnings. No ✗ marks.

**Stopped daemon (degraded state):** Shows ⚠ for "Daemon not running (no PID file)" with action: "Run 'brewprune watch --daemon'". Shows ⊘ for pipeline test: "The pipeline test requires a running daemon." Exit 0. The ⊘ symbol is used for skipped checks (distinct from warnings). Good visual differentiation.

**Blank state (rm -rf first):**
```
✗ Database not found at: /home/brewuser/.brewprune/brewprune.db
  Action: Run 'brewprune scan' to create database
⚠ Daemon not running (no PID file)
  Action: Run 'brewprune watch --daemon'
✗ Shim binary not found — usage tracking disabled
  Action: Run 'brewprune scan' to build it

Found 2 critical issue(s) and 1 warning(s). Run the suggested actions above to fix.
```
Exit 1. Correct. No bare "Error:" line after the summary. Clean.

---

### [AREA 6] Finding: No PATH Check for `~/.brewprune/bin`

- **Severity:** UX-improvement
- **What happens:** `doctor` checks whether the shim binary exists and whether PATH is "configured" (written to shell profile), but does not check whether `~/.brewprune/bin` is actually present in the current `PATH`. A user might have removed it manually from their profile after quickstart.
- **Expected:** An explicit PATH check: "✓ ~/.brewprune/bin in active PATH" or "⚠ ~/.brewprune/bin not in active PATH (add to PATH or restart shell)".
- **Repro:** `docker exec brewprune-r11 brewprune doctor` — shows "PATH configured" but not whether PATH is actually active.

---

### [AREA 6] Finding: Alias Tip Appears in Every Doctor Run

- **Severity:** UX-polish
- **What happens:** The tip about `~/.config/brewprune/aliases` appears in every `doctor` output, even in fully configured healthy states. It feels like noise after the first time.
- **Expected:** Show the alias tip only when no alias file exists, or only on first run.
- **Repro:** Run `brewprune doctor` multiple times — the alias tip always appears.

---

### Area 7: Destructive Operations (Remove & Undo)

**`--dry-run` previews:** All worked correctly. Output includes a table of packages to remove with `Dry-run mode: no packages will be removed.` clearly at the bottom. Exit 0.

**`remove --medium --dry-run`**: Shows 6 packages (5 safe + git), plus "⚠ 31 packages skipped (locked by dependents)". The skipped notice appears in the Summary section (after the table). Good placement here (compare with `--risky` where it appears before the table — see Area 3 finding).

**`undo --list` before removal**: "No snapshots available." with explanation. Clear.

**`remove --safe --yes` (actual removal)**: Creates snapshot (ID shown), removes 5 packages with a progress bar `[======>] 100%`, confirms: "✓ Removed 5 packages, freed 39 MB" and "Undo with: brewprune undo 1". Clean.

**`undo --list` after removal**: Shows ID, creation time, package count, reason ("before removal"). Clear.

**`undo latest --yes`**: Correctly identifies latest snapshot, shows packages to restore, performs restore with per-package confirmation ("Restored bat"), then warns to run `brewprune scan`. Exit 0.

**`remove nonexistent-package`**: "Error: package not found: nonexistent-package" with brew list/search suggestion and undo hint. Exit 1.

**`remove --safe --medium`**: "Error: only one tier flag can be specified at a time (got --safe and --medium)". Exit 1.

**`undo 999`**: "Error: snapshot 999 not found / Run 'brewprune undo --list' to see available snapshots". Exit 1.

**`undo` (no args)**: "Error: snapshot ID or 'latest' required / Usage: brewprune undo [snapshot-id | latest] / Use 'brewprune undo --list' to see available snapshots". Exit 1.

---

### [AREA 7] Finding: `remove` Exits 0 When All Packages Are Locked/Skipped

- **Severity:** UX-critical
- **What happens:** When `brewprune remove openssl@3` (or any package locked by dependents) is run without `--dry-run`, output is:
  ```
  ⚠  1 packages skipped (locked by dependents)
  No packages to remove.
  Exit: 0
  ```
  Exit code is 0 even though nothing was removed. Similarly for `remove acl`.
- **Expected:** When all requested packages are skipped (nothing removed), exit code should be 1 to allow scripts to detect the failure.
- **Repro:** `docker exec brewprune-r11 brewprune remove openssl@3; echo "Exit: $?"` → prints "Exit: 0"

---

### [AREA 7] Finding: `--no-snapshot` Warning Not Prominent Enough

- **Severity:** UX-polish
- **What happens:** `remove --help` mentions `--no-snapshot` with "(dangerous)" in the example comment, but there is no flag-level warning in the help text itself. Running `remove --medium --no-snapshot --yes` would silently skip snapshot creation.
- **Expected:** The `--no-snapshot` flag description should include a WARNING: prefix or similar visual marker.
- **Repro:** `docker exec brewprune-r11 brewprune remove --help` — `--no-snapshot` described as "Skip automatic snapshot creation (dangerous)" — the "(dangerous)" is easy to miss.

---

### Area 8: Edge Cases & Error Handling

**No-argument invocations:**
- `brewprune` → shows help (exit 0). Good.
- `brewprune unused` → runs with defaults, shows warning (exit 0). Good.
- `brewprune stats` → "No usage recorded yet (N packages with 0 runs). Run 'brewprune watch --daemon'." (exit 0). Good.
- `brewprune remove` → "Error: no tier specified / Try: brewprune remove --safe --dry-run / Or use --medium or --risky for more aggressive removal" (exit 1). Good.
- `brewprune explain` → "Error: missing package name. Usage: brewprune explain <package>" (exit 1). Good.
- `brewprune undo` → "Error: snapshot ID or 'latest' required / Usage: brewprune undo [snapshot-id | latest]" (exit 1). Good.

**Unknown subcommands:** All show valid command list and suggest `--help`. Exit 1.

**Invalid flag values:**
- `--invalid-flag` → "Error: unknown flag: --invalid-flag" (exit 1). Could be improved with a suggestion to run `--help`.
- `--tier invalid` → "Error: invalid --tier value "invalid": must be one of: safe, medium, risky" (exit 1). Excellent.
- `--min-score 200` → "Error: invalid min-score: 200 (must be 0-100)" (exit 1). Excellent.
- `--sort invalid` → "Error: invalid sort: invalid (must be score, size, or age)" (exit 1). Excellent.
- `--days -1` → "Error: --days must be a positive integer" (exit 1). Good.
- `--days abc` → "Error: --days must be a positive integer" (exit 1). Good.

**Conflicting flags:**
- `remove --safe --medium` → error with flag names listed. Exit 1.
- `remove --safe --medium --risky` → error lists all three flags. Exit 1.
- `remove --safe --tier medium` → "Error: cannot combine --tier with --safe: use one or the other." Exit 1.
- `watch --daemon --stop` → "Error: --daemon and --stop are mutually exclusive: use one or the other." Exit 1.
- `unused --tier safe --all` → "Error: --all and --tier are mutually exclusive." Exit 1.

**Missing database:**
- `unused`, `stats`, `remove --safe` → all: "Error: database not initialized — run 'brewprune scan' to create the database" (exit 1). Consistent and clear.
- `status` → still works, shows appropriate "stopped" state with 0 events and "never" scan. Exit 0.

---

### [AREA 8] Finding: `--invalid-flag` Error Doesn't Suggest `--help`

- **Severity:** UX-polish
- **What happens:** Unknown flag errors say only "Error: unknown flag: --invalid-flag" with no next-step guidance.
- **Expected:** Append "Run 'brewprune unused --help' to see valid flags."
- **Repro:** `docker exec brewprune-r11 brewprune unused --invalid-flag`

---

### Area 9: Output Quality & Visual Design

**Tables:** Columns are well-aligned across all commands. Package names in the `unused` table can be up to 15 characters, which fits the column width without truncation for the packages in this container. The `Status` column uses symbols (✓, ~, ⚠) paired with text labels for both color-sighted and color-blind users.

**Colors (inferred from symbol use, as docker exec strips color):** The `--help` output uses bullet characters (•). The doctor output uses ✓, ⚠, ✗, and ⊘ to distinguish four states (pass, warn, fail, skipped). These provide non-color differentiation.

**Terminology consistency:**
- "daemon" is used consistently throughout (not "service" or "background process"). Pass.
- "score" and "confidence score" are used, occasionally also "removal confidence." Minor inconsistency.
- "snapshot" is used consistently (not "backup" or "rollback point"). Pass.
- "tier" is used consistently (not "level" or "category"). Pass.

**Symbols:** Checkmarks (✓), bullets (•), warning (⚠), x-marks (✗), and progress bars are used. These are consistent and purposeful.

**Context lines:** Present throughout. `unused --min-score 70` shows "Showing 6 of 40 packages (score >= 70)" before and "Hidden: 34 below score threshold (70)" after. `stats` shows "Showing 5 of 40 packages (35 with no recorded usage — use --all to see all)" before and repeats the count in the summary.

**Progress indicators:** `remove --safe --yes` shows a real-time progress bar: `[=======================================>] 100%`. Excellent. `watch --daemon` shows "Starting daemon......" (animated dots in a live terminal). `undo latest --yes` shows "Restoring packages from snapshot......" then per-package lines. Good.

**Errors vs Warnings:** Errors use "Error:" prefix. Warnings use ⚠. Doctor uses ✗ for critical failures. Severity levels are visually distinct.

**`stats --all` sort annotation:** The "Sorted by: most used first" annotation appears AFTER the table, not before it. Pass.

---

### [AREA 9] Finding: `explain` Output Uses Different Score Framing Than `unused --verbose`

- **Severity:** UX-polish
- **What happens:** In `unused --verbose` the breakdown header is "Breakdown:" with no parenthetical. In `explain`, the header is "Breakdown:" but accompanied by "(removal confidence score: 0 = keep, 100 = safe to remove)". The `unused --verbose` format adds the explanation at the bottom of the full output in a "Breakdown:" section rather than per-package. Minor but inconsistent.
- **Expected:** Both views should show the same explanatory note per-package or both should omit it.
- **Repro:** Compare `brewprune unused --verbose` per-package blocks vs `brewprune explain git`.

---

### [AREA 9] Finding: `remove --dry-run` Does Not Display "DRY RUN" Banner Prominently

- **Severity:** UX-polish
- **What happens:** The dry-run notice "Dry-run mode: no packages will be removed." appears only at the very bottom of the output, after the full summary. If the output scrolls off screen, the user may miss that this was a preview.
- **Expected:** A "DRY RUN — NO CHANGES WILL BE MADE" banner at the TOP of the output, before the package table.
- **Repro:** `docker exec brewprune-r11 brewprune remove --safe --dry-run` — notice only appears at end.

---

## Positive Observations

1. **Error messages are excellent.** Every invalid flag shows valid options. Every unknown command shows the full valid command list. Conflicting flags produce specific, clear error messages naming the flags that conflict.

2. **`undo` flow is clean.** Snapshot creation is visible, IDs are shown, and `undo latest` correctly identifies and restores. The post-undo scan reminder is prominent and specific.

3. **Grace period UX is working.** `status` immediately after `watch --daemon` shows the "(no events yet — daemon started just now, this is normal)" message instead of an alarming warning. This was a regression fix from a prior round.

4. **`doctor` in blank state exits 1 cleanly.** No bare "Error:" line printed. The summary counts critical issues and warnings separately and directs users to run the suggested actions.

5. **`--tier` / `--all` conflict is handled gracefully** with a clear error and an explanation of what each flag does.

6. **`stats --all` sort annotation** appears after the table (not before), matching expected behavior.

7. **Tier filtering UI** (the bracket notation `[SAFE: 5 packages (39 MB)]` for active filter) is a clean visual indicator that communicates active state without extra words.

8. **`watch --help`** explicitly documents the 30-second polling interval, which is important for users trying to understand why events aren't showing up immediately.

9. **`--casks` on Linux** fails gracefully with a meaningful no-casks message rather than an error.

10. **`remove nonexistent-package` error** includes the undo hint ("If you recently ran 'brewprune undo', run 'brewprune scan' to update the index"). Well done.

---

## Recommendations Summary

### High Priority (UX-critical)

1. **Fix `-v` to show version.** It currently errors. This is a straightforward shorthand mapping. It was expected to work and is documented as working. (Area 1)

2. **Fix daemon startup offset initialization.** The daemon should not advance the read offset to the current end-of-file at startup. New daemons should start from the offset stored in `usage.offset` (or 0 if no offset file exists). Pre-existing log entries from immediately before the first cycle should be processed, not skipped. This is the most impactful reliability bug. (Area 4)

3. **`remove` should exit 1 when nothing is removed.** When all candidates are locked/skipped and "No packages to remove" is printed, the exit code must be 1. This is critical for scripting use cases. (Area 7)

### Medium Priority (UX-improvement)

4. **Fix score inconsistency between `unused --verbose` and `explain`.** Both commands query the same database but produce different usage scores for the same package. Investigate whether `unused` reads a cached score vs `explain` recomputes live. (Areas 3, 5)

5. **Show dependent package names in `explain`.** Currently shows a count ("9 unused dependents") but not the names. Names would let users make informed removal decisions. (Area 5)

6. **Add a PATH check to `doctor`.** Check whether `~/.brewprune/bin` is in the current `PATH` (not just in the shell profile file). (Area 6)

7. **Add stats `--package` undo hint for packages found in DB but not brew list.** Currently packages not found at all get the hint; packages in DB but missing from brew should get similar guidance.

### Low Priority (UX-polish)

8. **Add paging hint to `unused --verbose`** for large package lists.

9. **Move the "⚠ packages skipped" warning in `remove --risky --dry-run`** to appear after the table, not before it.

10. **Add "DRY RUN" banner at top of `remove --dry-run` output**, not just at the bottom.

11. **Show alias tip only when no alias file exists**, not on every `doctor` run.

12. **Add `--help` suggestion to unknown-flag errors.**

13. **Add `--no-snapshot` visual WARNING in help text** (current "(dangerous)" in examples is too subtle).

14. **Add subcommand prereq notes** (e.g., "Requires: brewprune scan") to `stats`, `unused`, `explain`, and `remove` help pages.

---

## Regression Verification (Round 11)

| Regression Check | Status | Observed Output |
|---|---|---|
| **Daemon records usage events** (core fix) | FAIL | With a freshly started daemon (new session), 5 shim commands + `sleep 35` → `Events: 0 total`. Daemon sets offset = file size at startup, skipping all pre-existing entries. Only works when daemon was already established from a prior session (as in the first Area 4 run, which showed 5 events correctly via the quickstart daemon). |
| **watch.log per-cycle logging** | PASS (with caveat) | In the first Area 4 run (using quickstart daemon), watch.log showed `processed 5 lines, resolved 5 packages, skipped 0`. In subsequent test runs with new daemons, no cycle summaries appeared because no entries were processed (see above). |
| **`remove` exits 1 when all removals fail** | FAIL | `brewprune remove openssl@3` → "No packages to remove." → Exit: 0. Should be exit 1. |
| **`remove` not-found undo hint** | PASS | `remove nonexistent-package` → error includes "If you recently ran 'brewprune undo', run 'brewprune scan' to update the index." Multi-line format confirmed. |
| **Stale dep graph after undo + scan** | PASS | `undo latest --yes` then `brewprune scan` then `explain bat` showed correct score (80/SAFE, no dependents). No stale rows observed. |
| **`doctor` exits 1 cleanly on critical issues** | PASS | Blank state doctor exits 1 with "Found 2 critical issue(s) and 1 warning(s)." No bare "Error:" line after summary. |
| **`stats --package <pkg>` undo hint** | PASS | `stats --package tmux` after undo (before scan) → "Error: package tmux not found. / If you recently ran 'brewprune undo', run 'brewprune scan' to update the index." Exit 1. |
| **`status` grace period** | PASS | `watch --daemon` followed immediately by `status` → "(no events yet — daemon started just now, this is normal)". Alarming shim warning not shown. |
| **`-v` shows version, not conflict** | FAIL | `brewprune -v` → "Error: unknown shorthand flag: 'v' in -v". Exit 1. Expected version string. |
| **`stats --all` sort annotation after table** | PASS | "Sorted by: most used first" appears after the table body, before the Summary line. Correct placement. |

**Regression Summary:** 3 FAIL, 1 PARTIAL PASS (watch.log logging works only when daemon is established), 6 PASS.

---

*End of Round 11 UX Audit Report*
