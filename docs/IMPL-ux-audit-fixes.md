# IMPL: UX Audit Fixes

**Source:** `docs/ux-audit.md` (21 issues, audited 2026-02-28)
**Scout date:** 2026-02-27

---

## Wave Structure

```
Wave 0 ── Agent 0: Invert score logic  (confidence.go)
              │
Wave 1 ── Agent A: Scan/setup flow     (scan.go, quickstart.go, watch.go, status.go)
          Agent B: Individual commands  (explain.go, doctor.go, stats.go, undo.go)
              │
Wave 2 ── Agent C: Output/rendering    (output/table.go, output/progress.go)
          Agent D: Core analysis UX    (unused.go, remove.go, root.go)
```

No hidden coupling was found between agents in the same wave. Wave 1 agents do
not touch output/table.go. Wave 2 agents do not touch confidence.go. The score
inversion fix (Wave 0) must land before Wave 1 agents so their manual testing
reflects correct output.

---

## Hidden Coupling Notes

- `remove.go` calls `displayConfidenceScores()` which hard-codes `getNeverTime()`
  for `LastUsed` — this is the root of the [REMOVE] "Last Used shows never" bug.
  **Agent D owns this.** Agent C must not change `RenderConfidenceTable`'s
  signature in a way that breaks Agent D's call site.

- `output/table.go` defines `ConfidenceScore` (the output-layer struct) and
  `formatTierLabel()`. **Agent C owns both.** Agent D reads them; no
  signature changes should be needed because Agent D only changes the value
  passed into the existing `LastUsed` field.

- `output/progress.go` contains both `ProgressBar` and `Spinner`. **Agent C
  owns both.** Only `ProgressBar.Finish()` and `Spinner.Start()` need changes.

- `root.go` (`internal/app/root.go`) is the cobra root command. **Agent D
  owns it.** Agents A and B do not touch it.

- `explain.go`'s `renderExplanation()` contains inline ANSI color codes.
  **Agent B owns explain.go** for the double-print and "missing arg" fixes.
  **Agent C owns output/table.go** for the NO_COLOR/isatty work. Agent B must
  use `output.IsColorEnabled()` (a helper Agent C will add to output/table.go)
  for any new color emission in explain.go. Agent B must NOT refactor all
  existing color codes in renderExplanation() — that belongs to Agent C's
  color-detection sweep.

---

## Score Logic Analysis (Wave 0)

The current `computeUsageScore()` in `confidence.go` awards **40 points when a
package was used within 7 days**. The total score is the sum of all components;
higher score → "safe" tier. A package used today scores 40+30+age+type ≥ 80,
which lands it in the **safe (remove)** tier. The audit correctly identifies
this as inverted.

**Fix:** Invert the usage component so that recent use → low removal pressure.
The cleanest correction that preserves the existing tier thresholds and all
other score components:

```
daysSince  current   corrected
≤7         40        0
≤30        30        10
≤90        20        20
≤365       10        30
never       0        40
```

This means a never-used package scores maximum usage points (40), and a
package used today scores 0 — exactly as the rubric intends. The existing test
`TestComputeScore_RecentlyUsedPackage` currently asserts `UsageScore == 40` and
`Tier == "safe"` for jq used 3 days ago; that test encodes the **broken**
behavior and must be updated alongside the fix.

Similarly, `generateReason()` uses `score.UsageScore >= 30` to detect
"recently used" in the risky-tier branch. After inversion, recently-used
packages score UsageScore 0, so that branch becomes `score.UsageScore == 0`
(or equivalently, the check detects recent use via the explanation detail).
The safer fix is to track last-used days directly in the reason generator or
pass it through. See Agent 0 constraints.

---

## Wave 0 Agent Prompt

# Wave 0 Agent 0: Fix inverted score logic

You are Wave 0 Agent 0. Your task is to invert the usage scoring in
`internal/analyzer/confidence.go` so that recent use produces low removal
pressure (not high), and update all affected tests, reason text, and
explanation strings.

## 1. File Ownership
You own these files. Do not touch any other files.
- `internal/analyzer/confidence.go` — modify
- `internal/analyzer/confidence_test.go` — modify

## 2. Interfaces You Must Implement
No new exported signatures. All existing exported signatures remain identical:
```go
func (a *Analyzer) ComputeScore(pkg string) (*ConfidenceScore, error)
func (a *Analyzer) GetPackagesByTier(tier string) ([]*ConfidenceScore, error)
```

The `ConfidenceScore` struct fields and their types do not change.

## 3. Interfaces You May Call
```go
// store
func (s *Store) GetLastUsage(pkg string) (*time.Time, error)
func (s *Store) GetDependents(pkg string) ([]string, error)
func (s *Store) GetPackage(pkg string) (*brew.Package, error)
func (s *Store) ListPackages() ([]*brew.Package, error)

// scanner
func IsCoreDependency(pkg string) bool
```

## 4. What to Implement

### computeUsageScore — invert the mapping
```
daysSince ≤ 7   → return 0   (was 40)
daysSince ≤ 30  → return 10  (was 30)
daysSince ≤ 90  → return 20  (was 20) — no change
daysSince ≤ 365 → return 30  (was 10)
never / error   → return 40  (was 0)
```
Comment: "0 = recently used (keep), 40 = never used (safe to remove)".

### generateReason — fix the risky-tier "recently used" branch
The current code checks `score.UsageScore >= 30` to detect recent use in the
risky tier. After inversion, `UsageScore >= 30` means *rarely or never used*,
not recently used. Fix this branch: detect recent use by re-fetching the last
usage time (the same value already computed in computeUsageScore), OR by
checking `score.UsageScore == 0` (which now means used within 7 days).

Use `score.UsageScore == 0` as the signal for "recently used, keep" in
generateReason's risky branch.

### generateReason — fix safe-tier reason text
The safe-tier branch currently returns "rarely used, safe to remove" when
`score.UsageScore != 0`. After inversion, a non-zero UsageScore in the safe
tier means the package hasn't been used recently. This text is now accurate
(rarely/never used → safe to remove). Ensure the zero-usage case still returns
"never used, no dependents" / "never used, only unused dependents" which is
correct.

After the fix the only safe-tier reason that was wrong was the fallback "rarely
used, safe to remove" on a package with UsageScore==40 (never used). That is
now correct.

### Explanation UsageDetail — no change required
The UsageDetail string is generated from the raw timestamp, not from UsageScore,
so it remains accurate ("used today", "last used 3 days ago", etc.).

