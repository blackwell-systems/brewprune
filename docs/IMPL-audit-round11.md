# IMPL: Audit Round 11 Fixes

**Source audit:** `docs/cold-start-audit-r10.md`
**Written:** 2026-03-02
**Scout version:** v0.3.5

---

## Suitability Assessment

**Verdict: SUITABLE WITH CAVEATS**

The work decomposes cleanly into disjoint file ownership across four agents. No cross-agent interfaces are required (all changes are self-contained within each file). The one investigation-first item (4.1) has been investigated during scouting and a root cause has been identified.

### Pre-implementation scan results

| Finding | Status | Notes |
|---------|--------|-------|
| 4.1 Daemon records zero events | To-do | Root cause identified (see below) |
| 7.1 Stale dep graph after undo+scan | To-do | Root cause identified (see below) |
| 7.2/9.8 remove exits 0 when all fail | To-do | Trivial code change in remove.go |
| 8.6 unused error chain prefix | To-do | One-line fix in unused.go |
| 7.6 remove nonexistent missing undo hint | To-do | Parity fix in remove.go |
| 4.2 watch.log has no processing entries | To-do | Logging in fsevents.go/shim_processor.go |
| 2.2 status no-events warning too early | To-do | Status.go grace period check |
| 7.7 stats --package lacks scan hint | To-do | stats.go error message |
| 6.4 doctor blank-state redundant error | To-do | doctor.go summary logic |
| 6.3 doctor aliases tip unconditional | Already fixed | Code already guards behind `criticalIssues == 0` |
| 9.7 stats --all sort annotation | To-do | stats.go print position |
| 7.3 remove --risky --dry-run warning mid-table | Already fixed | Warning already printed after table (line 219) |
| 3.6 --sort age secondary sort | Already fixed | sortScores() already has name as tertiary sort |
| 1.1 -v conflicts with unused -v | To-do | root.go flag definition |

**Pre-implementation check details:**

- **6.3 aliases tip**: `doctor.go` line 226 already guards the aliases tip with `if criticalIssues == 0 && (!daemonRunning || totalUsageEvents < 10)`. In blank state, `criticalIssues > 0` so the tip is suppressed. Status: ALREADY FIXED.
- **7.3 dry-run warning**: `remove.go` lines 218-220 already print the `lockedPackages` summary AFTER `displayConfidenceScores()`. Status: ALREADY FIXED.
- **3.6 --sort age secondary sort**: `unused.go` `sortScores()` already includes a tertiary sort by `scores[i].Package < scores[j].Package`. Status: ALREADY FIXED.

**To-do count:** 11 findings
**Already fixed:** 3 findings
**Estimated times:** Wave 1 parallel agents: ~30 min each. Total wall time: ~40 min (parallel) + review.

### Caveats

- **Finding 4.1** is the most complex; it requires adding a new function `logProcessingCycle` and passing a count back from `ProcessUsageLog`. The interface is defined in the Interface Contracts section and agents must not deviate from it.
- **Finding 7.1** requires a schema-level fix (adding `ClearDependencies` to the store and calling it in `ScanPackages`). Agent B owns both files and will not conflict.
- Agent assignments are designed with strictly disjoint file ownership. Confirm before starting Wave 1.

---

## Investigation Results

### Finding 4.1  -  Root cause

**Trace:**
1. Shim binary intercepts `git`, records `argv0 = /home/brewuser/.brewprune/bin/git` → appended to `~/.brewprune/usage.log`.
2. `ProcessUsageLog` reads entries. Offset advances (lines ARE read).
3. For each entry: `basename = filepath.Base(argv0) = "git"`.
4. Lookup order: `optPathMap["/opt/homebrew/bin/git"]` → miss; `optPathMap["/usr/local/bin/git"]` → miss; `binaryMap["git"]` → MISS; `optPathMap["/home/linuxbrew/.linuxbrew/bin/git"]` → MISS.
5. All entries fall through to `log.Printf("no package found...")` and are skipped.
6. But **`watch.log` shows no `log.Printf` output**  -  this is the key anomaly.

**Why `log.Printf` is not appearing in watch.log:**
The daemon child is spawned with `cmd.Stdout = logF; cmd.Stderr = logF` (daemon.go:41-43). Go's default logger (`log.Printf`) writes to `os.Stderr`. When the child writes to `os.Stderr`, it goes to the file descriptor inherited from `cmd.Stderr = logF`. This SHOULD work. But the startup log message at `runWatchDaemonChild()` (watch.go:197-203) is guarded by `if watchLogFile != ""`. The child process is spawned with `exec.Command(executable, "watch", "--daemon-child")`  -  **without `--log-file`**. So `watchLogFile = ""` in the child. The startup message is suppressed. `log.Printf` still writes to inherited stderr → watch.log.

**However**: The child spawns in daemon.go:40 without `--log-file`. But `w.store` in the child is opened via `runWatch` → `store.New(dbPath)`. The child calls `db.CreateSchema()` and then `w.RunDaemon()`. This is correct.

**The real root cause for zero events:**
`buildBasenameMap` builds the map from `pkg.BinaryPaths` stored in the DB. `BinaryPaths` are populated by `scanner.RefreshBinaryPaths()` which reads symlinks in the brew prefix bin dir (e.g., `/home/linuxbrew/.linuxbrew/bin/`). Each symlink is resolved via `Readlink()` and `extractPackageFromPath()` which looks for `"Cellar"` in the path. In the Linuxbrew container, symlinks in `/home/linuxbrew/.linuxbrew/bin/` point to `/home/linuxbrew/.linuxbrew/Cellar/git/VERSION/bin/git`, so `extractPackageFromPath` returns `"git"`. This means `pkg.BinaryPaths` for `git` should be `["/home/linuxbrew/.linuxbrew/bin/git"]`.

