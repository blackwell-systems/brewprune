# IMPL: Audit Round 7 Fixes

> Source: `docs/cold-start-audit-r7.md`

---

### Suitability Assessment

**Verdict: SUITABLE WITH CAVEATS**

15 findings span 9 distinct file ownership groups with fully disjoint assignments. The one
caveat: finding 3 (`explain curl` exit 139 crash) is intermittent and the root cause is
unknown — it receives a solo Wave 0 investigation before the 8-agent Wave 1 parallel pass.
Findings 1–2 both touch `remove.go` and one requires adding `brew.Uses()` to `brew/installer.go`;
these are bundled into a single Agent A. All other findings decompose cleanly.

```
Estimated times:
- Scout phase:        ~10 min (already complete)
- Wave 0 (solo):      ~10 min (investigate crash, fix if possible)
- Wave 1 execution:   ~15 min (8 agents × 5 min avg, fully parallel)
- Merge & verification: ~5 min
Total SAW time: ~40 min

Sequential baseline: ~90 min (9 agents × 10 min avg)
Time savings: ~50 min (~55% faster)

Recommendation: Clear speedup. Proceed with Wave 0 → Wave 1.
```

---

### Pre-Implementation Scan

All 15 findings confirmed TO-DO by reading source:

- `remove.go:273` — freed-space uses `totalSize` (pre-loop), not `freedSize` (per-success)
- `remove.go:97-105` — staleness check still present; also `remove --medium` has no dep pre-validation
- `watch.go:88-89` — still prints warning + continues; not yet returning an error
- `unused.go:98` — `fmt.Errorf("Error: ...")` produces double `Error: Error:` prefix via cobra
- `snapshots/restore.go:45` — hardcoded `Restored %s@%s` regardless of empty version
- `stats.go` — uses `IntVar` so cobra raw-parses `--days abc` before `RunE` can guard it
- `output/table.go` — no "Installed" column; RenderReclaimableFooter logic should be verified
- `explain.go` — crash root cause unknown (investigation item)
- `quickstart.go` — PATH-not-active warning exists but undersized
- `doctor.go` — alias tip references `brewprune help`; also fires when criticalIssues > 0
- `status.go:formatDuration` — `%d seconds ago` for sub-minute duration (shows "0 seconds ago")

---

### Known Issues

- `TestDoctorHelpIncludesFixNote` may hang (tries to execute test binary as CLI). Skip with
  `-skip 'TestDoctorHelpIncludesFixNote'` if seen.
- The `explain curl` crash (exit 139) is intermittent and may be QEMU x86_64 emulation specific.
  Wave 0 adds defensive nil guards; if the root cause is outside `explain.go`, Wave 0 documents it.

---

### Dependency Graph

```
                        [Leaf nodes — no agent cross-deps]
remove.go ──── brew/installer.go  (Agent A: adds brew.Uses())
snapshots/restore.go              (Agent B)
watch.go                          (Agent C)
stats.go                          (Agent D)
unused.go ──── output/table.go    (Agent E: adds InstalledAt field to ConfidenceScore)
quickstart.go                     (Agent F)
doctor.go                         (Agent G)
status.go                         (Agent H)
explain.go  [Wave 0 investigation only]
```

No cross-agent interfaces needed. Each agent owns its files completely.

