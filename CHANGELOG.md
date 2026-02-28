# Changelog

All notable changes to brewprune will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- **`unused` table redesigned for actionable data** — replaced opaque Score and Reason columns with Size (disk usage), Uses (7d) (shim execution count), Depended On (reverse dependency count), and a colored tier tag. Risky and critical packages now show `✗ keep` instead of a tier name. Same layout applied to `remove` confirmation table.
- **Risky packages hidden by default** — 143+ transitive dependency packages no longer clutter the output. Use `--all` to show them. Explicit `--tier risky` still works.
- **Casks show `n/a` for usage columns** — GUI apps can't be tracked via shims; `n/a` replaces misleading `0`/`never`.
- **Zero deps show `—`** — em dash replaces noisy `0 packages` for cleaner display.
- **Tier summary header** — per-tier package counts and sizes shown before the table.
- **Reclaimable space footer** — per-tier disk space totals shown after the table.

### Added
- **`--all` flag for `unused`** — shows all tiers including risky (hidden by default)
- **`GetUsageEventCountSince` store query** — returns usage event count for a package within a time window (used for 7-day column)
- **`GetReverseDependencyCount` store query** — returns number of packages depending on a given package
- **`brewprune` added to core dependencies** — no longer recommends removing itself

## [0.2.1] - 2026-02-27

### Fixed
- **Scan destroyed all usage history** — `InsertPackage` used `INSERT OR REPLACE` which deletes the existing row before inserting, triggering `ON DELETE CASCADE` on `usage_events`. Every `brewprune scan` silently wiped all collected usage data. Switched to `INSERT ... ON CONFLICT(name) DO UPDATE SET` which updates in-place without cascade deletes.

## [0.2.0] - 2026-02-27

### Added
- **Brew-native shim infrastructure** — `shim.RefreshShims` performs incremental diff of desired vs current symlinks with LookPath collision-safety; `WriteShimVersion`/`ReadShimVersion` track shim binary version via crash-safe temp-file rename
- **Shim startup version check** — `brewprune-shim` compares its embedded version against `~/.brewprune/shim.version` on every invocation and warns (rate-limited to once/day) when stale, prompting `brewprune scan`
- **`brewprune scan --refresh-shims` flag** — fast path used by the Homebrew formula `post_install` hook after upgrades. Reads binary paths from the existing DB, diffs symlinks via `shim.RefreshShims`, skipping the full dep tree rebuild. Rebuilds the shim binary only when absent
- **`brewprune doctor` pipeline test** — Check 8 executes a real shimmed binary and polls `usage_events` for up to 35s to verify the full shim → daemon → DB pipeline end-to-end
- **`brewprune quickstart` blessed workflow** — interactive first-run walkthrough (scan → daemon → PATH config → next steps) with `internal/shell/config.go` for auto-appending PATH entries to `.zprofile`, `.bash_profile`, or `conf.d/brewprune.fish`
- **Stale detection** — `brew.CheckStaleness` compares `brew list --formula` against the DB and warns when new formulae are installed since last scan (shown in `unused` and `remove` output)
- **Shell completions** — zsh, bash, and fish completions generated via cobra, shipped in release tarballs via `.goreleaser.yml`, and installed by the Homebrew formula
- **"Used ≠ needed" disclaimer** in `unused` output — clarifies that "Safe" means low observed execution risk, not that the package is unnecessary
- **"Why it's safe" inline scores** in `remove` — per-package score/tier/reason displayed before the confirmation prompt when removing by name
- **Docker integration test container** — 12-step pipeline test (scan → shims → status → daemon → usage tracking → stats → unused → doctor → remove dry-run → refresh-shims) with mock brew prefix, exercising the full brewprune pipeline without Homebrew installed
- **Scout-and-wave prompt template** — canonical reusable prompt for parallel agent coordination (`docs/scout-and-wave-prompt.md`)

### Fixed
- **Watcher package matching** — use opt path (`/opt/homebrew/opt/<pkg>`) for package resolution from shim log entries, fall back to basename extraction
- **Docker test environment** — mock brew prefix on PATH for `exec.LookPath`, `/opt/homebrew` symlink for shim `findRealBinary`, correct PID file path (`watch.pid`), ANSI-stripped score extraction

### Changed
- **`brewprune status` rewritten** — brew-native aligned column format showing tracking state, 24h event count, shim count, last scan time, and data quality indicator
- Homebrew formula `post_install` now calls `brewprune scan --refresh-shims` and installs shell completions

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

[Unreleased]: https://github.com/blackwell-systems/brewprune/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/blackwell-systems/brewprune/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/blackwell-systems/brewprune/compare/v0.1.5...v0.2.0
[0.1.5]: https://github.com/blackwell-systems/brewprune/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/blackwell-systems/brewprune/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/blackwell-systems/brewprune/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/blackwell-systems/brewprune/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/blackwell-systems/brewprune/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/blackwell-systems/brewprune/releases/tag/v0.1.0
