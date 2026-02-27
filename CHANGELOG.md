# Changelog

All notable changes to brewprune will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.5] - 2026-02-27

### Fixed
- **Infinite exec loop in brewprune-shim** — v0.1.4 bundled `brewprune-shim` into the Homebrew formula, placing it at `/opt/homebrew/bin/brewprune-shim`. This introduced a loop: when the shim was invoked as `brewprune-shim`, `findRealBinary` resolved to `/opt/homebrew/bin/brewprune-shim` (itself), and `syscall.Exec` would re-execute it indefinitely — logging thousands of spurious events per second with no CPU overhead (same PID, no fork). Fixed by returning `""` early when `name == "brewprune-shim"`, producing a clean error exit instead.

## [0.1.4] - 2026-02-27

### Fixed
- **Shim binary not found when installed via Homebrew** — `brewprune-shim` is now bundled in GoReleaser release tarballs and installed alongside `brewprune`. Strategy 1 (co-location lookup) now works correctly for Homebrew installs; the `go install` fallback is no longer needed in production.

### Changed
- Homebrew formula now installs both `brewprune` and `brewprune-shim` binaries

## [0.1.3] - 2026-02-27

### Added
- **PATH shim execution tracking** — `brewprune scan` builds a Go interceptor binary (`~/.brewprune/bin/brewprune-shim`) and creates symlinks for every Homebrew command on your PATH. When you run `git`, `gh`, `jq`, etc., the shim logs the execution to `~/.brewprune/usage.log` and hands off to the real binary with zero perceptible overhead.
- **`brewprune quickstart` command** — interactive walkthrough for first-time users (runs scan + starts daemon + shows next steps)
- **`brewprune doctor` command** — diagnostic tool checking database, daemon, shim binary, and PATH setup; provides specific fix commands
- **Package size calculation** — shows actual disk usage per package instead of "0 B"
- **Functional `--sort size`** — sorts by disk usage (largest first)
- **Functional `--sort age`** — sorts by installation date (oldest first)
- **Confidence summary in `unused` output** — data quality indicator (NOT READY / COLLECTING / GOOD / EXCELLENT)
- **Timeline reminder in scan output** — shows next steps after scan completes

### Fixed
- **Usage tracking was completely broken** — the previous fsnotify watcher listened for `Write`/`Chmod` events on Homebrew bin directories, which never fire on binary execution. Zero events were ever captured despite the daemon running. PATH shims fix this permanently.
- Sorting flags now work correctly (previously ignored by the render function)
- Size calculation now runs during scan and populates the database

### Changed
- `brewprune scan` shows PATH setup instructions when shim directory is not yet in PATH
- Help text: added `--dry-run` workflow examples to `remove` and `unused` commands
- README and docs updated to accurately reflect shim-based architecture; removed all FSEvents references
- Cask limitation clearly documented: casks are scored on heuristics only, never show usage data

### Removed
- `github.com/fsnotify/fsnotify` dependency

### Technical
- `Watcher` struct simplified from 8 fields to 4; −749 lines of dead fsnotify code removed
- Crash-safe offset tracking: offset file updated via temp-file rename after successful DB commit
- `go mod tidy` removed fsnotify from go.mod/go.sum

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

[Unreleased]: https://github.com/blackwell-systems/brewprune/compare/v0.1.5...HEAD
[0.1.5]: https://github.com/blackwell-systems/brewprune/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/blackwell-systems/brewprune/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/blackwell-systems/brewprune/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/blackwell-systems/brewprune/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/blackwell-systems/brewprune/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/blackwell-systems/brewprune/releases/tag/v0.1.0
