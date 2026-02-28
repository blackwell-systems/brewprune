# Cold-Start UX Audit Report - Round 5

**Metadata:**
- Audit Date: 2026-02-28
- Tool Version: brewprune version dev (commit: unknown, built: unknown)
- Container: brewprune-r5
- Environment: Ubuntu 22.04 with Homebrew
- Auditor: New user perspective

## Executive Summary

| Severity | Count |
|----------|-------|
| UX-critical | 3 |
| UX-improvement | 9 |
| UX-polish | 12 |

**Overall Assessment:** brewprune presents a well-designed and user-friendly interface with comprehensive help text, clear error messages, and thoughtful output formatting. The major pain points center around PATH configuration confusion after quickstart, lack of implementation for advertised features (--fix flag), and some inconsistencies in command behavior. The tool successfully guides new users through setup and provides actionable recommendations.

---

## 1. Discovery Findings

### [DISCOVERY] Help text is comprehensive and well-structured
- **Severity**: UX-polish (positive finding)
- **What happens**: Running `brewprune` or `brewprune --help` shows detailed, well-formatted help with:
  - Clear "IMPORTANT" callout about daemon requirement
  - Guided "Quick Start" section
  - Feature bullets
  - Practical examples
  - Command list with descriptions
- **Expected**: This is good behavior that should be preserved
- **Repro**: `brewprune --help`

### [DISCOVERY] Exit code is 0 for help, 1 for invalid commands
- **Severity**: UX-polish (positive finding)
- **What happens**:
  - `brewprune` (no args, shows help) exits with 0
  - `brewprune blorp` (invalid command) exits with 1
- **Expected**: This is correct Unix behavior
- **Repro**:
  - `brewprune; echo $?` → 0
  - `brewprune blorp; echo $?` → 1

### [DISCOVERY] Error messages are actionable
- **Severity**: UX-polish (positive finding)
- **What happens**: Invalid commands and flags provide clear guidance:
  - "Error: unknown command 'blorp' for 'brewprune'"
  - "Run 'brewprune --help' for usage."
  - "Error: invalid --tier value 'invalid': must be one of: safe, medium, risky"
- **Expected**: This is excellent error messaging
- **Repro**: `brewprune blorp`, `brewprune unused --tier invalid`

### [DISCOVERY] Version output lacks detail
- **Severity**: UX-improvement
- **What happens**: `brewprune --version` shows:
  - "brewprune version dev (commit: unknown, built: unknown)"
- **Expected**: In a dev environment this is acceptable, but production builds should include:
  - Actual commit hash
  - Build date
  - Maybe Go version for debugging
- **Repro**: `brewprune --version`

---

## 2. Setup / Onboarding Findings

### [SETUP] Quickstart is excellent for first-time setup
- **Severity**: UX-polish (positive finding)
- **What happens**: `brewprune quickstart` provides:
  - Step-by-step progress indicators (1/4, 2/4, etc.)
  - Green checkmarks for completed steps
  - Clear file paths (PID file, log file)
  - "Setup complete!" confirmation
  - Guidance on next steps
  - Wait time expectation (1-2 weeks)
- **Expected**: This is exemplary UX for setup workflows
- **Repro**: `brewprune quickstart`

### [SETUP] PATH configuration is confusing after quickstart
- **Severity**: UX-critical
- **What happens**:
  1. Quickstart reports: "✓ Added /home/brewuser/.brewprune/bin to PATH in /home/brewuser/.profile"
  2. Immediately shows: "Restart your shell (or source the config file) for this to take effect."
  3. But then later says: "✓ Usage tracking daemon started"
  4. Step 4/4 says: "✓ Tracking verified — brewprune is working"
  5. However, running `brewprune doctor` immediately after shows: "⚠ Shim directory not in PATH — executions won't be intercepted"
  6. The PATH warning conflicts with the "tracking verified" message
