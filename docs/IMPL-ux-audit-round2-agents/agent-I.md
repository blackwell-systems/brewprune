# Wave 1 Agent I: stats.go — hide zero-usage by default + explain pointer

You are Wave 1 Agent I. Fix two UX issues in stats.go.

## 1. File Ownership

You own these files. Do not touch any other files.
- `internal/app/stats.go` — modify
- `internal/app/stats_test.go` — modify

## 2. Interfaces You Must Implement

No new exported functions. You are modifying existing behavior only.

## 3. Interfaces You May Call

Existing `output.RenderUsageTable` and standard library — no new imports needed.

## 4. What to Implement

Read `internal/app/stats.go` first.

Fix these two findings from `docs/cold-start-audit.md`:

### Finding 1: `stats` default output shows all 40 packages including 0-usage

`brewprune stats` outputs all packages sorted by usage count. With 40 packages
and only 1 having any usage, this is 39 rows of "0 runs / never". The single
meaningful row (git: 1 use) is buried.

**Fix:** In `showUsageTrends`, add an `--all` flag to the `stats` command, and
by default only show packages with recorded usage. Add a summary footer showing
how many packages were hidden:

1. Add a new flag `statsAll bool` and register it:
```go
var statsAll bool
// in init():
statsCmd.Flags().BoolVar(&statsAll, "all", false, "Show all packages including those with no usage")
```

2. In `showUsageTrends`, after building `outputStats`, filter before rendering:
```go
var filteredStats map[string]output.UsageStats
hiddenCount := 0
if !statsAll {
    filteredStats = make(map[string]output.UsageStats)
    for pkg, s := range outputStats {
        if s.TotalRuns > 0 {
            filteredStats[pkg] = s
        } else {
            hiddenCount++
        }
    }
} else {
    filteredStats = outputStats
}

if len(filteredStats) == 0 {
    if hiddenCount > 0 {
        fmt.Printf("No usage recorded yet (%d packages with 0 runs). Run 'brewprune watch --daemon' to start tracking.\n", hiddenCount)
    } else {
        fmt.Println("No usage data found. Run 'brewprune watch' to collect usage data.")
    }
    return nil
}

table := output.RenderUsageTable(filteredStats)
fmt.Print(table)

fmt.Printf("\nSummary: %d packages used in last %d days (out of %d total)\n",
    usedCount, days, len(trends))
if hiddenCount > 0 && !statsAll {
    fmt.Printf("(%d packages with no recorded usage hidden — use --all to show)\n", hiddenCount)
}
```

### Finding 2: `stats --package` for never-used package gives no actionable info

`showPackageStats` for a package with 0 usage just shows zeros with no pointer
to `explain` for removal advice.

**Fix:** At the end of `showPackageStats`, after printing all stats, add:
```go
if stats.TotalUses == 0 {
    fmt.Println()
    fmt.Printf("Tip: Run 'brewprune explain %s' for removal recommendation.\n", pkg)
}
```

## 5. Tests to Write

Update `internal/app/stats_test.go`:

1. `TestShowUsageTrends_HidesZeroUsageByDefault` — verify that with `statsAll =
   false` (default), packages with 0 TotalRuns are not shown in the table, and
   the output contains "hidden" to indicate suppressed packages.
2. `TestShowUsageTrends_ShowAllFlag` — verify that with `statsAll = true`, all
   packages appear including those with 0 usage.
3. `TestShowPackageStats_ZeroUsage_ShowsExplainHint` — verify that when a
   package has 0 TotalUses, the output contains "brewprune explain".
4. Update `TestStatsCommand_Flags` to include the new `--all` flag.
5. Update `TestStatsCommand_EmptyDatabase` if needed to match new empty state
   messaging.

## 6. Verification Gate

```
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/app/... -run "TestStats|TestShowUsage|TestShowPackage" -v
go test ./...
```

All must pass before reporting completion.

## 7. Constraints

- The `--all` flag on `stats` is similar to `--all` on `unused`. Keep naming
  consistent.
- The `statsAll` variable must be a package-level var in stats.go (following
  the pattern of `statsDays` and `statsPackage`).
- The existing `TestStatsCommand_EmptyDatabase` test verifies "No usage data
  found" message — update it if the message changes for the zero-usage case.
- If you discover that correct implementation requires changing a file not in
  your ownership list, do NOT modify it. Report it in section 8.

## 8. Report

Append your completion report to `docs/IMPL-ux-audit-round2.md` under
`### Agent I — Completion Report`.

Include:
- What you implemented (function names, key decisions)
- Test results (pass/fail, count)
- Any deviations from the spec and why
- Any out-of-scope dependencies discovered (file name, required change, reason)