### Update tests in confidence_test.go
- `TestComputeScore_RecentlyUsedPackage`: jq used 3 days ago.
  - `UsageScore` must now be `0` (was `40`).
  - Total score: `0 + 30 + 15 + 10 = 55` → tier `medium` (was `95`, tier `safe`).
  - Update all assertions.

- `TestGetPackagesByTier`: the package `"risky_pkg"` is inserted with `lastUsed`
  3 days ago. After inversion it scores UsageScore=0, so its total score drops.
  Check that the test still verifies the right tier for each package, and fix
  any assertion that relied on old score values.

- `TestComputeScore_TierBoundaries`: all three cases have no usage events so
  UsageScore=40 now (was 0). Recalculate expected totals:
  - `medium_boundary_60`: 200 days old, no dependents, HasBinary=true.
    `40+30+20+10=100` → tier `safe`. Rename/update accordingly.
  - `medium_boundary_50`: 40 days old, no dependents, HasBinary=true.
    `40+30+10+10=90` → tier `safe`.
  - `risky_boundary_20`: 20 days old, has 1 dependent, HasBinary=true.
    `40+10+0+10=60` → tier `medium`. (DepsScore=10: 1 dependent, unused)
  Update all expected values. These packages now all move to higher tiers
  because they were never used. That's correct.

- `TestComputeScore_CriticalityPenalty`: packages have no usage events.
  After inversion UsageScore=40 for all. Recalculate:
  - `git` (HasBinary=true, 200 days): `40+30+20+10=100` → capped at 70, tier `medium`. OK.
  - `openssl@3` (HasBinary=true, TypeScore=0 for core): `40+30+20+0=90` → capped at 70, tier `medium`. Update expectedScore from 50 to 70.
  - `coreutils` (HasBinary=true, 200 days): `40+30+20+10=100` → capped at 70, tier `medium`. Update expectedScore from 60 to 70.
  The assertions `score.IsCritical && score.Score > 70` and `score.IsCritical && score.Tier == "safe"` remain valid and do not change.

- Add a new test `TestComputeScore_UsedTodayIsRisky`:
  Package used today (daysSince=0) should have UsageScore=0, and if the
  package otherwise has low other scores (e.g., <30 days old, has dependents)
  it should land in risky tier. At minimum, assert UsageScore==0 and
  Tier!="safe" for a recently-used package.

- Add a new test `TestComputeScore_NeverUsedIsHighScore`:
  Package never used should have UsageScore=40. Assert UsageScore==40.

## 5. Tests to Write
- `TestComputeScore_UsedTodayIsRisky` — package used today scores UsageScore=0
- `TestComputeScore_NeverUsedIsHighScore` — package never used scores UsageScore=40

## 6. Verification Gate
```
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/analyzer/...
```
All must pass. The full `go test ./...` will be run by Wave 1+ agents after
they update their own tests.

## 7. Constraints
- Do not touch any file outside `internal/analyzer/confidence.go` and
  `internal/analyzer/confidence_test.go`.
- Do not change the `ConfidenceScore` struct or any exported function signature.
- The tier thresholds (safe≥80, medium≥50, risky<50) must not change.
- The criticality cap (core deps capped at 70) must not change.
- The 40/30/20/10 point maximums for Usage/Deps/Age/Type must not change.
- Only the *mapping* from daysSince to points changes (inverted).
- The `generateReason` risky-tier "recently used" logic must be updated or the
  displayed reason will be wrong even though the tier is now correct.

## 8. Report
When done, report: the new score table, updated test assertions, and the
resulting scores for a package used today vs a package never used.

---

## Wave 1 Agent A Prompt

# Wave 1 Agent A: Scan/setup flow fixes

You are Wave 1 Agent A. Your task is to fix 6 UX issues in the scan and setup
flow commands.

## 1. File Ownership
You own these files. Do not touch any other files.
- `internal/app/scan.go` — modify
- `internal/app/quickstart.go` — modify
- `internal/app/watch.go` — modify
- `internal/app/status.go` — modify

## 2. Interfaces You Must Implement
No new exported functions. All existing RunE function signatures remain.

## 3. Interfaces You May Call
```go
// output package (unchanged by Agent A, do not modify)
output.NewSpinner(message string) *output.Spinner
output.NewProgress(total int, description string) *output.ProgressBar

// shim package
shim.IsShimSetup() (bool, string)
shim.GetShimDir() (string, error)
shim.GenerateShims(binaries []string) (int, error)

// watcher package
watcher.IsDaemonRunning(pidFile string) (bool, error)

// app package internals (same package)
runScan(cmd, args) error
runWatch(cmd, args) error  // called by startWatchDaemonFallback
```

## 4. What to Implement

### [SETUP-1] scan.go — TTY detection for spinner (audit issue: SETUP spinner garbage)
The `output.NewSpinner()` call in `runScan` starts an animated spinner that
emits `\r` overwrite sequences. In non-TTY contexts these become literal noise.

Check `os.Stdout` with `golang.org/x/term` or `github.com/mattn/go-isatty`
(already in go.mod as an indirect dep via mattn/go-isatty v0.0.20) before
creating any spinner. When stdout is not a TTY, print a single static line
(e.g. `fmt.Println("Discovering packages...")`) and skip the spinner entirely.
Apply this to **all three spinner calls** in `runScan`: "Discovering packages",
"Building dependency graph", "Building shim binary", and "Generating PATH
shims". Keep the `StopWithMessage` output but guard spinner creation.

The `output.NewSpinner` API is owned by Agent C (Wave 2); do not modify it.
Instead, call `isatty.IsTerminal(os.Stdout.Fd())` inline in scan.go before
each spinner creation and use a static `fmt.Println` as the non-TTY fallback.

### [SETUP-2] scan.go — "0 shims created" on re-scan (audit issue: SETUP 0 shims)
In `runScan`, after `shim.GenerateShims(allBinaries)` returns, the shim count
`shimCount` is the number of **newly created** shims (0 on a re-scan when all
shims already exist). The current message is:
```
✓ %d command shims created
```
Fix: count existing shims separately. If shimCount==0, count the symlinks
already present in the shim directory and display:
```
✓ %d shims up to date (0 new)
```
If shimCount>0, keep the existing message `✓ %d command shims created`.

Use `countSymlinks(shimDir)` — this helper already exists in `status.go` in
the same package. You may call it directly.

### [SETUP-3] scan.go — emoji consistency (audit issue: OUTPUT emoji style)
The scan footer currently uses `⚠️ ` (emoji with variation selector + double
space) in one branch and `⚠` (plain Unicode) in another.

Standardize: replace all occurrences of `⚠️ ` and `⚠️  ` in `scan.go` with
`⚠ ` (plain `\u26A0` + single space). This makes scan consistent with watch.go
which already uses `⚠`.