- **Expected**: Either:
  - Quickstart should acknowledge PATH isn't active yet and tracking won't work until shell restart
  - Or quickstart should test in a way that doesn't require PATH to be set
  - The success message "tracking verified" is misleading when PATH isn't actually configured in current session
- **Repro**: Run `brewprune quickstart` in a fresh shell, then `brewprune doctor`

### [SETUP] Doctor --fix flag is advertised but not implemented
- **Severity**: UX-critical
- **What happens**:
  - Doctor help text says: "Note: The --fix flag is not yet implemented. To fix issues automatically, re-run 'brewprune quickstart'."
  - But running `brewprune doctor --fix` shows: "Error: unknown flag: --fix"
- **Expected**: Either:
  - Remove the flag entirely and the note from help text
  - Or implement basic --fix functionality that re-runs quickstart or applies fixes
  - The help text advertising an unimplemented flag creates confusion
- **Repro**: `brewprune doctor --fix`

### [SETUP] Doctor pipeline test is very slow
- **Severity**: UX-improvement
- **What happens**: Doctor runs a "pipeline test" that takes 25+ seconds on first run, 3+ seconds on subsequent runs, with no progress indicator
- **Expected**:
  - Show a progress indicator: "Running pipeline test... (this may take 30 seconds)"
  - Or show what it's doing: "Testing shim → daemon → database pipeline..."
  - Or make it optional: "Run with --full to include pipeline test"
- **Repro**: `brewprune doctor` (wait 25 seconds)

### [SETUP] Doctor exit code is 0 even with warnings
- **Severity**: UX-improvement
- **What happens**: `brewprune doctor` finds warnings (PATH not configured, daemon not running) but still exits with code 0
- **Expected**:
  - Exit 0 for "all green"
  - Exit 1 for warnings (fixable issues)
  - Exit 2 for errors (broken state)
  - This allows scripts to detect degraded state
- **Repro**: `brewprune doctor` with issues, then `echo $?` → 0

### [SETUP] Quickstart daemon startup uses confusing dots animation
- **Severity**: UX-polish
- **What happens**: During "Step 3/4: Starting usage tracking service", the output shows:
  ```
  Starting daemon......
  ✓ Daemon started
  ```
  - The dots animation suggests waiting/progress
  - But it's not clear how long it will take
  - The message "brew found but using daemon mode (brew services not supported on Linux)" appears before the dots
- **Expected**:
  - Show a clearer status: "Starting daemon..." with spinner or just immediate success
  - Or show countdown: "Starting daemon (may take 5 seconds)..."
  - The current dots are ambiguous
- **Repro**: `brewprune quickstart` and watch Step 3

---

## 3. Core Feature: Unused Package Detection Findings

### [UNUSED] Default view balances information and safety well
- **Severity**: UX-polish (positive finding)
- **What happens**: `brewprune unused` (no flags) shows:
  - Header with tier summaries and counts
  - Safe and medium tiers (risky hidden by default)
  - Table with package name, size, score, usage, last used, dependents, status
  - Color-coded status icons: ✓ safe, ~ review, ⚠ risky
  - Footer with reclaimable space by tier
  - Notice about hidden risky packages
  - Data confidence indicator
  - Actionable tip about waiting 1-2 weeks
- **Expected**: This is excellent UX — shows useful info without overwhelming new users
- **Repro**: `brewprune unused`

### [UNUSED] Verbose output is extremely helpful
- **Severity**: UX-polish (positive finding)
- **What happens**: `brewprune unused --verbose` shows:
  - Individual package breakdown for each package
  - Score components with points (Usage: 40/40, Dependencies: 30/30, etc.)
  - Detailed reasoning for each component
  - "Reason" summary
  - Separator lines between packages
  - Helpful tip at bottom: "For easier viewing of long output, pipe to less"
- **Expected**: This is excellent detailed output for power users
- **Repro**: `brewprune unused --verbose`

