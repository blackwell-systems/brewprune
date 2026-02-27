# brewprune Implementation Plan

## 1. Project Overview

brewprune is a Homebrew package usage tracker and cleanup tool that solves the "100 packages installed, 20 actually used" problem through evidence-based removal with zero-fear rollback.

**The core problem:** Developers accumulate packages over time and never remove them because:
1. No way to know what's actually used vs installed-once-and-forgotten
2. Fear of breaking things by removing the wrong package
3. `brew autoremove` only tracks dependency chains, not actual usage

**The solution:** Track real usage via FSEvents, score packages by removal confidence (safe/medium/risky), and create automatic snapshots before removal so any mistake is one command away from being undone.

**Target users:**
- Developers with 50+ Homebrew packages who've lost track
- Anyone who's run out of disk space and doesn't know what's safe to remove
- Teams managing standardized dev environments

**Killer feature:** Confidence-scored removal with one-command rollback. `brewprune remove --safe` creates a snapshot, removes packages, and if anything breaks, `brewprune undo` restores everything instantly.

---

## 2. Architecture

### Package Structure

```
brewprune/
├── cmd/
│   └── brewprune/
│       └── main.go
├── internal/
│   ├── app/
│   │   ├── root.go
│   │   ├── scan.go
│   │   ├── watch.go
│   │   ├── unused.go
│   │   ├── remove.go
│   │   ├── undo.go
│   │   └── stats.go
│   ├── brew/
│   │   ├── packages.go      # Parse brew list/info/deps
│   │   ├── installer.go     # Reinstall packages
│   │   └── types.go
│   ├── watcher/
│   │   ├── fsevents.go      # FSEvents monitoring
│   │   ├── matcher.go       # Match binaries to packages
│   │   └── daemon.go
│   ├── scanner/
│   │   ├── inventory.go     # Discover installed packages
│   │   ├── dependencies.go  # Build dependency graph
│   │   └── types.go
│   ├── analyzer/
│   │   ├── confidence.go    # Scoring algorithm
│   │   ├── usage.go         # Usage statistics
│   │   └── recommendations.go
│   ├── snapshots/
│   │   ├── create.go        # Snapshot before removal
│   │   ├── restore.go       # Rollback from snapshot
│   │   └── types.go
│   ├── store/
│   │   ├── db.go
│   │   ├── schema.go
│   │   └── queries.go
│   └── output/
│       ├── table.go
│       └── progress.go
├── .goreleaser.yml
├── Makefile
├── go.mod
├── PLAN.md
└── README.md
```

### Key Design Decisions

- **Pure Go, no CGO** - Use modernc.org/sqlite for portability
- **Cobra CLI** - Consistent with other brew tools
- **lipgloss** - Styled terminal output
- **Optional TUI** - bubbletea for interactive mode (v2)
- **Local-only** - No network calls, everything on disk

---

## 3. Data Model

### SQLite Schema

Located at `~/.brewprune/brewprune.db`.

```sql
CREATE TABLE packages (
    name TEXT PRIMARY KEY,
    installed_at TIMESTAMP,
    install_type TEXT,     -- 'explicit' or 'dependency'
    version TEXT,
    tap TEXT,              -- 'homebrew/core', 'user/tap', etc.
    is_cask BOOLEAN,
    size_bytes INTEGER,
    has_binary BOOLEAN,    -- Does this package install executables?
    binary_paths TEXT      -- JSON array of bin paths
);

CREATE TABLE dependencies (
    package TEXT NOT NULL,
    depends_on TEXT NOT NULL,
    PRIMARY KEY (package, depends_on),
    FOREIGN KEY (package) REFERENCES packages(name),
    FOREIGN KEY (depends_on) REFERENCES packages(name)
);

CREATE TABLE usage_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    package TEXT NOT NULL,
    event_type TEXT NOT NULL,  -- 'exec', 'app_launch'
    binary_path TEXT,
    timestamp TIMESTAMP NOT NULL,
    FOREIGN KEY (package) REFERENCES packages(name)
);

CREATE TABLE snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP NOT NULL,
    reason TEXT,                -- 'manual', 'pre_removal'
    package_count INTEGER,
    snapshot_path TEXT NOT NULL -- Path to JSON file
);

CREATE TABLE snapshot_packages (
    snapshot_id INTEGER NOT NULL,
    package_name TEXT NOT NULL,
    version TEXT NOT NULL,
    tap TEXT,
    was_explicit BOOLEAN,
    FOREIGN KEY (snapshot_id) REFERENCES snapshots(id)
);

CREATE INDEX idx_usage_package ON usage_events(package);
CREATE INDEX idx_usage_timestamp ON usage_events(timestamp);
CREATE INDEX idx_deps_package ON dependencies(package);
CREATE INDEX idx_deps_depends ON dependencies(depends_on);
```