### [SETUP-4] scan.go — remove empty Version column (audit issue: OUTPUT empty Version column)
In `scan.go`, `runScan` calls `output.RenderPackageTable(packages)`. The
`RenderPackageTable` function in `output/table.go` renders a `Version` column
that is always empty because `pkg.Version` is never populated from Homebrew
metadata. Agent C (Wave 2) will remove the Version column from `table.go`.

For Agent A: no change required in `scan.go` itself — just be aware the table
output will change shape after Agent C's work. This is noted here for
coordination only.

### [SETUP-5] status.go — fix daemon suggestion (audit issue: SETUP status suggests brew services)
In `runStatus`, line:
```go
fmt.Printf(label+"stopped  (run 'brew services start brewprune')\n", "Tracking:")
```
Replace with:
```go
fmt.Printf(label+"stopped  (run 'brewprune watch --daemon')\n", "Tracking:")
```
No platform detection needed — always recommend `brewprune watch --daemon` as
the canonical way to start the daemon.

### [SETUP-6] quickstart.go — treat "daemon already running" as success (audit issue: SETUP quickstart alarming warning)
In `runQuickstart`, the daemon-start step:
```go
if daemonErr := startWatchDaemonFallback(cmd, args); daemonErr != nil {
    fmt.Printf("  ⚠ Could not start daemon: %v\n", daemonErr)
```
`startWatchDaemonFallback` calls `runWatch` → `startWatchDaemon`, which
returns `fmt.Errorf("daemon already running (PID file: %s)", watchPIDFile)`.

Fix: inspect the error string. If it contains "already running", print
`✓ Daemon already running` instead of the warning:
```go
if daemonErr := startWatchDaemonFallback(cmd, args); daemonErr != nil {
    if strings.Contains(daemonErr.Error(), "already running") {
        fmt.Println("  ✓ Daemon already running")
    } else {
        fmt.Printf("  ⚠ Could not start daemon: %v\n", daemonErr)
        fmt.Println("  Run 'brewprune watch --daemon' manually after setup.")
    }
}
```
Apply the same pattern to both daemon-start branches in quickstart.go (the
`brew services` fallback path and the `brew not found` path).

### [SETUP-7] watch.go — idempotent --daemon (audit issue: SETUP watch --daemon fails when already running)
In `startWatchDaemon`, when `running == true`, currently:
```go
return fmt.Errorf("daemon already running (PID file: %s)", watchPIDFile)
```
Change to exit 0 with informational message:
```go
if running {
    pid := readPIDFromFile(watchPIDFile) // helper below
    fmt.Printf("Daemon already running (PID %d). Nothing to do.\n", pid)
    return nil
}
```
Add a package-private helper `readPIDFromFile(path string) int` that reads and
parses the PID, returning 0 on error.

Note: quickstart.go already relies on the error message containing "already
running" (fixed in [SETUP-6] above). After this change, `startWatchDaemon`
returns nil, so quickstart's error-check branch for "already running" will
never be reached — it will simply print the success message from
`startWatchDaemon` directly. Verify in quickstart that the output for the
"daemon was already running" path still reads correctly (it will print
`✓ Daemon started` or the message from startWatchDaemon — trace the code flow
and adjust quickstart if needed to not double-print success).

## 5. Tests to Write
All test files are in `internal/app/`. Write in existing `*_test.go` files
unless they don't exist, in which case create `<cmd>_test.go`:

- `TestRunScan_ShimCountZeroShowsUpToDate` (scan_test.go) — mock shim.GenerateShims
  returning 0 and verify output contains "up to date"
- `TestRunStatus_DaemonStoppedSuggestsWatchDaemon` (scan_test.go or new status_test.go) —
  verify the stopped tracking line contains "watch --daemon"
- `TestStartWatchDaemon_AlreadyRunningIsIdempotent` (watch_test.go) — verify that
  calling startWatchDaemon when daemon is running returns nil

## 6. Verification Gate
```
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/app/...
```

## 7. Constraints
- Do not touch `output/table.go`, `output/progress.go`, or any analyzer file.
- Do not touch `explain.go`, `doctor.go`, `stats.go`, `undo.go`, `unused.go`,
  `remove.go`, or `root.go`.
- Import `github.com/mattn/go-isatty` for TTY detection — it is already in
  go.sum as an indirect dependency. Add it as a direct import in scan.go.
- The `strings.Contains` check for "already running" in quickstart.go is a
  temporary coupling; after watch.go is fixed to return nil, the check becomes
  dead code but is harmless. Do not remove it — leave it as a safety net.
- Do not change any exported function signatures.

## 8. Report
When done, report: which issues were fixed, test results, any behavior changes
that downstream agents should know about (especially the watch --daemon
idempotency change).

---

## Wave 1 Agent B Prompt

# Wave 1 Agent B: Individual command fixes

You are Wave 1 Agent B. Your task is to fix 5 UX issues across explain, doctor,
stats, and undo commands.

## 1. File Ownership
You own these files. Do not touch any other files.
- `internal/app/explain.go` — modify
- `internal/app/doctor.go` — modify
- `internal/app/stats.go` — modify
- `internal/app/undo.go` — modify

## 2. Interfaces You Must Implement
No new exported functions. Existing RunE signatures unchanged.

## 3. Interfaces You May Call
```go
// analyzer package (unchanged by Agent B)
analyzer.New(st *store.Store) *analyzer.Analyzer
func (a *Analyzer) ComputeScore(pkg string) (*ConfidenceScore, error)
func (a *Analyzer) GetUsageStats(pkg string) (*UsageStats, error)
func (a *Analyzer) GetUsageTrends(days int) (map[string]*UsageStats, error)

// output package (do not modify)
output.RenderSnapshotTable(snapshots []*store.Snapshot) string
output.RenderUsageTable(stats map[string]output.UsageStats) string
```

## 4. What to Implement

