# SAW Pattern Experiment Log — UX Audit Fixes

Tracking the use of Scout-and-Wave (SAW) to coordinate parallel agents fixing 21 UX issues found in a cold-start audit.

---

## Context

- **Tool:** brewprune v0.2.2
- **Trigger:** Cold-start audit run by a background agent in `bp-sandbox` Docker container
- **Findings:** 21 issues — 5 UX-critical, 9 UX-improvement, 7 UX-polish
- **Full audit:** `docs/ux-audit.md`
- **Date started:** 2026-02-28

---

## Why This Is an Interesting SAW Test Case

Previous SAW usage on this project was a single agent, single wave (file coupling prevented parallelism). This run is the first genuine multi-wave, multi-agent execution:

- **Wave 0** has a prerequisite that gates all downstream work (inverted score logic in `confidence.go`)
- **Waves 1 and 2** have genuine parallelism across ~10 files
- **21 findings** span multiple subsystems (scoring, output, CLI commands, rendering)
- File ownership conflicts are non-trivial — `table.go` is touched by both rendering and command fixes

This tests whether the scout correctly resolves ownership conflicts and whether the wave structure holds under real complexity.

---

## Hypothesis

The scout will:
1. Confirm Wave 0 as a prerequisite (score logic inversion blocks meaningful testing of everything else)
2. Identify `table.go` as a shared-file conflict and resolve it by assigning it to a single agent
3. Produce 4-5 agent prompts across 2-3 waves
4. Enable agents to work without stepping on each other

---

## Execution Log

### Phase 1 — Scout

- **Status:** Complete
- **Agent ID:** ad68585ab3438ac5a
- **Output:** `docs/IMPL-ux-audit-fixes.md`
- **Completed:** 2026-02-28
- **Tool uses:** 57 | **Tokens:** 99,002 | **Duration:** ~6.7 min

**Key findings:**
- Planned wave structure confirmed valid — no same-wave file collisions
- Score logic inversion confirmed: Wave 0 is a true prerequisite
- One hidden coupling found: Agent C (Wave 2) adds `IsColorEnabled()` that Agent D needs in the same wave — documented stub pattern in IMPL doc
- `remove.go` `LastUsed` bug traced to single line: `getNeverTime()` hard-coded where a store lookup should be
- `confidence_test.go` encodes the broken behavior — tests must be updated alongside the fix

### Scout Output — Wave Structure

```
Wave 0 ── Agent 0: confidence.go (score logic inversion)
              │
Wave 1 ── Agent A: scan.go, quickstart.go, watch.go, status.go
          Agent B: explain.go, doctor.go, stats.go, undo.go
              │
Wave 2 ── Agent C: output/table.go, output/progress.go
          Agent D: unused.go, remove.go, root.go
```

### Phase 2 — Wave 0

- **Status:** Complete ✓
- **Agent:** Agent 0 — fix inverted score logic
- **Files:** `internal/analyzer/confidence.go`, `internal/analyzer/confidence_test.go`
- **Commit:** `2f3b6a7`
- **Tool uses:** 31 | **Tokens:** 52,498 | **Duration:** ~4.7 min

**Result:** Score logic inverted correctly. One cascade caught at verification:
- `recommendations.go` had the same pre-inversion `UsageScore >= 30` check — fixed inline
- `recommendations_test.go` had two tests encoding old behavior — updated
- `TestGetRecommendations_NoSafePackages` renamed to `TestGetRecommendations_NeverUsedPackageIsSafe`
- All 16 confidence tests + recommendations tests pass

### Phase 3 — Wave 1

- **Status:** Complete ✓
- **Commit:** `3a613eb`
- **Agent A:** scan.go, quickstart.go, watch.go, status.go — 86 tool uses, ~6.9 min
- **Agent B:** explain.go, doctor.go, stats.go, undo.go — 110 tool uses, ~10.5 min

**Post-merge verification:** All tests passed clean. No inter-agent conflicts.

**Observation:** Agent A reported finding Agent B's files modified in its worktree and restoring them to HEAD. Worktree isolation appears incomplete — both agents may have been working against the same filesystem. Did not cause test failures but worth investigating in the pattern.

**Contract changes for Wave 2:**
- `watch.go`: `startWatchDaemon` now returns `nil` when already running (was error)
- `doctor.go`: warning-only path calls `os.Exit(2)` — untestable without subprocess pattern
- `isatty` is now a direct dep in `stats.go` and `scan.go`

### Phase 4 — Wave 2

- **Status:** Complete ✓
- **Commit:** `7a46131`
- **Agent C:** output/table.go, output/progress.go
- **Agent D:** unused.go, remove.go, root.go

**Agent C results:**
- `IsColorEnabled()` added (checks `NO_COLOR` + `isatty`); ANSI color gating across all render functions
- Version column removed from `RenderPackageTable`
- `formatTierLabel` updated: `"SAFE"→"✓ safe"`, `"MEDIUM"→"~ review"`, risky stays `"✗ keep"`
- Secondary sort by `LastUsed` in `RenderUsageTable`
- `progress.go`: non-TTY awareness — spinner skips goroutine, only prints at 100% completion

