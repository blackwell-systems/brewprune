# Roadmap

Items are grouped by priority. Within each group, order reflects implementation sequence (earlier items unblock later ones).

---

## High priority

### `brew services` integration
Wire `brewprune watch --daemon` into Homebrew's service manager so the happy path becomes:
```bash
brewprune scan
brew services start brewprune
```
Add a `[Service]` block to the Homebrew formula. Keep the launchd plist as a fallback for users who installed from source.

**Why:** eliminates the biggest first-run friction point. Daemon auto-starts on login without any manual setup.

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

## Low priority / research

### "Used" ≠ "needed" — UX improvements
Some packages are critical without being directly executed: background daemons, launchd services, language runtimes imported (not invoked), cron jobs. The scoring algorithm can still produce correct recommendations overall, but the UX should be explicit:

- Replace "SAFE" label with "SAFE TO REMOVE" in output
- Add a disclaimer line to `brewprune unused` output: *"Safe means low observed execution risk, not guaranteed safe. Review medium/risky tiers carefully."*
- Expand the protected packages list with common daemon-only packages

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
