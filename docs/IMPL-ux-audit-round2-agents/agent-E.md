# Wave 1 Agent E: quickstart.go — brew services message, PATH note, progress

You are Wave 1 Agent E. Fix three UX issues in quickstart.go.

## 1. File Ownership

You own these files. Do not touch any other files.
- `internal/app/quickstart.go` — modify

Note: There is no `quickstart_test.go` file. Do not create one — the scope is
limited to the existing file.

## 2. Interfaces You Must Implement

No new exported functions. You are modifying existing behavior only.

## 3. Interfaces You May Call

```go
// internal/output/progress.go — already exists, need to add import
output.NewSpinner(message string) *output.Spinner
(*output.Spinner).StopWithMessage(message string)
(*output.Spinner).Stop()
```

## 4. What to Implement

Read `internal/app/quickstart.go` first. Then read `internal/output/progress.go`
to understand the Spinner API.

Fix these three findings from `docs/cold-start-audit.md`:

### Finding 1: `quickstart` attempts `brew services start` on Linux with confusing failure

Step 3 of quickstart:
1. Finds `brew` in PATH
2. Runs `brew services start brewprune`
3. If it fails (it always will on Linux), prints:
   `⚠ brew services start failed (exit status 1) — falling back to brewprune watch --daemon`
4. Falls back to `watch --daemon`

A new user sees a failure message during setup, which is alarming.

**Fix:** Before running `brew services`, detect whether the system supports it.
On Linux, `brew services` requires `systemd` or `launchd` — check for
`/sbin/init` being systemd or use `runtime.GOOS`:

```go
import "runtime"

// In Step 3:
if brewPath, lookErr := exec.LookPath("brew"); lookErr == nil {
    if runtime.GOOS == "linux" {
        // brew services is not reliable on Linux; skip directly to daemon
        fmt.Println("  brew found but using daemon mode (brew services not supported on Linux)")
        // start daemon directly
    } else {
        // macOS: try brew services first
        fmt.Printf("  brew found at %s — running: brew services start brewprune\n", brewPath)
        // ... existing brew services logic ...
    }
}
```

If `brew services` is tried and fails on macOS, change the message from
`"⚠ brew services start failed (%v) — falling back to brewprune watch --daemon"`
to something less alarming:
`"  brew services unavailable — using daemon mode"`

### Finding 2: `quickstart` PATH step says "Restart your shell" but continues immediately

Step 2 writes the PATH export and prints "Restart your shell (or source the
config file) for this to take effect." Then step 3 starts the daemon
immediately. This is fine (the daemon doesn't need PATH), but the completion
message doesn't explain why `doctor` still warns about PATH.

**Fix:** Update the completion message at the end of `runQuickstart` to add a
note explaining the PATH state:

After:
```go
fmt.Println("Check status anytime: brewprune status")
fmt.Println("Run diagnostics:      brewprune doctor")
```

Add:
```go
fmt.Println()
fmt.Println("Note: If doctor reports 'PATH missing', restart your shell or run:")
fmt.Println("  source ~/.profile  (or ~/.zshrc / ~/.bashrc depending on your shell)")
```

### Finding 3: Self-test waits 35 seconds with no progress indicator

Step 4 prints "Waiting up to 35s..." and then blocks silently.

**Fix:** Import `"github.com/blackwell-systems/brewprune/internal/output"` (not
currently imported) and wrap the RunShimTest call with a spinner:

```go
// Step 4: Self-test
fmt.Println("Step 4/4: Running self-test (tracking verified)")
dbPath, dbErr := getDBPath()
if dbErr != nil {
    fmt.Printf("  ⚠ Could not get database path: %v\n", dbErr)
    fmt.Println("  Run 'brewprune doctor' for diagnostics")
} else {
    db, openErr := store.New(dbPath)
    if openErr != nil {
        fmt.Printf("  ⚠ Could not open database: %v\n", openErr)
        fmt.Println("  Run 'brewprune doctor' for diagnostics")
    } else {
        defer db.Close()
        spinner := output.NewSpinner("Verifying shim → daemon → database pipeline (up to 35s)...")
        testErr := RunShimTest(db, 35*time.Second)
        if testErr != nil {
            spinner.StopWithMessage(fmt.Sprintf("  ⚠ Self-test did not confirm tracking: %v", testErr))
            fmt.Println("  Run 'brewprune doctor' for diagnostics")
        } else {
            spinner.StopWithMessage("  ✓ Tracking verified — brewprune is working")
        }
    }
}
```

Remove the original `fmt.Println("  Waiting up to 35s for a usage event to appear in the database...")`.

## 5. Tests to Write

There is no existing quickstart_test.go. Do NOT create one — this is out of
scope for this agent. The verification gate (build + full test suite) is
sufficient.

## 6. Verification Gate

```
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./...
```

All must pass before reporting completion.

## 7. Constraints

- The `output` import is NOT currently in quickstart.go. You must add it to
  the import block.
- The `runtime` import for GOOS detection is NOT currently in quickstart.go.
  You must add it if you use the GOOS approach.
- Do NOT change `internal/app/shimtest.go` — only quickstart.go.
- Do NOT change any other files.
- If you discover that correct implementation requires changing a file not in
  your ownership list, do NOT modify it. Report it in section 8.

## 8. Report

Append your completion report to `docs/IMPL-ux-audit-round2.md` under
`### Agent E — Completion Report`.

Include:
- What you implemented (function names, key decisions)
- Test results (pass/fail, count)
- Any deviations from the spec and why
- Any out-of-scope dependencies discovered (file name, required change, reason)
