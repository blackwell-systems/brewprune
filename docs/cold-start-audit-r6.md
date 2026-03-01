# Cold-Start UX Audit Report - Round 6

**Metadata:**
- Audit Date: 2026-02-28
- Tool Version: brewprune version dev (commit: unknown, built: unknown)
- Container: brewprune-r6
- Environment: Ubuntu 22.04 with Homebrew
- Auditor: Claude (automated cold-start audit)

---

## Summary Table

| Severity       | Count |
|----------------|-------|
| UX-critical    | 4     |
| UX-improvement | 9     |
| UX-polish      | 8     |
| **Total**      | 21    |

---

## Findings by Area

---

### Area 1: Discovery & Help System

#### [HELP-1] `brewprune` bare command shows help instead of nudging the user to act

- **Severity**: UX-polish
- **What happens**: Running `brewprune` with no arguments prints the full help text (exit 0). This is the same output as `brewprune --help`.
- **Expected**: The bare invocation could print a shorter "what to do next" prompt (e.g., "Run `brewprune quickstart` to get started, or `brewprune --help` for full usage.") rather than the full help wall. The full help is appropriate for `--help`, but the bare command is often a "what can I do?" query that deserves a more action-oriented nudge.
- **Repro**: `docker exec brewprune-r6 brewprune`

---

#### [HELP-2] `doctor --help` omits mention of the `--fix` flag

- **Severity**: UX-polish
- **What happens**: The `doctor --help` output lists no flags other than `-h/--help` and `--db`. There is no mention of a `--fix` flag anywhere in the help or in actual `doctor` output. However, the main `--help` text for doctor mentions "Recommends next steps" which implies fix capability.
- **Expected**: If `--fix` does not exist, any references suggesting auto-fix should be removed. If it is planned or was described in prior audit prompts as expected, the flag should be implemented and documented.
- **Repro**: `docker exec brewprune-r6 brewprune doctor --help`

---

#### [HELP-3] `unused --help` uses `-v` for verbose but main flags list shows `--verbose`

- **Severity**: UX-polish
- **What happens**: The `unused --help` flags section lists `-v, --verbose` which is correct. However the examples section writes `brewprune unused --tier safe -v` (short form) while the rest of the docs and command outputs use `--verbose`. This is minor but inconsistent.
- **Expected**: Prefer the long form `--verbose` in all examples for discoverability.
- **Repro**: `docker exec brewprune-r6 brewprune unused --help`

---

#### [HELP-4] `watch --help` mentions "service" once in prose but no other command uses that word

- **Severity**: UX-polish
- **What happens**: The watch help text says "the usage tracking service" in the quickstart help, but the rest of the tool uses "daemon" consistently. The quickstart step 3 output says "Starting usage tracking service" in a comment. This is a minor terminology inconsistency.
- **Expected**: Use "daemon" consistently throughout all output and help text.
- **Repro**: `docker exec brewprune-r6 brewprune quickstart --help` ("Start the usage tracking service")

---

### Area 2: Setup & Onboarding

#### [ONBOARD-1] Quickstart step 3 emits duplicate/inconsistent output about daemon start

- **Severity**: UX-improvement
- **What happens**: During `brewprune quickstart`, Step 3 produces two distinct blocks of daemon startup text that partially duplicate each other:

  ```
  Step 3/4: Starting usage tracking service
    brew found but using daemon mode (brew services not supported on Linux)
  Starting daemon......
  ✓ Daemon started

  Usage tracking daemon started
    PID file: /home/brewuser/.brewprune/watch.pid
    Log file: /home/brewuser/.brewprune/watch.log

  To stop: brewprune watch --stop
    ✓ Usage tracking daemon started (watch --daemon)
  ```

  The "Usage tracking daemon started / PID file / Log file / To stop" block appears to be raw output from `brewprune watch --daemon` being passed through, and then the quickstart wrapper adds a second confirmation line `✓ Usage tracking daemon started (watch --daemon)`. The result is visually noisy and the indentation is inconsistent (the inner block is not indented to match the step format).

- **Expected**: Quickstart should capture daemon startup internally and emit a single clean confirmation line like `✓ Daemon started (PID: 1234, log: ~/.brewprune/watch.log)` with consistent indentation at the step level.
- **Repro**: `docker exec brewprune-r6 brewprune quickstart`

---

#### [ONBOARD-2] Doctor PATH warning references wrong shell config file