### [EXPLAIN-1] explain.go — fix double-print of error (audit issue: EXPLAIN error printed twice)
In `runExplain`, when `st.GetPackage(packageName)` fails, the error is returned:
```go
return fmt.Errorf("package not found: %s\nRun 'brewprune scan' to update package database", packageName)
```
The `main.go` handler `fmt.Fprintf(os.Stderr, "Error: %v\n", err)` then prints
it to stderr. Cobra's error handling also prints it. Because `SilenceErrors:
true` is set on RootCmd, cobra does NOT print the error — but the error is
printed twice because `runExplain` is called through cobra's RunE, and the
error surfaces to `main.go`'s handler which prints it once, then... actually,
trace the flow: `Execute()` → `RootCmd.Execute()` → returns error to
`Execute()` → returned to `main.go` which prints it. With `SilenceErrors:
true`, cobra does not print it internally. So the double-print is likely coming
from the command printing to stdout AND returning the error (which gets printed
by main.go).

Audit the `runExplain` code: the package-not-found error is **returned**, not
printed directly. If double-print is observed, the likely cause is that
`GetPackage` internally calls `fmt.Print` on error. Check the store's
`GetPackage`. If not, the double-print may be from cobra calling `cmd.PrintErr`
via its error hook. The fix: **print the error directly** to stderr and return
`nil` (suppressing the main.go print), or ensure the error is only returned
once. Use:
```go
if err != nil {
    fmt.Fprintf(os.Stderr, "Error: package not found: %s\nRun 'brewprune scan' to update package database\n", packageName)
    return nil
}
```
This guarantees exactly one print. Returning `nil` is acceptable here because
the error was already communicated to the user.

### [EXPLAIN-2] explain.go — better missing-arg error (audit issue: DISCOVERY explain no arg)
`explainCmd` already has `Args: cobra.ExactArgs(1)`, so cobra handles arity.
The default cobra message is `"accepts 1 arg(s), received 0"`. To customize it,
add a custom `Args` validator:
```go
Args: func(cmd *cobra.Command, args []string) error {
    if len(args) == 0 {
        return fmt.Errorf("missing package name. Usage: brewprune explain <package>")
    }
    return cobra.ExactArgs(1)(cmd, args)
},
```

### [EXPLAIN-3] explain.go — table footer alignment (audit issue: EXPLAIN footer padding)
In `renderExplanation`, the total row:
```go
fmt.Printf("│ %sTotal%s               │ %s%2d/100%s │ %s%-36s%s │\n",
    colorBold, colorReset,
    tierColor, score.Score, colorReset,
    tierColor, truncateDetail(strings.ToUpper(score.Tier)+" tier", 36), colorReset)
