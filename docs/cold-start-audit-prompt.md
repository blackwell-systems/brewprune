# Cold-Start UX Audit Report - Round 6

**Metadata:**
- Audit Date: 2026-02-28
- Tool Version: brewprune version dev (commit: unknown, built: unknown)
- Container: brewprune-r6
- Environment: Ubuntu 22.04 with Homebrew

---

You are performing a UX audit of **brewprune** - a tool that tracks Homebrew package usage and provides heuristic-scored removal recommendations with automatic snapshots for easy rollback.

You are acting as a **new user** encountering this tool for the first time.

You have access to a Docker container called `brewprune-r6` with brewprune installed and the following packages available:

```
acl, bat, brotli, bzip2, ca-certificates, curl, cyrus-sasl, expat, fd,
gettext, git, jq, keyutils, krb5, libedit, libevent, libgit2, libidn2,
libnghttp2, libnghttp3, libngtcp2, libssh2, libunistring, libxcrypt,
libxml2, lz4, ncurses, oniguruma, openldap, openssl@3, pcre2, readline,
ripgrep, sqlite, tmux, utf8proc, util-linux, xz, zlib-ng-compat, zstd
```

Run all commands using: `docker exec brewprune-r6 <command>`

---

## Audit Areas

### 1. Discovery & Help System

Test how a new user discovers brewprune's capabilities.

**Commands:**
- `docker exec brewprune-r6 brewprune --help`
- `docker exec brewprune-r6 brewprune --version`
- `docker exec brewprune-r6 brewprune -v`
- `docker exec brewprune-r6 brewprune help`
- `docker exec brewprune-r6 brewprune`
- `docker exec brewprune-r6 brewprune completion --help`
- `docker exec brewprune-r6 brewprune doctor --help`
- `docker exec brewprune-r6 brewprune explain --help`
- `docker exec brewprune-r6 brewprune quickstart --help`
- `docker exec brewprune-r6 brewprune remove --help`
- `docker exec brewprune-r6 brewprune scan --help`
- `docker exec brewprune-r6 brewprune stats --help`
- `docker exec brewprune-r6 brewprune status --help`
- `docker exec brewprune-r6 brewprune undo --help`
- `docker exec brewprune-r6 brewprune unused --help`
- `docker exec brewprune-r6 brewprune watch --help`

**Evaluate:**
- Is the Quick Start section clear and actionable?
- Do help messages explain prerequisites and dependencies between commands?
- Are examples realistic and helpful?
- Is terminology consistent (daemon vs service, score vs confidence, etc)?
- Are flags documented clearly?

---

### 2. Setup & Onboarding (First-Run Experience)

Test the recommended path for a new user setting up brewprune.

**Commands (Quickstart Path):**
- `docker exec brewprune-r6 brewprune quickstart`
- `docker exec brewprune-r6 brewprune status`
- `docker exec brewprune-r6 cat ~/.brewprune/watch.log`
- `docker exec brewprune-r6 ls -la ~/.brewprune/`
- `docker exec brewprune-r6 brewprune doctor`

**Commands (Manual Path):**
- `docker exec brewprune-r6 brewprune scan`
- `docker exec brewprune-r6 brewprune watch --daemon`
- `docker exec brewprune-r6 brewprune status`
- `docker exec brewprune-r6 brewprune doctor`

**Evaluate:**
- Does quickstart provide clear feedback at each step?
- Are errors or warnings actionable?
- Does status clearly show daemon is running?
- Does doctor provide helpful diagnostics?
- Is the PATH setup requirement clear?

---

### 3. Core Feature: Unused Package Discovery

Test the primary value proposition - identifying unused packages.

