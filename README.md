# brewprune

[![Blackwell Systems™](https://raw.githubusercontent.com/blackwell-systems/blackwell-docs-theme/main/badge-trademark.svg)](https://github.com/blackwell-systems)
[![CI](https://github.com/blackwell-systems/brewprune/actions/workflows/ci.yml/badge.svg)](https://github.com/blackwell-systems/brewprune/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/blackwell-systems/brewprune)](https://goreportcard.com/report/github.com/blackwell-systems/brewprune)
[![Go Version](https://img.shields.io/github/go-mod/go-version/blackwell-systems/brewprune)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

You have 100+ Homebrew packages installed. You use 20 of them. The other 80 are taking up 15GB of disk space, but you don't know which ones are safe to remove. `brew autoremove` only handles dependency chains—it doesn't track whether you actually *use* a package. Removing things manually is scary because one wrong move could break your workflow.

**brewprune solves this.** It monitors what you actually use, scores packages by removal safety, and creates automatic snapshots so you can undo any removal with one command. Less guesswork. Just data-driven cleanup.

**Requirements:**
- macOS 12+ (uses FSEvents for monitoring)
- Homebrew installed
- Formula support: full | Cask support: best-effort

**Privacy:**
- 100% local (all data in ~/.brewprune)
- No telemetry, no cloud sync, no network calls
- Only tracks FSEvents on Homebrew paths (not commands/arguments)

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

## Installation

### From Homebrew (recommended)

```bash
brew tap blackwell-systems/tap
brew install brewprune
```

### From source

```bash
go install github.com/blackwell-systems/brewprune@latest
```

## Quick Start (CRITICAL: Don't skip step 2!)

**New to brewprune? Try the interactive setup:**
```bash
brewprune quickstart
```

**Or follow these steps manually:**

**1. Scan your packages:**
```bash
brewprune scan
```

**2. ⚠️ START THE DAEMON (required for tracking):**
```bash
brewprune watch --daemon
```

**Without the daemon running, brewprune cannot track usage and all packages will show "never used".**

**3. Wait 1-2 weeks** for meaningful usage data, then:
```bash
brewprune unused --tier safe
```

**4. (Optional) Auto-start on login:**

macOS launchd service:
```bash
# Create service file
cat > ~/Library/LaunchAgents/com.blackwell-systems.brewprune.plist << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.blackwell-systems.brewprune</string>
    <key>ProgramArguments</key>
    <array>
        <string>/opt/homebrew/bin/brewprune</string>
        <string>watch</string>
        <string>--daemon</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
EOF

# Load the service
launchctl load ~/Library/LaunchAgents/com.blackwell-systems.brewprune.plist
```

Or add to `~/.zshrc`:
```bash
if ! pgrep -f "brewprune watch" > /dev/null; then
    brewprune watch --daemon
fi
```

## How it works

**Filesystem Monitoring**
brewprune watches Homebrew binary paths for filesystem activity consistent with execution, then records timestamps for the owning package. It uses macOS FSEvents to monitor:
- `$(brew --prefix)/bin` and `/opt/homebrew/bin` for formula binaries
- `/Applications` for cask app bundles

When activity occurs, brewprune records a usage timestamp for the owning package.

**Package Size Calculation**
During scan, brewprune calculates actual disk usage for each package using `du -sk` on the Cellar/Caskroom directories. This enables sorting by size and shows real space savings potential.

**SQLite Storage**
All usage data lives in `~/.brewprune/brewprune.db`. No cloud sync, no network calls.

**Heuristic Scoring**
Packages are scored 0-100 based on:
- **Usage (40 points):** Last 7d=40, 30d=30, 90d=20, 1yr=10, never=0
- **Dependencies (30 points):** No dependents=30, 1-3 unused=20, 1-3 used=10, 4+=0
- **Age (20 points):** >180d=20, >90d=15, >30d=10, <30d=0
- **Type (10 points):** Leaf with binaries=10, library=5, core dependency=0

**Tiers:**
- **Safe (80-100):** High likelihood of being unused
- **Medium (50-79):** Review before removal
- **Risky (0-49):** Keep unless certain

**This is a heuristic, not a guarantee.** Score is a heuristic based on observed patterns. Always review medium/risky packages before removal.

**Automatic Snapshots**
Before removal, brewprune creates a snapshot containing:
- Package names and installed versions
- Tap sources
- Removal timestamp

Snapshots enable rollback via `brewprune undo`. Exact version restoration depends on Homebrew bottle/formula availability.

## Safety & Risks

**What can go wrong:**
- Remove a package you need → Undo with `brewprune undo latest` (restores if versions available)
- Remove a library used only via imports → Check medium/risky packages carefully before removing
- Daemon stops silently → Run `brewprune status` to check, restart if needed

**What CANNOT go wrong:**
- Core dependencies protected (openssl, git, coreutils, etc.) - capped at "medium" tier
- Snapshots created automatically before every removal
- No system files touched (only Homebrew packages)
- All changes reversible via `brew install` even without snapshots

**If something breaks:** `brewprune undo latest` restores packages immediately.

## Timeline & Expectations

**Day 1:** Install → scan → start daemon → verify with `brewprune status`

**Days 2-14:** Daemon collects usage data in background (no action needed)

**Day 14+:** View unused packages → review carefully → remove safe tier with `--dry-run` first

**Ongoing:** Rescan after manual brew installs (`brewprune scan`), check status occasionally

**Important:** First 1-2 weeks show "never used" for most packages because tracking hasn't captured your workflow patterns yet. This is normal - wait for meaningful data.

## Limitations & Accuracy

**What brewprune tracks:**
- Executed binaries installed by Homebrew formulae
- App bundle access in /Applications (as a proxy for cask usage)
- Filesystem activity via FSEvents (not direct process execution)

**What it doesn't track:**
- Language imports (Python/Ruby/Node modules) unless a binary is executed
- Packages used only via `import` statements
- Background processes that don't execute binaries

**Accuracy notes:**
- First 1-2 weeks may show misleading "never used" scores (insufficient data)
- Cask usage detection is best-effort (app bundle access does not equal guaranteed user launch)
- Libraries without binaries will appear unused if only imported, not executed
- Score is a **heuristic**, not a certainty—always review before removing

## Commands

| Command | Description |
|---------|-------------|
| `brewprune quickstart` | Interactive setup walkthrough for first-time users |
| `brewprune doctor` | Diagnose issues and check system health |
| `brewprune status` | Check daemon status and tracking statistics |
| `brewprune scan` | Scan and index installed Homebrew packages |
| `brewprune watch [--daemon]` | Monitor package usage via filesystem events |
| `brewprune unused [--tier safe\|medium\|risky]` | List packages with heuristic scores |
| `brewprune stats [--days N] [--package NAME]` | Show usage statistics |
| `brewprune remove [--safe\|--medium\|--risky] [packages...]` | Remove packages (creates snapshot) |
| `brewprune undo [snapshot-id\|latest]` | Restore from snapshot |

**Note:** `unused` uses `--tier`, `remove` uses boolean flags `--safe/--medium/--risky`

**For complete command reference with all flags, exit codes, and detailed examples, see [CLI Reference](docs/CLI.md).**

### Common workflow

```bash
# 1. Initial scan
$ brewprune scan

# 2. Start monitoring (runs in background)
$ brewprune watch --daemon

# 3. After 1-2 weeks minimum, view unused packages
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
- `--tier safe|medium|risky` - Filter by heuristic tier
- `--min-score N` - Minimum heuristic score (0-100)
- `--sort score|size|age` - Sort order (score: highest first, size: largest first, age: oldest first)

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

## Daemon Mode

`brewprune watch --daemon` runs monitoring in the background:

**How it works:**
- Forks a background process that survives terminal closure
- Writes PID to `~/.brewprune/watch.pid`
- Logs to `~/.brewprune/watch.log`
- Batches filesystem events and writes to database every 30 seconds

**Management:**
```bash
# Start daemon
brewprune watch --daemon

# Check status
brewprune watch  # Shows if daemon is running

# Stop daemon
brewprune watch --stop

# View logs
tail -f ~/.brewprune/watch.log
```

**Permissions:** No special permissions required (doesn't need Full Disk Access)

**Start on login:** Add to launchd or use `brew services` (formula includes service)

## Troubleshooting

If you're experiencing issues, run the diagnostic tool:

```bash
brewprune doctor
```

This checks:
- Database exists and is accessible
- Daemon is running
- Usage events are being recorded
- Provides specific fix commands for any issues found

**Common issues:**

- **No usage data after days**: Check `brewprune status` to verify daemon is running
- **Can't find database**: Run `brewprune scan` to initialize
- **Daemon not running**: Start with `brewprune watch --daemon`

## Privacy

brewprune is completely local:
- All data stays in `~/.brewprune/brewprune.db` (SQLite)
- Snapshots stored in `~/.brewprune/snapshots/`
- No network calls
- No telemetry
- No cloud sync

Usage tracking only monitors filesystem activity on Homebrew binary paths. It doesn't track arguments, file contents, or browsing history.

## Comparison to alternatives

### vs `brew autoremove`
- `brew autoremove` only removes dependencies that are no longer needed by other packages
- It doesn't track whether *you* actually use a package
- brewprune tracks real usage and removes packages you never touch, even if they're not technically orphaned

### vs manual cleanup
- Manual cleanup is guesswork—you don't know what you last used or when
- brewprune gives you heuristic scores based on actual data
- Automatic snapshots mean you can always undo if you remove the wrong thing

### vs `brew cleanup`
- `brew cleanup` removes old versions and cache files
- brewprune removes entire unused packages
- Different use cases—run both for maximum disk reclamation

## FAQ

**Q: How long should I wait before running cleanup?**
A: At least 1-2 weeks of monitoring. The longer you wait, the better the heuristic scores.

**Q: What happens if I remove something I need?**
A: Run `brewprune undo latest` to reinstall the same package set (and specific versions when available from Homebrew). Exact version restoration depends on Homebrew bottle/formula availability.

**Q: What exactly does brewprune track?**
A: brewprune records package name + timestamp when Homebrew-managed executables or app bundles show filesystem activity consistent with use. **It does not record:**
- Command arguments
- File contents
- Shell history
- Terminal commands
- Network activity

It only knows "this binary/app was accessed at this time."

**Q: Does this work with Homebrew Cask?**
A: Yes, with best-effort accuracy. brewprune monitors both Homebrew bin directories (formulae) and `/Applications` (casks). Cask usage detection is based on app bundle access, which is a proxy for user launch.

**Q: What if I use a package via a script?**
A: As long as the script executes the binary, FSEvents will catch it. If you only import a library (e.g., Python/Ruby gems installed via Homebrew), brewprune won't detect usage—be careful with `--medium` and `--risky` in this case.

**Q: How do I see what snapshots I have?**
A: Run `brewprune undo --list` to see all available snapshots with their IDs, creation times, and package counts.

**Q: Why is git/openssl/coreutils only "medium" tier even though never used?**

A: brewprune protects 47 foundational packages by capping their scores at 70 (medium tier max). These packages are critical infrastructure that many other packages depend on indirectly.

Protected packages include: openssl, ca-certificates, git, curl, wget, cmake, pkg-config, autoconf, automake, gcc, llvm, ncurses, readline, gettext, sqlite, zlib, and more.

See the full list in code: [internal/scanner/dependencies.go](https://github.com/blackwell-systems/brewprune/blob/main/internal/scanner/dependencies.go)

## License

MIT

## Contributing

Pull requests welcome. For major changes, open an issue first.

---

**Made with ☕️ by a developer with 237 Homebrew packages and 32GB of disk anxiety.**