- **Severity**: UX-improvement
- **What happens**: The doctor warning says:
  ```
  Action: Restart your shell or run: source ~/.zprofile (or ~/.bash_profile)
  ```
  But the quickstart step 2 says it wrote to `~/.profile`, not `~/.zprofile` or `~/.bash_profile`. The action hint references files that do not contain the change.
- **Expected**: Doctor should detect which config file was actually modified and reference that specific file, e.g., `source ~/.profile`.
- **Repro**:
  ```
  docker exec brewprune-r6 brewprune quickstart
  docker exec brewprune-r6 brewprune doctor
  ```

---

#### [ONBOARD-3] Doctor pipeline test takes ~21-26 seconds with no progress indicator

- **Severity**: UX-improvement
- **What happens**: The doctor pipeline test message "Running pipeline test..." appears, then the terminal is silent for 21-26 seconds before printing the result. There is no spinner, dots, or elapsed time feedback.
- **Expected**: Either show a spinner/progress indicator, or note that the test takes ~30 seconds (it waits one daemon poll cycle). Without feedback, the user may think the tool has hung.
- **Repro**: `docker exec brewprune-r6 brewprune doctor` (wait through the pipeline test)

---

#### [ONBOARD-4] `cat ~/.brewprune/watch.log` from host perspective shows nothing

- **Severity**: UX-polish
- **What happens**: The watch.log file exists but is empty after daemon startup. There are no startup log lines (e.g., "daemon started", "watching usage.log"), making it hard for a new user to verify the daemon is alive by inspecting the log.
- **Expected**: The daemon should write at least one log line on startup (e.g., `[2026-02-28 21:54:01] brewprune-watch started, PID 1101`) so that `cat watch.log` provides confirmation.
- **Repro**: `docker exec brewprune-r6 cat /home/brewuser/.brewprune/watch.log`

---

### Area 3: Core Feature: Unused Package Discovery

#### [UNUSED-1] `--sort age` produces non-obvious ordering with no explanation

- **Severity**: UX-improvement
- **What happens**: `brewprune unused --sort age` produces an apparently random ordering - packages are not sorted oldest-first or newest-first in any obviously detectable order. All packages were installed on the same day in this container, so the sort produces what appears to be arbitrary tie-breaking. There is no note in the output or help text about what "age" means (install date? last used date?) or what direction the sort runs (oldest first? newest first?).
- **Expected**: Add a column header annotation or footer note clarifying sort direction (e.g., "sorted by install date, oldest first"). When packages share the same install date, secondary sort criteria should be documented.
- **Repro**: `docker exec brewprune-r6 brewprune unused --sort age`

---

#### [UNUSED-2] `--casks` flag silently has no effect - no feedback given

- **Severity**: UX-improvement
- **What happens**: `brewprune unused --casks` produces identical output to `brewprune unused` with no casks listed and no message indicating that cask tracking is not available or that no casks are installed. A new user cannot tell if the flag worked and found nothing, or if cask tracking is not implemented.
- **Expected**: When `--casks` is specified, the output should include a line such as "No casks installed" or "Cask tracking: 0 casks found" to confirm the flag was processed.
- **Repro**: `docker exec brewprune-r6 brewprune unused --casks`

---

#### [UNUSED-3] Verbose mode says "never observed execution" for packages but score is 40/40 on Usage - confusing inversion

- **Severity**: UX-improvement
- **What happens**: In `unused --verbose` and `explain` output, the Usage component shows `40/40 pts - never observed execution`. A score of 40/40 on "Usage" implies the package is heavily used, but the description says it was never used. The scoring is inverted (higher score = safer to remove) but the label "Usage" carries the connotation of measuring how much something is used, not confidence to remove.
- **Expected**: The component label should reflect what it actually measures. Options:
  1. Rename "Usage" to "Inactivity" or "Non-use" to match the inversion.
  2. Add a brief parenthetical at the top of the breakdown: "(higher score = safer to remove)" - this note does appear at the bottom of explain output but not in the verbose table header.
  3. Alternatively, invert the score display to show `0/40 usage events` and deduct from total for clarity.
  The note "Higher removal score = more confident to remove. Usage component: 0/40 means recently used. 40/40 means no usage ever observed." appears in `explain` but not in `unused --verbose`.
- **Repro**: `docker exec brewprune-r6 brewprune unused --tier safe --verbose`

---

#### [UNUSED-4] `--tier safe --all` silently ignores `--all` instead of warning about conflict

