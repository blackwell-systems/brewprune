# Store Package

The `store` package provides SQLite database operations for brewprune, handling persistence of packages, dependencies, usage events, and snapshots.

## Features

- **Pure Go SQLite**: Uses `modernc.org/sqlite` (no CGO required)
- **In-memory support**: Use `:memory:` for testing
- **Full CRUD operations**: Complete package lifecycle management
- **Foreign key constraints**: Cascade deletes for referential integrity
- **WAL mode**: Write-Ahead Logging for better concurrency
- **JSON serialization**: Handles array fields (binary_paths) automatically

## Database Schema

### Tables

- **packages**: Homebrew packages with metadata
- **dependencies**: Package dependency relationships
- **usage_events**: Binary execution tracking
- **snapshots**: Point-in-time backups
- **snapshot_packages**: Packages in each snapshot

### Indexes

- `idx_usage_package`: Fast usage lookups by package
- `idx_usage_timestamp`: Time-based usage queries
- `idx_deps_package`: Forward dependency lookups
- `idx_deps_depends`: Reverse dependency lookups (dependents)

## Usage

### Creating a store

```go
// Production database
store, err := store.New("~/.brewprune/brewprune.db")
if err != nil {
    log.Fatal(err)
}
defer store.Close()

// Create schema
if err := store.CreateSchema(); err != nil {
    log.Fatal(err)
}
```

### Testing with in-memory database

```go
store, err := store.New(":memory:")
if err != nil {
    t.Fatal(err)
}
defer store.Close()

if err := store.CreateSchema(); err != nil {
    t.Fatal(err)
}
```

### Package operations

```go
// Insert or update package
pkg := &brew.Package{
    Name:        "node",
    Version:     "20.0.0",
    InstalledAt: time.Now(),
    InstallType: "explicit",
    Tap:         "homebrew/core",
    IsCask:      false,
    SizeBytes:   104857600,
    HasBinary:   true,
    BinaryPaths: []string{"/usr/local/bin/node", "/usr/local/bin/npm"},
}
err := store.InsertPackage(pkg)

// Get single package
pkg, err := store.GetPackage("node")

// List all packages
packages, err := store.ListPackages()

// Delete package (cascades to dependencies and usage events)
err := store.DeletePackage("node")
```

### Dependency operations

```go
// Record dependency: app depends on lib
err := store.InsertDependency("app", "lib")

// Get all dependencies of a package
deps, err := store.GetDependencies("app")

// Get all packages that depend on a library (reverse lookup)
dependents, err := store.GetDependents("lib")
```

### Usage event operations

```go
// Record usage
event := &scanner.UsageEvent{
    Package:    "git",
    EventType:  "exec",
    BinaryPath: "/usr/local/bin/git",
    Timestamp:  time.Now(),
}
err := store.InsertUsageEvent(event)

// Get usage events since date
since := time.Now().Add(-30 * 24 * time.Hour)
events, err := store.GetUsageEvents("git", since)

// Get most recent usage
lastUsage, err := store.GetLastUsage("git")
if lastUsage != nil {
    fmt.Printf("Last used: %v\n", *lastUsage)
}
```

### Snapshot operations

```go
// Create snapshot
snapshotID, err := store.InsertSnapshot("pre_removal", 5, "/path/to/snapshot.json")

// Get snapshot
snapshot, err := store.GetSnapshot(snapshotID)

// List all snapshots (newest first)
snapshots, err := store.ListSnapshots()

// Add packages to snapshot
pkg := &store.SnapshotPackage{
    SnapshotID:  snapshotID,
    PackageName: "node",
    Version:     "20.0.0",
    Tap:         "homebrew/core",
    WasExplicit: true,
}
err := store.InsertSnapshotPackage(snapshotID, pkg)

// Get all packages in snapshot
packages, err := store.GetSnapshotPackages(snapshotID)
```

## Timestamp Handling

All timestamps are stored as RFC3339 strings in SQLite for consistency:
- Insertions use `time.Time.Format(time.RFC3339)`
- Retrievals use `time.Parse(time.RFC3339, str)`
- UTC recommended for consistency

## JSON Serialization

The `binary_paths` field is stored as a JSON array in SQLite:
- `[]string{}` → `[]`
- `[]string{"/bin/foo"}` → `["/bin/foo"]`
- `nil` → `null`

Empty slices and nil are both handled correctly on retrieval.

## Cascade Deletes

Foreign key constraints automatically cascade deletes:
- Deleting a package removes its dependencies and usage events
- Deleting a snapshot removes its snapshot_packages

## Performance

- **Connection pooling**: Max 1 connection (SQLite single-writer)
- **WAL mode**: Better read/write concurrency
- **Indexes**: Optimized for common queries

## Error Handling

All operations return descriptive errors:
- Not found: `"package X not found"`
- Foreign key violations: Automatic via SQLite
- JSON errors: Wrapped with context

## Testing

Run tests with:
```bash
go test ./internal/store/
```

Run with coverage:
```bash
go test -cover ./internal/store/
```

All tests use in-memory databases for speed and isolation.
