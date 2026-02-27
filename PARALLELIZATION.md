# brewprune Parallel Development Strategy

Based on comprehensive analysis of PLAN.md, this document defines clear development seams for 3-4 agents to work simultaneously without conflicts.

## Dependency Graph

```
Foundation Layer (no dependencies):
├── store/ (database + schema)
└── brew/types.go (core types)

Data Layer (depends on Foundation):
├── brew/packages.go (depends on: brew/types.go)
├── brew/installer.go (depends on: brew/types.go)
└── scanner/types.go (depends on: brew/types.go, store/)

Feature Layer (depends on Data Layer):
├── scanner/inventory.go (depends on: brew/packages.go, store/, scanner/types.go)
├── scanner/dependencies.go (depends on: brew/packages.go, store/, scanner/types.go)
├── watcher/ (depends on: store/, scanner/types.go)
├── analyzer/ (depends on: store/, scanner/types.go)
└── snapshots/ (depends on: brew/types.go, store/)

Command Layer (depends on Feature Layer):
├── app/scan.go (depends on: scanner/, output/)
├── app/watch.go (depends on: watcher/, output/)
├── app/unused.go (depends on: analyzer/, output/)
├── app/remove.go (depends on: snapshots/, brew/installer.go, output/)
├── app/undo.go (depends on: snapshots/, brew/installer.go, output/)
└── app/stats.go (depends on: analyzer/, output/)

UI Layer (depends on Command Layer):
└── output/ (depends on: brew/types.go)

Entry Point (depends on Command Layer):
└── cmd/brewprune/main.go (depends on: app/)
```

**Critical Path:** Foundation → Data → Feature → Commands → Entry

## Parallel Work Streams

### Stream 1: Foundation & Data Store
**Files:**
- `internal/store/db.go`
- `internal/store/schema.go`
- `internal/store/queries.go`
- `internal/brew/types.go`
- `internal/scanner/types.go`

**External Dependencies:** None
**Can Start:** Immediately
**Blocks:** All other streams

### Stream 2: Brew Integration
**Files:**
- `internal/brew/packages.go`
- `internal/brew/installer.go`

**External Dependencies:** `brew/types.go`
**Can Start:** After Stream 1
**Blocks:** Streams 3, 4, 5, 6

### Stream 3: Scanner
**Files:**
- `internal/scanner/inventory.go`
- `internal/scanner/dependencies.go`

**External Dependencies:** `brew/packages.go`, `store/`, `scanner/types.go`
**Can Start:** After Streams 1 & 2
**Blocks:** Streams 4, 5

### Stream 4: FSEvents Watcher
**Files:**
- `internal/watcher/fsevents.go`
- `internal/watcher/matcher.go`
- `internal/watcher/daemon.go`

**External Dependencies:** `store/`, `scanner/types.go`, `scanner/inventory.go`
**Can Start:** After Streams 1 & 3
**Complexity:** Large

### Stream 5: Analyzer
**Files:**
- `internal/analyzer/confidence.go`
- `internal/analyzer/usage.go`
- `internal/analyzer/recommendations.go`

**External Dependencies:** `store/`, `scanner/types.go`, `scanner/dependencies.go`
**Can Start:** After Streams 1 & 3
**Complexity:** Medium

### Stream 6: Snapshots
**Files:**
- `internal/snapshots/create.go`
- `internal/snapshots/restore.go`
- `internal/snapshots/types.go`

**External Dependencies:** `brew/types.go`, `brew/installer.go`, `store/`
**Can Start:** After Streams 1 & 2
**Complexity:** Medium

### Stream 7: Output Utilities
**Files:**
- `internal/output/table.go`
- `internal/output/progress.go`

**External Dependencies:** `brew/types.go`
**Can Start:** After Stream 1
**Complexity:** Small

### Stream 8: CLI Commands - Scan & Watch
**Files:**
- `internal/app/root.go`
- `internal/app/scan.go`
- `internal/app/watch.go`

**External Dependencies:** `scanner/`, `watcher/`, `output/`, `store/`
**Can Start:** After Streams 3, 4, 7
**Complexity:** Small

### Stream 9: CLI Commands - Analysis
**Files:**
- `internal/app/unused.go`
- `internal/app/stats.go`

**External Dependencies:** `analyzer/`, `output/`, `store/`
**Can Start:** After Streams 5, 7
**Complexity:** Small

### Stream 10: CLI Commands - Remove & Undo
**Files:**
- `internal/app/remove.go`
- `internal/app/undo.go`

