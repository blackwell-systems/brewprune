# Wave 1 Agent C: undo.go — help section ordering, exit codes, error framing

You are Wave 1 Agent C. Fix three UX issues in undo.go.

## 1. File Ownership

You own these files. Do not touch any other files.
- `internal/app/undo.go` — modify
- `internal/app/undo_test.go` — modify

## 2. Interfaces You Must Implement

No new exported functions. You are modifying existing behavior only.

## 3. Interfaces You May Call

Existing standard library (`os`, `fmt`).

## 4. What to Implement

Read `internal/app/undo.go` first.

Fix these three findings from `docs/cold-start-audit.md`:

### Finding 1: `undo --help` has non-standard section ordering

The `undoCmd.Long` field manually embeds Arguments, Flags, and Examples text
blocks. This causes cobra's auto-generated `Usage:` line to appear AFTER the
Examples section in the Long field, making the help page non-standard.

**Fix:** Restructure the command definition:
1. Keep only the Arguments section in `Long:` (remove the manually embedded
   Flags and Examples from Long).
2. Move the examples to the cobra `Example:` field on the command struct.
3. Remove the duplicate Flags section from Long (cobra auto-generates it).

After the fix, the Long field should contain only:
```
Restore previously removed packages from a snapshot.

Snapshots are automatically created before package removal operations
and can be used to rollback changes.

Arguments:
  snapshot-id  The numeric ID of the snapshot to restore
  latest       Restore the most recent snapshot
```

And add an `Example:` field to `undoCmd`:
```go
Example: `  brewprune undo --list           # List all snapshots
  brewprune undo latest           # Restore latest snapshot
  brewprune undo 42               # Restore snapshot ID 42
  brewprune undo 42 --yes         # Restore without confirmation`,
```

Cobra will then render: Long → Usage → Examples → Flags (standard order).

### Finding 2: `undo latest` exits 0 when no snapshots exist

In `runUndo`, the `[UNDO-1]` block currently:
```go
if len(snaps) == 0 {
    fmt.Println("No snapshots available.")
    fmt.Println("\nSnapshots are automatically created before package removal.")
    fmt.Println("Use 'brewprune remove' to remove packages and create snapshots.")
    return nil   // exits 0 — WRONG
}
```

**Fix:** Exit with code 1 using `os.Exit(1)` after printing the message,
similar to how doctor.go handles warning-only paths. The pattern used in the
codebase for printing to stderr and exiting non-zero without triggering the
double-print issue in main.go is to use `os.Exit`:

```go
if len(snaps) == 0 {
    fmt.Fprintln(os.Stderr, "Error: no snapshots available.")
    fmt.Fprintln(os.Stderr, "\nSnapshots are automatically created before package removal.")
    fmt.Fprintln(os.Stderr, "Use 'brewprune remove' to remove packages and create snapshots.")
    os.Exit(1)
}
```

Note: `os` is already imported in undo.go.

### Finding 3: `undo latest` message not clearly an error

This is covered by Finding 2's fix — prefixing with `"Error:"` and printing to
stderr makes it unambiguous that this is a failure state.

## 5. Tests to Write

Update `internal/app/undo_test.go`:

1. `TestRunUndo_LatestNoSnapshots_ExitsNonZero` — verify that `undo latest`
   with no snapshots produces output containing "Error:" and exits non-zero.
   Note: since os.Exit(1) is called, you may need to use a subprocess test
   pattern or check that the function calls `os.Exit`. Look at how
   `TestRunDoctor_WarningOnlyExitsCode2` handles `os.Exit(2)` in doctor_test.go
   for the pattern to follow.
2. `TestUndoHelp_UsageComesBeforeExamples` — verify that in the help output,
   "Usage:" appears before "Examples:" (both strings should be present, and
   Usage index < Examples index).
3. Update `TestRunUndo_LatestNoSnapshotsFriendlyMessage` — this existing test
   checks the friendly message; update it to expect "Error:" prefix and stderr
   output.

## 6. Verification Gate

```
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/app/... -run "TestUndo" -v
go test ./...
```

All must pass before reporting completion.

## 7. Constraints

- Do NOT change the `--list` or `--yes` flag definitions — only the Long/Example text.
- The `os.Exit(1)` approach for the no-snapshots case ensures the error is
  surfaced without the double-print issue from main.go's error handler.
- If you discover that correct implementation requires changing a file not in
  your ownership list, do NOT modify it. Report it in section 8.

## 8. Report

Append your completion report to `docs/IMPL-ux-audit-round2.md` under
`### Agent C — Completion Report`.

Include:
- What you implemented (function names, key decisions)
- Test results (pass/fail, count)
- Any deviations from the spec and why
- Any out-of-scope dependencies discovered (file name, required change, reason)
