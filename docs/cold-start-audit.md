# brewprune Cold-Start UX Audit

**Date:** 2026-02-28
**Environment:** Docker container `bp-audit3`, brewprune installed at `/home/linuxbrew/.linuxbrew/bin/brewprune`
**Auditor role:** New user, first encounter with the tool

---

## Summary Table

| Severity        | Count |
|-----------------|-------|
| UX-critical     | 4     |
| UX-improvement  | 9     |
| UX-polish       | 6     |
| **Total**       | **19** |

---

## Area 1: Discovery

### [DISCOVERY] `brewprune` with no args exits 0
- **Severity:** UX-improvement
- **What happens:** Running `brewprune` with no arguments prints the help text and exits with code `0`. Most CLI tools exit non-zero when invoked with no meaningful input, to signal "nothing was done."
- **Expected:** Exit code `1` (or at minimum, a nudge like "run `brewprune quickstart` to get started") so that scripts can detect accidental bare invocations.
- **Repro:** `docker exec bp-audit3 brewprune; echo $?` — prints help, exits `0`

---

### [DISCOVERY] `doctor --help` omits the `--fix` flag that users will try
- **Severity:** UX-critical
- **What happens:** The `doctor --help` output lists only `-h/--help`. There is no `--fix` flag implemented, yet user intuition from tools like `npm doctor --fix` leads users to try `brewprune doctor --fix`, which returns `Error: unknown flag: --fix` with exit code `1`.
- **Expected:** Either implement `--fix` (which could auto-write the PATH export to the shell config, restart the daemon, etc.) or explicitly note in help text that doctor is read-only and link to `quickstart` for remediation.
- **Repro:** `docker exec bp-audit3 brewprune doctor --fix` — `Error: unknown flag: --fix`

---

### [DISCOVERY] Unknown subcommand error message has awkward word order
- **Severity:** UX-polish
- **What happens:** `brewprune blorp` outputs:
  ```
  Run 'brewprune --help' for a list of available commands.
  Error: unknown command "blorp" for "brewprune"
  ```
  The helpful hint appears *before* the error message, which reads backwards. No "did you mean?" suggestion is offered.
- **Expected:** Error first, then the hint. Standard CLI convention is `Error: ...` followed by the hint.
- **Repro:** `docker exec bp-audit3 brewprune blorp`

---

### [DISCOVERY] `scan --help` exposes an internal post-install-hook detail as a user-facing example
- **Severity:** UX-polish
- **What happens:** The `scan --help` example section includes:
  ```
  # Fast path: refresh shims only (used by post_install hook)
  brewprune scan --refresh-shims
  ```
  "used by post_install hook" is an internal implementation detail that a new user will find confusing.
- **Expected:** Either remove this example from user-facing help or move it to an "Advanced" section.
- **Repro:** `docker exec bp-audit3 brewprune scan --help`

---

## Area 2: Setup / Onboarding

### [SETUP] `quickstart` prints a full 40-row package table mid-flow
- **Severity:** UX-improvement
- **What happens:** During `quickstart`, Step 1 prints the entire package inventory (all 40 packages with size, installed time, last used). This creates a wall of output that buries the subsequent steps and the critical PATH instruction.
- **Expected:** Step 1 should emit only a one-line summary (`Scan complete: 40 packages, 352 MB`). The package table belongs in `brewprune scan` or `brewprune unused`, not in the onboarding wizard.
- **Repro:** `docker exec bp-audit3 brewprune quickstart`

---

