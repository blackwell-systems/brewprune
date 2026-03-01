# Cold-Start UX Audit Report — Round 8

**Audit Date:** 2026-03-01
**Tool Version:** brewprune version dev (commit: unknown, built: unknown)
**Container:** brewprune-r8
**Environment:** Linux aarch64 (Ubuntu) with Homebrew (Linuxbrew)
**Auditor:** Claude (Sonnet 4.6)

---

## Summary Table

| Severity       | Count |
|----------------|-------|
| UX-critical    | 3     |
| UX-improvement | 9     |
| UX-polish      | 8     |
| **Total**      | 20    |

---

## Findings by Area

---

### Area 1: Discovery & Help System

#### [AREA 1] `brewprune` with no args shows full help (good), but is identical to `brewprune help`

- **Severity:** UX-polish
- **What happens:** Running `brewprune` with no args exits 0 and shows the full help page — identical output to `brewprune help`. There is no distinction between "no args" (which typically implies "what do I do?") and explicitly asking for help. A more guided first-run message might be more actionable.
- **Expected:** Either: (a) the no-args invocation is fine as-is (help is the right response), or (b) a condensed "getting started" message with a single prominent call to action (`brewprune quickstart`).
- **Repro:** `docker exec brewprune-r8 brewprune`

#### [AREA 1] `-v` flag is version, not verbose — potentially surprising

- **Severity:** UX-polish
- **What happens:** `-v` prints the version string (`brewprune version dev (commit: unknown, built: unknown)`). However, the `unused` command has a `-v`/`--verbose` flag documented separately. The global `-v` is not consistent with the subcommand `-v`.
- **Expected:** Either: the global `-v` is clearly documented as "version" (it is, in the global flags section), or it's changed to `-V` for version to free `-v` for verbose globally. The flag table shows `-v, --version  show version information` which is clear, but `-v` for verbose on subcommands could cause user confusion.
- **Repro:** `docker exec brewprune-r8 brewprune -v` vs `docker exec brewprune-r8 brewprune unused -v`

#### [AREA 1] `quickstart --help` does not describe the self-test step's success criteria

- **Severity:** UX-polish
- **What happens:** The quickstart help says "Run a self-test to confirm the shim → daemon → database pipeline works" but does not explain what happens if it fails, or that it takes ~30 seconds.
- **Expected:** Note the approximate duration of the self-test (30s) and what action to take if the self-test fails.
- **Repro:** `docker exec brewprune-r8 brewprune quickstart --help`

#### [AREA 1] `watch --help` mentions 30-second polling interval but help text buried

- **Severity:** UX-polish
- **What happens:** The watch help says "Usage data is written every 30 seconds to minimise I/O overhead." This is good, but it is the last line of the description and easy to miss.
- **Expected:** The 30-second interval should be more prominently positioned since it affects how long users must wait to verify tracking is working.
- **Repro:** `docker exec brewprune-r8 brewprune watch --help`

---

### Area 2: Setup & Onboarding

#### [AREA 2] Quickstart PATH warning appears after "Setup complete!" — confusing sequencing

- **Severity:** UX-improvement
- **What happens:** The quickstart output prints "Setup complete!" and follow-up instructions, then *after* that prints a large warning box: "TRACKING IS NOT ACTIVE YET / Your shell has not loaded the new PATH." A new user reading top-to-bottom may feel reassured by "Setup complete!" and then confused by the subsequent warning.
- **Expected:** The PATH activation warning should appear *before* "Setup complete!" or be integrated into the success message. Alternatively, success language should be "Almost done — one step remains" to avoid false finality.
- **Repro:** `docker exec brewprune-r8 brewprune quickstart`

```
Setup complete!
...
⚠  TRACKING IS NOT ACTIVE YET    ← appears after "complete", confusing
```

#### [AREA 2] Quickstart Step 2 reports different PATH behavior between first and second run

- **Severity:** UX-polish
- **What happens:** On first run: `✓ Added /home/brewuser/.brewprune/bin to PATH in /home/brewuser/.profile`. On second run: `✓ /home/brewuser/.brewprune/bin is already in PATH`. The second message says "already in PATH" which is slightly misleading — it's in the profile file, but not yet the active shell PATH (as evidenced by the warning that still appears).
- **Expected:** "already configured in PATH" or "already in ~/.profile" to be precise that it is the profile file, not the live shell PATH.
- **Repro:** Run `brewprune quickstart` twice.

#### [AREA 2] Doctor pipeline test fails on manual path setup but passes on quickstart path

- **Severity:** UX-critical
- **What happens:** When using the manual setup path (`scan` → `watch --daemon` → `doctor`), the pipeline test fails: `✗ Pipeline test: fail (35.388s) — no usage event recorded after 35.367s`. However, the same pipeline test passes when called via `quickstart`. The manual path doctor gives the misleading action "Run 'brewprune watch --daemon' to restart the daemon" — the daemon IS running; the issue is that shims are not yet in the PATH.
- **Expected:** The pipeline test failure message should identify that shims are not in the active PATH (not the daemon) as the likely root cause, and give the correct remediation: `source ~/.profile` or restart the shell.
- **Repro:**
  ```
  docker exec brewprune-r8 rm -rf /root/.brewprune
  docker exec brewprune-r8 brewprune scan
  docker exec brewprune-r8 brewprune watch --daemon
  docker exec brewprune-r8 brewprune doctor
  ```

