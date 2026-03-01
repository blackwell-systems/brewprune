# IMPL: Cold-Start Audit Round 6 Fixes

Source audit: `docs/cold-start-audit-r6.md` (21 findings: 4 UX-critical, 9 UX-improvement, 8 UX-polish)

---

## Suitability Assessment

**Verdict: SUITABLE**

All 21 findings have known root causes (no investigation-first blockers). The work decomposes into 8 files with completely disjoint ownership — each `internal/app/*.go` file owns one command handler, and there are no shared mutable globals between files. All 8 agents can run in a single parallel wave. The one cross-agent concern (ANSI TTY detection appearing in multiple files) is handled by scoping each agent's fix to their own file inline, avoiding any shared interface dependency.

**Pre-implementation scan results:**
- Total items: 21 findings (+ 4 deferred below)
- Already implemented: 0
- Partially implemented: 2 (ONBOARD-3 spinner present but not during the wait; TRACK-1 status shows shim state but not proactively)
- To-do: 19

**Deferred (not in this IMPL):**
- HELP-1 (bare command shows full help) — contentious design choice; acceptable as-is
- HELP-2 (--fix flag undocumented) — no --fix flag exists; remove doc references if any; trivial
- EDGE-2 (semantic subcommand suggestions) — cobra edit-distance can't catch "list"→"unused"; requires custom alias map, not worth parallel agent
- ONBOARD-4 (watch.log empty on startup) — requires changes in `internal/watcher/daemon.go`; lower priority, handle sequentially after wave

**Estimated times:**
- Agent execution: ~8 agents × 8 min avg = 64 min, parallelized to ~8 min
- Merge & verification: ~5 min
- Total SAW time: ~13 min
- Sequential baseline: ~64 min
- Time savings: ~51 min (80% faster)

---

## Known Issues

None identified. All tests pass (`go test ./...` clean as of 2026-02-28).

---

## Dependency Graph

All 8 agents are leaves — no agent's output is required as input by any other agent.

The one cross-cutting concern is ANSI color detection (DOCTOR-1, VISUAL-1). Both fixes are scoped to their own files (doctor.go and unused.go respectively) using an inline `isColorEnabled()` helper rather than a shared package, eliminating the dependency.

EXPLAIN-2 asks that `explain.go`'s box-drawing table match `unused.go`'s compact text format. Since unused.go's format is the *canonical target* (Agent G reads it, does not change it), there is no ordering constraint between Agent F and Agent G.

