# Wave 1 Agent K: table.go — add Score column + rename ✗ keep → ⚠ risky

You are Wave 1 Agent K. Fix two UX issues in output/table.go.

## 1. File Ownership

You own these files. Do not touch any other files.
- `internal/output/table.go` — modify
- `internal/output/table_test.go` — modify

## 2. Interfaces You Must Implement

No new exported functions. The existing `RenderConfidenceTable` signature does
not change. You are only modifying the rendered output.

```go
// No change to this signature:
func RenderConfidenceTable(scores []ConfidenceScore) string
```

The `ConfidenceScore` struct already has `Score int` — you are just rendering it.

## 3. Interfaces You May Call

Existing functions in table.go — no new imports.

## 4. What to Implement

Read `internal/output/table.go` first. The key function is
`RenderConfidenceTable` at line ~93.

Fix these two findings from `docs/cold-start-audit.md`:

### Finding 1: Score column is absent from the `unused` table

The `unused` table columns are: Package, Size, Uses (7d), Last Used, Depended
On, Status. There is no numeric score column, yet `--min-score` is a supported
flag and the entire system is score-based.

**Fix:** Add a "Score" column between "Size" and "Uses (7d)".

Current header:
```go
sb.WriteString(fmt.Sprintf("%-16s %-8s %-10s %-16s %-13s %s\n",
    "Package", "Size", "Uses (7d)", "Last Used", "Depended On", "Status"))
sb.WriteString(strings.Repeat("─", 80))
```

New header (adding %-7s for "Score"):
```go
sb.WriteString(fmt.Sprintf("%-16s %-8s %-7s %-10s %-16s %-13s %s\n",
    "Package", "Size", "Score", "Uses (7d)", "Last Used", "Depended On", "Status"))
sb.WriteString(strings.Repeat("─", 88))
```

The Score format: display as `"80/100"` (6 chars). Use `%-7s` for the column.

Current row format:
```go
sb.WriteString(fmt.Sprintf("%-16s %-8s %-10s %-16s %-13s %s%s%s\n",
    truncate(score.Package, 16),
    size,
    usesStr,
    lastUsed,
    depStr,
    tierColor,
    tierLabel,
    colorReset))
```

New row format (color-coded score):
```go
scoreStr := fmt.Sprintf("%d/100", score.Score)
// In the format string, add scoreStr after size:
sb.WriteString(fmt.Sprintf("%-16s %-8s %-7s %-10s %-16s %-13s %s%s%s\n",
    truncate(score.Package, 16),
    size,
    scoreStr,
    usesStr,
    lastUsed,
    depStr,
    tierColor,
    tierLabel,
    colorReset))
```

Update BOTH the color and non-color branches of the row rendering.
The separator line width must match the new total column width (88 dashes).

### Finding 2: `--tier risky` shows packages with "✗ keep" — tier label contradicts intent

The `formatTierLabel` function returns `"✗ keep"` for risky packages:
```go
default: // risky or critical
    return "✗ keep"
```

When a user runs `--tier risky` (explicitly asking to see risky packages to
evaluate removal at their own risk), every row shows `"✗ keep"` — which implies
"don't remove any of these", directly contradicting why the user filtered to
risky in the first place.

The `"✗ keep"` label made sense in the mixed default view (risky packages are
shown with `--all` as "keep these"). But the label is not context-aware.

**Fix:** Rename `"✗ keep"` to `"⚠ risky"` for all risky/critical packages.
This communicates the tier without implying a mandatory action:

```go
func formatTierLabel(tier string, isCritical bool) string {
    switch strings.ToLower(tier) {
    case "safe":
        return "✓ safe"
    case "medium":
        return "~ review"
    default: // risky or critical
        return "⚠ risky"
    }
}
```

This is consistent with `✓ safe` (positive) and `~ review` (neutral).

**Note:** Any tests that assert the string `"✗ keep"` must be updated to
`"⚠ risky"`.

## 5. Tests to Write

Update `internal/output/table_test.go`:

1. `TestRenderConfidenceTable_ScoreColumnPresent` — verify that the header
   contains "Score" and that a rendered row contains the score formatted as
   "80/100" (or similar `N/100` pattern).
2. `TestRenderConfidenceTable_RiskyLabel` — verify that a risky-tier package
   renders as `"⚠ risky"` (not `"✗ keep"`).
3. `TestFormatTierLabel_Risky` — verify `formatTierLabel("risky", false)` returns `"⚠ risky"`.
4. `TestFormatTierLabel_Critical` — verify `formatTierLabel("risky", true)` returns `"⚠ risky"`.
5. Update `TestRenderConfidenceTable` and `TestVisualConfidenceTable` — these
   existing tests likely assert on the column layout. Update them to include
   the Score column and the renamed tier label.
6. Update `TestRenderConfidenceTable_CaskDisplay` if it asserts on column
   widths or specific output format.

Also check `internal/output/example_test.go` — if it contains example output
showing `"✗ keep"` or the old column layout, update those examples too.

## 6. Verification Gate

```
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/output/... -v
go test ./...
```

All must pass before reporting completion.

## 7. Constraints

- Do NOT change the `ConfidenceScore` struct — it already has `Score int`.
- Do NOT change `RenderConfidenceTableVerbose` — that's a different function
  for the `--verbose` flag output.
- The score column uses plain `N/100` format (no color on the score number).
- The separator line (─ characters) must match the actual total column width
  exactly, or tests that check exact separator width will fail.
- `internal/output/example_test.go` may contain expected output strings that
  need updating. Read it and update as needed.
- If you discover that correct implementation requires changing a file not in
  your ownership list, do NOT modify it. Report it in section 8.

## 8. Report

Append your completion report to `docs/IMPL-ux-audit-round2.md` under
`### Agent K — Completion Report`.

Include:
- What you implemented (function names, key decisions)
- Test results (pass/fail, count)
- Any deviations from the spec and why
- Any out-of-scope dependencies discovered (file name, required change, reason)