### [UNUSED] Tier filtering works correctly
- **Severity**: UX-polish (positive finding)
- **What happens**:
  - `--tier safe` shows only safe tier (5 packages)
  - `--tier medium` shows only medium tier (31 packages)
  - `--tier risky` shows only risky tier (4 packages)
  - `--all` shows all tiers together
- **Expected**: This is correct behavior
- **Repro**: `brewprune unused --tier [safe|medium|risky]` or `brewprune unused --all`

### [UNUSED] Min-score filter is confusing with tier header
- **Severity**: UX-improvement
- **What happens**: Running `brewprune unused --min-score 70` shows:
  - Header: "SAFE: 5 packages (39 MB) · MEDIUM: 31 (180 MB) · RISKY: 4 (hidden, use --all)"
  - But only displays 5 packages (the ones with score >= 70)
  - Footer says: "Hidden: 35 below score threshold (70); 4 in risky tier"
- **Expected**: The header summary should reflect the filtered view:
  - Either update counts: "SAFE: 5 packages (39 MB) · MEDIUM: 0 (0 MB) · RISKY: 4 (hidden)"
  - Or clarify with text: "Showing 5 of 40 packages (score >= 70)"
  - Current header implies 31 medium packages exist in the output when they're actually filtered
- **Repro**: `brewprune unused --min-score 70`

### [UNUSED] Empty state for filters is handled well
- **Severity**: UX-polish (positive finding)
- **What happens**: `brewprune unused --tier safe --min-score 90` shows:
  - "No packages match: tier=safe, min-score=90"
  - Suggestions:
    - "Try lowering --min-score"
    - "Try a different --tier (safe, medium, risky)"
- **Expected**: This is good empty state messaging
- **Repro**: `brewprune unused --tier safe --min-score 90`

### [UNUSED] Header summary is very information-dense
- **Severity**: UX-polish
- **What happens**: Header shows: "SAFE: 5 packages (39 MB) · MEDIUM: 31 (180 MB) · RISKY: 4 (hidden, use --all)"
- **Expected**: This is good, but could be even clearer with icons:
  - "✓ SAFE: 5 (39 MB) · ~ MEDIUM: 31 (180 MB) · ⚠ RISKY: 4 (hidden)"
  - The icons would match the Status column for consistency
- **Repro**: `brewprune unused`

### [UNUSED] "Last Used" column shows "never" for most packages
- **Severity**: UX-improvement
- **What happens**: In a fresh install, every package shows "Last Used: never"
- **Expected**: Could be more informative:
  - "Last Used: not tracked (tracking started today)"
  - Or just "—" to indicate no data
  - "never" implies we've been watching for a while and truly never saw usage
- **Repro**: `brewprune unused` right after setup

### [UNUSED] Git shows "Last Used: just now" but is in RISKY tier
- **Severity**: UX-improvement
- **What happens**: Running git commands then checking unused shows:
  - git: "Last Used: just now" but Score: 30/100, Status: ⚠ risky
  - This is confusing — if it was just used, why is it risky to remove?
- **Expected**:
  - The "risky" label is because git is a core dependency (protected)
  - But to a new user, seeing "just used" + "risky to remove" is contradictory
  - Consider: Status: "🛡 protected" or "core dependency" instead of "⚠ risky"
  - Or tooltip: "⚠ risky (core dependency, low usage score)"
- **Repro**: Run `git --version`, wait 35s, then `brewprune unused --all`

### [UNUSED] Data confidence indicator is helpful but subtle
- **Severity**: UX-polish
- **What happens**: Footer shows: "Confidence: MEDIUM (2 events, tracking for 0 days)"
- **Expected**: This is good but easy to miss. Could be more prominent:
  - Use color: "Confidence: MEDIUM (2 events, tracking for 0 days)" in yellow
  - Or header: "⚠ Data Confidence: MEDIUM — wait 1-2 weeks for reliable recommendations"
- **Repro**: `brewprune unused`