- **Severity**: UX-improvement
- **What happens**: `brewprune unused --tier safe --all` runs without error and shows only safe packages (the `--tier` wins silently). The user receives no indication that `--all` was ignored or that the two flags conflict.
- **Expected**: Either reject the combination with an error ("--all and --tier cannot be used together"), or explicitly note in the output that `--all` was ignored because `--tier` takes precedence.
- **Repro**: `docker exec brewprune-r6 brewprune unused --tier safe --all`

---

#### [UNUSED-5] Summary banner shows "RISKY: 4 (134 MB)" even when `--tier safe` filters risky out

- **Severity**: UX-polish
- **What happens**: When running `brewprune unused --tier safe`, the header line reads:
  ```
  SAFE: 5 packages (39 MB) · MEDIUM: 31 (180 MB) · RISKY: 4 (134 MB)
  ```
  This is informative but potentially confusing - the MEDIUM and RISKY counts appear even though neither tier is shown in the table below. A user might wonder "why don't I see the medium packages?".
- **Expected**: Consider making the active tier visually distinguished (bold/underlined/bracketed) in the summary banner, e.g., `[SAFE: 5 packages (39 MB)] · MEDIUM: 31 · RISKY: 4`. Or append "(showing safe only)" to the banner.
- **Repro**: `docker exec brewprune-r6 brewprune unused --tier safe`

---

### Area 4: Data Collection & Tracking

#### [TRACK-1] Usage events recorded for `git` but not `jq`, `bat`, `fd`, `rg` despite running those commands

- **Severity**: UX-critical
- **What happens**: After running `git --version`, `jq --version`, `bat --version`, `fd --version`, and `rg --version` through the container's PATH and waiting 35 seconds, only `git` appears in usage.log and stats. The usage.log shows 3 entries, all for `/home/brewuser/.brewprune/bin/git`. No entries appear for jq, bat, fd, or ripgrep.

  ```
  1772315684839434691,/home/brewuser/.brewprune/bin/git
  1772315693451287675,/home/brewuser/.brewprune/bin/git
  1772315732464011797,/home/brewuser/.brewprune/bin/git
  ```

  This is because the PATH shims are not active in the container shell environment during `docker exec` - the shim directory `~/.brewprune/bin` is in `~/.profile` but not sourced in non-login shells invoked by `docker exec`. The `git` entries come from the quickstart self-test, not from the manually run commands.

- **Expected**: Either:
  1. The audit instructions should acknowledge that shims are not active without a login shell (this is a documentation/help gap, not a bug), or
  2. The tool should provide a clear in-output note whenever shims are inactive that explains commands run outside a restarted shell will not be tracked.
  The status output does show "Shims: inactive · 0 commands · PATH configured (restart shell to activate)" which is the correct signal, but it does not proactively warn when usage events are expected but not appearing.
- **Repro**:
  ```
  docker exec brewprune-r6 git --version
  docker exec brewprune-r6 brewprune stats --package git
  ```
  (git runs show 0 additional events after the initial self-test events)

---

#### [TRACK-2] `brewprune watch --daemon --stop` silently honors `--stop` without flagging the conflict

- **Severity**: UX-improvement
- **What happens**: `brewprune watch --daemon --stop` stops the daemon (the `--stop` flag wins) without any warning that `--daemon` and `--stop` are mutually exclusive and that `--daemon` was ignored.
- **Expected**: Print an error or at least a warning: "Warning: --daemon and --stop are mutually exclusive. Stopping daemon."
- **Repro**: `docker exec brewprune-r6 brewprune watch --daemon --stop`

---

#### [TRACK-3] `stats --all` output for packages with zero usage shows "→" trend for everything

- **Severity**: UX-polish
- **What happens**: The Trend column in `brewprune stats --all` shows `→` (flat arrow) for all 39 packages with no usage and also for `git` with 3 uses. The trend arrow carries no information - every package shows the same symbol regardless of whether usage is growing, declining, or nonexistent.
- **Expected**: For packages with zero usage, the trend could show `-` or `—` (no data). For packages with actual data points, the trend should reflect whether usage is increasing (`↑`), decreasing (`↓`), or stable (`→`). If trend calculation requires more history than is available, display "n/a" or "—" rather than a misleading "flat" arrow.
- **Repro**: `docker exec brewprune-r6 brewprune stats --all`

---