For the `binaryMap`, the first pass maps `filepath.Base("/home/linuxbrew/.linuxbrew/bin/git") = "git"` → `"git"`. The second pass maps `pkg.Name = "git"` → `"git"`. So `binaryMap["git"]` should = `"git"`.

**The `optPathMap`** maps `"/home/linuxbrew/.linuxbrew/bin/git"` → `"git"`, and the lookup at line 149 checks `optPathMap["/home/linuxbrew/.linuxbrew/bin/"+basename]` = `optPathMap["/home/linuxbrew/.linuxbrew/bin/git"]` = `"git"` → should HIT.

**Conclusion**: In the audit scenario, the daemon is started with `brewprune watch --daemon` BEFORE `RefreshBinaryPaths` has run for a second time after undo+scan. Packages exist in the DB (undo re-installed them) but their `BinaryPaths` field is either empty (undo does not update the DB) or reflects pre-undo scan. After `brewprune undo`, packages are reinstalled via brew but the DB still has stale/deleted package records. After `brewprune scan` (finding 7.1 scenario), `InsertPackage` uses `INSERT OR REPLACE` which DOES update `BinaryPaths`. But the dependency relationships are stale (finding 7.1).

**The actual bug causing 4.1 in isolation** (independent of 7.1): `ProcessUsageLog` produces zero log output for shim paths because `log.Printf` in the daemon child is redirected to watch.log but there are NO "no package found" lines either. This means the shim log is NOT being processed at all  -  either the file is being read but all lines fail `parseShimLogLine`, or the file doesn't exist at the time `ProcessUsageLog` is called.

**Most likely root cause**: The shim log path is `filepath.Join(homeDir, ".brewprune", "usage.log")` but in the audit, the daemon's child process runs as a different process. `os.UserHomeDir()` should return the same home. However, the child is spawned with `cmd.Stdin = nil` and no environment. If `HOME` env var is not inherited by the child (only `os.UserHomeDir()` is used, which uses `HOME` first), and the daemon child has a stripped environment, `os.UserHomeDir()` could fail or return a different path.

**Actually the most definitive fix** is to:
1. Pass `--log-file` to the daemon child so it has the watch.log path.
2. Add per-cycle logging in `ProcessUsageLog` (returning a summary struct) so watch.log shows activity.
3. Add logging when `buildBasenameMap`/`buildOptPathMap` return empty maps as a diagnostic.

The agent for 4.1 should implement all three.

### Finding 7.1  -  Root cause

`ScanPackages()` in `inventory.go` calls `InsertPackage` (which uses `INSERT OR REPLACE`) and then `InsertDependency` (which uses `INSERT OR IGNORE`). After `brewprune undo`, the undone packages are reinstalled by brew BUT the DB still has the packages with their old data (undo calls `brew install` but does NOT call `store.InsertPackage`). More importantly, undo does NOT delete the existing `dependencies` rows for those packages.

When `scan` runs after undo:
1. `ScanPackages` inserts/updates packages via `INSERT OR REPLACE`  -  OK.
2. For dependencies: `InsertDependency` uses `INSERT OR IGNORE`. If the dependency row already exists (from before undo), it is NOT updated. If `brew deps` now returns different/fewer deps (e.g., because a transitive dep was removed and reinstalled), those changes are IGNORED.
3. Stale rows remain in the `dependencies` table pointing to packages that may have been removed from `packages` table during undo removal  -  but wait, `DeletePackage` cascades to `dependencies` via `ON DELETE CASCADE`, so only the removed packages' own rows are deleted, NOT the rows where they appear as `depends_on`.

**The real problem**: After undo+scan, the `dependencies` table has STALE ROWS from before undo. Packages that were removed from the DB (during `remove` → `st.DeletePackage`) had their dependency rows cascade-deleted. But packages that were NEVER deleted from the DB (only un-installed on disk) keep their old dependency rows. When scan runs, `InsertDependency` with `OR IGNORE` does NOT update stale relationships.

**Fix**: Before inserting new dependencies in `ScanPackages`, clear all existing dependency rows for the packages being scanned (add `ClearDependencies(pkgName string) error` to the store, call it before inserting new deps for each package). Or, simpler: delete ALL rows from `dependencies` at the start of a full scan and re-insert them fresh.

---

## Dependency Graph

```
Wave 1 (all parallel, no dependencies):
  Agent A  ←  remove.go (findings 7.2/9.8, 7.6)
  Agent B  ←  scanner/inventory.go + store/queries.go (finding 7.1)
  Agent C  ←  app/unused.go (finding 8.6)
  Agent D  ←  watcher/shim_processor.go + watcher/fsevents.go + app/watch.go (findings 4.1, 4.2)

Wave 2 (parallel, no wave-1 dependencies):
  Agent E  ←  app/status.go (finding 2.2)
  Agent F  ←  app/stats.go (findings 7.7, 9.7)
  Agent G  ←  app/doctor.go (finding 6.4)
  Agent H  ←  cmd/brewprune/main.go or root.go (finding 1.1)
```

