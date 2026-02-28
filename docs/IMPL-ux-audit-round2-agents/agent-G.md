# Wave 1 Agent G: unused.go — stable secondary sort for --sort age + --min-score help

You are Wave 1 Agent G. Fix two UX issues in unused.go.

## 1. File Ownership

You own these files. Do not touch any other files.
- `internal/app/unused.go` — modify
- `internal/app/unused_test.go` — modify

## 2. Interfaces You Must Implement

No new exported functions. You are modifying existing behavior only.

## 3. Interfaces You May Call

Existing `sort`, `analyzer`, and `output` functions — no new imports needed.

## 4. What to Implement

Read `internal/app/unused.go` first.

Fix these two findings from `docs/cold-start-audit.md`:

### Finding 1: `--sort age` produces no visible reordering relative to default

The `sortScores` function has an `"age"` case:
```go
case "age":
    sort.Slice(scores, func(i, j int) bool {
        return scores[i].InstalledAt.Before(scores[j].InstalledAt) // Oldest first
    })
```

When packages have identical `InstalledAt` times (e.g., all installed in the
same container), `sort.Slice` produces an arbitrary, unstable order. This
makes the output look scrambled with no tier grouping.

**Fix:** Use `sort.SliceStable` and add secondary sort keys to break ties:
1. Primary: `InstalledAt` ascending (oldest first)
2. Secondary: `Tier` alphabetically (safe < medium < risky for grouping)
3. Tertiary: `Package` name alphabetically

```go
case "age":
    sort.SliceStable(scores, func(i, j int) bool {
        if !scores[i].InstalledAt.Equal(scores[j].InstalledAt) {
            return scores[i].InstalledAt.Before(scores[j].InstalledAt)
        }
        // Secondary: tier order (safe → medium → risky)
        tierOrder := map[string]int{"safe": 0, "medium": 1, "risky": 2}
        ti := tierOrder[scores[i].Tier]
        tj := tierOrder[scores[j].Tier]
        if ti != tj {
            return ti < tj
        }
        // Tertiary: alphabetical
        return scores[i].Package < scores[j].Package
    })
```

Also use `sort.SliceStable` for the other sort cases to prevent arbitrary
ordering when scores are equal:

For `"score"`:
```go
case "score":
    sort.SliceStable(scores, func(i, j int) bool {
        if scores[i].Score != scores[j].Score {
            return scores[i].Score > scores[j].Score
        }
        return scores[i].Package < scores[j].Package  // stable alpha fallback
    })
```

For `"size"`:
```go
case "size":
    sort.SliceStable(scores, func(i, j int) bool {
        if scores[i].SizeBytes != scores[j].SizeBytes {
            return scores[i].SizeBytes > scores[j].SizeBytes
        }
        return scores[i].Package < scores[j].Package  // stable alpha fallback
    })
```

### Finding 2: `--min-score` flag help text doesn't explain where scores come from

The current flag description is:
```go
unusedCmd.Flags().IntVar(&unusedMinScore, "min-score", 0, "Minimum confidence score (0-100)")
```

**Fix:** Update the description to point users to `explain`:
```go
unusedCmd.Flags().IntVar(&unusedMinScore, "min-score", 0, "Minimum confidence score (0-100). Use 'brewprune explain <package>' to see a package's score.")
```

## 5. Tests to Write

Update `internal/app/unused_test.go`:

1. `TestSortScores_AgeWithTieBreak` — create a slice with packages that all
   have identical `InstalledAt` times, call `sortScores(scores, "age")`, and
   verify the result is sorted alphabetically (the tertiary sort key).
2. `TestSortScores_ScoreWithTieBreak` — create a slice with packages that have
   identical scores, verify alphabetical secondary sort.
3. `TestMinScoreFlagDescription` — verify `unusedCmd.Flag("min-score").Usage`
   contains "explain".

## 6. Verification Gate

```
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/app/... -run "TestSort|TestMinScore|TestUnused" -v
go test ./...
```

All must pass before reporting completion.

## 7. Constraints

- Only change `sortScores` and the flag description. Do not change any other
  logic in `runUnused`.
- The `analyzer.ConfidenceScore` struct has `InstalledAt time.Time` and
  `Package string` — both are safe to access.
- If you discover that correct implementation requires changing a file not in
  your ownership list, do NOT modify it. Report it in section 8.

## 8. Report

Append your completion report to `docs/IMPL-ux-audit-round2.md` under
`### Agent G — Completion Report`.

Include:
- What you implemented (function names, key decisions)
- Test results (pass/fail, count)
- Any deviations from the spec and why
- Any out-of-scope dependencies discovered (file name, required change, reason)
