You are performing a UX audit of brewprune - a tool that tracks Homebrew package
usage and provides heuristic-scored removal recommendations with automatic snapshots
for easy rollback. It works by installing PATH shims that log command usage, running
a background daemon to process that log, and computing confidence scores (0-100) for
each package based on usage patterns, dependencies, age, and type.

You are acting as a **new user** encountering this tool for the first time.

You have access to a Docker container called `bp-audit4` with brewprune installed
at `/home/linuxbrew/.linuxbrew/bin/brewprune` and the following packages available:

```
acl, bat, brotli, bzip2, ca-certificates, curl, cyrus-sasl, expat, fd, gettext,
git, jq, keyutils, krb5, libedit, libevent, libgit2, libidn2, libnghttp2,
libnghttp3, libngtcp2, libssh2, libunistring, libxcrypt, libxml2, lz4, ncurses,
oniguruma, openldap, openssl@3, pcre2, readline, ripgrep, sqlite, tmux, utf8proc,
util-linux, xz, zlib-ng-compat, zstd
```

Run all commands using: `docker exec bp-audit4 <command>`

---

## Audit Areas

### 1. Discovery

Explore the top-level help and all subcommand help pages as a new user would.

```bash
docker exec bp-audit4 brewprune
docker exec bp-audit4 brewprune --help
docker exec bp-audit4 brewprune --version
docker exec bp-audit4 brewprune quickstart --help
docker exec bp-audit4 brewprune scan --help
docker exec bp-audit4 brewprune watch --help
docker exec bp-audit4 brewprune status --help
docker exec bp-audit4 brewprune unused --help
docker exec bp-audit4 brewprune stats --help
docker exec bp-audit4 brewprune explain --help
docker exec bp-audit4 brewprune remove --help
docker exec bp-audit4 brewprune undo --help
docker exec bp-audit4 brewprune doctor --help
docker exec bp-audit4 brewprune completion --help
docker exec bp-audit4 brewprune blorp
```

Check exit codes:
```bash
docker exec bp-audit4 sh -c 'brewprune; echo "Exit code: $?"'
docker exec bp-audit4 sh -c 'brewprune blorp; echo "Exit code: $?"'
```

### 2. Setup / Onboarding

Run the quickstart command and observe the setup workflow:

```bash
docker exec bp-audit4 brewprune quickstart
```

After quickstart, check the status:

```bash
docker exec bp-audit4 brewprune status
```

Then run diagnostics:

```bash
docker exec bp-audit4 brewprune doctor
docker exec bp-audit4 brewprune doctor --fix
```

Check PATH detection:

```bash
docker exec bp-audit4 sh -c 'echo $PATH'
docker exec bp-audit4 sh -c 'grep brewprune ~/.bashrc ~/.bash_profile ~/.zshrc ~/.profile 2>/dev/null || echo "Not found in shell configs"'
```

### 3. Core Feature: Unused Package Detection

Explore the primary feature - finding unused packages:

```bash
docker exec bp-audit4 brewprune unused
docker exec bp-audit4 brewprune unused --verbose
docker exec bp-audit4 brewprune unused --all
docker exec bp-audit4 brewprune unused --tier safe
docker exec bp-audit4 brewprune unused --tier medium
docker exec bp-audit4 brewprune unused --tier risky
docker exec bp-audit4 brewprune unused --tier invalid
docker exec bp-audit4 brewprune unused --min-score 70
docker exec bp-audit4 brewprune unused --min-score 70 --all
docker exec bp-audit4 brewprune unused --min-score 0
docker exec bp-audit4 brewprune unused --tier safe --min-score 90
```

### 4. Data / Tracking

Test the usage tracking system:

```bash
docker exec bp-audit4 brewprune status
docker exec bp-audit4 brewprune watch --daemon
docker exec bp-audit4 sh -c 'sleep 2 && brewprune status'
docker exec bp-audit4 sh -c 'git --version && jq --version && fd --version'
docker exec bp-audit4 sh -c 'sleep 35 && brewprune status'
docker exec bp-audit4 brewprune scan
docker exec bp-audit4 sh -c 'brewprune scan && echo "---" && brewprune scan'
```

