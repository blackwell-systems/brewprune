# Wave 1 Agent B: doctor.go — Fix: labels → Action: + pipeline test progress

You are Wave 1 Agent B. Fix two UX issues in doctor.go.

## 1. File Ownership

You own these files. Do not touch any other files.
- `internal/app/doctor.go` — modify
- `internal/app/doctor_test.go` — modify

## 2. Interfaces You Must Implement

No new exported functions. You are modifying existing behavior only.

## 3. Interfaces You May Call

```go
// internal/output/progress.go — existing, already used elsewhere
output.NewSpinner(message string) *output.Spinner
(*output.Spinner).StopWithMessage(message string)
(*output.Spinner).Stop()
```

## 4. What to Implement

Read `internal/app/doctor.go` first. Then read `internal/output/progress.go`
to understand the Spinner API.

Fix these two findings from `docs/cold-start-audit.md`:

### Finding: "Fix:" label in doctor output implies a `--fix` flag

In `doctor.go`, several places print `"  Fix: ..."` which users read as
implying a `--fix` flag. Running `brewprune doctor --fix` returns
`Error: unknown flag: --fix`.

**Fix:** Rename every occurrence of `"  Fix: "` in the fmt.Print/Println calls
within `runDoctor` to `"  Action: "`. This includes:
- `"  Fix: Run 'brewprune scan' to create database"`
- `"  Fix: Run 'brewprune scan'"`
- `"  Fix: Run 'brewprune watch --daemon'"`
- `"  Fix: Run 'brewprune watch --daemon'"` (stale PID)
- `"  Fix: Run 'brewprune scan' to build it"`
- The path-related fix line `"  Fix: %s\n"` where reason is the PATH setup instructions

Search for ALL occurrences of `Fix:` in doctor.go and rename them to `Action:`.
There should be ~6 occurrences. Use grep or read the full file to find them all.

Also fix the check 8 failure message which currently says:
`"  Fix: Run 'brewprune scan' to rebuild shims and restart the daemon"`

Rename that to `"  Action: Run 'brewprune scan' to rebuild shims and restart the daemon"`.

### Finding: Pipeline test runs 15-20 seconds with no progress indicator

Check 8 (pipeline test) in `runDoctor` calls `RunShimTest(db2, 35*time.Second)`
which blocks silently for up to 35 seconds.

**Fix:** Import `"github.com/blackwell-systems/brewprune/internal/output"` at
the top of doctor.go (it's not currently imported). Then wrap the pipeline test
with a spinner:

```go
// Check 8: End-to-end pipeline test (only when no critical issues)
if criticalIssues == 0 {
    pipelineStart := time.Now()
    db2, dbErr := store.New(resolvedDBPath)
    if dbErr != nil {
        fmt.Println("✗ Pipeline test: cannot open database:", dbErr)
        criticalIssues++
    } else {
        defer db2.Close()
        spinner := output.NewSpinner("Running pipeline test...")
        pipelineErr := RunShimTest(db2, 35*time.Second)
        pipelineElapsed := time.Since(pipelineStart).Round(time.Millisecond)
        if pipelineErr != nil {
            spinner.StopWithMessage(fmt.Sprintf("✗ Pipeline test: fail (%v)", pipelineElapsed))
            fmt.Printf("  %v\n", pipelineErr)
            fmt.Println("  Action: Run 'brewprune scan' to rebuild shims and restart the daemon")
            criticalIssues++
        } else {
            spinner.StopWithMessage(fmt.Sprintf("✓ Pipeline test: pass (%v)", pipelineElapsed))
        }
    }
}
```

Note: The spinner uses `os.Stdout` and detects TTY automatically. When running
via `docker exec` (non-TTY), it prints `"Running pipeline test...\n"` once and
doesn't animate — that's correct behavior.

## 5. Tests to Write

Update `internal/app/doctor_test.go`:

1. `TestRunDoctor_ActionLabelNotFix` — verify that the string `"Fix:"` does NOT
   appear in the doctor output (check that the rename was applied). Read the
   existing tests to understand how the command is invoked in test context.
2. `TestRunDoctor_WarningOnlyExitsCode2` — this existing test should still pass
   after the rename. Review it and ensure it still works.
3. `TestRunDoctor_PipelineTestShowsProgress` — verify that when doctor runs,
   the output contains "Running pipeline test..." (or some progress indication)
   before the result line. This may require capturing stdout during the test.

Note: Integration tests for doctor (that actually run the pipeline test) may
time out or fail in test environments. Keep tests that mock/skip the pipeline
step, or add a short-circuit via a test timeout.

## 6. Verification Gate

```
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/app/... -run "TestRunDoctor" -v
go test ./...
```

All must pass before reporting completion.

## 7. Constraints

- Do NOT add a `--fix` flag to doctor — that is a separate feature not in scope.
- The rename from `Fix:` to `Action:` applies ONLY to the output strings in
  `runDoctor`, not to any comment text or variable names.
- The spinner import (`internal/output`) is not currently in doctor.go. You
  must add it.
- If you discover that correct implementation requires changing a file not in
  your ownership list, do NOT modify it. Report it in section 8.

## 8. Report

Append your completion report to `docs/IMPL-ux-audit-round2.md` under
`### Agent B — Completion Report`.

Include:
- What you implemented (function names, key decisions)
- Test results (pass/fail, count)
- Any deviations from the spec and why
- Any out-of-scope dependencies discovered (file name, required change, reason)
