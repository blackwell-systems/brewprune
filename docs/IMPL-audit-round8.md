# IMPL: Audit Round 8 Fixes

> Source: `docs/cold-start-audit-r8.md`

---

### Suitability Assessment

**Verdict: SUITABLE WITH CAVEATS**

20 findings span 8 distinct file ownership groups with fully disjoint assignments. The one caveat:
finding 1 (daemon event pipeline drops most shim events) is investigation-first — the offset
tracking or path resolution logic needs to be understood before a fix can be written. This
receives a solo Wave 0 investigation before the 7-agent Wave 1 parallel pass.

Several findings are already implemented:
- Doctor 3-state PATH check (in-profile / active / missing) — already in `doctor.go`
- Doctor alias tip suppressed during critical failures — already gated by `criticalIssues == 0`
- Doctor "All checks passed!" line — already present at line 264-265

```
Estimated times:
- Scout phase:          ~15 min (already complete)
- Wave 0 (solo):        ~15 min (investigate pipeline event loss, fix if possible)
- Wave 1 execution:     ~15 min (7 agents × 5 min avg, fully parallel)
- Merge & verification: ~5 min
Total SAW time: ~50 min

Sequential baseline: ~80 min (8 agents × 10 min avg)
Time savings: ~30 min (~38% faster)

Recommendation: Clear speedup. Proceed with Wave 0 → Wave 1.
```

---

### Pre-Implementation Scan

**TO-DO:**
- `internal/watcher/shim_processor.go` — only 1 of 5+ shim events reaches DB; offset tracking
  or Linuxbrew path resolution suspected
- `internal/app/doctor.go:254` — pipeline failure action says "restart daemon" but daemon IS
  running; real fix is `source ~/.profile`
- `internal/app/quickstart.go:227-249` — PATH warning block appears after "Setup complete!";
  needs reordering or integration into success message
- `internal/app/quickstart.go:~75` — "already in PATH" wording misleading (it's in profile, not
  active shell PATH); should say "already configured in ~/.profile"
- `internal/app/unused.go:501-514 + 224-229` — two separate warning blocks when no usage data;
  consolidate into one
- `internal/app/unused.go:394` — footer says "(risky, hidden)" when risky IS being shown due to
  no-data fallback (`showRiskyImplicit == true`); add `|| showRiskyImplicit` to showAll condition
- `internal/app/unused.go` — `--sort age` output shows no sort direction indicator
- `internal/app/unused.go` — `--tier safe --all` error message slightly circular; make more
  actionable
- `internal/app/remove.go:191-197, 210` — skipped packages list printed before action table;
  reorder
- `internal/app/remove.go:355-356` — multiple flag error only reports first two flags; report all
- `internal/app/remove.go` — "explicitly installed (not a dependency)" warning when user named
  the package; remove or reframe
- `internal/app/explain.go:161` — MEDIUM recommendation suggests `brewprune remove <pkg>` without
  `--dry-run`; add `--dry-run` to suggested command
- `internal/app/status.go:174` — "Last scan: just now · 0 formulae" shown when DB doesn't exist
  (dir stat succeeds because `getDBPath()` creates directory); should show "never"
- `internal/app/stats.go` — default sort order (TotalRuns desc) not shown to user; add note
- `internal/app/root.go` — unknown subcommand error redirects to `--help` without listing
  valid commands inline
- `internal/app/watch.go` — 30-second polling interval note buried at end of description

**DONE (no agent needed):**
- Doctor 3-state PATH check (already implemented in `doctor.go`)
- Doctor alias tip suppression during critical failures (already gated)
- Doctor "All checks passed!" line (already present at line 264-265)
- Doctor PATH warning contextual message post-quickstart (multi-state already handled)

---

### Dependency Graph

```
                  [Leaf nodes — no agent cross-deps]
shim_processor.go          (Wave 0 Agent A: investigation + fix)
doctor.go                  (Agent B)
quickstart.go              (Agent C)
unused.go                  (Agent D)
remove.go                  (Agent E)
explain.go                 (Agent F)
status.go + stats.go       (Agent G)
root.go + watch.go         (Agent H)
```

No cross-agent interfaces. Each agent owns its files completely. Wave 1 does not depend on
Wave 0's output (different file sets).

---

### Interface Contracts

No new shared interfaces. All changes are internal to each agent's files.

---

### File Ownership

| File | Agent | Wave | Notes |
|------|-------|------|-------|
| `internal/watcher/shim_processor.go` | A | 0 | investigate + fix event pipeline |
| `internal/watcher/shim_processor_test.go` | A | 0 | |
| `internal/app/doctor.go` | B | 1 | fix pipeline failure action message |
| `internal/app/doctor_test.go` | B | 1 | |
| `internal/app/quickstart.go` | C | 1 | reorder PATH warning, fix "already in PATH" wording |
| `internal/app/quickstart_test.go` | C | 1 | |
| `internal/app/unused.go` | D | 1 | double warning, footer bug, sort direction, error msg |
| `internal/app/unused_test.go` | D | 1 | |
| `internal/app/remove.go` | E | 1 | skipped list order, multi-flag error, "explicit" warning |
| `internal/app/remove_test.go` | E | 1 | |
| `internal/app/explain.go` | F | 1 | add --dry-run to MEDIUM recommendation |
| `internal/app/explain_test.go` | F | 1 | |
| `internal/app/status.go` | G | 1 | "just now" → "never" when DB doesn't exist |
| `internal/app/status_test.go` | G | 1 | |
| `internal/app/stats.go` | G | 1 | add sort order note to output |
| `internal/app/stats_test.go` | G | 1 | |
| `internal/app/root.go` | H | 1 | list valid commands in unknown subcommand error |
| `internal/app/root_test.go` | H | 1 | |
| `internal/app/watch.go` | H | 1 | move polling interval note up |
| `internal/app/watch_test.go` | H | 1 | |

---

### Wave Structure

```
Wave 0: [A]          ← solo pipeline investigation (shim_processor.go)
           | (A completes)
Wave 1: [B][C][D][E][F][G][H]   ← 7 parallel agents
```

Wave 1 is gated on Wave 0 completing. If Wave 0's changes touch only `shim_processor.go`
(likely), Wave 1 agents start without any dependency on Wave 0's output.

---

### Agent Prompts

---

#### Wave 0 Agent A: Investigate shim event pipeline loss

You are Wave 0 Agent A. Investigate why only 1 of 5+ shim invocations reaches the database
after a 30-second polling cycle, and fix the root cause in `shim_processor.go`.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave0-agent-a 2>/dev/null || true

ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave0-agent-a"

if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then
  echo "ISOLATION FAILURE: Wrong directory"
  echo "Expected: $EXPECTED_DIR"
  echo "Actual: $ACTUAL_DIR"
  exit 1
fi