### Snapshot JSON Format

Stored in `~/.brewprune/snapshots/YYYY-MM-DD-HHMMSS.json`:

```json
{
  "created_at": "2026-02-26T17:30:00Z",
  "reason": "pre_removal",
  "packages": [
    {
      "name": "node@16",
      "version": "16.20.2",
      "tap": "homebrew/core",
      "was_explicit": true,
      "dependencies": ["icu4c", "libnghttp2"]
    },
    {
      "name": "postgresql@14",
      "version": "14.10",
      "tap": "homebrew/core",
      "was_explicit": true,
      "dependencies": ["icu4c", "openssl@3", "readline"]
    }
  ],
  "brew_version": "4.2.5"
}
```

---

## 4. Command Specifications

### 4.1 `brewprune scan`

Inventory all installed packages and show usage summary.

**Output:**
```
📦 100 packages installed (14.2 GB)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

By usage:
  Daily           12 packages    2.1 GB
  Weekly           8 packages    1.4 GB
  Monthly          5 packages    890 MB
  Never used      42 packages    8.2 GB
  Indeterminate   33 packages    1.6 GB  (libraries)

Safe to remove: 42 packages, 8.2 GB
Run: brewprune unused --safe
```

**Flags:**
- `--json` - JSON output
- `--verbose` - Show per-package breakdown

**Implementation:**
1. Call `brew list --json=v2` to get all packages
2. Parse install dates, versions, sizes
3. Query usage_events for each package
4. Compute usage frequency buckets
5. Render summary table

### 4.2 `brewprune watch`

Start FSEvents daemon to monitor package usage.

**Behavior:**
- Monitors `$(brew --prefix)/bin` for binary executions
- Monitors `/Applications` for app launches (casks only)
- Writes usage events to SQLite
- Runs as background daemon with PID file

**Flags:**
- `--daemon` - Run in background
- `--stop` - Stop running daemon
- `--interval 30s` - Event batch flush interval

**Implementation:**
1. Use `fsnotify` or raw FSEvents API
2. Watch directories for Open/Execute events
3. Match event path to package via binary_paths lookup
4. Batch writes to SQLite every N seconds
5. PID file at `~/.brewprune/daemon.pid`

### 4.3 `brewprune unused`

List never-used packages with confidence scores.

**Output:**
```
Safe to remove (high confidence):
  node@16        2.1 GB    installed 142 days ago, 0 uses, no dependents
  postgresql@14  890 MB    installed 89 days ago, 0 uses, no dependents
  htop             2 MB    installed 200 days ago, 0 uses, no dependents

Medium confidence (leaf dependencies, unused):
  libpng          45 MB    dep of ffmpeg (used 3x last month)
  jq               8 MB    installed 150 days ago, 0 uses, no dependents

Risky (dependencies of active packages):
  openssl@3         -      keep, dep of 12 active packages

Total reclaimable: 3.1 GB (safe), 53 MB (medium)

Remove safely: brewprune remove --safe
```

**Flags:**
- `--safe` - Show only safe packages
- `--medium` - Include medium confidence
- `--risky` - Show all including risky
- `--days N` - Unused for at least N days (default: 30)
- `--json`