#### [TRACK-4] `stats` summary line uses "1 days" instead of "1 day" (grammar)

- **Severity**: UX-polish
- **What happens**: `brewprune stats --days 1` outputs:
  ```
  Summary: 1 packages used in last 1 days (out of 40 total)
  ```
  Two issues: "1 packages" should be "1 package", and "last 1 days" should be "last 1 day".
- **Expected**: Proper pluralization: "1 package used in the last 1 day (out of 40 total)"
- **Repro**: `docker exec brewprune-r6 brewprune stats --days 1`

---

### Area 5: Package Explanation & Detail View

#### [EXPLAIN-1] Score inversion note buried at the bottom of `explain` output, after recommendation

- **Severity**: UX-improvement
- **What happens**: The `explain` output ends with:
  ```
  Note: Higher removal score = more confident to remove.
        Usage component: 0/40 means recently used (fewer points toward removal).
        40/40 means no usage ever observed.
  ```
  This critical framing note appears after the Breakdown table and after the Why/Recommendation sections. A user reading top-to-bottom sees "Usage: 0/40" first and naturally interprets it as "low usage score = bad" before reaching the clarifying note.
- **Expected**: Move the note to appear directly beneath the table header or as a preamble before the breakdown table. For example, add a single line: "(score measures removal confidence: higher = safer to remove)" immediately before or after the table.
- **Repro**: `docker exec brewprune-r6 brewprune explain jq`

---

#### [EXPLAIN-2] `explain` uses box-drawing table but `unused --verbose` uses plain dashes - inconsistent formatting

- **Severity**: UX-polish
- **What happens**: `brewprune explain git` renders a Unicode box-drawing table:
  ```
  ┌─────────────────────┬─────────┬─────...
  │ Component           │  Score  │ Detail
  ├─────────────────────┼─────────┼─────...
  ```
  But `brewprune unused --verbose` uses a plain text section format without a table:
  ```
  Breakdown:
    Usage:        40/40 pts - never observed execution
    Dependencies: 30/30 pts - no dependents
  ```
  These two views of the same underlying data use entirely different visual styles.
- **Expected**: Consistent formatting between `explain` and `unused --verbose`. Either both should use the box table, or both should use the aligned text format. Given that `unused --verbose` may show 30+ packages, the compact text format is probably the right choice for both.
- **Repro**:
  ```
  docker exec brewprune-r6 brewprune explain jq
  docker exec brewprune-r6 brewprune unused --tier safe --verbose
  ```

---

#### [EXPLAIN-3] `explain` for git shows "Criticality Penalty: -30" as a table row, but verbose shows no such row

- **Severity**: UX-polish
- **What happens**: `brewprune explain git` shows a "Criticality Penalty" row in the table with `-30` value. `brewprune unused --verbose` for the same package shows "Critical: YES - capped at 70 (core system dependency)" as a separate field after the breakdown lines. The two representations of the same concept use different language and different row names.
- **Expected**: Consistent terminology. If the penalty is called "Criticality Penalty" in explain, it should also say "Criticality Penalty" in verbose. Or vice versa.
- **Repro**:
  ```
  docker exec brewprune-r6 brewprune explain git
  docker exec brewprune-r6 brewprune unused --verbose (then check ca-certificates)
  ```

---

### Area 6: Diagnostics (Doctor)

#### [DOCTOR-1] Doctor color codes leak into terminal as raw ANSI escape sequence

- **Severity**: UX-critical
- **What happens**: The final summary line of `brewprune doctor` output contains a raw ANSI escape sequence visible in the captured output:
  ```
  [33mFound 1 warning(s). System is functional but not fully configured.[0m
  ```
  This appears to be a color-coding failure where the ANSI escape codes are being written but not interpreted - or being captured and echoed literally. In a standard terminal this would render as yellow text, but the escape codes are visible in piped or logged output without color stripping.
- **Expected**: This is most likely correct terminal color behavior that appears wrong only in captured output. However, the tool should use a color library that auto-detects terminal capability (e.g., checking `NO_COLOR`, `TERM`, or whether stdout is a TTY) and strips color codes when output is piped. Verify that `brewprune doctor 2>&1 | cat` strips colors correctly.
- **Repro**: `docker exec brewprune-r6 brewprune doctor 2>&1` (view raw output)

---

#### [DOCTOR-2] Doctor "Tip" about aliases config is shown even when no aliases issue exists