```
The ANSI color codes (e.g. `\033[32m`) are invisible but counted by `%s`
format widths — they have 0 visual width but non-zero byte length. The `%-36s`
format counts the color escape bytes as visual characters, causing the visible
content to appear shorter than intended, leaving trailing spaces inside the
border. Fix: use `truncateDetail` without color inside the width-padded field,
then apply color around the whole padded string:
```go
tierLabel := truncateDetail(strings.ToUpper(score.Tier)+" tier", 36)
paddedLabel := fmt.Sprintf("%-36s", tierLabel)
fmt.Printf("│ %sTotal%s               │ %s%2d/100%s │ %s%s%s │\n",
    colorBold, colorReset,
    tierColor, score.Score, colorReset,
    tierColor, paddedLabel, colorReset)
```
This ensures the 36-char pad is computed on the plain string, not the
color-wrapped string.

### [DOCTOR-1] doctor.go — exit codes and no "Error:" prefix for non-critical issues (audit issue: DOCTOR exit code 1)
Currently `runDoctor` returns `fmt.Errorf("diagnostics failed")` for any
non-zero issue count. This causes main.go to print `Error: diagnostics failed`
and exit 1 for even minor warnings (PATH not set).

Define two exit-code categories:
- **Critical issues** (database inaccessible, binary missing): return an error
  → exit 1 with "Error: ..." prefix.
- **Non-critical warnings only** (daemon stopped, PATH not set, no events):
  print a summary and `os.Exit(2)` directly, bypassing main.go's error handler.

Implementation:
1. Track `criticalIssues` and `warningIssues` separately in `runDoctor`.
2. Classify each check:
   - Critical: DB not found, DB not accessible, shim binary not found,
     pipeline test fail.
   - Warning: daemon not running, PATH not set, no usage events recorded.
3. At the end:
   ```go
   if criticalIssues > 0 {
       fmt.Printf("Found %d critical issue(s) and %d warning(s).\n",
           criticalIssues, warningIssues)
       return fmt.Errorf("diagnostics failed")
   }
   if warningIssues > 0 {
       fmt.Printf("Found %d warning(s). System is functional but not fully configured.\n",
           warningIssues)
       os.Exit(2)
   }
   ```
4. Replace `✗` with `⚠` for warning-level checks in the output lines.

### [DOCTOR-2] doctor.go — fix double-print (audit issue: DOCTOR error duplicated)
Same root cause as explain.go. The entire doctor output is printed twice when
captured. The likely cause: `runDoctor` uses `fmt.Println` for all its output
(goes to stdout), but the final `return fmt.Errorf("diagnostics failed")` also
causes main.go to print to stderr. In a `2>&1 | cat` capture, stdout and stderr
are interleaved and the error line appears at the end. That's a single extra
line, not a full duplication.

If doctor output is truly doubled (all lines twice), there may be a cobra
`PersistentPreRunE` or similar hook that invokes the command twice. Check
`root.go` for hooks. If none found, the "duplication" may be the audit
observing the cobra `Usage()` function printing on error. With
`SilenceUsage: true` set on RootCmd, cobra will not print usage on error.
Confirm `SilenceUsage: true` is set on RootCmd (it is — already present).

The fix is the same as explain.go: for error cases that should not print
`Error: diagnostics failed`, use `os.Exit(2)` instead of returning an error.
After [DOCTOR-1]'s fix, only genuine critical failures return an error to
main.go, which is correct. The double-print bug is resolved as a side effect.

### [STATS-1] stats.go — sort stats by Total Runs descending (audit issue: TRACKING stats sort)
In `showUsageTrends`, the `RenderUsageTable` function in `output/table.go`
already sorts by `TotalRuns` descending (confirmed in table.go lines 217-219).
However, packages with zero runs appear interspersed with used packages when
zero-run entries have the same TotalRuns=0. The sort is stable for non-zero
entries, but zero-run packages are in undefined order.

Add a secondary sort to the `RenderUsageTable` entries: when `TotalRuns` is
equal, sort by `LastUsed` descending (most recently used first), with zero-time
(never used) packages sorted to the bottom.

This change must be made in `output/table.go`, which is **Agent C's file**.
Agent B's fix is therefore in `stats.go` itself: before calling
`RenderUsageTable`, sort the `outputStats` map entries so that packages with
usage appear before packages without. Since maps are unordered, the sort in
`showUsageTrends` should pre-sort before building `outputStats`, passing a
slice-based structure rather than relying solely on the table renderer.

Actually, `RenderUsageTable` already handles sorting internally (table.go
lines 207-219). Agent B does not need to pre-sort in stats.go — the existing
table renderer already sorts by TotalRuns descending. If the audit observed
unstable ordering, it was because never-used packages all have TotalRuns=0
and are sorted randomly among themselves. The secondary sort (LastUsed) should
be added by **Agent C** in `RenderUsageTable`. Agent B should leave a comment
in stats.go noting that `RenderUsageTable` is expected to sort by TotalRuns
desc + LastUsed desc as secondary.

### [STATS-2] stats.go — style the per-package stats view (audit issue: TRACKING stats --package unstyled)
In `showPackageStats`, the current output is a plain key-value dump. Apply
minimal styling:
1. Print a bold package name header.
2. Color-code the Frequency value: "daily" → green, "weekly" → yellow,
   "monthly"/"rarely" → red, "never" → gray.
3. Add a separator line.

Use inline ANSI constants (same as `explain.go` already does — copy the const
block or define it at package level in a new `internal/app/colors.go` file if
you prefer, but do NOT add a file that Agent D or Agent A is also adding).
Adding a `colors.go` is safe since no other agent creates that file.

Example output:
```
Package:    jq
Total Uses: 1
Last Used:  2026-02-28 03:43:17
Days Since: 0
First Seen: 2026-02-28 03:08:20
Frequency:  daily   ← green
```

### [UNDO-1] undo.go — friendly message for `undo latest` with no snapshots (audit issue: EDGE undo latest terse error)
In `runUndo`, when `snapshotArg == "latest"` and `len(snapshots) == 0`,
currently returns `fmt.Errorf("no snapshots available")`.

Change to:
```go
if len(snapshots) == 0 {
    fmt.Println("No snapshots available.")
    fmt.Println("\nSnapshots are automatically created before package removal.")
    fmt.Println("Use 'brewprune remove' to remove packages and create snapshots.")
    return nil
}
```
This matches the `listSnapshots()` message exactly. Return nil to avoid
the `Error:` prefix.

## 5. Tests to Write
- `TestRunExplain_MissingArgError` (explain_test.go or new) — cobra rejects 0
  args with custom message containing "missing package name"
- `TestRunExplain_NotFoundPrintedOnce` — verify explain nonexistent package
  prints error exactly once (capture output)
- `TestRunDoctor_WarningOnlyExitsCode2` (doctor_test.go or new) — if only
  warnings, os.Exit(2) is called; use a test harness that catches os.Exit
- `TestShowPackageStats_FrequencyIsColored` (stats_test.go) — verify colored
  frequency output for "daily" package
- `TestRunUndo_LatestNoSnapshotsFriendlyMessage` (undo_test.go) — verify
  friendly multi-line message is printed and nil is returned

## 6. Verification Gate
```
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/app/...
```

## 7. Constraints
- Do not touch `output/table.go`, `output/progress.go`, `scan.go`,
  `quickstart.go`, `watch.go`, `status.go`, `unused.go`, `remove.go`,
  `root.go`, or any file under `internal/analyzer/`.
- The `os.Exit(2)` in doctor.go makes that code path untestable with
  normal `go test` unless you use a subprocess test pattern. Write the
  test using `exec.Command` to run the binary, or use a function pointer
  to inject the exit function. Note this constraint in your report.
- Do not change any exported function signatures.
- Colors in `showPackageStats` must be guarded: skip colors when stdout is
  not a TTY. Use `isatty.IsTerminal(os.Stdout.Fd())` with the same import
  as Agent A uses in scan.go. If you add `colors.go`, it must be in
  `internal/app/` and not duplicate a file Agent A is creating.

## 8. Report
When done, report: issues fixed, any test limitations (e.g., os.Exit testing
pattern used), deviations from this spec.

---

## Wave 2 Agent C Prompt

# Wave 2 Agent C: Output/rendering fixes

You are Wave 2 Agent C. Your task is to fix 5 rendering and output issues in
the output package.

## 1. File Ownership
You own these files. Do not touch any other files.
- `internal/output/table.go` — modify
- `internal/output/progress.go` — modify

## 2. Interfaces You Must Implement
```go
// New exported helper for TTY/color detection.
// Returns true if ANSI color codes should be emitted.
// Checks: os.Stdout is a TTY AND NO_COLOR env var is not set.
func IsColorEnabled() bool
```
All existing exported functions retain their signatures.

## 3. Interfaces You May Call
```go
// Already in go.mod as indirect dep — import as direct:
// github.com/mattn/go-isatty
isatty.IsTerminal(fd uintptr) bool

// stdlib
os.Getenv("NO_COLOR")
os.Stdout.Fd()
```

## 4. What to Implement

### [OUTPUT-1] table.go — NO_COLOR / isatty support (audit issue: OUTPUT ANSI leaks)
Add `IsColorEnabled() bool` at the top of `table.go`:
```go
func IsColorEnabled() bool {
    if os.Getenv("NO_COLOR") != "" {
        return false
    }
    return isatty.IsTerminal(os.Stdout.Fd())
}
```
Then wrap every ANSI code emission in the rendering functions with this check.
The pattern to use throughout:
```go
// Instead of: tierColor + label + colorReset
// Use:
label := formatTierLabel(score.Tier, score.IsCritical)
if IsColorEnabled() {
    sb.WriteString(getTierColor(score.Tier) + label + colorReset)
} else {
    sb.WriteString(label)
}
```
Apply to: `RenderConfidenceTable`, `RenderUsageTable`, `RenderTierSummary`,
`RenderReclaimableFooter`, and `RenderConfidenceTableVerbose`.

The constants `colorReset`, `colorGreen`, etc. remain; they are still used
internally. Do not delete them.

### [OUTPUT-2] table.go — remove empty Version column from RenderPackageTable (audit issue: OUTPUT empty Version column)
In `RenderPackageTable`, the current header and rows include a `Version` column
that is always blank. Remove it:
- Header: `%-20s %-8s %-13s %-13s` (Package, Size, Installed, Last Used)
- Row format matches.
- The separator `strings.Repeat("─", 80)` may need width adjustment.

### [OUTPUT-3] table.go — fix Status column for safe/medium packages in RenderConfidenceTable (audit issue: ANALYSIS unused --all Status column)
In `formatTierLabel`:
```go
func formatTierLabel(tier string, isCritical bool) string {
    if isCritical || strings.ToLower(tier) == "risky" {
        return "\u2717 keep"
    }
    return strings.ToUpper(tier)
}
```
Currently, safe packages show `SAFE` (correct) and medium packages show
`MEDIUM` (correct). The `✗ keep` only appears for risky/critical packages.
The audit's complaint (`✗ keep` for safe packages in `--all` mode) may be
a side effect of the score inversion bug (Wave 0): before Wave 0, all packages
scored high and landed in "safe" tier, but the `IsCritical` flag was true for
many packages, causing `✗ keep`.

After Wave 0's fix, verify the behavior. If the bug still exists post Wave 0,
change `formatTierLabel` to:
```go
func formatTierLabel(tier string, isCritical bool) string {
    switch strings.ToLower(tier) {
    case "safe":
        return "✓ safe"
    case "medium":
        return "~ review"
    default: // risky or critical
        return "✗ keep"
    }
}
```
Note: changing the safe label from `SAFE` to `✓ safe` is a format change that
Agent D's tests may depend on. Coordinate with Agent D. If Agent D's tests
check for `"SAFE"` in output, they will break. Use `✓ safe` and update
Agent D's test expectations in your report so Agent D can fix them.

### [OUTPUT-4] table.go — add secondary sort by LastUsed in RenderUsageTable (audit issue: TRACKING stats sort)
In `RenderUsageTable`, the current sort is by `TotalRuns` descending. Add a
secondary sort by `LastUsed` descending (more recent = higher), with zero-time
values (never used) sorted to the bottom:
```go
sort.SliceStable(entries, func(i, j int) bool {
    if entries[i].stats.TotalRuns != entries[j].stats.TotalRuns {
        return entries[i].stats.TotalRuns > entries[j].stats.TotalRuns
    }
    // Secondary: more recently used first; zero time (never) goes last
    iZero := entries[i].stats.LastUsed.IsZero()
    jZero := entries[j].stats.LastUsed.IsZero()
    if iZero != jZero {
        return jZero // non-zero before zero
    }
    return entries[i].stats.LastUsed.After(entries[j].stats.LastUsed)
})
```

### [OUTPUT-5] progress.go — fix duplicate 100% line in ProgressBar (audit issue: REMOVE progress bar duplicate)
In `ProgressBar`, the duplicate final line occurs because `Finish()` calls
`p.render()` (which prints via `\r`) and then `fmt.Fprintln` (which adds a
newline). The caller in `remove.go` also calls `progress.Increment()` for the
last item just before `Finish()`, which renders 100% once, then `Finish()`
renders 100% again.

Fix `Finish()`:
```go
func (p *ProgressBar) Finish() {
    p.mu.Lock()
    defer p.mu.Unlock()
    p.current = p.total
    p.render()          // renders the final 100% frame
    fmt.Fprintln(p.writer) // move to new line — this is the only newline
}
```
The issue is that `Increment()` calls `render()` (which uses `\r`, no newline),
and then `Finish()` calls `render()` again with the same 100% value, then adds
a newline. On a TTY the `\r` overwrites the previous line, so only one 100%
line appears. On a non-TTY (no `\r` overwrite), both renders appear as separate
lines.

Fix: in non-TTY context, suppress intermediate renders in `render()` and only
print on `Finish()`. OR: add a TTY check to `render()`:
```go
func (p *ProgressBar) render() {
    // ... existing percentage/bar computation ...
    if isatty.IsTerminal(p.writer.(interface{ Fd() uintptr }).Fd()) {
        fmt.Fprintf(p.writer, "\r%s %3d%% %s", bar.String(), percentage, p.description)
    } else {
        // Non-TTY: only print on completion (100%)
        if p.current == p.total {
            fmt.Fprintf(p.writer, "%s %3d%% %s\n", bar.String(), percentage, p.description)
        }
    }
}
```
Note: `p.writer` is an `io.Writer` which may not implement `Fd()`. Add a
helper that asserts the interface and falls back to non-TTY if not available.
Also suppress `fmt.Fprintln` in `Finish()` when non-TTY (since render() adds
its own newline in that case).

Similarly, apply TTY detection to `Spinner.Start()`: in `NewSpinner`, if stdout
is not a TTY, skip the goroutine animation and just print `message...` once.
In `Stop()` and `StopWithMessage()`, skip the `\r` clear when not a TTY.

### [OUTPUT-6] progress.go — Spinner non-TTY (also covered by SETUP-1 in Agent A)
Agent A adds inline isatty checks in scan.go **before** creating spinners.
Agent C adds isatty awareness **inside** the Spinner itself as a belt-and-
suspenders fix. These two changes are compatible and do not conflict. Agent C's
spinner fix benefits all callers system-wide; Agent A's fix benefits only scan.

## 5. Tests to Write
- `TestIsColorEnabled_NoColor` (table_test.go) — set NO_COLOR=1 env, verify false
- `TestIsColorEnabled_NonTTY` (table_test.go) — stdout is a pipe, verify false
- `TestRenderPackageTable_NoVersionColumn` (table_test.go) — verify Version not in header
- `TestRenderUsageTable_SortedByRunsThenLastUsed` (table_test.go) — packages with equal runs sorted by LastUsed desc
- `TestProgressBar_NoTTY_NoDuplicateLine` (progress_test.go) — SetWriter to a buffer (non-TTY), run Increment loop + Finish, verify single 100% line

## 6. Verification Gate
```
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/output/...
go test ./...
```

## 7. Constraints
- Do not touch any file outside `internal/output/table.go` and
  `internal/output/progress.go`.
- `IsColorEnabled()` must be exported (capital I) so Agent B can use it in
  explain.go and doctor.go/stats.go.
- Do not change the function signatures of any existing exported functions.
- The `ConfidenceScore` struct in table.go is a **local output-layer struct**
  (not the analyzer's struct). Do not rename or add fields unless required.
- The `formatTierLabel` change (OUTPUT-3) changes `"SAFE"` → `"✓ safe"` and
  `"MEDIUM"` → `"~ review"`. Agent D's code and tests that check for `"SAFE"`
  in table output must be updated. Report this in your output so Agent D knows.
- `isatty` is already in go.mod as indirect dep. Adding it as a direct import
  in table.go and progress.go does not require go.mod changes.

## 8. Report
When done, report: which rendering functions now respect NO_COLOR/isatty,
whether formatTierLabel was changed and what Agent D needs to update, and
test results.

---

## Wave 2 Agent D Prompt

# Wave 2 Agent D: Core analysis UX fixes

You are Wave 2 Agent D. Your task is to fix 6 UX issues in unused, remove, and
root commands.

## 1. File Ownership
You own these files. Do not touch any other files.
- `internal/app/unused.go` — modify
- `internal/app/remove.go` — modify
- `internal/app/root.go` — modify

## 2. Interfaces You Must Implement
No new exported functions. Existing RunE signatures unchanged.
You depend on `output.IsColorEnabled()` added by Agent C (Wave 2, same wave).
Wave 2 agents run in parallel; do not assume Agent C's changes are present
when you build. If `IsColorEnabled` is not yet available, stub it as:
```go
// temporary stub — will be provided by output package
func colorEnabled() bool { return true }
```
and replace with `output.IsColorEnabled()` before final commit.

## 3. Interfaces You May Call
```go
// From output package (Agent C adds IsColorEnabled, all else pre-exists)
output.IsColorEnabled() bool
output.RenderTierSummary(safe, medium, risky output.TierStats, showAll bool, caskCount int) string
output.RenderConfidenceTable(scores []output.ConfidenceScore) string
output.RenderReclaimableFooter(safe, medium, risky output.TierStats, showAll bool) string

// From store package
store.GetLastUsage(pkg string) (*time.Time, error)

// From analyzer package (post Wave 0 — correct score logic)
analyzer.New(st *store.Store) *analyzer.Analyzer
func (a *Analyzer) ComputeScore(pkg string) (*ConfidenceScore, error)
func (a *Analyzer) GetPackagesByTier(tier string) ([]*ConfidenceScore, error)
```

## 4. What to Implement

### [ROOT-1] root.go — bare invocation shows short nudge, not full help (audit issue: DISCOVERY no-args shows full help)
Set a `RunE` on `RootCmd` that checks if any arguments were provided and
prints a short usage hint instead of the full help:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    dbPath, _ := getDBPath()
    if _, err := os.Stat(dbPath); os.IsNotExist(err) {
        fmt.Println("brewprune: Homebrew package cleanup with usage tracking")
        fmt.Println()
        fmt.Println("Run 'brewprune quickstart' to get started.")
        fmt.Println("Run 'brewprune --help' for the full reference.")
    } else {
        fmt.Println("brewprune: Homebrew package cleanup with usage tracking")
        fmt.Println()
        fmt.Println("Tip: Run 'brewprune status' to check tracking status.")
        fmt.Println("     Run 'brewprune unused' to view recommendations.")
        fmt.Println("     Run 'brewprune --help' for all commands.")
    }
    return nil
},
```
Do not remove the Long description — it is still used by `brewprune --help`.

### [ROOT-2] root.go — unknown subcommand shows available commands (audit issue: DISCOVERY unknown subcommand)
Add a `PersistentPreRunE` or use cobra's `SuggestionsMinimumDistance` and
`DisableSuggestions` options. Cobra already supports suggestions via
`cmd.SuggestionsFor(args[0])`. Enable suggestions on RootCmd:
```go
RootCmd.SuggestFor = []string{} // ensure cobra suggestions are enabled
```
Cobra's suggestion feature is enabled by default when `DisableSuggestions`
is false. To also print the list of available commands on unknown subcommand,
set a custom `RunE` on RootCmd (done in ROOT-1 above) won't help here because
unknown commands never reach RunE.

Instead, use cobra's built-in mechanism: cobra already prints
`"unknown command X for Y"` and with suggestions enabled it will print
`"Did you mean this?\n  <suggestion>"`. This is cobra's default behavior
when `DisableSuggestions` is false (it is false by default). Verify
`SilenceUsage: true` and `SilenceErrors: true` are already set and that
cobra's suggestion distance is set reasonably:
```go
RootCmd.SuggestionsMinimumDistance = 2
```
This gives "did you mean" for close typos. For listing all commands, also
add to the root's `Long` description a note, or implement a custom
`FParseErrWhitelist` or `SetFlagErrorFunc`. The minimal correct fix is:
```go
RootCmd.SuggestionsMinimumDistance = 2
```
Cobra will automatically append `"Did you mean this?\n  <closest command>"` to
the unknown command error, which satisfies the audit requirement.

### [UNUSED-1] unused.go — show risky tier by default when no usage data exists (audit issue: ANALYSIS unused shows nothing)
In `runUnused`, when no tier flag and no `--all` flag are set, risky packages
are hidden. The audit says this is the wrong default when there is no usage
data at all (the user's first run after scan).

Add logic: if `unusedTier == ""` and `!unusedAll` and `eventCount == 0` (no
usage data), **implicitly treat risky as visible** and add a prominent banner:
```go
// After checkUsageWarning(st):
var eventCount int
row := st.DB().QueryRow("SELECT COUNT(*) FROM usage_events")
row.Scan(&eventCount)

showRiskyImplicit := (unusedTier == "" && !unusedAll && eventCount == 0)
```
Then in the filter loop:
```go
if !unusedAll && unusedTier == "" && s.Tier == "risky" && !showRiskyImplicit {
    continue
}
```
And in the tier summary call:
```go
fmt.Println(output.RenderTierSummary(safeTier, mediumTier, riskyTier, unusedAll || unusedTier != "" || showRiskyImplicit, caskCount))
```
Add a distinct banner when `showRiskyImplicit`:
```
⚠ No usage data yet — showing all packages (risky tier included).
  Run 'brewprune watch --daemon' and wait 1-2 weeks for better recommendations.
  Use 'brewprune unused --all' to always show all tiers.
```

### [UNUSED-2] unused.go — distinct message for --casks with no casks (audit issue: ANALYSIS unused --casks no message)
After the empty-result check in `runUnused`:
```go
if len(scores) == 0 {
    if unusedCasks {
        // Count total casks
        if caskCount == 0 {
            fmt.Println("No casks installed.")
        } else {
            fmt.Printf("No casks match the specified criteria (%d cask(s) installed).\n", caskCount)
        }
    } else {
        fmt.Println("No packages match the specified criteria.")
    }
    return nil
}
```

### [UNUSED-3] unused.go — document --tier risky implicit behavior (audit issue: EDGE tier risky vs no flags)
In `unusedCmd.Long`, add a note:
```
Note: specifying --tier shows that tier regardless of --all. Running
'brewprune unused --tier risky' shows all risky packages without needing --all.
```

### [REMOVE-1] remove.go — fix "Last Used" showing never (audit issue: REMOVE last used never)
In `displayConfidenceScores`, the `LastUsed` field is hard-coded to
`getNeverTime()` (returns `time.Time{}`):
```go
outputScores[i] = output.ConfidenceScore{
    ...
    LastUsed: getNeverTime(),  // BUG: always zero
    ...
}
```
Fix: use `getLastUsed(st, score.Package)` (already defined in `unused.go` in
the same package):
```go
LastUsed: getLastUsed(st, score.Package),
```

### [REMOVE-2] remove.go — dry-run hint in no-tier error (audit issue: REMOVE no flags no hint)
In `runRemove`, when `tier == ""`:
```go
return fmt.Errorf("no tier specified: use --safe, --medium, or --risky")
```
Change to:
```go
return fmt.Errorf("no tier specified. Use --safe, --medium, or --risky. Add --dry-run to preview changes first.")
```

### [REMOVE-3] remove.go / unused.go — flag consistency for --tier (audit issue: DISCOVERY flag inconsistency)
`unused` uses `--tier <value>`. `remove` uses `--safe`, `--medium`, `--risky`.
The audit recommends making them consistent. The minimal fix without breaking
backward compat: add `--tier <value>` as an **alias** on `removeCmd`:
```go
removeCmd.Flags().StringVar(&removeTierFlag, "tier", "", "Remove packages of specified tier: safe, medium, risky (alias for --safe/--medium/--risky)")
```
In `determineTier()`:
```go
func determineTier() string {
    if removeTierFlag != "" {
        return removeTierFlag
    }
    if removeFlagRisky { return "risky" }
    if removeFlagMedium { return "medium" }
    if removeFlagSafe   { return "safe" }
    return ""
}
```
Add `removeTierFlag string` to the var block. This is backward compatible —
existing `--safe`/`--medium`/`--risky` flags continue to work, and `--tier
medium` now also works.

## 5. Tests to Write
- `TestRunUnused_NoUsageDataShowsRisky` (unused_test.go) — when no events in DB
  and no flags, risky packages are shown
- `TestRunUnused_CasksNoCasksInstalledMessage` (unused_test.go) — --casks with
  0 casks shows "No casks installed"
- `TestDetermineTier_TierFlag` (remove_test.go or new) — --tier safe sets tier
  to "safe"
- `TestDisplayConfidenceScores_LastUsedNotNever` (remove_test.go) — when package
  has usage data, LastUsed is not zero in output
- `TestRootCmd_BareInvocationPrintsHint` (root_test.go) — bare invocation prints
  quickstart hint, not full help text

## 6. Verification Gate
```
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/app/...
go test ./...
```

## 7. Constraints
- Do not touch any file outside `unused.go`, `remove.go`, `root.go`.
- If Agent C changes `formatTierLabel` so that safe packages show `"✓ safe"`
  instead of `"SAFE"`, update any test in `unused_test.go` or `remove_test.go`
  that asserts on table output strings accordingly. Agent C's report will
  document this.
- Adding `--tier` to `removeCmd` must validate its value against
  `{safe, medium, risky}` before passing to `determineTier`. Return an error
  for invalid values.
- Do not change `getPackagesByTier()` or `displayConfidenceScores()` signatures.
- The `showRiskyImplicit` logic in [UNUSED-1] must not change behavior when
  `--all` or `--tier` is explicitly set. The implicit risky display only
  triggers when the user ran bare `brewprune unused` with no filtering flags
  and zero usage events.

## 8. Report
When done, report: which issues fixed, whether Agent C's formatTierLabel change
required test updates, and the `--tier` flag addition to remove's interface.

---

## Issue-to-Agent Mapping

| Audit Issue | Severity | Agent | File(s) |
|---|---|---|---|
| DISCOVERY bare invocation shows full help | UX-improvement | D | root.go |
| DISCOVERY unknown subcommand no suggestion | UX-improvement | D | root.go |
| DISCOVERY explain no arg cryptic error | UX-polish | B | explain.go |
| DISCOVERY remove/unused flag inconsistency | UX-improvement | D | remove.go |
| SETUP scan spinner garbage non-TTY | UX-critical | A + C | scan.go, progress.go |
| SETUP scan "0 shims created" on re-scan | UX-improvement | A | scan.go |
| SETUP status suggests brew services | UX-improvement | A | status.go |
| SETUP quickstart alarming warning | UX-improvement | A | quickstart.go |
| SETUP watch --daemon fails if already running | UX-improvement | A | watch.go |
| ANALYSIS unused shows nothing no usage data | UX-critical | D | unused.go |
| ANALYSIS unused --casks no distinct message | UX-polish | D | unused.go |
| ANALYSIS score logic inverted | UX-critical | 0 | confidence.go |
| ANALYSIS unused --all Status column wrong | UX-improvement | C | table.go |
| TRACKING PATH setup easy to miss | UX-critical | A | scan.go |
| TRACKING stats not sorted | UX-polish | C | table.go |
| TRACKING stats --package unstyled | UX-polish | B | stats.go |
| EXPLAIN table footer alignment | UX-polish | B | explain.go |
| EXPLAIN error printed twice | UX-polish | B | explain.go |
| DOCTOR exit code 1 for non-critical | UX-improvement | B | doctor.go |
| DOCTOR error duplicated | UX-polish | B | doctor.go |
| REMOVE last used shows never | UX-improvement | D | remove.go |
| REMOVE no flags no dry-run hint | UX-polish | D | remove.go |
| REMOVE progress bar duplicate line | UX-polish | C | progress.go |
| OUTPUT ANSI leaks non-color terminals | UX-improvement | C | table.go |
| OUTPUT scan emoji style inconsistency | UX-polish | A | scan.go |
| OUTPUT scan empty Version column | UX-polish | A+C | scan.go (no-op), table.go |
| EDGE undo latest terse error | UX-polish | B | undo.go |
| EDGE unused --tier risky implicit behavior | UX-improvement | D | unused.go |

(28 rows because some audit items map to multiple sub-fixes or the 21 audit
items expand when broken into implementation tasks.)

---

## File Ownership Summary (no overlap within any wave)

| Wave | Agent | Files owned |
|---|---|---|
| 0 | 0 | `internal/analyzer/confidence.go`, `internal/analyzer/confidence_test.go` |
| 1 | A | `internal/app/scan.go`, `internal/app/quickstart.go`, `internal/app/watch.go`, `internal/app/status.go` |
| 1 | B | `internal/app/explain.go`, `internal/app/doctor.go`, `internal/app/stats.go`, `internal/app/undo.go` |
| 2 | C | `internal/output/table.go`, `internal/output/progress.go` |
| 2 | D | `internal/app/unused.go`, `internal/app/remove.go`, `internal/app/root.go` |

No file appears in more than one agent's ownership list. Wave 1 agents do not
touch Wave 2 files and vice versa.

---

## Cross-Wave Dependencies

| Dependency | Direction | Risk |
|---|---|---|
| Wave 0 inverts score → all tiers change | 0 → 1,2 | Low — agents test against correct behavior |
| Agent C adds `IsColorEnabled()` | C → D (same wave) | Medium — Agent D stubs it until C lands |
| Agent C changes `formatTierLabel` output strings | C → D (same wave) | Medium — Agent D must update string assertions |
| Agent A makes watch `--daemon` return nil when running | A → quickstart | Low — quickstart still works, just dead code path |
| Agent B uses `isatty` import | B + A both add import | Low — same package, no conflict |