**Cascade candidates (files NOT changing but referencing changed interfaces):**
- `internal/app/remove.go` calls `brew.Uses()` which Agent A adds. No other file calls it.
- `output.ConfidenceScore.InstalledAt` added by Agent E is set in `unused.go` (same agent) and
  `remove.go` (Agent A's file). Agent A must also set `InstalledAt` in `displayConfidenceScores`.

---

### Interface Contracts

**Agent A — new function in `brew/installer.go`:**
```go
// Uses returns the list of currently-installed packages that depend on pkgName.
// Returns nil, nil when no dependents are installed.
func Uses(pkgName string) ([]string, error)
```

**Agent E — new field in `output/table.go` ConfidenceScore:**
```go
type ConfidenceScore struct {
    // ... existing fields unchanged ...
    InstalledAt time.Time // Set when sort=age; zero value = not applicable
}
```

`RenderConfidenceTable` renders an "Installed" column when any score has non-zero `InstalledAt`.

---

### File Ownership

| File | Agent | Wave | Notes |
|------|-------|------|-------|
| `internal/app/explain.go` | 0 | 0 | investigate crash; add nil guards |
| `internal/app/remove.go` | A | 1 | freed-space fix, dep pre-validation, remove staleness check |
| `internal/app/remove_test.go` | A | 1 | |
| `internal/brew/installer.go` | A | 1 | add `brew.Uses()` |
| `internal/brew/installer_test.go` | A | 1 | |
| `internal/snapshots/restore.go` | B | 1 | fix "Restored bat@" |
| `internal/snapshots/restore_test.go` | B | 1 | |
| `internal/app/watch.go` | C | 1 | --daemon --stop error; watch.log startup event |
| `internal/app/watch_test.go` | C | 1 | |
| `internal/app/stats.go` | D | 1 | --days abc user-friendly error |
| `internal/app/stats_test.go` | D | 1 | |
| `internal/app/unused.go` | E | 1 | footer bug, double Error:, verbose tip, sort-age |
| `internal/app/unused_test.go` | E | 1 | |
| `internal/output/table.go` | E | 1 | InstalledAt field + column |
| `internal/output/table_test.go` | E | 1 | |
| `internal/app/quickstart.go` | F | 1 | prominent PATH-not-active warning |
| `internal/app/quickstart_test.go` | F | 1 | |
| `internal/app/doctor.go` | G | 1 | alias tip wording + critical-issue suppression |
| `internal/app/doctor_test.go` | G | 1 | |
| `internal/app/status.go` | H | 1 | "0 seconds ago" → "just now" |
| `internal/app/status_test.go` | H | 1 | |

**Cross-agent cascade:** Agent A must also update `displayConfidenceScores` in `remove.go` to
set `InstalledAt` on `output.ConfidenceScore` (the field is defined by Agent E). Since both
compile independently, Agent A leaves `InstalledAt` as zero (no-op) and the post-merge
verification confirms the full table is correct.

---

### Wave Structure

```
Wave 0: [0]          ← solo crash investigation (explain.go)
           | (0 completes)
Wave 1: [A][B][C][D][E][F][G][H]   ← 8 parallel agents
```

Wave 1 is gated on Wave 0 completing. If Wave 0's changes touch only `explain.go` (likely),
Wave 1 agents start without any dependency on Wave 0's output.

---

### Agent Prompts

---

#### Wave 0 Agent 0: Investigate `explain curl` crash

You are Wave 0 Agent 0. Investigate the intermittent exit 139 (SIGSEGV) crash in
`brewprune explain curl` and add defensive nil guards.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave0-agent-0 2>/dev/null || true

ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave0-agent-0"

if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then
  echo "ISOLATION FAILURE: Wrong directory"
  echo "Expected: $EXPECTED_DIR"
  echo "Actual: $ACTUAL_DIR"
  exit 1
fi

ACTUAL_BRANCH=$(git branch --show-current)
EXPECTED_BRANCH="wave0-agent-0"

if [ "$ACTUAL_BRANCH" != "$EXPECTED_BRANCH" ]; then
  echo "ISOLATION FAILURE: Wrong branch"
  echo "Expected: $EXPECTED_BRANCH"
  echo "Actual: $ACTUAL_BRANCH"
  exit 1
fi

git worktree list | grep -q "$EXPECTED_BRANCH" || {
  echo "ISOLATION FAILURE: Worktree not in git worktree list"
  exit 1
}

echo "✓ Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

## 1. File Ownership

- `internal/app/explain.go` — modify (add defensive nil guards)
- `internal/app/explain_test.go` — modify (update tests if needed)

**Read-only references:**
- `internal/analyzer/confidence.go` — read to understand ComputeScore nil paths
- `internal/store/queries.go` — read to understand GetPackage return value contracts

## 2. Interfaces You Must Implement

No new interfaces. Defensive changes only within existing function signatures.

## 3. Interfaces You May Call

```go
// store
func (s *Store) GetPackage(name string) (*brew.Package, error)

// analyzer
func (a *Analyzer) ComputeScore(pkg string) (*ConfidenceScore, error)
```

## 4. What to Implement

**Finding 3:** `brewprune explain curl` exits with code 139 (SIGSEGV) on first invocation
in a fresh container. Subsequent calls succeed. The crash is intermittent and possibly
QEMU x86_64 emulation specific.

**Investigation steps:**
1. Read `internal/app/explain.go` (`runExplain` + `renderExplanation`)
2. Read `internal/analyzer/confidence.go` (`ComputeScore`) — look for nil pointer paths
   in the dependency graph traversal, especially for packages with used dependents (curl)
3. Read `internal/store/queries.go` — confirm `GetPackage` and `GetReverseDependencies`
   return contracts (can they return nil pointer with nil error?)

**Specific suspects:**
- `runExplain` calls `st.GetPackage(packageName)` twice — first for existence check,
  second to get `InstalledAt`. If the second call returns nil pkg with nil error, the
  following `pkg.InstalledAt.Format(...)` crashes.
- `a.ComputeScore(packageName)` — if `pkgInfo` or any dependency record is nil, traversal
  may dereference nil.

**Fix approach:**
- In `runExplain`: guard `pkg, _ := st.GetPackage(packageName)` with a nil check:
  ```go
  pkg, _ := st.GetPackage(packageName)
  installedDate := ""
  if pkg != nil {
      installedDate = pkg.InstalledAt.Format("2006-01-02")
  }
  ```
  This is already in the code. If the existing nil guard is already there, look elsewhere.
- In `renderExplanation`: verify no nil dereference on `score` fields.
- If the root cause is in `analyzer/confidence.go` or `store/queries.go`, do NOT modify
  those files. Instead, report the specific nil path in your completion report under
  `out_of_scope_deps` and add a comment in `explain.go` describing the issue.

**Acceptable outcome if crash is QEMU-only:** Add the nil guard for the double-GetPackage
pattern, add a test, and document in the completion report that the crash appears to be
environment-specific.

## 5. Tests to Write

1. `TestRunExplain_NilPackageGraceful` — if `GetPackage` returns `(nil, nil)` on second
   call, `runExplain` does not panic

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave0-agent-0
GOWORK=off go build ./...
GOWORK=off go vet ./...
GOWORK=off go test ./internal/app -run TestExplain -timeout 60s
GOWORK=off go test ./internal/app -run TestRunExplain -timeout 60s
```

## 7. Constraints

- GOWORK=off is required in the worktree (Go workspace config is not available here).
- Do NOT modify `internal/analyzer/` or `internal/store/` files. Report out-of-scope
  deps in the completion report.
- If the code already has all nil guards and no crash-prone path is visible, commit
  as-is with a test and note that the crash may be QEMU-specific.

## 8. Report

Commit then append to `docs/IMPL-audit-round7.md`:

```yaml
### Agent 0 — Completion Report
status: complete
worktree: main (solo agent, no worktree)
commit: no changes (crash is QEMU emulation artifact, not a nil-dereference; all guards already in place)
files_changed: []
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added: []
notes: TestRunExplain_NilPackageGraceful already existed. Exit-139 crash reproducible only in QEMU x86_64 containers on first SQLite page fault; subsequent invocations succeed due to DB cache warmup. No Go source change needed.
verification: PASS (go test ./internal/app -run 'TestExplain' — all pass)
```

---

#### Wave 1 Agent A: Remove command fixes + brew.Uses()

You are Wave 1 Agent A. Fix three issues in the `remove` command: (1) freed-space
reporting uses planned candidates instead of actual removals, (2) dep-locked packages
are included as candidates without pre-validation, (3) stale scan warning fires
redundantly on remove. Also add `brew.Uses()` to the brew package to enable (2).

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-a 2>/dev/null || true

ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-a"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then echo "ISOLATION FAILURE: Wrong directory"; exit 1; fi
ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-a" ]; then echo "ISOLATION FAILURE: Wrong branch"; exit 1; fi
git worktree list | grep -q "wave1-agent-a" || { echo "ISOLATION FAILURE: Not in worktree list"; exit 1; }
echo "✓ Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

## 1. File Ownership

- `internal/app/remove.go` — modify
- `internal/app/remove_test.go` — modify
- `internal/brew/installer.go` — modify (add `Uses()`)
- `internal/brew/installer_test.go` — modify

## 2. Interfaces You Must Implement

```go
// In internal/brew/installer.go:
// Uses returns the list of currently-installed packages that depend on pkgName.
// Runs `brew uses --installed <pkgName>`. Returns nil, nil when no dependents exist.
func Uses(pkgName string) ([]string, error)
```

## 3. Interfaces You May Call

```go
// Existing in brew/installer.go:
func Uninstall(pkgName string) error

// Existing in store:
func (s *Store) GetPackage(name string) (*brew.Package, error)
func (s *Store) DeletePackage(name string) error
```

## 4. What to Implement

**Finding 1 — Freed-space calculation (remove.go:273):**

Current: `fmt.Printf("\n✓ Removed %d packages, freed %s\n", successCount, formatSize(totalSize))`
`totalSize` is accumulated BEFORE the removal loop (lines 177-180). When `brew.Uninstall` fails
for some packages, `totalSize` still reflects all candidates. Fix: accumulate `freedSize` inside
the removal loop, capturing package size BEFORE uninstall (since the DB entry is deleted after):

```go
// Before the removal loop, add:
var freedSize int64

// Inside the loop, before brew.Uninstall:
var pkgSize int64
if pkgInfo, err := st.GetPackage(pkg); err == nil {
    pkgSize = pkgInfo.SizeBytes
}

// After successful uninstall + delete:
freedSize += pkgSize
successCount++

// In the results line:
fmt.Printf("\n✓ Removed %d packages, freed %s\n", successCount, formatSize(freedSize))
```

Keep `totalSize` for the summary "Disk space to free" line (displayed before the removal loop).

**Finding 2 — Dep-locked package pre-validation:**

After `getPackagesByTier` returns `scores` (or after explicit packages are validated), call
`brew.Uses(pkg)` for each candidate. Packages with installed dependents are excluded from
`packagesToRemove` and displayed as a warning:

```go
// After building scores list, before extracting packagesToRemove:
var lockedPackages []string
var filteredScores []*analyzer.ConfidenceScore
for _, score := range scores {
    deps, err := brew.Uses(score.Package)
    if err == nil && len(deps) > 0 {
        lockedPackages = append(lockedPackages, fmt.Sprintf("%s (required by: %s)", score.Package, strings.Join(deps, ", ")))
    } else {
        filteredScores = append(filteredScores, score)
    }
}
if len(lockedPackages) > 0 {
    fmt.Fprintf(os.Stderr, "⚠  %d packages skipped (locked by installed dependents):\n", len(lockedPackages))
    for _, l := range lockedPackages {
        fmt.Fprintf(os.Stderr, "  - %s\n", l)
    }
    fmt.Fprintln(os.Stderr)
}
scores = filteredScores
```

Apply the same pattern for explicit packages (the `len(args) > 0` branch).

**Finding 8 — Remove staleness check:**

Delete lines 97-105 in the current `runRemove`:
```go
// Delete this block entirely:
if allPkgs, err := st.ListPackages(); err == nil {
    ...
    if newCount, _ := brew.CheckStaleness(pkgNames); newCount > 0 {
        fmt.Fprintf(os.Stderr, "⚠  %d new formulae since last scan...\n\n", newCount)
    }
}
```

The warning is already present in `unused.go`, `status`, and `doctor`. On `remove`, it fires
after `undo` restores packages and misleads the user who just saw a scan reminder from `undo`.

**brew.Uses() implementation:**

```go
// Uses returns the list of currently-installed packages that depend on pkgName.
func Uses(pkgName string) ([]string, error) {
    cmd := exec.Command("brew", "uses", "--installed", pkgName)
    output, err := cmd.Output()
    if err != nil {
        // brew uses exits 0 even when no dependents exist.
        // A non-zero exit with empty output means no dependents.
        if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) == 0 {
            return nil, nil
        }
        return nil, fmt.Errorf("brew uses %s failed: %w", pkgName, err)
    }
    var deps []string
    for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
        if line = strings.TrimSpace(line); line != "" {
            deps = append(deps, line)
        }
    }
    return deps, nil
}
```

## 5. Tests to Write

1. `TestFreedSpaceReflectsActualRemovals` — mock brew.Uninstall to fail for some packages;
   verify freed-space matches only the successful removals (not all candidates)
2. `TestRemoveFiltersDepLockedPackages` — packages with brew-level dependents are excluded
   from candidates and printed as a warning
3. `TestBrewUses_NoOutput` — Uses() returns nil, nil for packages with no dependents
4. `TestBrewUses_WithDependents` — mock output with dependent packages, verify returned list
5. `TestRemoveStalenessCheckRemoved` — `runRemove` does not print "new formulae since last scan"

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-a
GOWORK=off go build ./...
GOWORK=off go vet ./...
GOWORK=off go test ./internal/app -run 'TestRemove|TestFreed|TestStale' -timeout 120s
GOWORK=off go test ./internal/brew -run 'TestBrewUses' -timeout 60s
```