---

## 4. Data / Tracking Findings

### [TRACKING] Status output is clear and concise
- **Severity**: UX-polish (positive finding)
- **What happens**: `brewprune status` shows:
  ```
  Tracking:     running (since 4 seconds ago, PID 1419)
  Events:       1 total · 1 in last 24h
  Shims:        active · 222 commands · PATH configured (restart shell to activate)
  Last scan:    4 seconds ago · 40 formulae · 72 KB
  Data quality: COLLECTING (0 of 14 days)
  ```
- **Expected**: This is excellent concise status output
- **Repro**: `brewprune status`

### [TRACKING] Status shows "PATH configured" but doctor says PATH missing
- **Severity**: UX-critical
- **What happens**:
  - `brewprune status` says: "Shims: active · 222 commands · PATH configured (restart shell to activate)"
  - `brewprune doctor` says: "⚠ Shim directory not in PATH — executions won't be intercepted"
  - Both are technically correct (PATH is in config file, but not active in current shell)
  - But the mixed messaging is confusing
- **Expected**:
  - Status should distinguish: "PATH configured (not active in current shell)"
  - Or status should test actual $PATH: "PATH configured in ~/.profile (active: no)"
  - Doctor should acknowledge: "PATH configured but not active in current shell (restart needed)"
- **Repro**: Run `brewprune quickstart`, then compare `brewprune status` and `brewprune doctor`

### [TRACKING] Commands executed don't show up immediately in stats
- **Severity**: UX-improvement
- **What happens**:
  1. Run `git --version && jq --version && fd --version`
  2. Commands execute successfully
  3. Run `brewprune status` immediately → Events: 2 total
  4. Wait 35 seconds (daemon processes every 30 seconds)
  5. Run `brewprune status` again → Events: 2 total (no change!)
- **Expected**:
  - Either the shims aren't working (because PATH isn't set in the Docker exec environment)
  - Or the events are being logged but not processed
  - Should provide feedback: "3 events pending processing (daemon runs every 30s)"
  - Or: "Run commands from an interactive shell for tracking to work"
- **Repro**:
  ```
  brewprune quickstart
  git --version && jq --version
  sleep 35
  brewprune status
  ```

### [TRACKING] Scan output is boring but appropriate
- **Severity**: UX-polish (positive finding)
- **What happens**: Running `brewprune scan` shows:
  - "✓ Database up to date (40 packages, 0 changes)"
  - Subsequent scans show same message
  - No verbose output unless something changes
- **Expected**: This is good — quiet when nothing to report
- **Repro**: `brewprune scan` (twice)

### [TRACKING] Data quality indicator is clear
- **Severity**: UX-polish (positive finding)
- **What happens**: Status shows: "Data quality: COLLECTING (0 of 14 days)"
- **Expected**: This clearly communicates that we're still gathering data
- **Repro**: `brewprune status`

---

## 5. Explanation / Detail Findings

### [EXPLAIN] Package detail is extremely comprehensive
- **Severity**: UX-polish (positive finding)
- **What happens**: `brewprune explain git` shows:
  - Package name (bold)
  - Score with color (30 in red, RISKY in red)
  - Install date
  - Detailed breakdown table with:
    - Component (Usage, Dependencies, Age, Type, Criticality Penalty)
    - Score for each component (0/40, 30/30, etc.)
    - Detailed explanation for each
  - "Why RISKY" summary
  - Recommendation (red-colored "Do not remove")
  - Rationale paragraph
  - Protected status
- **Expected**: This is outstanding detail for users who want to understand scoring
- **Repro**: `brewprune explain git`

### [EXPLAIN] Explain shows ANSI codes in some outputs
- **Severity**: UX-improvement
- **What happens**: Output contains ANSI escape sequences like `[1m`, `[0m`, `[31m`, `[32m` for formatting
- **Expected**: These should be properly rendered as bold/colors, not visible codes
  - This might be a terminal/Docker issue rather than brewprune issue
  - In most terminals, these render correctly
  - In the audit output, they're visible as raw codes