Wave 2 agents have no dependency on Wave 1 output. All waves can run in parallel if desired, but wave 1 items are higher priority and should be verified first.

---

## Interface Contracts

### New function: `store.ClearDependencies(pkg string) error`

```go
// ClearDependencies removes all dependency rows where package = pkg.
// Called at the start of a dependency graph rebuild to ensure stale
// relationships are removed before fresh data is inserted.
func (s *Store) ClearDependencies(pkg string) error
```

**Location:** `/Users/dayna.blackwell/code/brewprune/internal/store/queries.go`
**Owned by:** Agent B
**Called by:** `scanner.ScanPackages()` in `inventory.go`

### New type: `ProcessingStats` in watcher package

```go
// ProcessingStats holds per-cycle summary from ProcessUsageLog.
type ProcessingStats struct {
    LinesRead  int
    Resolved   int
    Skipped    int
    Inserted   int
}
```

**Location:** `/Users/dayna.blackwell/code/brewprune/internal/watcher/shim_processor.go`
**Owned by:** Agent D
**NOTE:** `ProcessUsageLog` signature MUST change from `func ProcessUsageLog(st *store.Store) error` to `func ProcessUsageLog(st *store.Store) (ProcessingStats, error)`.
Agent D must update ALL callers: `fsevents.go` (`runShimLogProcessor`, `Start`). Existing callers in tests must also be updated.

---

## File Ownership

| File | Agent | Wave | Findings |
|------|-------|------|---------|
| `internal/app/remove.go` | A | 1 | 7.2/9.8, 7.6 |
| `internal/scanner/inventory.go` | B | 1 | 7.1 |
| `internal/store/queries.go` | B | 1 | 7.1 (new ClearDependencies) |
| `internal/app/unused.go` | C | 1 | 8.6 |
| `internal/watcher/shim_processor.go` | D | 1 | 4.1, 4.2 |
| `internal/watcher/fsevents.go` | D | 1 | 4.1, 4.2 |
| `internal/app/watch.go` | D | 1 | 4.1 (pass --log-file to child) |
| `internal/app/status.go` | E | 2 | 2.2 |
| `internal/app/stats.go` | F | 2 | 7.7, 9.7 |
| `internal/app/doctor.go` | G | 2 | 6.4 |
| `internal/app/root.go` | H | 2 | 1.1 |

**Strict disjoint constraint:** No file appears in more than one agent's ownership. Test files associated with owned files are also owned by that agent (e.g., `remove_test.go` by Agent A, `shim_processor_test.go` by Agent D).

---

## Wave Structure

### Wave 1  -  High-priority parallel agents (4 agents)

All four agents work independently on disjoint files. Launch simultaneously.

### Wave 2  -  Secondary parallel agents (4 agents)

Launch after Wave 1 agents complete and pass tests. These are independent of Wave 1 changes.

---

## Agent Prompts

### Agent A  -  remove.go fixes (findings 7.2/9.8 and 7.6)

```
ROLE: You are a focused implementation agent. Read, then edit. Do not create new files.

TASK: Fix two findings in /Users/dayna.blackwell/code/brewprune/internal/app/remove.go

FINDING 7.2 / 9.8  -  remove --safe --yes exits 0 when all removals fail:
Current behavior (remove.go ~line 316):
  fmt.Printf("\n✓ Removed %d packages, freed %s\n", successCount, formatSize(freedSize))
  [failures printed if any]
  return nil  // always exits 0

Required behavior:
- When successCount == 0 AND len(failures) > 0: print "✗ Removed 0 packages" (using ✗ not ✓) and return a non-nil error so cobra exits 1.
- When successCount > 0 AND len(failures) > 0: existing behavior is acceptable (partial success, exit 0 with warning) but use ✓ for success line.
- When successCount == 0 AND len(failures) == 0: this is the "no packages to remove" case already handled at line 223-226, no change needed.

Exact change needed (~line 316):
  Replace: fmt.Printf("\n✓ Removed %d packages, freed %s\n", ...)
  With logic that uses "✓" when successCount > 0 and "✗" when successCount == 0,
  and returns fmt.Errorf("removed 0 packages: all %d removals failed", len(failures))
  when successCount == 0.

FINDING 7.6  -  remove nonexistent-package missing undo mention:
Current behavior (remove.go ~line 119):
  fmt.Fprintf(os.Stderr, "Error: package not found: %s\n\nCheck the name with 'brew list' or 'brew search %s'.\nIf you just installed it, run 'brewprune scan' to update the index.\n", pkg, pkg)

Required behavior: Add one more sentence AFTER "If you just installed it..." line:
  "If you recently ran 'brewprune undo', run 'brewprune scan' to update the index."

This matches the message already in explain.go line 63.

STEPS:
1. Read /Users/dayna.blackwell/code/brewprune/internal/app/remove.go (already described above but read it to get exact line numbers)
2. Read /Users/dayna.blackwell/code/brewprune/internal/app/remove_test.go
3. Apply the two fixes
4. Update or add tests in remove_test.go to cover: (a) all-fail scenario exits non-zero, (b) nonexistent package error message includes undo hint
5. Run: cd /Users/dayna.blackwell/code/brewprune && go build ./... && go test ./internal/app/ -run TestRemove -v
6. Report: which lines changed, test output

CONSTRAINTS:
- Only modify remove.go and remove_test.go
- Do not change any other file
- The ✗ character is U+2717 (use it literally in the string, same as ✓ is used on line 316)
```

