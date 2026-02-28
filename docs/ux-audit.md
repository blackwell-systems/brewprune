# brewprune UX Audit

**Date:** 2026-02-28
**Auditor:** Claude (acting as new user)
**Environment:** Docker container `bp-sandbox` with Homebrew packages: jq, ripgrep, fd, bat, git, curl, tmux
**Scope:** Full CLI surface — discovery, setup, analysis, tracking, explanation, diagnostics, removal, edge cases

---

## Summary

| Severity | Count |
|---|---|
| UX-critical | 5 |
| UX-improvement | 9 |
| UX-polish | 7 |
| **Total** | **21** |

---

## DISCOVERY

### [DISCOVERY] `brewprune` with no args shows full help instead of a brief usage hint

- **Severity**: UX-improvement
- **What happens**: Running `brewprune` with no arguments outputs the exact same full help text as `brewprune --help`. A new user gets the full wall of text immediately, which can be overwhelming and doesn't orient them toward the quickstart path.
- **Expected**: A short "you haven't set up yet" nudge (e.g., "Run `brewprune quickstart` to get started") with a pointer to `--help` for the full reference. Many CLIs show a short usage summary when invoked bare, not the full man-page equivalent.
- **Repro**: `brewprune`

---

### [DISCOVERY] `brewprune blorp` (unknown subcommand) gives no suggestion

- **Severity**: UX-improvement
- **What happens**: `Error: unknown command "blorp" for "brewprune"` — terse, no hint about what valid commands are or whether there is a "did you mean X?" alternative.
- **Expected**: Print the list of available subcommands (or "did you mean: ...?") below the error, similar to how git handles unknown commands.
- **Repro**: `brewprune blorp`

---

### [DISCOVERY] `brewprune explain` with no argument gives a cryptic arity error

- **Severity**: UX-polish
- **What happens**: `Error: accepts 1 arg(s), received 0` — generic cobra framework message that doesn't tell the user what argument is expected.
- **Expected**: `Error: missing package name. Usage: brewprune explain <package>`
- **Repro**: `brewprune explain`

---

### [DISCOVERY] `remove --help` and `unused --help` use inconsistent flag styles for tier selection

- **Severity**: UX-improvement
- **What happens**: `brewprune unused` uses `--tier safe|medium|risky` (a single flag with a value), while `brewprune remove` uses `--safe`, `--medium`, `--risky` (three separate boolean flags). The commands operate on the same concept — tier — but expose it completely differently.
- **Expected**: Consistent flag design. Either both use `--tier <value>` or both use `--safe/--medium/--risky`. The inconsistency is a trap: new users who learn `--tier` from `unused` will immediately run `brewprune remove --tier medium --dry-run` and get `Error: unknown flag: --tier`.
- **Repro**: `brewprune unused --help` vs `brewprune remove --help`, then `brewprune remove --tier medium --dry-run`

---

## SETUP FLOW

### [SETUP] Scan spinner renders as garbage in non-TTY / piped contexts

- **Severity**: UX-critical
- **What happens**: When `brewprune scan` is captured (e.g., piped to a file, run in CI, or run inside `quickstart`), the spinner animation produces unescaped carriage-return noise: `|  Discovering packages.../  Discovering packages...-  Discovering packages...\  Discovering packages...` — all on one line with the animation frames concatenated. This appears both in `brewprune scan` standalone and as sub-output within `brewprune quickstart`.
- **Expected**: Detect non-TTY and suppress the spinner, printing a single static line like `Discovering packages...` instead.
- **Repro**: `docker exec bp-sandbox brewprune scan 2>&1 | cat`

---

### [SETUP] Scan outputs "0 command shims created" on subsequent runs

- **Severity**: UX-improvement
- **What happens**: After the first `brewprune scan` creates 222 shims, re-running scan (e.g., as part of `quickstart`) reports `✓ 0 command shims created`. This makes it look like something failed or that scan was a no-op, which is alarming for a new user.
- **Expected**: On re-scan when shims already exist and nothing changed, report something like `✓ 222 shims up to date` rather than `0 created`.
- **Repro**: Run `brewprune scan` twice in sequence.

---

### [SETUP] `status` suggests `brew services start brewprune` even on Linux/Docker where brew services won't work

- **Severity**: UX-improvement
- **What happens**: `Tracking: stopped (run 'brew services start brewprune')` — on Linux (and in most CLI-only environments) `brew services` does not work. The correct command is `brewprune watch --daemon`. This sends new users down a dead-end path.
- **Expected**: Show `run 'brewprune watch --daemon'` as the suggested command. Possibly detect whether `brew services` is available before recommending it.
- **Repro**: `brewprune status` (after daemon is stopped)