- **Severity**: UX-polish
- **What happens**: After every doctor run (passing or failing), the following tip always appears:
  ```
  Tip: Create ~/.config/brewprune/aliases to declare alias mappings and improve tracking coverage.
       Example: ll=eza
       See 'brewprune help' for details.
  ```
  This tip is displayed regardless of context, including when the user just completed quickstart and everything is working.
- **Expected**: Tips shown by doctor should be contextual. The aliases tip should only appear when relevant (e.g., when few usage events are recorded, or only once during initial setup), not on every diagnostic run. Persistent tips that always appear lose their signal value.
- **Repro**: `docker exec brewprune-r6 brewprune doctor` (run repeatedly)

---

#### [DOCTOR-3] No color differentiation between `✓`, `⚠`, and `✗` items in doctor output

- **Severity**: UX-improvement
- **What happens**: Doctor output uses checkmarks (`✓`), warnings (`⚠`), and crosses (`✗`) consistently, which is good. However, in captured output (and likely in terminals) there is no color applied to these individual lines - only the final summary line gets color. A user scanning quickly must read each symbol carefully rather than being able to glance at a green/yellow/red visual pattern.
- **Expected**: Apply green color to `✓` lines, yellow to `⚠` lines, and red to `✗` lines, matching the final summary color scheme.
- **Repro**: `docker exec brewprune-r6 brewprune doctor`

---

### Area 7: Destructive Operations (Remove & Undo)

#### [REMOVE-1] `remove --safe --medium` does not error - silently uses `--medium` (last flag wins)

- **Severity**: UX-critical
- **What happens**: Running `brewprune remove --safe --medium` does not produce an error about conflicting tiers. Instead it silently uses the medium tier and prompts to remove 31 packages. A user who meant to run `--safe` has no indication that `--medium` overrode their intent.
- **Expected**: Combining two tier shortcut flags (`--safe`, `--medium`, `--risky`) should produce an error: "Error: only one tier flag can be specified at a time (got --safe and --medium)."
- **Repro**: `docker exec brewprune-r6 brewprune remove --safe --medium --dry-run`

---

#### [REMOVE-2] `remove --risky` shows no additional warning or confirmation step beyond `remove --safe`

- **Severity**: UX-critical
- **What happens**: `brewprune remove --risky` presents the same confirmation prompt as `--safe`:
  ```
  Remove 40 packages? [y/N]:
  ```
  There is no additional scary warning about risky packages (which include core system dependencies like git, openssl@3, ncurses). The `--help` text mentions "requires confirmation" for risky, but the actual prompt is identical to safe/medium.
- **Expected**: Risky-tier removal should require an extra confirmation step or a different, more alarming prompt. For example:
  ```
  WARNING: You are about to remove 4 risky packages including core dependencies
  (git, openssl@3, ncurses, zlib-ng-compat). This may break your system.
  Type "remove risky packages" to confirm:
  ```
  Or at minimum, require typing "yes" rather than just "y".
- **Repro**: `docker exec brewprune-r6 brewprune remove --risky`

---

#### [REMOVE-3] `remove --no-snapshot` shows "Snapshot: SKIPPED" in dry-run but no warning to the user

- **Severity**: UX-improvement
- **What happens**: `brewprune remove --no-snapshot --safe --dry-run` shows `Snapshot: SKIPPED (--no-snapshot)` in the summary section. The help text describes this as "dangerous" but the output does not visually emphasize the danger - it's presented the same way as "Snapshot: will be created".
- **Expected**: Flag the skipped snapshot with a warning character and color. For example: `⚠ Snapshot: SKIPPED (--no-snapshot) — removal cannot be undone!` in yellow/red.
- **Repro**: `docker exec brewprune-r6 brewprune remove --no-snapshot --safe --dry-run`

---

#### [REMOVE-4] `undo latest --yes` restore output lists packages as `bat@` with trailing `@` sign

- **Severity**: UX-polish
- **What happens**: During `brewprune undo latest --yes`, the packages to restore are listed as:
  ```
  - bat@ (explicit)
  - fd@ (explicit)
  - jq@ (explicit)
  - ripgrep@ (explicit)
  - tmux@ (explicit)
  ```
  The trailing `@` character appears to be a version specifier artifact from how brew snapshot records package names, but it looks like a display bug to a user.
- **Expected**: Strip the trailing `@` when displaying package names in restore output, or if the `@` is meaningful (e.g., it pins the version), include a note explaining what it means.
- **Repro**: `docker exec brewprune-r6 brewprune undo latest`