**Agent D results:**
- `showRiskyImplicit`: when no usage events and no explicit `--tier`/`--all`, show risky with warning banner
- `--casks` with 0 casks: friendly `"No casks installed."` message
- `--tier <value>` flag alias added with validation
- `getLastUsed(st, pkg)` replacing `getNeverTime()` hard-code in `displayConfidenceScores`

**Post-merge verification:** All 11 packages passed (`go clean -testcache && go test ./...`). No inter-agent conflicts.

**C→D coupling outcome:** Agent C renamed `"SAFE"` → `"✓ safe"`. Agent D's tests did not check format strings directly, so no conflict. The stub pattern documented in the IMPL doc was not needed — disjoint file ownership and behavior-focused tests absorbed the change naturally.

---

## Outcomes to Track

- [x] Did the scout correctly identify all file ownership conflicts?
  — **Yes.** `table.go` correctly assigned to Agent C only. No same-wave collisions occurred.
- [x] Was Wave 0 (score logic) treated as a true prerequisite?
  — **Yes.** Wave 0 was the only wave run before Wave 1 launched. Score inversion was validated before any downstream agents touched usage-dependent code.
- [x] Did parallel agents in Wave 1 complete without merge conflicts?
  — **Yes**, but with a caveat: Agent A reported finding Agent B's files in its worktree and restored them to HEAD. Disjoint file ownership prevented actual failures, but worktree isolation appears incomplete.
- [x] Did post-merge verification catch anything individual agents missed?
  — **Yes (Wave 0).** `recommendations.go` had the same inverted `UsageScore >= 30` check that Agent 0 didn't touch. Caught at the orchestrator verification gate and fixed inline.
- [x] Total wall-clock time vs estimated sequential time
  — **~29 min wall-clock** (Scout 6.7m + Wave 0 4.7m + Wave 1 ~10.5m parallel + Wave 2 ~7m parallel). Sequential estimate: ~35–40 min. Modest gain because Wave 0 gated everything and could not be parallelized.
- [x] Any agent failures or restarts required
  — **No agent failures.** Three orchestrator-level restarts were required due to unrelated Claude Code permission/hook setup issues at session start.

---

## Retrospective

### What worked well

**Wave structure held.** The scout's file ownership table was accurate — no same-wave file collisions across 10 files and 5 agents. The `table.go` conflict was correctly resolved by assignment to Agent C, and Agent D never touched it.

**Post-merge verification gate caught a real bug.** `recommendations.go` was outside all agent scopes but shared the same inverted score logic. Running `go test ./...` before committing Wave 0 surfaced this cascade. The gate was worth the overhead.

**Disjoint file ownership absorbed a format-string change.** Agent C changed tier label format strings. Agent D's tests checked behavior (tier assignment, score thresholds), not rendered strings. The C→D coupling the scout flagged as a risk resolved itself — no stub coordination needed.

**Parallelism across Wave 1 and Wave 2 was real.** Both waves had two genuinely independent agents with no file overlap. Each wave committed as a single merged result, and verification was clean.

### What to improve

**Worktree isolation is unreliable.** Agent A found Agent B's files modified in its worktree during Wave 1. The `isolation: "worktree"` parameter in the Task tool appears to land changes back in the main working tree when the worktree closes — so concurrent agents writing to separate worktrees are not truly isolated. This is a SAW pattern risk: it worked here because file ownership was fully disjoint, but overlapping files in a future wave would cause silent corruption.

**Orchestrator role is heavier than expected.** The orchestrator (this session) had to: read agent reports, apply cascade fixes, update the IMPL doc, run verification, and re-read the 59KB IMPL doc before each wave. This is non-trivial context load. Two improvements would help:
1. Agents should write to their own named section in the IMPL doc rather than only reading it
2. IMPL doc should be split: a small meta/index file + per-agent detail files (avoids loading 59KB for every orchestrator turn)

**IMPL doc size scales poorly.** At 59KB for 21 findings across 5 agents, the full IMPL doc is expensive to include in every context. For runs with more findings or more agents, this becomes a significant token cost per orchestrator turn.

**Wave 0 prerequisite limits wall-clock gains.** The score logic inversion had to complete before any Wave 1 agent could meaningfully test their changes. This serialized the first ~11 minutes. When a prerequisite wave exists, the theoretical parallelism gains of SAW are bounded by that serial prefix.

### Pattern assessment

SAW worked correctly for this use case. The scout correctly resolved ownership conflicts, waves executed cleanly, and post-merge verification caught the one inter-file cascade that wasn't in scope. The main improvement areas (worktree isolation, orchestrator overhead, IMPL doc size) are pattern-level concerns rather than failures — the pattern held under real complexity.