**Commands (No usage data yet):**
- `docker exec brewprune-r6 brewprune unused`
- `docker exec brewprune-r6 brewprune unused --all`
- `docker exec brewprune-r6 brewprune unused --tier safe`
- `docker exec brewprune-r6 brewprune unused --tier medium`
- `docker exec brewprune-r6 brewprune unused --tier risky`
- `docker exec brewprune-r6 brewprune unused --min-score 70`
- `docker exec brewprune-r6 brewprune unused --min-score 50`
- `docker exec brewprune-r6 brewprune unused --sort score`
- `docker exec brewprune-r6 brewprune unused --sort size`
- `docker exec brewprune-r6 brewprune unused --sort age`
- `docker exec brewprune-r6 brewprune unused --casks`
- `docker exec brewprune-r6 brewprune unused --verbose`
- `docker exec brewprune-r6 brewprune unused --tier safe --verbose`

**Evaluate:**
- Is the output table readable and well-aligned?
- Are column headers clear?
- Does the tool explain when usage data is missing?
- Are tier labels (safe/medium/risky) color-coded consistently?
- Does --verbose add clarity or overwhelm?
- Is the default view (no flags) appropriate for new users?
- Does the warning banner about missing usage data appear when appropriate?

---

### 4. Data Collection & Tracking

Test the usage tracking mechanism that feeds the scoring system.

**Commands (Setup):**
- `docker exec brewprune-r6 brewprune status`
- `docker exec brewprune-r6 brewprune watch --daemon`
- `docker exec brewprune-r6 cat ~/.brewprune/watch.pid`
- `docker exec brewprune-r6 ps aux | grep brewprune`

**Commands (Generate usage):**
- `docker exec brewprune-r6 git --version`
- `docker exec brewprune-r6 jq --version`
- `docker exec brewprune-r6 bat --version`
- `docker exec brewprune-r6 fd --version`
- `docker exec brewprune-r6 ripgrep --version`
- `docker exec brewprune-r6 sleep 35`
- `docker exec brewprune-r6 cat ~/.brewprune/usage.log`

**Commands (Verify tracking):**
- `docker exec brewprune-r6 brewprune status`
- `docker exec brewprune-r6 brewprune stats`
- `docker exec brewprune-r6 brewprune stats --days 1`
- `docker exec brewprune-r6 brewprune stats --package git`
- `docker exec brewprune-r6 brewprune stats --all`

**Commands (Stop daemon):**
- `docker exec brewprune-r6 brewprune watch --stop`
- `docker exec brewprune-r6 brewprune status`

**Evaluate:**
- Does status clearly show daemon state (running/stopped)?
- Are usage events being recorded and visible?
- Does stats output explain what the data means?
- Is the 30-second polling interval explained anywhere?
- Does watch --stop provide confirmation feedback?
- Are error messages helpful if the daemon isn't running?

---

### 5. Package Explanation & Detail View

Test the per-package drill-down feature.

**Commands (Valid packages):**
- `docker exec brewprune-r6 brewprune explain git`
- `docker exec brewprune-r6 brewprune explain jq`
- `docker exec brewprune-r6 brewprune explain bat`
- `docker exec brewprune-r6 brewprune explain openssl@3`
- `docker exec brewprune-r6 brewprune explain curl`

**Commands (Invalid packages):**
- `docker exec brewprune-r6 brewprune explain nonexistent-package`
- `docker exec brewprune-r6 brewprune explain`
- `docker exec brewprune-r6 brewprune explain --help`

**Evaluate:**
- Does explain show a clear scoring breakdown?
- Are the score components (usage, dependencies, age, type) explained?
- Is the reasoning actionable?
- Are error messages helpful for invalid packages?
- Does the output match the terminology used in unused --verbose?

---

### 6. Diagnostics (Doctor)

Test the health check and diagnostic system.

**Commands (After quickstart):**
- `docker exec brewprune-r6 brewprune doctor`

**Commands (With stopped daemon):**
- `docker exec brewprune-r6 brewprune watch --stop`
- `docker exec brewprune-r6 brewprune doctor`

**Commands (Before any setup):**
- `docker exec brewprune-r6 rm -rf ~/.brewprune`
- `docker exec brewprune-r6 brewprune doctor`

