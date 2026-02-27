# brewprune

You have 100+ Homebrew packages installed. You use 20 of them. The other 80 are taking up 15GB of disk space, but you don't know which ones are safe to remove. `brew autoremove` only handles dependency chains—it doesn't track whether you actually *use* a package. Removing things manually is scary because one wrong move could break your workflow.

**brewprune solves this.** It monitors what you actually use, scores packages by removal safety, and creates automatic snapshots so you can undo any removal with one command. No guesswork. No fear. Just data-driven cleanup.

## Quick example

```bash
$ brewprune scan

📦 100 packages installed (14.2 GB)

Safe to remove (high confidence):
  node@16        2.1 GB    installed 142 days ago, 0 uses
  postgresql@14  890 MB    installed 89 days ago, 0 uses
  python@3.9     1.2 GB    installed 201 days ago, 0 uses

Medium confidence:
  imagemagick    450 MB    installed 67 days ago, last used 45 days ago

Risky (low confidence):
  git            180 MB    installed 12 days ago, used 847 times

Total reclaimable (safe only): 4.2 GB
```

```bash
$ brewprune remove --safe
Creating snapshot... ✓
Removing node@16...
Removing postgresql@14...
Removing python@3.9...

Removed 3 packages, reclaimed 4.2 GB
```

```bash
# If something breaks, rollback is one command:
$ brewprune undo
Restored snapshot from 2026-02-26 14:23
Reinstalled 3 packages
```

## How it works

**FSEvents monitoring**
brewprune watches `/usr/local/bin`, `/opt/homebrew/bin`, and `/Applications` for file access. When you run a Homebrew-installed binary, it gets logged with a timestamp.

**SQLite usage database**
All usage data lives in `~/.brewprune/usage.db`. No cloud sync, no network calls—everything stays local.

**Confidence scoring**
Packages are scored based on:
- **Install age** - Older installations with no usage → high removal confidence
- **Usage frequency** - Never used → safe, used recently → risky
- **Dependency status** - Leaf packages (no dependents) → safer to remove
- **Recency** - Last used 3+ months ago → medium confidence

Scoring thresholds:
- **Safe** - Installed 60+ days ago, 0-1 uses OR last used 90+ days ago
- **Medium** - Installed 30+ days ago, moderate usage OR last used 30-90 days ago
- **Risky** - Recent installation, frequent usage, or used in last 30 days

**Automatic snapshots**
Before any removal, brewprune creates a snapshot containing:
- List of removed packages and versions
- Removal timestamp
- Disk space reclaimed

Rollback reinstalls exact versions from the snapshot.

## Commands

| Command | Description |
|---------|-------------|
| `brewprune scan` | Analyze installed packages and show removal candidates |
| `brewprune remove --safe` | Remove packages with high confidence scores |
| `brewprune remove --medium` | Remove packages with medium+ confidence scores |
| `brewprune remove <package>` | Remove a specific package (creates snapshot) |
| `brewprune undo` | Restore the most recent snapshot |
| `brewprune list-snapshots` | Show all snapshots with rollback points |
| `brewprune monitor` | Start usage monitoring daemon (runs in background) |
| `brewprune status` | Show monitoring status and database stats |

### Usage examples

```bash
# Start monitoring (do this once, runs in background)
$ brewprune monitor --daemon

# After a few weeks, scan for cleanup candidates
$ brewprune scan

# Remove only the safest packages
$ brewprune remove --safe

# Or be more aggressive
$ brewprune remove --medium

# Check what was removed
$ brewprune list-snapshots

# Rollback if something broke
$ brewprune undo
```

## Installation

### From source

```bash
go install github.com/yourusername/brewprune@latest
```

### Homebrew tap (coming soon)

```bash
brew tap yourusername/brewprune
brew install brewprune
```

### First-time setup

After installation, start the monitoring daemon:

```bash
brewprune monitor --daemon
```

This runs in the background and tracks package usage via FSEvents. Let it run for at least a week before doing cleanup—more data means better confidence scores.

## Privacy

brewprune is completely local:
- All data stays in `~/.brewprune/usage.db` (SQLite)
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
A: Run `brewprune undo` to restore the exact packages you removed. Snapshots include version info, so you get back exactly what you had.

**Q: Does this track what I do in my terminal?**
A: No. It only tracks *which binaries are executed*, not what you pass to them. If you run `git commit -m "secret"`, brewprune only sees "git was used at 2:34pm"—nothing about arguments or content.

**Q: Can I exclude packages from removal?**
A: Yes. Add packages to `~/.brewprune/exclude.txt` (one per line) and they'll never show up in removal candidates.

**Q: Does this work with Homebrew Cask?**
A: Yes. brewprune monitors both `/usr/local/bin` (formulae) and `/Applications` (casks).

**Q: What if I use a package via a script?**
A: As long as the script executes the binary, FSEvents will catch it. If you only import a library (e.g., Python/Ruby gems installed via Homebrew), brewprune won't detect usage—be careful with `--medium` in this case.

## Roadmap

- [ ] Web UI for browsing usage history
- [ ] Export reports (JSON, CSV)
- [ ] Scheduled scanning (weekly email summaries)
- [ ] Integration with `brew bundle` for reproducible environments
- [ ] Dependency tree visualization
- [ ] "Dry run" mode with detailed impact analysis

## License

MIT

## Contributing

Pull requests welcome. For major changes, open an issue first.

---

**Made with ☕️ by a developer with 237 Homebrew packages and 32GB of disk anxiety.**
