# Roadmap

Items are grouped by priority. Within each group, order reflects implementation sequence (earlier items unblock later ones).

---

## Vision: brewprune as a natural extension of brew

The goal is for brewprune to feel like a missing feature of Homebrew, not a separate tool you have to babysit. Three things must be true:

1. **It installs and starts like brew things do** — `brew install`, `brew services start`, done.
2. **It behaves like brew commands** — verbs, output style, and defaults match what brew users expect.
3. **It never feels like a separate system** — no manual sync steps, no "remember to rescan."

**Elevator pitch:** *Homebrew can tell you what's installed and what depends on what. It can't tell you what you actually use. brewprune adds that missing signal, and makes cleanup reversible.*

### Decided: keep existing verbs
`unused`, `remove`, `undo` are clear and don't need renaming. Aliases create two competing mental models via docs drift. If a rename ever happens it's a single v0.2.0 breaking change, never an alias. Until then: make the UX and output feel brew-native without touching the verb names.

---

## High priority

### `brew services` integration
Make `brew services start brewprune` the primary path. Add a service `do` stanza to the formula:
```bash
brew services start brewprune
brew services stop brewprune
brew services restart brewprune
```
Update Quick Start and `brewprune quickstart` so the blessed workflow becomes:
```bash
brew install brewprune
brewprune scan
brew services start brewprune
```
`brewprune watch --daemon` stays as a fallback for source installs. Drop the launchd plist from the primary docs path (keep in Daemon Mode as reference only).

**Why:** people already trust `brew services` for daemon lifecycle. Removes the entire "daemon" mental model for most users.

---

### Shim upgrade staleness fix (two-pronged)
When users run `brew upgrade brewprune`, the shim binary at `~/.brewprune/bin/brewprune-shim` becomes stale — it's a copy of the old binary. Tracking silently breaks until the user remembers to rescan. This is the most "not brew-native" failure mode.

**Primary fix — formula `post_install` hook:**
After every brew install or upgrade, run `brewprune scan --refresh-shims` (fast path, no full rescan) to rebuild the shim binary and add any new symlinks.

**Belt-and-suspenders — shim version check:**
Embed a build version in the shim binary. On startup, compare against a version file at `~/.brewprune/shim.version`. If mismatch, log a rate-limited warning once:
```
brewprune upgraded; run 'brewprune scan' to refresh shims (or 'brewprune doctor').
```
This catches source installs and any case where the formula hook didn't run.

**Why:** "I upgraded and it silently stopped tracking" is the first-user failure mode that kills trust.

---

### `brewprune quickstart` as the one blessed workflow
`quickstart` should handle the full setup end-to-end — no separate docs steps required:
1. Run `brewprune scan`
2. Verify `~/.brewprune/bin` is in PATH (write to the correct shell config if not)
3. Start the service (`brew services start brewprune` if available, else `brewprune watch --daemon`)
4. Run a self-test and confirm "tracking verified" before exiting

First users should be able to run `brewprune quickstart` and have a fully working setup with zero separate steps.

---

### `brewprune doctor` end-to-end self-test
`doctor` currently checks static state (binary exists, daemon running, PATH order). It does not prove the pipeline actually works.

Add a `brewprune _shimtest` internal command that:
1. Runs a known shimmed binary (or a no-op built-in)
2. Waits one daemon poll cycle (≤30s)
3. Confirms the event appears in the database
4. Reports pass/fail with timing

Surface this in `brewprune doctor` as a live pipeline check. Also add unit tests asserting:
- shim never execs itself (`findRealBinary("brewprune-shim") == ""`)
- real path resolution never resolves into `~/.brewprune/bin`

**Why:** would have caught the exec loop before it shipped. Cuts "why is nothing being tracked" issues entirely.

---

### Exec path disambiguation in daemon
`buildBasenameMap` maps `filepath.Base(binPath) → pkg.Name`. If two formulae ship the same binary name (e.g. `convert` from ImageMagick and another package), last-write wins silently.

The shim already logs the full path (`/Users/.../.brewprune/bin/git`). The real binary resolves to `/opt/homebrew/bin/git`. Use the opt path for matching instead of basename alone.

**Why:** correctness issue, not polish. Silent mis-attribution of usage events skews scores.

---

## Medium priority

### Brew-aware stale detection
After `brew install` or `brew upgrade`, brewprune's package DB drifts silently (new packages aren't shimmed). Two layers:

**A. Lightweight — prompt on drift:**
On any command that reads the package DB (`unused`, `stats`, `remove`), compare last scan timestamp against `brew list`. If new packages exist, print:
```
New formulae detected since last scan. Run 'brewprune scan' to update shims.
```

**B. Opt-in brew hook:**
Wrap `brew` in a zsh function or use `~/.config/homebrew/brew-wrap` to trigger `brewprune scan --refresh-shims` automatically after installs/upgrades. Opt-in only.

**Why:** "I installed something and forgot to scan" is the most common way tracking silently degrades.

---

### Incremental scan (`--refresh-shims`)
`brewprune scan` re-indexes all packages every time (~6s). Add `brewprune scan --refresh-shims` that:
1. Diffs current `brew list` against the DB
2. Only adds/removes symlinks for changed packages
3. Skips the full dep tree rebuild if nothing structural changed

Required before formula `post_install` hook can be fast enough to run on every upgrade.

---

### "Why it's safe" inline at removal time
Before the removal confirmation prompt, show a one-line score summary per package:
```
node@16    90/100  SAFE    never used · no dependents · installed 8 months ago
```
Keep `--yes` to skip for scripts.

**Why:** makes "safe" mean something at the moment it matters.

---

### brew-native `status` output
Mirror what `brew services list` feels like — structured, scannable:
```
Tracking:     running (since 2 days ago, PID 17430)
Events:       1,842 total  ·  38 in last 24h
Shims:        active  ·  660 commands  ·  PATH ok
Last scan:    2 days ago  ·  166 formulae  ·  6.1 GB
Data quality: COLLECTING (7 of 14 days)
```
Use brew terminology throughout: "formulae" not "packages", "casks: not tracked (by design)".

---

### Shell completions
Ship zsh (and bash/fish) completions. Cobra can generate these automatically. Include in the formula's `def install` block.

---

### "Used" ≠ "needed" — UX clarity
Some packages are critical without being directly executed (daemons, imports, services). The scoring is still correct overall, but the UX should be explicit:
- Add a disclaimer line to `brewprune unused` output: *"Safe = low observed execution risk. Review medium/risky tiers before removing libraries or daemons."*
- Expand protected packages list with common daemon-only packages

---

## Low priority / research

### Docs restructure to mirror brew docs
Use headings that match the brew documentation mental model:
- Install → Services → Usage → Uninstall / rollback → Troubleshooting

Reframe the "daemon requirement" as "Enable tracking" — same truth, brew-native phrasing.

---

## Already done

- PATH shims replacing fsnotify (v0.1.3)
- Shim binary bundled in Homebrew formula (v0.1.4)
- Exec loop guard: `findRealBinary` returns `""` for `brewprune-shim` (v0.1.5)
- `IsCritical` flag + 47 protected packages capped at score 70
- `brewprune explain` for per-package score breakdown
- `brewprune doctor` for static health checks
- `brewprune quickstart` interactive setup
- Crash-safe offset tracking (temp-file rename)
- Automatic snapshots + `brewprune undo`
