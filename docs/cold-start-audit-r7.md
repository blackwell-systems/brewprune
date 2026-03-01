# Cold-Start UX Audit Report - Round 7

**Metadata:**
- Audit Date: 2026-02-28
- Tool Version: brewprune version dev (commit: unknown, built: unknown)
- Container: brewprune-r7
- Environment: Ubuntu 22.04 with Homebrew (Linux)
- Auditor: Claude agent (Sonnet 4.6)

---

## Summary Table

| # | Area | Severity | Title |
|---|------|----------|-------|
| 1 | Remove | UX-critical | `remove --medium --no-snapshot --yes` reports "Removed 0 packages, freed 180 MB" despite 0 actual removals |
| 2 | Remove | UX-critical | `remove --medium` includes dependency-locked packages without flagging them as unremovable |
| 3 | Explain | UX-critical | `explain curl` exits with code 139 (segfault/SIGBUS) — intermittent crash |
| 4 | Data Tracking | UX-critical | Shims don't capture jq, bat, fd, rg usage — only git appears in usage.log after running all five tools |
| 5 | Undo | UX-improvement | Restored package lines show bare version string: "Restored bat@" (missing version number) |
| 6 | Watch | UX-improvement | `watch --daemon --stop` silently picks one behavior instead of erroring; behavior is unpredictable |
| 7 | Errors | UX-improvement | `stats --days abc` exposes raw Go parse error instead of a user-friendly message |
| 8 | Remove | UX-improvement | Stale scan warning ("5 new formulae since last scan") appears on `remove` even after undo restores packages, misleads the user |
| 9 | Unused | UX-improvement | `--sort age` output is visually indistinguishable from random order — no age column, no explanation |
| 10 | Verbose | UX-improvement | `unused --verbose` output is extremely long (36 package blocks) with no pagination prompt by default |
| 11 | Doctor | UX-polish | Alias tip in `doctor` output references `brewprune help` for details, but `brewprune help` has no alias documentation |
| 12 | Status | UX-polish | `status` shows "since 0 seconds ago" for a freshly started daemon — awkward phrasing |
| 13 | Unused | UX-polish | `unused --all` and `unused` (default) both show the Reclaimable footer line with "(risky, hidden)" even when risky IS shown |
| 14 | Explain | UX-polish | `explain git` shows "Usage: 0/40 pts - used today" but the label is misleading (0 pts = maximum usage, not zero usage) |
| 15 | Watch | UX-polish | `watch.log` is always empty — daemon logs nothing; harder to debug issues |

---

## Area 1: Discovery & Help System

### Commands Run
- `brewprune --help` — EXIT 0
- `brewprune --version` — EXIT 0
- `brewprune -v` — EXIT 0
- `brewprune help` — EXIT 0
- `brewprune` — EXIT 0
- All subcommand `--help` — EXIT 0

### Observations

**`brewprune --help` / `brewprune help` / `brewprune` (no args)**

All three produce identical output — the full help page. This is good: the zero-arg invocation is informative rather than producing a terse error.

Output structure:
```
brewprune tracks Homebrew package usage...

IMPORTANT: You must run 'brewprune watch --daemon'...

Quick Start:
  brewprune quickstart   # Recommended...
  Or manually:
  1. brewprune scan
  ...

Features: ...
Examples: ...
Available Commands: ...
Flags: ...
```

The IMPORTANT block is well-placed. Quick Start explains both quickstart and manual paths clearly. Feature bullet list is accurate.

**`--version` / `-v`**

Both output `brewprune version dev (commit: unknown, built: unknown)`. This is expected for a dev build but worth noting as a papercut in a shipped binary.

**Subcommand `--help` pages**

All subcommand help pages are well-structured and accurate. Notable strengths:
- `unused --help` explains the scoring formula (Usage 40pt, Dependencies 30pt, Age 20pt, Type 10pt), tier filtering behavior, and the default view logic in detail.
- `watch --help` explains the shim pipeline concisely.
- `remove --help` documents the tier shortcut flags clearly, including the equivalence of `--safe` and `--tier safe`.
- `undo --help` gives clear examples with real syntax.
- `scan --help` explains when to run the command and the `--refresh-shims` fast path.

**Issues:**

### [HELP] Alias tip references `brewprune help` which has no alias docs