---

#### [REMOVE-5] After `undo`, user is told to run `brewprune scan` but next `remove` also reminds them

- **Severity**: UX-polish
- **What happens**: After `brewprune undo latest --yes` completes, the output says:
  ```
  Run 'brewprune scan' to update the package database.
  ```
  If the user then runs `brewprune remove --safe --medium`, the output begins with:
  ```
  ⚠  5 new formulae since last scan. Run 'brewprune scan' to update shims.
  ```
  The duplicate nudge is not harmful but creates noise. More importantly, the `remove` command proceeds to run despite the stale database, which could produce inaccurate results (the restored packages may show incorrect scores).
- **Expected**: Either auto-run scan after undo (since it's always needed), or block `remove` if the database is known stale and require an explicit `--stale-ok` opt-in.
- **Repro**:
  ```
  docker exec brewprune-r6 brewprune undo latest --yes
  docker exec brewprune-r6 brewprune remove --safe --medium
  ```

---

### Area 8: Edge Cases & Error Handling

#### [EDGE-1] Missing database produces internal SQL error message, not user-friendly guidance

- **Severity**: UX-critical
- **What happens**: After `rm -rf ~/.brewprune`, running `brewprune unused`, `brewprune stats`, or `brewprune remove --safe` produces:
  ```
  Error: failed to list packages: failed to list packages: SQL logic error: no such table: packages (1)
  ```
  The error message exposes the internal SQL error and duplicates the "failed to list packages" phrase twice in the chain.
- **Expected**: Intercept the "no such table" error and replace it with a user-friendly message:
  ```
  Error: brewprune database not found or not initialized.
  Run 'brewprune scan' to create the database.
  ```
  The raw SQL error chain should not be user-visible.
- **Repro**:
  ```
  docker exec brewprune-r6 rm -rf /home/brewuser/.brewprune
  docker exec brewprune-r6 brewprune unused
  ```

---

#### [EDGE-2] Unknown subcommands give minimal guidance, no suggestions

- **Severity**: UX-improvement
- **What happens**: `brewprune blorp`, `brewprune list`, and `brewprune prune` all produce:
  ```
  Error: unknown command "blorp" for "brewprune"
  Run 'brewprune --help' for usage.
  ```
  No suggestions for what the user may have meant.
- **Expected**: For plausible near-matches, suggest the correct command. For example:
  - `brewprune list` -> "Did you mean `brewprune unused`?"
  - `brewprune prune` -> "Did you mean `brewprune remove`?"
  Even a generic "Available commands: completion, doctor, explain, quickstart, remove, scan, stats, status, undo, unused, watch" would be more useful than just pointing to `--help`.
- **Repro**: `docker exec brewprune-r6 brewprune list`

---

#### [EDGE-3] `stats --days abc` shows raw Go parser error

- **Severity**: UX-improvement
- **What happens**: `brewprune stats --days abc` outputs:
  ```
  Error: invalid argument "abc" for "--days" flag: strconv.ParseInt: parsing "abc": invalid syntax
  ```
  The internal `strconv.ParseInt` detail and "invalid syntax" are Go implementation details that are inappropriate for user-facing error messages.
- **Expected**: Replace with a clean message: `Error: --days must be a positive integer (got "abc")`
- **Repro**: `docker exec brewprune-r6 brewprune stats --days abc`

---

### Area 9: Output Quality & Visual Design

#### [VISUAL-1] Confidence footer uses raw ANSI codes instead of rendered color in piped output

- **Severity**: UX-polish
- **What happens**: The footer line in `unused` output shows:
  ```
  Confidence: [33mMEDIUM[0m (3 events, tracking for 0 days)
  ```
  The `[33m` and `[0m` are visible when output is captured. This is the same issue as DOCTOR-1 - color codes are emitted without TTY detection.
- **Expected**: Auto-detect TTY and strip ANSI codes when output is piped, or use a color library with proper `NO_COLOR` / `isatty` support.
- **Repro**: `docker exec brewprune-r6 brewprune unused 2>&1 | cat`

---

#### [VISUAL-2] `remove bat fd --dry-run` output format differs from `remove --tier --dry-run`

- **Severity**: UX-polish
- **What happens**: `brewprune remove bat fd --dry-run` produces a different format from `brewprune remove --safe --dry-run`:

  The explicit package version:
  ```
  Warnings:
    ⚠️  bat: explicitly installed (not a dependency)
    ⚠️  fd: explicitly installed (not a dependency)

    bat                    80/100  SAFE    rarely used, safe to remove
    fd                     80/100  SAFE    rarely used, safe to remove
  ```

  The tier version uses the full aligned table with Size, Score, Uses (7d), Last Used, Depended On, Status columns. The explicit package version uses a simpler two-column layout with odd indentation and no column headers. The emoji warning `⚠️` vs the ASCII `⚠` are also inconsistent.

- **Expected**: Consistent output format between explicit package removal and tier removal. Both should use the same table layout. The "explicitly installed" warnings are valuable but should be visually integrated into a consistent design.
- **Repro**:
  ```
  docker exec brewprune-r6 brewprune remove --safe --dry-run
  docker exec brewprune-r6 brewprune remove bat fd --dry-run
  ```

---

## Positive Observations

### What works well

1. **Quickstart is functional and clear.** The 4-step numbered format is clean, each step reports success, and the final summary gives the user concrete next steps. The pipeline self-test is a genuinely useful feature.

2. **Doctor is actionable.** Each warning includes an explicit "Action:" line with the exact command to run. This is excellent UX for a diagnostic tool.

3. **The unused table is well-designed.** Column alignment is consistent, headers are clear, the tier summary banner at the top is informative, and the footer correctly identifies hidden tiers. The `--min-score` filter also shows "Showing X of Y packages (score >= N)" which is excellent.

4. **Error messages for invalid enum values are precise.** `--tier invalid`, `--min-score 200`, `--sort invalid` all produce clean, specific errors with valid options listed.

5. **Snapshot/undo flow is solid.** The undo confirmation prompt, snapshot detail display, progress bar during restore, and post-restore guidance are all well-executed.

6. **`explain` output is thorough.** The breakdown table, reasoning text, recommendation, and "Protected" flag together give the user a complete picture of why a package scored the way it did.

7. **`remove --safe` with no packages to show doesn't crash.** (When all packages were removed and restored in sequence, remove re-evaluated correctly after scan was needed.)

8. **`watch --stop` feedback is clean.** "Stopping daemon......" followed by "✓ Daemon stopped" with clear spinner animation is good UX.

9. **Status output after daemon stop is immediately clear.** "Tracking: stopped (run 'brewprune watch --daemon')" is exactly the right level of conciseness.

---

## Recommendations Summary

### High Priority (UX-critical)

1. **Fix raw SQL errors** (EDGE-1): The "no such table: packages" error leaking through is the most jarring cold-start failure. A new user who accidentally runs commands before `scan` will be confused.

2. **Warn on conflicting tier flags** (REMOVE-1): `--safe --medium` should error, not silently use medium. This is a footgun for users who mistype.

3. **Add risky-tier removal escalation** (REMOVE-2): The `--risky` tier includes core system packages (git, openssl, ncurses). It needs a more alarming prompt than `--safe`.

4. **Fix shim tracking transparency** (TRACK-1): When commands are run but not tracked (because shims aren't active), the tool is silent. Proactively surfacing "0 new events tracked" after expected activity would help new users understand why their commands aren't showing up.

### Medium Priority (UX-improvement)

5. **Fix doctor PATH action hint** (ONBOARD-2): The file referenced in the action hint doesn't match what quickstart actually wrote.

6. **Add pipeline test progress indicator** (ONBOARD-3): 21-26 seconds of silence looks like a hang.

7. **Clean up quickstart daemon startup output** (ONBOARD-1): The duplicated/inconsistent daemon startup block is the most visually noisy part of an otherwise clean onboarding experience.

8. **Clarify score inversion** (UNUSED-3): The 40/40 "usage score" meaning "never used" is unintuitive. Surface the inversion note earlier in verbose/explain output.

9. **Improve trend column** (TRACK-3): `→` for everything is meaningless. Use `—` for no-data packages.

### Lower Priority (UX-polish)

10. **Consistent formatting** between `explain` table and `unused --verbose` text (EXPLAIN-2).
11. **Strip trailing `@`** from package names in undo restore output (REMOVE-4).
12. **Pluralization** in stats summary ("1 days", "1 packages") (TRACK-4).
13. **Color-code doctor result lines** per severity rather than only the summary (DOCTOR-3).
14. **Reduce always-on aliases tip** in doctor output (DOCTOR-2).
