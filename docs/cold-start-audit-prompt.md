You are performing a UX audit of brewprune - a tool that tracks Homebrew package
usage and provides heuristic-scored removal recommendations with automatic snapshots
for easy rollback. It works by installing PATH shims that log command usage, running
a background daemon to process that log, and computing confidence scores (0-100) for
each package based on usage patterns, dependencies, age, and type.

You are acting as a **new user** encountering this tool for the first time.

You have access to a Docker container called `bp-sandbox` with brewprune installed
at `/home/linuxbrew/.linuxbrew/bin/brewprune` and the following packages available:

```
acl, bat, brotli, bzip2, ca-certificates, curl, cyrus-sasl, expat, fd, gettext,
git, jq, keyutils, krb5, libedit, libevent, libgit2, libidn2, libnghttp2,
libnghttp3, libngtcp2, libssh2, libunistring, libxcrypt, libxml2, lz4, ncurses,
oniguruma, openldap, openssl@3, pcre2, readline, ripgrep, sqlite, tmux, utf8proc,
util-linux, xz, zlib-ng-compat, zstd
```

Run all commands using: `docker exec bp-sandbox <command>`

---

## Audit Areas

### 1. Discovery

Explore the top-level help and all subcommand help pages as a new user would.

```
docker exec bp-sandbox brewprune --help
docker exec bp-sandbox brewprune scan --help
docker exec bp-sandbox brewprune unused --help
docker exec bp-sandbox brewprune remove --help
docker exec bp-sandbox brewprune watch --help
docker exec bp-sandbox brewprune explain --help
docker exec bp-sandbox brewprune stats --help
docker exec bp-sandbox brewprune doctor --help
docker exec bp-sandbox brewprune undo --help
docker exec bp-sandbox brewprune status --help
docker exec bp-sandbox brewprune quickstart --help
```

Evaluate: Is the top-level help clear about what brewprune does? Is the Quick Start
section sufficient? Are subcommand descriptions consistent and complete? Does flag
documentation match actual behavior?

---

### 2. Setup / Onboarding

Follow the onboarding flow a new user would take.

```
docker exec bp-sandbox brewprune quickstart
docker exec bp-sandbox brewprune scan
docker exec bp-sandbox brewprune status
```

Evaluate: Does `quickstart` complete without errors? Does it explain what it did?
Are PATH setup instructions clear (i.e., add `~/.brewprune/bin` to front of PATH)?
Does `status` give useful feedback about whether setup succeeded?

---

### 3. Core Feature: Unused

Explore the unused command and its flags exhaustively.

```
docker exec bp-sandbox brewprune unused
docker exec bp-sandbox brewprune unused --tier safe
docker exec bp-sandbox brewprune unused --tier medium
docker exec bp-sandbox brewprune unused --tier risky
docker exec bp-sandbox brewprune unused --all
docker exec bp-sandbox brewprune unused --sort size
docker exec bp-sandbox brewprune unused --sort age
docker exec bp-sandbox brewprune unused --min-score 70
docker exec bp-sandbox brewprune unused -v
docker exec bp-sandbox brewprune unused --casks
```

Evaluate: Is the default output (no flags) useful and clear? Are tier labels visually
distinct (note any color usage)? Does `--all` expand the visible set meaningfully?
Does `-v` (verbose) show a useful scoring breakdown? Is the no-usage-data warning
banner helpful? Are `--sort` and `--min-score` intuitive?

---

### 4. Tracking / Daemon

Start the daemon, check status, wait, then view usage stats.

```
docker exec bp-sandbox brewprune watch --daemon
docker exec bp-sandbox brewprune status
```

Wait 5 seconds, then:

```
docker exec bp-sandbox brewprune stats
docker exec bp-sandbox brewprune stats --package git
```

Evaluate: Does the daemon start silently or with confirmation? Does `status` clearly
show the daemon is running and what it is doing? Does `stats` present data in a
readable format? Does `stats --package git` give per-package detail that a user
would find actionable?