**Implementation:**
1. Query all packages
2. Join with usage_events (last 30/60/90 days)
3. Build dependency graph
4. Compute confidence score per package
5. Sort by size descending
6. Render table with confidence badges

### 4.4 `brewprune remove`

Remove packages with automatic snapshot.

**Behavior:**
1. Validate package list
2. Check dependencies (warn if removing something depended on)
3. Create snapshot JSON
4. Record snapshot in database
5. Call `brew uninstall <pkg>` for each
6. Report success + snapshot ID

**Flags:**
- `--safe` - Remove all safe packages
- `--medium` - Remove safe + medium
- `--risky` - Remove all (dangerous!)
- `--dry-run` - Show what would be removed
- `--force` - Skip confirmation prompts

**Example:**
```bash
brewprune remove --safe

Creating snapshot... ✓ (snapshot-20260226-173045)
Removing 3 packages:
  node@16 (2.1 GB)
  postgresql@14 (890 MB)
  htop (2 MB)

Removed 3 packages, reclaimed 3.0 GB

If anything breaks: brewprune undo
```

### 4.5 `brewprune undo`

Rollback most recent removal.

**Behavior:**
1. Load most recent snapshot from DB
2. Read snapshot JSON
3. Reinstall each package at recorded version via `brew install <pkg>@<version>`
4. Handle tap sources (add tap if needed)
5. Report success

**Flags:**
- `--snapshot ID` - Restore specific snapshot
- `--list` - List available snapshots

**Example:**
```bash
brewprune undo

Restoring snapshot from 5 minutes ago...
Reinstalling 3 packages:
  node@16@16.20.2
  postgresql@14@14.10
  htop@3.2.2

Restored 3 packages (3.0 GB)
```

### 4.6 `brewprune stats`

Show usage trends and statistics.

**Output:**
```
Usage trends (last 90 days)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Most used:
  python@3.12    238 runs    daily
  git            156 runs    daily
  node           89 runs     weekly

Never used (installed > 30 days):
  42 packages, 8.2 GB

Storage by category:
  Active          6.0 GB  (58%)
  Unused          8.2 GB  (32%)
  Indeterminate   1.0 GB  (10%)
```

**Flags:**
- `--days N` - Time window (default: 90)
- `--json`

---

## 5. Confidence Scoring Algorithm

### Score Components

Each package gets a score from 0-100 based on four factors:

**1. Usage (40 points)**
- Last 7 days: 40 points
- Last 30 days: 30 points
- Last 90 days: 20 points
- Last year: 10 points
- Never: 0 points

**2. Dependencies (30 points)**
- No dependents: 30 points
- 1-3 dependents (unused): 20 points
- 1-3 dependents (used): 10 points
- 4+ dependents: 0 points

**3. Age (20 points)**
- Installed > 180 days: 20 points
- Installed > 90 days: 15 points
- Installed > 30 days: 10 points
- Installed < 30 days: 0 points

**4. Type (10 points)**
- Leaf package with binaries: 10 points
- Library with no binaries: 5 points
- Core dependency (openssl, icu4c): 0 points

### Confidence Tiers

- **Safe (80-100)**: No usage, no active dependents, installed > 90 days, has binaries
- **Medium (50-79)**: Low usage or leaf libraries
- **Risky (0-49)**: Recently used, has dependents, or core library

### Library Inference

For packages with no binaries (libpng, openssl):
- If any dependent was used in last 30 days → mark library as "indirectly used"
- If all dependents are unused → library is unused
- If no dependents → leaf library (medium confidence)

---

## 6. Snapshot and Rollback Strategy

### Snapshot Creation

**When:**
- Before every `brewprune remove` (automatic)
- On demand via `brewprune snapshot create`

**What's captured:**
- Package name, exact version, tap source
- Whether it was explicitly installed or a dependency
- List of its dependencies (for recreation)
- Brew version (compatibility check)

