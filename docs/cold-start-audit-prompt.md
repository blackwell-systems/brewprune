# Cold-Start UX Audit Prompt - Round 9

**Metadata:**
- Audit Date: 2026-03-01
- Tool Version: brewprune version dev (commit: unknown, built: unknown)
- Container: brewprune-r9
- Environment: Linux aarch64 (Ubuntu) with Homebrew (Linuxbrew)
- Binary location: /home/linuxbrew/.linuxbrew/bin/brewprune
- PATH in container: /home/linuxbrew/.linuxbrew/bin:/home/linuxbrew/.linuxbrew/sbin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

---

You are performing a UX audit of **brewprune** - a tool that tracks Homebrew package usage and provides heuristic-scored removal recommendations with automatic snapshots for easy rollback.

You are acting as a **new user** encountering this tool for the first time.

You have access to a Docker container called `brewprune-r9` with brewprune installed and the following packages available:

```
acl, bat, brotli, bzip2, ca-certificates, curl, cyrus-sasl, expat, fd,
gettext, git, jq, keyutils, krb5, libedit, libevent, libgit2, libidn2,
libnghttp2, libnghttp3, libngtcp2, libssh2, libunistring, libxcrypt,
libxml2, lz4, ncurses, oniguruma, openldap, openssl@3, pcre2, readline,
ripgrep, sqlite, tmux, utf8proc, util-linux, xz, zlib-ng-compat, zstd
```

Run ALL commands using: `docker exec brewprune-r9 <command>`

---

## Audit Areas

### Area 1: Discovery & Help System

Test how a new user discovers brewprune's capabilities from zero knowledge.

**Commands to run (in order):**

```
docker exec brewprune-r9 brewprune --help
docker exec brewprune-r9 brewprune --version
docker exec brewprune-r9 brewprune -v
docker exec brewprune-r9 brewprune help
docker exec brewprune-r9 brewprune
docker exec brewprune-r9 brewprune completion --help
docker exec brewprune-r9 brewprune doctor --help
docker exec brewprune-r9 brewprune explain --help
docker exec brewprune-r9 brewprune quickstart --help
docker exec brewprune-r9 brewprune remove --help
docker exec brewprune-r9 brewprune scan --help
docker exec brewprune-r9 brewprune stats --help
docker exec brewprune-r9 brewprune status --help
docker exec brewprune-r9 brewprune undo --help
docker exec brewprune-r9 brewprune unused --help
docker exec brewprune-r9 brewprune watch --help
```

**Reference: known help output structure**

`brewprune --help` shows a Quick Start section, feature list, examples, Available Commands list, and global flags. Subcommands include: completion, doctor, explain, help, quickstart, remove, scan, stats, status, undo, unused, watch. Global flag: `--db string` (database path).

`brewprune -v` and `--version` both print: `brewprune version dev (commit: unknown, built: unknown)`

**Evaluate:**
- Is the Quick Start section clear and actionable for a brand-new user?
- Does `brewprune` with no args produce useful output or just an error?
- Do help messages explain prerequisites and dependencies between commands?
- Are examples realistic and helpful?
- Is terminology consistent (daemon vs service, score vs confidence, tier vs level)?
- Are all flags documented clearly with defaults noted?
- Does `-v` behave as `--version` (not `--verbose`)? Is this surprising?

---

### Area 2: Setup & Onboarding (First-Run Experience)

Test the recommended path and the manual path for a new user setting up brewprune. Capture ~/.brewprune/ contents after each step to observe what files are created.

**Commands (Quickstart Path):**

```
docker exec brewprune-r9 brewprune quickstart
docker exec brewprune-r9 brewprune status
docker exec brewprune-r9 ls -la /root/.brewprune/
docker exec brewprune-r9 cat /root/.brewprune/watch.log
docker exec brewprune-r9 brewprune doctor
```

**Commands (Manual Path - run after resetting state):**

```
docker exec brewprune-r9 rm -rf /root/.brewprune
docker exec brewprune-r9 brewprune scan
docker exec brewprune-r9 brewprune status
docker exec brewprune-r9 brewprune watch --daemon
docker exec brewprune-r9 brewprune status
docker exec brewprune-r9 ls -la /root/.brewprune/
docker exec brewprune-r9 cat /root/.brewprune/watch.pid
docker exec brewprune-r9 brewprune doctor
```