ACTUAL_BRANCH=$(git branch --show-current)
EXPECTED_BRANCH="wave0-agent-a"

if [ "$ACTUAL_BRANCH" != "$EXPECTED_BRANCH" ]; then
  echo "ISOLATION FAILURE: Wrong branch"
  echo "Expected: $EXPECTED_BRANCH"
  echo "Actual: $ACTUAL_BRANCH"
  exit 1
fi

git worktree list | grep -q "$EXPECTED_BRANCH" || {
  echo "ISOLATION FAILURE: Worktree not in git worktree list"
  exit 1
}

echo "✓ Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

## 1. File Ownership

- `internal/watcher/shim_processor.go` — modify (fix pipeline event loss)
- `internal/watcher/shim_processor_test.go` — modify (add regression tests)

**Read-only references:**
- `internal/store/queries.go` — understand BatchInsertUsage and GetUsageOffset/SetUsageOffset
- `internal/watcher/watcher.go` — understand how shim_processor is called in the poll loop

## 2. Interfaces You Must Implement

No new interfaces. Fix existing behavior within `ProcessUsageLog`.

## 3. Interfaces You May Call

```go
// store (existing)
func (s *Store) GetUsageOffset() (int64, error)
func (s *Store) SetUsageOffset(offset int64) error
func (s *Store) BatchInsertUsage(events []UsageEvent) error
```

## 4. What to Implement

**Finding:** After 5 shim invocations (git, jq, bat, fd, rg), only 1 usage event reaches the
database after the 30-second polling cycle. `usage.log` contains 6 entries (including 1 from
quickstart self-test), but 5 are lost.

**Investigation steps:**

1. Read `internal/watcher/shim_processor.go` — the full `ProcessUsageLog` function
2. Read `internal/store/queries.go` — `GetUsageOffset` and `SetUsageOffset` contract
3. Read `internal/watcher/watcher.go` — how often `ProcessUsageLog` is called, whether it
   runs in a loop or once per poll tick

**Primary suspects (in priority order):**

- **Offset advancing past all events on first flush**: After processing, `SetUsageOffset` may
  record the end-of-file position. On the next tick, `GetUsageOffset` starts from there,
  finding nothing. If the offset is set to EOF *before* batch-insert, a crash or early return
  would lose all events from that tick.

- **Linuxbrew path resolution failure**: `optPathMap` is keyed by full opt path
  (`/opt/homebrew/bin/git`, `/usr/local/bin/git`). Linuxbrew uses
  `/home/linuxbrew/.linuxbrew/bin/`. The basename fallback via `binaryMap[basename]` should
  handle this — confirm it actually works, and confirm the fallback fires for all 5 packages.

- **Single-event batch limit**: Check if `BatchInsertUsage` is called with a batch of 1 when
  multiple events are available. If the function inserts only the first event and discards the
  rest, all others are silently lost.

- **Off-by-one in line parsing**: If `parseShimLogLine` fails on any line (malformed entry),
  subsequent entries in that batch might be skipped without error.

**Fix:** After root cause identification, fix the specific issue. Common fixes:
- If offset bug: set offset AFTER successful batch insert, not before
- If path resolution: add Linuxbrew path prefix to `optPathMap` lookup chain
- If batch truncation: ensure all parsed lines are included in the batch

## 5. Tests to Write

1. `TestProcessUsageLog_MultipleLinesRecorded` — writes 5 shim log entries, calls
   `ProcessUsageLog`, verifies all 5 events inserted (not just 1)
2. `TestProcessUsageLog_LinuxbrewPathResolution` — log entry with Linuxbrew path
   (`/home/linuxbrew/.linuxbrew/bin/git`) resolves to the correct package name
3. `TestProcessUsageLog_OffsetAdvancesAfterInsert` — verifies offset is not advanced
   if batch insert fails (events not silently lost on error)

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave0-agent-a
go build ./...
go vet ./...
go test ./internal/watcher/...
go test ./...
```

## 7. Constraints

- Do not change the `ProcessUsageLog` function signature
- Do not modify store layer files (queries.go) — report any needed store changes as
  out-of-scope dependencies
- If root cause is not found, document all investigation findings in the completion report
  and add defensive logging (write to watcher's log file, not stdout)
- Do not mark status complete if the pipeline bug is not fixed

## 8. Report

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave0-agent-a
git add internal/watcher/
git commit -m "wave0-agent-a: fix shim event pipeline loss in ProcessUsageLog"
```

Append completion report to this IMPL doc under `### Agent A — Completion Report`.

```yaml
### Agent A — Completion Report
status: complete | partial | blocked
worktree: .claude/worktrees/wave0-agent-a
commit: {sha}
files_changed:
  - internal/watcher/shim_processor.go
  - internal/watcher/shim_processor_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestProcessUsageLog_MultipleLinesRecorded
  - TestProcessUsageLog_LinuxbrewPathResolution
  - TestProcessUsageLog_OffsetAdvancesAfterInsert
verification: PASS | FAIL ({command})
```

Include: root cause identified, fix applied, and confidence in fix (high/medium/low).

---

#### Wave 1 Agent B: Fix doctor pipeline failure action message

You are Wave 1 Agent B. Fix the misleading action message in `doctor.go` when the pipeline
test fails but the daemon IS running.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-b 2>/dev/null || true

ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-b"

if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then
  echo "ISOLATION FAILURE: Wrong directory"; echo "Expected: $EXPECTED_DIR"; echo "Actual: $ACTUAL_DIR"; exit 1
fi

ACTUAL_BRANCH=$(git branch --show-current)
EXPECTED_BRANCH="wave1-agent-b"

if [ "$ACTUAL_BRANCH" != "$EXPECTED_BRANCH" ]; then
  echo "ISOLATION FAILURE: Wrong branch"; echo "Expected: $EXPECTED_BRANCH"; echo "Actual: $ACTUAL_BRANCH"; exit 1
fi

git worktree list | grep -q "$EXPECTED_BRANCH" || { echo "ISOLATION FAILURE: Worktree not in git worktree list"; exit 1; }

echo "✓ Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

## 1. File Ownership

- `internal/app/doctor.go` — modify
- `internal/app/doctor_test.go` — modify

## 2. Interfaces You Must Implement

No new interfaces. Behavior change only.

## 3. Interfaces You May Call

Existing doctor helper functions — read the file to understand the check structure.

## 4. What to Implement