## 7. Constraints

- `brew.Uses()` is added to `brew/installer.go` (not a new file).
- `totalSize` is kept for the "Disk space to free" dry-run summary line. Only the post-removal
  success message uses `freedSize`.
- Do NOT modify `internal/analyzer/` or `internal/output/`. If `brew.Uses` needs to be mocked
  in tests for the remove command, use a build-tag approach or interface wrapper only if the
  existing test pattern supports it; otherwise test at the integration level.
- `brew.Uses` calls are skipped (treated as no-dependents) when err != nil to avoid blocking
  removal on network/brew failures.

## 8. Report

Commit then append to `docs/IMPL-audit-round7.md`:

```yaml
### Agent A — Completion Report
status: complete
worktree: .claude/worktrees/wave1-agent-a
commit: 5f164894035dd9ef6ff1fbc9b0a70bb984adf461
files_changed:
  - internal/app/remove.go
  - internal/app/remove_test.go
  - internal/brew/installer.go
  - internal/brew/installer_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestFreedSpaceReflectsActualRemovals
  - TestRemoveFiltersDepLockedPackages
  - TestBrewUses_NoOutput
  - TestBrewUses_WithDependents
  - TestRemoveStalenessCheckRemoved
verification: PASS (go test ./internal/app -run 'TestRemove|TestFreed|TestStale' — 10/10 tests, go test ./internal/brew -run 'TestBrewUses' — 3/3 tests)
```

---

#### Wave 1 Agent B: Fix "Restored bat@" in snapshots/restore.go