#### [AREA 2] `status` after manual scan shows "PATH configured (restart shell to activate)" but shims are not in docker exec PATH

- **Severity:** UX-improvement
- **What happens:** In both manual and quickstart paths, `status` consistently shows `Shims: inactive · 0 commands · PATH configured (restart shell to activate)`. This message is correct and clear: the shim directory has been added to the profile but is not active in the current shell.
- **Expected:** This is actually good behavior — no issue. The "inactive" label with inline explanation is informative. No change needed. (Noted as a positive.)
- **Repro:** `docker exec brewprune-r8 brewprune status`

#### [AREA 2] Doctor always shows "PATH configured (restart shell to activate)" with no way to confirm tracking is actually working

- **Severity:** UX-improvement
- **What happens:** The `doctor` output always shows this as a warning even in a "healthy" post-quickstart state when the pipeline test passes. There is no distinction between "PATH not configured at all" (critical) and "PATH configured in profile but shell not yet restarted" (warning/informational).
- **Expected:** After a successful pipeline test, the PATH warning could be softened: "PATH profile entry confirmed — shims will activate on next shell session." The warning icon is appropriate but the action message "Restart your shell or run: source ~/.profile" could be more contextually relevant.
- **Repro:** `docker exec brewprune-r8 brewprune doctor` (after quickstart)

#### [AREA 2] Doctor "Tip" about aliases always appears regardless of system state

- **Severity:** UX-polish
- **What happens:** Every `doctor` run includes: "Tip: Create ~/.config/brewprune/aliases to declare alias mappings." This tip appears in all three system states: healthy, degraded, and blank. In a blank state (no database, critical failures), this tip is noise.
- **Expected:** The alias tip should only appear in healthy or near-healthy states, not when there are critical failures. Or it could be demoted to only show when all checks pass.
- **Repro:** `docker exec brewprune-r8 brewprune doctor` in any state.

---

### Area 3: Core Feature — Unused Package Discovery

#### [AREA 3] Double warning banner on default `unused` invocation (no data)

- **Severity:** UX-improvement
- **What happens:** Running `brewprune unused` with no usage data shows TWO separate warning blocks back-to-back:
  1. A multi-line "WARNING: No usage data available" block with step-by-step instructions.
  2. A second `⚠ No usage data yet — showing all packages (risky tier included)` notice.
  The second notice is redundant and adds visual noise.
- **Expected:** One consolidated warning banner that covers both messages: explains there is no usage data, notes that all tiers including risky are being shown as a result, and provides the action steps.
- **Repro:** `docker exec brewprune-r8 brewprune unused`

#### [AREA 3] `--sort age` does not sort by "age" in any meaningful sense

- **Severity:** UX-improvement
- **What happens:** `brewprune unused --sort age` replaces the "Last Used" column header with "Installed" and shows install dates. However, in this container all packages were installed on the same day (`2026-02-28`), so the sort produces a random/arbitrary order. More critically, there is no indication of sort direction (oldest first? newest first?) in the output.
- **Expected:** The table header should indicate the sort column (e.g., bold or annotated with `↑`/`↓`). The sort direction should be documented in `--help` output. "Installed" date as a proxy for "age" is reasonable but could be labeled "Installed date" in the column header.
- **Repro:** `docker exec brewprune-r8 brewprune unused --sort age`

#### [AREA 3] `--tier safe --all` produces a clear error but the error message could be improved

- **Severity:** UX-polish
- **What happens:** `brewprune unused --tier safe --all` exits 1 with: `Error: --all and --tier cannot be used together; --tier already filters to a specific tier`. The error message is technically correct but slightly circular.
- **Expected:** "Error: --all and --tier are mutually exclusive. Use --tier safe to show only safe packages, or --all to show all tiers." — separate the logic from the error explanation.
- **Repro:** `docker exec brewprune-r8 brewprune unused --tier safe --all`

#### [AREA 3] `--remove --medium --dry-run` skipped packages list is very long and printed before the table

- **Severity:** UX-improvement
- **What happens:** `brewprune remove --medium --dry-run` prints 31 lines of "skipped (locked by installed dependents)" before showing the actual removal candidates. For a new user, this is alarming — it looks like the tool is mostly saying "no" before eventually showing what it will do.
- **Expected:** The skipped packages list should either: (a) be printed after the candidates table and summary, or (b) be collapsed to a summary line ("31 packages skipped due to dependencies — run with --verbose to see details") with verbose expansion available.
- **Repro:** `docker exec brewprune-r8 brewprune remove --medium --dry-run`

#### [AREA 3] `--verbose` output on `unused` shows all 40 packages, is very long

- **Severity:** UX-polish
- **What happens:** `brewprune unused --verbose` with no tier filter shows expanded scoring breakdowns for all 40 packages. This is approximately 400 lines of output. The verbose mode is useful for investigating a small set of packages but overwhelming when applied globally.
- **Expected:** A note at the top like "Showing verbose breakdown for 40 packages. Use --tier safe --verbose to narrow scope." Or `--verbose` without `--tier` could default to only showing safe+medium (honoring the default tier behavior), reducing output.
- **Repro:** `docker exec brewprune-r8 brewprune unused --verbose`

