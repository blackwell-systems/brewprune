# Wave 1 Agent F: scan.go — suppress stale daemon warning when daemon running

You are Wave 1 Agent F. Fix one UX issue in scan.go.

## 1. File Ownership

You own these files. Do not touch any other files.
- `internal/app/scan.go` — modify
- `internal/app/scan_test.go` — modify

## 2. Interfaces You Must Implement

No new exported functions. You are modifying existing behavior only.

## 3. Interfaces You May Call

```go
// internal/watcher — already imported in other files
watcher.IsDaemonRunning(pidFile string) (bool, error)

// internal/app/common.go or root.go — already exists
getDefaultPIDFile() (string, error)
```

Check whether `watcher` is already imported in scan.go. If not, you will need
to add the import.

## 4. What to Implement

Read `internal/app/scan.go` first.

Fix this finding from `docs/cold-start-audit.md`:

### Finding: `scan` shows stale "start watch daemon" warning when daemon is running

In `runScan`, near the end of the function, when `shimCount == 0` the code
unconditionally prints:
```go
fmt.Println("\n⚠ NEXT STEP: Start usage tracking with 'brewprune watch --daemon'")
fmt.Println("   Wait 1-2 weeks for meaningful recommendations.")
```

And even when `shimCount > 0` and PATH is OK, the same message appears.

After a user runs `quickstart` (which starts the daemon), running `scan` again
still shows the daemon start warning even though the daemon is already running.

**Fix:** Before printing the "start daemon" warning, check if the daemon is
already running. If it is, print a confirmation instead:

```go
// Show next-step guidance based on daemon state
pidFile, pidErr := getDefaultPIDFile()
daemonAlreadyRunning := false
if pidErr == nil {
    if running, runErr := watcher.IsDaemonRunning(pidFile); runErr == nil && running {
        daemonAlreadyRunning = true
    }
}

if shimCount > 0 {
    if ok, reason := shim.IsShimSetup(); !ok {
        fmt.Printf("\n⚠ Usage tracking requires one more step:\n  %s\n", reason)
        fmt.Println("  Then restart your shell and run: brewprune watch --daemon")
    } else if daemonAlreadyRunning {
        fmt.Println("\n✓ Daemon is running — usage tracking is active.")
    } else {
        fmt.Println("\n⚠ NEXT STEP: Start usage tracking with 'brewprune watch --daemon'")
        fmt.Println("   Wait 1-2 weeks for meaningful recommendations.")
    }
} else {
    if daemonAlreadyRunning {
        fmt.Println("\n✓ Daemon is running — usage tracking is active.")
    } else {
        fmt.Println("\n⚠ NEXT STEP: Start usage tracking with 'brewprune watch --daemon'")
        fmt.Println("   Wait 1-2 weeks for meaningful recommendations.")
    }
}
```

Look at the actual current code structure in scan.go carefully — the shimCount
check and the PATH check are nested. Make sure your changes preserve the
existing PATH-missing message path.

Also check `internal/app/watch.go` to verify the import path for `watcher`:
it's `"github.com/blackwell-systems/brewprune/internal/watcher"`.

## 5. Tests to Write

Update `internal/app/scan_test.go`:

1. `TestRunScan_DaemonRunning_SuppressesWarning` — verify that when the daemon
   is running, the scan output shows "✓ Daemon is running" instead of the
   "NEXT STEP: Start usage tracking" warning. Since actually starting the daemon
   in a unit test is complex, mock the PID file or check that the logic path is
   covered.

   Look at the existing test structure in scan_test.go for patterns on how to
   test command behavior without actually running brew/scan.

## 6. Verification Gate

```
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/app/... -run "TestScan" -v
go test ./...
```

All must pass before reporting completion.

## 7. Constraints

- The `watcher` package import must be added to scan.go if not already present.
  Check the existing imports.
- Preserve the existing `--quiet` flag behavior: when `scanQuiet` is true,
  none of the next-step messages should be printed.
- Do NOT change the scan logic itself — only the post-scan messaging.
- If you discover that correct implementation requires changing a file not in
  your ownership list, do NOT modify it. Report it in section 8.

## 8. Report

Append your completion report to `docs/IMPL-ux-audit-round2.md` under
`### Agent F — Completion Report`.

Include:
- What you implemented (function names, key decisions)
- Test results (pass/fail, count)
- Any deviations from the spec and why
- Any out-of-scope dependencies discovered (file name, required change, reason)
