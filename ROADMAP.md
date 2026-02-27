# Roadmap

Items are grouped by priority. Within each group, order reflects implementation sequence (earlier items unblock later ones).

---

## Vision: brewprune as a natural extension of brew

The goal is for brewprune to feel like a missing feature of Homebrew, not a separate tool you have to babysit. Three things must be true for that to land:

1. **It installs and starts like brew things do** — `brew install`, `brew services start`, done.
2. **It behaves like brew commands** — verbs, output style, and defaults match what brew users expect.
3. **It never feels like a separate system** — no manual sync steps, no "remember to rescan."

**Elevator pitch:** *Homebrew can tell you what's installed and what depends on what. It can't tell you what you actually use. brewprune adds that missing signal, and makes cleanup reversible.*

The items below are ordered to get there.

---

## High priority

### `brew services` integration
Wire `brewprune watch --daemon` into Homebrew's service manager. Add a service `do` stanza to the formula so these work out of the box:
```bash
brew services start brewprune
brew services stop brewprune
brew services restart brewprune
```
`brewprune watch --daemon` stays as a fallback for source installs, but the primary Quick Start becomes:
```bash
brew install brewprune
brewprune scan
brew services start brewprune
```
Keep the launchd plist docs as a fallback, but don't lead with it.

**Why:** people already trust `brew services` for daemon lifecycle. One line of install experience eliminates the entire "daemon" mental model for most users.

---

### `brewprune doctor` end-to-end self-test
`doctor` currently checks static state (binary exists, daemon running, PATH order). It does not prove the pipeline actually works.

Add a `brewprune _shimtest` internal command that:
1. Runs a known shimmed binary (or a no-op built-in)
2. Waits one daemon poll cycle (≤30s)
3. Confirms the event appears in the database
4. Reports pass/fail with timing

Surface this in `brewprune doctor` as a live check. Also add unit tests asserting:
- shim never execs itself (`findRealBinary("brewprune-shim") == ""`)
- real path resolution never resolves into `~/.brewprune/bin`

**Why:** would have caught the exec loop before it shipped, and cuts "why is nothing being tracked" support issues entirely.

---

### Exec path disambiguation in daemon
`buildBasenameMap` maps `filepath.Base(binPath) → pkg.Name`. If two formulae ship the same binary name (e.g. `convert` from ImageMagick and another package), last-write wins silently.

The shim already logs the full path (`/Users/.../.brewprune/bin/git`), and the real binary resolves to `/opt/homebrew/bin/git`. Use the opt path for matching instead of basename alone to eliminate collision ambiguity.

**Why:** correctness issue. Silent mis-attribution of usage events skews scores.

---

## Medium priority

### "Why it's safe" inline at removal time
`brewprune explain <pkg>` exists but isn't surfaced during `brewprune remove`. Before the removal confirmation prompt, show a one-line score summary per package:

```
node@16    90/100  SAFE    never used · no dependents · installed 8 months ago
```

Keep `--yes` to skip it for scripts.

**Why:** makes "safe" mean something visible at the moment it matters. Reduces second-guessing and accidental removals.

---

### Incremental scan (`--refresh-shims`)
`brewprune scan` re-indexes all 166 packages every time (~6s). Add `brewprune scan --refresh-shims` that:
1. Diffs current `brew list` against the DB
2. Only adds/removes symlinks for changed packages
3. Skips the full dep tree rebuild if nothing structural changed

Optionally: detect new Homebrew installs automatically and prompt the user.

**Why:** scan friction compounds over time. New packages aren't shimmed until the user remembers to rescan.

---

### Brew-aware stale detection
After `brew install` or `brew upgrade`, brewprune's package DB drifts silently. Fix this with two layers:

**A. Lightweight — prompt on drift:**
On any command that reads the package DB (`unused`, `stats`, `remove`), compare last scan timestamp against current `brew list` output. If new packages exist, print:
```
New packages detected since last scan. Run 'brewprune scan' to update shims.
```

**B. Better — opt-in brew hook:**
Provide an install step that wraps `brew` in a zsh function or uses `~/.config/homebrew/brew-wrap` to trigger `brewprune scan --refresh-shims` automatically after `brew install` / `brew upgrade`. This is opt-in only (too invasive as a default).

**Why:** "I installed something and forgot to scan" is the most common way the system silently stops tracking new tools.

---

## Medium priority

### "Why it's safe" inline at removal time
`brewprune explain <pkg>` exists but isn't surfaced during `brewprune remove`. Before the removal confirmation prompt, show a one-line score summary per package:

```
node@16    90/100  SAFE    never used · no dependents · installed 8 months ago
```

Keep `--yes` to skip it for scripts.

**Why:** makes "safe" mean something visible at the moment it matters. Reduces second-guessing and accidental removals.

---

### brew-native `status` output
`brewprune status` should mirror what `brew services list` feels like — structured, scannable, no jargon:
```
Tracking:     running (since 2 days ago, PID 17430)
Events:       1,842 total  ·  38 in last 24h
Shims:        active  ·  660 commands  ·  PATH ok
Last scan:    2 days ago  ·  166 formulae  ·  6.1 GB
Data quality: COLLECTING (7 of 14 days)
```
Also use brew's terminology in output throughout: "formulae" not "packages", "casks: not tracked (by design)".

**Why:** output is the product surface users see most. Matching brew's tone makes brewprune feel native, not bolted on.

---

### Shell completions
Ship zsh (and optionally bash/fish) completions for all commands and flags. Cobra can generate these automatically. Include them in the Homebrew formula's `def install` block.

**Why:** brew tools that feel native have completions. Small polish signal that matters to power users.

---

### "Used" ≠ "needed" — UX improvements
Some packages are critical without being directly executed: background daemons, launchd services, language runtimes imported (not invoked), cron jobs. The scoring algorithm can still produce correct recommendations overall, but the UX should be explicit:

- Replace "SAFE" label with "SAFE TO REMOVE" in output
- Add a disclaimer line to `brewprune unused` output: *"Safe means low observed execution risk, not guaranteed safe. Review medium/risky tiers carefully."*
- Expand the protected packages list with common daemon-only packages

---

## Low priority / research

### Brew-y verb aliases
Brew uses `install`, `uninstall`, `list`, `info`. Consider aliases:
- `brewprune uninstall` → `brewprune remove`
- `brewprune restore` → `brewprune undo`
- `brewprune list --unused` → `brewprune unused`

Don't rename — alias only. Keep existing commands working. Decide after watching how users actually talk about the tool.

---

### Docs restructure to mirror brew docs
Use headings that match the brew documentation mental model:
- Install → Services → Usage → Uninstall / rollback → Troubleshooting

Reframe the "daemon requirement" as "Enable tracking (recommended)" — same truth, brew-native phrasing.

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