- **Severity**: UX-polish
- **What happens**: `doctor` output includes: `Tip: Create ~/.config/brewprune/aliases to declare alias mappings and improve tracking coverage. See 'brewprune help' for details.` But `brewprune help` has no mention of aliases, the config file path, or the format.
- **Expected**: The tip should point to a specific command or doc that explains aliases (e.g., `brewprune doctor --help`, a `brewprune alias --help`, or a URL), or include the brief format inline.
- **Repro**: `brewprune doctor` (any state with running daemon)

### [HELP] `doctor --help` does not mention `--fix` flag

- **Severity**: UX-polish
- **What happens**: The audit prompt referenced a `--fix` flag for doctor. Running `brewprune doctor --help` reveals no such flag exists. The help page only shows `-h/--help`. This is not a bug — the flag was simply never implemented — but if referenced in documentation or onboarding, it would confuse users.
- **Expected**: No mention of `--fix` if it doesn't exist.
- **Repro**: `brewprune doctor --help`

---

## Area 2: Setup & Onboarding (First-Run Experience)

### Commands Run
- `brewprune quickstart` — EXIT 0
- `brewprune status` (post-quickstart) — EXIT 0
- `cat ~/.brewprune/watch.log` — EXIT 0 (file exists but empty)
- `ls -la ~/.brewprune/` — EXIT 0
- `brewprune doctor` (post-quickstart) — EXIT 0
- `brewprune scan` (manual path) — EXIT 0
- `brewprune watch --daemon` (manual path) — EXIT 0
- `brewprune doctor` (with stopped daemon) — EXIT 0

### Observations

**Quickstart output:**
```
Welcome to brewprune! Running end-to-end setup...

Step 1/4: Scanning installed Homebrew packages
  ✓ Scan complete: 40 packages, 352 MB

Step 2/4: Verifying ~/.brewprune/bin is in PATH
  ✓ Added /home/brewuser/.brewprune/bin to PATH in /home/brewuser/.profile
  Restart your shell (or source the config file) for this to take effect.

Step 3/4: Starting usage tracking daemon
  brew found but using daemon mode (brew services not supported on Linux)
  ✓ Daemon started (log: ~/.brewprune/watch.log)

Step 4/4: Running self-test (tracking verified)
Verifying shim → daemon → database pipeline...
  ✓ Self-test passed (tracking will work after shell restart)

Setup complete!

IMPORTANT: Wait 1-2 weeks before acting on recommendations.
...
```

Step-by-step feedback is clear and actionable. The `brew services not supported on Linux` notice is informative without being alarming. The self-test ran and passed in ~22 seconds (per doctor output).

**`~/.brewprune/` directory after quickstart:**
```
bin/           (shim directory)
brewprune.db   (73 KB)
brewprune.db-shm
brewprune.db-wal
usage.log      (54 bytes, 2 entries)
usage.offset   (2 bytes)
watch.log      (0 bytes, empty)
watch.pid      (5 bytes)
```

**watch.log is always empty** — the daemon does not write to it. This is notable because quickstart tells the user the log path, and doctor/status reference it. A user trying to debug tracking issues has no log to inspect.

**`brewprune status` post-quickstart:**
```
Tracking:     running (since 3 seconds ago, PID 1113)
Events:       1 total · 1 in last 24h
Shims:        inactive · 0 commands · PATH configured (restart shell to activate)
Last scan:    3 seconds ago · 40 formulae · 72 KB
Data quality: COLLECTING (0 of 14 days)
```

Clean and informative. "Data quality: COLLECTING" clearly communicates the early-stage state.

### [ONBOARDING] "since 0 seconds ago" is awkward

- **Severity**: UX-polish
- **What happens**: `status` shows `running (since 0 seconds ago, PID 2121)` immediately after start.
- **Expected**: "just now" or "since a moment ago" for sub-5-second age.
- **Repro**: `brewprune watch --daemon && brewprune status`

### [ONBOARDING] watch.log is always empty

- **Severity**: UX-polish
- **What happens**: `~/.brewprune/watch.log` exists and is referenced in daemon start output, but contains zero bytes. There are no daemon lifecycle messages, errors, or processing confirmations written to it.
- **Expected**: At minimum, the daemon should write startup/shutdown timestamps and periodic heartbeat lines (e.g., "processed N events at 03:20:15"). This makes debugging broken pipelines possible.
- **Repro**: `brewprune quickstart; cat ~/.brewprune/watch.log`

---

## Area 3: Core Feature - Unused Package Discovery