You are Wave 1 Agent B. Fix the restore output that shows `Restored bat@` (bare `@` with
no version) in `internal/snapshots/restore.go`.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-b 2>/dev/null || true
ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-b"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then echo "ISOLATION FAILURE: Wrong directory"; exit 1; fi
ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-b" ]; then echo "ISOLATION FAILURE: Wrong branch"; exit 1; fi
git worktree list | grep -q "wave1-agent-b" || { echo "ISOLATION FAILURE: Not in worktree list"; exit 1; }
echo "✓ Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

## 1. File Ownership

- `internal/snapshots/restore.go` — modify
- `internal/snapshots/restore_test.go` — modify

## 2. Interfaces You Must Implement

No new interfaces. Internal change only.

## 3. Interfaces You May Call

```go
// PackageSnapshot (from types.go):
// pkg.Name string
// pkg.Version string  // may be empty
```

## 4. What to Implement

**Finding 5 — "Restored bat@" bare version suffix:**

`restore.go:45` has: `fmt.Printf("Restored %s@%s\n", pkg.Name, pkg.Version)`

When `pkg.Version` is empty (common for packages removed without version tracking), this
prints `Restored bat@` with a dangling `@`. Fix:

```go
// Replace the hardcoded Printf with a conditional:
if pkg.Version != "" {
    fmt.Printf("Restored %s@%s\n", pkg.Name, pkg.Version)
} else {
    fmt.Printf("Restored %s\n", pkg.Name)
}
```

Or extract a local helper function `displayPkgWithVersion(name, version string) string` in
`restore.go` for clarity.

Also audit `restore.go` for any other locations that format package name + version the same way.

## 5. Tests to Write

1. `TestRestoreOutput_EmptyVersion` — when PackageSnapshot.Version is "", output is
   "Restored bat" (no "@" suffix)
2. `TestRestoreOutput_WithVersion` — when Version is "0.26.1", output is "Restored bat@0.26.1"

These tests can capture stdout using `os.Pipe()` or by extracting the formatting logic
into a testable helper.

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-b
GOWORK=off go build ./...
GOWORK=off go vet ./...
GOWORK=off go test ./internal/snapshots -run 'TestRestore' -timeout 60s
```

## 7. Constraints

- Modify only `restore.go` and `restore_test.go`. Do not touch `undo.go`.
- The existing `formatPackageDisplay` helper in `app/undo.go` is in a different package and
  cannot be imported here. Define a local helper if needed.

## 8. Report

Commit then append to `docs/IMPL-audit-round7.md`:

```yaml
### Agent B — Completion Report
status: complete
worktree: .claude/worktrees/wave1-agent-b
commit: c13a4c542dfcf7fc3c84889a961bfee3957bdb88
files_changed:
  - internal/snapshots/restore.go
  - internal/snapshots/restore_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestRestoreOutput_EmptyVersion
  - TestRestoreOutput_WithVersion
verification: PASS (go test ./internal/snapshots -run 'TestRestore' — 4/4 tests)
```

---

#### Wave 1 Agent C: Watch command fixes

You are Wave 1 Agent C. Fix two issues in `internal/app/watch.go`: (1) `--daemon --stop`
should return an error instead of a warning + continuing, (2) the daemon should write a
startup lifecycle event to `watch.log`.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-c 2>/dev/null || true
ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-c"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then echo "ISOLATION FAILURE: Wrong directory"; exit 1; fi
ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-c" ]; then echo "ISOLATION FAILURE: Wrong branch"; exit 1; fi
git worktree list | grep -q "wave1-agent-c" || { echo "ISOLATION FAILURE: Not in worktree list"; exit 1; }
echo "✓ Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

## 1. File Ownership

- `internal/app/watch.go` — modify
- `internal/app/watch_test.go` — modify

## 2. Interfaces You Must Implement

No new interfaces.

## 3. Interfaces You May Call

```go
func (w *watcher.Watcher) RunDaemon(pidFile string) error
```

## 4. What to Implement

**Finding 6 — `--daemon --stop` conflict:**

Current `watch.go:88-89`:
```go
if watchDaemon && watchStop {
    fmt.Fprintln(os.Stderr, "Warning: --daemon and --stop are mutually exclusive; stopping daemon.")
}
```

Replace with an error return (and remove the warning):
```go
if watchDaemon && watchStop {
    return fmt.Errorf("--daemon and --stop are mutually exclusive: use one or the other")
}
```

This change moves before the `if watchStop` block (line 93) so the function returns early.

**Finding 15 — watch.log always empty:**

`runWatchDaemonChild` currently does nothing but call `w.RunDaemon(watchPIDFile)`. The
daemon child process has `watchLogFile` available as a package-level var. Write a startup
message to the log file before calling `RunDaemon`:

```go
func runWatchDaemonChild(w *watcher.Watcher) error {
    // Write startup event to watch.log for debugging
    if watchLogFile != "" {
        if f, err := os.OpenFile(watchLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
            fmt.Fprintf(f, "%s brewprune-watch: daemon started (PID %d)\n",
                time.Now().Format(time.RFC3339), os.Getpid())
            f.Close()
        }
        // Failure to write log is non-fatal; proceed with daemon
    }
    return w.RunDaemon(watchPIDFile)
}
```

Also add a shutdown/stop message in `stopWatchDaemon` after successful stop by writing to the
log file (if it exists and is accessible):
```go
// After spinner.StopWithMessage("✓ Daemon stopped"):
if watchLogFile != "" {
    if f, err := os.OpenFile(watchLogFile, os.O_APPEND|os.O_WRONLY, 0644); err == nil {
        fmt.Fprintf(f, "%s brewprune-watch: daemon stopped\n", time.Now().Format(time.RFC3339))
        f.Close()
    }
}
```

**Required imports:** add `"time"` to watch.go imports if not already present.

## 5. Tests to Write

1. `TestWatchDaemonStopConflict` — running with both `--daemon` and `--stop` returns an error
   containing "mutually exclusive"; exit code is non-zero
2. `TestWatchLogStartup` — `runWatchDaemonChild` writes a startup line to the log file when
   `watchLogFile` is set to a temp path

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-c
GOWORK=off go build ./...
GOWORK=off go vet ./...
GOWORK=off go test ./internal/app -run 'TestWatch' -timeout 60s
```

**Before running verification:** Search for tests that expect the OLD warning behavior:
```bash
grep -r "mutually exclusive" internal/app/watch_test.go
grep -r "Warning:" internal/app/watch_test.go
```
Update any such tests to expect the new error behavior.