---

### [SETUP] `quickstart` runs brew services and silently falls back — but still prints alarming `⚠` lines

- **Severity**: UX-improvement
- **What happens**: `quickstart` attempts `brew services start brewprune`, that fails, it falls back to `brewprune watch --daemon`, but then also reports `⚠ Could not start daemon: daemon already running`. The "daemon already running" error is benign (quickstart ran after the user had already started the daemon) but the output is alarming. A new user running quickstart for the first time after manually starting the daemon would see two `⚠` lines in the step that should be the smoothest part.
- **Expected**: If the daemon is already running, that's a success state — emit `✓ Daemon already running` rather than a warning.
- **Repro**: `brewprune watch --daemon` then `brewprune quickstart`

---

### [SETUP] `watch --daemon` fails with error when daemon is already running, instead of no-oping

- **Severity**: UX-improvement
- **What happens**: `Error: daemon already running (PID file: ...)` and exits with code 1. For idempotent operations in a setup flow, an error exit is unnecessarily disruptive.
- **Expected**: Exit 0 with a message like `Daemon already running (PID XXXX). Nothing to do.`
- **Repro**: `brewprune watch --daemon` (second invocation while daemon is running)

---

## CORE ANALYSIS

### [ANALYSIS] `unused` (no flags) shows nothing and the reason is buried in the warning

- **Severity**: UX-critical
- **What happens**: Before any usage data is recorded, `brewprune unused` with no flags shows a full warning block, then a summary line `SAFE: 0 · MEDIUM: 0 · RISKY: 40 (hidden)`, then `No packages match the specified criteria.` A new user's natural first action after running `brewprune scan` is `brewprune unused` — and they see nothing actionable. The reason (no usage data yet, 40 packages hidden in risky) is explained but easy to miss.
- **Expected**: When there is no usage data at all and no tier filter is active, either: (a) show the risky tier by default with a prominent warning banner, or (b) suggest the user run `brewprune unused --all` explicitly and explain why. The current state — zero output rows with a hidden count — reads as "this tool does nothing."
- **Repro**: `brewprune scan` then immediately `brewprune unused`

---

### [ANALYSIS] `unused --casks` with no casks installed produces identical output to `unused` with no casks

- **Severity**: UX-polish
- **What happens**: `brewprune unused --casks` shows the same risky-hidden summary and `No packages match the specified criteria`. There is no message indicating whether casks were actually checked or whether zero casks are installed.
- **Expected**: A distinct message like `No casks installed` or `No casks found matching criteria (0 casks installed)` to confirm the flag was honored.
- **Repro**: `brewprune unused --casks`

---

### [ANALYSIS] Score logic marks recently-used packages as "SAFE" to remove and the explanation contradicts itself

- **Severity**: UX-critical
- **What happens**: After running `jq --version`, `rg --version`, `fd --version` (via shims), all three packages receive score 80 (SAFE). The explain output says:
  - `Usage: 40/40 pts — used today`
  - `Why SAFE: rarely used, safe to remove`
  - `Recommendation: Safe to remove.`

  A package that was used today scores maximum usage points AND is labeled "rarely used, safe to remove." These two statements directly contradict each other. The scoring rubric in `unused --help` says usage accounts for 40 points with "recent activity indicates active use" — meaning high usage score should indicate the package is heavily used and SHOULD NOT be removed, not that it is safe to remove.

  The issue appears to be an inverted interpretation: 40/40 usage points is being treated as "high confidence for removal" when it should mean "this package is actively used, keep it." The safe tier is intended for packages with low confidence to keep (high removal confidence), but the scoring maps "used today" to 40/40 points which then pushes the total into the SAFE removal tier — the opposite of correct behavior.

- **Expected**: A package used today should score very LOW for removal confidence (not high). Usage points should subtract from removal confidence, not add to it. OR the scoring system labels should be inverted at the display layer to clarify: "40/40 pts usage = 0 removal pressure from this dimension."
- **Repro**: Use shimmed packages (`PATH="/home/brewuser/.brewprune/bin:$PATH" jq --version`), wait 35s, then `brewprune explain jq`

---

### [ANALYSIS] `unused --all` "Status" column shows `✗ keep` for every package, even safe-tier ones after usage data is collected

- **Severity**: UX-improvement
- **What happens**: After packages are tracked, `unused --all` shows every package with `✗ keep` in red in the Status column — including packages in the SAFE tier. The `✗ keep` label appears to be misapplied; a SAFE package should show something like `✓ remove` or `SAFE`.
- **Expected**: Status column should reflect the tier: `SAFE` packages show green `✓ safe`, medium show yellow `~ review`, risky show `✗ keep`. Currently even the SAFE packages show `✗ keep` in `--all` mode, which contradicts the summary header saying they are SAFE.
- **Repro**: `brewprune unused --all` (when packages are present in the SAFE tier)

