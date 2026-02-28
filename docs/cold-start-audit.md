# brewprune Cold-Start UX Audit

**Date:** 2026-02-27
**Auditor:** Cold-start new user (automated audit via Docker sandbox)
**Environment:** Docker container `bp-sandbox`, brewprune installed at `/home/linuxbrew/.linuxbrew/bin/brewprune`

---

## Summary Table

| Severity | Count |
|---|---|
| UX-critical | 5 |
| UX-improvement | 8 |
| UX-polish | 5 |
| **Total** | **18** |

---

## Findings

### Area 1: Discovery

#### [DISCOVERY] `brewprune` with no args exits 0 and shows a minimal prompt
- **Severity**: UX-improvement
- **What happens**: Running `brewprune` alone outputs a 3-line tip block and exits with code 0. There is no help text, no subcommand listing, no one-line description of what the tool does.
- **Expected**: Either show the full `--help` output (the conventional default for CLI tools), or at minimum show the Available Commands table so new users can orient themselves without knowing to run `--help`.
- **Repro**: `docker exec bp-sandbox brewprune`

```
brewprune: Homebrew package cleanup with usage tracking

Tip: Run 'brewprune status' to check tracking status.
     Run 'brewprune unused' to view recommendations.
     Run 'brewprune --help' for all commands.
EXIT:0
```

---

#### [DISCOVERY] Top-level help Quick Start omits `quickstart` command
- **Severity**: UX-polish
- **What happens**: The Quick Start section in `brewprune --help` lists 4 manual steps (`scan`, `watch --daemon`, wait, `unused --tier safe`) but does not mention `brewprune quickstart`, which automates all of those steps.
- **Expected**: The Quick Start section should lead with `brewprune quickstart` as the recommended first step for new users, then show the manual steps for those who want control.
- **Repro**: `docker exec bp-sandbox brewprune --help`

---

#### [DISCOVERY] `doctor --help` documents no flags beyond `-h`; `--fix` is advertised nowhere
- **Severity**: UX-critical
- **What happens**: `doctor --help` lists no `--fix` flag. Running `brewprune doctor --fix` returns `Error: unknown flag: --fix` (exit 1). However, `doctor` output tells users how to fix problems but offers no automated remediation.
- **Expected**: Either implement `--fix` (as the audit expected based on the prompt), or ensure `doctor` help makes it unambiguous that no automation exists. The doctor output says "Fix: add shim directory to PATH before Homebrew" — if there is no `--fix` flag, that word choice misleads users into thinking a fix flag might exist.
- **Repro**: `docker exec bp-sandbox brewprune doctor --fix`

```
Error: unknown flag: --fix
EXIT:1
```

---

#### [DISCOVERY] `undo --help` has non-standard section ordering
- **Severity**: UX-polish
- **What happens**: The `undo --help` page shows Arguments, then Flags, then Examples, then Usage — with `Usage:` appearing near the bottom after `Examples:`. All other help pages show `Usage:` near the top in the standard cobra layout.
- **Expected**: `Usage:` should appear before Examples to match the rest of the help pages and cobra conventions.
- **Repro**: `docker exec bp-sandbox brewprune undo --help`

---

#### [DISCOVERY] `remove` has both `--safe`/`--medium`/`--risky` boolean flags AND a `--tier` string flag doing the same thing
- **Severity**: UX-improvement
- **What happens**: The flags list for `remove` includes `--safe`, `--medium`, `--risky` (boolean shortcuts) and also `--tier string`. Both `--safe` and `--tier safe` produce identical output. There is nothing in the help text explaining the relationship between these two interfaces, which makes the flags section confusing.
- **Expected**: The help text should clarify that `--tier safe` is equivalent to `--safe`, or consolidate to one interface. Alternatively, note that the boolean flags are shortcuts for the corresponding `--tier` value.
- **Repro**: `docker exec bp-sandbox brewprune remove --help`

---

### Area 2: Setup / Onboarding

