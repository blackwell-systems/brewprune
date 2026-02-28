# Wave 1 Agent J: explain.go — exit code, scoring direction note, table width

You are Wave 1 Agent J. Fix three UX issues in explain.go.

## 1. File Ownership

You own these files. Do not touch any other files.
- `internal/app/explain.go` — modify
- `internal/app/explain_test.go` — modify

## 2. Interfaces You Must Implement

No new exported functions. You are modifying existing behavior only.

## 3. Interfaces You May Call

Existing `os`, `fmt`, `strings`, `analyzer` — no new imports needed.

## 4. What to Implement

Read `internal/app/explain.go` first.

Fix these three findings from `docs/cold-start-audit.md`:

### Finding 1: `explain nonexistent` exits with code 0 on package-not-found error

The current code in `runExplain`:
```go
_, err = st.GetPackage(packageName)
if err != nil {
    fmt.Fprintf(os.Stderr, "Error: package not found: %s\nRun 'brewprune scan' to update package database\n", packageName)
    return nil   // exits 0 — WRONG
}
```

The comment says `[EXPLAIN-1]` "return nil so main.go's error handler is never
reached, guaranteeing exactly one print." But returning nil means exit 0, which
is wrong for an error condition.

**Fix:** Use `os.Exit(1)` after printing the error (same pattern as other
commands in the codebase that need non-zero exit without double-print):
```go
if err != nil {
    fmt.Fprintf(os.Stderr, "Error: package not found: %s\nRun 'brewprune scan' to update package database\n", packageName)
    os.Exit(1)
}
```

### Finding 2: `explain git` shows "0/40 pts — used today" which contradicts itself

In `renderExplanation`, the table shows:
```
│ Usage               │  0/40   │ used today                           │
```

A score of 0/40 looks like "bad" but means "actively used, keep it." The
scoring system uses low scores for active packages to indicate they should NOT
be removed. New users read "0/40 pts" as poor/low.

**Fix 1:** Change the column header from "Points" to "Removal pts":
```go
fmt.Println("│ Component           │ Removal │ Detail                               │")
fmt.Println("│                     │  pts    │                                      │")
```

Wait — looking at the actual header:
```go
fmt.Println("│ Component           │ Points  │ Detail                               │")
```

Change "Points" to "Removal pts" doesn't fit in 7 chars. Use "Score" instead:
```go
fmt.Println("│ Component           │  Score  │ Detail                               │")
```

**Fix 2:** Add a note after the table explaining the scoring direction. After
the closing line of the breakdown table, before the "Why TIER:" section, add:

```go
fmt.Println()
fmt.Println("Note: Higher removal score = more confident to remove.")
fmt.Println("      Usage: 0/40 means recently used (lower = keep this package).")
```

### Finding 3: `explain` verbose table truncates "Detail" column with "..." at 38 chars

The `truncateDetail` function truncates at 36 characters:
```go
score.UsageScore, truncateDetail(score.Explanation.UsageDetail, 36))
```

This is too narrow. The table width is fixed at the border characters, and the
Detail column is 36 chars wide.

**Fix:** Increase the Detail column width from 36 to 50 characters, and update
the table border lines and format strings accordingly.

Current table format:
```
┌─────────────────────┬─────────┬──────────────────────────────────────┐
│ Component           │ Points  │ Detail                               │
├─────────────────────┼─────────┼──────────────────────────────────────┤
│ Usage               │ %2d/40   │ %-36s │\n
```
The "Detail" column uses 38 chars including the trailing space and `│`.

New format (50-char detail column):
```
┌─────────────────────┬─────────┬────────────────────────────────────────────────────┐
│ Component           │  Score  │ Detail                                             │
├─────────────────────┼─────────┼────────────────────────────────────────────────────┤
│ Usage               │ %2d/40   │ %-50s │\n
```

Count the dashes: Component column = 21 chars, Score/Points column = 9 chars,
Detail column = 52 chars (50 content + 2 spaces). Adjust ALL border lines to
match. Also update the Total row format string.

The `truncateDetail` call site max width changes from 36 → 50.
The `truncateDetail` function itself is fine — just pass 50.

## 5. Tests to Write

Update `internal/app/explain_test.go`:

1. `TestRunExplain_NotFound_ExitsNonZero` — verify that `explain nonexistent`
   exits non-zero. Since `os.Exit(1)` is called, use the subprocess test
   pattern: exec the test binary with `-test.run=TestRunExplain_NotFound_ExitsNonZero`
   as a subprocess and check its exit code. Look at how
   `TestRunDoctor_WarningOnlyExitsCode2` in `doctor_test.go` handles os.Exit
   testing for the pattern.
2. `TestRenderExplanation_ScoringNote` — call `renderExplanation` with a
   test score and verify the output contains "Note:" and "recently used" (or
   the equivalent text from the note you add).
3. `TestRenderExplanation_DetailNotTruncated` — verify that a detail string of
   40 characters renders without "..." in the output (was truncated before,
   should not be now).
4. Update `TestRunExplain_NotFoundPrintedOnce` — this existing test currently
   checks that the not-found message is printed exactly once (without double-
   print). It should still pass after the os.Exit change, but you may need to
   use a subprocess approach to check the exit code too.

## 6. Verification Gate

```
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/app/... -run "TestRunExplain|TestRenderExplain" -v
go test ./...
```

All must pass before reporting completion.

## 7. Constraints

- `os.Exit(1)` for not-found is the right fix. The comment `[EXPLAIN-1]`
  says "return nil so main.go's error handler is never reached, guaranteeing
  exactly one print" — this was a previous attempt to avoid double-print.
  `os.Exit(1)` also avoids double-print AND sets the correct exit code.
- The table width change must be consistent: all border lines, format strings,
  and `truncateDetail` max widths must use 50.
- Do NOT change `truncateDetail` function behavior — only the call sites.
- If you discover that correct implementation requires changing a file not in
  your ownership list, do NOT modify it. Report it in section 8.

## 8. Report

Append your completion report to `docs/IMPL-ux-audit-round2.md` under
`### Agent J — Completion Report`.

Include:
- What you implemented (function names, key decisions)
- Test results (pass/fail, count)
- Any deviations from the spec and why
- Any out-of-scope dependencies discovered (file name, required change, reason)
