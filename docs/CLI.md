# brewprune CLI Reference

Complete reference for all brewprune commands, flags, and usage patterns.

## Table of Contents

- [Commands](#commands)
  - [brewprune scan](#brewprune-scan)
  - [brewprune watch](#brewprune-watch)
  - [brewprune status](#brewprune-status)
  - [brewprune unused](#brewprune-unused)
  - [brewprune stats](#brewprune-stats)
  - [brewprune explain](#brewprune-explain)
  - [brewprune remove](#brewprune-remove)
  - [brewprune undo](#brewprune-undo)
- [Global Flags](#global-flags)
- [Exit Codes](#exit-codes)
- [Output Formats](#output-formats)
- [Environment Variables](#environment-variables)
- [Examples by Use Case](#examples-by-use-case)

## Commands

### brewprune scan

Scans and indexes installed Homebrew packages.

**Description:**

Scans all installed Homebrew packages and stores them in the brewprune database. This command discovers all installed packages via brew, builds the dependency graph, and optionally refreshes binary path mappings. The package inventory is cached in the database for fast access by other commands.

The scan command should be run:
- After installing brewprune for the first time
- After installing or removing packages manually with brew
- Periodically to keep the database in sync with brew

**Usage:**
```bash
brewprune scan [flags]
```

**Flags:**
- `--refresh-binaries` - Refresh binary path mappings (default: true)
- `--quiet` - Suppress output

**Exit Codes:**
- 0: Success
- 1: Error (database error, brew command failure)

**Examples:**
```bash
# Scan all packages
brewprune scan

# Scan without refreshing binary paths
brewprune scan --refresh-binaries=false

# Scan quietly (suppress output)
brewprune scan --quiet
```

**Output:**
Displays a progress spinner for each scan phase, followed by a summary table showing discovered packages and total size.

---

### brewprune watch

Monitors package usage via filesystem events.

**Description:**

Starts monitoring filesystem events to track package usage in real-time. The watch command monitors binary executions in Homebrew directories and tracks when packages are used. This data is used to build confidence scores for removal recommendations.

The watcher tracks:
- Binary executions in brew bin/sbin directories
- Application launches from /Applications
- Frequency and recency of usage

Usage data is written to the database periodically (every 30 seconds) to minimize I/O overhead.

**Watch modes:**
- **Foreground (default):** Run in current terminal with Ctrl+C to stop
- **Daemon:** Run as background process with automatic restart on reboot
- **Stop:** Stop a running daemon

**Usage:**
```bash
brewprune watch [flags]
```

**Flags:**
- `--daemon` - Run as background daemon
- `--stop` - Stop running daemon
- `--pid-file PATH` - Custom PID file location (default: `~/.brewprune/watch.pid`)
- `--log-file PATH` - Custom log file location (default: `~/.brewprune/watch.log`)

**Exit Codes:**
- 0: Success
- 1: Error (daemon already running, failed to start/stop)

**Examples:**
```bash
# Run in foreground (Ctrl+C to stop)
brewprune watch

# Run as background daemon
brewprune watch --daemon

# Stop running daemon
brewprune watch --stop

# Use custom PID and log files
brewprune watch --daemon --pid-file /tmp/watch.pid --log-file /tmp/watch.log
```

**Daemon Management:**
```bash
# Start daemon
brewprune watch --daemon

# Check if daemon is running
brewprune status

# Stop daemon
brewprune watch --stop

# View daemon logs
tail -f ~/.brewprune/watch.log
```

**Output:**
- Foreground mode: Shows startup messages and tracks events with periodic status updates
- Daemon mode: Outputs PID file and log file locations, then detaches to background

---

### brewprune status

Checks daemon status and tracking statistics.

**Description:**

Displays the current status of the brewprune daemon and tracking statistics. This command helps verify that usage tracking is working correctly.

**Shows:**
- Daemon running status and PID
- Database location and validity
- Number of packages being tracked
- Total usage events logged
- Time since tracking started
- Most recent package activity

**Usage:**
```bash
brewprune status
```

**Flags:**
None

**Exit Codes:**
- 0: Success
- 1: Error (failed to read PID file or database)

**Examples:**
```bash
# Check status
brewprune status
```

**Sample Output:**
```
Daemon Status:        ✓ Running (PID 12345)
Database:             ✓ Found (/Users/user/.brewprune/brewprune.db)
Packages Tracked:     127
Events Logged:        3,456

Last Event: 2 minutes ago (git)
```

If daemon is not running, shows a warning with instructions to start it.

---

### brewprune unused

Lists unused packages with confidence scores.

**Description:**

Analyzes installed packages and displays confidence scores for removal. The confidence score (0-100) is computed from:
- **Usage patterns (40 points):** Recent activity indicates active use
- **Dependencies (30 points):** Fewer dependents = safer to remove
- **Age (20 points):** Older installations may be stale
- **Type (10 points):** Leaf packages are safer than core dependencies

**Packages are classified into tiers:**
- **safe (80-100):** High confidence for removal
- **medium (50-79):** Review before removal
- **risky (0-49):** Keep unless certain

Core dependencies (git, openssl, etc.) are capped at 70 to prevent accidental removal.

**Usage:**
```bash
brewprune unused [flags]
```

**Flags:**
- `--tier TIER` - Filter by tier: safe, medium, risky
- `--min-score N` - Minimum confidence score (0-100)
- `--sort ORDER` - Sort by: score (highest first), size (largest first), age (oldest first) (default: score)
- `-v, --verbose` - Show detailed explanation for each package

**Exit Codes:**
- 0: Success
- 1: Error (invalid flag value, database error)

**Examples:**
```bash
# Show all unused packages
brewprune unused

# Show only safe-to-remove packages
brewprune unused --tier safe

# Show packages with confidence >= 70
brewprune unused --min-score 70

# Sort by size instead of score
brewprune unused --sort size

# Sort by age (oldest first)
brewprune unused --sort age

# Show verbose output with detailed scoring breakdown
brewprune unused --tier safe --verbose
```

**Output:**

Standard mode displays a table with:
- Package name
- Confidence score (0-100)
- Tier (safe/medium/risky)
- Last used timestamp
- Reason for score

Verbose mode (`-v`) includes detailed breakdown of:
- Usage score (0-40 points)
- Dependencies score (0-30 points)
- Age score (0-20 points)
- Type score (0-10 points)
- Explanations for each component

**Warning:**
If no usage events are found (daemon not running), displays a warning banner explaining that recommendations are based on heuristics only without actual usage tracking.

---

### brewprune stats

Shows usage statistics for packages.

**Description:**

Displays usage statistics and trends for installed packages. Without flags, shows usage trends for all packages in the last 30 days. Use `--package` to view detailed statistics for a specific package. Use `--days` to adjust the time window for analysis.

**Usage frequency is classified as:**
- **daily:** Used in last 7 days with high frequency
- **weekly:** Used in last 30 days
- **monthly:** Used in last 90 days
- **never:** No recorded usage

**Usage:**
```bash
brewprune stats [flags]
```

**Flags:**
- `--days N` - Time window in days (default: 30)
- `--package NAME` - Show stats for specific package

**Exit Codes:**
- 0: Success
- 1: Error (invalid days value, database error)

**Examples:**
```bash
# Show usage trends for all packages (last 30 days)
brewprune stats

# Show usage trends for last 90 days
brewprune stats --days 90

# Show detailed stats for a specific package
brewprune stats --package git

# Show recent activity (last 7 days)
brewprune stats --days 7
```

**Output:**

For all packages: Displays a table with package names, total runs, last used timestamp, and frequency classification.

For specific package (`--package`): Shows detailed statistics:
- Total uses
- Last used timestamp
- Days since last use
- First seen timestamp
- Frequency classification

---

### brewprune explain

Shows detailed scoring explanation for a package.

**Description:**

Displays detailed breakdown of removal confidence score for a specific package. Shows component scores, reasoning, and recommendations for the package.

**Usage:**
```bash
brewprune explain [package]
```

**Arguments:**
- `package` - Name of package to explain (required)

**Flags:**
None

**Exit Codes:**
- 0: Success
- 1: Error (package not found, database error)

**Examples:**
```bash
# Explain score for git package
brewprune explain git

# Explain score for node
brewprune explain node
```

**Output:**

Displays a detailed breakdown table showing:
- Overall score and tier (with color coding)
- Installation date
- Component scores breakdown:
  - Usage (0-40 points) with explanation
  - Dependencies (0-30 points) with explanation
  - Age (0-20 points) with explanation
  - Type (0-10 points) with explanation
- Criticality penalty if applicable (-30 points for core dependencies)
- Total score and tier
- Why this tier was assigned
- Recommendation (safe to remove / review before removing / do not remove)
- Protected status if core dependency

---

### brewprune remove

Removes unused Homebrew packages.

**Description:**

Removes unused Homebrew packages based on confidence tiers or explicit list. If no packages are specified, removes packages based on tier flags. If packages are specified, validates and removes those specific packages.

**Safety features:**
- Validates removal candidates before proceeding
- Warns about dependent packages
- Creates automatic snapshot (unless `--no-snapshot`)
- Requires confirmation for risky operations

**Usage:**
```bash
brewprune remove [flags]
brewprune remove [packages...]
```

**Tier Flags (when no packages specified):**
- `--safe` - Remove only safe-tier packages (80-100 score)
- `--medium` - Remove safe and medium-tier packages (50-100 score)
- `--risky` - Remove all unused packages (0-100 score, requires confirmation)

**Safety Flags:**
- `--dry-run` - Show what would be removed without removing
- `--yes` - Skip confirmation prompts
- `--no-snapshot` - Skip automatic snapshot creation (dangerous!)

**Exit Codes:**
- 0: Success
- 1: Error (invalid flags, package not found, removal failed)

**Examples:**
```bash
# Remove safe packages
brewprune remove --safe

# Preview medium-tier removal (dry-run)
brewprune remove --medium --dry-run

# Actually remove medium-tier packages
brewprune remove --medium

# Remove specific packages
brewprune remove wget curl

# Remove all unused packages without confirmation (dangerous!)
brewprune remove --risky --yes

# Remove without creating snapshot (not recommended!)
brewprune remove --safe --no-snapshot
```

**Output:**

Displays:
1. Table of packages to be removed with scores
2. Summary (package count, disk space to free, snapshot status)
3. Confirmation prompt (unless `--yes`)
4. Progress bar during removal
5. Results with success count and freed space
6. Snapshot ID for rollback
7. Any failures or warnings

**Important Notes:**
- Core dependencies are protected and will be skipped even if explicitly specified
- Snapshots are created automatically before removal (unless `--no-snapshot`)
- Use `--dry-run` first to preview changes
- Use `brewprune undo` to rollback if needed

---

### brewprune undo

Restores packages from a snapshot.

**Description:**

Restores previously removed packages from a snapshot. Snapshots are automatically created before package removal operations and can be used to rollback changes.

**Usage:**
```bash
brewprune undo [snapshot-id | latest] [flags]
```

**Arguments:**
- `snapshot-id` - The numeric ID of the snapshot to restore
- `latest` - Restore the most recent snapshot

**Flags:**
- `--list` - List all available snapshots
- `--yes` - Skip confirmation prompt

**Exit Codes:**
- 0: Success (or partial success with warnings)
- 1: Error (invalid snapshot ID, snapshot not found)

**Examples:**
```bash
# List all snapshots
brewprune undo --list

# Restore latest snapshot
brewprune undo latest

# Restore specific snapshot by ID
brewprune undo 42

# Restore without confirmation
brewprune undo latest --yes
```

**Output:**

When listing (`--list`):
- Table of all snapshots with ID, creation time, reason, and package count
- Instructions to restore

When restoring:
- Snapshot details (ID, creation time, reason, package count)
- List of packages to restore
- Confirmation prompt (unless `--yes`)
- Progress indicator during restoration
- Success message with package count
- Reminder to run `brewprune scan` to update database

**Important Notes:**
- Exact version restoration depends on Homebrew bottle/formula availability
- If a specific version is not available, Homebrew will install the latest version
- After restoration, run `brewprune scan` to update the package database
- Partial restoration may occur if some packages fail to install

---

## Global Flags

These flags work with all commands:

- `--db PATH` - Custom database path (default: `~/.brewprune/brewprune.db`)

**Example:**
```bash
# Use custom database location
brewprune scan --db /tmp/brewprune.db
brewprune unused --db /tmp/brewprune.db
```

---

## Exit Codes

All commands use consistent exit codes:

- **0:** Success
- **1:** Error (general error, invalid flags, command failure)

Specific error scenarios return exit code 1 with descriptive error messages on stderr.

---

## Output Formats

### Tables

brewprune uses formatted ASCII tables with:
- Header row with column names
- Separator line
- Data rows with aligned columns
- Color coding (green for safe, yellow for medium, red for risky)

### Progress Indicators

Two types of progress indicators:

**Spinner:** Used for indeterminate operations
```
⠋ Loading packages...
```

**Progress Bar:** Used for operations with known total
```
Removing packages [████████████████████████████] 42/42
```

### Color Coding

- **Green:** Safe tier, success messages
- **Yellow:** Medium tier, warnings
- **Red:** Risky tier, errors
- **Bold:** Headers, important information

### Size Formatting

Disk sizes are displayed in human-readable format:
- Bytes (B) for < 1 KB
- Kilobytes (KB) for < 1 MB
- Megabytes (MB) for < 1 GB
- Gigabytes (GB) for >= 1 GB

### Time Formatting

Timestamps are displayed as:
- "N seconds ago" for < 1 minute
- "N minutes ago" for < 1 hour
- "N hours ago" for < 1 day
- "N days ago" for < 1 month
- "N months ago" for < 1 year
- "N years ago" for >= 1 year
- "never" for zero time

---

## Environment Variables

brewprune respects standard environment variables:

- **HOME:** User home directory (used for default paths)

### Default Paths

All paths relative to `~/.brewprune/`:

- **Database:** `~/.brewprune/brewprune.db`
- **PID File:** `~/.brewprune/watch.pid`
- **Log File:** `~/.brewprune/watch.log`
- **Snapshots:** `~/.brewprune/snapshots/`

Override with flags:
```bash
# Custom database
brewprune scan --db /custom/path/db.sqlite

# Custom PID and log files
brewprune watch --daemon --pid-file /tmp/watch.pid --log-file /tmp/watch.log
```

---

## Examples by Use Case

### First-Time Setup

Complete setup workflow:

```bash
# 1. Scan installed packages
brewprune scan

# 2. Start daemon to track usage (CRITICAL!)
brewprune watch --daemon

# 3. Verify daemon is running
brewprune status

# 4. (Optional) Auto-start on login
# Add to ~/.zshrc or create launchd service (see README)
```

### Regular Cleanup

After 1-2 weeks of tracking:

```bash
# 1. Check daemon status
brewprune status

# 2. View unused packages
brewprune unused --tier safe

# 3. Preview removal (dry-run)
brewprune remove --safe --dry-run

# 4. Remove safe packages
brewprune remove --safe

# 5. Verify results
brewprune scan
```

### Reviewing Specific Packages

Investigate individual packages:

```bash
# View detailed score breakdown
brewprune explain node

# Check usage statistics
brewprune stats --package node

# View recent usage (last 7 days)
brewprune stats --days 7 --package node
```

### Emergency Rollback

If something breaks after removal:

```bash
# Restore latest snapshot immediately
brewprune undo latest

# Or restore specific snapshot
brewprune undo --list
brewprune undo 42

# Update database after restoration
brewprune scan
```

### Advanced Usage

```bash
# Find largest unused packages
brewprune unused --sort size --tier safe

# Find oldest unused packages
brewprune unused --sort age --tier medium

# Remove specific packages
brewprune remove wget curl nodejs

# Skip confirmation (for automation)
brewprune remove --safe --yes

# Stop and restart daemon
brewprune watch --stop
brewprune watch --daemon

# View daemon logs
tail -f ~/.brewprune/watch.log
```

### Troubleshooting

```bash
# Check if daemon is running
brewprune status

# Restart daemon
brewprune watch --stop
brewprune watch --daemon

# Rescan if database is out of sync
brewprune scan

# View all snapshots
brewprune undo --list

# Remove packages without snapshot (not recommended)
brewprune remove wget --no-snapshot
```

---

## Additional Resources

- **Main Documentation:** [README.md](../README.md)
- **Source Code:** [github.com/blackwell-systems/brewprune](https://github.com/blackwell-systems/brewprune)
- **Issues & Support:** [GitHub Issues](https://github.com/blackwell-systems/brewprune/issues)

---

**Note:** This CLI reference is current as of the latest release. For the most up-to-date information, run `brewprune [command] --help`.