---

### Area 4: Data Collection & Tracking

#### [AREA 4] Only 1 event recorded after 5 shim invocations — stats discrepancy

- **Severity:** UX-critical
- **What happens:** After running 5 shimmed commands (`git`, `jq`, `bat`, `fd`, `rg`), only 1 usage event is recorded in the database after the 30-second polling cycle. `brewprune status` shows `Events: 1 total · 1 in last 24h`. The usage.log contains 6 entries (one from the quickstart self-test for git, plus 5 from manual invocations), but only 1 event makes it into the database.

  Further investigation: `stats --package git` shows `Total Uses: 1`, but `stats --package jq`, `stats --package bat`, `stats --package fd`, and `stats --package ripgrep` all show `Total Uses: 0 / Last Used: never`. The daemon appears to only process one event per log flush cycle, or the offset tracking is writing the wrong position after the first flush.

- **Expected:** All 5 (or 6 including self-test) shimmed invocations should be reflected in the database. `stats` should show at minimum `git: 1, jq: 1, bat: 1, fd: 1, ripgrep: 1`.
- **Repro:**
  ```
  docker exec brewprune-r8 brewprune watch --daemon
  docker exec brewprune-r8 /home/brewuser/.brewprune/bin/git --version
  docker exec brewprune-r8 /home/brewuser/.brewprune/bin/jq --version
  docker exec brewprune-r8 /home/brewuser/.brewprune/bin/bat --version
  docker exec brewprune-r8 /home/brewuser/.brewprune/bin/fd --version
  docker exec brewprune-r8 /home/brewuser/.brewprune/bin/rg --version
  docker exec brewprune-r8 sleep 35
  docker exec brewprune-r8 brewprune stats --all
  ```
  Result: only `ripgrep` shows 1 run in stats (from quickstart self-test context), not git/jq/bat/fd.

  Note: After the next quickstart (which runs a self-test using git), `git` then shows 1 event. This suggests the daemon processes only the most-recent or single-event per flush.

#### [AREA 4] `stats --package` shows no usage for packages used via shims (git, jq, bat, fd, rg)

- **Severity:** UX-critical (same root cause as above)
- **What happens:** After running shimmed commands and waiting for the polling cycle, `brewprune stats --package git` shows `Total Uses: 0 / Last Used: never` despite `git` having an entry in `usage.log`. The log contains `/home/brewuser/.brewprune/bin/git` entries but these do not resolve to the `git` package in the database.
- **Expected:** `stats --package git` should show the usage count after the polling cycle processes the log.
- **Repro:** Same as above, then `docker exec brewprune-r8 brewprune stats --package git`

#### [AREA 4] `watch --stop` provides good confirmation feedback

- **Severity:** (Positive observation — no issue)
- **What happens:** `brewprune watch --stop` prints `Stopping daemon......` with animated dots, then `✓ Daemon stopped`. Clean and clear.

#### [AREA 4] PID file is removed after `watch --stop` (good), but watch.log is not cleaned up

- **Severity:** UX-polish
- **What happens:** After `watch --stop`, the `ls` of `.brewprune/` shows `watch.log` still present but `watch.pid` is gone (as expected). The log file persists. On next start, a new log line is appended. This is fine for debugging but a new user may not know the log is there.
- **Expected:** This is acceptable behavior. The log is useful for debugging. No change needed. (Noted here for completeness.)

#### [AREA 4] `stats` default output only shows 1 package when 40 exist but 39 have no usage

- **Severity:** UX-improvement
- **What happens:** `brewprune stats` (default, no flags) shows `Showing 1 of 40 packages (39 with no recorded usage — use --all to see all)`. This is accurate and the hint to `--all` is helpful. However, a new user who just installed the tool may be alarmed that only 1 package appears. The summary line at the bottom is good: "1 package used in the last 30 days."
- **Expected:** Good behavior overall. One improvement: the summary could note the time window more prominently: "In the last 30 days: 1 of 40 packages used."

---

### Area 5: Package Explanation & Detail View

#### [AREA 5] `explain` score breakdown is clear and complete

- **Severity:** (Positive observation — no issue)
- **What happens:** The 4 score components (Usage 40pts, Dependencies 30pts, Age 20pts, Type 10pts) are clearly shown with points earned and a plain-language reason. The cap at 70 for core dependencies is clearly flagged as `Critical: YES - capped at 70`. The total and tier are prominently shown at the top of the breakdown.

#### [AREA 5] `explain` for `openssl@3` verbose description has a minor inconsistency

- **Severity:** UX-polish
- **What happens:** `brewprune explain openssl@3` shows `Dependencies: 0/30 pts - 1 used, 8 unused dependents` in the `--verbose` invocation after the daemon has tracked some usage. However, `unused --verbose` (with no usage data) shows `Dependencies: 0/30 pts - 9 unused dependents`. The dependency count in `explain` is dynamic based on current data, while `unused --verbose` reflects a static snapshot. This is correct behavior but the terminology "1 used" could confuse users (it means "1 dependent that has been used", not "openssl@3 itself was used").
- **Expected:** "1 used, 8 unused dependents" → "1 active dependent, 8 inactive dependents" or "9 dependents (1 recently used)" for clarity.
- **Repro:** `docker exec brewprune-r8 brewprune explain openssl@3` (after some usage tracking)

