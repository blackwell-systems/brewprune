# Wave 1 Agent D: Quickstart PATH warning ordering + self-test duration

You are Wave 1 Agent D. Your task is to:
1. Reorder the quickstart summary so the critical PATH warning appears BEFORE the "Setup
   complete" text, not after.
2. Announce the self-test expected duration BEFORE the spinner starts (Step 4).

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-d 2>/dev/null || true

ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-d"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then
  echo "ISOLATION FAILURE: Wrong directory"; echo "Expected: $EXPECTED_DIR"; echo "Actual: $ACTUAL_DIR"; exit 1
fi

ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-d" ]; then
  echo "ISOLATION FAILURE: Wrong branch"; echo "Expected: wave1-agent-d"; echo "Actual: $ACTUAL_BRANCH"; exit 1
fi

git worktree list | grep -q "wave1-agent-d" || { echo "ISOLATION FAILURE: Worktree not in list"; exit 1; }
echo "✓ Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

## 1. File Ownership

- `internal/app/quickstart.go` — modify
- `internal/app/quickstart_test.go` — modify

## 2. Interfaces You Must Implement

No new cross-agent interfaces.

## 3. Interfaces You May Call

```go
isOnPATH(dir string) bool                      // from status.go (already shared)
isConfiguredInShellProfile(dir string) bool    // from status.go (already shared)
detectShellConfig() string                      // from doctor.go (already shared)
```

## 4. What to Implement

### 4.1 Reorder PATH warning before completion message (UX-improvement)

**Current output (quickstart.go ~line 229-257):**
```
Setup complete — one step remains:

IMPORTANT: Wait 1-2 weeks before acting on recommendations.

What happens next:
  • The daemon runs in the background...
  • After 1-2 weeks, run: brewprune unused --tier safe

Check status anytime: brewprune status
Run diagnostics:      brewprune doctor

⚠  TRACKING IS NOT ACTIVE YET

   Your shell has not loaded the new PATH...
```

**Problem:** "Setup complete" followed by "Wait 1-2 weeks" followed by the tracking warning is
confusing — users may act on the "complete" message before reading the critical warning.

**Target output (when PATH not active):**
```
⚠  TRACKING IS NOT ACTIVE YET

   Your shell has not loaded the new PATH. Commands you run now
   will NOT be tracked by brewprune.

   To activate tracking immediately:
     source ~/.profile

   Or restart your terminal.

Setup complete — one step remains (see warning above).

IMPORTANT: Wait 1-2 weeks before acting on recommendations.

What happens next:
  • The daemon runs in the background, tracking Homebrew binary usage
  • After 1-2 weeks, run: brewprune unused --tier safe

Check status anytime: brewprune status
Run diagnostics:      brewprune doctor
```

**Implementation:** In `runQuickstart`, look at the summary block (lines ~229-257 of
quickstart.go). Move the `pathNotActive` warning block BEFORE the "Setup complete" print.
Update the "Setup complete" message when pathNotActive to read:
```
Setup complete — one step remains (see warning above).
```

When PATH is active (`!pathNotActive`):
```
Setup complete!

IMPORTANT: Wait 1-2 weeks before acting on recommendations.
...
```

### 4.2 Announce self-test duration before spinner (UX-polish)

**Current output (quickstart.go ~line 193):**
```
Step 4/4: Running self-test (tracking verified)
[spinner starts immediately]
```

**Target output:**
```
Step 4/4: Running self-test (~30s)
[spinner starts with "Verifying shim → daemon → database pipeline"]
```

**Implementation:** Change the step header on line 193:
```go
// Before:
fmt.Println("Step 4/4: Running self-test (tracking verified)")
// After:
fmt.Println("Step 4/4: Running self-test (~30s)")
```

The spinner itself already shows the descriptive message. The duration hint moves to the
step header, which users see before the spinner starts.

## 5. Tests to Write

1. `TestQuickstart_PathWarningBeforeCompletion` — verify that when PATH is not active, the
   "TRACKING IS NOT ACTIVE YET" text appears in output BEFORE "Setup complete".

2. `TestQuickstart_SetupCompleteWhenPathActive` — verify that when PATH is active, "Setup
   complete!" (without caveat) appears and no tracking warning is shown.

3. `TestQuickstart_SelfTestStepShowsDuration` — verify that "Step 4/4:" line contains "~30s"
   before the spinner message.

Check existing quickstart tests for any assertions about output ordering and update them.

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-d
go build ./...
go vet ./...
go test ./internal/app -run 'TestQuickstart' -v
```

## 7. Constraints

- The reordering must preserve all existing content — only the ORDER changes, not the content.
- The "one step remains" phrasing must stay; only the parenthetical changes from "(see warning
  above)" to nothing when path is active.
- Do NOT add a new call to `isOnPATH` — `pathNotActive` is already computed on line 229 and
  can be reused.

## 8. Report

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-d
git add internal/app/quickstart.go internal/app/quickstart_test.go
git commit -m "wave1-agent-d: path warning before completion + self-test duration hint"
```

Append to this file:

```yaml
### Agent D — Completion Report
status: complete | partial | blocked
worktree: .claude/worktrees/wave1-agent-d
commit: {sha}
files_changed:
  - internal/app/quickstart.go
  - internal/app/quickstart_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestQuickstart_PathWarningBeforeCompletion
  - TestQuickstart_SetupCompleteWhenPathActive
  - TestQuickstart_SelfTestStepShowsDuration
verification: PASS | FAIL
```

---

### Agent D — Completion Report
status: complete
worktree: .claude/worktrees/wave1-agent-d
commit: e885e01
files_changed:
  - internal/app/quickstart.go
  - internal/app/quickstart_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestQuickstart_PathWarningBeforeCompletion
  - TestQuickstart_SetupCompleteWhenPathActive
  - TestQuickstart_SelfTestStepShowsDuration
verification: PASS
