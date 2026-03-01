# IMPL: Round 9 Audit Fixes

**Feature slug:** `audit-round9`
**Audit source:** `docs/cold-start-audit-r9.md`
**Findings addressed:** 15 of 18 (3 UX-critical, 8 UX-improvement, 4 UX-polish)

---

## Suitability Assessment

**Verdict:** SUITABLE WITH CAVEATS

Two investigation-first items exist: a post-undo segfault (exit 139, no output, UX-critical) and
a shim event resolution failure for git/jq (UX-improvement). Both are assigned to Wave 0 agents
with fully disjoint file ownership. Wave 0 runs two parallel investigation agents rather than a
single solo agent — this is a deliberate deviation from the strict convention, justified by the
fact that the two investigations operate on completely separate packages
(`internal/app/{explain,stats}.go` vs `internal/watcher/shim_processor.go`) and neither result
gates the other. Wave 1 is not blocked by either Wave 0 agent — the 6 Wave 1 files are all
independent of both investigation areas.

Remaining 13 items are straightforward text/logic changes with known file assignments. Each
agent owns 1–2 files with no cross-agent dependencies.

**Estimated times:**
- Scout phase: ~30 min (deep codebase read + IMPL doc)
- Agent execution: ~80 min (8 agents × ~10 min avg, accounting for parallelism → ~35 min wall time for Wave 0 duo + Wave 1 sextet)
- Merge & verification: ~10 min
- Total SAW time: ~75 min

- Sequential baseline: ~150 min (8 agents × ~20 min avg including build/test cycle)
- Time savings: ~75 min (~50% faster)

**Recommendation:** Clear speedup. Go build+test cycle is ~10–15s; with 8 agents most of that
is parallelized. Proceed.

---

## Known Issues

- `TestDoctorHelpIncludesFixNote` — Hangs (tries to execute test binary as CLI)
  - Status: Pre-existing, unrelated to this work
  - Workaround: Skip with `-skip 'TestDoctorHelpIncludesFixNote'`

---

## Pre-Implementation Scan

```
Pre-implementation scan results:
- Total items: 15 findings
- Already implemented: 0 items
- Partially implemented: 2 items
  - Quickstart: "Setup complete — one step remains:" added, but PATH warning still appears AFTER summary text (not before)
  - Unused casks: early exit for empty casks is AFTER checkUsageWarning (warning still prints first)
- To-do: 13 items

Agent adjustments:
- Agents D, E: "complete the partial implementation"
- Agents A, B, C, F, G, H: proceed as planned (to-do)
```

---

## Dependency Graph

All 8 agents own disjoint files. No cross-agent type or function dependencies exist.

```
Wave 0:  [A]  [B]   <- two parallel investigation-first agents
              | (A+B complete — no downstream dependency, Wave 1 proceeds independently)
Wave 1: [C] [D] [E] [F] [G] [H]  <- 6 parallel agents
```

**Root nodes (no dependencies):** All Wave 0 and Wave 1 files are leaves — no new types or
functions cross agent boundaries.

**Cascade candidates:** None. All changes are behavior-only (output text, error handling,
conditional logic). No type renames, no interface changes.

---

## Interface Contracts

No new cross-agent interfaces. All changes are internal to each agent's owned files.

---

## File Ownership

| File | Agent | Wave | Notes |
|------|-------|------|-------|
| `internal/app/explain.go` | A | 0 | segfault fix + protected count + recommendation format |
| `internal/app/stats.go` | A | 0 | segfault fix + error chain |
| `internal/watcher/shim_processor.go` | B | 0 | git/jq resolution |
| `internal/watcher/shim_processor_test.go` | B | 0 | new/updated tests |
| `internal/app/doctor.go` | C | 1 | pipeline WARN + active-PATH check |
| `internal/app/doctor_test.go` | C | 1 | updated tests |
| `internal/app/quickstart.go` | D | 1 | PATH warning ordering + self-test duration |
| `internal/app/quickstart_test.go` | D | 1 | updated tests |
| `internal/app/unused.go` | E | 1 | sort footer + casks warning skip |
| `internal/app/unused_test.go` | E | 1 | updated tests |
| `internal/app/remove.go` | F | 1 | nonexistent error + error chain |
| `internal/app/remove_test.go` | F | 1 | updated tests |
| `internal/app/status.go` | G | 1 | shims "0 commands" label |
| `internal/app/status_test.go` | G | 1 | updated tests |
| `internal/app/undo.go` | H | 1 | progress rendering + stale DB warning |
| `internal/app/undo_test.go` | H | 1 | updated tests |

*Agent A may also need to touch `internal/app/explain_test.go` and `internal/app/stats_test.go`.*

---

## Wave Structure

```
Wave 0:  [A] [B]               <- 2 parallel investigation-first agents (disjoint files)
              |
Wave 1: [C] [D] [E] [F] [G] [H]  <- 6 parallel agents
```

Wave 0 does NOT gate Wave 1 — the 6 Wave 1 files are independent of both investigation areas.
Wave 1 may launch as soon as Wave 0 agents commit their findings (or even in parallel, since
Wave 1 files do not depend on Wave 0 outputs).

---

## Agent Prompts

Full prompts are in per-agent files:

- [Agent A — segfault + explain fixes](IMPL-audit-round9-agents/agent-a.md)
- [Agent B — shim processor git/jq resolution](IMPL-audit-round9-agents/agent-b.md)
- [Agent C — doctor pipeline WARN + PATH check](IMPL-audit-round9-agents/agent-c.md)
- [Agent D — quickstart ordering + self-test duration](IMPL-audit-round9-agents/agent-d.md)
- [Agent E — unused sort footer + casks banner](IMPL-audit-round9-agents/agent-e.md)
- [Agent F — remove error + error chain](IMPL-audit-round9-agents/agent-f.md)
- [Agent G — status shims label](IMPL-audit-round9-agents/agent-g.md)
- [Agent H — undo progress + stale DB warning](IMPL-audit-round9-agents/agent-h.md)

---

## Wave Execution Loop

After each wave completes:
1. Read each agent's completion report from `### Agent {letter} — Completion Report` sections appended to their per-agent files.
2. Merge all agent worktrees back into the main branch.
3. Run: `cd /Users/dayna.blackwell/code/brewprune && go build ./... && go vet ./... && go test ./... -skip 'TestDoctorHelpIncludesFixNote'`
4. Fix any compiler errors or integration issues.
5. Update the Status section below (tick checkboxes).
6. Commit the wave's changes.
7. Launch the next wave.

---

## Status

- [ ] Wave 0 Agent A — segfault investigation + fix (explain.go, stats.go)
- [ ] Wave 0 Agent B — shim processor git/jq resolution (shim_processor.go)
- [ ] Wave 1 Agent C — doctor pipeline WARN + active-PATH check (doctor.go)
- [ ] Wave 1 Agent D — quickstart PATH warning ordering + self-test duration (quickstart.go)
- [ ] Wave 1 Agent E — unused sort footer + casks banner skip (unused.go)
- [ ] Wave 1 Agent F — remove nonexistent error + error chain (remove.go)
- [ ] Wave 1 Agent G — status shims "0 commands" label (status.go)
- [ ] Wave 1 Agent H — undo progress rendering + stale DB warning (undo.go)