#### [AREA 5] `explain` with no args gives clear error

- **Severity:** (Positive observation — no issue)
- **What happens:** `brewprune explain` exits 1 with `Error: missing package name. Usage: brewprune explain <package>`. Clean and direct.

#### [AREA 5] `explain` for nonexistent package gives helpful error with alternatives

- **Severity:** (Positive observation — no issue)
- **What happens:** `brewprune explain nonexistent-package` exits 1 with:
  ```
  Error: package not found: nonexistent-package
  Check the name with 'brew list' or 'brew search nonexistent-package'.
  If you just installed it, run 'brewprune scan' to update the index.
  ```
  All three lines add value. Excellent error message.

#### [AREA 5] `explain` recommendation for MEDIUM packages suggests `brewprune remove <package>` directly

- **Severity:** UX-improvement
- **What happens:** For a MEDIUM-tier package like `git`, the recommendation says: "Review before removing. Check if you use this package indirectly. If certain, run 'brewprune remove git'." Suggesting `remove git` for a MEDIUM package without mentioning `--dry-run` first is slightly risky for a new user.
- **Expected:** Append `--dry-run` to the suggested command: "If certain, run 'brewprune remove git --dry-run' to preview, then without --dry-run to remove."
- **Repro:** `docker exec brewprune-r8 brewprune explain git`

---

### Area 6: Diagnostics

#### [AREA 6] Doctor pipeline test correctly skips when daemon is not running

- **Severity:** (Positive observation — no issue)
- **What happens:** `doctor` with stopped daemon shows `⊘ Pipeline test skipped (daemon not running)` with a clear explanation. The skip is well-labeled and the icon `⊘` is distinct from `✓` and `✗`.

#### [AREA 6] Doctor blank-state output is clean and actionable

- **Severity:** (Positive observation — no issue)
- **What happens:** With `~/.brewprune` completely removed, `doctor` produces:
  ```
  ✗ Database not found at: /home/brewuser/.brewprune/brewprune.db
    Action: Run 'brewprune scan' to create database
  ⚠ Daemon not running (no PID file)
    Action: Run 'brewprune watch --daemon'
  ✗ Shim binary not found — usage tracking disabled
    Action: Run 'brewprune scan' to build it
  ```
  Both critical issues point to the same fix (`brewprune scan`), which is correct. The exit code is 1. Clean behavior.

#### [AREA 6] Doctor healthy-state (post-quickstart with pipeline pass) exit code is 0 despite warning

- **Severity:** UX-polish
- **What happens:** After quickstart, `doctor` exits 0 despite the PATH warning. The output says "Found 1 warning(s). System is functional but not fully configured." Exit 0 for a warning-only state is appropriate and the message is accurate. However, the distinction between "exit 0 with warnings" and "exit 0 all clear" is not visually distinct — there is no final "All checks passed!" line for the clean state.
- **Expected:** When all checks pass (including pipeline test) with no warnings: print a final `✓ All checks passed.` line. When warnings exist: print the summary as currently shown.

#### [AREA 6] Doctor does not check PATH directly (only "PATH configured in profile")

- **Severity:** UX-improvement
- **What happens:** Doctor checks whether the shim directory is present in the shell profile (`~/.profile`), but does not check whether `~/.brewprune/bin` is actually in the current `$PATH`. This means if a user has deleted the PATH line from their profile after quickstart, doctor would not catch it.
- **Expected:** Add a check: `⚠ Shim directory not found in $PATH (active shell)` separate from "PATH configured in profile." The current check conflates the two.
- **Repro:** This is a design gap — no single repro command.

---

### Area 7: Destructive Operations

#### [AREA 7] Dry-run output clearly labeled "Dry-run mode: no packages will be removed"

- **Severity:** (Positive observation — no issue)
- **What happens:** All `--dry-run` invocations end with `Dry-run mode: no packages will be removed.` in the output. The summary shows "Snapshot: will be created" to indicate what would happen. Clear and accurate.

#### [AREA 7] Snapshot creation output includes ID and undo command immediately after removal

- **Severity:** (Positive observation — no issue)
- **What happens:** After `remove --safe --yes`, the output shows:
  ```
  Snapshot: ID 1
  Undo with: brewprune undo 1
  ```
  The undo command is right there where it's needed most. Excellent UX.

#### [AREA 7] `undo latest --yes` shows full restoration detail with package names

- **Severity:** (Positive observation — no issue)
- **What happens:** `brewprune undo latest --yes` shows the snapshot details, lists all packages to restore, shows a progress bar, then confirms each package individually (`Restored bat`, `Restored fd`, etc.), followed by a summary. The post-restore warning "Run 'brewprune scan' to update the package database" is a helpful reminder.

#### [AREA 7] `undo` snapshot list does not show disk space reclaimed by each snapshot

- **Severity:** UX-polish
- **What happens:** `undo --list` shows:
  ```
  ID    Created           Packages   Reason
  1     just now          5          before removal
  ```
  There is no size information. A user wondering whether to keep or discard a snapshot cannot tell how much space the removed packages represented.