- **Repro**: `brewprune explain git` (view in non-ANSI terminal)

### [EXPLAIN] Error for nonexistent package is helpful
- **Severity**: UX-polish (positive finding)
- **What happens**: `brewprune explain nonexistent-package` shows:
  - "Error: package not found: nonexistent-package"
  - "Check the name with 'brew list' or 'brew search nonexistent-package'."
  - "If you just installed it, run 'brewprune scan' to update the index."
- **Expected**: This is excellent error guidance with next steps
- **Repro**: `brewprune explain nonexistent-package`

### [EXPLAIN] Missing package name shows clear usage
- **Severity**: UX-polish (positive finding)
- **What happens**: `brewprune explain` (no package) shows:
  - "Error: missing package name. Usage: brewprune explain <package>"
- **Expected**: This is clear and concise
- **Repro**: `brewprune explain`

### [STATS] Default stats view is concise
- **Severity**: UX-polish (positive finding)
- **What happens**: `brewprune stats` shows:
  - Header: "Showing 1 of 40 packages (39 with no recorded usage — use --all to see all)"
  - Table with: Package, Total Runs, Last Used, Frequency, Trend
  - Only shows packages with usage
  - Summary: "1 packages used in last 30 days (out of 40 total)"
  - Reminder: "(39 packages with no recorded usage hidden — use --all to show)"
- **Expected**: This is good progressive disclosure — show interesting data, hide noise
- **Repro**: `brewprune stats`

### [STATS] Package-specific stats are detailed
- **Severity**: UX-polish (positive finding)
- **What happens**: `brewprune stats --package git` shows:
  - Total Uses: 2
  - Last Used: timestamp
  - Days Since: 0
  - First Seen: timestamp
  - Frequency: daily
  - Tip: "Run 'brewprune explain git' for removal recommendation"
- **Expected**: This is comprehensive without overwhelming
- **Repro**: `brewprune stats --package git`

### [STATS] All packages view is very long
- **Severity**: UX-improvement
- **What happens**: `brewprune stats --all` shows all 40 packages in a table, most with "never" usage
- **Expected**: Could provide guidance:
  - "Tip: pipe to less for easier scrolling: brewprune stats --all | less"
  - Or paginate automatically for tall outputs
  - Or default to showing only packages with usage (current behavior is good)
- **Repro**: `brewprune stats --all`

---

## 6. Diagnostics Findings

### [DOCTOR] Doctor identifies issues correctly
- **Severity**: UX-polish (positive finding)
- **What happens**: `brewprune doctor` correctly detects:
  - Database exists and is accessible
  - Package count
  - Usage event count
  - Daemon running/not running
  - Shim binary presence
  - PATH configuration issues
  - Pipeline test results
- **Expected**: This is thorough diagnostic coverage
- **Repro**: `brewprune doctor`

### [DOCTOR] Doctor after killing daemon provides clear action
- **Severity**: UX-polish (positive finding)
- **What happens**: After killing daemon, doctor shows:
  - "⚠ Daemon not running (no PID file)"
  - "Action: Run 'brewprune watch --daemon'"
  - "⊘ Pipeline test skipped (daemon not running)"
  - "The pipeline test requires a running daemon to record usage events"
- **Expected**: This is excellent actionable guidance
- **Repro**: `pkill -f "brewprune watch" && brewprune doctor`

### [DOCTOR] Pipeline test runs even when daemon is running
- **Severity**: UX-improvement
- **What happens**: Even with daemon running and PATH issues, doctor runs the 3-25 second pipeline test
- **Expected**:
  - Skip pipeline test if critical issues exist (no daemon, no PATH)
  - Or make it optional: "Run 'brewprune doctor --full' to include pipeline test"
  - The test is valuable but slow for routine checks
