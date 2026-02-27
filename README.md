# brewprune

[![Blackwell Systems™](https://raw.githubusercontent.com/blackwell-systems/blackwell-docs-theme/main/badge-trademark.svg)](https://github.com/blackwell-systems)
[![CI](https://github.com/blackwell-systems/brewprune/actions/workflows/ci.yml/badge.svg)](https://github.com/blackwell-systems/brewprune/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/blackwell-systems/brewprune)](https://goreportcard.com/report/github.com/blackwell-systems/brewprune)
[![Go Version](https://img.shields.io/github/go-mod/go-version/blackwell-systems/brewprune)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

You have 100+ Homebrew packages installed. You use 20 of them. The other 80 are taking up 15GB of disk space, but you don't know which ones are safe to remove. `brew autoremove` only handles dependency chains—it doesn't track whether you actually *use* a package. Removing things manually is scary because one wrong move could break your workflow.

**brewprune solves this.** It monitors what you actually use, scores packages by removal safety, and creates automatic snapshots so you can undo any removal with one command. No guesswork. No fear. Just data-driven cleanup.

## Quick example

```bash
$ brewprune unused --tier safe

Package              Score    Tier       Last Used     Reason
────────────────────────────────────────────────────────────────────────────────
node@16              85       SAFE       never         never used, no dependents
postgresql@14        90       SAFE       never         never used, no dependents
python@3.9           82       SAFE       6 months ago  rarely used, safe to remove

Summary: 3 safe, 2 medium, 1 risky packages
```

```bash
$ brewprune remove --safe
Creating snapshot...
Snapshot created: ID 1

Removing 3 packages...

✓ Removed 3 packages, freed 4.2 GB

Snapshot: ID 1
Undo with: brewprune undo 1
```

```bash
# If something breaks, rollback is one command:
$ brewprune undo latest
Restoring 3 packages from snapshot...

✓ Restored 3 packages from snapshot 1

Run 'brewprune scan' to update the package database.
```

## How it works

**FSEvents monitoring**
brewprune watches Homebrew binary directories (`$(brew --prefix)/bin`, `/Applications`) for file access. When you run a Homebrew-installed binary, it gets logged with a timestamp.

**SQLite usage database**
All usage data lives in `~/.brewprune/brewprune.db`. No cloud sync, no network calls—everything stays local.

**Confidence scoring**
Each package receives a score from 0-100 based on four weighted factors:

**Total Score = Usage (40pts) + Dependencies (30pts) + Age (20pts) + Type (10pts)**

- **Usage Score (40 points):**
  - Last 7 days: 40 points
  - Last 30 days: 30 points
  - Last 90 days: 20 points
  - Last year: 10 points
  - Never used: 0 points

- **Dependencies Score (30 points):**
  - No dependents: 30 points
  - 1-3 unused dependents: 20 points
  - 1-3 used dependents: 10 points
  - 4+ dependents: 0 points

- **Age Score (20 points):**
  - Installed >180 days: 20 points
  - Installed >90 days: 15 points
  - Installed >30 days: 10 points
  - Installed <30 days: 0 points

- **Type Score (10 points):**
  - Leaf package with binaries: 10 points
  - Library with no binaries: 5 points
  - Core dependency: 0 points

**Confidence Tiers:**
- **Safe (80-100):** High confidence, minimal impact
- **Medium (50-79):** Review before removal
- **Risky (0-49):** Keep unless certain

**Automatic snapshots**
Before any removal, brewprune creates a snapshot in `~/.brewprune/snapshots/` containing:
- Package names and exact versions
- Tap sources for reinstallation
- Timestamp and reason for snapshot

Rollback reinstalls packages from the snapshot using `brew install`.

## Commands

| Command | Description |
|---------|-------------|
| `brewprune scan` | Scan and index installed Homebrew packages |
| `brewprune watch` | Monitor package usage via filesystem events (can run as daemon) |
| `brewprune unused [--tier safe\|medium\|risky]` | List unused packages with confidence scores |
| `brewprune stats [--days N] [--package NAME]` | Show usage statistics for packages |
| `brewprune remove [--tier safe\|medium\|risky] [packages...]` | Remove unused packages (creates snapshot) |
| `brewprune undo [snapshot-id\|latest]` | Restore packages from a snapshot |

### Common workflow

```bash
# 1. Initial scan
$ brewprune scan

# 2. Start monitoring (runs in background)
$ brewprune watch --daemon

# 3. After a week or two, view unused packages
$ brewprune unused --tier safe

# 4. Check usage statistics
$ brewprune stats --days 30

# 5. Remove safe packages with dry-run first
$ brewprune remove --safe --dry-run

# 6. Actually remove them (creates snapshot automatically)
$ brewprune remove --safe

# 7. If something breaks, rollback
$ brewprune undo latest

# 8. Stop the monitoring daemon when not needed
$ brewprune watch --stop
```

### Key flags

**`brewprune unused` flags:**
- `--tier safe|medium|risky` - Filter by confidence tier
- `--min-score N` - Minimum confidence score (0-100)
- `--sort score|size|age` - Sort order

**`brewprune remove` flags:**
- `--safe` - Remove only safe-tier packages
- `--medium` - Remove safe + medium-tier packages
- `--risky` - Remove all unused packages (requires confirmation)
- `--dry-run` - Show what would be removed without removing
- `--yes` - Skip confirmation prompts
- `--no-snapshot` - Skip automatic snapshot creation (dangerous!)

**`brewprune watch` flags:**
- `--daemon` - Run as background daemon
- `--stop` - Stop running daemon
- `--pid-file PATH` - Custom PID file location
- `--log-file PATH` - Custom log file location

**`brewprune undo` flags:**
- `--list` - List all available snapshots
- `--yes` - Skip confirmation prompt

**`brewprune stats` flags:**
- `--days N` - Time window in days (default: 30)
- `--package NAME` - Show stats for specific package

## Installation

### From source

```bash
go install github.com/blackwell-systems/brewprune@latest
```

### Homebrew tap (coming soon)

```bash
brew tap blackwell-systems/brewprune
brew install brewprune
```

### First-time setup

After installation, run an initial scan and start the monitoring daemon:

```bash
# Scan existing packages
brewprune scan

# Start background monitoring
brewprune watch --daemon
```

The daemon runs in the background and tracks package usage via FSEvents. Let it run for at least a week before doing cleanup—more data means better confidence scores.

## Privacy

brewprune is completely local:
- All data stays in `~/.brewprune/brewprune.db` (SQLite)
- Snapshots stored in `~/.brewprune/snapshots/`
- No network calls
- No telemetry
- No cloud sync

Usage tracking only monitors Homebrew binary paths. It doesn't track arguments, file contents, or browsing history.

## Comparison to alternatives

### vs `brew autoremove`
- `brew autoremove` only removes dependencies that are no longer needed by other packages
- It doesn't track whether *you* actually use a package
- brewprune tracks real usage and removes packages you never touch, even if they're not technically orphaned

### vs manual cleanup
- Manual cleanup is guesswork—you don't know what you last used or when
- brewprune gives you confidence scores based on actual data
- Automatic snapshots mean you can always undo if you remove the wrong thing

### vs `brew cleanup`
- `brew cleanup` removes old versions and cache files
- brewprune removes entire unused packages
- Different use cases—run both for maximum disk reclamation

## FAQ

**Q: How long should I wait before running cleanup?**
A: At least 1-2 weeks of monitoring. The longer you wait, the better the confidence scores.

**Q: What happens if I remove something I need?**
A: Run `brewprune undo latest` to restore the exact packages you removed. Snapshots include version info, so you get back exactly what you had.

**Q: Does this track what I do in my terminal?**
A: No. It only tracks *which binaries are executed*, not what you pass to them. If you run `git commit -m "secret"`, brewprune only sees "git was used at 2:34pm"—nothing about arguments or content.

**Q: Does this work with Homebrew Cask?**
A: Yes. brewprune monitors both Homebrew bin directories (formulae) and `/Applications` (casks).

**Q: What if I use a package via a script?**
A: As long as the script executes the binary, FSEvents will catch it. If you only import a library (e.g., Python/Ruby gems installed via Homebrew), brewprune won't detect usage—be careful with `--medium` and `--risky` in this case.

**Q: How do I see what snapshots I have?**
A: Run `brewprune undo --list` to see all available snapshots with their IDs, creation times, and package counts.

## Roadmap

- [x] Confidence-based package scoring
- [x] Automatic snapshot creation and rollback
- [x] Daemon mode for background monitoring
- [x] Dry-run mode for safe previews
- [ ] Web UI for browsing usage history
- [ ] Export reports (JSON, CSV)
- [ ] Integration with `brew bundle` for reproducible environments
- [ ] Dependency tree visualization

## License

MIT

## Contributing

Pull requests welcome. For major changes, open an issue first.

---

**Made with ☕️ by a developer with 237 Homebrew packages and 32GB of disk anxiety.**