**External Dependencies:** `snapshots/`, `brew/installer.go`, `analyzer/`, `output/`
**Can Start:** After Streams 2, 5, 6, 7
**Complexity:** Small

### Stream 11: Entry Point & Infrastructure
**Files:**
- `cmd/brewprune/main.go`
- `Makefile`
- `.goreleaser.yml`
- `go.mod`
- `.github/workflows/`

**External Dependencies:** `app/`
**Can Start:** Immediately (scaffold), finalize after commands
**Complexity:** Small

## Recommended Execution Plan

### Phase 1: Foundation (4 hours, 1 agent)
**Agent A - Foundation:**
1. Initialize Go module, directory structure
2. Implement `store/` (db.go, schema.go, queries.go)
3. Implement `brew/types.go`
4. Implement `scanner/types.go`
5. Write unit tests for store

**Deliverable:** Database + core types ready

### Phase 2: Data Layer (4 hours, 2 agents)
**Agent A - Brew Integration (Stream 2):**
- `brew/packages.go` (parse brew CLI)
- `brew/installer.go` (install/uninstall)
- Unit tests with mock data

**Agent B - Scanner (Stream 3):**
- `scanner/inventory.go`
- `scanner/dependencies.go`
- Unit tests with in-memory DB

**Deliverable:** Can discover packages and build dependency graphs

### Phase 3: Feature Layer (6 hours, 4 agents)
**Agent A - Watcher (Stream 4):**
- `watcher/fsevents.go`
- `watcher/matcher.go`
- `watcher/daemon.go`

**Agent B - Analyzer (Stream 5):**
- `analyzer/confidence.go`
- `analyzer/usage.go`
- `analyzer/recommendations.go`

**Agent C - Snapshots (Stream 6):**
- `snapshots/types.go`
- `snapshots/create.go`
- `snapshots/restore.go`

**Agent D - Output (Stream 7):**
- `output/table.go`
- `output/progress.go`

**Deliverable:** All core features implemented

### Phase 4: Commands (4 hours, 3 agents)
**Agent A - Root + Scan & Watch (Stream 8):**
- `app/root.go`
- `app/scan.go`
- `app/watch.go`

**Agent B - Analysis Commands (Stream 9):**
- `app/unused.go`
- `app/stats.go`

**Agent C - Removal Commands (Stream 10):**
- `app/remove.go`
- `app/undo.go`

**Deliverable:** All CLI commands functional

### Phase 5: Integration & Polish (4 hours, 2 agents)
**Agent A - Entry Point (Stream 11):**
- `cmd/brewprune/main.go`
- `Makefile`, `.goreleaser.yml`
- CI/CD setup

**Agent B - Testing & Docs:**
- Integration tests
- README updates
- Error handling polish

**Deliverable:** Production-ready v0.1.0

## Conflict Zones (Avoid Parallel Writes)

**High Risk:**
- `go.mod` / `go.sum` - Agent A initializes with all deps upfront
- `internal/app/root.go` - Assigned to Stream 8 exclusively
- `README.md` - Assigned to Phase 5 Agent B

**Medium Risk:**
- `internal/store/queries.go` - Completed in Phase 1
- `brew/types.go` - Completed in Phase 1

**Safe for Parallel:**
- All other stream-specific files are isolated

## Interface Contracts

### Store Interface (Phase 1 delivers)
```go
type Store struct { db *sql.DB }

func New(dbPath string) (*Store, error)
func (s *Store) Close() error
func (s *Store) CreateSchema() error
func (s *Store) InsertPackage(pkg *Package) error
func (s *Store) GetPackage(name string) (*Package, error)
func (s *Store) ListPackages() ([]*Package, error)
func (s *Store) InsertDependency(pkg, dep string) error
func (s *Store) GetDependencies(pkg string) ([]string, error)
func (s *Store) InsertUsageEvent(event *UsageEvent) error
func (s *Store) GetUsageEvents(pkg string, since time.Time) ([]*UsageEvent, error)
func (s *Store) InsertSnapshot(snap *Snapshot) (int64, error)
func (s *Store) GetSnapshot(id int64) (*Snapshot, error)
func (s *Store) ListSnapshots() ([]*Snapshot, error)
```

### Brew Packages Interface (Phase 2 delivers)
```go
func ListInstalled() ([]*Package, error)
func GetPackageInfo(name string) (*Package, error)
func GetDependencyTree(pkg string) (map[string][]string, error)
func GetBrewPrefix() (string, error)
func PackageExists(name string) (bool, error)
```