#### [SETUP] `quickstart` self-test takes up to 35 seconds with no progress indicator
- **Severity**: UX-improvement
- **What happens**: Step 4 prints "Waiting up to 35s for a usage event to appear in the database..." and then hangs silently until the test completes. In the observed run it took approximately 20 seconds with no dots, spinners, or elapsed-time feedback.
- **Expected**: Show a progress indicator (dots, elapsed seconds, or a spinner) during the wait so the user does not think the tool has frozen.
- **Repro**: `docker exec bp-sandbox brewprune quickstart`

```
Step 4/4: Running self-test (tracking verified)
  Waiting up to 35s for a usage event to appear in the database...
  [~20 seconds of silence]
  ✓ Tracking verified — brewprune is working
```

---

#### [SETUP] `scan` shows stale "start watch daemon" warning even when daemon is already running
- **Severity**: UX-improvement
- **What happens**: After `quickstart` has started the daemon, running `brewprune scan` again shows:
  ```
  ⚠ NEXT STEP: Start usage tracking with 'brewprune watch --daemon'
     Wait 1-2 weeks for meaningful recommendations.
  ```
  The daemon is running at this point. The warning is incorrect and will confuse users who have already completed setup.
- **Expected**: `scan` should check whether the daemon is already running and suppress or replace the warning with a confirmation (e.g., "Daemon is running — tracking active").
- **Repro**: `docker exec bp-sandbox brewprune quickstart` then `docker exec bp-sandbox brewprune scan`

---

#### [SETUP] `quickstart` PATH step says "Restart your shell" but quickstart continues immediately without waiting
- **Severity**: UX-polish
- **What happens**: Step 2 writes the PATH export to `.profile` and prints "Restart your shell (or source the config file) for this to take effect." Step 3 then immediately starts the daemon. Since the PATH is not yet active, `doctor` continues to report `Shims: PATH missing` even after `quickstart` succeeds.
- **Expected**: `quickstart` should either activate the PATH in the current session (e.g., by exporting it within the process) or call this out explicitly in its completion message so users understand why `doctor` still warns about PATH.
- **Repro**: `docker exec bp-sandbox brewprune quickstart` → `docker exec bp-sandbox brewprune doctor`

---

#### [SETUP] `quickstart` attempts `brew services start brewprune` before falling back to `watch --daemon`, printing a confusing error
- **Severity**: UX-improvement
- **What happens**: Step 3 of quickstart shows:
  ```
  brew found at /home/linuxbrew/.linuxbrew/bin/brew — running: brew services start brewprune
  ⚠ brew services start failed (exit status 1) — falling back to brewprune watch --daemon
  ```
  A new user sees a failure message during setup, which is alarming even though the fallback succeeds. On Linux, `brew services` is not expected to work.
- **Expected**: Detect the platform (or the `brew services` capability) before attempting it, and skip the `brew services` attempt silently on Linux. If `brew services` is tried and fails, use a less alarming message like "Using daemon mode instead" rather than the word "failed".
- **Repro**: `docker exec bp-sandbox brewprune quickstart`

---

### Area 3: Core Feature: Unused

#### [UNUSED] Score column is absent from the `unused` table
- **Severity**: UX-improvement
- **What happens**: The `unused` table columns are: Package, Size, Uses (7d), Last Used, Depended On, Status. There is no numeric score column. The status column shows "✓ safe", "~ review", or "✗ keep" as the only tier signal.
- **Expected**: Given that the entire scoring system is built around a 0-100 confidence score, and `--min-score` is a supported flag, new users would reasonably expect to see the score in the table. Without it, `--min-score 70` filtering feels opaque — the user has to use `explain` to discover individual package scores.
- **Repro**: `docker exec bp-sandbox brewprune unused` (observe no Score column)

---

#### [UNUSED] `--sort age` produces no visible reordering relative to default
- **Severity**: UX-improvement
- **What happens**: `brewprune unused --sort age` outputs a table in what appears to be a scrambled order, with no grouping by tier visible. In the observed output, packages with identical install times (all 2 hours ago in the sandbox) are sorted in a seemingly arbitrary order (oniguruma, jq, pcre2, bzip2, ripgrep, ...) with safe and medium packages interleaved.
- **Expected**: When sorting by age and packages have identical ages (or all say "installed today"), the output should fall back to a secondary sort (e.g., name) to produce stable, readable output. The tier grouping that exists in the default view disappears under `--sort age`, which removes a useful navigational cue.
- **Repro**: `docker exec bp-sandbox brewprune unused --sort age`

