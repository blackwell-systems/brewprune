# Wave 1 Agent G: Status shims "0 commands" label clarification

You are Wave 1 Agent G. Your task is to fix the misleading "0 commands" shim count in
`brewprune status`. The count shows 0 shim symlinks even when usage events have been recorded,
because "commands" refers to symlinks in the shim directory — not to events recorded. The label
needs to clarify what is being counted.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-g 2>/dev/null || true

ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-g"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then
  echo "ISOLATION FAILURE: Wrong directory"; echo "Expected: $EXPECTED_DIR"; echo "Actual: $ACTUAL_DIR"; exit 1
fi

ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-g" ]; then
  echo "ISOLATION FAILURE: Wrong branch"; echo "Expected: wave1-agent-g"; echo "Actual: $ACTUAL_BRANCH"; exit 1
fi

git worktree list | grep -q "wave1-agent-g" || { echo "ISOLATION FAILURE: Worktree not in list"; exit 1; }
echo "✓ Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

## 1. File Ownership

- `internal/app/status.go` — modify
- `internal/app/status_test.go` — modify

## 2. Interfaces You Must Implement

No new cross-agent interfaces.

## 3. Interfaces You May Call

Existing `status.go` functions (no changes to their signatures):
```go
countSymlinks(dir string) int   // counts symlinks in ~/.brewprune/bin/
isOnPATH(dir string) bool
isConfiguredInShellProfile(dir string) bool
```

## 4. What to Implement

### 4.1 Clarify shim count label (UX-improvement)

**Current output:**
```
Shims:        inactive · 0 commands · PATH configured (restart shell to activate)
```

**Problem:** "0 commands" is opaque. The shim directory (`~/.brewprune/bin/`) has 0 symlinks
because after quickstart, symlinks are only populated during `scan`. But the user may have
recorded 3 usage events, making "0 commands" feel inconsistent with "3 events total".

**What `shimCount` actually counts:** symlinks in `~/.brewprune/bin/` (i.e., the number of
Homebrew binaries for which brewprune has set up intercept shims). This is the number of
commands that WILL be tracked when the PATH is active.

**Two options:**

**Option A:** Rename "commands" to "shims" to be more precise:
```
Shims:        inactive · 0 shims · PATH configured (restart shell to activate)
```

**Option B:** Rename to "intercepted" to clarify the meaning:
```
Shims:        inactive · 0 intercepted · PATH configured (restart shell to activate)
```

**Option C:** Show a more descriptive label and explain what 0 means in context. When
`shimCount == 0` and shim binary exists but PATH not active, say:
```
Shims:        not yet active · PATH configured (restart shell to activate)
```
(omit the count when 0 — it adds no information)

**Recommended:** Option C for `shimCount == 0` + Option A ("N shims") for `shimCount > 0`:
```go
// status.go line ~161
if shimCount == 0 {
    fmt.Printf(label+"%s · %s\n", "Shims:", shimStatus, pathStatus)
} else {
    fmt.Printf(label+"%s · %d shims · %s\n", "Shims:", shimStatus, shimCount, pathStatus)
}
```

Also, fix the "inactive" vs "not yet active" distinction. When the shim binary exists, PATH is
configured but not sourced, and shimCount == 0:
```
Shims:        not yet active · PATH configured (restart shell to activate)
```

When shimCount > 0 and active:
```
Shims:        active · 5 shims · PATH active ✓
```

When shimCount > 0 and inactive (should not normally happen but handle gracefully):
```
Shims:        inactive · 5 shims · PATH missing ⚠
```

### 4.2 Shim status when DB is deleted

**Current behavior:** When `~/.brewprune` is deleted, `status` still shows "PATH configured"
if the shell profile has the PATH export. This is confusing because the shim directory doesn't
exist.

**Fix:** Before showing pathStatus, check if the shim directory actually exists:
```go
shimDirExists := false
if _, err := os.Stat(shimDir); err == nil {
    shimDirExists = true
}
```

If `!shimDirExists`, override pathStatus to: `"shims not installed (run 'brewprune scan' to build)"`
and set `shimStatus = "not installed"`.

This is a small addition that prevents the confusing "PATH configured" message when there's
nothing to configure.

## 5. Tests to Write

1. `TestStatus_ShimsLabelWhenZeroShimsPathConfigured` — verify that when shimCount == 0 and
   PATH is configured but not active, the output says "not yet active" and omits the "0 shims"
   count.

2. `TestStatus_ShimsLabelWhenShimsPresent` — verify that when shimCount > 0 and active, the
   output says "N shims" (not "N commands").

3. `TestStatus_ShimDirMissingShowsNotInstalled` — verify that when the shim directory doesn't
   exist, the Shims line shows "not installed" rather than "inactive · PATH configured".

Check existing tests for the old "0 commands" string and update them.

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-g
go build ./...
go vet ./...
go test ./internal/app -run 'TestStatus' -v
```

## 7. Constraints

- Do NOT change the Events line or the Last scan line.
- The shimCount computation (`countSymlinks`) is correct — only the label changes.
- The `shimActive` variable currently checks `shimCount > 0`. After the fix, this logic stays
  the same — "active" means "symlinks exist AND PATH is active".

## 8. Report

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-g
git add internal/app/status.go internal/app/status_test.go
git commit -m "wave1-agent-g: clarify shims label and count in status output"
```

Append to this file:

```yaml
### Agent G — Completion Report
status: complete | partial | blocked
worktree: .claude/worktrees/wave1-agent-g
commit: {sha}
files_changed:
  - internal/app/status.go
  - internal/app/status_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestStatus_ShimsLabelWhenZeroShimsPathConfigured
  - TestStatus_ShimsLabelWhenShimsPresent
  - TestStatus_ShimDirMissingShowsNotInstalled
verification: PASS | FAIL
```

---

### Agent G — Completion Report