**Evaluate:**
- Does quickstart provide clear feedback at each of its 4 steps (scan, PATH, daemon, self-test)?
- Does quickstart confirm success clearly at the end?
- Are errors or warnings actionable?
- Does `status` clearly distinguish between daemon running vs stopped?
- Does `doctor` provide helpful diagnostics with actionable next steps?
- Is the PATH setup requirement (~/.brewprune/bin) communicated clearly?
- Is there a meaningful difference in the experience between quickstart and manual paths?

---

### Area 3: Core Feature — Unused Package Discovery

Test the primary value proposition. Run with no usage data (fresh state after quickstart or scan) first, then with usage data after the daemon has run.

**Commands (Default and basic flags):**

```
docker exec brewprune-r9 brewprune unused
docker exec brewprune-r9 brewprune unused --all
docker exec brewprune-r9 brewprune unused --tier safe
docker exec brewprune-r9 brewprune unused --tier medium
docker exec brewprune-r9 brewprune unused --tier risky
```

**Commands (Score filtering):**

```
docker exec brewprune-r9 brewprune unused --min-score 70
docker exec brewprune-r9 brewprune unused --min-score 50
```

**Commands (Sort options):**

```
docker exec brewprune-r9 brewprune unused --sort score
docker exec brewprune-r9 brewprune unused --sort size
docker exec brewprune-r9 brewprune unused --sort age
```

**Commands (Other flags):**

```
docker exec brewprune-r9 brewprune unused --casks
docker exec brewprune-r9 brewprune unused --verbose
docker exec brewprune-r9 brewprune unused --tier safe --verbose
```

**Commands (Potentially conflicting flag combinations — note: per --help, --tier always shows the specified tier regardless of --all):**

```
docker exec brewprune-r9 brewprune unused --tier safe --all
docker exec brewprune-r9 brewprune unused --all --tier medium
```

**Evaluate:**
- Is the output table readable and well-aligned?
- Are column headers clear and consistently named?
- Does the tool explain when usage data is missing (no daemon history)?
- Does the warning banner about missing usage data appear when expected?
- Are tier labels (safe/medium/risky) visually distinct (e.g., color-coded)?
- Does `--verbose` add meaningful clarity or overwhelm?
- Is the default view (no flags) appropriate for new users — does it include safe + medium but hide risky?
- When `--tier safe --all` is used, does one flag clearly win? Is the behavior explained?
- Is the `--sort size` option functional (does size data exist in container)?
- Does `--casks` produce different output in a Linux/Linuxbrew environment with no casks?

---

### Area 4: Data Collection & Tracking

Test the usage tracking mechanism that feeds the scoring system. This requires starting the daemon, exercising shim-wrapped commands, waiting for the 30-second polling cycle, and verifying the pipeline end-to-end.

**Commands (Start daemon and verify):**

```
docker exec brewprune-r9 brewprune watch --daemon
docker exec brewprune-r9 cat /root/.brewprune/watch.pid
docker exec brewprune-r9 brewprune status
```

**Commands (Generate usage via shimmed commands):**

```
docker exec brewprune-r9 /root/.brewprune/bin/git --version
docker exec brewprune-r9 /root/.brewprune/bin/jq --version
docker exec brewprune-r9 /root/.brewprune/bin/bat --version
docker exec brewprune-r9 /root/.brewprune/bin/fd --version
docker exec brewprune-r9 /root/.brewprune/bin/rg --version
```

**Commands (Wait for daemon polling cycle and verify):**

```
docker exec brewprune-r9 sleep 35
docker exec brewprune-r9 cat /root/.brewprune/usage.log
docker exec brewprune-r9 brewprune status
docker exec brewprune-r9 brewprune stats
docker exec brewprune-r9 brewprune stats --days 1
docker exec brewprune-r9 brewprune stats --days 7
docker exec brewprune-r9 brewprune stats --days 90
docker exec brewprune-r9 brewprune stats --package git
docker exec brewprune-r9 brewprune stats --package jq
docker exec brewprune-r9 brewprune stats --all
```