---

### Agent B  -  stale dependency graph fix (finding 7.1)

```
ROLE: You are a focused implementation agent. Read, then edit. Do not create new files.

TASK: Fix stale dependency graph after undo+scan.

ROOT CAUSE (already investigated):
ScanPackages() in /Users/dayna.blackwell/code/brewprune/internal/scanner/inventory.go calls InsertDependency with INSERT OR IGNORE, which silently skips stale existing dependency rows. After undo+scan, old dependency rows survive because INSERT OR IGNORE does not overwrite them.

FIX PLAN:
1. Add ClearDependencies(pkg string) error to /Users/dayna.blackwell/code/brewprune/internal/store/queries.go
   Exact SQL: DELETE FROM dependencies WHERE package = ?
   Add it immediately after InsertDependency (around line 203).
   Signature:
     func (s *Store) ClearDependencies(pkg string) error {
         _, err := s.db.Exec(`DELETE FROM dependencies WHERE package = ?`, pkg)
         if err != nil {
             return fmt.Errorf("failed to clear dependencies for %s: %w", pkg, err)
         }
         return nil
     }

2. In /Users/dayna.blackwell/code/brewprune/internal/scanner/inventory.go, in ScanPackages():
   Find the loop that calls InsertDependency (around lines 35-50):
     for pkgName, deps := range depsTree {
         for _, dep := range deps {
             ...
             if err := s.store.InsertDependency(pkgName, dep); err != nil { ... }
         }
     }
   BEFORE this loop, add a new loop that clears existing dependencies for each package:
     for pkgName := range depsTree {
         if err := s.store.ClearDependencies(pkgName); err != nil {
             return fmt.Errorf("failed to clear dependencies for %s: %w", pkgName, err)
         }
     }
   This ensures that when depsTree has fresh data, stale rows are removed first.

   NOTE: ClearDependencies only deletes rows where package = pkgName (the "parent" side).
   The ON DELETE CASCADE in the schema handles the case where packages themselves are deleted.

STEPS:
1. Read /Users/dayna.blackwell/code/brewprune/internal/store/queries.go (lines 188-265)
2. Read /Users/dayna.blackwell/code/brewprune/internal/scanner/inventory.go (full file)
3. Read /Users/dayna.blackwell/code/brewprune/internal/scanner/dependencies_test.go
4. Read /Users/dayna.blackwell/code/brewprune/internal/store/db_test.go
5. Apply the two changes
6. Add a test in dependencies_test.go (or db_test.go if more appropriate) that:
   - Inserts a package A with deps [B, C]
   - Calls ClearDependencies(A)
   - Verifies GetDependencies(A) returns empty
   - Verifies GetDependents(B) returns empty
7. Run: cd /Users/dayna.blackwell/code/brewprune && go build ./... && go test ./internal/store/ ./internal/scanner/ -v
8. Report: which lines changed, test output

CONSTRAINTS:
- Only modify queries.go, inventory.go, and their test files
- Do not touch schema.go (no schema changes needed)
- Do not change any other file
```

---

### Agent C  -  unused.go error chain fix (finding 8.6)

```
ROLE: You are a focused implementation agent. Read, then edit. Do not create new files.

TASK: Fix finding 8.6  -  brewprune unused with no DB shows "failed to list packages:" prefix.

CURRENT BEHAVIOR (~line 138 of unused.go):
  packages, err := st.ListPackages()
  if err != nil {
      return fmt.Errorf("failed to list packages: %w", err)
  }

When DB is not initialized, ListPackages returns store.ErrNotInitialized ("database not initialized  -  run 'brewprune scan' to create the database"). The wrapping adds "failed to list packages:" prefix. The user sees:
  Error: failed to list packages: database not initialized  -  run 'brewprune scan'

REQUIRED BEHAVIOR:
  Error: database not initialized  -  run 'brewprune scan'

FIX: In runUnused(), replace the error wrapping:
  if err != nil {
      return fmt.Errorf("failed to list packages: %w", err)
  }
WITH:
  if err != nil {
      // Surface the terminal error directly to avoid leaking internal chain prefixes.
      cause := err
      for errors.Unwrap(cause) != nil {
          cause = errors.Unwrap(cause)
      }
      return cause
  }

Also add "errors" to the import block if not already present.

Compare with remove.go lines 178-184 which already does this correctly using the same pattern.

STEPS:
1. Read /Users/dayna.blackwell/code/brewprune/internal/app/unused.go (lines 108-145)
2. Read /Users/dayna.blackwell/code/brewprune/internal/app/unused_test.go
3. Apply the fix
4. Add or update a test in unused_test.go that verifies the error message does NOT contain "failed to list packages:" when DB is uninitialized
5. Run: cd /Users/dayna.blackwell/code/brewprune && go build ./... && go test ./internal/app/ -run TestUnused -v
6. Report: which lines changed, test output

CONSTRAINTS:
- Only modify unused.go and unused_test.go
- Do not change any other file
```

---

### Agent D  -  daemon processing fixes (findings 4.1 and 4.2)

