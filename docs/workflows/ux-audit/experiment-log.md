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

- **Status:** Running (2 parallel agents)
- **Agent A:** scan.go, quickstart.go, watch.go, status.go
- **Agent B:** explain.go, doctor.go, stats.go, undo.go

### Phase 4 — Wave 2

*(pending Wave 1)*

---

## Outcomes to Track

- [ ] Did the scout correctly identify all file ownership conflicts?
- [ ] Was Wave 0 (score logic) treated as a true prerequisite?
- [ ] Did parallel agents in Wave 1 complete without merge conflicts?
- [ ] Did post-merge verification catch anything individual agents missed?
- [ ] Total wall-clock time vs estimated sequential time
- [ ] Any agent failures or restarts required

---

## Retrospective

*(filled in after completion)*
