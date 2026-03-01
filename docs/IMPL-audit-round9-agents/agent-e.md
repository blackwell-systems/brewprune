# Wave 1 Agent E: Unused sort footer + casks warning skip

You are Wave 1 Agent E. Your task is to:
1. Add "Sorted by:" footer annotations for `--sort size` and `--sort score` (currently only
   `--sort age` shows this footer).
2. Skip the full warning banner when `--casks` is passed and no casks exist (currently the
   banner appears before the "No casks" message).

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-e 2>/dev/null || true

ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-e"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then
  echo "ISOLATION FAILURE: Wrong directory"; echo "Expected: $EXPECTED_DIR"; echo "Actual: $ACTUAL_DIR"; exit 1
fi

ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-e" ]; then
  echo "ISOLATION FAILURE: Wrong branch"; echo "Expected: wave1-agent-e"; echo "Actual: $ACTUAL_BRANCH"; exit 1
fi

git worktree list | grep -q "wave1-agent-e" || { echo "ISOLATION FAILURE: Worktree not in list"; exit 1; }
echo "✓ Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

## 1. File Ownership

- `internal/app/unused.go` — modify
- `internal/app/unused_test.go` — modify

## 2. Interfaces You Must Implement

No new cross-agent interfaces.

## 3. Interfaces You May Call

All existing `unused.go` functions (no changes to their signatures).

## 4. What to Implement

### 4.1 Sort footer for all sort modes (UX-polish)

**Current behavior (unused.go ~lines 380-388):**
```go
if unusedSort == "age" {
    if allSameInstallTime {
        fmt.Println("Note: All packages installed at the same time — age sort has no effect...")
    } else {
        fmt.Println("Sorted by: install date (oldest first)")
    }
}
```

Only `--sort age` shows a "Sorted by:" annotation. `--sort size` and `--sort score` show nothing.

**Fix:** Extend the footer to cover all sort modes:

```go
switch unusedSort {
case "age":
    if allSameInstallTime {
        fmt.Println()
        fmt.Println("Note: All packages installed at the same time — age sort has no effect. Sorted by tier, then alphabetically.")
    } else {
        fmt.Println()
        fmt.Println("Sorted by: install date (oldest first)")
    }
case "size":
    fmt.Println()
    fmt.Println("Sorted by: size (largest first)")
case "score":
    fmt.Println()
    fmt.Println("Sorted by: score (highest first)")
}
```

Note: the existing age block already has `fmt.Println()` before the annotation. The size and
score cases need it too.

### 4.2 Skip warning banner when --casks + no casks found (UX-polish)

**Current behavior:** `checkUsageWarning` is called at line 130, which is BEFORE the cask
count check at line 195. So when `--casks` is passed and there are no casks, the full warning
banner prints, then "No casks found" prints.

**Target:** When `--casks` is set and `caskCount == 0`, skip `checkUsageWarning` entirely and
exit early (clean, no banner).

**Fix:** Move the cask count check and early exit to BEFORE the `checkUsageWarning` call.

Read `unused.go` carefully. The current structure is:
```go
// line 126-130: compute showRiskyImplicit
showRiskyImplicit := ...
checkUsageWarning(st, showRiskyImplicit)   // line 130 — banner may print

// line 133: create analyzer
// line 136-143: list packages
// ...
// line 173-198: compute cask stats + early exit for empty casks
```

After the fix, the structure should be:
```go
showRiskyImplicit := ...

// Get packages and cask count FIRST (needed for early-exit check)
// NOTE: This requires moving some computation earlier.
```

The cleanest fix without major restructuring: add a pre-check before `checkUsageWarning`:

```go
// Quick pre-check: if --casks requested and no casks exist, skip warning + exit early
if unusedCasks {
    // Open store, count casks, exit if zero (no warning banner needed)
    quickCaskCheck, _ := st.DB().QueryRow("SELECT COUNT(*) FROM packages WHERE is_cask = 1").Scan(&tmpCount)
    if tmpCount == 0 {
        fmt.Println("No casks found in the Homebrew database.")
        fmt.Println("Cask tracking requires cask packages to be installed (brew install --cask <name>).")
        return nil
    }
}
checkUsageWarning(st, showRiskyImplicit)
```

Actually, the cleanest approach is to pass `unusedCasks` to `checkUsageWarning` and have it
return early if `unusedCasks && caskCount == 0`. But since we don't have `caskCount` yet at
that point, use a direct DB query in the pre-check.

Alternative (simpler): Add a boolean parameter to `checkUsageWarning`:
```go
func checkUsageWarning(st *store.Store, showRiskyImplicit bool, skipIfNoCasks bool) {
    if skipIfNoCasks {
        var caskCount int
        st.DB().QueryRow("SELECT COUNT(*) FROM packages WHERE is_cask = 1").Scan(&caskCount)
        if caskCount == 0 {
            return // Skip warning — casks command with no casks is a clean no-op
        }
    }
    ...
}
```

Call it as: `checkUsageWarning(st, showRiskyImplicit, unusedCasks)`

Choose whichever approach is cleaner given the existing code structure.

## 5. Tests to Write

1. `TestUnused_SortSizeShowsFooter` — verify "Sorted by: size (largest first)" appears in
   output when `--sort size` is used.

2. `TestUnused_SortScoreShowsFooter` — verify "Sorted by: score (highest first)" appears in
   output when `--sort score` is used.

3. `TestUnused_SortAgeFooterUnchanged` — verify existing age sort footer still works.

4. `TestUnused_CasksWithNoCasksSkipsWarning` — verify that `--casks` with no casks in DB
   outputs only the "No casks found" message, without the warning banner.

Update any existing tests that assert `checkUsageWarning` behavior or sort output.

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-e
go build ./...
go vet ./...
go test ./internal/app -run 'TestUnused' -v
```

## 7. Constraints

- Do NOT change `checkUsageWarning`'s behavior for non-cask invocations.
- The sort footer must only appear when `len(scores) > 0` (no footer on empty results).
- If you change the `checkUsageWarning` signature, update ALL call sites in `unused.go`
  (there may be only one call site).

## 8. Report

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-e
git add internal/app/unused.go internal/app/unused_test.go
git commit -m "wave1-agent-e: sort footer for size/score + casks warning skip"
```

Append to this file:

```yaml
### Agent E — Completion Report
status: complete | partial | blocked
worktree: .claude/worktrees/wave1-agent-e
commit: {sha}
files_changed:
  - internal/app/unused.go
  - internal/app/unused_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestUnused_SortSizeShowsFooter
  - TestUnused_SortScoreShowsFooter
  - TestUnused_SortAgeFooterUnchanged
  - TestUnused_CasksWithNoCasksSkipsWarning
verification: PASS | FAIL
```

---

### Agent E — Completion Report