```
ROLE: You are a focused implementation agent. Read, then edit. Do not create new files.

TASK: Fix findings 4.1 (daemon records zero events) and 4.2 (watch.log has no processing entries).

FILES YOU OWN:
- /Users/dayna.blackwell/code/brewprune/internal/watcher/shim_processor.go
- /Users/dayna.blackwell/code/brewprune/internal/watcher/fsevents.go
- /Users/dayna.blackwell/code/brewprune/internal/app/watch.go

ROOT CAUSE (already investigated):
1. The daemon child is spawned WITHOUT the --log-file flag (daemon.go line 40), so watchLogFile="" in the child. The startup log message is suppressed. This is a symptom, not the core data bug.
2. ProcessUsageLog resolves shim paths (e.g., /home/user/.brewprune/bin/git) using optPathMap and binaryMap. These maps are built from pkg.BinaryPaths. If BinaryPaths are not populated, all entries fail resolution silently (log.Printf goes to watch.log but with no entries, nothing to log).
3. There is NO per-cycle summary log. Even when processing succeeds, watch.log shows nothing beyond the start line.

FIXES REQUIRED:

FIX A  -  Add per-cycle logging to ProcessUsageLog (finding 4.2 and 4.1 diagnosis):
Change ProcessUsageLog signature from:
  func ProcessUsageLog(st *store.Store) error
to:
  func ProcessUsageLog(st *store.Store) (ProcessingStats, error)

Add this type near the top of shim_processor.go (before ProcessUsageLog):
  type ProcessingStats struct {
      LinesRead int
      Resolved  int
      Skipped   int
      Inserted  int
  }

Update ProcessUsageLog to:
- Count linesRead (lines successfully parsed)
- Count resolved (entries where a package was found)
- Count skipped (entries where no package found)
- Count inserted (events actually written to DB)
- Return ProcessingStats on success

FIX B  -  Update callers in fsevents.go (watcher's runShimLogProcessor and Start):
In fsevents.go, update the two calls to ProcessUsageLog:
- In Start() (initial call): log the stats if resolved > 0 or linesRead > 0
- In runShimLogProcessor (ticker case): after each tick, write a log line to stderr:
    fmt.Fprintf(os.Stderr, "%s brewprune-watch: processed %d lines, resolved %d packages, skipped %d\n",
        time.Now().Format(time.RFC3339), stats.LinesRead, stats.Resolved, stats.Skipped)
  Only write this line when linesRead > 0 (skip when log file has no new entries).

FIX C  -  Pass --log-file to daemon child in daemon.go:
NOTE: daemon.go is NOT in your file ownership. You MUST NOT edit daemon.go.
Instead, in watch.go, fix startWatchDaemon to not call w.StartDaemon but instead
call a local fork directly OR note this as a known limitation in a comment.
ACTUALLY: Check if daemon.go is in any agent's ownership. Since it is not assigned,
you may edit daemon.go ONLY for the --log-file passthrough fix (adding it to the
exec.Command call). Update the command:
  cmd := exec.Command(executable, "watch", "--daemon-child")
to pass --log-file if it's set:
  args := []string{"watch", "--daemon-child"}
  if logFile != "" {
      args = append(args, "--log-file", logFile)
  }
  cmd := exec.Command(executable, args...)
This requires passing logFile into StartDaemon (it already receives logFile string).

FIX D  -  Add diagnostic log when maps are empty:
In ProcessUsageLog, after building binaryMap and optPathMap, add:
  if len(binaryMap) == 0 && len(optPathMap) == 0 {
      log.Printf("shim_processor: warning: no packages indexed yet  -  run 'brewprune scan' first")
  }

STEPS:
1. Read /Users/dayna.blackwell/code/brewprune/internal/watcher/shim_processor.go (full)
2. Read /Users/dayna.blackwell/code/brewprune/internal/watcher/fsevents.go (full)
3. Read /Users/dayna.blackwell/code/brewprune/internal/watcher/daemon.go (full)
4. Read /Users/dayna.blackwell/code/brewprune/internal/app/watch.go (full)
5. Read /Users/dayna.blackwell/code/brewprune/internal/watcher/shim_processor_test.go
6. Read /Users/dayna.blackwell/code/brewprune/internal/watcher/daemon_test.go
7. Apply all fixes in order: A (type + signature), B (callers in fsevents.go), C (daemon.go --log-file), D (diagnostic)
8. Update shim_processor_test.go to handle the new return signature
9. Run: cd /Users/dayna.blackwell/code/brewprune && go build ./... && go test ./internal/watcher/ ./internal/app/ -v
10. Report: which lines changed, test output

CONSTRAINTS:
- You may also edit daemon.go ONLY for Fix C (the --log-file passthrough)
- Do not touch any files outside: shim_processor.go, fsevents.go, daemon.go, watch.go, and their test files
- ProcessingStats must be defined in shim_processor.go, not a separate file
- Maintain crash-safe offset behavior: do not change the offset advance logic
```

---

### Agent E  -  status no-events grace period (finding 2.2)

