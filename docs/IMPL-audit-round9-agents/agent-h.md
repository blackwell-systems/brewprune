# Wave 1 Agent H: Undo progress rendering + stale DB warning expansion

You are Wave 1 Agent H. Your task is to:
1. Fix the undo progress rendering: currently both a progress bar and an item-by-item list run
   simultaneously, producing confusing interleaved output. Pick one consistent style.
2. Expand the post-undo warning to cover ALL database-dependent commands (not just `remove`).

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claire/worktrees/wave1-agent-h 2>/dev/null || \
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-h 2>/dev/null || true

ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-h"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then
  echo "ISOLATION FAILURE: Wrong directory"; echo "Expected: $EXPECTED_DIR"; echo "Actual: $ACTUAL_DIR"; exit 1
fi

ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-h" ]; then
  echo "ISOLATION FAILURE: Wrong branch"; echo "Expected: wave1-agent-h"; echo "Actual: $ACTUAL_BRANCH"; exit 1
fi

git worktree list | grep -q "wave1-agent-h" || { echo "ISOLATION FAILURE: Worktree not in list"; exit 1; }
echo "✓ Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

## 1. File Ownership

- `internal/app/undo.go` — modify
- `internal/app/undo_test.go` — modify

## 2. Interfaces You Must Implement

No new cross-agent interfaces.

## 3. Interfaces You May Call

```go
output.NewProgress(n int, label string) *Progress   // progress bar
output.NewSpinner(msg string) *Spinner               // spinner
snapMgr.RestoreSnapshot(id int64) error              // calls brew install internally
```

## 4. What to Implement

### 4.1 Fix undo progress rendering (UX-polish)

**Current behavior (undo.go lines 147-168):**

```go
fmt.Printf("Restoring %d packages...\n", len(snapshotPackages))
progress := output.NewProgress(len(snapshotPackages), "Restoring packages")

// Use a spinner for the restoration process since snapshots.RestoreSnapshot
// doesn't provide per-package progress
progress.Finish() // Clear the progress bar
spinner := output.NewSpinner("Restoring packages from snapshot...")
spinner.Start()
err = snapMgr.RestoreSnapshot(snapshotID)
spinner.Stop()
```

**Problem:** `NewProgress` is called (which renders a progress bar at 0%), then immediately
`progress.Finish()` is called (which renders it at 100%). Then a SEPARATE spinner starts.
The audit observed: `[=================>] 100% Restoring packages` followed by
`Restoring packages from snapshot......` and then individual `Restored X` lines from
`snapshots/restore.go`.

The "Restored bat", "Restored fd", etc. lines come from `restore.go:45`:
```go
fmt.Printf("Restored %s\n", formatRestoredPkg(pkg.Name, pkg.Version))
```

This is inside `snapMgr.RestoreSnapshot()`. There's no way to suppress it from `undo.go`
without changing `restore.go` (which is out of scope).

**Fix strategy:** Remove the progress bar entirely (it's immediately finished anyway, so it
adds no value). Keep only the spinner. This gives:
```
Restoring packages from snapshot......
Restored bat
Restored fd
Restored jq
Restored ripgrep
Restored tmux
```

The spinner stops when `RestoreSnapshot` returns. The item-by-item list from `restore.go`
is printed directly to stdout during the operation — this is acceptable as the canonical
progress display for restoration.

**Implementation:**
```go
fmt.Printf("Restoring %d packages...\n", len(snapshotPackages))
// Use spinner only — progress bar is immediately finished and adds visual noise.
spinner := output.NewSpinner("Restoring packages from snapshot...")
spinner.Start()
err = snapMgr.RestoreSnapshot(snapshotID)
spinner.Stop()
```

Remove the `progress := output.NewProgress(...)` and `progress.Finish()` lines.

### 4.2 Expand post-undo warning to cover all database-dependent commands (UX-improvement)

**Current warning (undo.go line 167):**
```go
fmt.Println("\n⚠  Run 'brewprune scan' to update the package database before running 'brewprune remove'.")
```

**Problem:** The undo warning only mentions `remove`. But `explain`, `stats --package`, and
`unused` also require a fresh scan after undo (they depend on the DB having current package
data). The audit reports a segfault on `explain` and `stats --package` after undo — while
the segfault is fixed by Agent A, the warning should still cover these commands.

**Fix:**
```go
fmt.Println("\n⚠  Run 'brewprune scan' to update the package database.")
fmt.Println("   Commands that need a fresh scan: remove, unused, explain, stats --package")
```

Or as a single line:
```go
fmt.Println("\n⚠  Run 'brewprune scan' to update the database before using: remove, unused, explain, stats --package")
```

Choose the clearer wording. The key requirement is that `explain` and `stats --package` are
explicitly mentioned alongside `remove`.

## 5. Tests to Write

1. `TestUndo_ProgressBarRemovedFromOutput` — verify that the `undo` output does NOT contain
   the immediate 100% progress bar. Check that the "Restoring packages from snapshot" spinner
   message is present instead.

2. `TestUndo_PostUndoWarningMentionsAllCommands` — verify that the post-undo warning text
   mentions "explain" and "stats" (not just "remove").

Check existing undo tests for any assertions about the progress bar output and update them.

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-h
go build ./...
go vet ./...
go test ./internal/app -run 'TestUndo' -v
```

## 7. Constraints

- Do NOT modify `internal/snapshots/restore.go` — the per-package "Restored X" output is
  intentional and useful for the user to see each package being restored.
- The spinner should still wrap `RestoreSnapshot` to show activity during the (30-60s)
  installation process.
- The warning message must remain visible (printed to stdout, not stderr).

## 8. Report

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-h
git add internal/app/undo.go internal/app/undo_test.go
git commit -m "wave1-agent-h: fix undo progress rendering + expand stale DB warning"
```

Append to this file:

```yaml
### Agent H — Completion Report
status: complete | partial | blocked
worktree: .claude/worktrees/wave1-agent-h
commit: {sha}
files_changed:
  - internal/app/undo.go
  - internal/app/undo_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestUndo_ProgressBarRemovedFromOutput
  - TestUndo_PostUndoWarningMentionsAllCommands
verification: PASS | FAIL
```

---

### Agent H — Completion Report
status: complete
worktree: .claude/worktrees/wave1-agent-h
commit: 972cebb
files_changed:
  - internal/app/undo.go
  - internal/app/undo_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestUndo_ProgressBarRemovedFromOutput
  - TestUndo_PostUndoWarningMentionsAllCommands
verification: PASS