### Brew Installer Interface (Phase 2 delivers)
```go
func Uninstall(pkgName string) error
func Install(pkgName, version string) error
func AddTap(tap string) error
func TapExists(tap string) (bool, error)
```

### Scanner Interface (Phase 2 delivers)
```go
type Scanner struct { store *store.Store }

func New(store *store.Store) *Scanner
func (s *Scanner) ScanPackages() error
func (s *Scanner) GetInventory() ([]*brew.Package, error)
func (s *Scanner) RefreshBinaryPaths() error
func (s *Scanner) BuildDependencyGraph() (map[string][]string, error)
func (s *Scanner) GetDependents(pkg string) ([]string, error)
func (s *Scanner) GetLeafPackages() ([]string, error)
func IsCoreDependency(pkg string) bool
```

### Watcher Interface (Phase 3 delivers)
```go
type Watcher struct { store *store.Store }

func New(store *store.Store) (*Watcher, error)
func (w *Watcher) Start() error
func (w *Watcher) Stop() error
func (w *Watcher) BuildBinaryMap() error
func (w *Watcher) MatchPathToPackage(path string) (string, bool)
func (w *Watcher) StartDaemon(pidFile, logFile string) error
func StopDaemon(pidFile string) error
func IsDaemonRunning(pidFile string) (bool, error)
```

### Analyzer Interface (Phase 3 delivers)
```go
type Analyzer struct { store *store.Store }

func New(store *store.Store) *Analyzer
func (a *Analyzer) ComputeScore(pkg string) (*ConfidenceScore, error)
func (a *Analyzer) GetPackagesByTier(tier string) ([]*ConfidenceScore, error)
func (a *Analyzer) GetUsageStats(pkg string) (*UsageStats, error)
func (a *Analyzer) GetUsageTrends(days int) (map[string]*UsageStats, error)
func (a *Analyzer) GetRecommendations() (*Recommendation, error)
func (a *Analyzer) ValidateRemoval(packages []string) ([]string, error)
```

### Snapshots Interface (Phase 3 delivers)
```go
type Manager struct { store *store.Store }

func New(store *store.Store, snapshotDir string) *Manager
func (m *Manager) CreateSnapshot(packages []string, reason string) (int64, error)
func (m *Manager) RestoreSnapshot(id int64) error
func (m *Manager) ListSnapshots() ([]*store.Snapshot, error)
func (m *Manager) CleanupOldSnapshots() error
```

### Output Interface (Phase 3 delivers)
```go
func RenderPackageTable(packages []*brew.Package) string
func RenderConfidenceTable(scores []*analyzer.ConfidenceScore) string
func RenderUsageTable(stats map[string]*analyzer.UsageStats) string
func RenderSnapshotTable(snapshots []*store.Snapshot) string

type ProgressBar struct{}
func NewProgress(total int, description string) *ProgressBar

type Spinner struct{}
func NewSpinner(message string) *Spinner
```

## Timeline Summary

**With 4 agents in parallel:**
- Phase 1: 4 hours (1 agent)
- Phase 2: 4 hours (2 agents, 8 agent-hours)
- Phase 3: 6 hours (4 agents, 24 agent-hours)
- Phase 4: 4 hours (3 agents, 12 agent-hours)
- Phase 5: 4 hours (2 agents, 8 agent-hours)

**Total calendar time:** ~22 hours (~3 work days)
**Total agent-hours:** ~56 hours

**Sequential time:** ~56 hours (~7 work days)
**Parallelization speedup:** 2.5x

## Validation Checklist

Before merging parallel streams:
- [ ] All tests pass independently
- [ ] No import cycles
- [ ] Interface contracts match
- [ ] No duplicate types
- [ ] Store queries don't conflict
- [ ] Cobra commands properly registered
- [ ] Dependencies reconciled in go.mod
- [ ] Test fixtures don't overlap
- [ ] No file overwrites

## Critical Foundation Files

These 5 files establish interfaces and enable parallel work:

1. `internal/store/db.go` - Core persistence, all streams depend on it
2. `internal/brew/types.go` - Central types used everywhere
3. `internal/store/schema.go` - Database schema defines data model
4. `internal/brew/packages.go` - Brew parsing, determines data accuracy
5. `internal/app/root.go` - Cobra integration point for all commands

Get these right in Phase 1-2, and parallel development in Phase 3-4 is smooth.