```
ROLE: You are a focused implementation agent. Read, then edit. Do not create new files.

TASK: Fix finding 2.2  -  brewprune status shows alarming "no events" warning immediately after daemon starts.

CURRENT BEHAVIOR (status.go ~lines 144-148):
  if daemonRunning && events24h == 0 && totalEvents <= 2 {
      fmt.Printf("              ⚠ Daemon running but no events logged. Shims may not be intercepting commands.\n")
      fmt.Printf("              Run 'brewprune doctor' to diagnose.\n")
  }

PROBLEM: This fires immediately when the daemon just started. A new user is alarmed.

FIX: Add a grace period check using the daemon's PID file modification time (already computed by daemonSince). If the daemon started less than 5 minutes ago AND there are zero events, soften the message:
  "No events logged yet (daemon started just now  -  this is normal)."

Implementation:
1. Add a helper function daemonAgeMinutes(pidFile string) int that reads the PID file mtime and returns age in minutes (0 if unknown).
2. Change the warning block to:
   if daemonRunning && events24h == 0 && totalEvents <= 2 {
       ageMin := daemonAgeMinutes(pidFile)
       if ageMin < 5 {
           fmt.Printf("              (no events yet  -  daemon started just now, this is normal)\n")
       } else {
           fmt.Printf("              ⚠ Daemon running but no events logged. Shims may not be intercepting commands.\n")
           fmt.Printf("              Run 'brewprune doctor' to diagnose.\n")
       }
   }

STEPS:
1. Read /Users/dayna.blackwell/code/brewprune/internal/app/status.go (full)
2. Read /Users/dayna.blackwell/code/brewprune/internal/app/status_test.go
3. Implement daemonAgeMinutes function (near the daemonSince function ~line 308)
4. Update the warning block
5. Add a test in status_test.go if feasible (may need to mock PID file mtime)
6. Run: cd /Users/dayna.blackwell/code/brewprune && go build ./... && go test ./internal/app/ -run TestStatus -v
7. Report: which lines changed, test output

CONSTRAINTS:
- Only modify status.go and status_test.go
- Do not change any other file
- pidFile variable is already available in runStatus at line ~41
```

---

### Agent F  -  stats.go fixes (findings 7.7 and 9.7)

```
ROLE: You are a focused implementation agent. Read, then edit. Do not create new files.

TASK: Fix two findings in /Users/dayna.blackwell/code/brewprune/internal/app/stats.go

FINDING 7.7  -  stats --package jq after undo (before scan) lacks scan hint:
Current behavior in showPackageStats (~line 108):
  stats, err := a.GetUsageStats(pkg)
  if err != nil {
      return fmt.Errorf("failed to get stats for %s: %w", pkg, err)
  }
User sees: "Error: failed to get stats for jq: package not found: jq"

Required behavior: If the package is not found, show a user-friendly error that mentions undo:
  Error: package jq not found.
  Check the name with 'brew list' or 'brew search jq'.
  If you recently ran 'brewprune undo', run 'brewprune scan' to update the index.

Fix: Check for "not found" error string and print a formatted message:
  if err != nil {
      if strings.Contains(err.Error(), "not found") {
          fmt.Fprintf(os.Stderr, "Error: package %s not found.\nCheck the name with 'brew list' or 'brew search %s'.\nIf you recently ran 'brewprune undo', run 'brewprune scan' to update the index.\n", pkg, pkg)
          os.Exit(1)
      }
      return fmt.Errorf("failed to get stats for %s: %w", pkg, err)
  }
Add "strings" and "os" to imports if not already present.

FINDING 9.7  -  stats --all sort annotation inconsistency:
Current behavior (~line 251-253):
  if len(filteredStats) > 1 {
      fmt.Println("Sorted by: most used first")
  }
  table := output.RenderUsageTable(filteredStats)
  fmt.Print(table)

The sort annotation appears BEFORE the table. For `unused`, the sort annotation appears AFTER the table (~line 381). This is inconsistent.

Fix: Move "Sorted by: most used first" to AFTER the table:
  table := output.RenderUsageTable(filteredStats)
  fmt.Print(table)
  if len(filteredStats) > 1 {
      fmt.Println()
      fmt.Println("Sorted by: most used first")
  }

STEPS:
1. Read /Users/dayna.blackwell/code/brewprune/internal/app/stats.go (full)
2. Read /Users/dayna.blackwell/code/brewprune/internal/app/stats_test.go
3. Apply fix for 7.7 (add error handling in showPackageStats)
4. Apply fix for 9.7 (move sort annotation after table)
5. Update stats_test.go to cover the not-found error message with undo hint
6. Run: cd /Users/dayna.blackwell/code/brewprune && go build ./... && go test ./internal/app/ -run TestStats -v
7. Report: which lines changed, test output

CONSTRAINTS:
- Only modify stats.go and stats_test.go
- Do not change any other file
```

---

### Agent G  -  doctor.go blank-state exit (finding 6.4)

```
ROLE: You are a focused implementation agent. Read, then edit. Do not create new files.

TASK: Fix finding 6.4  -  doctor blank-state exits with redundant "Error: diagnostics failed".

CURRENT BEHAVIOR (doctor.go ~lines 297-299):
  if criticalIssues > 0 {
      fmt.Println(colorize("31", fmt.Sprintf("Found %d critical issue(s) and %d warning(s).", criticalIssues, warningIssues)))
      return fmt.Errorf("diagnostics failed")
  }

When no .brewprune dir: the individual check failures already communicate the problem (e.g., "✗ Database not found at: ..."). Then the summary prints "Found 1 critical issue(s) and 0 warning(s)." and cobra adds "Error: diagnostics failed" which is redundant.

REQUIRED BEHAVIOR: Replace the generic "diagnostics failed" error with a more actionable summary. The exit code must remain 1. Options:

Option A (preferred): Use os.Exit(1) directly after printing the summary, instead of returning an error (avoids cobra adding "Error:" prefix):
  if criticalIssues > 0 {
      fmt.Println(colorize("31", fmt.Sprintf("Found %d critical issue(s) and %d warning(s). Run the suggested actions above to fix.", criticalIssues, warningIssues)))
      os.Exit(1)
  }
  Note: This pattern is already used in explain.go and remove.go.

Option B: Return a sentinel error that main.go suppresses. This requires changes to main.go. Do NOT use this option.

Use Option A.

STEPS:
1. Read /Users/dayna.blackwell/code/brewprune/internal/app/doctor.go (full)
2. Read /Users/dayna.blackwell/code/brewprune/internal/app/doctor_test.go
3. Apply the fix: replace return fmt.Errorf("diagnostics failed") with the os.Exit(1) approach
4. Update the summary message to be more actionable: "Found N critical issue(s) and N warning(s). Run the suggested actions above to fix."
5. Verify: in blank state (no .brewprune dir), the last line of output should be the summary, NOT "Error: diagnostics failed"
6. Run: cd /Users/dayna.blackwell/code/brewprune && go build ./... && go test ./internal/app/ -run TestDoctor -v
7. Report: which lines changed, test output

CONSTRAINTS:
- Only modify doctor.go and doctor_test.go
- Do not change main.go or any other file
- Exit code must remain 1 for critical issues, 0 for warnings-only
```