### Commands Run
- `brewprune unused` — EXIT 0
- `brewprune unused --all` — EXIT 0
- `brewprune unused --tier safe/medium/risky` — EXIT 0
- `brewprune unused --min-score 70/50` — EXIT 0
- `brewprune unused --sort score/size/age` — EXIT 0
- `brewprune unused --casks` — EXIT 0
- `brewprune unused --verbose` — EXIT 0
- `brewprune unused --tier safe --verbose` — EXIT 0

### Observations

**Default `unused` output (table):**
```
SAFE: 5 packages (39 MB) · MEDIUM: 31 (180 MB) · RISKY: 4 (hidden, use --all)

Package          Size     Score   Uses (7d)  Last Used        Depended On   Status
────────────────────────────────────────────────────────────────────────────────────────
bat              5 MB     80/100  0          —                —             ✓ safe
...
```

Table is clean, columns are well-aligned, and the header bar showing tier summaries at the top is excellent orientation. "—" for empty cells is a clean choice.

**Tier filtering with `--tier`** is well-implemented: the selected tier is visually highlighted in the header bar with brackets: `[SAFE: 5 packages (39 MB)]`.

**`--min-score` filtering** shows a count banner: `Showing 5 of 40 packages (score >= 70)` and a Hidden line at the bottom. Good.

**`--casks`** gracefully reports: `No casks found in the Homebrew database.` with an explanation.

**`--verbose`** expands every package into a multi-line block showing score components. Useful but very long for 36 packages. A "pipe to less" tip appears at the end of verbose output — this is a nice touch.

### [UNUSED] `--sort age` output appears unsorted

- **Severity**: UX-improvement
- **What happens**: `brewprune unused --sort age` produces a list that appears to be in random order (not chronological, not alphabetical). All packages were installed at the same time in this environment, so age sorting yields undefined order. The table has no "Install Age" column to show what it sorted by.
- **Expected**: When sorting by age, an "Installed" column should appear (or the existing table should at minimum indicate the sort column with an arrow). When all ages are equal, the secondary sort should be deterministic (e.g., alphabetical by name).
- **Repro**: `brewprune unused --sort age`

### [UNUSED] `--verbose` with no `--tier` dumps 36 package blocks

- **Severity**: UX-improvement
- **What happens**: `brewprune unused --verbose` without a tier filter prints verbose blocks for all 36 visible packages (safe + medium), producing hundreds of lines. The "pipe to less" tip appears only at the very end, after the user has already been overwhelmed.
- **Expected**: Either (a) prompt the user before dumping verbose for more than N packages, or (b) move the "pipe to less" tip to before the output, or (c) auto-paginate.
- **Repro**: `brewprune unused --verbose`

### [UNUSED] Reclaimable footer incorrectly shows "(risky, hidden)" when risky is shown

- **Severity**: UX-polish
- **What happens**: When running `brewprune unused --all` (which explicitly shows all tiers including risky), the footer still reads: `Reclaimable: 39 MB (safe) · 180 MB (medium) · 134 MB (risky, hidden)`.
- **Expected**: When `--all` is used, the risky column in the footer should read `134 MB (risky)` without "(hidden)".
- **Repro**: `brewprune unused --all`

### [EXPLAIN] `explain git` shows confusing "0/40 pts - used today" label

- **Severity**: UX-polish
- **What happens**: For `git` (which had 2 recorded uses), `explain` shows:
  ```
  Usage:         0/40 pts - used today
  ```
  The "0/40 pts" means 0 points toward removal confidence (i.e., the package is ACTIVELY USED, which makes it hard to justify removing). But the label "0/40 pts" reads as "zero out of forty" which a new user might interpret as "the usage score is zero, so it's not used."
- **Expected**: Invert the framing or clarify: e.g., `Usage: 0/40 pts - recently used (penalizes removal confidence)` or better yet, label the column "Usage Penalty" vs "Removal Signal."
- **Repro**: Use git a few times, then `brewprune explain git`

---

## Area 4: Data Collection & Tracking

### Commands Run
- `brewprune status` — EXIT 0
- `brewprune watch --daemon` — EXIT 0
- `cat ~/.brewprune/watch.pid` — EXIT 0
- `ps aux | grep brewprune` — EXIT 0
- Run `git`, `jq`, `bat`, `fd`, `rg --version` — EXIT 0
- `sleep 35` — EXIT 0
- `cat ~/.brewprune/usage.log` — EXIT 0
- `brewprune status` — EXIT 0
- `brewprune stats` — EXIT 0
- `brewprune stats --days 1` — EXIT 0
- `brewprune stats --package git` — EXIT 0
- `brewprune stats --all` — EXIT 0
- `brewprune watch --stop` — EXIT 0
- `brewprune status` (post-stop) — EXIT 0

