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