**Storage:**
- JSON file: `~/.brewprune/snapshots/<timestamp>.json`
- Database record: snapshots + snapshot_packages tables
- Auto-cleanup: Delete snapshots > 90 days old

### Rollback Process

1. **Load snapshot JSON**
2. **For each package:**
   - Add tap if not present: `brew tap <tap>`
   - Install specific version: `brew install <pkg>@<version>`
   - If version unavailable, install latest and warn
3. **Verify installation**
4. **Report success/failures**

### Edge Cases

**Version no longer available:**
- Homebrew only keeps recent versions
- Fallback: install latest version, warn user
- Future enhancement: cache formulae locally

**Tap removed:**
- If tap no longer exists, warn and skip package
- User must manually find alternative

**Dependency conflicts:**
- Brew handles this natively
- If conflict, brew will error and we report it

---

## 7. FSEvents Watcher Implementation

### Monitoring Strategy

**What to watch:**
- `$(brew --prefix)/bin` - All brew-installed binaries
- `$(brew --prefix)/sbin` - System binaries
- `/Applications` - GUI apps from casks (filter by brew ownership)

**How:**
- Use `fsnotify` Go library (cross-platform)
- Register watches on directories
- Filter events: Open, Execute, Stat
- Match event path to package

### Matching Binaries to Packages

**Approach 1: Lookup table (fast)**
- At scan time, build map: `binary_path → package_name`
- Store in `packages.binary_paths` as JSON array
- FSEvent → lookup → package

**Approach 2: Reverse query (slow but accurate)**
- FSEvent on `/usr/local/bin/node`
- Run `brew which node` → get package name
- Insert usage event

Use Approach 1 for performance. Rebuild lookup table on `brewprune scan`.

### Daemon Management

**Start:**
```bash
brewprune watch --daemon
```

**PID file:** `~/.brewprune/daemon.pid`

**Stop:**
```bash
brewprune watch --stop
# OR
kill $(cat ~/.brewprune/daemon.pid)
```

**Logging:** Write to `~/.brewprune/watcher.log`

### Permissions

FSEvents requires no special permissions on macOS. On Linux (inotify), same. App launch tracking from /Applications requires reading app bundle Info.plist files (public).

---

## 8. Testing Strategy

### Unit Tests

Per-package tests with mocked data:

- `brew/packages_test.go` - Parse brew JSON output
- `analyzer/confidence_test.go` - Scoring algorithm
- `snapshots/create_test.go` - Snapshot generation
- `snapshots/restore_test.go` - Rollback logic
- `store/db_test.go` - SQLite queries (in-memory DB)

### Integration Tests

With actual brew commands (require brew installed):

- `scanner/inventory_test.go` - Discover installed packages
- `brew/installer_test.go` - Install/uninstall packages
- End-to-end: scan → remove → undo cycle

Mark integration tests with build tags:
```go
//go:build integration
```

Run via: `go test -tags=integration ./...`

### Test Fixtures

- Mock brew JSON outputs in `testdata/`
- SQLite in-memory databases for store tests
- Sample snapshot JSONs

---

## 9. Implementation Phases

### Phase 1: Foundation (Day 1)

1. Go module init, Cobra CLI scaffold
2. SQLite schema creation
3. Brew package parser (`brew list --json`, `brew info`, `brew deps`)
4. Basic inventory scanner
5. Store package + dependency data

**Deliverable:** `brewprune scan` works, shows installed packages

### Phase 2: Usage Tracking (Day 1-2)

6. FSEvents watcher implementation
7. Binary-to-package matching
8. Usage event recording
9. Daemon mode (background process)
10. PID file management

**Deliverable:** `brewprune watch` monitors usage

### Phase 3: Analysis (Day 2)

11. Confidence scoring algorithm
12. Usage statistics computation
13. Dependency graph analysis
14. Library inference logic

**Deliverable:** `brewprune unused` shows scored packages

### Phase 4: Removal & Snapshots (Day 2-3)