---

#### [UNUSED] `unused --tier risky` shows packages with "✗ keep" status — tier label and status label contradict each other
- **Severity**: UX-critical
- **What happens**: `brewprune unused --tier risky` shows four packages (zlib-ng-compat, ncurses, openssl@3, git) all with a `✗ keep` status. The user explicitly asked for risky packages to evaluate removal, but the status column says "keep" for all of them. There is no explanation for why packages in the "risky" tier are shown as "keep".
- **Expected**: The risky tier should be clearly explained as meaning "we recommend against removing these" rather than "these are risky candidates for removal." Alternatively, rename the tier. The current name "risky" implies risk to the user (risky to keep? risky to remove?), but the status "keep" directly contradicts any removal intent. The user calling `--tier risky` is trying to see what they might remove at their own risk, not a list of "keep" items.
- **Repro**: `docker exec bp-sandbox brewprune unused --tier risky`

```
Package          Status
────────────────────────────────────────────
zlib-ng-compat   ✗ keep
ncurses          ✗ keep
openssl@3        ✗ keep
git              ✗ keep
```

---

#### [UNUSED] `--min-score 70` is not documented to explain what scores are available
- **Severity**: UX-polish
- **What happens**: `brewprune unused --min-score 70` works and shows only the 5 safe-tier packages (all scored 80). However, there is no way to discover from the `unused` table what a given package's score actually is (the score column is absent). A user must know to run `explain` per-package to find scores.
- **Expected**: Either add a Score column to the `unused` table, or document in `--min-score` help that scores can be viewed with `brewprune explain <package>`.
- **Repro**: `docker exec bp-sandbox brewprune unused --min-score 70`

---

### Area 4: Tracking / Daemon

#### [TRACKING] `status` shows `Shims: PATH missing` but `Data quality: COLLECTING` — the contradiction is not explained
- **Severity**: UX-improvement
- **What happens**: `brewprune status` simultaneously shows:
  ```
  Shims:        active · 222 commands · PATH missing ⚠
  Data quality: COLLECTING (0 of 14 days)
  ```
  And the daemon is recording events (1 total). A new user will be confused: if PATH is missing, how is tracking working? (The self-test injected a synthetic event.) There is no explanation.
- **Expected**: Add a note clarifying that the 1 event is from the setup self-test and not from real shim interception, so the user understands they still need to fix PATH for actual tracking to work.
- **Repro**: `docker exec bp-sandbox brewprune status`

---

#### [TRACKING] `stats --package` for a never-used package gives no actionable information
- **Severity**: UX-polish
- **What happens**: `brewprune stats --package jq` shows:
  ```
  Package: jq
  Total Uses: 0
  Last Used: never
  Days Since: N/A
  First Seen: 2026-02-28 03:08:20
  Frequency: never
  ```
  The output is technically correct but offers no guidance. There is no suggestion like "run `brewprune explain jq` to see removal recommendation" or "has been tracked for 0 days."
- **Expected**: Include a pointer to `brewprune explain <package>` for removal advice, and/or show the current confidence score inline.
- **Repro**: `docker exec bp-sandbox brewprune stats --package jq`

---

#### [TRACKING] `stats` default output lists all 40 packages including those with 0 usage — no filtering or summary-first view
- **Severity**: UX-polish
- **What happens**: `brewprune stats` outputs all 40 packages sorted by usage (git first, then 39 packages all with 0 runs). The table is 40 rows long. For a user with many packages, this is a wall of "never / → " rows.
- **Expected**: Default view should show only packages with recorded usage, with a summary line like "39 packages with no usage (use --all to show)". The current output buries the signal (1 used package) in noise (39 unused ones).
- **Repro**: `docker exec bp-sandbox brewprune stats`

---

### Area 5: Explain

#### [EXPLAIN] `explain nonexistent` exits with code 0 on package-not-found error
- **Severity**: UX-critical
- **What happens**: `brewprune explain nonexistent` prints an error message but exits with code 0:
  ```
  Error: package not found: nonexistent
  Run 'brewprune scan' to update package database
  EXIT:0
  ```