- **Expected:** Add a "Size" column to `undo --list`: `ID | Created | Packages | Size | Reason`
- **Repro:** `docker exec brewprune-r8 brewprune undo --list`

#### [AREA 7] `remove --safe --medium` error only reports the first two flags, ignores `--risky`

- **Severity:** UX-polish
- **What happens:** `brewprune remove --safe --medium --risky` produces: `Error: only one tier flag can be specified at a time (got --safe and --medium)`. The `--risky` flag is silently ignored in the error message — only the first two conflicting flags are mentioned.
- **Expected:** Report all conflicting flags: `Error: only one tier flag can be specified at a time (got --safe, --medium, and --risky)`
- **Repro:** `docker exec brewprune-r8 brewprune remove --safe --medium --risky`

#### [AREA 7] `remove bat fd --dry-run` warns about "explicitly installed" — slightly confusing

- **Severity:** UX-polish
- **What happens:** When removing named packages, the output shows `⚠ bat: explicitly installed (not a dependency)`. This is a warning, but for a user who just asked to remove `bat`, being told it is "explicitly installed" sounds like a reason NOT to remove it. The intent seems to be "this package was installed directly by you (not as a dependency), so you know what it does."
- **Expected:** Reframe the message: "bat: explicitly installed by you (not a dependency)" or just remove the warning since the user explicitly named the package. The warning is redundant when the user specified the package name.
- **Repro:** `docker exec brewprune-r8 brewprune remove bat fd --dry-run`

---

### Area 8: Edge Cases & Error Handling

#### [AREA 8] Unknown subcommands suggest `--help` but do not list valid commands

- **Severity:** UX-improvement
- **What happens:** `brewprune blorp` exits 1 with `Error: unknown command "blorp" for "brewprune"\nRun 'brewprune --help' for usage.` The error is clear but requires the user to run another command to see valid options.
- **Expected:** Include a brief list of valid subcommands inline: `Error: unknown command "blorp". Valid commands: scan, unused, remove, undo, status, stats, explain, doctor, quickstart, watch, completion`. Or at least: "Did you mean: scan? Run 'brewprune --help' for all commands."
- **Repro:** `docker exec brewprune-r8 brewprune blorp`

#### [AREA 8] `remove` with no flags gives helpful usage error with example

- **Severity:** (Positive observation — no issue)
- **What happens:** `brewprune remove` exits 1 with:
  ```
  Error: no tier specified
  Try:
    brewprune remove --safe --dry-run
  Or use --medium or --risky for more aggressive removal
  ```
  Three-line error with a concrete example and escalation path. Excellent.

#### [AREA 8] Invalid enum values show valid alternatives consistently

- **Severity:** (Positive observation — no issue)
- **What happens:** All invalid enum values show valid options:
  - `--tier invalid` → `must be one of: safe, medium, risky`
  - `--sort invalid` → `must be score, size, or age`
  - `--min-score 200` → `must be 0-100`
  - `--days -1` / `--days abc` → `must be a positive integer`
  All these are clean, specific, and consistent.

#### [AREA 8] `status` with no database still exits 0 and shows partial data

- **Severity:** UX-improvement
- **What happens:** After removing `~/.brewprune`, `brewprune status` exits 0 and shows:
  ```
  Tracking:     stopped  (run 'brewprune watch --daemon')
  Events:       0 total · 0 in last 24h
  Shims:        inactive · 0 commands · PATH configured (restart shell to activate)
  Last scan:    just now · 0 formulae · 4 KB
  Data quality: COLLECTING (0 of 14 days)
  ```
  The "Last scan: just now · 0 formulae" is confusing — no scan was actually run, the database doesn't exist. "just now" implies a recent scan happened, which is misleading.
- **Expected:** When the database does not exist, `status` should show: `Last scan: never — run 'brewprune scan'` and exit with a non-zero code or at minimum a clear "not initialized" indication.
- **Repro:** `docker exec brewprune-r8 rm -rf /home/brewuser/.brewprune && docker exec brewprune-r8 brewprune status`

#### [AREA 8] `watch --daemon --stop` conflict detected and reported clearly

- **Severity:** (Positive observation — no issue)
- **What happens:** `brewprune watch --daemon --stop` exits 1 with: `Error: --daemon and --stop are mutually exclusive: use one or the other`. Clear and specific.

---

### Area 9: Output Quality & Visual Design

#### [AREA 9] Tables are well-aligned with consistent column widths

- **Severity:** (Positive observation — no issue)
- **What happens:** All tabular output (`unused`, `stats --all`, `remove --dry-run`) uses fixed-width columns with proper alignment. Column headers are separated by a `────` divider line. Package names align left, sizes and scores align right within their columns. No truncation observed for the package names in this 40-package environment.

#### [AREA 9] Tier status column uses distinct symbols: `✓ safe`, `~ medium`, `⚠ risky`

- **Severity:** (Positive observation — no issue)
- **What happens:** The Status column in `unused` output uses:
  - `✓ safe` — checkmark prefix, visually positive
  - `~ medium` — tilde prefix, visually neutral
  - `⚠ risky` — warning prefix, visually cautionary
  The symbols are consistent across `unused` default, `unused --all`, `unused --verbose`, and `remove --dry-run`.