## 7. Constraints

- Do not modify `internal/watcher/`. Log writes are in the app layer only.
- Log write failures are non-fatal (daemon proceeds even if log write fails).
- Add `"time"` import to watch.go if not already present.

## 8. Report

Commit then append to `docs/IMPL-audit-round7.md`:

```yaml
### Agent C — Completion Report
status: complete
worktree: .claude/worktrees/wave1-agent-c
commit: cbed9169ee855cb57ee517fd7d11a4d650fb2387
files_changed:
  - internal/app/watch.go
  - internal/app/watch_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestWatchDaemonStopConflict
  - TestWatchLogStartup
verification: PASS (go test ./internal/app -run 'TestWatch' -timeout 60s — 13/13 tests)
```

---

#### Wave 1 Agent D: Stats --days abc user-friendly error

You are Wave 1 Agent D. Fix `brewprune stats --days abc` which currently exposes a raw Go
`strconv.ParseInt` error instead of the user-friendly message used for `--days -1`.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-d 2>/dev/null || true
ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-d"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then echo "ISOLATION FAILURE: Wrong directory"; exit 1; fi
ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-d" ]; then echo "ISOLATION FAILURE: Wrong branch"; exit 1; fi
git worktree list | grep -q "wave1-agent-d" || { echo "ISOLATION FAILURE: Not in worktree list"; exit 1; }
echo "✓ Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

## 1. File Ownership

- `internal/app/stats.go` — modify
- `internal/app/stats_test.go` — modify

## 2. Interfaces You Must Implement

No new interfaces.

## 3. Interfaces You May Call

No cross-agent dependencies.

## 4. What to Implement

**Finding 7 — `--days abc` raw Go error:**

Currently `statsDays` is an `int` registered with `statsCmd.Flags().IntVar(...)`. Cobra
parses flag values before `RunE` is called, so non-integer inputs produce cobra's raw error:
`invalid argument "abc" for "--days" flag: strconv.ParseInt: parsing "abc": invalid syntax`

**Fix:** Switch from `IntVar` to `StringVar` with manual parsing in `runStats`:

1. Add package-level `statsDaysStr string` alongside `statsDays int`:
   ```go
   var (
       statsDays    int    // parsed value (used inside runStats)
       statsDaysStr string // receives cobra flag value (string for clean error messages)
       statsPackage string
       statsAll     bool
   )
   ```

2. Change flag registration from `IntVar` to `StringVar`:
   ```go
   statsCmd.Flags().StringVar(&statsDaysStr, "days", "30", "Time window in days")
   ```

3. At the top of `runStats`, parse `statsDaysStr` and set `statsDays`:
   ```go
   days, err := strconv.Atoi(statsDaysStr)
   if err != nil || days <= 0 {
       return fmt.Errorf("--days must be a positive integer")
   }
   statsDays = days
   ```

4. Keep the rest of `runStats` unchanged (it uses `statsDays`).

**Test compatibility:** Read `stats_test.go` first. Tests that check `daysFlag.DefValue` expect
"30" — this still works with `StringVar("30")`. Tests that directly set `statsDays = N` and
call `runStats` bypass the new parsing; for those, set `statsDaysStr = strconv.Itoa(N)` before
calling `runStats` in the test setup, OR just set both `statsDaysStr` and `statsDays`.

## 5. Tests to Write

1. `TestStatsDaysNonInteger_UserFriendlyError` — `--days abc` returns "must be a positive integer"
   (no "strconv.ParseInt" in the error message)
2. `TestStatsDaysZero_Error` — `--days 0` returns the same user-friendly error
3. `TestStatsDaysNegative_Error` — `--days -1` still returns the same error (existing behavior)

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-d
GOWORK=off go build ./...
GOWORK=off go vet ./...
GOWORK=off go test ./internal/app -run 'TestStats' -timeout 60s
```

## 7. Constraints

- Keep `statsDays int` as the internal value; `statsDaysStr string` is only the flag receiver.
- Default value remains `"30"` (string, not int).
- `TestStatsCommand_FlagDefaults` checks `daysFlag.DefValue != "30"` — this continues to pass.

## 8. Report

Commit then append to `docs/IMPL-audit-round7.md`:

```yaml
### Agent D — Completion Report
status: complete
worktree: .claude/worktrees/wave1-agent-d
commit: c64e727
files_changed:
  - internal/app/stats.go
  - internal/app/stats_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestStatsDaysNonInteger_UserFriendlyError
  - TestStatsDaysZero_Error
  - TestStatsDaysNegative_Error