- **Expected**: An error condition (package not found) should exit with a non-zero code (1) so scripts and CI pipelines can detect the failure.
- **Repro**: `docker exec bp-sandbox brewprune explain nonexistent; echo $?` → outputs `0`

---

#### [EXPLAIN] `explain git` shows "Usage: 0/40 pts — used today" which contradicts itself
- **Severity**: UX-improvement
- **What happens**: In the `explain git` table, the Usage row shows `0/40` points with detail "used today." The score is 0 because git was recently used, but the presentation is confusing — "0 points" usually means bad/low, yet the reason is that git IS being used (a good thing, meaning keep it). The negative framing is inverted from user expectation.
- **Expected**: The detail text and point value should be consistent in meaning. If usage = 0/40 means "actively used, do not remove," consider labeling it as "0/40 pts (package is in active use)" or restructuring so higher points indicate higher removal confidence across all dimensions. At minimum, add a parenthetical like "0/40 pts — recently used (lower score = keep this package)".
- **Repro**: `docker exec bp-sandbox brewprune explain git`

```
│ Usage │ 0/40 │ used today │
```

---

#### [EXPLAIN] `explain` verbose table truncates the "Detail" column with "..." at 38 chars
- **Severity**: UX-polish
- **What happens**: In the `explain` table for git, the Type row shows: `foundational package (reduced con...` — the detail is truncated at the table column width. On a standard 80+ column terminal, there is room to show the full string "foundational package (reduced confidence)".
- **Expected**: Either widen the Detail column, use terminal width detection to size columns dynamically, or avoid truncating at all by wrapping or using a wider default.
- **Repro**: `docker exec bp-sandbox brewprune explain git`

```
│ Type  │  0/10  │ foundational package (reduced con... │
```

---

### Area 6: Diagnostics

#### [DIAGNOSTICS] `doctor` documents no `--fix` flag but uses the word "Fix:" in its output
- **Severity**: UX-critical
- **What happens**: `doctor` output says:
  ```
  ⚠ Shim directory not in PATH — executions won't be intercepted
    Fix: add shim directory to PATH before Homebrew:
    export PATH="/home/brewuser/.brewprune/bin":$PATH
  ```
  Running `brewprune doctor --fix` returns `Error: unknown flag: --fix`. A user reading "Fix:" naturally tries `--fix`.
- **Expected**: Either implement `--fix` to automate the PATH fix (e.g., source the profile or re-run quickstart step 2), or rename the label from "Fix:" to "How to fix:" or "Action needed:" to avoid implying a flag exists.
- **Repro**: `docker exec bp-sandbox brewprune doctor` then `docker exec bp-sandbox brewprune doctor --fix`

---

#### [DIAGNOSTICS] `doctor` pipeline test takes 15-20 seconds with no progress indicator
- **Severity**: UX-improvement
- **What happens**: The pipeline test (`✓ Pipeline test: pass (20.206s)`) runs at the end of `doctor` with no visible progress. The user sees a blank pause of 15-20 seconds before the final line appears.
- **Expected**: Show an in-progress indicator such as "Running pipeline test..." before the result, so users do not think the tool is frozen. The elapsed time shown in the result is helpful but only appears after the wait.
- **Repro**: `docker exec bp-sandbox brewprune doctor`

---

### Area 7: Remove (Dry-Run)

#### [REMOVE] `remove nonexistent --dry-run` error message is doubled
- **Severity**: UX-polish
- **What happens**: The error message reads: `Error: package nonexistent not found: package nonexistent not found` — the phrase "package nonexistent not found" appears twice.
- **Expected**: `Error: package "nonexistent" not found`
- **Repro**: `docker exec bp-sandbox brewprune remove nonexistent --dry-run`

```
Error: package nonexistent not found: package nonexistent not found
EXIT:1
```

---

#### [REMOVE] Dry-run output does not show score or tier for each package in the table
- **Severity**: UX-improvement
- **What happens**: `brewprune remove --safe --dry-run` shows a table identical to `unused --tier safe` output (Package, Size, Uses, Last Used, Depended On, Status) but does not show numeric scores. The "Summary" block shows aggregate disk space but no per-package score column.
- **Expected**: The remove dry-run output — which is the last thing a user sees before deciding to proceed — should show scores so users can make an informed decision. This is the highest-stakes view.
- **Repro**: `docker exec bp-sandbox brewprune remove --safe --dry-run`