- **Note:** Without terminal color output captured (docker exec strips ANSI in some modes), the symbols alone carry the tier meaning effectively. If colors are rendered in a real terminal, this is likely even more effective.

#### [AREA 9] Reclaimable space summary at bottom of `unused` output is very useful

- **Severity:** (Positive observation — no issue)
- **What happens:** The footer `Reclaimable: 39 MB (safe) · 248 MB (medium) · 66 MB (risky, hidden)` gives an immediate answer to "how much can I free?" This appears consistently at the bottom of all `unused` variants.

#### [AREA 9] `stats --all` table has no sort order explanation

- **Severity:** UX-polish
- **What happens:** `stats --all` shows all 40 packages in a seemingly random order (not alphabetical, not by usage count). The one package with usage (`git` or `ripgrep` depending on state) appears first, followed by the rest in an unclear order.
- **Expected:** The default sort for `stats --all` should be documented or at least noted in output, e.g., "Sorted by: most used first." A `--sort` flag for `stats` would be useful but is not currently documented.
- **Repro:** `docker exec brewprune-r8 brewprune stats --all`

#### [AREA 9] Terminology is consistent across all commands

- **Severity:** (Positive observation — no issue)
- **What happens:** Reviewing all output:
  - "daemon" is used consistently (not "service" or "background process")
  - "score" is used consistently (not "confidence" — though the score is a "confidence score" in help text, the tables show "Score")
  - "snapshot" is used consistently (not "backup" or "rollback point")
  - "tier" is used consistently (not "level" or "category")
  One minor note: `unused --help` says "confidence score" in the description but the table header is "Score". This is a very minor inconsistency.

#### [AREA 9] Progress indicators exist for slow operations

- **Severity:** (Positive observation — no issue)
- **What happens:** Operations with delays use animated dots: `Starting daemon......`, `Stopping daemon......`, `Running pipeline test (~30s)......`. The `remove --yes` operation shows a progress bar `[=======================================>] 100% Removing packages`. The `undo latest --yes` shows a similar bar.

#### [AREA 9] `unused` default shows "Reclaimable: X MB (risky, hidden)" even when risky is shown

- **Severity:** UX-polish
- **What happens:** When no usage data exists and risky tier is auto-shown (the special fallback behavior), the footer still says `(risky, hidden)` even though risky IS being displayed in the table. This is contradictory.
- **Expected:** When risky tier is displayed (due to no-data fallback), the footer should say `Reclaimable: 39 MB (safe) · 248 MB (medium) · 66 MB (risky)` — without the "hidden" qualifier.
- **Repro:** `docker exec brewprune-r8 brewprune unused` (with no usage data, default view)

```
# Footer says "risky, hidden" but risky packages ARE shown above:
Reclaimable: 39 MB (safe) · 248 MB (medium) · 66 MB (risky, hidden)  ← contradicts table
```

---

## Positive Observations

The following behaviors are notably good and worth preserving:

1. **Error messages are specific and actionable.** Almost every error includes a clear next step (`run 'brewprune scan'`, `run 'brewprune watch --daemon'`, etc.). The `explain nonexistent-package` error is a standout example.

2. **Dry-run is prominently labeled.** The `Dry-run mode: no packages will be removed.` footer on all dry-run outputs is unambiguous.

3. **Snapshot ID shown immediately after removal.** Printing `Snapshot: ID 1 / Undo with: brewprune undo 1` right after the removal operation puts the safety net information exactly where users need it.

4. **Quickstart step-by-step progress is clear.** Each of the 4 steps shows `✓` on success with a brief result summary. The daemon step correctly notes the Linux-specific behavior ("brew found but using daemon mode").

5. **`doctor` degrades gracefully in blank state.** No crash, no stack trace, just actionable `✗` items with commands to run.

6. **`unused` tier filter bracket notation is clever.** `[SAFE: 5 packages] · MEDIUM: 32 · RISKY: 3 (filtered to safe)` visually indicates which tier is active without hiding the counts of other tiers.

7. **Conflict detection is consistently applied.** `--all`/`--tier`, `--daemon`/`--stop`, `--safe`/`--medium` — all pairs are detected and reported before any action is taken.

8. **`status` distinguishes daemon running vs stopped.** `running (since X minutes, PID N)` vs `stopped (run 'brewprune watch --daemon')` is clear and includes the recovery command inline.

9. **`remove --medium` correctly locks dependency packages.** The 31-package skip list demonstrates the dependency-locking logic works correctly. Even though the list is verbose (see finding), the underlying logic is sound.

10. **`undo` post-restore warning is timely.** "Run 'brewprune scan' to update the package database before running 'brewprune remove'" appears immediately after restore, preventing a stale-database remove cycle.

---

## Recommendations Summary

### Priority 1: Fix (UX-critical)

1. **Daemon event pipeline drops most shim events.** Only 1 of 5+ shimmed invocations appears in the database. This is the core feature of the tool. Investigate the `usage.offset` tracking — the offset may be advancing past all events on the first flush rather than per-event. This is the most important bug in the tool.

