# IMPL: Audit Round 12 Fixes

**Source audit:** `docs/cold-start-audit-r11.md`
**Written:** 2026-03-02
**Scout version:** v0.3.6

---

## Suitability Assessment

**Verdict: SUITABLE**

All 17 findings decompose into disjoint file ownership across five parallel agents. One finding (#2, daemon offset) required careful root-cause investigation during scouting (see below). No cross-agent interfaces are required beyond the pre-existing `ProcessUsageLog` / `store.Store` signatures  -  all changes are self-contained.

### Pre-implementation scan results

| # | Finding | Severity | Status | Notes |
|---|---------|----------|--------|-------|
| 1 | `-v` fails instead of printing version | UX-critical | To-do | root.go: register `-v` as shorthand for `--version` |
| 2 | Daemon skips events written before first cycle | UX-critical | To-do | root cause identified  -  see below |
| 3 | `remove` exits 0 when all packages locked/skipped | UX-critical | To-do | remove.go: return error instead of nil |
| 4 | Score inconsistency: `unused --verbose` vs `explain` | UX-improvement | To-do | Both call ComputeScore; inconsistency is display framing |
| 5 | `explain` doesn't list dependent package names | UX-improvement | To-do | explain.go: call store.GetDependents, list names |
| 6 | `doctor` lacks active PATH check | UX-improvement | Already fixed | doctor.go line 189 already calls `isOnPATH(shimDir)` |
| 7 | Quickstart self-test has no progress indication | UX-improvement | Already fixed | quickstart.go uses `output.NewSpinner` at Step 4 |
| 8 | `--verbose` output has no paging hint for large lists | UX-polish | Already fixed | unused.go lines 303-311 print paging hint when >10 packages |
| 9 | `stats` with no usage shows confusing package count | UX-polish | To-do | stats.go: fix count to use total scanned packages |
| 10 | `--help` flag section inconsistency (no `-v, --version`) | UX-polish | To-do | root.go: adding `-v` shorthand (finding #1) fixes this |
| 11 | No prereq notes in subcommand help | UX-polish | To-do | unused.go, stats.go, explain.go, remove.go: add one-liner to Long |
| 12 | `--risky` dry-run shows skipped warning before table header | UX-polish | Already fixed | remove.go line 218-220: warning printed after displayConfidenceScores() |
| 13 | `--no-snapshot` flag description needs WARNING marker | UX-polish | To-do | remove.go: update flag usage string |
| 14 | Alias tip appears on every `doctor` run | UX-polish | Already fixed | doctor.go line 226: guarded by `!daemonRunning || totalUsageEvents < 10` |
| 15 | Unknown flag errors don't suggest `--help` | UX-polish | To-do | root.go: cobra SetFlagErrorFunc |
| 16 | `explain` uses different score framing than `unused --verbose` | UX-polish | To-do | explain.go: change Breakdown header to match unused.go |
| 17 | `remove --dry-run` "DRY RUN" notice only at bottom | UX-polish | To-do | remove.go: add banner at top before table |

**Pre-implementation check details:**

- **Finding #6 (doctor active PATH check):** `doctor.go` lines 189-218 already implement a three-state PATH check: `isOnPATH(shimDir)` for active, `isConfiguredInShellProfile(shimDir)` for configured-but-not-sourced, and missing. The audit finding describes exactly what the code already does. Status: ALREADY FIXED.
- **Finding #7 (quickstart progress):** `quickstart.go` lines 205-207 already use `output.NewSpinner("Verifying shim → daemon → database pipeline")` with a `WithTimeout(35 * time.Second)` call. The spinner provides visual progress during the 30s wait. Status: ALREADY FIXED.
- **Finding #8 (paging hint):** `unused.go` lines 302-311 already print the paging tip when `len(scores) > 10` and stdout is a TTY. Status: ALREADY FIXED.
- **Finding #12 (skipped warning placement):** `remove.go` lines 213-220 (tier path) print `lockedPackages` summary AFTER `displayConfidenceScores()`. Status: ALREADY FIXED.
- **Finding #14 (alias tip):** `doctor.go` line 226 guards the aliases tip with `criticalIssues == 0 && (!daemonRunning || totalUsageEvents < 10)`. Status: ALREADY FIXED.

**Root cause  -  Finding #2 (daemon offset):**

The audit reports `usage.offset = 267`, `usage.log size = 267`. This means the offset file was written to EOF, so `ProcessUsageLog` finds offset == file size and reads 0 new lines.

Looking at `shim_processor.go` lines 181-187: when all lines are read but none resolved (i.e., `len(events) == 0`), the code still advances `newOffset` to the scanned position and writes it to the offset file. This occurs at startup when:

1. Daemon starts, `Watcher.Start()` calls `ProcessUsageLog()` immediately (fsevents.go line 45).
2. The binary maps (`binaryMap`, `optPathMap`) are built from the store  -  but if the store's package index is stale or the packages weren't committed to WAL yet, the maps may be empty.
3. With empty maps, all log lines are skipped (`stats.Skipped++`, not `stats.Resolved++`), `events` stays empty.
4. But `newOffset` has advanced to the end of the file (all lines were consumed by the reader).
5. Lines 181-184: `len(events) == 0` branch writes `newOffset` to the offset file  -  permanently skipping those entries.

**Fix:** In the `len(events) == 0` branch, only advance the offset if at least one package was indexed (i.e., `len(binaryMap) > 0 || len(optPathMap) > 0`). If no packages are indexed at all, do NOT advance the offset  -  the entries should be retried on the next tick when scan may have completed. Add a `ProcessingStats.SkippedNoIndex` field to distinguish "binary not found" skips from "no packages indexed" state.

**To-do count:** 12 findings
**Already fixed:** 5 findings (6, 7, 8, 12, 14)

---

## Known Issues

None that affect the wave structure. The score inconsistency (finding #4) is a framing/display issue: both `unused --verbose` and `explain` call `a.ComputeScore()` → `computeUsageScore()` → `store.GetLastUsage()` against the same DB. The inconsistency seen in the audit was a timing artifact (unused ran before daemon cycle, explain after). The code fix is to align the `Breakdown:` header text in `unused.go`'s `RenderConfidenceTableVerbose` to match `explain.go`'s format (finding #16).

---

## Dependency Graph

```
Wave 1 (all parallel  -  no cross-agent dependencies):
  Agent A: root.go               (findings #1, #10, #15)
  Agent B: internal/watcher/     (finding #2)
  Agent C: internal/app/remove.go (findings #3, #13, #17)
  Agent D: internal/app/explain.go + internal/output/table.go (findings #5, #16)
  Agent E: internal/app/stats.go + internal/app/unused.go + internal/app/scan.go
           (findings #9, #11)

No Wave 2 required  -  all findings are independent.
```

---

## Interface Contracts

### Agent B: shim_processor.go offset guard

The existing `ProcessingStats` struct gains one new field. Callers (`fsevents.go`) do not use `SkippedNoIndex` directly (it is for logging only), so no callers need updating:

```go
// ProcessingStats holds per-cycle summary from ProcessUsageLog.
type ProcessingStats struct {
    LinesRead      int
    Resolved       int
    Skipped        int
    Inserted       int
    SkippedNoIndex int  // NEW: lines skipped because no packages are indexed
}
```

The fix condition in the `len(events) == 0` branch:

```go
// Only advance offset when packages are indexed. If no packages are
// indexed at all, retain the current offset so entries are retried on
// the next tick after scan completes.
if newOffset != offset && (len(binaryMap) > 0 || len(optPathMap) > 0) {
    return stats, writeShimOffsetAtomic(offsetPath, newOffset)
}
return stats, nil
```

### Agent C: remove exit-code contract

When `len(packagesToRemove) == 0` after locked-package filtering, return an error (not nil):

```go
if len(packagesToRemove) == 0 {
    fmt.Println("No packages to remove.")
    return fmt.Errorf("no packages removed: all candidates were locked by dependents")
}
```

The error causes `cobra` to print it and exit 1 via `main.go`'s error handling. Do NOT call `os.Exit(1)` directly  -  let cobra handle it so the error message is consistent.

### Agent D: explain.go dependent list format

New output block after the Dependencies line in `renderExplanation`:

```
  Dependencies:  0/30 pts - 9 unused dependents
  Depended on by: curl, libssh2, krb5, ... (up to 8 names, then "and N more")
```

Uses `store.GetDependents(packageName)` which already exists in `queries.go`. The `explain.go` function already calls `a.ComputeScore(packageName)` which internally calls `store.GetDependents`  -  but the result is not surfaced to the display layer. Agent D must call `st.GetDependents(packageName)` directly in `runExplain` and pass the slice to `renderExplanation`.

`renderExplanation` signature change:
```go
// Before:
func renderExplanation(score *analyzer.ConfidenceScore, installedDate string)
// After:
func renderExplanation(score *analyzer.ConfidenceScore, installedDate string, dependents []string)
```

Only `runExplain` calls `renderExplanation`. No cross-agent impact.

---

## File Ownership Table

| File | Agent | Findings |
|------|-------|---------|
| `internal/app/root.go` | A | #1, #10, #15 |
| `internal/app/root_test.go` | A | tests for #1, #15 |
| `internal/watcher/shim_processor.go` | B | #2 |
| `internal/watcher/shim_processor_test.go` | B | tests for #2 |
| `internal/app/remove.go` | C | #3, #13, #17 |
| `internal/app/remove_test.go` | C | tests for #3, #17 |
| `internal/app/explain.go` | D | #5, #16 |
| `internal/app/explain_test.go` | D | tests for #5, #16 |
| `internal/output/table.go` | D | #16 (verbose Breakdown header) |
| `internal/app/stats.go` | E | #9, #11 (stats prereq) |
| `internal/app/unused.go` | E | #11 (unused prereq) |
| `internal/app/scan.go` | E | #11 (scan prereq note  -  not needed, scan is the prereq) |
| `internal/app/stats_test.go` | E | tests for #9 |
| `internal/app/unused_test.go` | E | tests for #11 |

**No file is owned by more than one agent.**

---

## Wave Structure

### Wave 1  -  Single wave, all agents parallel

All five agents may start simultaneously. There are no shared files and no interface dependencies between agents.

```
Wave 1:
  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐
  │ Agent A │  │ Agent B │  │ Agent C │  │ Agent D │  │ Agent E │
  │ root.go │  │ shim_   │  │ remove  │  │ explain │  │ stats   │
  │ -v flag │  │processor│  │ exit 1  │  │ deps    │  │ unused  │
  │ --help  │  │ offset  │  │ dry-run │  │ framing │  │ prereqs │
  │ suggest │  │ bug     │  │ banner  │  │         │  │ count   │
  └─────────┘  └─────────┘  └─────────┘  └─────────┘  └─────────┘
       ↓              ↓            ↓            ↓            ↓
  Post-merge: go build ./... && go vet ./... && go test ./...
```

---

## Agent Prompts

---

### Agent A  -  Root Command: `-v` shorthand, help flag listing, unknown-flag `--help` suggestion

**1. Role**

Fix three root-command UX issues: register `-v` as the shorthand for `--version`; add a `--help` suggestion to unknown-flag errors; verify help flags are displayed consistently.

**2. Context**

Finding #1: `brewprune -v` errors with "unknown shorthand flag: 'v' in -v". The comment in `root.go` line 112 says `-v` is "reserved for --verbose in subcommands"  -  but cobra shorthand flags are scoped per command, so `-v` on root and `-v` on `unused` (a subcommand) do NOT conflict. The unused subcommand registers its own `-v` shorthand at the subcommand level (unused.go line 83: `BoolVarP(&unusedVerbose, "verbose", "v", false, ...)`), which is independent of the root command's flags.

Finding #10: Once `-v` is registered, `brewprune --help` will show `-v, --version` in the Flags section automatically (cobra behavior). No separate fix needed.

Finding #15: Unknown flag errors (e.g., `brewprune unused --invalid-flag`) currently print "Error: unknown flag: --invalid-flag" with no next step. Cobra provides `cobra.Command.SetFlagErrorFunc` to customize this behavior.

**3. Files owned**

- `/Users/dayna.blackwell/code/brewprune/internal/app/root.go`
- `/Users/dayna.blackwell/code/brewprune/internal/app/root_test.go`

**4. Interface contracts**

None. Root command flags are consumed only by cobra's dispatch mechanism.

**5. Implementation tasks**

1. **Register `-v` as shorthand for `--version`** (root.go `init()` function, line 113):
   ```go
   // Before:
   RootCmd.Flags().BoolVar(&versionFlag, "version", false, "show version information")
   // After:
   RootCmd.Flags().BoolVarP(&versionFlag, "version", "v", false, "show version information")
   ```
   Remove the comment on line 112 that says `-v` is reserved.

2. **Add `--help` suggestion to unknown-flag errors** for all subcommands. The cleanest approach is to set a `PersistentPreRunE` on `RootCmd` or use `SetFlagErrorFunc` on each subcommand. Because cobra does not support `SetFlagErrorFunc` on the root propagating to children, the recommended pattern is:
   - In `RootCmd`'s `init()`, after all subcommands are added, iterate them and call `SetFlagErrorFunc`:
   ```go
   for _, sub := range RootCmd.Commands() {
       sub := sub // capture
       sub.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
           return fmt.Errorf("%w\nRun 'brewprune %s --help' to see valid flags.", err, cmd.Name())
       })
   }
   ```
   Place this at the bottom of `init()` in root.go, after all `AddCommand` calls.

3. The existing `TestRootVersionFlagNoShorthand` test in `root_test.go` currently asserts that `-v` is NOT registered on root. This test was written to document the previous decision. **Update it** to assert that `-v` IS the shorthand for `--version`, and that `unused`'s `-v` still works (the subcommand flag is independent).

**6. Tests to write/update**

In `root_test.go`:

- `TestRootVersionFlagNoShorthand` → rename to `TestRootVersionFlagShorthand` and flip the assertion:
  ```go
  flag := RootCmd.Flags().ShorthandLookup("v")
  if flag == nil {
      t.Error("expected -v to be registered as shorthand for --version on root command")
  }
  if flag != nil && flag.Name != "version" {
      t.Errorf("expected -v shorthand to map to 'version', got %q", flag.Name)
  }
  ```

- `TestRootVersionFlagShorthand_PrintsVersion`  -  new test that calls `RootCmd.RunE` with `versionFlag = true` and verifies the version string is printed (mirrors `TestRootCmd_BareInvocationShowsHelp` pattern):
  ```go
  func TestRootVersionFlagShorthand_PrintsVersion(t *testing.T) { ... }
  ```

- `TestUnknownFlagSuggestsHelp`  -  new test that calls a subcommand's `FlagErrorFunc` with a synthetic error and verifies the "Run 'brewprune <cmd> --help'" suffix appears.

**7. Verification gate**

```bash
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/app -run TestRoot -v
go test ./internal/app -run TestUnknownFlag -v
```

**8. Out-of-scope**

- Do NOT modify `unused.go` or its `-v` shorthand  -  that is Agent E's file and the subcommand flag does not conflict.
- Do NOT modify any other subcommand's flag definitions.
- Do NOT change the `validCommandsList` string.

**9. Completion report format**

When done, append to this IMPL doc:

```yaml
agent: A
status: complete
findings_fixed: [1, 10, 15]
files_modified:
  - internal/app/root.go
  - internal/app/root_test.go
tests_added:
  - TestRootVersionFlagShorthand
  - TestRootVersionFlagShorthand_PrintsVersion
  - TestUnknownFlagSuggestsHelp
tests_updated:
  - TestRootVersionFlagNoShorthand  # renamed and assertion flipped
build_ok: true
vet_ok: true
notes: ""
```

---

### Agent B  -  Watcher: Daemon Offset Initialization Bug

**1. Role**

Fix the critical daemon offset bug that causes all pre-existing `usage.log` entries to be permanently skipped when the daemon starts with an empty (or stale) package index.

**2. Context**

Finding #2: After `scan` + `watch --daemon` + 5 shim commands + `sleep 35`, status shows "Events: 0". Root cause (identified during scouting):

`Watcher.Start()` (fsevents.go line 45) immediately calls `ProcessUsageLog()`. Inside `ProcessUsageLog`:
1. `readShimOffset` returns the stored offset (or 0 if no offset file).
2. `buildBasenameMap` and `buildOptPathMap` build lookups from the store.
3. All log lines are read from offset. For each line, the binary name is looked up in the maps.
4. **If the maps are empty** (no packages indexed, or store not yet populated), every line fails lookup → `stats.Skipped++` → line is not added to `events`.
5. After the read loop, `newOffset` has advanced past all read lines.
6. In the `len(events) == 0` branch (lines 181-187), **`newOffset != offset`** so `writeShimOffsetAtomic` is called  -  permanently advancing the offset to EOF.
7. On the next tick, `readShimOffset` returns the EOF offset → no lines are read → events stay at 0 forever.

The fix: in the `len(events) == 0` branch, only write the new offset when the package maps are non-empty. If both maps are empty, the skip was due to "no packages indexed", not "binary not found"  -  retain the old offset so entries are retried.

**3. Files owned**

- `/Users/dayna.blackwell/code/brewprune/internal/watcher/shim_processor.go`
- `/Users/dayna.blackwell/code/brewprune/internal/watcher/shim_processor_test.go`

**4. Interface contracts**

Add `SkippedNoIndex int` field to `ProcessingStats` (new, additive  -  callers that ignore it are unaffected):

```go
type ProcessingStats struct {
    LinesRead      int
    Resolved       int
    Skipped        int
    Inserted       int
    SkippedNoIndex int  // lines not advanced because binaryMap+optPathMap were both empty
}
```

**5. Implementation tasks**

1. **Add `SkippedNoIndex` to `ProcessingStats`** struct (lines 20-26 of shim_processor.go).

2. **Add a `noPackagesIndexed` boolean** after building the maps:
   ```go
   noPackagesIndexed := len(binaryMap) == 0 && len(optPathMap) == 0
   ```

3. **Modify the `len(events) == 0` branch** (currently lines 181-187):
   ```go
   if len(events) == 0 {
       if noPackagesIndexed {
           // No packages are indexed  -  do NOT advance offset.
           // Retain current offset so these entries are retried on next tick
           // after 'brewprune scan' has populated the database.
           stats.SkippedNoIndex = stats.LinesRead // all read lines were retained
           log.Printf("shim_processor: offset not advanced (no packages indexed yet  -  run 'brewprune scan')")
           return stats, nil
       }
       // Advance offset even if no events matched (skip unknown binaries).
       if newOffset != offset {
           return stats, writeShimOffsetAtomic(offsetPath, newOffset)
       }
       return stats, nil
   }
   ```

4. **Update `logProcessingStats`** in `fsevents.go` to log `SkippedNoIndex` if non-zero  -  but `fsevents.go` is NOT in Agent B's file list. Instead, add a `log.Printf` inside `ProcessUsageLog` itself (already done in step 3 above).

   Actually: `fsevents.go` is also in `internal/watcher/`  -  Agent B owns all of `internal/watcher/`. Agent B **may** update `fsevents.go`'s `logProcessingStats` to print `SkippedNoIndex`, but this is optional and low-priority.

**6. Tests to write/update**

In `shim_processor_test.go`, add:

- `TestProcessUsageLog_EmptyMapsDoNotAdvanceOffset`  -  creates a temp `usage.log` with valid entries, creates an in-memory store WITHOUT any packages (so maps are empty), calls `ProcessUsageLog`, then reads `usage.offset` and asserts it is still 0 (not advanced):
  ```go
  func TestProcessUsageLog_EmptyMapsDoNotAdvanceOffset(t *testing.T) {
      // arrange: temp home, usage.log with 3 valid lines, empty store (no packages)
      // act: ProcessUsageLog(emptyStore)
      // assert: usage.offset file does not exist OR contains "0"
      // assert: stats.SkippedNoIndex == 3
  }
  ```

- `TestProcessUsageLog_WithPackagesAdvancesOffset`  -  control test: same log + store WITH packages → offset IS advanced and events inserted.

- `TestProcessUsageLog_PartialMapsAdvanceOffset`  -  store has one package (maps non-empty), log has entries for that package AND unknown binaries → offset advances (the one known event is resolved, unknown entries become `stats.Skipped`).

**7. Verification gate**

```bash
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/watcher -run TestProcessUsageLog -v
go test ./internal/watcher -v
```

**8. Out-of-scope**

- Do NOT modify `fsevents.go`'s Watcher start sequence  -  the fix is entirely in `ProcessUsageLog`.
- Do NOT change the 30-second ticker interval.
- Do NOT modify any `internal/app/` files.

**9. Completion report format**

```yaml
agent: B
status: complete
findings_fixed: [2]
files_modified:
  - internal/watcher/shim_processor.go
  - internal/watcher/shim_processor_test.go
tests_added:
  - TestProcessUsageLog_EmptyMapsDoNotAdvanceOffset
  - TestProcessUsageLog_WithPackagesAdvancesOffset
  - TestProcessUsageLog_PartialMapsAdvanceOffset
build_ok: true
vet_ok: true
notes: ""
```

---

### Agent C  -  Remove Command: Exit Code, Dry-Run Banner, `--no-snapshot` Warning

**1. Role**

Fix three `remove` command UX issues: exit 1 when nothing is removed due to all candidates being locked; add a prominent "DRY RUN" banner at the top of dry-run output; strengthen the `--no-snapshot` flag description.

**2. Context**

Finding #3: `brewprune remove openssl@3` prints "No packages to remove." and exits 0. Scripting users cannot detect the "nothing removed" case. The fix is to return an error (not nil) when `packagesToRemove` is empty after dep-lock filtering.

Finding #17: `remove --dry-run` shows "Dry-run mode: no packages will be removed." only at the very bottom after the full summary. A user who pipes the output or whose terminal scrolls may miss this. Fix: print a "DRY RUN  -  NO CHANGES WILL BE MADE" banner immediately after the table header line, before the package table.

Finding #13: `--no-snapshot` flag description "(dangerous)" in the Examples section is too subtle. The flag's `Usage` string in `init()` currently reads "Skip automatic snapshot creation (dangerous)". Fix: change it to "Skip automatic snapshot creation [WARNING: removal cannot be undone]".

**3. Files owned**

- `/Users/dayna.blackwell/code/brewprune/internal/app/remove.go`
- `/Users/dayna.blackwell/code/brewprune/internal/app/remove_test.go`

**4. Interface contracts**

The exit-code fix must use `return fmt.Errorf(...)` (not `os.Exit`) so cobra handles error formatting consistently with all other commands. The error message must be actionable:

```go
if len(packagesToRemove) == 0 {
    fmt.Println("No packages to remove.")
    return fmt.Errorf("all candidates were skipped (locked by dependents)  -  run with --verbose for details")
}
```

**5. Implementation tasks**

1. **Finding #3  -  exit 1 on no-op removal** (remove.go lines 223-226):
   ```go
   // Before:
   if len(packagesToRemove) == 0 {
       fmt.Println("No packages to remove.")
       return nil
   }
   // After:
   if len(packagesToRemove) == 0 {
       fmt.Println("No packages to remove.")
       return fmt.Errorf("all candidates were skipped (locked by dependents)  -  run with --verbose for details")
   }
   ```
   This applies to BOTH the explicit-package path and the tier-based path. In the explicit path the empty-list case means all specified packages were dep-locked. In the tier-based path it means all tier packages were dep-locked. Both should exit 1.

2. **Finding #17  -  DRY RUN banner at top** (remove.go). The dry-run banner must appear BEFORE the package table, immediately after the "Packages to remove (X tier):" label line. Currently the flow is:
   ```
   fmt.Printf("\nPackages to remove (%s tier):\n\n", tier)
   displayConfidenceScores(st, scores)
   // skipped summary after table
   ```
   Change to:
   ```go
   fmt.Printf("\nPackages to remove (%s tier):\n", tier)
   if removeFlagDryRun {
       fmt.Println()
       fmt.Println("  *** DRY RUN  -  NO CHANGES WILL BE MADE ***")
   }
   fmt.Println()
   displayConfidenceScores(st, scores)
   ```
   Apply the same pattern in the explicit-package path (the `fmt.Printf("\nPackages to remove (explicit):\n\n"` block).

3. **Finding #13  -  `--no-snapshot` flag warning** (remove.go `init()` line 74):
   ```go
   // Before:
   removeCmd.Flags().BoolVar(&removeFlagNoSnapshot, "no-snapshot", false, "Skip automatic snapshot creation (dangerous)")
   // After:
   removeCmd.Flags().BoolVar(&removeFlagNoSnapshot, "no-snapshot", false, "Skip automatic snapshot creation [WARNING: removal cannot be undone]")
   ```

**6. Tests to write/update**

In `remove_test.go`:

- `TestRemoveAllLockedExitsNonZero`  -  new test that simulates `packagesToRemove` being empty after filtering and verifies the returned error is non-nil:
  ```go
  func TestRemoveAllLockedExitsNonZero(t *testing.T) {
      // Test the empty packagesToRemove branch logic
      // Simulate: all packages locked → packagesToRemove is empty
      // Assert: error returned (not nil), message contains "skipped"
  }
  ```

- `TestDryRunBannerAppearsAtTop`  -  new test that verifies the DRY RUN banner string appears in output before the table header. Can be done by capturing stdout and checking string positions:
  ```go
  func TestDryRunBannerAppearsAtTop(t *testing.T) {
      // Verify "DRY RUN" appears before any package name in the output
  }
  ```

- `TestNoSnapshotFlagDescription`  -  new test that checks the `--no-snapshot` flag usage string contains "WARNING":
  ```go
  func TestNoSnapshotFlagDescription(t *testing.T) {
      flag := removeCmd.Flags().Lookup("no-snapshot")
      if flag == nil { t.Fatal("no-snapshot flag not found") }
      if !strings.Contains(flag.Usage, "WARNING") {
          t.Errorf("no-snapshot flag Usage %q does not contain 'WARNING'", flag.Usage)
      }
  }
  ```

**7. Verification gate**

```bash
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/app -run TestRemove -v
go test ./internal/app -run TestDryRun -v
go test ./internal/app -run TestNoSnapshot -v
```

**8. Out-of-scope**

- Do NOT modify `output/table.go` or `explain.go`.
- Do NOT change the `displayConfidenceScores` function signature.
- Do NOT change confirmation logic for non-dry-run removals.
- The existing "Dry-run mode: no packages will be removed." line at the bottom should remain  -  the fix adds a banner at the TOP, it does not remove the bottom notice.

**9. Completion report format**

```yaml
agent: C
status: complete
findings_fixed: [3, 13, 17]
files_modified:
  - internal/app/remove.go
  - internal/app/remove_test.go
tests_added:
  - TestRemoveAllLockedExitsNonZero
  - TestDryRunBannerAppearsAtTop
  - TestNoSnapshotFlagDescription
build_ok: true
vet_ok: true
notes: ""
```

---

### Agent D  -  Explain Command: Dependent Names List + Score Framing Consistency

**1. Role**

Fix two `explain` command UX issues: list the names of dependent packages (not just the count); align the `Breakdown:` header framing with `unused --verbose`.

**2. Context**

Finding #5: `brewprune explain openssl@3` shows "Dependencies: 0/30 pts - 9 unused dependents" but does not list which packages depend on openssl@3. The user must manually run `brew uses openssl@3` to discover this. Fix: call `store.GetDependents(packageName)` in `runExplain` and display the dependent names under the Dependencies line.

Finding #16: `explain` prints:
```
Breakdown:
  (removal confidence score: 0 = keep, 100 = safe to remove)
  Usage: ...
```
While `unused --verbose` (via `output.RenderConfidenceTableVerbose`) prints:
```
Breakdown:
  Usage: ...
```
without any parenthetical. The fix aligns them: update `RenderConfidenceTableVerbose` in `output/table.go` to add the same explanatory line, so both commands show:
```
Breakdown:
  (score measures removal confidence: higher = safer to remove)
```
Note: `unused.go`'s `showConfidenceAssessment` (line 593) already prints `"  (score measures removal confidence: higher = safer to remove)"`. The verbose table at line 236 in `table.go` currently only prints `"\nBreakdown:\n"`. Make `RenderConfidenceTableVerbose` consistent with `explain.go`'s wording.

**3. Files owned**

- `/Users/dayna.blackwell/code/brewprune/internal/app/explain.go`
- `/Users/dayna.blackwell/code/brewprune/internal/app/explain_test.go`
- `/Users/dayna.blackwell/code/brewprune/internal/output/table.go`

**4. Interface contracts**

`renderExplanation` signature change (explain.go only, not exported):

```go
// Before:
func renderExplanation(score *analyzer.ConfidenceScore, installedDate string)
// After:
func renderExplanation(score *analyzer.ConfidenceScore, installedDate string, dependents []string)
```

Only called from `runExplain`. No other callers.

`store.GetDependents(pkg string) ([]string, error)` already exists in `queries.go`. Agent D calls it directly in `runExplain`:
```go
dependents, err := st.GetDependents(packageName)
if err != nil {
    dependents = nil // non-fatal: best effort
}
```

**5. Implementation tasks**

1. **Finding #5  -  list dependent names** (explain.go):

   In `runExplain` (after `a.ComputeScore`), retrieve dependents:
   ```go
   dependents, err := st.GetDependents(packageName)
   if err != nil {
       dependents = nil // non-fatal
   }
   renderExplanation(score, installedDate, dependents)
   ```

   In `renderExplanation`, after the Dependencies line, add:
   ```go
   fmt.Printf("  %-13s %2d/30 pts - %s\n", "Dependencies:", score.DepsScore, truncateDetail(score.Explanation.DepsDetail, 50))
   if len(dependents) > 0 {
       const maxNames = 8
       var listed []string
       if len(dependents) <= maxNames {
           listed = dependents
           fmt.Printf("  Depended on by: %s\n", strings.Join(listed, ", "))
       } else {
           listed = dependents[:maxNames]
           fmt.Printf("  Depended on by: %s, and %d more\n", strings.Join(listed, ", "), len(dependents)-maxNames)
       }
   }
   ```

2. **Finding #16  -  score framing consistency** (output/table.go `RenderConfidenceTableVerbose`):

   After line 236 (`sb.WriteString("\nBreakdown:\n")`), add:
   ```go
   sb.WriteString("  (score measures removal confidence: higher = safer to remove)\n")
   ```
   This matches the phrasing already used in `unused.go`'s `showConfidenceAssessment` (line 594).

   Also update `explain.go`'s `renderExplanation` (line 136):
   ```go
   // Before:
   fmt.Println("  (removal confidence score: 0 = keep, 100 = safe to remove)")
   // After:
   fmt.Println("  (score measures removal confidence: higher = safer to remove)")
   ```
   This makes both commands use identical phrasing.

**6. Tests to write/update**

In `explain_test.go`:

- `TestRenderExplanation_ListsDependentNames`  -  creates a `ConfidenceScore` with `DepsScore=0`, calls `renderExplanation` with `dependents=[]string{"curl","libssh2","krb5"}`, captures stdout, asserts "Depended on by:" and the package names appear:
  ```go
  func TestRenderExplanation_ListsDependentNames(t *testing.T) { ... }
  ```

- `TestRenderExplanation_TruncatesLongDependentList`  -  10 dependents → asserts "and 2 more" appears.

- `TestRenderExplanation_NoDependents`  -  `dependents=nil` → asserts "Depended on by" does NOT appear.

- `TestRenderExplanation_BreakdownFramingConsistent`  -  asserts the output contains "score measures removal confidence" (not the old "0 = keep, 100 = safe to remove" phrasing).

In `table.go` tests (if a test file exists at `internal/output/`):

- Check if `internal/output/table_test.go` exists. If so, add `TestRenderConfidenceTableVerbose_BreakdownHeader` to verify the new framing line appears in verbose output.

**7. Verification gate**

```bash
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/app -run TestRenderExplanation -v
go test ./internal/app -run TestExplain -v
go test ./internal/output -v
```

**8. Out-of-scope**

- Do NOT modify `unused.go` or `stats.go`.
- Do NOT modify `analyzer/confidence.go`  -  the score computation is correct.
- Do NOT add new store methods  -  `GetDependents` already exists.
- Do NOT change `showConfidenceAssessment` in `unused.go`  -  Agent E owns that file.

**9. Completion report format**

```yaml
agent: D
status: complete
findings_fixed: [5, 16]
files_modified:
  - internal/app/explain.go
  - internal/app/explain_test.go
  - internal/output/table.go
tests_added:
  - TestRenderExplanation_ListsDependentNames
  - TestRenderExplanation_TruncatesLongDependentList
  - TestRenderExplanation_NoDependents
  - TestRenderExplanation_BreakdownFramingConsistent
build_ok: true
vet_ok: true
notes: ""
```

---

### Agent E  -  Stats Count Fix + Prereq Notes in Subcommand Help

**1. Role**

Fix the confusing package count in `stats` when no usage exists; add one-line prerequisite notes to `stats`, `unused`, `explain`, and `remove` subcommand help text.

**2. Context**

Finding #9: After a fresh scan of 40 packages, `brewprune stats` prints "No usage recorded yet (35 packages with 0 runs)." but scan reported 40 packages. The count `35` comes from `hiddenCount` which counts packages with `TotalRuns == 0` in the current time window, not the total scanned count. The fix: when no usage has been recorded at all, get the total count from `db.ListPackages()` and use that in the message.

Finding #11: Subcommand help pages (`stats --help`, `unused --help`, `explain --help`, `remove --help`) do not mention that `brewprune scan` must be run first. The fix: add a "Requires: run 'brewprune scan' first to initialize the database" line to the `Long` description of each command.

**3. Files owned**

- `/Users/dayna.blackwell/code/brewprune/internal/app/stats.go`
- `/Users/dayna.blackwell/code/brewprune/internal/app/stats_test.go`
- `/Users/dayna.blackwell/code/brewprune/internal/app/unused.go`
- `/Users/dayna.blackwell/code/brewprune/internal/app/unused_test.go`

Note: `explain.go` and `remove.go` also need prereq notes (finding #11), but those files are owned by Agents D and C respectively. Agent E must NOT modify those files. Instead:
- Agent C (remove.go) should add the prereq note to `removeCmd.Long`.
- Agent D (explain.go) should add the prereq note to `explainCmd.Long`.
- Agents C and D should be notified of this requirement via their agent prompts.

**Update for Agents C and D:** Each should also add the following line to the `Long` description of their commands, immediately before the `Example:` section:

```
Requires: run 'brewprune scan' first to initialize the database.
```

**4. Interface contracts**

None. Stats and unused changes are self-contained output text.

**5. Implementation tasks**

1. **Finding #9  -  fix `stats` package count** (stats.go `showUsageTrends` function):

   The current no-usage path (lines 238-244):
   ```go
   if len(filteredStats) == 0 {
       if hiddenCount > 0 {
           fmt.Printf("No usage recorded yet (%d packages with 0 runs)...", hiddenCount)
       } else {
           fmt.Println("No usage data found...")
       }
       return nil
   }
   ```

   The problem: `hiddenCount` counts only packages in the `outputStats` map (packages with non-zero usage in the `trends` result). If the DB has no usage events at all, `trends` returns all packages with `TotalUses=0`, so `hiddenCount` may not equal the total scanned count (it should equal `len(outputStats)`, but `len(trends)` may differ if some packages have historical data vs. zero).

   Fix: replace the confusing count with `len(trends)` (total packages in the analysis window), which always equals the number of scanned packages:
   ```go
   if len(filteredStats) == 0 {
       if hiddenCount > 0 {
           totalScanned := len(trends)
           fmt.Printf("No usage recorded yet (%d packages with 0 runs). Run 'brewprune watch --daemon' to start tracking.\n", totalScanned)
       } else {
           fmt.Println("No usage data found. Run 'brewprune watch' to collect usage data.")
       }
       return nil
   }
   ```

   This ensures the count matches `scan` output. If `len(trends)` still does not match (possible if the analyzer filters some packages), an alternative is to call `st.ListPackages()` directly at the top of `showUsageTrends` and use `len(packages)`. Choose whichever produces the consistent count  -  prefer `len(trends)` first.

2. **Finding #11  -  prereq notes in `stats` and `unused`** (stats.go, unused.go):

   For `statsCmd.Long` (stats.go), prepend or append:
   ```
   Requires: run 'brewprune scan' first to initialize the database.
   ```
   Place it as the last paragraph before Examples, after the frequency classification block.

   For `unusedCmd.Long` (unused.go), add similarly.

   Exact placement: add a blank line + the note at the end of each `Long` string, before closing the backtick. Example for stats.go:
   ```go
   Long: `Display usage statistics and trends for installed packages.
   ...existing text...
   Requires: run 'brewprune scan' first to initialize the database.`,
   ```

3. **Reminder for Agents C and D:** Also add the prereq line to `removeCmd.Long` (Agent C) and `explainCmd.Long` (Agent D).

**6. Tests to write/update**

In `stats_test.go`:

- `TestStatsNoUsageMessageIncludesTotalCount`  -  creates a store with 5 packages but no usage events, calls `showUsageTrends` (or the relevant logic), captures stdout, asserts the printed count equals `len(packages)`:
  ```go
  func TestStatsNoUsageMessageIncludesTotalCount(t *testing.T) { ... }
  ```

In `unused_test.go`:

- `TestUnusedLongHasPrereqNote`  -  asserts `unusedCmd.Long` contains "brewprune scan":
  ```go
  func TestUnusedLongHasPrereqNote(t *testing.T) {
      if !strings.Contains(unusedCmd.Long, "brewprune scan") {
          t.Error("unusedCmd.Long should mention 'brewprune scan' as a prerequisite")
      }
  }
  ```

In `stats_test.go`:

- `TestStatsLongHasPrereqNote`  -  same pattern for `statsCmd.Long`.

**7. Verification gate**

```bash
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/app -run TestStats -v
go test ./internal/app -run TestUnused -v
```

**8. Out-of-scope**

- Do NOT modify `explain.go` or `remove.go` for the prereq notes  -  those belong to Agents D and C.
- Do NOT change the `--verbose` paging hint logic  -  already fixed.
- Do NOT modify the analyzer or store packages.

**9. Completion report format**

```yaml
agent: E
status: complete
findings_fixed: [9, 11]
files_modified:
  - internal/app/stats.go
  - internal/app/stats_test.go
  - internal/app/unused.go
  - internal/app/unused_test.go
tests_added:
  - TestStatsNoUsageMessageIncludesTotalCount
  - TestUnusedLongHasPrereqNote
  - TestStatsLongHasPrereqNote
build_ok: true
vet_ok: true
notes: "Agents C and D must also add prereq notes to their respective Long strings"
```

---

## Wave Execution Loop

```
1. Start all 5 agents simultaneously (Wave 1).
2. Each agent self-verifies with: go build ./... && go vet ./... && focused go test
3. Each agent appends its completion YAML to this doc.
4. Orchestrator collects all 5 completion reports.
5. Orchestrator runs full post-merge suite: go build ./... && go vet ./... && go test ./...
6. Orchestrator confirms regression tests still pass (see checklist below).
```

---

## Orchestrator Post-Merge Checklist

After all agents complete and their changes are merged, the orchestrator runs:

```bash
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./...
```

Then verifies each regression item:

| Regression Check | Command to verify | Expected |
|---|---|---|
| `-v` shows version | `./brewprune -v` | prints version string, exits 0 |
| `--verbose` subcommand `-v` still works | `./brewprune unused -v --tier safe 2>/dev/null \| head -5` | verbose output, not error |
| Daemon offset not advanced on empty index | `go test ./internal/watcher -run TestProcessUsageLog_EmptyMapsDoNotAdvanceOffset` | PASS |
| `remove` exits 1 on all-locked | confirm `TestRemoveAllLockedExitsNonZero` passes | PASS |
| explain lists dependent names | `go test ./internal/app -run TestRenderExplanation_ListsDependentNames` | PASS |
| Verbose breakdown framing consistent | `go test ./internal/app -run TestRenderExplanation_BreakdownFramingConsistent` | PASS |
| stats count matches total | `go test ./internal/app -run TestStatsNoUsageMessageIncludesTotalCount` | PASS |
| prereq notes present | `go test ./internal/app -run TestUnusedLongHasPrereqNote` | PASS |
| Existing tests not broken | `go test ./...` | all pass |

---

## Status Checklist

```
[ ] Agent A  -  root.go: -v shorthand, --help suggestion (findings #1, #10, #15)
[ ] Agent B  -  shim_processor.go: offset bug (finding #2)
[ ] Agent C  -  remove.go: exit code, dry-run banner, --no-snapshot warning (findings #3, #13, #17)
[ ] Agent D  -  explain.go + table.go: dependent names, framing (findings #5, #16)
[ ] Agent E  -  stats.go + unused.go: count fix, prereq notes (findings #9, #11)
[ ] Post-merge full suite: go build ./... && go vet ./... && go test ./...
[ ] Orchestrator regression checklist passed
```

---

## Already Fixed (No Action Required)

| Finding | Evidence |
|---------|---------|
| #6 doctor active PATH check | `doctor.go` lines 189-218: `isOnPATH(shimDir)` → three-state PATH messaging |
| #7 quickstart self-test progress | `quickstart.go` lines 205-207: `output.NewSpinner(...)` with 35s timeout |
| #8 --verbose paging hint | `unused.go` lines 302-311: prints tip when >10 packages on TTY |
| #12 --risky dry-run warning placement | `remove.go` lines 213-220: summary after `displayConfidenceScores()` |
| #14 alias tip conditional | `doctor.go` line 226: `!daemonRunning || totalUsageEvents < 10` guard |

---

### Agent B  -  Completion Report

agent: B
status: complete
findings_fixed: [2]
files_modified:
  - internal/watcher/shim_processor.go
  - internal/watcher/shim_processor_test.go
tests_added:
  - TestProcessUsageLog_EmptyMapsDoNotAdvanceOffset
  - TestProcessUsageLog_WithPackagesAdvancesOffset
  - TestProcessUsageLog_PartialMapsAdvanceOffset
build_ok: true
vet_ok: true
notes: ""

### Agent A  -  Completion Report

agent: A
status: complete
findings_fixed: [1, 10, 15]
files_modified:
  - internal/app/root.go
  - internal/app/root_test.go
tests_added:
  - TestRootVersionFlagShorthand
  - TestUnknownFlagSuggestsHelp
tests_updated:
  - TestRootVersionFlagNoShorthand (renamed + assertion flipped)
build_ok: true
vet_ok: true
notes: "FlagErrorFunc setup extracted into setupFlagErrorFuncs() called from Execute(); TestMain in root_test.go calls setupFlagErrorFuncs() before tests so TestUnknownFlagSuggestsHelp can inspect the func without calling Execute(). This is needed because subcommands (unused, etc.) are registered by other files' init() functions that run after root.go's init(), making a simple post-AddCommand loop in init() incomplete."

### Agent C  -  Completion Report

agent: C
status: complete
findings_fixed: [3, 11 (remove.go prereq note), 13, 17]
files_modified:
  - internal/app/remove.go
  - internal/app/remove_test.go
tests_added:
  - TestRemoveAllLockedExitsNonZero
  - TestDryRunBannerAppearsAtTop
  - TestNoSnapshotFlagDescription
build_ok: true
vet_ok: true
notes: ""

### Agent E  -  Completion Report

agent: E
status: complete
findings_fixed: [9, 11 (stats.go and unused.go prereq notes)]
files_modified:
  - internal/app/stats.go
  - internal/app/stats_test.go
  - internal/app/unused.go
  - internal/app/unused_test.go
tests_added:
  - TestStatsNoUsageMessageIncludesTotalCount
  - TestStatsLongHasPrereqNote
  - TestUnusedLongHasPrereqNote
build_ok: true
vet_ok: true
notes: "Agents C and D handle prereq notes for remove.go and explain.go respectively"

*End of IMPL-audit-round12.md*

### Agent D  -  Completion Report

agent: D
status: complete
findings_fixed: [5, 11 (explain.go prereq note), 16]
files_modified:
  - internal/app/explain.go
  - internal/app/explain_test.go
  - internal/output/table.go
tests_added:
  - TestRenderExplanation_ListsDependentNames
  - TestRenderExplanation_TruncatesLongDependentList
  - TestRenderExplanation_NoDependents
  - TestRenderExplanation_BreakdownFramingConsistent
build_ok: true
vet_ok: true
notes: ""