### Observations

**Daemon process:**
```
brewuser 2121 ... /usr/bin/qemu-x86_64 /home/linuxbrew/.linuxbrew/bin/brewprune watch --daemon-child
```

Note: PID 1113 from quickstart's daemon was defunct (zombie process). The subsequent `brewprune watch --daemon` started a fresh process at PID 2121. The defunct PID 1113 is a Linux-specific issue with qemu emulation and not a brewprune bug per se, but it means quickstart may leave a zombie behind.

**usage.log after running git, jq, bat, fd, rg:**
```
1772335188165120518,/home/brewuser/.brewprune/bin/git
1772335196116770024,/home/brewuser/.brewprune/bin/git
```

Only `git` appears. Running `jq`, `bat`, `fd`, and `rg` through the PATH produced no shim log entries. This is a critical tracking gap.

**Root cause:** The shims are not active because `~/.brewprune/bin` is not on the active PATH in this shell session (quickstart noted "PATH configured (restart shell to activate)"). The self-test during quickstart works because it uses a controlled environment to exercise the pipeline. Real PATH-based tracking cannot work until the user restarts their shell.

This is a known limitation but creates a confusing experience: the user runs commands, then checks usage.log, sees only git (from the self-test), and has no idea their tool usage isn't being captured.

### [TRACKING] Shim gap: no usage recorded for jq, bat, fd, rg after running them

- **Severity**: UX-critical
- **What happens**: After quickstart, running `jq --version`, `bat --version`, `fd --version`, `rg --version` produces no entries in `usage.log`. Only the self-test's git invocation is logged. After 35 seconds (one daemon poll cycle), `brewprune status` still shows 2 total events.
- **Expected**: The quick start should either (a) make the PATH active immediately (via `exec $SHELL` or sourcing the profile within the quickstart subprocess), or (b) display a prominent warning: "Tracking is NOT yet active — run `source ~/.profile` to start capturing usage now."
- **Repro**: `brewprune quickstart; jq --version; sleep 35; cat ~/.brewprune/usage.log`

**`brewprune stats` output:**
```
Showing 1 of 40 packages (39 with no recorded usage — use --all to see all)

Package    Total Runs Last Used     Frequency  Trend
git        2          2 minutes ago daily      →
```

Stats output is clean. The "Frequency" classification and trend arrow (→) are good visual cues. The summary line is accurate.

**`brewprune stats --package git`:**
```
Package: git
Total Uses: 2
Last Used: 2026-03-01 03:19:56
Days Since: 0
First Seen: 2026-02-28 08:44:47
Frequency: daily
Tip: Run 'brewprune explain git' for removal recommendation...
```

Per-package stats detail is useful. The cross-reference tip to `explain` is a nice UX touch.

**`brewprune watch --stop`:**
```
Stopping daemon......
✓ Daemon stopped
```

Clean feedback. The animated dots (`.......`) indicate waiting/progress.

**`brewprune status` post-stop:**
```
Tracking:     stopped  (run 'brewprune watch --daemon')
```

The inline action hint is helpful.

---

## Area 5: Package Explanation & Detail View

### Commands Run
- `brewprune explain git` — EXIT 0
- `brewprune explain jq` — EXIT 0
- `brewprune explain bat` — EXIT 0
- `brewprune explain openssl@3` — EXIT 0
- `brewprune explain curl` — EXIT 139 (first run), EXIT 0 (second run after container restart)
- `brewprune explain nonexistent-package` — EXIT 1
- `brewprune explain` — EXIT 1

### Observations

**`explain git` output:**
```
Package: git
Score:   30 (RISKY)
Installed: 2026-02-28

Breakdown:
  Usage:         0/40 pts - used today
  Dependencies: 30/30 pts - no dependents
  Age:           0/20 pts - installed today
  Type:          0/10 pts - foundational package (reduced confidence)
  Critical: YES - capped at 70 (core system dependency)
  Total: 30/100 (RISKY tier)

Why RISKY: recently used, keep

Recommendation: Do not remove. This is a foundational package...
Protected: YES (part of 47 core dependencies)
```