### 5. Explanation / Detail

Drill down into specific packages:

```bash
docker exec bp-audit4 brewprune explain git
docker exec bp-audit4 brewprune explain jq
docker exec bp-audit4 brewprune explain openssl@3
docker exec bp-audit4 brewprune explain nonexistent-package
docker exec bp-audit4 brewprune explain
docker exec bp-audit4 brewprune stats
docker exec bp-audit4 brewprune stats --package git
docker exec bp-audit4 brewprune stats --package jq
docker exec bp-audit4 brewprune stats --package git --days 7
docker exec bp-audit4 brewprune stats --all
```

### 6. Diagnostics

Test the diagnostic and health check features:

```bash
docker exec bp-audit4 brewprune doctor
docker exec bp-audit4 brewprune status
docker exec bp-audit4 sh -c 'pkill -f "brewprune watch" && sleep 1 && brewprune doctor'
docker exec bp-audit4 sh -c 'brewprune doctor && echo "Exit code: $?"'
```

### 7. Destructive / Write Operations

Test removal operations (always with --dry-run first):

```bash
docker exec bp-audit4 brewprune remove --help
docker exec bp-audit4 brewprune remove --tier safe --dry-run
docker exec bp-audit4 brewprune remove --dry-run nonexistent-package
docker exec bp-audit4 brewprune undo --list
docker exec bp-audit4 brewprune undo latest
docker exec bp-audit4 brewprune undo 999
```

### 8. Edge Cases

Test boundary conditions and error handling:

```bash
docker exec bp-audit4 brewprune
docker exec bp-audit4 brewprune invalid-command
docker exec bp-audit4 brewprune --invalid-flag
docker exec bp-audit4 brewprune unused --tier invalid-tier
docker exec bp-audit4 brewprune explain
docker exec bp-audit4 brewprune stats --package
docker exec bp-audit4 brewprune remove
docker exec bp-audit4 brewprune undo
docker exec bp-audit4 brewprune --db /nonexistent/path.db status
```

### 9. Output Review

Examine all output for:

- **Table alignment**: Do columns line up? Are headers clear?
- **Colors**: What colors are used? Are they semantic (green=good, red=bad)?
- **Headers/footers**: Are summaries helpful? Is context provided?
- **Terminology**: Is language consistent? Are abbreviations explained?
- **Error messages**: Are errors actionable? Do they suggest next steps?
- **Progress indicators**: Is long-running work indicated clearly?
- **Empty states**: What happens when no data exists? Is it clear vs. alarming?

---

## Instructions

1. **Run ALL commands** listed above via `docker exec bp-audit4 <command>`. Do not skip areas.

2. **Note exact output** at each step:
   - What appears on screen (full text for errors, summaries for tables)
   - Color usage (e.g., "package names in bold white, tiers in green/yellow/red")
   - Exit codes for commands that should fail
   - Table formatting and alignment
   - Progress indicators and wait times

3. **Act as a new user**:
   - What's confusing or unclear?
   - What behavior is surprising?
   - What's missing that you'd expect?
   - What works well and should be preserved?

4. **Document findings** using this format:

### [AREA] Finding Title
- **Severity**: UX-critical / UX-improvement / UX-polish
- **What happens**: What the user actually sees
- **Expected**: What better behavior looks like
- **Repro**: Exact command(s)

**Severity guide:**
- **UX-critical**: Broken, misleading, or completely missing behavior that blocks the user
- **UX-improvement**: Confusing or unhelpful behavior that a user would notice and dislike
- **UX-polish**: Minor friction, inconsistency, or missed opportunity for clarity

5. **Write the complete report** to `docs/cold-start-audit.md` using the Write tool:
   - Include a summary table at the top with counts by severity
   - Group findings by area (Discovery, Setup, Core Feature, etc.)
   - Include the environment details (container name, packages, date)

---

## IMPORTANT

- Run ALL commands via `docker exec bp-audit4 <command>` - never run brewprune directly on the host
- Capture exact error messages and exit codes
- Note timing (e.g., "took 3 seconds", "no progress indicator for 45s")
- Be thorough - this audit drives the next round of fixes