- **Repro**: `brewprune doctor` (wait 25 seconds)

### [DOCTOR] Warning summary could be color-coded
- **Severity**: UX-polish
- **What happens**: Final line says: "Found 1 warning(s). System is functional but not fully configured."
- **Expected**: Could use color for emphasis:
  - Yellow text for warnings
  - Red text for errors
  - Green text for "All checks passed!"
- **Repro**: `brewprune doctor`

---

## 7. Destructive / Write Operations Findings

### [REMOVE] Dry-run mode is safe and clear
- **Severity**: UX-polish (positive finding)
- **What happens**: `brewprune remove --tier safe --dry-run` shows:
  - "Packages to remove (safe tier):"
  - Table of packages to remove
  - Summary: Packages: 5, Disk space: 39 MB, Snapshot: will be created
  - "Dry-run mode: no packages will be removed."
- **Expected**: This is excellent dry-run output
- **Repro**: `brewprune remove --tier safe --dry-run`

### [REMOVE] Missing tier flag provides helpful suggestion
- **Severity**: UX-polish (positive finding)
- **What happens**: `brewprune remove` (no tier) shows:
  - "Error: no tier specified"
  - "Try: brewprune remove --safe --dry-run"
  - "Or use --medium or --risky for more aggressive removal"
- **Expected**: This is great error guidance
- **Repro**: `brewprune remove`

### [REMOVE] Nonexistent package error is clear
- **Severity**: UX-polish (positive finding)
- **What happens**: `brewprune remove --dry-run nonexistent-package` shows:
  - "Error: package 'nonexistent-package' not found"
- **Expected**: Could add suggestion: "Check package name with 'brew list' or 'brewprune scan'"
- **Repro**: `brewprune remove --dry-run nonexistent-package`

### [UNDO] List snapshots when none exist is informative
- **Severity**: UX-polish (positive finding)
- **What happens**: `brewprune undo --list` shows:
  - "No snapshots available."
  - "Snapshots are automatically created before package removal."
  - "Use 'brewprune remove' to remove packages and create snapshots."
- **Expected**: This is good empty state messaging
- **Repro**: `brewprune undo --list`

### [UNDO] Missing argument error is clear
- **Severity**: UX-polish (positive finding)
- **What happens**: `brewprune undo` (no arg) shows:
  - "Error: snapshot ID or 'latest' required"
  - "Usage: brewprune undo [snapshot-id | latest]"
  - "Use 'brewprune undo --list' to see available snapshots"
- **Expected**: This is clear error with guidance
- **Repro**: `brewprune undo`

### [UNDO] Undo latest when no snapshots provides context
- **Severity**: UX-polish (positive finding)
- **What happens**: `brewprune undo latest` shows:
  - "Error: no snapshots available."
  - "Snapshots are automatically created before package removal."
  - "Run 'brewprune undo --list' to see all available snapshots."
  - "Use 'brewprune remove' to remove packages and create snapshots."
- **Expected**: This is comprehensive error guidance
- **Repro**: `brewprune undo latest`

### [UNDO] Invalid snapshot ID error could be more helpful
- **Severity**: UX-improvement
- **What happens**: `brewprune undo 999` shows:
  - "Error: snapshot 999 not found: snapshot 999 not found" (duplicated message)
- **Expected**:
  - Remove duplication: "Error: snapshot 999 not found"
  - Add suggestion: "Run 'brewprune undo --list' to see available snapshots"
- **Repro**: `brewprune undo 999`

---

## 8. Edge Cases Findings

### [EDGE] Invalid database path error is clear
- **Severity**: UX-polish (positive finding)
- **What happens**: `brewprune --db /nonexistent/path.db status` shows:
  - "Error: database path does not exist: /nonexistent/path.db"
  - "Check the --db path or run 'brewprune quickstart' to set up."
- **Expected**: This is helpful error messaging
- **Repro**: `brewprune --db /nonexistent/path.db status`