---

## USAGE TRACKING

### [TRACKING] Usage data is 0 for 35+ seconds after shim use if shim dir is not in PATH

- **Severity**: UX-critical
- **What happens**: Running `jq --version` (without the shim dir in PATH) produces no entry in `usage.log`. The daemon never sees the usage. `brewprune stats` shows 0 events for jq, rg, fd. Only after running via `export PATH="/home/brewuser/.brewprune/bin:$PATH"` do events appear. But the PATH setup step is a manual, opt-in step with no enforcement — the scan output warning is easy to miss.
- **Expected**: The PATH setup step is the single most important part of brewprune's setup. It should be impossible to miss. Suggestions: (a) `brewprune scan` should block or loudly error if it detects PATH is not configured correctly after building shims, (b) `quickstart` should verify PATH is active (not just write to `.profile`) before proceeding to the self-test, (c) the self-test in quickstart should fail loudly if events don't appear.
- **Repro**: `brewprune scan`, then `jq --version` (without updating PATH), then wait 35s, then `brewprune stats` — all show 0 events.

---

### [TRACKING] `stats` output is not sorted usefully — packages with usage are buried in the middle

- **Severity**: UX-polish
- **What happens**: `brewprune stats` shows 40 packages. The three used packages (jq, fd, ripgrep) appear at the top in one run but scattered in another. There's no visible sort order documentation in the help text, and the default appears unstable.
- **Expected**: Default sort by `Total Runs` descending, with packages that have never been used grouped at the bottom. Optionally support `--sort` flags like `unused` does.
- **Repro**: `brewprune stats`

---

### [TRACKING] `stats --package jq` output is unstyled/plain compared to all other commands

- **Severity**: UX-polish
- **What happens**: The per-package stats view is a simple key-value dump with no headers, no color, no table formatting:
  ```
  Package: jq
  Total Uses: 1
  Last Used: 2026-02-28 03:43:17
  Days Since: 0
  First Seen: 2026-02-28 03:08:20
  Frequency: daily
  ```
  Every other command uses tables with headers and colored tiers.
- **Expected**: Style this output consistently with the rest of the CLI — at minimum, use a header and color-code the frequency value.
- **Repro**: `brewprune stats --package jq`

---

## EXPLANATION

### [EXPLAIN] `explain` table footer row has misaligned padding — trailing spaces inside border

- **Severity**: UX-polish
- **What happens**: The `SAFE tier` string in the footer row of the explain table appears to have trailing padding that leaves visible whitespace before the `│` border character. This is cosmetically inconsistent with the other cells.
- **Expected**: Consistent padding with no trailing whitespace in table cells.
- **Repro**: `brewprune explain jq`

---

### [EXPLAIN] `explain` for nonexistent package prints the error message twice

- **Severity**: UX-polish
- **What happens**:
  ```
  Error: package not found: nonexistent-package
  Run 'brewprune scan' to update package database

  Error: package not found: nonexistent-package
  Run 'brewprune scan' to update package database
  ```
  The error block is printed twice — once to stdout and once to stderr (or it is emitted twice by the error handler).
- **Expected**: Error message printed exactly once.
- **Repro**: `brewprune explain nonexistent-package`

---

## DIAGNOSTICS

### [DOCTOR] `doctor` exits with code 1 and prints `Error: diagnostics failed` even for non-critical issues

