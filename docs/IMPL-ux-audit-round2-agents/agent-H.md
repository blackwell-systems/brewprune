# Wave 1 Agent H: status.go — synthetic event explanation

You are Wave 1 Agent H. Fix one UX issue in status.go.

## 1. File Ownership

You own these files. Do not touch any other files.
- `internal/app/status.go` — modify
- `internal/app/status_test.go` — modify

## 2. Interfaces You Must Implement

No new exported functions. You are modifying existing behavior only.

## 3. Interfaces You May Call

Existing standard library and store functions — no new imports.

## 4. What to Implement

Read `internal/app/status.go` first.

Fix this finding from `docs/cold-start-audit.md`:

### Finding: `status` shows "PATH missing" + "COLLECTING (0 of 14 days)" contradiction

After `quickstart`, `brewprune status` shows:
```
Shims:        active · 222 commands · PATH missing ⚠
Data quality: COLLECTING (0 of 14 days)
```
But the daemon is recording events (1 total from the self-test). A new user is
confused: if PATH is missing, how is tracking working?

The answer is: the 1 event is from the quickstart self-test which injected a
synthetic event. Real shim-based tracking doesn't work until PATH is fixed.

**Fix:** In `runStatus`, after printing the "Shims:" line, detect the case
where PATH is missing but totalEvents > 0, and add an explanatory note:

Currently the shim line is printed as:
```go
fmt.Printf(label+"%s · %d commands · %s\n", "Shims:", shimStatus, shimCount, pathStatus)
```

After this line, check if PATH is missing but there are events:
```go
if !pathOK && totalEvents > 0 {
    fmt.Printf("              %s\n", "Note: events are from setup self-test, not real shim interception.")
    fmt.Printf("              %s\n", "Real tracking starts when PATH is fixed and shims are in front of Homebrew.")
}
```

This gives users the context they need to understand the contradiction without
changing the data shown.

The label format string `"%-14s"` is defined as `const label = "%-14s"` in the
function. Match the indentation of the note lines to align with the output.

## 5. Tests to Write

Update `internal/app/status_test.go`:

1. `TestRunStatus_PathMissingWithEvents_ShowsNote` — verify that when shim PATH
   is missing and there are usage events, the output contains "setup self-test"
   or equivalent explanatory text. Since directly setting PATH state in a test
   is complex, test via the output string: construct a test scenario where
   `isOnPATH` returns false and `totalEvents > 0` (may require refactoring the
   function slightly or testing via subprocess). At minimum, document what the
   test should verify.

   If the test is difficult to write without mocking, write a simpler test that
   verifies the status command at least runs and doesn't crash (the existing
   TestRunStatus_DaemonStoppedSuggestsWatchDaemon pattern).

## 6. Verification Gate

```
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/app/... -run "TestRunStatus" -v
go test ./...
```

All must pass before reporting completion.

## 7. Constraints

- Only add the explanatory note — do not change any existing output lines.
- The note must only appear when `!pathOK && totalEvents > 0`.
- If you discover that correct implementation requires changing a file not in
  your ownership list, do NOT modify it. Report it in section 8.

## 8. Report

Append your completion report to `docs/IMPL-ux-audit-round2.md` under
`### Agent H — Completion Report`.

Include:
- What you implemented (function names, key decisions)
- Test results (pass/fail, count)
- Any deviations from the spec and why
- Any out-of-scope dependencies discovered (file name, required change, reason)