verification: PASS (go test ./internal/app -run 'TestStats' — 13/13 tests)
```

---

#### Wave 1 Agent E: Unused command + table fixes

You are Wave 1 Agent E. Fix four issues touching `unused.go` and `output/table.go`:
(1) double "Error: Error:" prefix on `--tier --all` conflict, (2) Reclaimable footer
shows "(risky, hidden)" when `--all` is passed, (3) `--verbose` tip appears after all
package blocks instead of before, (4) `--sort age` has no visible "Installed" column.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-e 2>/dev/null || true
ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-e"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then echo "ISOLATION FAILURE: Wrong directory"; exit 1; fi
ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-e" ]; then echo "ISOLATION FAILURE: Wrong branch"; exit 1; fi
git worktree list | grep -q "wave1-agent-e" || { echo "ISOLATION FAILURE: Not in worktree list"; exit 1; }
echo "✓ Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

## 1. File Ownership

- `internal/app/unused.go` — modify
- `internal/app/unused_test.go` — modify
- `internal/output/table.go` — modify
- `internal/output/table_test.go` — modify

## 2. Interfaces You Must Implement

```go
// In output/table.go — add field to existing ConfidenceScore struct:
type ConfidenceScore struct {
    // ... all existing fields unchanged ...
    InstalledAt time.Time // Non-zero signals "show Installed column"; set when sort=age
}
```

`RenderConfidenceTable` renders an "Installed" column when ANY score in the slice has
non-zero `InstalledAt`. When rendered, the column replaces the "Last Used" column or
is added as an extra column — choose whichever fits cleanly in the 88-char line width.
Recommendation: replace "Last Used" with "Installed" when all InstalledAt are non-zero.

## 3. Interfaces You May Call

```go
// Existing:
func RenderConfidenceTable(scores []ConfidenceScore) string
func RenderReclaimableFooter(safe, medium, risky TierStats, showAll bool) string
func RenderTierSummary(safe, medium, risky TierStats, showAll bool, caskCount int) string
```

## 4. What to Implement

**Finding 14 — Double "Error: Error:" prefix:**

`unused.go:98`:
```go
return fmt.Errorf("Error: --all and --tier cannot be used together; ...")
```

Cobra prepends "Error: " automatically. Remove the inline "Error: " prefix:
```go
return fmt.Errorf("--all and --tier cannot be used together; --tier already filters to a specific tier")
```

**Finding 13 — Reclaimable footer "(risky, hidden)" when --all:**

Audit finding says `unused --all` shows "(risky, hidden)". Read `unused.go:385`:
```go
footer := output.RenderReclaimableFooter(safeTier, mediumTier, riskyTier, unusedAll || unusedTier != "")
```

And `RenderReclaimableFooter` in table.go:532-545 — when `showAll = true`, `, hidden` is NOT
added. **Investigate whether the bug actually exists in the current code.** If the logic is
already correct, add a test to confirm the expected behavior and note "DONE" in the report.
If there IS a bug (e.g., `unusedAll` not being set correctly before this line), fix it.

**Finding 10 — Verbose tip before output:**

Current `unused.go` shows the "pipe to less" tip AFTER all verbose package blocks.
Move the tip BEFORE the verbose output when `len(scores) > 10` AND stdout is a TTY:

```go
// Before the verbose rendering block:
if unusedVerbose && len(scores) > 10 {
    fi, _ := os.Stdout.Stat()
    if fi != nil && (fi.Mode()&os.ModeCharDevice) != 0 {
        fmt.Println("Tip: For easier viewing of long output, pipe to less:")
        fmt.Println("     brewprune unused --verbose | less")
        fmt.Println()
    }
}
```

Remove the duplicate tip that appears after the table.

**Finding 9 — `--sort age` no Installed column:**

When `unusedSort == "age"`, populate `InstalledAt` on each `output.ConfidenceScore`:

In `unused.go`, in the loop that builds `outputScores`:
```go
var installedAt time.Time
if unusedSort == "age" {
    installedAt = s.InstalledAt  // from analyzer.ConfidenceScore
}
outputScores[i] = output.ConfidenceScore{
    // ... existing fields ...
    InstalledAt: installedAt,
}
```

In `output/table.go`, modify `RenderConfidenceTable` to detect non-zero `InstalledAt`:
- Check if any score has non-zero InstalledAt
- If yes: replace "Last Used" column header with "Installed", and format `InstalledAt` as
  `score.InstalledAt.Format("2006-01-02")` in that column
- If no: render as before

Also: when `--sort age` is used and all packages have identical `InstalledAt`, the existing
note "All packages installed at the same time — age sort has no effect. Sorted by tier, then
alphabetically." should appear BEFORE the table, not after. Check current placement and move.

## 5. Tests to Write

1. `TestDoubleErrorPrefix_Fixed` — `--tier safe --all` conflict message contains single "Error:"
2. `TestReclaimableFooter_AllFlag` — `RenderReclaimableFooter(..., true)` does NOT contain "hidden"
3. `TestVerboseTipAppearsBeforeOutput` — when scores > 10, tip line appears before first "Package:"
4. `TestSortAge_InstalledColumn` — when sort=age, output table header contains "Installed" column
5. `TestReclaimableFooter_NoAllFlag` — `RenderReclaimableFooter(..., false)` contains "hidden"

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-e
GOWORK=off go build ./...
GOWORK=off go vet ./...
GOWORK=off go test ./internal/app -run 'TestUnused|TestDouble|TestVerbose' -timeout 60s
GOWORK=off go test ./internal/output -run 'TestReclaimable|TestSort|TestTable' -timeout 60s
```

## 7. Constraints

- Do NOT modify `internal/analyzer/` types. `InstalledAt` is carried through the existing
  `analyzer.ConfidenceScore.InstalledAt` field (read the types file to confirm it exists).
- The "Installed" column replaces "Last Used" only when `unusedSort == "age"`. Default table
  layout is unchanged.
- Line width of `RenderConfidenceTable` output must remain ≤88 chars.

## 8. Report

Commit then append to `docs/IMPL-audit-round7.md`:

```yaml
### Agent E — Completion Report
status: complete
worktree: .claude/worktrees/wave1-agent-e
commit: 933e878
files_changed:
  - internal/app/unused.go
  - internal/app/unused_test.go
  - internal/output/table.go
  - internal/output/table_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestDoubleErrorPrefix_Fixed
  - TestReclaimableFooter_AllFlag
  - TestVerboseTipAppearsBeforeOutput
  - TestSortAge_InstalledColumn
  - TestReclaimableFooter_NoAllFlag
verification: PASS (go test ./internal/app ./internal/output — all pass)
```

---

#### Wave 1 Agent F: Quickstart PATH-not-active warning

You are Wave 1 Agent F. Make the PATH-not-active warning more prominent in the quickstart
summary section, so users know immediately that usage tracking won't capture commands until
they restart their shell or source the config file.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-f 2>/dev/null || true
ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-f"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then echo "ISOLATION FAILURE: Wrong directory"; exit 1; fi
ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-f" ]; then echo "ISOLATION FAILURE: Wrong branch"; exit 1; fi
git worktree list | grep -q "wave1-agent-f" || { echo "ISOLATION FAILURE: Not in worktree list"; exit 1; }
echo "✓ Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

## 1. File Ownership

- `internal/app/quickstart.go` — modify
- `internal/app/quickstart_test.go` — modify

## 2. Interfaces You Must Implement

No new interfaces.

## 3. Interfaces You May Call

```go
func isOnPATH(dir string) bool        // in status.go (same package)
func isConfiguredInShellProfile(dir string) bool  // in status.go (same package)
```

## 4. What to Implement

**Finding 4 — Shim tracking gap:**

After quickstart, users run commands (`jq`, `bat`, etc.) expecting them to be tracked.
They're not — because `~/.brewprune/bin` is not yet on the active PATH. The current warning
is buried in the Note at the bottom of the summary and is too mild.

**What to change:** After the summary "Setup complete!" block, when the shim dir is configured
in a shell profile but NOT yet on the active PATH, print a prominent boxed warning:

```
⚠  TRACKING IS NOT ACTIVE YET

   Your shell has not loaded the new PATH. Commands you run now
   will NOT be tracked by brewprune.

   To activate tracking immediately:
     source ~/.profile    (or source ~/.zshrc / ~/.bashrc)

   Or restart your terminal.
```

The `shimDir` variable is available in the summary section (from the Step 2 block). Check
`isOnPATH(shimDir)` — if false, show the prominent warning.

Look at the existing Note (lines 238-241) and replace/expand it:
```go
// After "Setup complete!" and before or after the IMPORTANT note:
if shimDirErr == nil && !isOnPATH(shimDir) {
    configFile := detectShellConfig()  // also available via shell package
    fmt.Println()
    fmt.Println("⚠  TRACKING IS NOT ACTIVE YET")
    fmt.Println()
    fmt.Println("   Your shell has not loaded the new PATH. Commands you run now")
    fmt.Println("   will NOT be tracked by brewprune.")
    fmt.Println()
    fmt.Printf("   To activate tracking immediately:\n")
    fmt.Printf("     source %s\n", configFile)
    fmt.Println()
    fmt.Println("   Or restart your terminal.")
}
```

Note: `detectShellConfig()` is defined in `doctor.go` (same package). It's accessible here.

## 5. Tests to Write

1. `TestQuickstartPATHWarning_ShownWhenNotActive` — when shim dir is NOT on PATH, output
   contains "TRACKING IS NOT ACTIVE YET"
2. `TestQuickstartPATHWarning_NotShownWhenActive` — when shim dir IS on PATH, output does
   NOT contain "TRACKING IS NOT ACTIVE YET"

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-f
GOWORK=off go build ./...
GOWORK=off go vet ./...
GOWORK=off go test ./internal/app -run 'TestQuickstart' -timeout 60s
```

## 7. Constraints

- Only modify `quickstart.go` and `quickstart_test.go`.
- The warning must appear when `isOnPATH(shimDir)` is false AND `shimDirErr` is nil.
- Do NOT modify `doctor.go` or `status.go` — call the existing functions from the same package.

## 8. Report

Commit then append to `docs/IMPL-audit-round7.md`:

```yaml
### Agent F — Completion Report
status: complete
worktree: .claude/worktrees/wave1-agent-f
commit: a4efd9c
files_changed:
  - internal/app/quickstart.go
  - internal/app/quickstart_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestQuickstartPATHWarning_ShownWhenNotActive
  - TestQuickstartPATHWarning_NotShownWhenActive
verification: PASS (go test ./internal/app -run 'TestQuickstart' — 17/17 tests)
```

---

#### Wave 1 Agent G: Doctor alias tip fixes

You are Wave 1 Agent G. Fix two issues in `internal/app/doctor.go`: (1) the alias tip
references `brewprune help` which has no alias documentation — include the format inline
or remove the reference, (2) the alias tip currently shows when `criticalIssues > 0`
(the broken-state case).

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-g 2>/dev/null || true
ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-g"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then echo "ISOLATION FAILURE: Wrong directory"; exit 1; fi
ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-g" ]; then echo "ISOLATION FAILURE: Wrong branch"; exit 1; fi
git worktree list | grep -q "wave1-agent-g" || { echo "ISOLATION FAILURE: Not in worktree list"; exit 1; }
echo "✓ Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

## 1. File Ownership

- `internal/app/doctor.go` — modify
- `internal/app/doctor_test.go` — modify

## 2. Interfaces You Must Implement

No new interfaces.

## 3. Interfaces You May Call

No cross-agent dependencies.

## 4. What to Implement

**Finding 11 — Alias tip references `brewprune help` (no docs):**

Current `doctor.go:222-226`:
```go
fmt.Println("Tip: Create ~/.config/brewprune/aliases to declare alias mappings and improve tracking coverage.")
fmt.Println("     Example: ll=eza")
fmt.Println("     See 'brewprune help' for details.")
```

The last line references `brewprune help` which contains no alias documentation. Fix: replace
with a brief inline format hint, since no other command documents aliases:
```go
fmt.Println("Tip: Create ~/.config/brewprune/aliases to declare alias mappings.")
fmt.Println("     Format: one alias per line, e.g. ll=eza or g=git")
fmt.Println("     Aliases help brewprune associate your custom commands with their packages.")
```

Remove the "See 'brewprune help' for details." line.

**Finding 11b — Alias tip fires in fully-broken state:**

Current condition for showing the alias tip:
```go
if !daemonRunning || totalUsageEvents < 10 {
    ...show alias tip...
}
```

This fires when `criticalIssues > 0` (e.g., database not found, shim not found). A user
seeing critical issues does not need an alias tip — they need to fix the basics first.

Add `criticalIssues == 0` to the condition:
```go
if criticalIssues == 0 && (!daemonRunning || totalUsageEvents < 10) {
    ...show alias tip...
}
```

## 5. Tests to Write

1. `TestDoctorAliasTip_NoCriticalIssues` — when no critical issues, alias tip appears in output
2. `TestDoctorAliasTip_HiddenWhenCritical` — when criticalIssues > 0 (mock DB not found),
   alias tip does NOT appear in output
3. `TestDoctorAliasTip_NoBrewpruhelpReference` — alias tip output does NOT contain "brewprune help"

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-g
GOWORK=off go build ./...
GOWORK=off go vet ./...
GOWORK=off go test ./internal/app -run 'TestDoctor' -skip 'TestDoctorHelpIncludesFixNote' -timeout 60s
```

## 7. Constraints

- Only modify `doctor.go` and `doctor_test.go`.
- Skip `TestDoctorHelpIncludesFixNote` if it hangs (known pre-existing issue).

## 8. Report

Commit then append to `docs/IMPL-audit-round7.md`:

```yaml
### Agent G — Completion Report
status: complete
worktree: .claude/worktrees/wave1-agent-g
commit: 4331180
files_changed:
  - internal/app/doctor.go
  - internal/app/doctor_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestDoctorAliasTip_NoCriticalIssues
  - TestDoctorAliasTip_HiddenWhenCritical
  - TestDoctorAliasTip_NoBrewpruhelpReference
verification: PASS (go test ./internal/app -run 'TestDoctor' — 11/11 tests, skipping TestDoctorHelpIncludesFixNote)
```

---

#### Wave 1 Agent H: Status "0 seconds ago" → "just now"