**Cascade candidates (files that reference changed semantics but are NOT in any agent's scope):**
- `internal/store/queries.go` is owned by Agent H. The caller wrapping chain that produces the double-message error in EDGE-1 is in `internal/app/unused.go` and `internal/app/remove.go`, but the fix is entirely in `queries.go` (return a sentinel error). No cascading change needed in callers — `fmt.Errorf("... %w", err)` will pass the improved message through automatically.

---

## Interface Contracts

No new cross-agent interfaces. Each agent's changes are self-contained within their owned files. Existing function signatures remain unchanged.

---

## File Ownership

| File | Agent | Wave | Findings |
|------|-------|------|----------|
| `internal/app/remove.go` | A | 1 | REMOVE-1, REMOVE-2, REMOVE-3, VISUAL-2 |
| `internal/app/remove_test.go` | A | 1 | — |
| `internal/app/stats.go` | B | 1 | TRACK-3, TRACK-4, EDGE-3 |
| `internal/app/watch.go` | B | 1 | TRACK-2 |
| `internal/app/stats_test.go` | B | 1 | — |
| `internal/app/watch_test.go` | B | 1 | — |
| `internal/app/doctor.go` | C | 1 | ONBOARD-2, ONBOARD-3, DOCTOR-1, DOCTOR-2, DOCTOR-3 |
| `internal/app/doctor_test.go` | C | 1 | — |
| `internal/app/quickstart.go` | D | 1 | ONBOARD-1, HELP-4 |
| `internal/app/quickstart_test.go` | D | 1 | — |
| `internal/app/undo.go` | E | 1 | REMOVE-4, REMOVE-5 |
| `internal/app/undo_test.go` | E | 1 | — |
| `internal/app/unused.go` | F | 1 | UNUSED-2, UNUSED-3, UNUSED-4, UNUSED-5, VISUAL-1 |
| `internal/app/unused_test.go` | F | 1 | — |
| `internal/app/explain.go` | G | 1 | EXPLAIN-1, EXPLAIN-2, EXPLAIN-3 |
| `internal/app/explain_test.go` | G | 1 | — |
| `internal/store/queries.go` | H | 1 | EDGE-1 |
| `internal/store/db_test.go` | H | 1 | — |

---

## Wave Structure

```
Wave 1: [A] [B] [C] [D] [E] [F] [G] [H]   ← 8 parallel agents, fully independent
```

No Wave 0 needed. No Wave 2 needed. Single wave, maximum parallelism.

---

## Agent Prompts

---

### Wave 1 Agent A: Remove command — tier conflict, risky escalation, --no-snapshot warning, display consistency

You are Wave 1 Agent A. Fix 4 UX issues in `internal/app/remove.go`.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-a 2>/dev/null || true

ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-a"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then
  echo "ISOLATION FAILURE: Wrong directory"; echo "Expected: $EXPECTED_DIR"; echo "Actual: $ACTUAL_DIR"; exit 1
fi
ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-a" ]; then
  echo "ISOLATION FAILURE: Wrong branch"; echo "Expected: wave1-agent-a"; echo "Actual: $ACTUAL_BRANCH"; exit 1
fi
echo "✓ Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

## 1. File Ownership

- `internal/app/remove.go` — modify
- `internal/app/remove_test.go` — modify

## 2. Interfaces You Must Implement

No new exported interfaces. All changes are internal to the package.

## 3. Interfaces You May Call

Existing: `output.NewSpinner`, `output.Bold`, `output.Color*` — already imported.

## 4. What to Implement

Read `internal/app/remove.go` fully before starting. Key locations:

**REMOVE-1** (`determineTier()` ~line 279): When multiple tier shorthand flags are set simultaneously (`--safe --medium`, `--safe --risky`, `--medium --risky`, or any combination with `--tier`), return an error: `"only one tier flag can be specified at a time (got --safe and --medium)"`. Count how many of `{removeRisky, removeMedium, removeSafe}` are true; if >1, return error. Also reject combining `--tier` with any shorthand.

**REMOVE-2** (`confirmRemoval()` ~line 376): For risky-tier removal, require a more alarming confirmation. Instead of `Remove N packages? [y/N]:`, show:
```
WARNING: You are about to remove N risky packages that may include core dependencies.
This could break installed tools. Removal cannot be undone without a snapshot.
Type "yes" to confirm (or press Enter to cancel):
```
Accept only the literal string `"yes"` (not `"y"`) for risky confirmation. For safe/medium tiers, keep the existing `[y/N]` prompt.

**REMOVE-3** (dry-run/confirm output ~line 196): When `--no-snapshot` is active, change the snapshot line from plain text to a yellow warning:
`⚠  Snapshot: SKIPPED (--no-snapshot) — removal cannot be undone!`
Use ANSI yellow only when stdout is a TTY (check `os.Getenv("NO_COLOR") == "" && term.IsTerminal(int(os.Stdout.Fd()))` using `golang.org/x/term`, or use an inline isatty check; if you can't import term cleanly, use `os.Getenv("TERM") != "dumb" && os.Stdout.Fd() > 0`).

**VISUAL-2** (explicit-package remove display): When packages are specified by name (not by tier), the output uses a different format than tier-based removal. Find the display path for explicit packages in `remove.go` and make it use the same `displayConfidenceScores()` table as tier-based removal. The "explicitly installed" warnings (`⚠ bat: explicitly installed`) should appear above the table, consistently indented. Use plain `⚠` (not `⚠️` emoji).

## 5. Tests to Write

1. `TestDetermineTier_ConflictShorthands` — calling `determineTier` with multiple shorthand flags set returns an error
2. `TestDetermineTier_ConflictShorthandAndTierFlag` — combining `--tier` and a shorthand returns an error
3. `TestConfirmRemoval_RiskyRequiresYes` — risky confirm rejects "y", accepts "yes"
4. `TestNoSnapshotWarning_DryRun` — dry-run output contains "cannot be undone" when --no-snapshot

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/app -run TestRemove -run TestDetermine -run TestConfirm -run TestNoSnapshot -timeout 60s
```

## 7. Constraints

- Do NOT change the behavior for `--yes` flag bypassing confirmation — it already works and `--yes --risky` is an intentional power-user shortcut.
- Do NOT change `displayConfidenceScores` signature; only change how it's called for the explicit-package path.
- Report any tests that need updating to expect the new error messages in your completion report.

## 8. Report

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-a
git add internal/app/remove.go internal/app/remove_test.go
git commit -m "wave1-agent-a: fix tier conflict detection, risky escalation, --no-snapshot warning, display consistency"
```

Append to `docs/IMPL-audit-round6.md`:

```yaml
### Agent A — Completion Report
status: complete | partial | blocked
worktree: .claude/worktrees/wave1-agent-a
commit: {sha}
files_changed:
  - internal/app/remove.go
  - internal/app/remove_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestDetermineTier_ConflictShorthands
  - TestDetermineTier_ConflictShorthandAndTierFlag
  - TestConfirmRemoval_RiskyRequiresYes
  - TestNoSnapshotWarning_DryRun
verification: PASS | FAIL
```

---

### Wave 1 Agent B: Stats pluralization, trend column, --days error, watch flag conflict

You are Wave 1 Agent B. Fix 4 UX issues in `internal/app/stats.go` and `internal/app/watch.go`.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-b 2>/dev/null || true
ACTUAL_DIR=$(pwd); EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-b"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then echo "ISOLATION FAILURE: Wrong directory"; exit 1; fi
ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-b" ]; then echo "ISOLATION FAILURE: Wrong branch"; exit 1; fi
echo "✓ Isolation verified"
```

## 1. File Ownership

- `internal/app/stats.go` — modify
- `internal/app/watch.go` — modify
- `internal/app/stats_test.go` — modify
- `internal/app/watch_test.go` — modify

## 2. Interfaces You Must Implement

No new exported interfaces.

## 3. Interfaces You May Call

Existing store, output, cobra APIs already imported in both files.

## 4. What to Implement

Read `internal/app/stats.go` and `internal/app/watch.go` fully before starting.

**TRACK-3** (stats.go, Trend column ~line 190): The `Trend` field is hardcoded to `"stable"` (→ arrow) for all packages. Fix: for packages with zero usage events, show `"—"` (em dash) instead of `"→"`. For packages with actual usage data, keep `"→"` as-is (real trend calculation is out of scope — we just need to stop showing a fake arrow for zero-data packages). Find where `Trend: "stable"` is set and add a condition: if the package has no events, use `"—"`.

**TRACK-4** (stats.go ~line 229): Fix pluralization in the summary line. `"last %d days"` → `"last %d day"` when days==1. `"N packages used"` → `"N package used"` when count==1. Add a simple `pluralize(n int, singular, plural string) string` helper.

**EDGE-3** (stats.go ~line 61): `--days abc` shows raw Go `strconv.ParseInt` error. Replace the cobra-generated error with a custom validator. In the stats command's `PreRunE` or at the top of `runStats`, check if `statsDays <= 0` OR if the flag value cannot be parsed as an integer, and return `fmt.Errorf("--days must be a positive integer (got %q)", statsCmd.Flag("days").Value.String())`. Note: cobra already rejects non-integer values for int flags before RunE is called, so you may need to use `StringVar` + manual parsing, OR intercept cobra's error in `main.go`/`Execute()`. Check the current flag type first. If it's already `IntVar`, add the `--days < 1` check and document that `--days abc` is handled by cobra (acceptable clean error).

**TRACK-2** (watch.go `runWatch` ~line 69): When both `--daemon` and `--stop` are set, print a warning and proceed with `--stop` (rather than silently ignoring `--daemon`): `"Warning: --daemon and --stop are mutually exclusive; stopping daemon."`. Add this check at the top of `runWatch()` before the stop check.

## 5. Tests to Write

1. `TestPluralize` — helper returns correct singular/plural forms
2. `TestStatsPluralization_OneDay` — summary uses "1 day" not "1 days" with --days 1
3. `TestStatsPluralization_OnePackage` — "1 package" not "1 packages" when count is 1
4. `TestTrendColumn_ZeroUsage` — zero-usage packages show "—" not "→"
5. `TestWatchDaemonStopConflict` — --daemon --stop prints warning

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/app -run TestStats -run TestPluralize -run TestTrend -run TestWatch -timeout 60s
```

## 7. Constraints

- The `pluralize` helper is private (lowercase). No exported API changes.
- Do not change the actual trend calculation logic — only distinguish "no data" from "flat trend".

## 8. Report

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-b
git add internal/app/stats.go internal/app/stats_test.go internal/app/watch.go internal/app/watch_test.go
git commit -m "wave1-agent-b: fix stats pluralization, trend column, --days error, watch flag conflict"
```

Append to `docs/IMPL-audit-round6.md`:

```yaml
### Agent B — Completion Report
status: complete | partial | blocked
worktree: .claude/worktrees/wave1-agent-b
commit: {sha}
files_changed:
  - internal/app/stats.go
  - internal/app/watch.go
  - internal/app/stats_test.go
  - internal/app/watch_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestPluralize
  - TestStatsPluralization_OneDay
  - TestStatsPluralization_OnePackage
  - TestTrendColumn_ZeroUsage
  - TestWatchDaemonStopConflict
verification: PASS | FAIL
```

---

### Wave 1 Agent C: Doctor — PATH hint, pipeline spinner, ANSI codes, aliases tip, line colors

You are Wave 1 Agent C. Fix 5 UX issues in `internal/app/doctor.go`.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-c 2>/dev/null || true
ACTUAL_DIR=$(pwd); EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-c"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then echo "ISOLATION FAILURE: Wrong directory"; exit 1; fi
ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-c" ]; then echo "ISOLATION FAILURE: Wrong branch"; exit 1; fi
echo "✓ Isolation verified"
```

## 1. File Ownership

- `internal/app/doctor.go` — modify
- `internal/app/doctor_test.go` — modify

## 2. Interfaces You Must Implement

No new exported interfaces.

## 3. Interfaces You May Call

`output.NewSpinner`, `shell.GetConfigFile()` or similar if available, `isatty`-equivalent.

## 4. What to Implement

Read `internal/app/doctor.go` fully before starting. Key areas:

**ONBOARD-2** (PATH action hint ~line 168): The action hint for PATH not configured is hardcoded to `~/.zprofile`. Fix: detect the actual file that quickstart wrote to. Look for a config constant or function that returns the shell config path (check `internal/shell/` or `internal/config/` packages). If a `GetConfigFile()` helper exists, use it. If not, use `shell.DetectShell()` to pick the right config file (zsh→`~/.zprofile`, bash→`~/.bash_profile`, unknown→`~/.profile`). The hint should say `source <actual-file>` matching what quickstart wrote.

**ONBOARD-3** (pipeline test feedback ~lines 204-218): The pipeline test runs for 21-26 seconds. Check if a spinner is already started before the test. If yes, it may not be refreshing or may stop before the wait. Ensure the spinner is running during the entire wait period, not just during setup. If the spinner is stopped before the wait, restart it with a message like `"Running pipeline test (~30s)..."` and keep it running until the result arrives.

**DOCTOR-1** (ANSI codes in summary ~lines 233, 239): The final summary line uses raw ANSI color codes. Wrap the color formatting in a TTY check: define an inline helper at the top of the function:
```go
colorize := func(code, s string) string {
    if os.Getenv("NO_COLOR") != "" { return s }
    f, ok := os.Stdout.(*os.File)
    if !ok { return s }
    if fi, err := f.Stat(); err != nil || (fi.Mode()&os.ModeCharDevice) == 0 { return s }
    return code + s + "\033[0m"
}
```
Use `colorize` for the summary line color and for DOCTOR-3 line colors below.

**DOCTOR-2** (aliases tip always shown ~lines 179-187): The aliases tip currently displays unconditionally when `~/.config/brewprune/aliases` does not exist. Change condition: only show the tip if usage event count is below a threshold (e.g., `totalEvents < 10`) OR if this is the first run (db has fewer than 5 packages). If the daemon is running and events are accumulating normally, suppress the tip. Alternatively, show the tip only once per week using a timestamp file at `~/.brewprune/aliases-tip-shown`. The simpler fix: check `totalEvents < 5` and only show then.

**DOCTOR-3** (color per doctor result line): Apply color to individual check result lines (not just the summary). `✓` lines → green, `⚠` lines → yellow, `✗` lines → red. Use the `colorize` helper from DOCTOR-1. Find where the check results are printed and wrap each with the appropriate color code.

## 5. Tests to Write

1. `TestDoctorPATHHint_UsesDetectedShell` — PATH action hint references detected config file, not hardcoded ~/.zprofile
2. `TestDoctorANSI_PipedOutputNoColor` — doctor output piped to non-TTY contains no ANSI codes
3. `TestDoctorAliasesTip_SuppressedWithEvents` — aliases tip not shown when usage events > threshold
4. `TestDoctorAliasesTip_ShownWhenNoEvents` — aliases tip shown when no events recorded

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/app -run TestDoctor -timeout 120s
```

Note: Some doctor tests may take >30s due to pipeline test. The 120s timeout accommodates this.

## 7. Constraints

- Do NOT remove the `output.NewSpinner` usage for the pipeline test — only ensure it covers the full wait.
- Do NOT add a new package dependency for isatty — use `os.File.Stat()` mode bits instead.
- If `shell.GetConfigFile()` does not exist, do NOT create it (out of scope). Use the shell-detection logic inline.

## 8. Report

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-c
git add internal/app/doctor.go internal/app/doctor_test.go
git commit -m "wave1-agent-c: fix PATH hint, ANSI codes, pipeline spinner, aliases tip, check line colors"
```

Append to `docs/IMPL-audit-round6.md`:

```yaml
### Agent C — Completion Report
status: complete | partial | blocked
worktree: .claude/worktrees/wave1-agent-c
commit: {sha}
files_changed:
  - internal/app/doctor.go
  - internal/app/doctor_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestDoctorPATHHint_UsesDetectedShell
  - TestDoctorANSI_PipedOutputNoColor
  - TestDoctorAliasesTip_SuppressedWithEvents
  - TestDoctorAliasesTip_ShownWhenNoEvents
verification: PASS | FAIL
```

---

### Wave 1 Agent D: Quickstart — daemon output cleanup, service→daemon terminology

You are Wave 1 Agent D. Fix 2 UX issues in `internal/app/quickstart.go`.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-d 2>/dev/null || true
ACTUAL_DIR=$(pwd); EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-d"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then echo "ISOLATION FAILURE: Wrong directory"; exit 1; fi
ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-d" ]; then echo "ISOLATION FAILURE: Wrong branch"; exit 1; fi
echo "✓ Isolation verified"
```

## 1. File Ownership

- `internal/app/quickstart.go` — modify
- `internal/app/quickstart_test.go` — modify

## 2. Interfaces You Must Implement

No new exported interfaces.

## 3. Interfaces You May Call

`watcher.IsDaemonRunning`, `watcher.StartDaemon` / `watcher.StopDaemon` as currently called.

## 4. What to Implement

Read `internal/app/quickstart.go` fully before starting.

**ONBOARD-1** (daemon startup output bleed-through ~lines 120-133): Step 3 of quickstart currently calls `startWatchDaemonFallback` which produces raw output that bleeds through into the quickstart step display, creating visual noise. The issue: the function prints its own multi-line block ("Usage tracking daemon started / PID file / Log file / To stop") which then gets followed by quickstart's own `✓` confirmation line.

Fix: capture the daemon startup output or suppress the inner block. Options:
1. Redirect stdout temporarily during the inner call and emit only the PID/log info inline.
2. Add a `quiet bool` parameter to the daemon start function so quickstart can suppress its verbose output. **But this touches watch.go (Agent B's file)** — do NOT modify watch.go.
3. Instead: look for whether `startWatchDaemonFallback` is a separate function in quickstart.go that you can modify directly. If the bleed-through output comes from a quickstart-internal function, fix it there. If it comes from calling a watch.go function directly, capture stdout: `old := os.Stdout; r, w, _ := os.Pipe(); os.Stdout = w; call(); w.Close(); io.Copy(io.Discard, r); os.Stdout = old` — only discarding the inner verbose output, then print a clean single line.

The target output for step 3 should be:
```
Step 3/4: Starting usage tracking daemon
  ✓ Daemon started (PID: 1234, log: ~/.brewprune/watch.log)
```

**HELP-4** (service → daemon terminology ~line 120): Find all occurrences of "service" in quickstart.go's user-visible output strings and replace with "daemon". Check step labels and log messages. Keep any technical brew-services references (they're accurate for the brew services path).

Also check: any reference to `~/.profile` in the "what we wrote" message — verify it's consistent with what doctor.go's ONBOARD-2 fix references (but do NOT modify doctor.go — that's Agent C).

## 5. Tests to Write

1. `TestQuickstartDaemonOutput_NoBleedThrough` — quickstart step 3 output does not contain "PID file:" or "Log file:" as standalone lines (these come from the inner watch --daemon output)
2. `TestQuickstartTerminology_NoServiceWord` — output does not contain "service" (case insensitive) in user-visible strings

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/app -run TestQuickstart -timeout 60s
```

## 7. Constraints

- Do NOT modify `internal/app/watch.go` (owned by Agent B). Work within quickstart.go.
- If capturing stdout is needed, use `os.Pipe()` — do not import external packages.

## 8. Report

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-d
git add internal/app/quickstart.go internal/app/quickstart_test.go
git commit -m "wave1-agent-d: fix daemon output bleed-through and service→daemon terminology"
```

Append to `docs/IMPL-audit-round6.md`:

```yaml
### Agent D — Completion Report
status: complete | partial | blocked
worktree: .claude/worktrees/wave1-agent-d
commit: {sha}
files_changed:
  - internal/app/quickstart.go
  - internal/app/quickstart_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestQuickstartDaemonOutput_NoBleedThrough
  - TestQuickstartTerminology_NoServiceWord
verification: PASS | FAIL
```

---

### Wave 1 Agent E: Undo — trailing @ in package names, stale DB warning

You are Wave 1 Agent E. Fix 2 UX issues in `internal/app/undo.go`.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-e 2>/dev/null || true
ACTUAL_DIR=$(pwd); EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-e"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then echo "ISOLATION FAILURE: Wrong directory"; exit 1; fi
ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-e" ]; then echo "ISOLATION FAILURE: Wrong branch"; exit 1; fi
echo "✓ Isolation verified"
```

## 1. File Ownership

- `internal/app/undo.go` — modify
- `internal/app/undo_test.go` — modify

## 2. Interfaces You Must Implement

No new exported interfaces.

## 3. Interfaces You May Call

`snapshots.Snapshot` struct fields as already imported.

## 4. What to Implement

Read `internal/app/undo.go` fully before starting.

**REMOVE-4** (trailing `@` in package display ~line 132): The package list during restore shows `bat@`, `fd@` because of a format string like `fmt.Printf("  - %s@%s%s\n", pkg.PackageName, pkg.Version, ...)` where `pkg.Version` is an empty string. Fix: when `pkg.Version == ""`, omit the `@` separator entirely. Change to:
```go
nameDisplay := pkg.PackageName
if pkg.Version != "" {
    nameDisplay = pkg.PackageName + "@" + pkg.Version
}
fmt.Printf("  - %s (%s)\n", nameDisplay, pkg.InstallType)
```
Read the actual field names from the snapshot types before implementing.

**REMOVE-5** (stale DB nudge redundancy): After `undo latest --yes`, the output already says "Run 'brewprune scan' to update the package database." Subsequent `remove` commands also warn about the stale database. This is acceptable behavior but the undo message could be made stronger: change from a passive suggestion to a clear instruction: `"⚠  Run 'brewprune scan' to update the package database before running remove."` This small wording change reduces the chance of users ignoring it.

## 5. Tests to Write

1. `TestUndoPackageDisplay_NoTrailingAt` — package names without version are displayed without `@` suffix
2. `TestUndoPackageDisplay_WithVersion` — package names with version show `name@version` format
3. `TestUndoPostRestoreMessage_IncludesScanHint` — completion message includes "brewprune scan"

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/app -run TestUndo -timeout 60s
```

## 7. Constraints

- Do NOT change the snapshot data model or store layer (out of scope).
- The format change for REMOVE-4 must handle both cases: version empty AND version non-empty.

## 8. Report

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-e
git add internal/app/undo.go internal/app/undo_test.go
git commit -m "wave1-agent-e: fix trailing @ in undo output, improve stale DB message"
```

Append to `docs/IMPL-audit-round6.md`:

```yaml
### Agent E — Completion Report
status: complete | partial | blocked
worktree: .claude/worktrees/wave1-agent-e
commit: {sha}
files_changed:
  - internal/app/undo.go
  - internal/app/undo_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestUndoPackageDisplay_NoTrailingAt
  - TestUndoPackageDisplay_WithVersion
  - TestUndoPostRestoreMessage_IncludesScanHint
verification: PASS | FAIL
```

---

### Wave 1 Agent F: Unused — casks feedback, score inversion note, flag conflict, tier banner, ANSI

You are Wave 1 Agent F. Fix 5 UX issues in `internal/app/unused.go`.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-f 2>/dev/null || true
ACTUAL_DIR=$(pwd); EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-f"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then echo "ISOLATION FAILURE: Wrong directory"; exit 1; fi
ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-f" ]; then echo "ISOLATION FAILURE: Wrong branch"; exit 1; fi
echo "✓ Isolation verified"
```

## 1. File Ownership

- `internal/app/unused.go` — modify
- `internal/app/unused_test.go` — modify

## 2. Interfaces You Must Implement

No new exported interfaces. Agent G (explain.go) will READ unused.go's verbose format as its canonical reference; you establish the format, G matches it.

## 3. Interfaces You May Call

Existing store, analyzer, output APIs already imported.

## 4. What to Implement

Read `internal/app/unused.go` fully before starting.

**UNUSED-2** (`--casks` filter, ~line 193): When `--casks` is set and after filtering the list contains zero casks, print: `"No casks found in the Homebrew database. Cask usage tracking requires cask packages to be installed."` and return. Do not silently return an empty table.

**UNUSED-3** (score inversion note ~line 513-528 `showConfidenceAssessment`): The verbose breakdown section for each package does not include the score-inversion note ("higher score = safer to remove"). Add a line directly below the breakdown header: `"  (score measures removal confidence: higher = safer to remove)"`. This makes it consistent with the note in `explain` output.

**UNUSED-4** (`--tier` + `--all` conflict, ~line 100-101): When both `unusedAll` (--all) and `unusedTier` (--tier) are set, return an error: `"--all and --tier cannot be used together; --tier already filters to a specific tier"`.

**UNUSED-5** (tier summary banner active tier, ~line where banner is printed): When `--tier` is set and the banner shows all tiers, visually mark the active tier. Wrap it in brackets: `[SAFE: 5 packages (39 MB)]` vs `MEDIUM: 31 · RISKY: 4`. Append `"(filtered)"` or just use brackets/bold. If ANSI is available, bold the active tier count.

**VISUAL-1** (`showConfidenceAssessment` ANSI leakage ~lines 513-528): The confidence footer line uses raw ANSI codes without checking if stdout is a TTY. Wrap ANSI emission in the same TTY check as DOCTOR-1's `colorize` approach:
```go
isColor := os.Getenv("NO_COLOR") == "" && func() bool {
    f, ok := os.Stdout.(*os.File)
    if !ok { return false }
    fi, err := f.Stat(); return err == nil && (fi.Mode()&os.ModeCharDevice) != 0
}()
```
Guard all `\033[` sequences in this function with `isColor`.

## 5. Tests to Write

1. `TestUnusedCasks_NoCasksMessage` — --casks with empty cask list prints informative message, exits 0
2. `TestUnusedVerbose_ScoreInversionNote` — verbose output contains "measures removal confidence"
3. `TestUnused_TierAndAllConflict` — --tier + --all returns error
4. `TestUnused_TierBannerHighlightsActive` — banner with --tier safe marks [SAFE] distinctly
5. `TestUnusedConfidenceFooter_NoANSIWhenPiped` — confidence footer has no ANSI codes when output is not a TTY

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/app -run TestUnused -timeout 60s
```

## 7. Constraints

- The verbose text format in `showConfidenceAssessment` is the CANONICAL format that Agent G (explain.go) will match. Do not change it after establishing it; document it in your completion report notes.
- Do not change the table rendering in `output/table.go` (out of scope).

## 8. Report

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-f
git add internal/app/unused.go internal/app/unused_test.go
git commit -m "wave1-agent-f: fix casks feedback, score note, flag conflict, tier banner, ANSI"
```

Append to `docs/IMPL-audit-round6.md`:

```yaml
### Agent F — Completion Report
status: complete | partial | blocked
worktree: .claude/worktrees/wave1-agent-f
commit: {sha}
files_changed:
  - internal/app/unused.go
  - internal/app/unused_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestUnusedCasks_NoCasksMessage
  - TestUnusedVerbose_ScoreInversionNote
  - TestUnused_TierAndAllConflict
  - TestUnused_TierBannerHighlightsActive
  - TestUnusedConfidenceFooter_NoANSIWhenPiped
verification: PASS | FAIL
```

---

### Wave 1 Agent G: Explain — score note position, format consistency, terminology

You are Wave 1 Agent G. Fix 3 UX issues in `internal/app/explain.go`.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-g 2>/dev/null || true
ACTUAL_DIR=$(pwd); EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-g"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then echo "ISOLATION FAILURE: Wrong directory"; exit 1; fi
ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-g" ]; then echo "ISOLATION FAILURE: Wrong branch"; exit 1; fi
echo "✓ Isolation verified"
```

## 1. File Ownership

- `internal/app/explain.go` — modify
- `internal/app/explain_test.go` — modify

## 2. Interfaces You Must Implement

No new exported interfaces.

## 3. Interfaces You May Call

`analyzer.ConfidenceScore` struct fields — already imported.

## 4. What to Implement

Read `internal/app/explain.go` AND `internal/app/unused.go` (read-only reference) before starting. The verbose format in `unused.go`'s `showConfidenceAssessment` function is your canonical format target for EXPLAIN-2.

**EXPLAIN-1** (score inversion note position ~line 158): The note "Higher removal score = more confident to remove. Usage component: 0/40 means recently used..." currently appears at the bottom of `renderExplanation()`, after the recommendation. Move it to appear directly after the table header but before the first component row. Target position: between the table header (`┌─────...`) and "│ Usage..." row. Or more simply, add a single line just before the table: `"(score measures removal confidence: higher = safer to remove)"` Then keep the detailed version at the bottom or remove the duplicate.

**EXPLAIN-2** (format inconsistency): The `renderExplanation()` function uses a Unicode box-drawing table. The `unused --verbose` format (in `showConfidenceAssessment` in unused.go) uses a compact plain-text format:
```
  Breakdown:
    Usage:        40/40 pts - never observed execution
    Dependencies: 30/30 pts - no dependents
    Age:          20/20 pts - ...
    Type:         10/10 pts - ...
  Critical: YES - capped at 70 (core system dependency)
```
Change `renderExplanation()` to use the plain-text format instead of the box-drawing table. This makes `explain` visually consistent with `unused --verbose`. Keep the header, score summary, "Why TIER:", and "Recommendation:" sections — only replace the table portion.

**EXPLAIN-3** (criticality terminology ~line 143): The explain table row says `"Criticality Penalty"` with value `-30`. But `unused --verbose` shows `"Critical: YES - capped at 70 (core system dependency)"`. After applying EXPLAIN-2 (switching to plain text format), use the same language: `"Critical: YES - capped at 70 (core system dependency)"` in the breakdown section.

## 5. Tests to Write

1. `TestRenderExplanation_ScoreNoteBeforeBreakdown` — output contains "measures removal confidence" before the first component line
2. `TestRenderExplanation_PlainTextFormat` — output does not contain "┌" (box drawing chars)
3. `TestRenderExplanation_CriticalTerminology` — critical package shows "Critical: YES" not "Criticality Penalty"

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/app -run TestExplain -run TestRender -timeout 60s
```

## 7. Constraints

- Do NOT modify `internal/app/unused.go` (owned by Agent F). Read it for format reference only.
- The plain-text format must be readable for packages with long package names — keep truncation logic from `truncateDetail`.

## 8. Report

```bash
cd /Users/dayna.blackwell/code/brewprune/.clone/worktrees/wave1-agent-g
git add internal/app/explain.go internal/app/explain_test.go
git commit -m "wave1-agent-g: fix score note position, format consistency, criticality terminology"
```

Append to `docs/IMPL-audit-round6.md`:

```yaml
### Agent G — Completion Report
status: complete | partial | blocked
worktree: .claude/worktrees/wave1-agent-g
commit: {sha}
files_changed:
  - internal/app/explain.go
  - internal/app/explain_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestRenderExplanation_ScoreNoteBeforeBreakdown
  - TestRenderExplanation_PlainTextFormat
  - TestRenderExplanation_CriticalTerminology
verification: PASS | FAIL
```

---

### Wave 1 Agent H: Store — friendly error for uninitialized database

You are Wave 1 Agent H. Fix 1 UX issue in `internal/store/queries.go`.

## 0. CRITICAL: Isolation Verification (RUN FIRST)

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-h 2>/dev/null || true
ACTUAL_DIR=$(pwd); EXPECTED_DIR="/Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-h"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then echo "ISOLATION FAILURE: Wrong directory"; exit 1; fi
ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-h" ]; then echo "ISOLATION FAILURE: Wrong branch"; exit 1; fi
echo "✓ Isolation verified"
```

## 1. File Ownership

- `internal/store/queries.go` — modify
- `internal/store/db_test.go` — modify

## 2. Interfaces You Must Implement

No new exported interfaces. The error sentinel can be unexported or documented in the completion report.

## 3. Interfaces You May Call

`strings.Contains` for "no such table" detection.

## 4. What to Implement

Read `internal/store/queries.go` fully before starting.

**EDGE-1** (raw SQL error message): When `ListPackages()`, `ListUsageEvents()`, or similar query functions fail with a SQLite "no such table" error, the raw error chain `"failed to list packages: SQL logic error: no such table: packages (1)"` is returned and bubbles up to the user.

Fix: Add an error wrapping sentinel at the top of queries.go:
```go
// ErrNotInitialized is returned when the database schema has not been created.
// Callers should check for this with errors.Is and show a user-friendly message.
var ErrNotInitialized = errors.New("database not initialized (run 'brewprune scan' to create the database)")
```

In `ListPackages()` (and any other key query functions that could fail with "no such table"), detect and wrap:
```go
if strings.Contains(err.Error(), "no such table") {
    return nil, fmt.Errorf("%w", ErrNotInitialized)
}
```

The app-layer callers do NOT need to change — `fmt.Errorf("failed to list packages: %w", ErrNotInitialized)` will produce a clean message: `"Error: failed to list packages: database not initialized (run 'brewprune scan' to create the database)"`. This is a significant improvement over the raw SQL message.

Apply the check to: `ListPackages()`, `GetPackage()`, `ListUsageEvents()` — the three functions most likely to be called before the schema exists.

Also check: `internal/store/schema.go` — does `CreateSchema()` produce a clear error if called twice or if the DB is corrupt? Note any issues in your completion report but do not fix them (out of scope).

## 5. Tests to Write

1. `TestListPackages_NoSchema_ReturnsErrNotInitialized` — calling ListPackages on a fresh empty DB (no CreateSchema called) returns ErrNotInitialized
2. `TestGetPackage_NoSchema_ReturnsErrNotInitialized` — same for GetPackage

## 6. Verification Gate

```bash
cd /Users/dayna.blackwell/code/brewprune
go build ./...
go vet ./...
go test ./internal/store -timeout 60s
```

## 7. Constraints

- `ErrNotInitialized` may be exported — it's useful for callers to check. Export it (`ErrNotInitialized` not `errNotInitialized`).
- Do NOT change function signatures. Only add the internal error detection.
- If SQLite returns a different error string for missing tables (e.g., sqlite3 vs go-sqlite3 driver), check both. Add a test that constructs a fresh DB and runs a query without CreateSchema to confirm the actual error string.

## 8. Report

```bash
cd /Users/dayna.blackwell/code/brewprune/.claude/worktrees/wave1-agent-h
git add internal/store/queries.go internal/store/db_test.go
git commit -m "wave1-agent-h: return ErrNotInitialized for uninitialized database queries"
```

Append to `docs/IMPL-audit-round6.md`:

```yaml
### Agent H — Completion Report
status: complete | partial | blocked
worktree: .claude/worktrees/wave1-agent-h
commit: {sha}
files_changed:
  - internal/store/queries.go
  - internal/store/db_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestListPackages_NoSchema_ReturnsErrNotInitialized
  - TestGetPackage_NoSchema_ReturnsErrNotInitialized
verification: PASS | FAIL
```

---

## Wave Execution Loop

After all Wave 1 agents complete:

1. Read each agent's completion report from their named section above.
2. Check for `status: partial` or `blocked` — resolve before merging.
3. Merge all agent worktrees:
   ```bash
   cd /Users/dayna.blackwell/code/brewprune
   git merge wave1-agent-a wave1-agent-b wave1-agent-c wave1-agent-d wave1-agent-e wave1-agent-f wave1-agent-g wave1-agent-h
   ```
4. Run post-merge verification:
   ```bash
   go build ./...
   go vet ./...
   go test ./... -timeout 180s
   ```
5. Fix any merge conflicts or compilation errors, then commit.
6. Build and smoke-test against r6 container (if still running):
   ```bash
   docker build -t brewprune-r6-fixed -f docker/Dockerfile.sandbox .
   docker run -d --name brewprune-r6-fixed --rm brewprune-r6-fixed sleep 3600
   docker exec brewprune-r6-fixed brewprune unused
   docker exec brewprune-r6-fixed brewprune doctor
   docker exec brewprune-r6-fixed brewprune remove --safe --medium --dry-run  # should error now
   ```

**Cascade candidates to verify post-merge:**
- `internal/store/queries.go` (Agent H): verify that callers in unused.go, remove.go, stats.go, explain.go all show the improved error message when the DB is not initialized.
- `internal/app/explain.go` (Agent G): verify visual consistency with `unused --verbose` output in the merged codebase.

---

## Status

- [ ] Wave 1 Agent A — remove.go: REMOVE-1, REMOVE-2, REMOVE-3, VISUAL-2
- [ ] Wave 1 Agent B — stats.go/watch.go: TRACK-2, TRACK-3, TRACK-4, EDGE-3
- [ ] Wave 1 Agent C — doctor.go: ONBOARD-2, ONBOARD-3, DOCTOR-1, DOCTOR-2, DOCTOR-3
- [ ] Wave 1 Agent D — quickstart.go: ONBOARD-1, HELP-4
- [ ] Wave 1 Agent E — undo.go: REMOVE-4, REMOVE-5
- [ ] Wave 1 Agent F — unused.go: UNUSED-2, UNUSED-3, UNUSED-4, UNUSED-5, VISUAL-1
- [ ] Wave 1 Agent G — explain.go: EXPLAIN-1, EXPLAIN-2, EXPLAIN-3
- [ ] Wave 1 Agent H — store/queries.go: EDGE-1