2. **Doctor pipeline failure message blames the daemon, not PATH.** When the manual setup path fails the pipeline test (because shims are not in the active shell PATH), doctor says "Run 'brewprune watch --daemon' to restart the daemon." The daemon is running fine; the real fix is `source ~/.profile`. Fix the error message to diagnose PATH as the likely cause.

3. **Quickstart success/warning sequencing.** "Setup complete!" followed by "TRACKING IS NOT ACTIVE YET" creates a false sense of completion. Reorder or integrate the PATH warning into the success message.

### Priority 2: Improve (UX-improvement)

4. **Consolidate the double warning banner in `unused` (no data state).** Merge the two overlapping warning blocks into one.

5. **`status` with no database shows misleading "Last scan: just now · 0 formulae."** Show "never" for last scan when the database does not exist.

6. **`doctor` PATH check conflates "in profile" with "in active shell."** Add a separate check against the actual `$PATH` environment variable.

7. **`stats --package` shows zero usage for packages that were shimmed.** Same root cause as finding #1 (pipeline event loss), but surfaces here as a user-visible discrepancy.

8. **`remove --medium --dry-run` skipped packages list prints before the action table.** Move or collapse the skipped list.

9. **`explain` recommendation for MEDIUM packages suggests `remove <package>` without `--dry-run`.** Add `--dry-run` to the suggested command.

10. **Unknown subcommands should list valid alternatives.** Add a short list of valid commands to the "unknown command" error.

### Priority 3: Polish (UX-polish)

11. **Footer `(risky, hidden)` shown when risky tier IS visible.** Fix the footer to not say "hidden" when the risky tier is being displayed due to the no-data fallback.

12. **`remove --safe --medium --risky` only reports first two conflicting flags.** Report all three in the error.

13. **`undo --list` missing Size column.** Add disk size of removed packages to the snapshot list.

14. **`explain` MEDIUM recommendation missing `--dry-run`.** See Priority 2 item 9.

15. **Doctor "Tip" about aliases appears in all states including critical failure.** Suppress the tip during critical failures.

16. **`stats --all` table has no documented sort order.** Add a sort indicator or note in the output header.

17. **`--sort age` direction is undocumented.** Note sort direction in output or `--help`.

18. **`remove bat fd --dry-run` "explicitly installed" warning is confusing** when the user named the package explicitly.

---

## Appendix: Command Execution Log

All commands executed in order, with exit codes.

