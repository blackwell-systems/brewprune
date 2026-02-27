# Changelog

All notable changes to brewprune will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/blackwell-systems/brewprune/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/blackwell-systems/brewprune/releases/tag/v0.1.0