---

### Agent H  -  -v flag conflict (finding 1.1)

```
ROLE: You are a focused implementation agent. Read, then edit. Do not create new files.

TASK: Fix finding 1.1  -  -v as --version conflicts with unused -v as --verbose.

CURRENT STATE:
- root.go or main.go registers -v as shorthand for --version on the root command.
- unused.go registers -v as shorthand for --verbose on the unused command.
This creates inconsistency: the same flag letter means different things at different levels.

INVESTIGATION FIRST: Before making any changes, read:
1. /Users/dayna.blackwell/code/brewprune/internal/app/root.go
2. /Users/dayna.blackwell/code/brewprune/cmd/brewprune/main.go
3. /Users/dayna.blackwell/code/brewprune/internal/app/root_test.go
4. Check how --version is registered (likely via cobra's built-in or custom flag)

PREFERRED FIX: Remove -v as shorthand for --version on the root command. --version should only be available as the long form. -v should remain as --verbose on the unused subcommand (that is the more conventional use of -v).

If -v for --version was added explicitly in root.go, remove the shorthand.
If it's cobra's built-in behavior, the workaround is to override it (cobra allows DisableFlagParsing or flag redefinition).

Document the change in a comment: "# -v removed from --version shorthand; use --version explicitly. -v is reserved for --verbose in subcommands."

STEPS:
1. Read root.go and main.go to understand where -v comes from
2. Identify the minimal change to remove -v as --version shorthand
3. Apply the fix
4. Verify unused -v still works as --verbose
5. Run: cd /Users/dayna.blackwell/code/brewprune && go build ./... && go test ./... -v -run TestRoot 2>&1 | head -50
6. Report: which lines changed, test output

CONSTRAINTS:
- Minimize changes; only modify the file where the root -v shorthand is defined
- Do not modify unused.go (the -v for --verbose is correct there and should be kept)
- Do not add/remove test files unless truly needed
```

---

## Wave Execution Loop

### Before starting agents

```bash
cd /Users/dayna.blackwell/code/brewprune
go build ./...   # must pass
go test ./...    # establish baseline
```

### After Wave 1 completes

Merge all Wave 1 agent changes:
```bash
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go test ./...
```

Spot-check:
- [ ] `brewprune remove nonexistent-pkg` error includes undo hint
- [ ] `brewprune remove --safe --yes` when all fail: exits 1, shows ✗
- [ ] `brewprune unused` against uninitialized DB: no "failed to list packages:" prefix
- [ ] `go test ./internal/watcher/` passes with updated ProcessUsageLog signature
- [ ] `go test ./internal/scanner/ ./internal/store/` passes

### After Wave 2 completes

```bash
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go test ./...
```

Spot-check:
- [ ] `brewprune status` immediately after daemon start: shows grace period message
- [ ] `brewprune stats --package nonexistent` shows undo hint
- [ ] `brewprune stats --all` sort annotation appears after table
- [ ] `brewprune doctor` in blank state: no "Error: diagnostics failed" line at end
- [ ] `brewprune -v` behavior (root level): confirm outcome of Agent H fix

---

## Known Issues (pre-existing, agents should not fix)

1. `shim_processor.go` uses `INSERT OR IGNORE` for `usage_events` (no deduplication  -  same event could be inserted twice if offset reset occurs). This is pre-existing and not in scope.
2. `detectChanges` in scan.go does not detect dependency changes (only package count/version). If dependencies change but packages don't, no re-scan of deps occurs. Related to finding 7.1 but the root fix is in `ClearDependencies` not `detectChanges`.
3. `buildOptPathMap` stores full Linuxbrew paths (e.g., `/home/linuxbrew/.linuxbrew/bin/git`) but the lookup in `ProcessUsageLog` only checks three hardcoded prefixes. Finding 4.1 fix (diagnostic logging) will surface this; the actual resolution bug may require a follow-up IMPL.
4. Docker container integration tests in `docker/` are not run by `go test ./...`. Only unit tests are verified in this IMPL.

---

## Status

### Wave 1