---

### Area 8: Undo

#### [UNDO] `undo latest` exits 0 when no snapshots exist, but should exit non-zero
- **Severity**: UX-critical
- **What happens**: `brewprune undo latest` with no snapshots available outputs:
  ```
  No snapshots available.

  Snapshots are automatically created before package removal.
  Use 'brewprune remove' to remove packages and create snapshots.
  EXIT:0
  ```
  Exit code 0 means "success" but the operation did not succeed — nothing was restored.
- **Expected**: Exit with a non-zero code (e.g., 1) when no snapshots are available and nothing was restored.
- **Repro**: `docker exec bp-sandbox brewprune undo latest; echo $?` → outputs `0`

---

#### [UNDO] `undo` with no args gives a good hint, but `undo latest` message is not clearly an error
- **Severity**: UX-polish
- **What happens**: `brewprune undo` (no args) correctly says "Error: snapshot ID or 'latest' required (use --list to see available snapshots)" with exit 1. But `brewprune undo latest` with no snapshots uses a friendly informational tone ("No snapshots available") rather than an error format, which combined with exit 0 reads as if the command completed normally.
- **Expected**: "No snapshots available" should be prefixed with "Error:" or shown as a warning so it is clear this is a failure state, and the exit code should be non-zero (see finding above).
- **Repro**: `docker exec bp-sandbox brewprune undo latest`

---

### Area 9: Edge Cases

#### [EDGE] Unknown subcommand gives no suggestions
- **Severity**: UX-improvement
- **What happens**: `brewprune blorp` outputs:
  ```
  Error: unknown command "blorp" for "brewprune"
  EXIT:1
  ```
  There is no "Did you mean...?" suggestion, and no pointer to `brewprune --help`.
- **Expected**: Append "Run 'brewprune --help' for a list of available commands." or, for close typos, offer a "Did you mean X?" suggestion. Many modern CLIs (git, go, cargo) do this by default.
- **Repro**: `docker exec bp-sandbox brewprune blorp`

---

## Observations by Area (Behaviors That Work Well)

These areas worked correctly and deserve acknowledgment:

1. **Tier validation errors**: `--tier invalid` and `--sort invalid` both produce clear, informative messages listing valid values, and correctly exit non-zero.
2. **Verbose scoring (`unused -v`)**: The verbose per-package breakdown is detailed and educational, covering all four scoring dimensions with point values and explanatory text.
3. **`explain` for known packages**: The table format is clear, color-coded (red for RISKY, green for SAFE), and the Recommendation text is specific and actionable.
4. **Dry-run clarity**: `remove --safe --dry-run` and `remove --tier safe --dry-run` both clearly state "Dry-run mode: no packages will be removed." at the bottom.
5. **`undo --help` argument documentation**: The `Arguments:` section listing `snapshot-id` and `latest` is a nice touch not present in most cobra help pages.
6. **`stats --package` for nonexistent package**: Returns a proper error message and exits non-zero (exit 1).
7. **`quickstart` completion message**: The "What happens next" section is clear and appropriately sets expectations about the 1-2 week wait.
8. **Confidence footer on `unused`**: The "Confidence: MEDIUM (1 events, tracking for 0 days)" footer with tip is a valuable trust signal for new users.

---

## Color Usage Notes

Color is used in the following places (observed via ANSI escape codes):

- `explain`: Package name is **bold**, score and tier label are colored red (RISKY) or green (SAFE), and the "Total" row in the breakdown table inherits those colors.
- `unused`: No color observed in the table or tier summary header. Tier labels in the summary line ("SAFE: 5", "MEDIUM: 31", "RISKY: 4") appear to be plain text when captured via `docker exec`.
- `remove --dry-run`: No color observed.
- `doctor`: Check marks (✓) and warning signs (⚠) are the only visual distinction; no color observed in output.

The absence of color in the `unused` table means tier distinctions (✓ safe vs ~ review vs ✗ keep) rely entirely on the text glyphs. In a real terminal with color support this may differ, but the symbols alone provide adequate differentiation.