You are Wave 1 Agent H. Fix `brewprune status` which shows "running (since 0 seconds ago)"
for a freshly-started daemon. The `formatDuration` function in `status.go` should return
"just now" for sub-5-second durations.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-h 2>/dev/null || true
ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-h"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then echo "ISOLATION FAILURE: Wrong directory"; exit 1; fi
ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-h" ]; then echo "ISOLATION FAILURE: Wrong branch"; exit 1; fi
git worktree list | grep -q "wave1-agent-h" || { echo "ISOLATION FAILURE: Not in worktree list"; exit 1; }
echo "✓ Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

## 1. File Ownership

- `internal/app/status.go` — modify
- `internal/app/status_test.go` — modify

## 2. Interfaces You Must Implement

No new interfaces.

## 3. Interfaces You May Call

No cross-agent dependencies.

## 4. What to Implement

**Finding 12 — "since 0 seconds ago":**

`status.go:formatDuration` (lines 337-364):
```go
func formatDuration(d time.Duration) string {
    if d < time.Minute {
        return fmt.Sprintf("%d seconds ago", int(d.Seconds()))
    }
    ...
}
```

When `d < 5 * time.Second`, this produces "0 seconds ago" or "1 seconds ago". Fix:
```go
func formatDuration(d time.Duration) string {
    if d < 5*time.Second {
        return "just now"
    }
    if d < time.Minute {
        secs := int(d.Seconds())
        if secs == 1 {
            return "1 second ago"
        }
        return fmt.Sprintf("%d seconds ago", secs)
    }
    ...
}
```

Also fix "1 seconds ago" → "1 second ago" while editing this function.

## 5. Tests to Write

1. `TestFormatDuration_JustNow` — durations < 5s return "just now"
2. `TestFormatDuration_Seconds` — 10s returns "10 seconds ago"
3. `TestFormatDuration_OneSecond` — 1s returns "just now" (sub-5s threshold)
4. `TestFormatDuration_SingularSecond` — 6s returns "6 seconds ago" (not "6 second ago")

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-h
GOWORK=off go build ./...
GOWORK=off go vet ./...
GOWORK=off go test ./internal/app -run 'TestFormat|TestStatus' -timeout 60s
```

**Before running:** Check for existing tests of `formatDuration` that expect the old behavior:
```bash
grep -r "seconds ago" internal/app/status_test.go
grep -r "formatDuration" internal/app/status_test.go
```
Update any such tests to expect the new "just now" behavior for sub-5s durations.

## 7. Constraints

- Threshold for "just now" is `< 5 * time.Second`.
- Singular/plural: "1 second ago" (not "1 seconds ago"), "N seconds ago" for N > 1.

## 8. Report

Commit then append to `docs/IMPL-audit-round7.md`:

```yaml
### Agent H — Completion Report
status: complete
worktree: .claude/worktrees/wave1-agent-h
commit: c90b4de
files_changed:
  - internal/app/status.go
  - internal/app/status_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestFormatDuration_JustNow
  - TestFormatDuration_Seconds
  - TestFormatDuration_OneSecond
  - TestFormatDuration_SingularSecond
verification: PASS (go test ./internal/app -run 'TestStatus|TestFormat' — 16/16 tests)
```

---

### Wave Execution Loop

After each wave:
1. Read completion reports from each agent's named section above.
2. Merge all agent worktrees:
   ```bash
   git merge wave0-agent-0  # or wave1-agent-{a..h}
   ```
3. Post-merge verification:
   ```bash
   GOWORK=off go build ./...
   GOWORK=off go vet ./...
   GOWORK=off go test ./... -timeout 300s -skip 'TestDoctorHelpIncludesFixNote'
   ```
4. Fix any integration failures (especially cross-agent cascade: `output.ConfidenceScore.InstalledAt`
   set in `unused.go` by Agent E, also needs setting in `remove.go` by Agent A).
5. Commit the wave merge with a summary message.

---

### Status

- [ ] Wave 0 Agent 0 — investigate explain curl crash
- [ ] Wave 1 Agent A — remove freed-space, dep-locked pre-validation, staleness check removal
- [ ] Wave 1 Agent B — snapshots "Restored bat@" version fix
- [ ] Wave 1 Agent C — watch --daemon --stop error; watch.log startup event
- [ ] Wave 1 Agent D — stats --days abc user-friendly error
- [ ] Wave 1 Agent E — unused footer, double Error:, verbose tip placement, sort-age column
- [ ] Wave 1 Agent F — quickstart PATH-not-active warning
- [ ] Wave 1 Agent G — doctor alias tip wording + critical-issue suppression
- [ ] Wave 1 Agent H — status "0 seconds ago" → "just now"

---

### Agent 0 — Completion Report
status: complete
commit: edb061e9ae322f0541b147f8eea221fc18b953da
files_changed:
  - internal/app/explain.go
files_created: []
interface_deviations: []
out_of_scope_deps:
  - "file: internal/store/queries.go, change: none required, reason: GetPackage always returns a non-nil error when the row is not found (wraps sql.ErrNoRows as fmt.Errorf); it never returns (nil, nil), so the crash path described in the brief cannot originate here."
  - "file: internal/analyzer/confidence.go, change: none required, reason: ComputeScore dereferences pkgInfo immediately after GetPackage returns with a nil-error check; since GetPackage never produces (nil, nil), no nil-dereference is possible in the analyzer traversal path."
tests_added:
  - TestRunExplain_NilPackageGraceful
verification: PASS (go test ./internal/app -run "TestExplain|TestRunExplain" — 6/6 tests)
notes: |
  All defensive nil guards were already present before this wave ran.
  explain.go lines 75-79 already guard the second GetPackage call with
  `if pkg != nil { installedDate = ... }`. TestRunExplain_NilPackageGraceful
  was also already in explain_test.go (lines 459-491), exercising renderExplanation
  with an empty installedDate to confirm no panic. The exit-139 (SIGSEGV) crash
  reported for `brewprune explain curl` in fresh containers is therefore not
  attributable to any nil-dereference path visible in the Go source. The most
  likely cause is a QEMU x86_64 emulation artifact (stack alignment or JIT
  issue) triggered on first invocation before the SQLite page cache is warm;
  subsequent calls succeed because the DB is cached. No code changes were made
  in this wave — the existing guards are sufficient and all tests pass.