- **Severity**: UX-improvement
- **What happens**: When `doctor` finds any issue (even a non-blocking one like PATH not being configured), it exits with code 1 and appends `Error: diagnostics failed`. This makes it hard to use `doctor` in scripts (can't distinguish "critical failure" from "warnings found"), and the `Error:` prefix makes a routine PATH warning feel like a crash.
- **Expected**: Exit code 1 only for critical issues (database inaccessible, binary missing). Exit code 2 for "issues found but system is functional." Don't prefix warning-level output with `Error:`.
- **Repro**: `brewprune doctor` (with daemon stopped and PATH not set)

---

### [DOCTOR] `doctor` error message is also duplicated

- **Severity**: UX-polish
- **What happens**: Like `explain`, the full doctor output (all check lines + "Found N issue(s)" + "Error: diagnostics failed") is printed twice when captured.
- **Expected**: Output printed exactly once.
- **Repro**: `docker exec bp-sandbox brewprune doctor 2>&1`

---

## REMOVAL

### [REMOVE] `remove --dry-run` "Last Used" column shows `never` even for recently-used packages

- **Severity**: UX-improvement
- **What happens**: After packages have been used (fd, jq, ripgrep all show `1 minute ago` in `unused` output), the `remove --safe --dry-run` table shows `never` in the "Last Used" column for the same packages.
- **Expected**: Consistent `Last Used` data across `unused`, `remove --dry-run`, and `remove --safe` outputs. If the data is tracked, show it everywhere.
- **Repro**: Use shimmed packages, wait for daemon to process, then `brewprune remove --safe --dry-run`

---

### [REMOVE] `remove` with no flags gives no hint about the flag syntax

- **Severity**: UX-polish
- **What happens**: `Error: no tier specified: use --safe, --medium, or --risky` — reasonable message, but doesn't mention that `--dry-run` is available. A new user might not know to preview before removing.
- **Expected**: Include a dry-run hint: `Error: no tier specified. Use --safe, --medium, or --risky. Add --dry-run to preview changes first.`
- **Repro**: `brewprune remove`

---

### [REMOVE] Progress bar during removal renders a duplicate final line

- **Severity**: UX-polish
- **What happens**: The progress bar output during removal ends with two identical 100% lines:
  ```
  [=======================================>] 100% Removing packages
  [=======================================>] 100% Removing packages
  ```
  The bar appears to print the final frame twice before clearing.
- **Expected**: Progress bar completes once cleanly and either clears or is replaced by the success message.
- **Repro**: `brewprune remove --safe --yes`

---

## OUTPUT & STYLE

### [OUTPUT] ANSI escape codes leak as raw text in non-color terminals and captured output

- **Severity**: UX-improvement
- **What happens**: Throughout the CLI, color codes are emitted unconditionally: `[32mSAFE[0m`, `[33mMEDIUM[0m`, `[31m✗ keep[0m`, `[1mPackage: jq[0m`. In environments where color is not supported (CI, log files, `| cat`, terminals with `NO_COLOR` set), these appear as literal escape sequences, making output hard to read.
- **Expected**: Detect TTY (`isatty`) and suppress ANSI codes when output is not a terminal, or respect the `NO_COLOR` environment variable standard.
- **Repro**: `docker exec bp-sandbox brewprune unused --all 2>&1 | cat`

---

### [OUTPUT] `scan` output "NEXT STEP" warning uses a different emoji style than the rest of the CLI

- **Severity**: UX-polish
- **What happens**: The `scan` footer uses `⚠️  NEXT STEP:` (emoji with variation selector and double space) while `watch --daemon` output uses `⚠` (plain Unicode). The `quickstart` output mixes `⚠️` and `⚠` in the same run.
- **Expected**: Consistent warning indicator across all commands.
- **Repro**: `brewprune scan` output footer vs `brewprune watch --daemon` output

---

### [OUTPUT] `scan` output shows "Version" column that is always empty

- **Severity**: UX-polish
- **What happens**: The scan package table has a `Version` column that contains no data for any package. It occupies significant horizontal space and adds visual noise with no information content.
- **Expected**: Either populate the version column from Homebrew metadata, or remove it from the table. An empty column erodes trust in the data completeness.
- **Repro**: `brewprune scan` (look at the package list table — Version column is blank for all 40 packages)

---

## EDGE CASES

### [EDGE] `undo latest` when no snapshots exist exits with code 1 and terse error; `undo --list` gives a friendly message

- **Severity**: UX-polish
- **What happens**: `brewprune undo --list` (no snapshots) gives a friendly: "No snapshots available. Snapshots are automatically created before package removal." But `brewprune undo latest` gives only `Error: no snapshots available` and exits 1. The two paths are inconsistent.
- **Expected**: `undo latest` with no snapshots should give the same friendly message as `undo --list`.
- **Repro**: `brewprune undo latest` (before any removals)

---

### [EDGE] `unused --tier risky` implicitly shows risky tier without `--all`, but `unused` alone does not

- **Severity**: UX-improvement
- **What happens**: `brewprune unused --tier risky` shows all 40 risky packages without needing `--all`, but `brewprune unused` (no flags) hides the risky tier and says `use --all`. A user who tries `--tier risky` gets the data; a user who just runs `unused` does not. This asymmetry is confusing — `--tier risky` functions as an implicit `--all` for the risky tier.
- **Expected**: Document this behavior explicitly in the help text, or unify the behavior: if `--tier` is specified, always show that tier regardless of `--all`. The current implicit behavior should at minimum be called out in `unused --help`.
- **Repro**: Compare `brewprune unused` vs `brewprune unused --tier risky`

---
