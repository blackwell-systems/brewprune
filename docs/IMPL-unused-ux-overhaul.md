# IMPL: Unused UX Overhaul

## Feature Description

Overhaul the `brewprune unused` rendering to make output actionable:
1. Hide risky-tier packages by default (add `--all` flag)
2. Add tier summary header with per-tier counts and sizes
3. Add reclaimable space footer
4. Show `n/a` for cask usage columns (casks can't be tracked via shims)
5. Show `—` instead of `0 packages` for zero deps
6. Pass `IsCask` through the rendering pipeline

### Pre-completed work (do NOT touch these files)

- `internal/scanner/dependencies.go` — `brewprune` added to `coreDependencies`
- `internal/analyzer/types.go` — `IsCask bool` added to `ConfidenceScore`
- `internal/analyzer/confidence.go` — `IsCask` populated from `pkgInfo.IsCask`

---

### Dependency Graph

```
analyzer.ConfidenceScore.IsCask (DONE)
         |
         v
output.ConfidenceScore.IsCask ─────> output.RenderConfidenceTable()
output.TierStats ──────────────────> output.RenderTierSummary()
                                     output.RenderReclaimableFooter()
         |                                    |
         v                                    v
   app/unused.go (consumes all output funcs)
   app/remove.go (consumes RenderConfidenceTable)
```

Root nodes: `output/table.go` (new types + rendering functions)
Leaf nodes: `app/unused.go`, `app/remove.go` (consume output layer)

All rendering files are tightly coupled — a single agent owns them all.

### Interface Contracts

```go
// output/table.go — NEW types and functions

type TierStats struct {
    Count     int
    SizeBytes int64
}

func RenderTierSummary(safe, medium, risky TierStats, showAll bool) string
func RenderReclaimableFooter(safe, medium, risky TierStats, showAll bool) string

// output/table.go — MODIFIED struct (add field)
type ConfidenceScore struct {
    // ... existing fields ...
    IsCask bool // NEW: true for cask/GUI apps
}

// output/table.go — MODIFIED behavior
// formatDepCount(0) returns "—" instead of "0 packages"
// RenderConfidenceTable: IsCask rows show "n/a" for Uses (7d) and Last Used
```

### File Ownership

| File | Agent | Wave | Depends On |
|------|-------|------|------------|
| `internal/output/table.go` | A | 1 | — |
| `internal/output/table_test.go` | A | 1 | — |
| `internal/output/example_test.go` | A | 1 | — |
| `internal/app/unused.go` | A | 1 | — |
| `internal/app/remove.go` | A | 1 | — |

### Wave Structure

```
Wave 1: [A]   <- single agent, all rendering files
```

One wave, one agent. The files share too many interfaces for parallel split.

### Agent Prompts

---

# Wave 1 Agent A: Rendering UX overhaul

You are Wave 1 Agent A. You will overhaul the `brewprune unused` and `remove` command output to make it more actionable by hiding risky packages, adding tier summaries, handling casks properly, and cleaning up zero-dep display.

## 1. File Ownership

You own these files. Do not touch any other files.
- `internal/output/table.go` - modify
- `internal/output/table_test.go` - modify
- `internal/output/example_test.go` - modify
- `internal/app/unused.go` - modify
- `internal/app/remove.go` - modify

## 2. Interfaces You Must Implement

```go
// In internal/output/table.go

type TierStats struct {
    Count     int
    SizeBytes int64
}

// RenderTierSummary renders the tier breakdown header.
// Format: "SAFE: 5 packages (43 MB) · MEDIUM: 19 (186 MB) · RISKY: 143 (hidden, use --all)"
// When showAll=true, risky shows size instead of "hidden".
// Use ANSI colors: green=safe, yellow=medium, red=risky.
func RenderTierSummary(safe, medium, risky TierStats, showAll bool) string

// RenderReclaimableFooter renders the reclaimable space summary.
// Format: "Reclaimable: 43 MB (safe) · 186 MB (medium) · 4.2 GB (risky, hidden)"
// When showAll=true, risky shows without "hidden".
func RenderReclaimableFooter(safe, medium, risky TierStats, showAll bool) string
```

Add `IsCask bool` field to the existing `ConfidenceScore` struct in `table.go`.

## 3. Interfaces You May Call

Already implemented in codebase:

```go
// internal/store/queries.go
func (s *Store) GetPackage(name string) (*brew.Package, error)
func (s *Store) GetUsageEventCountSince(pkg string, since time.Time) (int, error)
func (s *Store) GetReverseDependencyCount(pkg string) (int, error)

// internal/output/table.go (existing)
func formatSize(bytes int64) string  // human-readable size
func getTierColor(tier string) string // ANSI color code
const colorReset = "\033[0m"

// internal/analyzer/types.go (already modified)
type ConfidenceScore struct {
    // ... includes IsCask bool
}
```

## 4. What to Implement

Read these files first:
- `internal/output/table.go` — current RenderConfidenceTable, formatDepCount, ConfidenceScore struct
- `internal/app/unused.go` — full runUnused function, computeSummary, showConfidenceAssessment
- `internal/app/remove.go` — displayConfidenceScores function

### Changes to `internal/output/table.go`:

1. Add `IsCask bool` to the output-layer `ConfidenceScore` struct
2. Add `TierStats` struct
3. Modify `formatDepCount(count int)`: return `"\u2014"` (em dash) when count == 0
4. Modify `RenderConfidenceTable()` row rendering:
   - If `score.IsCask`: print `"n/a"` for Uses (7d) column (instead of the integer) and `"n/a"` for Last Used (instead of relative time / "never")
   - Uses the existing `formatDepCount` which now handles zero
5. Add `RenderTierSummary()`: colored one-line tier breakdown
6. Add `RenderReclaimableFooter()`: reclaimable space per tier

### Changes to `internal/app/unused.go`:

1. Add `unusedAll bool` variable and `--all` flag in `init()`:
   ```go
   unusedCmd.Flags().BoolVar(&unusedAll, "all", false, "Show all tiers including risky")
   ```
2. After computing ALL scores (the loop at ~line 122-141), compute tier stats by looping through all scores (before any filtering):
   ```go
   var safeTier, mediumTier, riskyTier output.TierStats
   for _, s := range allScores {
       switch s.Tier {
       case "safe": safeTier.Count++; safeTier.SizeBytes += s.SizeBytes
       case "medium": mediumTier.Count++; mediumTier.SizeBytes += s.SizeBytes
       case "risky": riskyTier.Count++; riskyTier.SizeBytes += s.SizeBytes
       }
   }
   ```
3. When `unusedAll == false` AND `unusedTier == ""`: filter out risky-tier packages from the scores slice before rendering. When `unusedTier != ""` (explicitly set): do NOT filter — explicit tier flag overrides default hiding.
4. Build a `map[string]bool` from the packages list (name -> IsCask) and set `IsCask` on each `output.ConfidenceScore` during conversion.
5. Print `output.RenderTierSummary(safeTier, mediumTier, riskyTier, unusedAll || unusedTier != "")` before the table.
6. Replace the summary block (lines ~206-209: `Summary: N safe...` + `Note: Safe =...`) with `output.RenderReclaimableFooter(...)`.
7. Keep `showConfidenceAssessment()` as-is after the footer.

### Changes to `internal/app/remove.go`:

1. In `displayConfidenceScores()`: for each score, call `st.GetPackage(score.Package)` to get `IsCask`. If lookup fails, default to `false`.
2. Set `IsCask` on each `output.ConfidenceScore`.

## 5. Tests to Write

In `internal/output/table_test.go`:

1. Update existing `TestRenderConfidenceTable` subcases: `"0 packages"` assertions become `"\u2014"` (em dash)
2. TestRenderConfidenceTable_CaskDisplay - IsCask=true shows "n/a" for uses and last used columns
3. TestRenderTierSummary_ShowAll - all three tiers with sizes visible
4. TestRenderTierSummary_HideRisky - risky shows "hidden, use --all"
5. TestRenderReclaimableFooter_ShowAll - all sizes visible
6. TestRenderReclaimableFooter_HideRisky - risky shows "hidden"
7. TestFormatDepCount_Zero - returns em dash

In `internal/output/example_test.go`:
- Update `ExampleRenderConfidenceTable` output to match new dep count format

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./...
```

All must pass.

## 7. Constraints

- Do NOT touch `internal/scanner/dependencies.go`, `internal/analyzer/types.go`, or `internal/analyzer/confidence.go` — those changes are already done.
- Use `"\u2014"` (em dash, —) not a hyphen for zero deps.
- Cask `n/a` should be plain text, no color codes.
- `RenderTierSummary` and `RenderReclaimableFooter` must use the existing `formatSize()` helper.
- The `--all` flag must not interfere with `--tier`. If `--tier risky` is set, show risky regardless of `--all`.
- Non-fatal error handling: if `st.GetPackage()` fails in remove.go, default `IsCask` to false.

## 8. Report

When done, report:
- What you implemented (function names, key decisions)
- Test results (pass/fail, count)
- Any deviations from the spec and why
- Any interface contract changes

---

### Wave Execution Loop

After the wave completes:
1. Review agent output for correctness.
2. Merge agent worktree back into main branch.
3. Run full verification: `go build ./... && go vet ./... && go test ./...`
4. Fix any integration issues.
5. Update this doc: tick status checkboxes, note any contract changes.
6. Commit.

### Status

- [ ] Wave 1 Agent A - Rendering UX overhaul (table.go, unused.go, remove.go + tests)