**Evaluate:**
- Does doctor clearly identify issues?
- Are recommendations actionable?
- Does it check all critical components (DB, daemon, usage events)?
- Is the output color-coded for pass/fail?
- Does the --fix flag mention appear helpful or confusing?

---

### 7. Destructive Operations (Remove & Undo)

Test package removal and rollback features with safety mechanisms.

**Commands (Dry-run first):**
- `docker exec brewprune-r6 brewprune remove --safe --dry-run`
- `docker exec brewprune-r6 brewprune remove --medium --dry-run`
- `docker exec brewprune-r6 brewprune remove --risky --dry-run`
- `docker exec brewprune-r6 brewprune remove --tier safe --dry-run`
- `docker exec brewprune-r6 brewprune remove bat fd --dry-run`

**Commands (Snapshots):**
- `docker exec brewprune-r6 brewprune undo --list`

**Commands (Actual removal - if safe packages exist):**
- `docker exec brewprune-r6 brewprune remove --safe --yes`
- `docker exec brewprune-r6 brewprune undo --list`
- `docker exec brewprune-r6 brewprune status`

**Commands (Rollback):**
- `docker exec brewprune-r6 brewprune undo latest`
- `docker exec brewprune-r6 brewprune undo latest --yes`

**Commands (Invalid operations):**
- `docker exec brewprune-r6 brewprune remove nonexistent-package`
- `docker exec brewprune-r6 brewprune remove --safe --medium`
- `docker exec brewprune-r6 brewprune undo 999`
- `docker exec brewprune-r6 brewprune undo`

**Evaluate:**
- Does --dry-run clearly state it's a preview?
- Are confirmation prompts clear and safe?
- Does snapshot creation provide feedback?
- Is the undo process intuitive and safe?
- Are error messages for invalid operations helpful?
- Does --yes skip confirmations as expected?
- Is the --no-snapshot warning sufficiently scary?

---

### 8. Edge Cases & Error Handling

Test boundary conditions and invalid input handling.

**Commands (No arguments):**
- `docker exec brewprune-r6 brewprune`
- `docker exec brewprune-r6 brewprune unused`
- `docker exec brewprune-r6 brewprune stats`
- `docker exec brewprune-r6 brewprune remove`
- `docker exec brewprune-r6 brewprune explain`
- `docker exec brewprune-r6 brewprune undo`

**Commands (Unknown subcommands):**
- `docker exec brewprune-r6 brewprune blorp`
- `docker exec brewprune-r6 brewprune list`
- `docker exec brewprune-r6 brewprune prune`

**Commands (Invalid flags):**
- `docker exec brewprune-r6 brewprune unused --invalid-flag`
- `docker exec brewprune-r6 brewprune remove --safe --medium --risky`
- `docker exec brewprune-r6 brewprune stats --days -1`
- `docker exec brewprune-r6 brewprune stats --days abc`
- `docker exec brewprune-r6 brewprune unused --tier invalid`
- `docker exec brewprune-r6 brewprune unused --min-score 200`
- `docker exec brewprune-r6 brewprune unused --sort invalid`

**Commands (Conflicting flags):**
- `docker exec brewprune-r6 brewprune remove --safe --tier medium`
- `docker exec brewprune-r6 brewprune unused --tier safe --all`
- `docker exec brewprune-r6 brewprune watch --daemon --stop`

**Commands (Missing prerequisites):**
- `docker exec brewprune-r6 rm -rf ~/.brewprune`
- `docker exec brewprune-r6 brewprune unused`
- `docker exec brewprune-r6 brewprune stats`
- `docker exec brewprune-r6 brewprune remove --safe`

**Evaluate:**
- Are error messages specific and actionable?
- Do unknown subcommands suggest valid alternatives?
- Are invalid enum values listed with valid options?
- Does the tool gracefully handle missing database/daemon?
- Are validation errors shown before any destructive action?

---

### 9. Output Quality & Visual Design

Review the overall presentation, formatting, and consistency.