### [EDGE] Invalid flag shows usage hint
- **Severity**: UX-polish (positive finding)
- **What happens**: `brewprune --invalid-flag` shows:
  - "Error: unknown flag: --invalid-flag"
- **Expected**: Could add: "Run 'brewprune --help' for available flags"
- **Repro**: `brewprune --invalid-flag`

### [EDGE] Flag requiring argument shows clear error
- **Severity**: UX-polish (positive finding)
- **What happens**: `brewprune stats --package` (no value) shows:
  - "Error: flag needs an argument: --package"
- **Expected**: This is clear, though could add: "Usage: brewprune stats --package <name>"
- **Repro**: `brewprune stats --package`

---

## 9. Output Formatting Findings

### [OUTPUT] Table alignment is excellent
- **Severity**: UX-polish (positive finding)
- **What happens**: All tables are well-aligned with:
  - Consistent column widths
  - Clear headers
  - Horizontal separator lines (using ─ characters)
  - Proper spacing
- **Expected**: This is professional table formatting
- **Repro**: Any command with table output (unused, stats, explain)

### [OUTPUT] Color usage is semantic
- **Severity**: UX-polish (positive finding)
- **What happens**: Colors are used consistently:
  - Green checkmarks (✓) for success
  - Yellow warning icon (⚠) for warnings
  - Red text for errors and risky items
  - Green text for safe items
  - Tilde (~) for review items
- **Expected**: This is good semantic color usage
- **Repro**: Various commands (status, doctor, unused, explain)

### [OUTPUT] Progress indicators are present for long operations
- **Severity**: UX-improvement
- **What happens**:
  - Quickstart shows step progress (1/4, 2/4, etc.) ✓
  - Scan shows minimal output ✓
  - Doctor pipeline test has no progress indicator ✗
  - Remove dry-run shows immediate results ✓
- **Expected**: Add progress/spinner for doctor pipeline test
- **Repro**: `brewprune doctor` (notice no indicator during 25s test)

### [OUTPUT] Headers and footers provide context
- **Severity**: UX-polish (positive finding)
- **What happens**:
  - Tables have descriptive headers
  - Footers show summaries (reclaimable space, hidden counts)
  - Tips appear at bottom of relevant outputs
  - Confidence indicators provide data quality context
- **Expected**: This is excellent contextual information
- **Repro**: `brewprune unused`, `brewprune stats`

### [OUTPUT] Terminology is mostly consistent
- **Severity**: UX-improvement
- **What happens**:
  - "unused" vs "removal candidates" (both used)
  - "packages" vs "formulae" (both used — "formulae" is Homebrew term)
  - "daemon" vs "service" vs "tracking" (all used for watch daemon)
  - Status column: "safe" / "review" / "risky" inconsistent with tier names "SAFE" / "MEDIUM" / "RISKY"
- **Expected**:
  - Pick one term and stick with it
  - "packages" is more user-friendly than "formulae"
  - Status icons should match tier names: ✓ safe, ~ medium, ⚠ risky (not "review")
- **Repro**: Compare help text and output across commands

### [OUTPUT] Empty states are well-handled
- **Severity**: UX-polish (positive finding)
- **What happens**: Commands with no data show helpful messages:
  - No snapshots: "Use 'brewprune remove' to remove packages and create snapshots."
  - No matching packages: "Suggestions: Try lowering --min-score"
  - No usage data: "(39 packages with no recorded usage hidden — use --all to show)"
- **Expected**: This is excellent empty state handling
- **Repro**: `brewprune undo --list`, `brewprune unused --tier safe --min-score 90`

---

## 10. Documentation / Help Text Findings

### [DOCS] Help text uses practical examples
- **Severity**: UX-polish (positive finding)
- **What happens**: All command help pages include:
  - Usage synopsis
  - Description/explanation
  - Flags section
  - Examples section with realistic scenarios
  - Some include tips and notes