**Commands (Stop daemon and verify):**

```
docker exec brewprune-r9 brewprune watch --stop
docker exec brewprune-r9 brewprune status
docker exec brewprune-r9 ls -la /root/.brewprune/
```

**Evaluate:**
- Does `status` clearly show daemon PID and state before and after stopping?
- Are usage events recorded in usage.log after running shimmed commands?
- Does `stats` output explain what the data means for a new user?
- Is the 30-second polling interval explained anywhere in help or output?
- Does `watch --stop` provide confirmation feedback?
- After stopping daemon, does `status` clearly indicate it is stopped?
- Does `stats --all` include packages with no usage? Is the output useful?
- Note: shims are in ~/.brewprune/bin — running `git` directly may not use the shim if ~/.brewprune/bin is not in PATH inside docker exec context.

---

### Area 5: Package Explanation & Detail View

Test the per-package drill-down feature with valid installed packages, invalid packages, and no arguments.

**Commands (Valid packages from container's brew list):**

```
docker exec brewprune-r9 brewprune explain git
docker exec brewprune-r9 brewprune explain jq
docker exec brewprune-r9 brewprune explain bat
docker exec brewprune-r9 brewprune explain openssl@3
docker exec brewprune-r9 brewprune explain curl
```

**Commands (Invalid and edge cases):**

```
docker exec brewprune-r9 brewprune explain nonexistent-package
docker exec brewprune-r9 brewprune explain
docker exec brewprune-r9 brewprune explain --help
```

**Evaluate:**
- Does `explain` show a clear breakdown of all 4 score components: usage (40pts), dependencies (30pts), age (20pts), type (10pts)?
- Is the total score prominently displayed?
- Is the tier classification (safe/medium/risky) shown and consistent with `unused` output?
- Is the reasoning written in plain language a new user would understand?
- Are dependency lists shown (which packages depend on this one)?
- Are error messages helpful for invalid packages (does it suggest checking `brewprune scan`)?
- Does `brewprune explain` with no args give a clear usage error?
- Does output for `openssl@3` reflect the cap at 70 mentioned in `unused --help`?
- Does the scoring breakdown match what `unused --verbose` shows?

---

### Area 6: Diagnostics

Test the health check and diagnostic system under different system states.

**Commands (After quickstart — healthy state):**

```
docker exec brewprune-r9 brewprune doctor
```

**Commands (With stopped daemon — degraded state):**

```
docker exec brewprune-r9 brewprune watch --stop
docker exec brewprune-r9 brewprune doctor
```

**Commands (Before any setup — blank state):**

```
docker exec brewprune-r9 rm -rf /root/.brewprune
docker exec brewprune-r9 brewprune doctor
```

**Evaluate:**
- Does `doctor` clearly identify issues with specific check names?
- Are recommendations actionable (does it say exactly what command to run next)?
- Does it check all 4 expected components: database, daemon, usage events, next steps?
- Is output color-coded to distinguish passing vs failing checks?
- Does `doctor` degrade gracefully when ~/.brewprune does not exist at all?
- Is the output format consistent across healthy, degraded, and blank states?
- Are there any checks that seem missing (e.g., PATH check for ~/.brewprune/bin)?

---

### Area 7: Destructive Operations (Remove & Undo)

Test package removal and rollback features with safety mechanisms. Always run dry-run first.

**Commands (Dry-run previews — safe to run anytime):**

```
docker exec brewprune-r9 brewprune remove --safe --dry-run
docker exec brewprune-r9 brewprune remove --medium --dry-run
docker exec brewprune-r9 brewprune remove --risky --dry-run
docker exec brewprune-r9 brewprune remove --tier safe --dry-run
docker exec brewprune-r9 brewprune remove bat fd --dry-run
```

**Commands (Check snapshot list before any removal):**

```
docker exec brewprune-r9 brewprune undo --list
```

**Commands (Actual removal — safe tier only with --yes):**

```
docker exec brewprune-r9 brewprune remove --safe --yes
docker exec brewprune-r9 brewprune undo --list
docker exec brewprune-r9 brewprune status
```

**Commands (Rollback):**

```
docker exec brewprune-r9 brewprune undo latest --yes
docker exec brewprune-r9 brewprune undo --list
```

**Commands (Invalid and conflicting operations):**

```
docker exec brewprune-r9 brewprune remove nonexistent-package
docker exec brewprune-r9 brewprune remove --safe --medium
docker exec brewprune-r9 brewprune undo 999
docker exec brewprune-r9 brewprune undo
```

**Evaluate:**
- Does `--dry-run` clearly label output as a preview (not real removal)?
- Is the list of packages to be removed shown before confirmation?
- Does snapshot creation produce visible feedback (snapshot ID shown)?
- Is `undo --list` output clear enough to know which snapshot to restore?
- Does `undo latest --yes` clearly confirm what was restored?
- Are error messages for conflicting flags (`--safe --medium`) specific and helpful?
- Does `remove nonexistent-package` fail gracefully with a clear error?
- Does `undo 999` fail clearly when snapshot ID does not exist?
- Does `undo` with no args show usage rather than silently succeed or crash?
- Is the `--no-snapshot` danger warning prominent enough?

---

### Area 8: Edge Cases & Error Handling

Test boundary conditions and invalid input handling across all commands.

**Commands (No-argument invocations):**

```
docker exec brewprune-r9 brewprune
docker exec brewprune-r9 brewprune unused
docker exec brewprune-r9 brewprune stats
docker exec brewprune-r9 brewprune remove
docker exec brewprune-r9 brewprune explain
docker exec brewprune-r9 brewprune undo
```

**Commands (Unknown subcommands):**

```
docker exec brewprune-r9 brewprune blorp
docker exec brewprune-r9 brewprune list
docker exec brewprune-r9 brewprune prune
```

**Commands (Invalid flag values):**

```
docker exec brewprune-r9 brewprune unused --invalid-flag
docker exec brewprune-r9 brewprune unused --tier invalid
docker exec brewprune-r9 brewprune unused --min-score 200
docker exec brewprune-r9 brewprune unused --sort invalid
docker exec brewprune-r9 brewprune stats --days -1
docker exec brewprune-r9 brewprune stats --days abc
```

**Commands (Conflicting flags):**

```
docker exec brewprune-r9 brewprune remove --safe --medium --risky
docker exec brewprune-r9 brewprune remove --safe --tier medium
docker exec brewprune-r9 brewprune unused --tier safe --all
docker exec brewprune-r9 brewprune watch --daemon --stop
```

**Commands (Missing prerequisites — after rm -rf):**

```
docker exec brewprune-r9 rm -rf /root/.brewprune
docker exec brewprune-r9 brewprune unused
docker exec brewprune-r9 brewprune stats
docker exec brewprune-r9 brewprune remove --safe
docker exec brewprune-r9 brewprune status
```

**Evaluate:**
- Are error messages specific and actionable (not just "error")?
- Do unknown subcommands suggest `brewprune --help` or list valid commands?
- Are invalid enum values shown with the list of valid options?
- Does the tool gracefully handle missing database (no crash, clear message)?
- Are validation errors shown before any destructive action begins?
- Does `remove` with no tier flag and no packages produce a clear usage error?
- Does `watch --daemon --stop` detect the conflict and fail clearly?
- Does `unused --tier invalid` list the valid tier names?

---

### Area 9: Output Quality & Visual Design

Review the overall presentation, formatting, and consistency of all output modes.

**Commands (Capture all output modes for review):**

```
docker exec brewprune-r9 brewprune --help
docker exec brewprune-r9 brewprune unused
docker exec brewprune-r9 brewprune unused --all
docker exec brewprune-r9 brewprune unused --tier safe --verbose
docker exec brewprune-r9 brewprune status
docker exec brewprune-r9 brewprune stats
docker exec brewprune-r9 brewprune stats --all
docker exec brewprune-r9 brewprune explain git
docker exec brewprune-r9 brewprune explain openssl@3
docker exec brewprune-r9 brewprune doctor
docker exec brewprune-r9 brewprune undo --list
docker exec brewprune-r9 brewprune remove --safe --dry-run
```

**Evaluate:**
- **Tables:** Are columns aligned? Do headers stand out? Is data truncated gracefully?
- **Colors:** Are colors used consistently? Are tier colors intuitive (green=safe, yellow=medium, red=risky)?
- **Formatting:** Is bold/italic/underline used appropriately? Are lists and bullets clear?
- **Spacing:** Is whitespace used effectively? Are sections visually separated?
- **Terminology:** Is language consistent across all commands?
  - daemon vs service vs background process
  - score vs confidence vs confidence score
  - snapshot vs backup vs rollback point
  - tier vs level vs category
- **Symbols:** Are emoji or unicode characters (checkmarks, bullets, etc.) used? Are they helpful or distracting?
- **Context lines:** Do outputs include helpful counts (e.g., "Showing 10 of 40 packages")?
- **Errors vs Warnings:** Are severity levels visually distinct (e.g., red for errors, yellow for warnings)?
- **Progress indicators:** Are there spinners or progress messages for slow operations (scan, remove)?

---

## Instructions for Audit Execution

Run **ALL** commands in sequence. Do not skip areas.

For each command, note:
1. **Exact output** (copy full text, including any headers, footers, banners)
2. **Exit code** (0 for success, non-zero for error — capture with `; echo "Exit: $?"`)
3. **Color usage** (describe precisely: "package names in bold white", "tier label 'safe' in green", "warning in yellow")
4. **Timing** (note if any command is slow, shows no progress, or has an unexpected delay)
5. **Behavior** (e.g., "prompts for confirmation", "writes to ~/.brewprune/usage.log", "daemon starts silently")

Describe visual formatting precisely:
- "Table with 4 columns: Package | Score | Tier | Last Used"
- "Tier column shows 'safe' in green, 'medium' in yellow, 'risky' in red"
- "Warning banner with yellow background: 'No usage data — showing heuristic scores only'"
- "Error messages prefixed with 'Error:' in red"

---

## Findings Format

For each issue found, use this template:

### [AREA N] Finding Title

- **Severity**: UX-critical / UX-improvement / UX-polish
- **What happens**: What the user actually sees
- **Expected**: What better behavior looks like
- **Repro**: Exact command(s)

### Severity Guide

- **UX-critical**: Broken, misleading, or completely missing behavior that blocks the user or causes data loss risk
- **UX-improvement**: Confusing or unhelpful behavior that a user would notice and dislike
- **UX-polish**: Minor friction, inconsistency, or missed opportunity for clarity

---

## Report Structure

Your final report must include:

1. **Summary Table** at the top:
   ```
   | Severity       | Count |
   |----------------|-------|
   | UX-critical    | X     |
   | UX-improvement | Y     |
   | UX-polish      | Z     |
   | **Total**      | N     |
   ```

2. **Findings by Area** (one section per audit area, use the area names exactly):
   - Area 1: Discovery & Help System
   - Area 2: Setup & Onboarding
   - Area 3: Core Feature — Unused Package Discovery
   - Area 4: Data Collection & Tracking
   - Area 5: Package Explanation & Detail View
   - Area 6: Diagnostics
   - Area 7: Destructive Operations
   - Area 8: Edge Cases & Error Handling
   - Area 9: Output Quality & Visual Design

3. **Positive Observations** (encouraged):
   - Highlight what works well
   - Note improvements vs previous rounds if applicable

4. **Recommendations Summary**:
   - High-level themes or patterns
   - Suggested priorities for the next fix round

---

## Final Steps

Write the complete audit report to:

**`/Users/dayna.blackwell/code/brewprune/docs/cold-start-audit-r9.md`**

Use the **Write** tool to create this file.

---

## IMPORTANT

- Run ALL commands via `docker exec brewprune-r9 <command>` — never run brewprune directly on the host
- Capture exact error messages and exit codes for every command
- Note timing (e.g., "took 3 seconds", "no progress indicator for 45 seconds")
- Note the shim path: shims are in `/root/.brewprune/bin/` inside the container — when testing usage tracking (Area 4), invoke via the full shim path since ~/.brewprune/bin may not be in the docker exec PATH
- Be thorough — this audit drives the next round of fixes
