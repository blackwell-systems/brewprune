# Wave 1 Agent F: Remove nonexistent-package error + error chain

You are Wave 1 Agent F. Your task is to:
1. Improve the error message for `remove nonexistent-package` to match the helpful context
   already in `explain nonexistent-package`.
2. Fix the error chain exposure in `remove.go` when no database exists.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-f 2>/dev/null || true

ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-f"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then
  echo "ISOLATION FAILURE: Wrong directory"; echo "Expected: $EXPECTED_DIR"; echo "Actual: $ACTUAL_DIR"; exit 1
fi

ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-f" ]; then
  echo "ISOLATION FAILURE: Wrong branch"; echo "Expected: wave1-agent-f"; echo "Actual: $ACTUAL_BRANCH"; exit 1
fi

git worktree list | grep -q "wave1-agent-f" || { echo "ISOLATION FAILURE: Worktree not in list"; exit 1; }
echo "✓ Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

## 1. File Ownership

- `internal/app/remove.go` — modify
- `internal/app/remove_test.go` — modify

## 2. Interfaces You Must Implement

No new cross-agent interfaces.

## 3. Interfaces You May Call

```go
st.GetPackage(name string) (*brew.Package, error)   // returns error if not found
store.ErrNotInitialized                              // sentinel for "run scan" error
```

## 4. What to Implement

### 4.1 Helpful error for nonexistent package (UX-improvement)

**Current behavior (remove.go ~line 119):**
```go
pkgInfo, err := st.GetPackage(pkg)
if err != nil {
    return fmt.Errorf("package %q not found", pkg)
}
```

**Current output:**
```
Error: package "nonexistent-package" not found
```

**Target output (matching explain.go's error):**
```
Error: package not found: nonexistent-package

Check the name with 'brew list' or 'brew search nonexistent-package'.
If you just installed it, run 'brewprune scan' to update the index.
```

**Fix:** Use the same `fmt.Fprintf(os.Stderr, ...)` + `os.Exit(1)` pattern as `explain.go`:

```go
pkgInfo, err := st.GetPackage(pkg)
if err != nil {
    fmt.Fprintf(os.Stderr, "Error: package not found: %s\n\nCheck the name with 'brew list' or 'brew search %s'.\nIf you just installed it, run 'brewprune scan' to update the index.\n", pkg, pkg)
    os.Exit(1)
}
```

This is in the `len(args) > 0` branch of `runRemove`, inside the loop over `filteredArgs`.

### 4.2 Fix error chain for no-database case (UX-improvement)

**Current behavior when no DB exists:**
```
Error: failed to get packages: failed to list packages: database not initialized — run 'brewprune scan' to create the database
```

**Target:**
```
Error: database not initialized — run 'brewprune scan' to create the database
```

The error chain is created at `remove.go:177`:
```go
scores, err := getPackagesByTier(anlzr, tier)
if err != nil {
    return fmt.Errorf("failed to get packages: %w", err)
}
```

And `getPackagesByTier` wraps: `return nil, fmt.Errorf("failed to get %s tier: %w", t, err)`
which wraps `err` from `anlzr.GetPackagesByTier(t)` which wraps `ErrNotInitialized`.

**Fix:** Unwrap the error chain before returning. Add a helper or use `errors.Is` to detect
`store.ErrNotInitialized` and surface it directly:

```go
scores, err := getPackagesByTier(anlzr, tier)
if err != nil {
    // Unwrap to surface terminal error directly (avoid internal chain exposure)
    cause := err
    for errors.Unwrap(cause) != nil {
        cause = errors.Unwrap(cause)
    }
    return cause
}
```

Add `"errors"` to imports if not present.

Also apply the same unwrapping to the `store.New` error path (line ~85):
```go
st, err := store.New(dbPath)
if err != nil {
    return fmt.Errorf("failed to open database: %w", err)
}
```
This one is fine as-is (just one level of wrapping, DB open errors are valid context).

## 5. Tests to Write

1. `TestRemove_NonexistentPackageHelpfulError` — verify that removing a nonexistent package
   produces the multi-line helpful error (with "brew list" and "brewprune scan" suggestions),
   not just "package X not found".

2. `TestRemove_NoDatabaseErrorUnwrapped` — verify that when no DB exists, the error message
   is the terminal "database not initialized" message without the "failed to get packages"
   prefix.

Check existing tests in `remove_test.go` for assertions about the old "package %q not found"
format and update them.

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-f
go build ./...
go vet ./...
go test ./internal/app -run 'TestRemove' -v
```

## 7. Constraints

- The `os.Exit(1)` pattern is already used in `explain.go` for package-not-found. Use the
  same pattern for consistency.
- Only the NAMED PACKAGE error gets the helpful context. Tier-based removal errors (when no
  packages found in tier) are handled separately and should not change.
- Do NOT unwrap the `store.New` error — that's a valid single-level wrap.

## 8. Report

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-f
git add internal/app/remove.go internal/app/remove_test.go
git commit -m "wave1-agent-f: helpful remove error + error chain unwrapping"
```

Append to this file:

```yaml
### Agent F — Completion Report
status: complete | partial | blocked
worktree: .claude/worktrees/wave1-agent-f
commit: {sha}
files_changed:
  - internal/app/remove.go
  - internal/app/remove_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestRemove_NonexistentPackageHelpfulError
  - TestRemove_NoDatabaseErrorUnwrapped
verification: PASS | FAIL
```

---

### Agent F — Completion Report