| Command | Exit Code | Notes |
|---------|-----------|-------|
| `brewprune --help` | 0 | Full help shown |
| `brewprune --version` | 0 | Version string |
| `brewprune -v` | 0 | Same as --version |
| `brewprune help` | 0 | Identical to --help |
| `brewprune` (no args) | 0 | Identical to --help |
| `brewprune completion --help` | 0 | Subcommand list shown |
| `brewprune doctor --help` | 0 | Check list shown |
| `brewprune explain --help` | 0 | Usage shown |
| `brewprune quickstart --help` | 0 | 4 steps described |
| `brewprune remove --help` | 0 | Full flag docs |
| `brewprune scan --help` | 0 | Full flag docs |
| `brewprune stats --help` | 0 | Frequency classification explained |
| `brewprune status --help` | 0 | Field list shown |
| `brewprune undo --help` | 0 | Snapshot usage shown |
| `brewprune unused --help` | 0 | Score formula + tier logic |
| `brewprune watch --help` | 0 | 30s polling mentioned |
| `brewprune quickstart` | 0 | 4 steps + PATH warning |
| `brewprune status` (post-quickstart) | 0 | Running, 1 event |
| `ls /root/.brewprune/` | 2 | Permission denied (wrong user) |
| `ls /home/brewuser/.brewprune/` | 0 | 10 files shown |
| `cat watch.log` | 0 | Daemon start line |
| `brewprune doctor` (post-quickstart) | 0 | 1 warning (PATH) |
| `rm -rf /home/brewuser/.brewprune` | 0 | State reset |
| `brewprune scan` | 0 | 40 packages, 222 shims |
| `brewprune status` (post-scan) | 0 | Stopped, 0 events |
| `brewprune watch --daemon` | 0 | Daemon started |
| `brewprune status` (post-daemon) | 0 | Running, 0 events, warning |
| `ls /home/brewuser/.brewprune/` | 0 | 6 files (no usage.log yet) |
| `cat watch.pid` | 0 | PID printed |
| `brewprune doctor` (manual path) | 1 | Pipeline test FAIL |
| `brewprune unused` (default) | 0 | Double warning, all 40 shown |
| `brewprune unused --all` | 0 | All 40, no double warning |
| `brewprune unused --tier safe` | 0 | 5 packages |
| `brewprune unused --tier medium` | 0 | 32 packages |
| `brewprune unused --tier risky` | 0 | 3 packages |
| `brewprune unused --min-score 70` | 0 | 6 packages shown |
| `brewprune unused --min-score 50` | 0 | 37 packages shown |
| `brewprune unused --sort score` | 0 | Score-ordered |
| `brewprune unused --sort size` | 0 | Size-ordered |
| `brewprune unused --sort age` | 0 | Install-date ordered |
| `brewprune unused --casks` | 0 | "No casks found" |
| `brewprune unused --verbose` | 0 | All 40 packages expanded |
| `brewprune unused --tier safe --verbose` | 0 | 5 packages expanded |
| `brewprune unused --tier safe --all` | 1 | Conflict error |
| `brewprune unused --all --tier medium` | 1 | Conflict error |
| `brewprune watch --daemon` (already running) | 0 | "Already running" |
| `cat watch.pid` | 0 | PID 2478 |
| `brewprune status` (daemon running) | 0 | Running, 0 events, warning |
| `shim: /home/brewuser/.brewprune/bin/git --version` | 0 | git 2.53.0 |
| `shim: /home/brewuser/.brewprune/bin/jq --version` | 0 | jq-1.8.1 |
| `shim: /home/brewuser/.brewprune/bin/bat --version` | 0 | bat 0.26.1 |
| `shim: /home/brewuser/.brewprune/bin/fd --version` | 0 | fd 10.3.0 |
| `shim: /home/brewuser/.brewprune/bin/rg --version` | 0 | ripgrep 15.1.0 |
| `sleep 35` | 0 | Waited for polling cycle |
| `cat usage.log` | 0 | 6 entries in log |
| `brewprune status` (post-poll) | 0 | 1 event (not 6) |
| `brewprune stats` | 0 | Only ripgrep shown |
| `brewprune stats --days 1/7/90` | 0 | Same result |
| `brewprune stats --package git` | 0 | Total Uses: 0 (wrong) |
| `brewprune stats --package jq` | 0 | Total Uses: 0 (wrong) |
| `brewprune stats --all` | 0 | Only ripgrep=1, rest 0 |
| `brewprune watch --stop` | 0 | Daemon stopped |
| `brewprune status` (post-stop) | 0 | Stopped |
| `ls /home/brewuser/.brewprune/` | 0 | No watch.pid |
| `brewprune explain git` | 0 | Score 70, MEDIUM |
| `brewprune explain jq` | 0 | Score 80, SAFE |
| `brewprune explain bat` | 0 | Score 80, SAFE |
| `brewprune explain openssl@3` | 0 | Score 40, RISKY |
| `brewprune explain curl` | 0 | Score 60, MEDIUM |
| `brewprune explain nonexistent-package` | 1 | Helpful error |
| `brewprune explain` (no args) | 1 | Usage error |
| `brewprune doctor` (stopped daemon) | 0 | 2 warnings, pipeline skipped |
| `rm -rf /home/brewuser/.brewprune` | 0 | State reset |
| `brewprune doctor` (blank state) | 1 | 2 critical, 1 warning |
| `brewprune quickstart` (restore) | 0 | All 4 steps pass |
| `brewprune remove --safe --dry-run` | 0 | 5 packages previewed |
| `brewprune remove --medium --dry-run` | 0 | 31 skipped, 5 candidates |
| `brewprune remove --risky --dry-run` | 0 | 34 skipped, 6 candidates |
| `brewprune remove --tier safe --dry-run` | 0 | Same as --safe |
| `brewprune remove bat fd --dry-run` | 0 | 2 packages, "explicitly installed" |
| `brewprune undo --list` (pre-removal) | 0 | "No snapshots available" |
| `brewprune remove --safe --yes` | 0 | 5 removed, Snapshot ID 1 |
| `brewprune undo --list` (post-removal) | 0 | 1 snapshot shown |
| `brewprune status` (post-removal) | 0 | 35 formulae |
| `brewprune undo latest --yes` | 0 | 5 restored |
| `brewprune undo --list` (post-undo) | 0 | Snapshot still listed |
| `brewprune remove nonexistent-package` | 1 | Clear error |
| `brewprune remove --safe --medium` | 1 | Conflict error |
| `brewprune undo 999` | 1 | "not found" error |
| `brewprune undo` (no args) | 1 | Usage error with hint |
| `brewprune` (no args) | 0 | Help shown |
| `brewprune unused` (no args) | 0 | (see Area 3) |
| `brewprune stats` (no args) | 0 | Default 30d |
| `brewprune remove` (no args) | 1 | Usage error |
| `brewprune explain` (no args) | 1 | Usage error |
| `brewprune undo` (no args) | 1 | Usage error |
| `brewprune blorp` | 1 | Unknown command |
| `brewprune list` | 1 | Unknown command |
| `brewprune prune` | 1 | Unknown command |
| `brewprune unused --invalid-flag` | 1 | Unknown flag |
| `brewprune unused --tier invalid` | 1 | Valid values listed |
| `brewprune unused --min-score 200` | 1 | Range error |
| `brewprune unused --sort invalid` | 1 | Valid values listed |
| `brewprune stats --days -1` | 1 | Positive integer required |
| `brewprune stats --days abc` | 1 | Positive integer required |
| `brewprune remove --safe --medium --risky` | 1 | Conflict (only --safe+--medium reported) |
| `brewprune remove --safe --tier medium` | 1 | Conflict error |
| `brewprune unused --tier safe --all` | 1 | Conflict error |
| `brewprune watch --daemon --stop` | 1 | Conflict error |
| `brewprune unused` (no database) | 1 | DB error with scan hint |
| `brewprune stats` (no database) | 1 | DB error with scan hint |
| `brewprune remove --safe` (no database) | 1 | DB error with scan hint |
| `brewprune status` (no database) | 0 | Misleading "last scan: just now" |