15. Snapshot creation (JSON + DB)
16. Package removal via brew uninstall
17. Snapshot rollback (reinstall packages)
18. Snapshot management (list, cleanup)

**Deliverable:** `brewprune remove --safe`, `brewprune undo` work end-to-end

### Phase 5: Polish (Day 3)

19. Styled output with lipgloss
20. Progress bars for operations
21. Confirmation prompts
22. Error handling improvements
23. README + documentation
24. CI setup (GitHub Actions)

**Deliverable:** Production-ready v0.1.0

### Phase 6: Advanced Features (Future)

- TUI mode with bubbletea
- `brewprune diff` - Compare snapshots
- `brewprune doctor` - Health check (stale deps, outdated packages)
- `brewprune optimize` - Suggest package replacements (python@3.9 → python@3.12)
- Homebrew tap for distribution

---

## 10. Known Limitations

**1. Library packages**
- No direct usage tracking for libraries (libpng, openssl)
- Must infer from dependents
- Some false positives/negatives inevitable

**2. Snapshot version availability**
- Homebrew doesn't guarantee old versions remain available
- Rollback may install latest version instead of exact version
- Mitigation: warn user, consider local formula caching

**3. FSEvents overhead**
- Watching large directories has some CPU cost
- Batch event processing mitigates this
- User can run daemon only when needed

**4. Cask tracking limitations**
- App launches harder to track than binary executions
- Must rely on FSEvents on app bundle
- May miss background services, menu bar apps

**5. Brew internals dependency**
- Relies on `brew` CLI and JSON output format
- If Homebrew changes output format, parser breaks
- Mitigation: version check, graceful degradation

**6. Multi-user systems**
- Usage tracking per-user, packages system-wide
- Package may be "unused" by you but used by another user
- Mitigation: warn if system-wide brew installation detected

**7. Version pinning**
- Homebrew discourages version pinning
- Rollback may not restore exact environment
- Mitigation: snapshot includes dependencies for manual recovery

---

## 11. Critical Implementation Details

### Brew Commands

**List packages:**
```bash
brew list --json=v2
```

**Package info:**
```bash
brew info --json=v2 node@16
```

**Dependency tree:**
```bash
brew deps --tree node@16
```

**Uninstall:**
```bash
brew uninstall node@16
```

**Install specific version:**
```bash
brew install node@16
```

**Check if package exists:**
```bash
brew search --formula node@16
```

### Directory Locations

- Brew prefix: `brew --prefix` (usually `/usr/local` or `/opt/homebrew`)
- Binaries: `$(brew --prefix)/bin`
- Casks: `/Applications` + `$(brew --prefix)/Caskroom`
- Config: `~/.brewprune/`
- Database: `~/.brewprune/brewprune.db`
- Snapshots: `~/.brewprune/snapshots/`
- PID file: `~/.brewprune/daemon.pid`
- Logs: `~/.brewprune/watcher.log`

### Performance Targets

- `brewprune scan`: < 2s for 100 packages
- `brewprune unused`: < 1s query time
- `brewprune remove --safe`: < 10s for 10 packages
- `brewprune undo`: < 30s for 10 packages
- FSEvents watcher: < 5% CPU idle, < 50MB RAM

---

## 12. Distribution

**Initial release:**
- `go install github.com/blackwell-systems/brewprune/cmd/brewprune@latest`

**Future:**
- Homebrew tap: `brew tap blackwell-systems/tap`
- `brew install brewprune`

**goreleaser config:**
- Build for darwin (amd64, arm64)
- Linux support (optional, works but brew less common)
- Distribute via GitHub Releases

---

## 13. References

- [claudewatch PLAN.md](https://github.com/blackwell-systems/claudewatch/blob/main/PLAN.md) - Architecture template
- [Homebrew JSON API](https://docs.brew.sh/Manpage#list-options-installed_formulae) - Data source
- [fsnotify](https://github.com/fsnotify/fsnotify) - FSEvents library
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [bubbletea](https://github.com/charmbracelet/bubbletea) - TUI framework (future)