**Commands (Capture all output modes):**
- `docker exec brewprune-r6 brewprune --help`
- `docker exec brewprune-r6 brewprune unused --all`
- `docker exec brewprune-r6 brewprune unused --tier safe --verbose`
- `docker exec brewprune-r6 brewprune status`
- `docker exec brewprune-r6 brewprune stats`
- `docker exec brewprune-r6 brewprune explain git`
- `docker exec brewprune-r6 brewprune doctor`
- `docker exec brewprune-r6 brewprune undo --list`
- `docker exec brewprune-r6 brewprune remove --safe --dry-run`

**Evaluate:**
- **Tables:** Are columns aligned? Do headers stand out? Is data truncated gracefully?
- **Colors:** Are colors used consistently? Do they enhance readability? Are tier colors intuitive (green=safe, red=risky)?
- **Formatting:** Is bold/italic used appropriately? Are lists and bullets clear?
- **Spacing:** Is whitespace used effectively? Are sections visually separated?
- **Terminology:** Is language consistent across all commands (e.g., daemon vs service, score vs confidence, snapshot vs backup)?
- **Symbols:** Are emoji/unicode characters used? Are they helpful or distracting?
- **Headers/Footers:** Do outputs include helpful context (e.g., "Showing 10 of 40 packages")?
- **Errors vs Warnings:** Are severity levels visually distinct?

---

## Instructions for Audit Execution

Run **ALL** commands in sequence. Do not skip areas.

For each command, note:
1. **Exact output** (copy full text)
2. **Exit code** (0 for success, non-zero for error)
3. **Color usage** (e.g., "package names in bold white, tier labels in green/yellow/red")
4. **Behavior** (e.g., "prompts for confirmation", "writes to ~/.brewprune/usage.log")

Describe visual formatting precisely:
- "Table with 4 columns: Package, Score, Tier, Last Used"
- "Tier column shows 'safe' in green, 'medium' in yellow, 'risky' in red"
- "Warning banner in yellow with warning symbol"
- "Error messages prefixed with 'Error:' in red"

---

## Findings Format

For each issue found, use this template:

### [AREA] Finding Title

- **Severity**: UX-critical / UX-improvement / UX-polish
- **What happens**: What the user actually sees
- **Expected**: What better behavior looks like
- **Repro**: Exact command(s)

### Severity Guide

- **UX-critical**: Broken, misleading, or completely missing behavior that blocks the user
- **UX-improvement**: Confusing or unhelpful behavior that a user would notice and dislike
- **UX-polish**: Minor friction, inconsistency, or missed opportunity for clarity

---

## Report Structure

Your final report should include:

1. **Summary Table** at the top:
   ```
   | Severity       | Count |
   |----------------|-------|
   | UX-critical    | X     |
   | UX-improvement | Y     |
   | UX-polish      | Z     |
   | **Total**      | N     |
   ```

2. **Findings by Area** (one section per audit area):
   - Area 1: Discovery & Help System
   - Area 2: Setup & Onboarding
   - Area 3: Core Feature
   - Area 4: Data Collection & Tracking
   - Area 5: Package Explanation
   - Area 6: Diagnostics
   - Area 7: Destructive Operations
   - Area 8: Edge Cases
   - Area 9: Output Quality

3. **Positive Observations** (optional but encouraged):
   - Highlight what works well
   - Note improvements from previous rounds (if applicable)

4. **Recommendations Summary** (optional):
   - High-level themes or patterns
   - Suggested priorities for fixes

---

## Final Steps

Write the complete audit report to:

**`/Users/dayna.blackwell/code/brewprune/docs/cold-start-audit-r6.md`**

Use the **Write** tool to create this file.

---

## IMPORTANT

- Run ALL commands via `docker exec brewprune-r6 <command>` - never run brewprune directly on the host
- Capture exact error messages and exit codes
- Note timing (e.g., "took 3 seconds", "no progress indicator for 45s")
- Be thorough - this audit drives the next round of fixes