Good structure. The "Critical: YES - capped at 70" row clearly explains why scores are bounded. The Protected count (47 core dependencies) gives useful context.

**`explain jq` and `explain bat`** — clean, consistent with the verbose unused output format.

**`explain openssl@3`** — correct: flags as RISKY with 9 dependents.

### [EXPLAIN] `explain curl` crashes with exit code 139

- **Severity**: UX-critical
- **What happens**: First invocation of `brewprune explain curl` exits with code 139 (SIGSEGV or SIGBUS). No error message is printed. Subsequent invocations succeed.
- **Expected**: The command should never crash silently. The intermittent nature suggests a race condition or uninitialized state on first lookup of a package with "used dependents."
- **Repro**: Fresh container, run quickstart, then immediately `brewprune explain curl`. (Reproduced once; second run was clean.)

**`explain nonexistent-package` (EXIT 1):**
```
Error: package not found: nonexistent-package
Check the name with 'brew list' or 'brew search nonexistent-package'.
If you just installed it, run 'brewprune scan' to update the index.
```

Excellent error message — specific, actionable, with three recovery paths.

**`explain` (no args, EXIT 1):**
```
Error: missing package name. Usage: brewprune explain <package>
```

Clean. Could also suggest running `brewprune unused` to discover package names.

---

## Area 6: Diagnostics (Doctor)

### Commands Run
- `brewprune doctor` (after quickstart) — EXIT 0
- `brewprune doctor` (with stopped daemon) — EXIT 0
- `rm -rf ~/.brewprune; brewprune doctor` — EXIT 1

### Observations

**Doctor after quickstart:**
```
Running brewprune diagnostics...

✓ Database found: /home/brewuser/.brewprune/brewprune.db
✓ Database is accessible
✓ 40 packages tracked
✓ 1 usage events recorded
✓ Daemon running (PID 1113)
✓ Shim binary found: /home/brewuser/.brewprune/bin/brewprune-shim
⚠ PATH configured (restart shell to activate)
  Action: Restart your shell or run: source ~/.profile

Tip: Create ~/.config/brewprune/aliases...
Running pipeline test (~30s)......
✓ Pipeline test: pass (22.257s)

Found 1 warning(s). System is functional but not fully configured.
```

Doctor is thorough and actionable. The pipeline test (which takes ~22-30 seconds) gives genuine confidence. The `(~30s)` annotation sets accurate expectations for the wait.

The "Found 1 warning(s). System is functional..." summary line is a great pattern.

**Doctor with stopped daemon:**
```
⚠ Daemon not running (no PID file)
  Action: Run 'brewprune watch --daemon'
⊘ Pipeline test skipped (daemon not running)
  The pipeline test requires a running daemon...
```

The `⊘` symbol for skipped checks is a useful visual distinction from `✓` and `⚠`.

**Doctor with no setup (rm -rf ~/.brewprune):**
```
✗ Database not found at: /home/brewuser/.brewprune/brewprune.db
  Action: Run 'brewprune scan' to create database
⚠ Daemon not running (no PID file)
  Action: Run 'brewprune watch --daemon'
✗ Shim binary not found — usage tracking disabled
  Action: Run 'brewprune scan' to build it

Found 2 critical issue(s) and 1 warning(s).
Error: diagnostics failed
EXIT:1
```

Good use of `✗` for critical failures vs `⚠` for warnings. Exit code 1 is correct for critical issues. "diagnostics failed" as a final summary line is appropriate.