- **Expected**: This is comprehensive help text
- **Repro**: Any `brewprune [command] --help`

### [DOCS] Unused help text explains scoring system
- **Severity**: UX-polish (positive finding)
- **What happens**: `brewprune unused --help` includes:
  - Scoring components (40+30+20+10 = 100)
  - Tier definitions with ranges
  - Core dependency capping explanation
  - Tier filtering behavior
  - Examples of common workflows
- **Expected**: This is excellent educational content in help text
- **Repro**: `brewprune unused --help`

### [DOCS] Doctor help mentions --fix isn't implemented
- **Severity**: UX-improvement
- **What happens**: Doctor help says: "Note: The --fix flag is not yet implemented."
- **Expected**:
  - If flag isn't implemented, don't mention it or accept it as a flag
  - Currently creates confusion (help mentions it, but running it gives "unknown flag")
  - Better: implement basic --fix that re-runs quickstart or remove mention entirely
- **Repro**: `brewprune doctor --help`, then `brewprune doctor --fix`

### [DOCS] Remove help clearly explains safety model
- **Severity**: UX-polish (positive finding)
- **What happens**: `brewprune remove --help` explains:
  - Tier behavior
  - Shortcut flags (--safe, --medium, --risky)
  - Safety features (validation, warnings, snapshots, confirmation)
  - Examples with progression (dry-run → review → execute)
- **Expected**: This is excellent safety-focused documentation
- **Repro**: `brewprune remove --help`

---

## Summary of Recommendations

### Critical (Must Fix)
1. **Fix PATH configuration messaging** - Reconcile conflicting messages between quickstart/status/doctor about PATH configuration
2. **Remove or implement --fix flag** - Either implement `doctor --fix` or remove mentions of it entirely
3. **Clarify post-quickstart tracking status** - Don't claim "tracking verified" when PATH isn't active in current shell

### Important (Should Fix)
1. **Add progress indicator to doctor pipeline test** - 25 seconds is too long without feedback
2. **Make doctor exit codes meaningful** - 0 for success, 1 for warnings, 2 for errors
3. **Fix header counts with --min-score filter** - Header should reflect filtered counts or clarify what's shown
4. **Distinguish "protected/core dependency" from "risky"** - git used recently but shown as risky is confusing
5. **Explain why commands aren't being tracked** - In Docker/non-interactive shells, provide feedback about why tracking isn't working
6. **Consistent terminology** - Pick "packages" vs "formulae", align status labels with tier names
7. **Fix duplicate error message** - `brewprune undo 999` shows error twice
8. **Clarify "never used" vs "no data yet"** - Fresh installs show "never" which implies long observation period

### Polish (Nice to Have)
1. Add color to confidence indicators (yellow for MEDIUM, red for LOW)
2. Make pipeline test optional (`--full` flag)
3. Add icons to header tier summary for consistency
4. Version output should include commit/build date in production builds
5. Add suggestions to more error messages (invalid flag → show help)
6. Paginate or warn about long outputs (stats --all)
7. Consider using 🛡 or "protected" instead of "⚠ risky" for core dependencies

---

## Overall Assessment

brewprune demonstrates **exceptional attention to UX detail** across the board. The tool successfully guides new users from discovery → setup → daily use → troubleshooting with clear, actionable messages at every step. The three critical issues are related to messaging consistency around PATH configuration rather than broken functionality. Fixing these would elevate an already-strong user experience to excellent.

**Standout positive features:**
- Comprehensive, practical help text
- Excellent error messages with next steps
- Progressive disclosure (safe defaults, --verbose for detail)
- Clear empty states
- Safety-first design (dry-run, snapshots, confirmations)
- Professional table formatting
- Semantic use of color and icons

**Areas needing improvement:**
- PATH configuration messaging confusion
- Unimplemented --fix flag
- Slow operations without progress indicators
- Some terminology inconsistencies
- Exit codes don't reflect warning/error state
