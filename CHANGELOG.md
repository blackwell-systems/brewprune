# Changelog

All notable changes to brewprune will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.5] - 2026-02-27

### Added
- **PATH shim execution tracking** — `brewprune scan` now builds a Go interceptor binary (`~/.brewprune/bin/brewprune-shim`) and creates symlinks for every Homebrew command on your PATH. When you run `git`, `gh`, `jq`, etc., the shim logs the execution to `~/.brewprune/usage.log` and hands off to the real binary with zero perceptible overhead.
- **`cmd/brewprune-shim`** — standalone binary that uses `syscall.Exec` (no fork) and O_APPEND atomic log writes
- **`internal/shim` package** — `BuildShimBinary`, `GenerateShims` (LookPath collision-safe), `RemoveShims`, `IsShimSetup`
- **`internal/watcher/shim_processor.go`** — reads usage.log from a tracked byte offset, resolves basenames to packages, and batch-inserts events in a single SQLite transaction every 30 seconds
- **Crash-safe offset tracking** — offset file updated via temp-file rename after successful commit; events are never lost on crash
- **`brewprune doctor`** now checks shim binary exists and `~/.brewprune/bin` is correctly positioned in PATH

### Fixed
- **Usage tracking was completely broken** — the previous fsnotify watcher listened for `Write`/`Chmod` events on Homebrew bin directories, which never fire on binary execution (only on file modification). Zero events were ever captured despite the daemon running. PATH shims fix this permanently.

### Changed
- `brewprune scan` shows PATH setup instructions when shim directory is not yet in PATH
- `brewprune watch` description updated to reflect shim log processing
- README and docs updated to remove all FSEvents references

### Removed
- `github.com/fsnotify/fsnotify` dependency (no longer needed)
- `internal/watcher.BuildBinaryMap` and `MatchPathToPackage` (dead code, only served the broken fsnotify path)

### Technical
- `Watcher` struct simplified from 8 fields to 4 (store, stopCh, wg, batchTicker)
- All fsnotify imports, event handlers, and tests removed; −749 lines of dead code
- `go mod tidy` removed fsnotify from go.mod/go.sum

## [0.1.4] - 2026-02-27

### Added
- **`brewprune quickstart` command**: Interactive walkthrough for first-time users (runs scan + starts daemon + shows next steps)
- **`brewprune doctor` command**: Diagnostic tool that checks database, daemon status, and provides fix suggestions
- **Timeline reminder in scan output**: Shows "⚠️ NEXT STEP: Start usage tracking" after scan completes
- **Confidence summary in unused output**: Displays data quality (LOW/MEDIUM/HIGH) based on event count and tracking duration
- **Data quality indicator in status output**: Shows NOT READY/COLLECTING/GOOD/EXCELLENT based on tracking duration

### Changed
- **Help text improvements**: Added `--dry-run` workflow examples to `remove` and `unused` commands
- **Better onboarding**: First-time users now see clear next steps at every stage

### Technical
- Added `GetEventCount()` and `GetFirstEventTime()` helper methods to store package
- Timeline expectations now prominent in user-facing output
- Diagnostic checks for daemon health, database state, and usage data

## [0.1.3] - 2026-02-27

### Added
- Package size calculation - shows actual disk usage instead of "0 B"
- Functional `--sort size` - sorts by disk usage (largest first)
- Functional `--sort age` - sorts by installation date (oldest first)

### Fixed
- Size calculation now runs during scan and populates database
- Sorting flags now work correctly (previously ignored by render function)

### Changed
- Removed roadmap section from README (all core features complete)

### Technical
- Added `calculatePackageSize()` function using `du -sk`
- Added `SizeBytes` and `InstalledAt` fields to `ConfidenceScore`
- Fixed `RenderConfidenceTable()` to respect caller's sort order

## [0.1.2] - 2026-02-27

### Added
- **`brewprune status` command**: Check daemon status, database stats, event count, and tracking uptime at a glance
- **`brewprune explain <package>` command**: Deep-dive scoring analysis with detailed component breakdown and recommendations
- **`--verbose` flag for `unused`**: Show per-package scoring breakdown (Usage/Dependencies/Age/Type details)
- **Explainability scoring**: Added `ScoreExplanation` struct with detailed reasoning for each score component
- **Criticality penalty system**: Core dependencies (git, openssl, coreutils, etc.) capped at score 70 (medium tier max)
- **Expanded core dependencies**: Protection list increased from 15 to 47 packages including build tools, compilers, and essential libraries
- **Warning banner in `unused`**: Prominent alert when no usage data exists (LOW CONFIDENCE)
- **README improvements**: Added Privacy callout, Safety & Risks section, Timeline expectations, and Protected Packages FAQ

### Changed
- **Help text**: Root command now emphasizes daemon requirement with IMPORTANT notice and Quick Start steps
- **README structure**: Moved Quick Start to line 70 (immediately after Installation) with daemon setup impossible to miss
- **Scoring display**: Verbose mode shows component breakdown with points and detailed explanations

### Technical
- `ScoreExplanation` type with UsageDetail, DepsDetail, AgeDetail, TypeDetail fields
- `IsCritical` boolean flag on scores to identify foundational packages
- `RenderConfidenceTableVerbose()` for expanded output format
- launchd service configuration example for auto-start on login
- Updated terminology: "heuristic scoring" consistently throughout documentation

## [0.1.1] - 2026-02-26

### Fixed
- **Homebrew 5.x compatibility**: Updated from deprecated `brew list --json=v2` to `brew info --json=v2 --installed`
- **JSON parsing**: Fixed struct field types for `installed_on_request` (bool) and `installed_time` (int64)
- **Scan performance**: Optimized from per-package dependency calls to single `brew deps --installed --tree` (4.7s vs 3-6min for 166 packages)
- **Foreign key errors**: Skip dependencies that don't exist as installed packages

### Changed
- **Documentation terminology**: Changed "Confidence scoring" to "Heuristic scoring" for accuracy
- **README improvements**: Added Requirements, Limitations & Accuracy sections, clarified FSEvents monitoring scope

## [0.1.0] - 2026-02-26

### Added
- Real-time FSEvents monitoring for package usage tracking
- Confidence-based removal scoring algorithm (Usage 40pts + Dependencies 30pts + Age 20pts + Type 10pts)
- Three-tier safety classification (Safe 80-100, Medium 50-79, Risky 0-49)
- Automatic snapshot creation before package removal
- One-command rollback with exact version restoration
- Six CLI commands: scan, watch, unused, stats, remove, undo
- SQLite local storage (~/.brewprune/brewprune.db)
- Daemon mode for background usage monitoring
- Dependency-aware removal validation
- Progress indicators and formatted table output
- Support for both Homebrew formulae and casks
- Core dependency protection

### Technical
- Pure Go implementation with no CGO dependencies
- modernc.org/sqlite for cross-platform SQLite
- Cobra CLI framework
- GitHub Actions CI/CD with golangci-lint
- GoReleaser for multi-platform builds
- 12,676 lines of code (4,797 implementation + 7,879 tests)
- 83% test coverage across all packages

[Unreleased]: https://github.com/blackwell-systems/brewprune/compare/v0.1.4...HEAD
[0.1.4]: https://github.com/blackwell-systems/brewprune/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/blackwell-systems/brewprune/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/blackwell-systems/brewprune/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/blackwell-systems/brewprune/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/blackwell-systems/brewprune/releases/tag/v0.1.0