**Minor issue:** The Alias tip always appears even in the fully-broken state. It should only appear when the system is otherwise healthy (it's a "nice to have" improvement, not a first-aid instruction).

---

## Area 7: Destructive Operations (Remove & Undo)

### Commands Run
- `brewprune remove --safe/--medium/--risky --dry-run` — EXIT 0
- `brewprune remove --tier safe --dry-run` — EXIT 0
- `brewprune remove bat fd --dry-run` — EXIT 0
- `brewprune undo --list` — EXIT 0
- `brewprune remove --safe --yes` — EXIT 0
- `brewprune undo latest` — EXIT 0 (cancelled)
- `brewprune undo latest --yes` — EXIT 0
- `brewprune remove nonexistent-package` — EXIT 1
- `brewprune remove --safe --medium` — EXIT 1
- `brewprune undo 999` — EXIT 1
- `brewprune undo` — EXIT 1
- `brewprune remove --medium --no-snapshot --yes` — EXIT ? (partial failures)

### Observations

**`remove --safe --dry-run`:**
```
Packages to remove (safe tier):
[table of 5 packages]

Summary:
  Packages: 5
  Disk space to free: 39 MB
  Snapshot: will be created

Dry-run mode: no packages will be removed.
```

Dry-run label is clear. Summary section is well-structured.

**`remove bat fd --dry-run` (explicit packages):**
```
  ⚠ bat: explicitly installed (not a dependency)
  ⚠ fd: explicitly installed (not a dependency)
```

The explicit-install warnings are a nice safety touch.

**`remove --safe --yes` (actual removal):**
```
Creating snapshot...
Snapshot created: ID 1

Removing 5 packages...
[=======================================>] 100% Removing packages

✓ Removed 5 packages, freed 39 MB

Snapshot: ID 1
Undo with: brewprune undo 1
```

Progress bar is clean. Post-removal undo instruction is excellent UX.

**`undo latest` (confirmation prompt):**
```
Restore 5 packages? [y/N]: Restoration cancelled.
```

Correct default-to-cancel behavior.

**`undo latest --yes`:**
```
Restoring packages from snapshot......
Restored bat@
Restored fd@
Restored jq@
Restored ripgrep@
Restored tmux@

✓ Restored 5 packages from snapshot 1

⚠  Run 'brewprune scan' to update the package database before running 'brewprune remove'.
```

The post-restore scan reminder is essential and well-placed. However, the restored package lines show `Restored bat@` — the `@` suffix with no version is a formatting defect.

### [REMOVE] `remove --medium` includes dependency-locked packages and silently fails

- **Severity**: UX-critical
- **What happens**: `brewprune remove --medium --no-snapshot --yes` includes ~31 packages. Most of them fail silently with `brew uninstall` errors like:
  ```
  - zstd: brew uninstall zstd failed: exit status 1 (output: Error: Refusing to uninstall...
    because it is required by curl and git...)
  ```
  The summary line incorrectly reports `✓ Removed 0 packages, freed 180 MB` — claiming 180 MB freed despite removing nothing.
- **Expected**: (a) The claimed disk-freed number should match actual removals. (b) The `remove` command should pre-validate which packages can be uninstalled (respecting brew's dependency lock) before displaying them as candidates and before executing. (c) Packages that brew refuses to uninstall should be excluded from medium-tier recommendations or clearly flagged as "requires --ignore-dependencies."
- **Repro**: `brewprune remove --medium --no-snapshot --yes`

### [REMOVE] Stale scan warning appears incorrectly after undo

- **Severity**: UX-improvement
- **What happens**: After `brewprune undo latest --yes` restores 5 packages, subsequent `remove` commands show: `⚠  5 new formulae since last scan. Run 'brewprune scan' to update shims.` This is technically correct (the database is stale), but the message appears on every remove invocation until the user scans — including before displaying the dry-run table. It's noisy and the undo output already tells the user to scan.
- **Expected**: Either suppress the warning when the user just ran undo (which always ends with a scan reminder), or make the warning appear only in `status` and `doctor` rather than inline on destructive operations.
- **Repro**: `brewprune undo latest --yes; brewprune remove --safe --dry-run`

### [UNDO] Restored package lines show bare "@" suffix

- **Severity**: UX-improvement
- **What happens**: Undo restore output prints:
  ```
  Restored bat@
  Restored fd@
  ```
  The `@` is a version separator (e.g., `bat@0.26.1`) but the version number is missing.
- **Expected**: Either show the full version (`Restored bat@0.26.1`) or omit the `@` entirely (`Restored bat`).
- **Repro**: `brewprune undo latest --yes`

### [REMOVE] `remove --safe --medium` gives a clear conflict error

Good behavior: `Error: only one tier flag can be specified at a time (got --safe and --medium)` — specific and actionable.

### [UNDO] `undo 999` gives a clear not-found error

Good behavior: `Error: snapshot 999 not found` with `Run 'brewprune undo --list'` hint.

### [REMOVE] `remove` with no args gives a helpful suggestion

Good behavior: `Error: no tier specified` with `Try: brewprune remove --safe --dry-run`.

---

## Area 8: Edge Cases & Error Handling

### Commands Run
- `brewprune` (no args) — EXIT 0
- `brewprune unused/stats/remove/explain/undo` (no args) — EXIT 0/1
- `brewprune blorp/list/prune` — EXIT 1
- `brewprune unused --invalid-flag` — EXIT 1
- `brewprune remove --safe --medium --risky` — EXIT 1
- `brewprune stats --days -1` — EXIT 1
- `brewprune stats --days abc` — EXIT 1
- `brewprune unused --tier invalid` — EXIT 1
- `brewprune unused --min-score 200` — EXIT 1
- `brewprune unused --sort invalid` — EXIT 1
- `brewprune remove --safe --tier medium` — EXIT 1
- `brewprune unused --tier safe --all` — EXIT 1
- `brewprune watch --daemon --stop` — EXIT 0
- Missing database scenarios — EXIT 1

### Observations

**Unknown subcommands:**
```
Error: unknown command "blorp" for "brewprune"
Run 'brewprune --help' for usage.
```
The message is minimal — no suggestions for similar valid commands. Commands like `list` and `prune` are natural guesses from users. A "Did you mean: unused?" suggestion for `list`, or "Did you mean: remove?" for `prune` would reduce friction.

**Invalid enum values:**
- `--tier invalid` → `Error: invalid --tier value "invalid": must be one of: safe, medium, risky` — excellent.
- `--sort invalid` → `Error: invalid sort: invalid (must be score, size, or age)` — excellent.
- `--min-score 200` → `Error: invalid min-score: 200 (must be 0-100)` — excellent.
- `--days -1` → `Error: --days must be a positive integer` — good.

**Raw Go parse error:**
### [ERRORS] `stats --days abc` exposes raw Go parse error

- **Severity**: UX-improvement
- **What happens**: `brewprune stats --days abc` outputs:
  ```
  Error: invalid argument "abc" for "--days" flag: strconv.ParseInt: parsing "abc": invalid syntax
  ```
  The `strconv.ParseInt: parsing "abc": invalid syntax` portion is a raw Go runtime message.
- **Expected**: `Error: --days must be a positive integer` (same as `--days -1`).
- **Repro**: `brewprune stats --days abc`

**`watch --daemon --stop` (conflicting flags):**
```
Warning: --daemon and --stop are mutually exclusive; stopping daemon.
```

Choosing to act rather than error is acceptable, but this is unpredictable — it silently picked `--stop` over `--daemon`. An error with "use one or the other" would be less surprising. Marked as UX-improvement.

**Missing database:**
```
Error: failed to list packages: database not initialized — run 'brewprune scan' to create the database
```
Clear and actionable. The full chain `failed to list packages: database not initialized` is slightly verbose but acceptable.

**`unused --tier safe --all` conflict:**
```
Error: Error: --all and --tier cannot be used together; --tier already filters to a specific tier
```
The doubled `Error: Error:` prefix is a minor defect.

### [ERRORS] `unused --tier safe --all` double "Error:" prefix

- **Severity**: UX-polish
- **What happens**: Error message reads `Error: Error: --all and --tier cannot be used together...`
- **Expected**: Single `Error:` prefix.
- **Repro**: `brewprune unused --tier safe --all`

---

## Area 9: Output Quality & Visual Design

### Overall Assessment

**Tables:** All tables are well-aligned with consistent column widths. Column headers use a separator line (`────`) and stand out clearly. Data is not truncated — package names like `zlib-ng-compat` and `ca-certificates` display fully.

**Colors** (inferred from symbols used in plaintext capture):
- `✓` used for passed checks and safe-tier packages
- `~` used for medium-tier packages
- `⚠` used for risky-tier packages and warnings
- `✗` used for critical failures in doctor
- `⊘` used for skipped checks

Without a color-capable terminal in this audit environment, color rendering cannot be verified directly. Symbol selection is consistent and meaningful.

**Formatting:** Bold is used for package names in `explain` output (`Package: git`). Sections are separated by `────` divider lines. The overall visual hierarchy is clear.

**Spacing:** Whitespace is used effectively between sections. No excessive blank lines.

**Terminology:** Consistent throughout — "confidence score," "tier," "daemon," "snapshot" are used the same way across all commands and help text. No observed "service" vs "daemon" confusion.

**Symbols:** Unicode check/warning/cross marks are used throughout. They render as ASCII-friendly characters even in plain terminals.

**Headers/footers:** The tier summary banner (`SAFE: 5 · MEDIUM: 31 · RISKY: 4`) appears at the top of every `unused` output, and the `Reclaimable:` and `Hidden:` summary lines at the bottom provide good framing. The `Confidence: MEDIUM (2 events, tracking for 0 days)` footer is a strong addition — it contextualizes the recommendations relative to data maturity.

**Errors vs warnings:** `Error:` prefix on stderr for errors, `⚠` for in-output warnings. The distinction is clear.

**Specific visual defects:**
1. `remove --medium` header says "Removing 5 packages" during the progress bar but the actual removal target was 36 — this appears to be from a prior removal state in this audit run and may be session-specific.
2. `Removed 0 packages, freed 180 MB` is a contradictory summary line (see finding above).

---

## Positive Observations (What Works Well)

1. **Quickstart is excellent.** Four numbered steps with live feedback, a self-test, and a clear next-steps footer. This is the most important UX surface and it's polished.

2. **`unused` default view is well-calibrated.** Showing safe + medium by default (hiding risky) is the right choice. The tier summary banner gives full context without overwhelming.

3. **`explain` error messages are exemplary.** The nonexistent-package error message (`Check the name with 'brew list'... If you just installed it, run 'brewprune scan'`) is one of the best error messages in the tool.

4. **`doctor` is comprehensive.** Checking database, daemon, events, shim binary, PATH, AND running a live pipeline test gives genuine confidence. The tiered output (`✓`, `⚠`, `✗`, `⊘`) is well-designed.

5. **`remove` safety features work.** Snapshot creation is automatic, the progress bar is clear, and the post-removal undo hint is excellent UX.

6. **`undo` confirmation flow is correct.** Default-to-cancel `[y/N]` prompt, `--yes` bypass, detailed package list before confirming — all correct.

7. **Flag validation is strong.** Every invalid enum value produces a message naming the valid options. Every range violation names the valid range. This is consistently good across all commands.

8. **`stats` cross-reference tip** ("Run 'brewprune explain git' for removal recommendation") naturally guides users deeper into the tool.

9. **`--verbose` pagination tip** at the bottom of long verbose output is a practical addition even if its placement is late.

10. **Scan output** after `undo` correctly shows package install timestamps distinguishing newly-restored packages from existing ones.

---

## Recommendations Summary

### Priority 1 — Fix Before Shipping

| Finding | Fix |
|---------|-----|
| `remove --medium` reports "freed 180 MB" despite removing nothing | Fix the disk-freed calculation to use actual removal results, not planned candidates |
| `remove --medium` includes dependency-locked packages as candidates | Pre-validate candidates against `brew` dependency graph; exclude locked packages or flag them |
| `explain curl` intermittent exit 139 crash | Investigate and fix the segfault in the explain codepath for packages with "used dependents" |
| Shim tracking gap (PATH not active after quickstart) | Add a prominent warning in quickstart and status output when tracking is configured but not yet active in the current shell |

### Priority 2 — High-Impact Improvements

| Finding | Fix |
|---------|-----|
| `stats --days abc` exposes raw Go parse error | Catch the error and emit the same message as `--days -1` |
| `undo` restore output shows `Restored bat@` (missing version) | Fix the formatting to show full version or omit the `@` |
| `watch --daemon --stop` silently picks `--stop` | Return an error: "Error: --daemon and --stop are mutually exclusive" |
| `--sort age` table shows no age column and has undefined order when ages are equal | Add an "Installed" column when sorting by age; use name as deterministic secondary sort |
| `unused --verbose` with no tier filter dumps 36+ blocks | Show a "pipe to less" tip before the output, or only allow `--verbose` with a `--tier` filter |

### Priority 3 — Polish

| Finding | Fix |
|---------|-----|
| `unused --all` footer still says "(risky, hidden)" | Conditionally remove "hidden" label when risky is shown |
| `unused --tier safe --all` shows doubled "Error: Error:" | Fix the error formatting to single prefix |
| Daemon `watch.log` always empty | Write lifecycle events (start/stop/poll results) to watch.log |
| `status` shows "since 0 seconds ago" | Use "just now" for sub-5-second age |
| Doctor alias tip references `brewprune help` which has no alias docs | Point to actual docs, a URL, or include the format inline |
| Doctor alias tip appears even in fully-broken state | Suppress the alias tip when critical issues exist |
| Unknown commands give no "did you mean" suggestion | Implement fuzzy-match suggestions for common misses (list→unused, prune→remove) |
| `explain` score framing — "0/40 pts - used today" is counterintuitive | Clarify that 0 points = recent use = NOT safe to remove |

---

*End of Round 7 Audit Report*