**Finding (UX-critical):** When the manual setup path is used (scan → watch --daemon → doctor),
the pipeline test fails because shims are not yet in the active shell PATH (they are in
`~/.profile` but the shell hasn't been restarted). Doctor currently says:

```
✗ Pipeline test: fail (35.4s) — no usage event recorded after 35.4s
  Action: Run 'brewprune watch --daemon' to restart the daemon
```

The daemon IS running. The real fix is `source ~/.profile` or restart the shell. The action
message blames the wrong component.

**Fix:** In `doctor.go`, find the pipeline test failure branch (around line 254). Read the
current PATH status (3-state check already implemented). When the pipeline test fails AND the
daemon is running AND the shim directory is configured in profile but not in the active PATH,
use the action message:

```
Action: Shims not in active PATH — run: source ~/.profile (or restart your shell)
```

When the pipeline fails AND the daemon is NOT running:

```
Action: Run 'brewprune watch --daemon' to start the daemon
```

Read `doctor.go` fully before implementing to understand how the PATH/shim checks are
structured and where the pipeline test result is set.

## 5. Tests to Write

1. `TestDoctorPipelineFailureMessage_DaemonRunningPathNotActive` — verifies that when pipeline
   fails with daemon running and PATH not active, the action message mentions `source ~/.profile`
   (or PATH activation), not `watch --daemon`
2. `TestDoctorPipelineFailureMessage_DaemonNotRunning` — verifies that when pipeline fails with
   daemon not running, the action message mentions `watch --daemon`

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-b
go build ./...
go vet ./...
go test ./internal/app/ -run TestDoctor -v
go test ./...
```

## 7. Constraints

- Do not modify any file outside `doctor.go` and `doctor_test.go`
- If the PATH/daemon state is not accessible at the point where the pipeline action message is
  set, add the state lookup there (do not thread it through new function parameters)
- Do not change doctor's exit codes

## 8. Report

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-b
git add internal/app/doctor.go internal/app/doctor_test.go
git commit -m "wave1-agent-b: fix doctor pipeline failure action message"
```

Append to `### Agent B — Completion Report` in this IMPL doc.

```yaml
### Agent B — Completion Report
status: complete | partial | blocked
worktree: .claude/worktrees/wave1-agent-b
commit: {sha}
files_changed:
  - internal/app/doctor.go
  - internal/app/doctor_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestDoctorPipelineFailureMessage_DaemonRunningPathNotActive
  - TestDoctorPipelineFailureMessage_DaemonNotRunning
verification: PASS | FAIL ({command})
```

---

#### Wave 1 Agent C: Fix quickstart PATH warning sequencing and wording

You are Wave 1 Agent C. Fix two UX issues in `quickstart.go`: (1) the PATH warning appearing
after "Setup complete!" and (2) "already in PATH" wording being slightly misleading.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-c 2>/dev/null || true

ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-c"

if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then
  echo "ISOLATION FAILURE: Wrong directory"; echo "Expected: $EXPECTED_DIR"; echo "Actual: $ACTUAL_DIR"; exit 1
fi

ACTUAL_BRANCH=$(git branch --show-current)
EXPECTED_BRANCH="wave1-agent-c"

if [ "$ACTUAL_BRANCH" != "$EXPECTED_BRANCH" ]; then
  echo "ISOLATION FAILURE: Wrong branch"; echo "Expected: $EXPECTED_BRANCH"; echo "Actual: $ACTUAL_BRANCH"; exit 1
fi

git worktree list | grep -q "$EXPECTED_BRANCH" || { echo "ISOLATION FAILURE: Worktree not in git worktree list"; exit 1; }

echo "✓ Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

## 1. File Ownership

- `internal/app/quickstart.go` — modify
- `internal/app/quickstart_test.go` — modify

## 2. Interfaces You Must Implement

No new interfaces. Output order and wording changes only.

## 3. Interfaces You May Call

Existing quickstart helpers — read the file to understand structure.

## 4. What to Implement

**Finding 1 (UX-improvement):** The quickstart output prints "Setup complete!" and next steps,
then *afterward* prints the large `⚠ TRACKING IS NOT ACTIVE YET` PATH warning block. A user
reading top-to-bottom feels done after "Setup complete!" and is then confused by the warning.

Read `quickstart.go` lines 227-249 to see the exact output order. Fix: either
(a) print the PATH warning *before* the "Setup complete!" summary, or
(b) change "Setup complete!" to "Setup complete — one step remains:" when PATH is not yet active.

Option (b) is preferred — it avoids splitting the summary block.

**Finding 2 (UX-polish):** On second quickstart run, Step 2 reports:
`✓ /home/brewuser/.brewprune/bin is already in PATH`

This is slightly misleading because the directory is in `~/.profile` but not necessarily in the
active shell PATH. Fix: change the message to say "already configured in ~/.profile" or
"already added to ~/.profile" to distinguish the profile file from the live `$PATH`.

**Finding 3 (UX-polish):** `quickstart --help` self-test description does not mention:
- The test takes ~30 seconds
- What to do if the self-test fails

Add a note to the quickstart command's `Long` description field (read the file to find it).
Keep it to 1-2 sentences.

## 5. Tests to Write

1. `TestQuickstartPathWarningBeforeOrInSuccess` — verifies that when PATH is not yet active,
   the PATH activation reminder is not separated from the final step output (appears integrated
   into success message or before it, not after a "Setup complete!" line)
2. `TestQuickstartAlreadyConfiguredWording` — verifies that the "already in PATH" message
   uses "configured" or "added to ~/.profile" rather than "already in PATH"

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-c
go build ./...
go vet ./...
go test ./internal/app/ -run TestQuickstart -v
go test ./...
```

## 7. Constraints

- Do not change the quickstart logic — only output order and wording
- Do not modify any file outside `quickstart.go` and `quickstart_test.go`
- The `⚠ TRACKING IS NOT ACTIVE YET` block content may remain as-is; only its placement changes

## 8. Report

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-c
git add internal/app/quickstart.go internal/app/quickstart_test.go
git commit -m "wave1-agent-c: fix quickstart PATH warning order and wording"
```

Append to `### Agent C — Completion Report` in this IMPL doc.

```yaml
### Agent C — Completion Report
status: complete | partial | blocked
worktree: .claude/worktrees/wave1-agent-c
commit: {sha}
files_changed:
  - internal/app/quickstart.go
  - internal/app/quickstart_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestQuickstartPathWarningBeforeOrInSuccess
  - TestQuickstartAlreadyConfiguredWording
verification: PASS | FAIL ({command})
```

---

#### Wave 1 Agent D: Fix unused output issues

You are Wave 1 Agent D. Fix four UX issues in `unused.go`: double warning banner, footer
"risky, hidden" contradiction, sort direction note, and --all/--tier error message clarity.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-d 2>/dev/null || true

ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-d"

if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then
  echo "ISOLATION FAILURE: Wrong directory"; echo "Expected: $EXPECTED_DIR"; echo "Actual: $ACTUAL_DIR"; exit 1
fi

ACTUAL_BRANCH=$(git branch --show-current)
EXPECTED_BRANCH="wave1-agent-d"

if [ "$ACTUAL_BRANCH" != "$EXPECTED_BRANCH" ]; then
  echo "ISOLATION FAILURE: Wrong branch"; echo "Expected: $EXPECTED_BRANCH"; echo "Actual: $ACTUAL_BRANCH"; exit 1
fi

git worktree list | grep -q "$EXPECTED_BRANCH" || { echo "ISOLATION FAILURE: Worktree not in git worktree list"; exit 1; }

echo "✓ Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

## 1. File Ownership

- `internal/app/unused.go` — modify
- `internal/app/unused_test.go` — modify

## 2. Interfaces You Must Implement

No new interfaces.

## 3. Interfaces You May Call

```go
// output (existing)
func RenderReclaimableFooter(safe, medium, risky TierStats, showAll bool) string
```

## 4. What to Implement

Read `unused.go` fully before making changes.

**Finding 1 (UX-improvement):** Double warning banner when no usage data exists. There are two
separate warning blocks: `checkUsageWarning()` (lines 501-514) AND the `showRiskyImplicit`
banner (lines 224-229). Consolidate into one. When `showRiskyImplicit` is true, update the
warning message to include the note that risky packages are being shown:

```
⚠  No usage data yet — run 'brewprune quickstart' to set up tracking.
   All packages (including risky tier) are shown until usage data is collected.
   To set up tracking:
   1. Run 'brewprune quickstart' to configure shims and start the daemon
   2. Use your packages normally for a few days
   3. Return here for confidence-scored recommendations
```

Then suppress the second banner entirely (the `showRiskyImplicit` one).

**Finding 2 (UX-polish):** Footer contradiction. At line 394:
```go
output.RenderReclaimableFooter(safeTier, mediumTier, riskyTier, unusedAll || unusedTier != "")
```
When `showRiskyImplicit == true`, risky IS being shown but the footer's `showAll=false` causes
it to say "(risky, hidden)". Fix: change to:
```go
output.RenderReclaimableFooter(safeTier, mediumTier, riskyTier, unusedAll || unusedTier != "" || showRiskyImplicit)
```

**Finding 3 (UX-improvement):** `--sort age` shows no sort direction indicator. Add a line
below the tier summary header when `--sort age` is active:
```
Sorted by: install date (oldest first)
```
Read the code to confirm whether oldest-first or newest-first is the actual sort order, and
use the correct label.

**Finding 4 (UX-polish):** `--tier safe --all` error message. Current:
```
Error: --all and --tier cannot be used together; --tier already filters to a specific tier
```
Change to:
```
Error: --all and --tier are mutually exclusive
  Use --tier safe to show only safe packages, or --all to show all tiers
```

## 5. Tests to Write

1. `TestUnusedDoubleWarningConsolidated` — verifies that with no usage data, only ONE warning
   block appears in output (not two)
2. `TestUnusedFooterNoHiddenWhenRiskyImplicit` — verifies that when `showRiskyImplicit` is
   true, the footer does not contain "(risky, hidden)"
3. `TestUnusedSortAgeDirectionNote` — verifies that `--sort age` output includes a sort
   direction note
4. `TestUnusedAllTierMutualExclusiveError` — verifies improved error message format for
   `--all --tier` combination

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-d
go build ./...
go vet ./...
go test ./internal/app/ -run TestUnused -v
go test ./...
```

## 7. Constraints

- Do not modify `output/table.go` or `output/` — only `unused.go`
- Do not change the `RenderReclaimableFooter` signature
- The `checkUsageWarning()` function may be modified or its output suppressed in the no-data
  case, but do not delete it if it serves other code paths

## 8. Report

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-d
git add internal/app/unused.go internal/app/unused_test.go
git commit -m "wave1-agent-d: fix unused double warning, footer, sort note, error message"
```

Append to `### Agent D — Completion Report` in this IMPL doc.

```yaml
### Agent D — Completion Report
status: complete | partial | blocked
worktree: .claude/worktrees/wave1-agent-d
commit: {sha}
files_changed:
  - internal/app/unused.go
  - internal/app/unused_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestUnusedDoubleWarningConsolidated
  - TestUnusedFooterNoHiddenWhenRiskyImplicit
  - TestUnusedSortAgeDirectionNote
  - TestUnusedAllTierMutualExclusiveError
verification: PASS | FAIL ({command})
```

---

#### Wave 1 Agent E: Fix remove output ordering and error messages

You are Wave 1 Agent E. Fix three UX issues in `remove.go`: skipped packages list printed
before the action table, multi-flag error reporting only two flags, and misleading "explicitly
installed" warning.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-e 2>/dev/null || true

ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-e"

if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then
  echo "ISOLATION FAILURE: Wrong directory"; echo "Expected: $EXPECTED_DIR"; echo "Actual: $ACTUAL_DIR"; exit 1
fi

ACTUAL_BRANCH=$(git branch --show-current)
EXPECTED_BRANCH="wave1-agent-e"

if [ "$ACTUAL_BRANCH" != "$EXPECTED_BRANCH" ]; then
  echo "ISOLATION FAILURE: Wrong branch"; echo "Expected: $EXPECTED_BRANCH"; echo "Actual: $ACTUAL_BRANCH"; exit 1
fi

git worktree list | grep -q "$EXPECTED_BRANCH" || { echo "ISOLATION FAILURE: Worktree not in git worktree list"; exit 1; }

echo "✓ Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

## 1. File Ownership

- `internal/app/remove.go` — modify
- `internal/app/remove_test.go` — modify

## 2. Interfaces You Must Implement

No new interfaces.

## 3. Interfaces You May Call

Existing remove helpers — read the file fully before implementing.

## 4. What to Implement

Read `remove.go` fully before making changes, paying attention to:
- Where `setFlags` is constructed and the error at line 355-356
- Where skipped packages are printed (lines 191-197)
- Where the action table is printed (line 210 area)
- Where the "explicitly installed" warning is emitted

**Finding 1 (UX-improvement):** Skipped packages list printed before action table. For
`remove --medium --dry-run`, 31 "skipped (locked by installed dependents)" lines appear before
the removal candidates table. Fix: move the skipped list to print AFTER the action table and
summary. Alternatively (preferred): collapse the skipped list to a summary line by default and
expand only with `--verbose`:

```
31 packages skipped (locked by dependents) — run with --verbose to see details
```

When `--verbose` is set, print the full list after the action table.

**Finding 2 (UX-polish):** Multi-flag error at line 355-356 only reports the first two flags.
When 3+ tier flags are set (e.g. `--safe --medium --risky`), all should be reported:

```
Error: only one tier flag can be specified at a time (got --safe, --medium, and --risky)
```

Find the `setFlags` slice construction and use proper comma-separated formatting with "and"
before the last item (Oxford comma style for 3+ items).

**Finding 3 (UX-polish):** "explicitly installed (not a dependency)" warning when the user
named the package. When a user runs `brewprune remove bat fd --dry-run`, they explicitly chose
to remove `bat` and `fd`. Seeing `⚠ bat: explicitly installed (not a dependency)` is
confusing — it sounds like a reason NOT to remove.

Fix: suppress this warning when packages are explicitly named on the command line. The warning
is useful in tier-based mode (where the tool decides what to consider), not in name-based mode.

## 5. Tests to Write

1. `TestRemoveSkippedListAppearsAfterTable` — verifies that skipped packages appear after the
   removal table in output, not before it
2. `TestRemoveMultiFlagErrorReportsAll` — verifies that `--safe --medium --risky` error message
   includes all three flags, not just the first two
3. `TestRemoveExplicitPackageNoExplicitlyInstalledWarning` — verifies that named-package removal
   does not emit the "explicitly installed" warning

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-e
go build ./...
go vet ./...
go test ./internal/app/ -run TestRemove -v
go test ./...
```

## 7. Constraints

- Do not modify any file outside `remove.go` and `remove_test.go`
- Do not change remove's exit codes or the removal logic itself
- The verbose expansion of the skipped list must be consistent with the existing `--verbose`
  flag behavior in the file

## 8. Report

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-e
git add internal/app/remove.go internal/app/remove_test.go
git commit -m "wave1-agent-e: fix remove skipped order, multi-flag error, explicit warning"
```

Append to `### Agent E — Completion Report` in this IMPL doc.

```yaml
### Agent E — Completion Report
status: complete | partial | blocked
worktree: .claude/worktrees/wave1-agent-e
commit: {sha}
files_changed:
  - internal/app/remove.go
  - internal/app/remove_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestRemoveSkippedListAppearsAfterTable
  - TestRemoveMultiFlagErrorReportsAll
  - TestRemoveExplicitPackageNoExplicitlyInstalledWarning
verification: PASS | FAIL ({command})
```

---

#### Wave 1 Agent F: Add --dry-run to explain MEDIUM recommendation

You are Wave 1 Agent F. Add `--dry-run` to the suggested removal command in `explain.go`'s
MEDIUM-tier recommendation.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-f 2>/dev/null || true

ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-f"

if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then
  echo "ISOLATION FAILURE: Wrong directory"; echo "Expected: $EXPECTED_DIR"; echo "Actual: $ACTUAL_DIR"; exit 1
fi

ACTUAL_BRANCH=$(git branch --show-current)
EXPECTED_BRANCH="wave1-agent-f"

if [ "$ACTUAL_BRANCH" != "$EXPECTED_BRANCH" ]; then
  echo "ISOLATION FAILURE: Wrong branch"; echo "Expected: $EXPECTED_BRANCH"; echo "Actual: $ACTUAL_BRANCH"; exit 1
fi

git worktree list | grep -q "$EXPECTED_BRANCH" || { echo "ISOLATION FAILURE: Worktree not in git worktree list"; exit 1; }

echo "✓ Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

## 1. File Ownership

- `internal/app/explain.go` — modify
- `internal/app/explain_test.go` — modify

## 2. Interfaces You Must Implement

No new interfaces. Single string change in recommendation output.

## 3. Interfaces You May Call

Existing explain helpers — read the file to find where recommendations are rendered.

## 4. What to Implement

**Finding (UX-improvement):** `brewprune explain git` (MEDIUM tier) shows:

```
Recommendation: Review before removing. Check if you use this package indirectly.
                If certain, run 'brewprune remove git'
```

A new user following this advice would run remove without seeing a dry-run preview first.

Fix at line ~161: change the suggested command to:
```
If certain, run 'brewprune remove git --dry-run' to preview, then without --dry-run to remove.
```

Also check: does the SAFE-tier recommendation also suggest `brewprune remove <pkg>` without
`--dry-run`? If so, apply the same fix to the SAFE recommendation. The RISKY recommendation
likely already says to use caution; review it for consistency.

While reviewing `explain.go`, also check the "1 used, 8 unused dependents" wording in verbose
mode (Finding [AREA 5] openssl@3 description). If `dependentCount` labels are in this file,
change "1 used, N unused dependents" to "1 active dependent, N inactive dependents". If the
label is in `output/` or `analyzer/`, document as out-of-scope dep.

## 5. Tests to Write

1. `TestExplainMediumRecommendationIncludesDryRun` — verifies that MEDIUM recommendation
   contains `--dry-run` in the suggested command
2. `TestExplainSafeRecommendationIncludesDryRun` — verifies that SAFE recommendation (if it
   suggests `brewprune remove`) also contains `--dry-run`

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-f
go build ./...
go vet ./...
go test ./internal/app/ -run TestExplain -v
go test ./...
```

## 7. Constraints

- Do not modify any file outside `explain.go` and `explain_test.go`
- If "1 used / N unused dependents" label is outside these files, document as out-of-scope dep

## 8. Report

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-f
git add internal/app/explain.go internal/app/explain_test.go
git commit -m "wave1-agent-f: add --dry-run to explain MEDIUM removal recommendation"
```

Append to `### Agent F — Completion Report` in this IMPL doc.

```yaml
### Agent F — Completion Report
status: complete | partial | blocked
worktree: .claude/worktrees/wave1-agent-f
commit: {sha}
files_changed:
  - internal/app/explain.go
  - internal/app/explain_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestExplainMediumRecommendationIncludesDryRun
  - TestExplainSafeRecommendationIncludesDryRun
verification: PASS | FAIL ({command})
```

---

#### Wave 1 Agent G: Fix status "just now" and add stats sort note

You are Wave 1 Agent G. Fix two UX issues: (1) `status` showing "Last scan: just now · 0
formulae" when no scan has ever run, and (2) `stats` output showing no sort order indicator.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-g 2>/dev/null || true

ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-g"

if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then
  echo "ISOLATION FAILURE: Wrong directory"; echo "Expected: $EXPECTED_DIR"; echo "Actual: $ACTUAL_DIR"; exit 1
fi

ACTUAL_BRANCH=$(git branch --show-current)
EXPECTED_BRANCH="wave1-agent-g"

if [ "$ACTUAL_BRANCH" != "$EXPECTED_BRANCH" ]; then
  echo "ISOLATION FAILURE: Wrong branch"; echo "Expected: $EXPECTED_BRANCH"; echo "Actual: $ACTUAL_BRANCH"; exit 1
fi

git worktree list | grep -q "$EXPECTED_BRANCH" || { echo "ISOLATION FAILURE: Worktree not in git worktree list"; exit 1; }

echo "✓ Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

## 1. File Ownership

- `internal/app/status.go` — modify
- `internal/app/status_test.go` — modify
- `internal/app/stats.go` — modify
- `internal/app/stats_test.go` — modify

## 2. Interfaces You Must Implement

No new interfaces.

## 3. Interfaces You May Call

Read both files fully before implementing.

## 4. What to Implement

**Finding 1 (UX-improvement) in `status.go`:** After removing `~/.brewprune`, `brewprune
status` shows "Last scan: just now · 0 formulae" because `getDBPath()` creates the `.brewprune`
directory, making the `os.Stat` succeed and returning the directory's fresh mtime.

Fix at line 174: Before setting `dbMtime` from `fi.ModTime()`, check if `formulaeCount == 0`.
When `formulaeCount == 0`, set `dbMtime = "never"` (or equivalent "never" constant/string) and
the scan line to `Last scan: never — run 'brewprune scan'`.

Read the file to find the exact pattern used for the "never" case (there may be an existing
pattern for other "never" displays in the file).

**Finding 2 (UX-polish) in `stats.go`:** `stats --all` shows packages in no visually apparent
order. The sort is by TotalRuns desc (established in `output/table.go`), but users see no
indication.

Fix: In `stats.go`, after the package list is loaded and before calling the render function,
print a single line:

```
Sorted by: most used first
```

Only print this when `--all` is set (showing all packages) or when more than 1 package is
shown. If `--package` is specified (single package view), no sort note is needed.

## 5. Tests to Write

1. `TestStatusLastScanNeverWhenZeroFormulae` — verifies that when `formulaeCount == 0`, the
   status output shows "never" for last scan (not "just now")
2. `TestStatsAllShowsSortNote` — verifies that `stats --all` output includes the sort note
   "Sorted by: most used first"
3. `TestStatsSinglePackageNoSortNote` — verifies that `stats --package <name>` output does
   NOT include the sort note

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-g
go build ./...
go vet ./...
go test ./internal/app/ -run "TestStatus|TestStats" -v
go test ./...
```

## 7. Constraints

- Do not modify any file outside `status.go`, `status_test.go`, `stats.go`, `stats_test.go`
- Do not change status or stats exit codes
- The sort note for stats should be a simple printed line, not a table header change (the
  table header is in `output/table.go` which is outside your scope)

## 8. Report

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-g
git add internal/app/status.go internal/app/status_test.go internal/app/stats.go internal/app/stats_test.go
git commit -m "wave1-agent-g: fix status just-now zero formulae, add stats sort note"
```

Append to `### Agent G — Completion Report` in this IMPL doc.

```yaml
### Agent G — Completion Report
status: complete | partial | blocked
worktree: .claude/worktrees/wave1-agent-g
commit: {sha}
files_changed:
  - internal/app/status.go
  - internal/app/status_test.go
  - internal/app/stats.go
  - internal/app/stats_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestStatusLastScanNeverWhenZeroFormulae
  - TestStatsAllShowsSortNote
  - TestStatsSinglePackageNoSortNote
verification: PASS | FAIL ({command})
```

---

#### Wave 1 Agent H: Improve unknown subcommand error and watch help

You are Wave 1 Agent H. Improve two UX issues: (1) unknown subcommand error redirects to
`--help` without listing valid commands inline, and (2) watch --help polling interval note
is buried at the end of the description.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-h 2>/dev/null || true

ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-h"

if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then
  echo "ISOLATION FAILURE: Wrong directory"; echo "Expected: $EXPECTED_DIR"; echo "Actual: $ACTUAL_DIR"; exit 1
fi

ACTUAL_BRANCH=$(git branch --show-current)
EXPECTED_BRANCH="wave1-agent-h"

if [ "$ACTUAL_BRANCH" != "$EXPECTED_BRANCH" ]; then
  echo "ISOLATION FAILURE: Wrong branch"; echo "Expected: $EXPECTED_BRANCH"; echo "Actual: $ACTUAL_BRANCH"; exit 1
fi

git worktree list | grep -q "$EXPECTED_BRANCH" || { echo "ISOLATION FAILURE: Worktree not in git worktree list"; exit 1; }

echo "✓ Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

## 1. File Ownership

- `internal/app/root.go` — modify
- `internal/app/root_test.go` — modify (or create if it doesn't exist)
- `internal/app/watch.go` — modify
- `internal/app/watch_test.go` — modify

## 2. Interfaces You Must Implement

No new interfaces.

## 3. Interfaces You May Call

Read `root.go` and `watch.go` fully before implementing.

## 4. What to Implement

**Finding 1 (UX-improvement) in `root.go`:** `brewprune blorp` exits 1 with:
```
Error: unknown command "blorp" for "brewprune"
Run 'brewprune --help' for usage.
```

Fix: Add a custom `RunE` or use cobra's `PersistentPreRunE` / `SetHelpCommand` / error handler
to append a compact list of valid subcommands to the unknown command error:

```
Error: unknown command "blorp" for "brewprune"
Valid commands: scan, unused, remove, undo, status, stats, explain, doctor, quickstart, watch, completion
Run 'brewprune --help' for usage.
```

Read `root.go` to understand how the cobra command is configured. Cobra's
`SilenceErrors`/`SilenceUsage` settings and `RunE` on RootCmd may need adjustment.

If cobra doesn't easily allow custom error text here, set a custom usage template or use
`cobra.OnInitialize` / `cmd.SetHelpTemplate`. Do not fight cobra's error handling — find the
path of least resistance that adds the valid commands line.

**Finding 2 (UX-polish) in `watch.go`:** The watch command's `Long` description ends with
"Usage data is written every 30 seconds to minimise I/O overhead." This is the last line and
easy to miss. Users need to know about the 30-second delay to understand why tracking isn't
visible immediately after a shim invocation.

Fix: Move the 30-second note to an earlier position in the `Long` description, perhaps as the
second sentence after the main description. The positioning should make it salient to users
checking `watch --help` while waiting for tracking to appear.

## 5. Tests to Write

1. `TestRootUnknownCommandIncludesValidCommands` — verifies that `brewprune blorp` error
   output includes a list of valid command names (e.g., contains "scan" and "unused")
2. `TestWatchHelpPollingNoteProminent` — verifies that "30 seconds" appears before the last
   line of the watch Long description (i.e., it is not the last sentence)

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-h
go build ./...
go vet ./...
go test ./internal/app/ -run "TestRoot|TestWatch" -v
go test ./...
```

## 7. Constraints

- Do not modify any file outside `root.go`, `root_test.go`, `watch.go`, `watch_test.go`
- Do not change exit codes
- The valid commands list should be hardcoded (not dynamically discovered) to keep the error
  handling simple and to avoid iterating cobra's command tree at error time

## 8. Report

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-h
git add internal/app/root.go internal/app/root_test.go internal/app/watch.go internal/app/watch_test.go
git commit -m "wave1-agent-h: list valid commands in unknown subcommand error, move watch polling note"
```

Append to `### Agent H — Completion Report` in this IMPL doc.

```yaml
### Agent H — Completion Report
status: complete | partial | blocked
worktree: .claude/worktrees/wave1-agent-h
commit: {sha}
files_changed:
  - internal/app/root.go
  - internal/app/watch.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestRootUnknownCommandIncludesValidCommands
  - TestWatchHelpPollingNoteProminent
verification: PASS | FAIL ({command})
```

---

### Agent A — Completion Report

```yaml
status: complete
commit: 16b2a254937e7e03002a5cb5278f6c8f010921c8
files_changed:
  - internal/watcher/shim_processor.go
  - internal/watcher/shim_processor_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestProcessUsageLog_MultipleLinesRecorded
  - TestProcessUsageLog_LinuxbrewPathResolution
  - TestProcessUsageLog_OffsetAdvancesAfterInsert
verification: PASS (go test ./... — 34/34 tests pass, 1 skipped)
```

**Root cause identified:** `bufio.Scanner` internal read-ahead. When the scanner is wrapping
a file, its first `Read` call on the underlying `*os.File` typically reads the entire small file
(or a 64KB chunk) into an internal buffer. After `scanner.Scan()` returns the first line,
`f.Seek(0, io.SeekCurrent)` does NOT return the byte offset after that line — it returns
wherever the OS file cursor landed after the scanner's last `Read` call, which for a sub-64KB
file is EOF.

In the reported scenario: 6 entries in `usage.log` (~280 bytes total). On the first
`ProcessUsageLog` call (at watcher startup), the scanner buffered the entire file in one
`Read`. If only 1 package was in the DB (the quickstart self-test package), 5 events hit
`continue` (package not found). The 1 found event was inserted. Then `newOffset =
f.Seek(0, io.SeekCurrent)` = 280 (EOF), and that was saved. On the next tick, the offset was
at EOF, so nothing was re-scanned. The 5 unresolved events were permanently lost.

There was a second latent bug: when `maxShimLogLinesPerTick` is hit on a large file, the
scanner would have buffered ahead past the cap boundary, causing `newOffset` to jump past
lines that were never logically processed — silently skipping them on subsequent ticks.

**Fix applied:** Replaced `bufio.Scanner` with `bufio.Reader.ReadString('\n')` and manual
per-line offset accumulation (`newOffset += int64(len(raw))`). This tracks exactly how many
bytes have been logically consumed from the reader's perspective, independent of the internal
read buffer position. Also added the Linuxbrew prefix
(`/home/linuxbrew/.linuxbrew/bin/<name>`) to the opt-path lookup chain.

**Confidence: high.** The scanner read-ahead behavior is verified experimentally — after
reading 1 line from a 278-byte file with `bufio.Scanner`, `f.Seek(0, io.SeekCurrent)`
returns 278. The `bufio.Reader` approach tracks bytes returned by `ReadString`, not OS
cursor position.

**For Wave 1 agents:** No store-layer changes needed. The fix is entirely within
`ProcessUsageLog`. The Linuxbrew path resolution lookup was added in the opt-path chain
(after the basename fallback, not before it — Linuxbrew is tertiary priority). Wave 1 agents
working on unrelated files should not encounter conflicts.

---

### Agent B — Completion Report

```yaml
status: complete
worktree: .claude/worktrees/wave1-agent-b
commit: 006cea2
files_changed:
  - internal/app/doctor.go
  - internal/app/doctor_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestDoctorPipelineFailureMessage_DaemonRunningPathNotActive
  - TestDoctorPipelineFailureMessage_DaemonNotRunning
verification: PASS (go test ./... — all 11 packages pass)
```

Implementation notes: Two boolean variables (`shimPathActive`, `shimPathConfigured`) are now tracked before Check 6 and populated during the existing shim/PATH check so they are accessible at the pipeline failure branch in Check 8. The action message is now selected based on three conditions: (1) daemon running + shim dir in profile but not in active PATH → tells user to `source ~/.zprofile` (shell-specific via `detectShellConfig()`); (2) daemon not running → `watch --daemon` to start; (3) fallback → original "restart the daemon" message. `TestDoctorPipelineFailureMessage_DaemonNotRunning` exercises the skip path (not pipeline failure), since doctor skips the pipeline test when the daemon is not running, and verifies the daemon check action mentions `watch --daemon` and the skip message appears.

---

### Agent C — Completion Report

```yaml
status: complete
worktree: .claude/worktrees/wave1-agent-c
commit: e049cc7
files_changed:
  - internal/app/quickstart.go
  - internal/app/quickstart_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestQuickstartSuccessMessageWhenPathNotActive
  - TestQuickstartAlreadyConfiguredWording
verification: PASS (go test ./... — 11/11 packages)
```

All three findings addressed:

- Finding 1: Introduced `pathNotActive` bool in the summary block. When true, heading reads "Setup complete — one step remains:" so the PATH warning block follows as the natural "one step." When false (PATH already active), heading stays "Setup complete!" with no warning block.
- Finding 2: Changed "already in PATH" to "already configured in ~/.profile" to clarify the entry is in the profile file, not necessarily the live shell environment.
- Finding 3: Added two sentences to the `Long` description: "The self-test takes approximately 30 seconds. If it fails, run 'brewprune doctor' to diagnose."

No existing test assertions referenced the old "Setup complete!" or "already in PATH" strings, so no existing tests required updates. Build, vet, and full test suite all pass (GOWORK=off required due to go.work pointing only to main repo).

---

### Agent D — Completion Report

```yaml
status: complete
worktree: .claude/worktrees/wave1-agent-d
commit: 44fe3ada3a907bb65f83f67512d295c3685dd33d
files_changed:
  - internal/app/unused.go
  - internal/app/unused_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestUnusedDoubleWarningConsolidated
  - TestUnusedFooterNoHiddenWhenRiskyImplicit
  - TestUnusedSortAgeDirectionNote
  - TestUnusedAllTierMutualExclusiveError
verification: PASS (go test ./... — all packages pass, 19/19 TestUnused* tests pass)
```

All four findings addressed:

- **Finding 1 (double warning)**: Removed the second `showRiskyImplicit` banner block. Updated `checkUsageWarning` to accept a `showRiskyImplicit bool` parameter; when true, the consolidated warning includes "All tiers (including risky) are shown as a result." in the single block. The old separate `⚠ No usage data yet — showing all packages (risky tier included)` notice is gone.

- **Finding 2 (footer contradiction)**: Fixed `RenderReclaimableFooter` call to pass `unusedAll || unusedTier != "" || showRiskyImplicit` as `showAll`, preventing the "(risky, hidden)" label when risky IS shown implicitly.

- **Finding 3 (sort direction note)**: Added a conditional note after table rendering. When `--sort age` is active and packages have distinct install times, prints `Sorted by: install date (oldest first)`. The same-install-time fallback note is preserved and gated inside the `unusedSort == "age"` block.

- **Finding 4 (error message)**: Changed the `--all/--tier` conflict error to: `--all and --tier are mutually exclusive\n  Use --tier safe to show only safe packages, or --all to show all tiers`. Updated two existing tests (`TestUnused_TierAndAllConflict`, `TestDoubleErrorPrefix_Fixed`) that asserted the old "cannot be used together" phrase to assert "mutually exclusive" instead.

Tests run with `GOWORK=off` from the worktree root since the parent `go.work` only references the main repo.

---

### Agent E — Completion Report

```yaml
status: complete
worktree: .claude/worktrees/wave1-agent-e
commit: f332d2d
files_changed:
  - internal/app/remove.go
  - internal/app/remove_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestRemoveSkippedSummaryAppearsAfterTable
  - TestRemoveMultiFlagErrorReportsAll
  - TestRemoveExplicitPackageNoExplicitlyInstalledWarning
verification: PASS (go test ./... — 11/11 packages)
```

All three findings implemented:

- Finding 1: Skipped packages list now deferred until after the action table in both the explicit-args and tier-based branches. Tier-based branch emits `N packages skipped (locked by dependents) — run with --verbose to see details`; explicit-args branch emits a simpler summary (no verbose note, since no `--verbose` flag exists on the remove command).
- Finding 2: Multi-flag error now uses Oxford comma formatting for 3+ flags: `got --safe, --medium, and --risky`. Two-flag case unchanged: `got --safe and --medium`.
- Finding 3: In the explicit-args branch, `ValidateRemoval` warnings containing "explicitly installed" are filtered out before display. Other warnings (e.g., "used recently") pass through unchanged.

---

### Agent F — Completion Report

```yaml
status: complete
worktree: .claude/worktrees/wave1-agent-f
commit: e3d6ff3
files_changed:
  - internal/app/explain.go
  - internal/app/explain_test.go
interface_deviations: []
out_of_scope_deps:
  - "1 used, N unused dependents" label — not generated in explain.go; search needed in output/ or analyzer/ packages (not modified per constraints)
tests_added:
  - TestExplainMediumRecommendationIncludesDryRun
  - TestExplainSafeRecommendationIncludesDryRun
verification: PASS (go test ./... — 11/11 packages)
```

**Notes:**

- MEDIUM recommendation updated: `"If certain, run 'brewprune remove git --dry-run' to preview, then without --dry-run to remove."`
- SAFE recommendation also updated to include `--dry-run`: `"Run 'brewprune remove --safe --dry-run' to preview, then without --dry-run to remove all safe-tier packages."` The SAFE tier does not suggest `brewprune remove <pkg>` (per-package), it suggests `--safe` (bulk), but the same UX risk applies — a user following the advice would run `remove --safe` without a preview first.
- Fix 3 ("1 used, N unused dependents" label): Not present in `explain.go` or `explain_test.go`. Likely generated in `internal/output/` or `internal/analyzer/`. Documented as out-of-scope per constraints.
- Build and all tests pass with `GOWORK=off go test ./...`.

---

### Agent G — Completion Report

```yaml
status: complete
worktree: .claude/worktrees/wave1-agent-g
commit: 0c549d970e28d2f6d6d2666c81853c6c05fd2826
files_changed:
  - internal/app/status.go
  - internal/app/status_test.go
  - internal/app/stats.go
  - internal/app/stats_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestStatusLastScanNeverWhenZeroFormulae
  - TestStatsAllShowsSortNote
  - TestStatsSinglePackageNoSortNote
verification: PASS (go test ./... — 11/11 packages)
```

**Implementation notes:**

Finding 1 (status.go): The "Last scan" line previously always used `formatDuration(time.Since(fi.ModTime()))`, which produced "just now" when `formulaeCount == 0` because `getDBPath()` creates the `.brewprune` directory as a side effect, making `os.Stat` succeed on a freshly-created directory. The fix checks `formulaeCount == 0` first and unconditionally sets `dbMtime = "never — run 'brewprune scan'"` in that case. The time-based path is only taken when formulae exist. This follows the same "never" string convention already used elsewhere in the file (e.g. `formatTime` returns "never" for zero `time.Time` values).

Finding 2 (stats.go): Added a `fmt.Println("Sorted by: most used first")` call in `showUsageTrends` guarded by `len(filteredStats) > 1`, inserted just before the `output.RenderUsageTable` call. The `showPackageStats` path (single-package `--package` mode) is untouched, so the note never appears there. The guard `> 1` also suppresses the note for edge cases where only a single package ends up in the filtered set even with `--all`.

---

### Agent H — Completion Report

```yaml
status: complete
worktree: .claude/worktrees/wave1-agent-h
commit: 2994c1b
files_changed:
  - internal/app/root.go
  - internal/app/root_test.go
  - internal/app/watch.go
  - internal/app/watch_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestRootUnknownCommandIncludesValidCommands
  - TestWatchHelpPollingNoteProminent
verification: PASS (go test ./... — all packages pass)
```

**Approach for Finding 1 (root.go — unknown command error):**

Used a custom `Args` validator on `RootCmd` instead of intercepting cobra's error path. Cobra's `Find()` function checks whether `commandFound.Args == nil`: when nil it calls `legacyArgs` (which generates the plain "unknown command" error); when non-nil it returns `nil` and defers validation to `execute()` → `ValidateArgs()`. By setting a non-nil `Args` function that checks `len(args) > 0 && cmd.HasSubCommands()`, the custom error is generated inside `execute()` and propagated through cobra's standard error-printing path (`SilenceErrors: false`).

The hardcoded `validCommandsList` constant was added as a package-level `var` alongside the other root command vars.

**Approach for Finding 2 (watch.go — polling note prominence):**

Moved "Usage data is written every 30 seconds to minimise I/O overhead." from the last line of the `Long` description to the second line (immediately after the opening sentence). The note now appears in the first paragraph, before the detailed explanation of the shim log mechanism.

**Test contamination fix:**

My custom `Args` approach changed the cobra execution path for unknown commands: previously the error came from `Find()` (bypassing `execute()`), now it comes from `ValidateArgs()` inside `execute()`. This exposed a latent test contamination bug: tests that called `Execute()` with `--help` left the pflag "help" flag value set to `true`. Subsequent tests calling `Execute()` with an unknown command would hit the `helpVal=true` check in `execute()` and return `flag.ErrHelp` (which `ExecuteC` converts to nil) instead of the unknown-command error.

Fixed by adding explicit help-flag resets (`f.Value.Set("false"); f.Changed = false`) at the start of `TestExecute_UnknownCommandHelpHint` and `TestUnknownSubcommandErrorOrder`. The new `TestRootUnknownCommandIncludesValidCommands` test avoids this issue entirely by calling `RootCmd.Args(RootCmd, []string{"blorp"})` directly rather than going through `Execute()`.