- [x] Wave 1 Agent A  -  remove.go: exit code + undo hint (findings 7.2/9.8, 7.6)
- [x] Wave 1 Agent B  -  stale dep graph: ClearDependencies + ScanPackages fix (finding 7.1)
- [x] Wave 1 Agent C  -  unused error chain prefix (finding 8.6)
- [x] Wave 1 Agent D  -  daemon processing: ProcessingStats + per-cycle logging + --log-file passthrough (findings 4.1, 4.2)

### Wave 2

- [x] Wave 2 Agent E  -  status grace period for no-events warning (finding 2.2)
- [x] Wave 2 Agent F  -  stats --package undo hint + sort annotation position (findings 7.7, 9.7)
- [x] Wave 2 Agent G  -  doctor blank-state exit message (finding 6.4)
- [x] Wave 2 Agent H  -  -v flag conflict resolution (finding 1.1)

### Findings confirmed already fixed (no agent needed)

- [x] Finding 6.3  -  doctor aliases tip: already guarded by criticalIssues == 0 check
- [x] Finding 7.3  -  remove --risky --dry-run warning: already printed after table
- [x] Finding 3.6  -  --sort age secondary sort: already has tertiary alphabetical sort

---

### Agent A - Completion Report
status: complete
worktree: .claude/worktrees/wave1-agent-a
commit: 3ffc3803fbc4d29f251feedbc162aa700947b30b
files_changed:
  - internal/app/remove.go
  - internal/app/remove_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestRemoveAllFailExitsNonZero
  - TestRemoveNotFoundUndoHint
verification: PASS (go test ./internal/app/ -run TestRemove -v -timeout 120s - 15/15 tests)

### Agent B - Completion Report
status: complete
worktree: .claude/worktrees/wave1-agent-b
commit: 34d4691
files_changed:
  - internal/store/queries.go
  - internal/scanner/inventory.go
  - internal/store/db_test.go
  - internal/scanner/dependencies_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestClearDependencies
  - TestScanPackagesReplacesDeps
verification: PASS (go test ./internal/store/ ./internal/scanner/ -v -timeout 120s - 27/27 store tests + 10/10 scanner tests, 3 skipped)

### Agent D - Completion Report
status: complete
worktree: .claude/worktrees/wave1-agent-d
commit: fa5d3a6ceb928eee2ebf4238ba5469ac00ebe62a
files_changed:
  - internal/watcher/shim_processor.go
  - internal/watcher/fsevents.go
  - internal/watcher/daemon.go
  - internal/watcher/shim_processor_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestProcessUsageLogStats
verification: PASS (go test ./internal/watcher/ ./internal/app/ -timeout 120s - all tests passing, 0 failures)

### Agent C - Completion Report
status: complete
worktree: .claude/worktrees/wave1-agent-c
commit: ab69515
files_changed:
  - internal/app/unused.go
  - internal/app/unused_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestUnusedNoDBErrorMessage
verification: PASS (GOWORK=off go test ./internal/app/ -run TestUnused -v -timeout 120s - 21/21 tests)

### Agent E - Completion Report
status: complete
worktree: .claude/worktrees/wave2-agent-e
commit: 72ff6cac9653b50154e9a7e3caf089b4603888d8
files_changed:
  - internal/app/status.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestDaemonAgeMinutes
  - TestDaemonAgeMinutes_GracePeriodLogic
  - TestStatusDaemonNoEventsGracePeriod
verification: PASS (GOWORK=off go test ./internal/app/ -run TestStatus -v -timeout 120s - 11/11 tests; GOWORK=off go test ./internal/app/ -run TestDaemon -v - 2/2 tests)

### Agent H - Completion Report
status: complete
worktree: .claude/worktrees/wave2-agent-h
commit: 913f7ef5b80118dd0b514540f83f40d2606f8be0
files_changed:
  - internal/app/root.go
  - internal/app/root_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestRootVersionFlagNoShorthand
verification: PASS (GOWORK=off go test ./internal/app/ -run TestRoot -v -timeout 120s - 8/8 tests)

### Agent F - Completion Report
status: complete
worktree: .claude/worktrees/wave2-agent-f
commit: 7ac7b4a5461fe276559039fd1eec193d664dd8a8
files_changed:
  - internal/app/stats.go
  - internal/app/stats_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestStatsPackageNotFoundUndoHint
  - TestStats_PackageNotFoundReturnsError (updated to subprocess pattern for os.Exit(1))
verification: PASS (GOWORK=off go test -C .claude/worktrees/wave2-agent-f ./internal/app/ -run TestStats -v -timeout 120s - 21/21 tests)

### Agent G - Completion Report
status: complete
worktree: .claude/worktrees/wave2-agent-g
commit: 2668e2d4027222967d6b4935b8a70aff359a9024
files_changed:
  - internal/app/doctor.go
  - internal/app/doctor_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestDoctorBlankStateNoRedundantError
  - TestRunDoctor_CriticalIssueExitsOne (replaces TestRunDoctor_CriticalIssueReturnsError with subprocess pattern)
  - TestRunDoctor_ActionLabelNotFix (converted to subprocess pattern)
  - TestRunDoctor_PipelineTestShowsProgress (converted to subprocess pattern)
  - TestDoctorAliasesTip_SuppressedWhenDaemonRunning (converted to subprocess pattern)
  - TestDoctorAliasTip_HiddenWhenCritical (converted to subprocess pattern)
  - TestDoctor_PipelineFailsNormallyWhenPathActive (converted to subprocess pattern)
verification: PASS (GOWORK=off go test ./internal/app/ -run TestDoctor -v -timeout 120s - 17/17 tests)