---

### 5. Explain

Test the explain subcommand with valid packages, an unknown package, and no argument.

```
docker exec bp-sandbox brewprune explain git
docker exec bp-sandbox brewprune explain jq
docker exec bp-sandbox brewprune explain nonexistent
docker exec bp-sandbox brewprune explain
```

Evaluate: Does explain show a clear scoring breakdown with labeled components
(usage, dependencies, age, type)? Is it clear why a package got its score?
Does the unknown package case give a useful error message? Does the no-argument
case give a helpful usage hint rather than a cryptic error?

---

### 6. Diagnostics

Run the doctor command in normal and fix mode.

```
docker exec bp-sandbox brewprune doctor
docker exec bp-sandbox brewprune doctor --fix
```

Evaluate: Does `doctor` surface all relevant checks (database, daemon, usage events)?
Are check results clearly labeled as passing or failing? Does `--fix` explain what
it is going to fix before doing it, and confirm what it fixed afterward?

---

### 7. Remove (Dry-Run Only)

Test remove in dry-run mode only — do not actually remove packages.

```
docker exec bp-sandbox brewprune remove --safe --dry-run
docker exec bp-sandbox brewprune remove --tier safe --dry-run
docker exec bp-sandbox brewprune remove nonexistent --dry-run
```

Evaluate: Does `--dry-run` clearly indicate no changes will be made? Is the list
of packages that would be removed legible and informative (scores, tiers, sizes)?
Is there a clear visual distinction between `--safe` (flag) and `--tier safe`
(flag + value) — do both work? Does removing a nonexistent package give a clear
error rather than silently succeeding?

---

### 8. Undo

Explore the undo subcommand and snapshot listing.

```
docker exec bp-sandbox brewprune undo
docker exec bp-sandbox brewprune undo latest
docker exec bp-sandbox brewprune undo --help
```

Evaluate: Does `brewprune undo` with no arguments give clear guidance (e.g., suggest
`--list` or `latest`)? If no snapshots exist, is the error message informative?
Does `undo latest` confirm what it is about to restore before proceeding?
Does `--help` describe the `snapshot-id | latest` argument clearly?

---

### 9. Edge Cases

Probe error handling and unknown input behavior.

```
docker exec bp-sandbox brewprune
docker exec bp-sandbox brewprune blorp
docker exec bp-sandbox brewprune unused --tier invalid
docker exec bp-sandbox brewprune remove --tier invalid --dry-run
docker exec bp-sandbox brewprune unused --sort invalid
```

Evaluate: Does `brewprune` with no args show help or a useful prompt (not a blank
screen or bare error)? Does an unknown subcommand (`blorp`) give a clear "unknown
command" message with a suggestion? Do invalid flag values (`--tier invalid`,
`--sort invalid`) produce user-friendly validation errors listing valid options?
Are exit codes non-zero for all error cases?

---

Run ALL commands. Do not skip areas.
Note exact output, errors, exit codes, and behavior at each step.
Describe color usage where relevant (e.g. "tier labels appear in green/yellow/red").

---

## Findings Format

For each issue found, use:

### [AREA] Finding Title
- **Severity**: UX-critical / UX-improvement / UX-polish
- **What happens**: What the user actually sees
- **Expected**: What better behavior looks like
- **Repro**: Exact command(s)

Severity guide:
- **UX-critical**: Broken, misleading, or completely missing behavior that blocks the user
- **UX-improvement**: Confusing or unhelpful behavior that a user would notice and dislike
- **UX-polish**: Minor friction, inconsistency, or missed opportunity for clarity

---

## Report

- Group findings by area
- Include a summary table at the top: total count by severity
- Write the complete report to `docs/cold-start-audit.md` using the Write tool

IMPORTANT: Run ALL commands via `docker exec bp-sandbox <command>`.
Do not run brewprune directly on the host.