### [SETUP] PATH instruction appears twice with contradictory framing
- **Severity:** UX-improvement
- **What happens:** After the scan step, a `⚠` block warns "add shim directory to PATH." Then Step 2 reports `✓ Added /home/brewuser/.brewprune/bin to PATH in /home/brewuser/.profile` followed by another "Restart your shell" instruction. The user sees two different PATH messages that appear to contradict each other (one warns, one says it's done).
- **Expected:** A single clear PATH message at the end of the flow, explaining that the config file was updated but the current shell session needs a restart. The in-scan warning should be suppressed during `quickstart`.
- **Repro:** `docker exec bp-audit3 brewprune quickstart`

---

### [SETUP] `status` shows "PATH missing" warning immediately after successful `quickstart`
- **Severity:** UX-improvement
- **What happens:** After running `quickstart`, `status` shows:
  ```
  Shims: active · 222 commands · PATH missing ⚠
         Note: events are from setup self-test, not real shim interception.
  ```
  This is technically correct (the current shell session hasn't sourced the updated profile), but to a new user who just ran `quickstart` successfully, seeing `PATH missing` looks like setup failed.
- **Expected:** `status` should distinguish between "PATH not yet in effect in current session" and "PATH was never configured." If the profile file already contains the export, status should say something like `PATH configured (restart shell to activate)` rather than the alarming `PATH missing ⚠`.
- **Repro:** `docker exec bp-audit3 brewprune quickstart && docker exec bp-audit3 brewprune status`

---

### [SETUP] Re-running `scan` after `quickstart` reprints the full package table
- **Severity:** UX-polish
- **What happens:** Running `brewprune scan` a second time reprints all build messages (`Building shim binary...`, `Generating PATH shims...`) and the full 40-package table, even when nothing has changed.
- **Expected:** On a re-scan with no changes detected, output should be terse: `✓ Database up to date (40 packages, 0 changes)`. Verbose output should only appear on first scan or with an explicit flag.
- **Repro:** `docker exec bp-audit3 brewprune scan` (after quickstart has already run)

---

## Area 3: Core Feature — Unused

### [UNUSED] `--sort age` returns packages in no visible order
- **Severity:** UX-improvement
- **What happens:** `brewprune unused --sort age` returns packages in what appears to be random order. All packages were installed at the same time in this environment, so there is no meaningful age difference — but the output gives no indication of this. A user who types `--sort age` expecting a meaningful sort gets a shuffled list with no explanation.
- **Expected:** When all packages share the same install timestamp, note "all packages installed at the same time — age sort has no effect" or fall back to a secondary sort (e.g., score or name). The sort direction (newest-first or oldest-first) should also be documented or indicated in the column header.
- **Repro:** `docker exec bp-audit3 brewprune unused --sort age`

---

### [UNUSED] `--min-score 70` silently suppresses risky tier without explanation
- **Severity:** UX-improvement
- **What happens:** `brewprune unused --min-score 70` shows only the 5 safe-tier packages (score 80), with no medium-tier packages (scores 50-65) visible. The risky tier is hidden by the separate `--all` rule, not by `--min-score`. The footer does not explain the interaction between these two filters.
- **Expected:** The footer should clarify: "Showing packages with score >= 70. X packages below threshold hidden. Risky tier also hidden (use --all to include)." The interaction between `--min-score` and risky-tier suppression should be surfaced.
- **Repro:** `docker exec bp-audit3 brewprune unused --min-score 70`

---

### [UNUSED] Verbose `-v` with no tier filter dumps hundreds of lines without warning
- **Severity:** UX-polish
- **What happens:** `brewprune unused -v` prints a full detailed breakdown for all 36 non-risky packages — hundreds of lines — with no paging, no prompt, and no truncation.
- **Expected:** Either recommend `brewprune unused --tier safe -v` in the flag description (the help already shows this example, but the default behavior remains overwhelming), or warn before rendering if the output will exceed ~30 packages.
- **Repro:** `docker exec bp-audit3 brewprune unused -v`

---

### [UNUSED] Summary header tier labels (`SAFE`, `MEDIUM`, `RISKY`) are not color-coded
- **Severity:** UX-polish
- **What happens:** The table rows use colored symbols (`✓ safe` in green, `~ review` in yellow, `⚠ risky` in red), but the summary header line (`SAFE: 5 packages · MEDIUM: 31 · RISKY: 4`) uses plain text with no color.
- **Expected:** The summary header tier labels should use the same color coding as the table rows for visual consistency.
- **Repro:** `docker exec bp-audit3 brewprune unused`

---

## Area 4: Tracking / Daemon

### [DAEMON] `stats --package git` crashed with exit 139 (SIGSEGV) on first invocation
- **Severity:** UX-critical
- **What happens:** The first call to `brewprune stats --package git` (immediately after daemon start) returned exit code `139` (segmentation fault) with no output. A subsequent call succeeded normally.
- **Expected:** `stats --package` should never crash. If this is a timing issue (daemon still processing), the command should wait briefly and retry, or output a message like "Usage data still being processed — try again in a few seconds."
- **Repro:** `docker exec bp-audit3 brewprune stats --package git` immediately after `brewprune watch --daemon`

---

### [DAEMON] `stats` default output hides 39 of 40 packages without prominent notice
- **Severity:** UX-improvement
- **What happens:** `brewprune stats` shows only the 1 package with usage data (git), and the footer reads: `(39 packages with no recorded usage hidden — use --all to show)`. This note is easy to miss. A new user may think only git is installed.
- **Expected:** Surface the hidden count more prominently, e.g., as a banner line before the table: `Showing 1 of 40 packages (39 with no recorded usage — use --all to see all)`.
- **Repro:** `docker exec bp-audit3 brewprune stats`

---

### [DAEMON] `stats --package git` output omits trend detail and scoring context
- **Severity:** UX-improvement
- **What happens:** `brewprune stats --package git` outputs a terse 6-line block:
  ```
  Package: git
  Total Uses: 2
  Last Used: 2026-02-28 06:44:10
  Days Since: 0
  First Seen: 2026-02-28 03:08:52
  Frequency: daily
  ```
  There is no per-day breakdown, no mention of whether the single event was from the shim or the self-test pipeline, and no link to `brewprune explain git` for scoring context.
- **Expected:** Per-package stats should clarify the data source (shim interception vs. self-test), include a usage-over-time breakdown even if sparse, and suggest `brewprune explain <package>` for the full scoring picture.
- **Repro:** `docker exec bp-audit3 brewprune stats --package git`

---

## Area 5: Explain

### [EXPLAIN] The "Usage: 0/40 means recently used" note is confusingly worded
- **Severity:** UX-improvement
- **What happens:** The `explain` output includes:
  ```
  Note: Higher removal score = more confident to remove.
        Usage: 0/40 means recently used (lower = keep this package).
  ```
  This note appears for every package. The phrase "0/40 means recently used" is technically correct but confusing because git (which *was* used) and packages that were *never* used both show `0/40` for different reasons. The scoring direction is counter-intuitive without clear explanation.
- **Expected:** Rewrite as: "The usage component scores removal confidence from observed activity. 0/40 means the package was recently used (fewer points toward removal). 40/40 means no usage was ever observed."
- **Repro:** `docker exec bp-audit3 brewprune explain git`

---

### [EXPLAIN] `explain nonexistent` suggests running `scan` even when scan cannot help
- **Severity:** UX-polish
- **What happens:** `brewprune explain nonexistent` returns:
  ```
  Error: package not found: nonexistent
  Run 'brewprune scan' to update package database
  ```
  `scan` cannot help if the package is genuinely not installed. The suggestion is misleading for the common case of a typo.
- **Expected:** "Package 'nonexistent' is not installed or not in the database. If you recently installed it, run 'brewprune scan' to update the index."
- **Repro:** `docker exec bp-audit3 brewprune explain nonexistent`

---

## Area 6: Diagnostics

### [DOCTOR] `doctor --fix` is not implemented but users will expect it
- **Severity:** UX-critical
- **What happens:** `brewprune doctor --fix` returns `Error: unknown flag: --fix` with exit code `1`. The `doctor --help` page does not mention `--fix` at all, but users coming from tools like `npm doctor` or `go doctor` will attempt it.
- **Expected:** Either implement `--fix` with at minimum the PATH remediation step (re-write the profile export and instruct the user to source it), or add a note to `doctor --help`: "To fix issues, re-run 'brewprune quickstart'."
- **Repro:** `docker exec bp-audit3 brewprune doctor --fix`

---

### [DOCTOR] `doctor` exits with code 2 for warnings; scripts expect 0 or 1
- **Severity:** UX-improvement
- **What happens:** `brewprune doctor` exits with code `2` when the only finding is a PATH warning (non-blocking). Exit code `2` conventionally means "misuse of shell built-in" in POSIX tools and is unexpected here.
- **Expected:** Use exit code `0` for "all clear," exit code `1` for "issues found" (warnings or errors). If a warning-vs-error distinction is needed, document it in `doctor --help`.
- **Repro:** `docker exec bp-audit3 brewprune doctor; echo $?` — prints `2`

---

## Area 7: Remove (Dry-Run)

No findings. All tested behaviors were correct:

- `--dry-run` clearly labeled: `Dry-run mode: no packages will be removed.`
- Both `--safe` and `--tier safe` produced identical output (consistent behavior, documented equivalence).
- `remove nonexistent --dry-run` returned `Error: package "nonexistent" not found` with exit code `1`.
- Summary block showed package count, disk space freed, and snapshot notice.

---

## Area 8: Undo

### [UNDO] `undo` (no args) exits 1 but `undo --list` (empty) exits 0 — inconsistent
- **Severity:** UX-polish
- **What happens:** `brewprune undo` with no arguments exits `1` with a usage error. `brewprune undo --list` with no snapshots available exits `0`. The asymmetry in exit codes for similar "no snapshots" states is inconsistent.
- **Expected:** `undo` with no args could exit `0` and print usage guidance (since no action failed), or the exit codes should be consistently documented.
- **Repro:** `docker exec bp-audit3 brewprune undo; echo $?` — `1`
  `docker exec bp-audit3 brewprune undo --list; echo $?` — `0`

---

### [UNDO] `undo latest` error does not suggest `--list` as a next step
- **Severity:** UX-polish
- **What happens:** `brewprune undo latest` returns:
  ```
  Error: no snapshots available.

  Snapshots are automatically created before package removal.
  Use 'brewprune remove' to remove packages and create snapshots.
  ```
  The message is clear but does not suggest `undo --list` for users who expect prior snapshots to exist.
- **Expected:** Add: "Run 'brewprune undo --list' to see all available snapshots."
- **Repro:** `docker exec bp-audit3 brewprune undo latest`

---

## Area 9: Edge Cases

All four invalid-input cases produced user-friendly validation errors with exit code `1`. However:

### [EDGE] Tier validation error messages have inconsistent phrasing across commands
- **Severity:** UX-polish
- **What happens:**
  - `unused --tier invalid` → `Error: invalid tier: invalid (must be safe, medium, or risky)` — value unquoted, colon separator
  - `remove --tier invalid` → `Error: invalid tier "invalid": must be safe, medium, or risky` — value quoted, different structure
- **Expected:** Standardize to one format across all commands, e.g.: `Error: invalid --tier value "invalid": must be one of: safe, medium, risky`
- **Repro:** Compare `docker exec bp-audit3 brewprune unused --tier invalid` vs `docker exec bp-audit3 brewprune remove --tier invalid --dry-run`

---

## Appendix: All Commands and Exit Codes

| Command | Exit Code |
|---------|-----------|
| `brewprune --help` | 0 |
| `brewprune scan --help` | 0 |
| `brewprune unused --help` | 0 |
| `brewprune remove --help` | 0 |
| `brewprune watch --help` | 0 |
| `brewprune explain --help` | 0 |
| `brewprune stats --help` | 0 |
| `brewprune doctor --help` | 0 |
| `brewprune undo --help` | 0 |
| `brewprune status --help` | 0 |
| `brewprune quickstart --help` | 0 |
| `brewprune quickstart` | 0 |
| `brewprune scan` | 0 |
| `brewprune status` | 0 |
| `brewprune unused` | 0 |
| `brewprune unused --tier safe` | 0 |
| `brewprune unused --tier medium` | 0 |
| `brewprune unused --tier risky` | 0 |
| `brewprune unused --all` | 0 |
| `brewprune unused --sort size` | 0 |
| `brewprune unused --sort age` | 0 |
| `brewprune unused --min-score 70` | 0 |
| `brewprune unused -v` | 0 |
| `brewprune unused --casks` | 0 |
| `brewprune watch --daemon` | 0 |
| `brewprune status` (post-daemon) | 0 |
| `brewprune stats` | 0 |
| `brewprune stats --package git` (1st attempt) | 139 (SIGSEGV crash) |
| `brewprune stats --package git` (2nd attempt) | 0 |
| `brewprune stats --package nonexistent` | 1 |
| `brewprune explain git` | 0 |
| `brewprune explain jq` | 0 |
| `brewprune explain nonexistent` | 1 |
| `brewprune explain` (no args) | 1 |
| `brewprune doctor` | 2 |
| `brewprune doctor --fix` | 1 |
| `brewprune remove --safe --dry-run` | 0 |
| `brewprune remove --tier safe --dry-run` | 0 |
| `brewprune remove nonexistent --dry-run` | 1 |
| `brewprune undo` (no args) | 1 |
| `brewprune undo latest` (no snapshots) | 1 |
| `brewprune undo --list` (no snapshots) | 0 |
| `brewprune` (no args) | 0 |
| `brewprune blorp` | 1 |
| `brewprune unused --tier invalid` | 1 |
| `brewprune remove --tier invalid --dry-run` | 1 |
| `brewprune unused --sort invalid` | 1 |
