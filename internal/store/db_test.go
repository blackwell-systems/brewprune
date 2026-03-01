package store

import (
	"errors"
	"testing"
	"time"

	"github.com/blackwell-systems/brewprune/internal/brew"
)

// TestListPackages_NoSchema_ReturnsErrNotInitialized verifies that calling
// ListPackages on a fresh DB (no CreateSchema) returns ErrNotInitialized.
func TestListPackages_NoSchema_ReturnsErrNotInitialized(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer s.Close()

	// Do NOT call CreateSchema — simulate uninitialized database.
	_, err = s.ListPackages()
	if err == nil {
		t.Fatal("ListPackages() should return an error on uninitialized DB")
	}
	t.Logf("raw error from ListPackages on uninitialized DB: %v", err)
	if !errors.Is(err, ErrNotInitialized) {
		t.Errorf("ListPackages() error = %v; want errors.Is(err, ErrNotInitialized) to be true", err)
	}
}

// TestGetPackage_NoSchema_ReturnsErrNotInitialized verifies that calling
// GetPackage on a fresh DB (no CreateSchema) returns ErrNotInitialized.
func TestGetPackage_NoSchema_ReturnsErrNotInitialized(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer s.Close()

	// Do NOT call CreateSchema — simulate uninitialized database.
	_, err = s.GetPackage("somepackage")
	if err == nil {
		t.Fatal("GetPackage() should return an error on uninitialized DB")
	}
	t.Logf("raw error from GetPackage on uninitialized DB: %v", err)
	if !errors.Is(err, ErrNotInitialized) {
		t.Errorf("GetPackage() error = %v; want errors.Is(err, ErrNotInitialized) to be true", err)
	}
}

// TestErrNotInitialized_ErrorMessage verifies that the ErrNotInitialized
// sentinel has a human-readable message that includes "brewprune scan".
func TestErrNotInitialized_ErrorMessage(t *testing.T) {
	msg := ErrNotInitialized.Error()
	if msg == "" {
		t.Error("ErrNotInitialized.Error() should not be empty")
	}
	if !containsString(msg, "brewprune scan") {
		t.Errorf("ErrNotInitialized message %q should contain 'brewprune scan'", msg)
	}
}

// containsString is a simple helper for substring checks without importing strings.
func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && stringContains(s, sub))
}

func stringContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Helper function to create an in-memory store for testing
func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}

	if err := store.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	return store
}

func TestNew(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer store.Close()

	if store.db == nil {
		t.Error("Store.db should not be nil")
	}
}

func TestCreateSchema(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Verify tables exist by querying sqlite_master
	tables := []string{"packages", "dependencies", "usage_events", "snapshots", "snapshot_packages"}
	for _, table := range tables {
		var name string
		err := store.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("Table %s not found: %v", table, err)
		}
	}

	// Verify indexes exist
	indexes := []string{"idx_usage_package", "idx_usage_timestamp", "idx_deps_package", "idx_deps_depends"}
	for _, index := range indexes {
		var name string
		err := store.db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name=?", index).Scan(&name)
		if err != nil {
			t.Errorf("Index %s not found: %v", index, err)
		}
	}
}

func TestInsertAndGetPackage(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	now := time.Now().UTC().Truncate(time.Second)
	pkg := &brew.Package{
		Name:        "node",
		Version:     "20.0.0",
		InstalledAt: now,
		InstallType: "explicit",
		Tap:         "homebrew/core",
		IsCask:      false,
		SizeBytes:   104857600, // 100 MB
		HasBinary:   true,
		BinaryPaths: []string{"/usr/local/bin/node", "/usr/local/bin/npm"},
	}

	// Insert package
	if err := store.InsertPackage(pkg); err != nil {
		t.Fatalf("InsertPackage() failed: %v", err)
	}

	// Get package
	retrieved, err := store.GetPackage("node")
	if err != nil {
		t.Fatalf("GetPackage() failed: %v", err)
	}

	// Verify fields
	if retrieved.Name != pkg.Name {
		t.Errorf("Name = %s, want %s", retrieved.Name, pkg.Name)
	}
	if retrieved.Version != pkg.Version {
		t.Errorf("Version = %s, want %s", retrieved.Version, pkg.Version)
	}
	if !retrieved.InstalledAt.Equal(pkg.InstalledAt) {
		t.Errorf("InstalledAt = %v, want %v", retrieved.InstalledAt, pkg.InstalledAt)
	}
	if retrieved.InstallType != pkg.InstallType {
		t.Errorf("InstallType = %s, want %s", retrieved.InstallType, pkg.InstallType)
	}
	if retrieved.Tap != pkg.Tap {
		t.Errorf("Tap = %s, want %s", retrieved.Tap, pkg.Tap)
	}
	if retrieved.IsCask != pkg.IsCask {
		t.Errorf("IsCask = %v, want %v", retrieved.IsCask, pkg.IsCask)
	}
	if retrieved.SizeBytes != pkg.SizeBytes {
		t.Errorf("SizeBytes = %d, want %d", retrieved.SizeBytes, pkg.SizeBytes)
	}
	if retrieved.HasBinary != pkg.HasBinary {
		t.Errorf("HasBinary = %v, want %v", retrieved.HasBinary, pkg.HasBinary)
	}
	if len(retrieved.BinaryPaths) != len(pkg.BinaryPaths) {
		t.Errorf("BinaryPaths length = %d, want %d", len(retrieved.BinaryPaths), len(pkg.BinaryPaths))
	}
	for i, path := range retrieved.BinaryPaths {
		if path != pkg.BinaryPaths[i] {
			t.Errorf("BinaryPaths[%d] = %s, want %s", i, path, pkg.BinaryPaths[i])
		}
	}
}

func TestInsertPackageReplace(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	now := time.Now().UTC().Truncate(time.Second)

	// Insert initial package
	pkg1 := &brew.Package{
		Name:        "python",
		Version:     "3.11.0",
		InstalledAt: now,
		InstallType: "explicit",
		Tap:         "homebrew/core",
		IsCask:      false,
		SizeBytes:   52428800, // 50 MB
		HasBinary:   true,
		BinaryPaths: []string{"/usr/local/bin/python3"},
	}

	if err := store.InsertPackage(pkg1); err != nil {
		t.Fatalf("InsertPackage() failed: %v", err)
	}

	// Update package with different version
	pkg2 := &brew.Package{
		Name:        "python",
		Version:     "3.12.0",
		InstalledAt: now.Add(24 * time.Hour),
		InstallType: "explicit",
		Tap:         "homebrew/core",
		IsCask:      false,
		SizeBytes:   62428800, // 60 MB
		HasBinary:   true,
		BinaryPaths: []string{"/usr/local/bin/python3", "/usr/local/bin/python3.12"},
	}

	if err := store.InsertPackage(pkg2); err != nil {
		t.Fatalf("InsertPackage() (update) failed: %v", err)
	}

	// Verify updated package
	retrieved, err := store.GetPackage("python")
	if err != nil {
		t.Fatalf("GetPackage() failed: %v", err)
	}

	if retrieved.Version != "3.12.0" {
		t.Errorf("Version = %s, want 3.12.0", retrieved.Version)
	}
	if retrieved.SizeBytes != 62428800 {
		t.Errorf("SizeBytes = %d, want 62428800", retrieved.SizeBytes)
	}
}

func TestGetPackageNotFound(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	_, err := store.GetPackage("nonexistent")
	if err == nil {
		t.Error("GetPackage() should return error for nonexistent package")
	}
}

func TestListPackages(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	now := time.Now().UTC().Truncate(time.Second)

	packages := []*brew.Package{
		{
			Name:        "git",
			Version:     "2.40.0",
			InstalledAt: now,
			InstallType: "explicit",
			Tap:         "homebrew/core",
			IsCask:      false,
			SizeBytes:   20971520, // 20 MB
			HasBinary:   true,
			BinaryPaths: []string{"/usr/local/bin/git"},
		},
		{
			Name:        "node",
			Version:     "20.0.0",
			InstalledAt: now,
			InstallType: "explicit",
			Tap:         "homebrew/core",
			IsCask:      false,
			SizeBytes:   104857600, // 100 MB
			HasBinary:   true,
			BinaryPaths: []string{"/usr/local/bin/node"},
		},
		{
			Name:        "postgresql",
			Version:     "15.2",
			InstalledAt: now,
			InstallType: "dependency",
			Tap:         "homebrew/core",
			IsCask:      false,
			SizeBytes:   41943040, // 40 MB
			HasBinary:   true,
			BinaryPaths: []string{"/usr/local/bin/postgres"},
		},
	}

	for _, pkg := range packages {
		if err := store.InsertPackage(pkg); err != nil {
			t.Fatalf("InsertPackage() failed for %s: %v", pkg.Name, err)
		}
	}

	// List all packages
	retrieved, err := store.ListPackages()
	if err != nil {
		t.Fatalf("ListPackages() failed: %v", err)
	}

	if len(retrieved) != len(packages) {
		t.Errorf("ListPackages() returned %d packages, want %d", len(retrieved), len(packages))
	}

	// Verify packages are sorted by name
	expectedOrder := []string{"git", "node", "postgresql"}
	for i, pkg := range retrieved {
		if pkg.Name != expectedOrder[i] {
			t.Errorf("Package[%d].Name = %s, want %s", i, pkg.Name, expectedOrder[i])
		}
	}
}

func TestDeletePackage(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	now := time.Now().UTC().Truncate(time.Second)
	pkg := &brew.Package{
		Name:        "htop",
		Version:     "3.2.2",
		InstalledAt: now,
		InstallType: "explicit",
		Tap:         "homebrew/core",
		IsCask:      false,
		SizeBytes:   1048576, // 1 MB
		HasBinary:   true,
		BinaryPaths: []string{"/usr/local/bin/htop"},
	}

	if err := store.InsertPackage(pkg); err != nil {
		t.Fatalf("InsertPackage() failed: %v", err)
	}

	// Delete package
	if err := store.DeletePackage("htop"); err != nil {
		t.Fatalf("DeletePackage() failed: %v", err)
	}

	// Verify deletion
	_, err := store.GetPackage("htop")
	if err == nil {
		t.Error("GetPackage() should return error after deletion")
	}
}

func TestDeletePackageNotFound(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	err := store.DeletePackage("nonexistent")
	if err == nil {
		t.Error("DeletePackage() should return error for nonexistent package")
	}
}

func TestInsertAndGetDependencies(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Insert test packages
	packages := []string{"app", "libA", "libB", "libC"}
	for _, name := range packages {
		pkg := &brew.Package{
			Name:        name,
			Version:     "1.0.0",
			InstalledAt: time.Now(),
			InstallType: "explicit",
			Tap:         "homebrew/core",
			IsCask:      false,
			SizeBytes:   1048576,
			HasBinary:   false,
			BinaryPaths: []string{},
		}
		if err := store.InsertPackage(pkg); err != nil {
			t.Fatalf("InsertPackage() failed for %s: %v", name, err)
		}
	}

	// Insert dependencies: app depends on libA, libB, libC
	deps := []string{"libA", "libB", "libC"}
	for _, dep := range deps {
		if err := store.InsertDependency("app", dep); err != nil {
			t.Fatalf("InsertDependency() failed: %v", err)
		}
	}

	// Get dependencies
	retrieved, err := store.GetDependencies("app")
	if err != nil {
		t.Fatalf("GetDependencies() failed: %v", err)
	}

	if len(retrieved) != len(deps) {
		t.Errorf("GetDependencies() returned %d dependencies, want %d", len(retrieved), len(deps))
	}

	// Verify dependencies are sorted
	for i, dep := range retrieved {
		if dep != deps[i] {
			t.Errorf("Dependency[%d] = %s, want %s", i, dep, deps[i])
		}
	}
}

func TestGetDependents(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Insert test packages
	packages := []string{"app1", "app2", "lib"}
	for _, name := range packages {
		pkg := &brew.Package{
			Name:        name,
			Version:     "1.0.0",
			InstalledAt: time.Now(),
			InstallType: "explicit",
			Tap:         "homebrew/core",
			IsCask:      false,
			SizeBytes:   1048576,
			HasBinary:   false,
			BinaryPaths: []string{},
		}
		if err := store.InsertPackage(pkg); err != nil {
			t.Fatalf("InsertPackage() failed for %s: %v", name, err)
		}
	}

	// Insert dependencies: app1 and app2 both depend on lib
	if err := store.InsertDependency("app1", "lib"); err != nil {
		t.Fatalf("InsertDependency() failed: %v", err)
	}
	if err := store.InsertDependency("app2", "lib"); err != nil {
		t.Fatalf("InsertDependency() failed: %v", err)
	}

	// Get dependents of lib
	dependents, err := store.GetDependents("lib")
	if err != nil {
		t.Fatalf("GetDependents() failed: %v", err)
	}

	expected := []string{"app1", "app2"}
	if len(dependents) != len(expected) {
		t.Errorf("GetDependents() returned %d dependents, want %d", len(dependents), len(expected))
	}

	for i, dependent := range dependents {
		if dependent != expected[i] {
			t.Errorf("Dependent[%d] = %s, want %s", i, dependent, expected[i])
		}
	}
}

func TestInsertDependencyIdempotent(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Insert test packages
	for _, name := range []string{"app", "lib"} {
		pkg := &brew.Package{
			Name:        name,
			Version:     "1.0.0",
			InstalledAt: time.Now(),
			InstallType: "explicit",
			Tap:         "homebrew/core",
			IsCask:      false,
			SizeBytes:   1048576,
			HasBinary:   false,
			BinaryPaths: []string{},
		}
		if err := store.InsertPackage(pkg); err != nil {
			t.Fatalf("InsertPackage() failed for %s: %v", name, err)
		}
	}

	// Insert same dependency twice
	if err := store.InsertDependency("app", "lib"); err != nil {
		t.Fatalf("InsertDependency() failed: %v", err)
	}
	if err := store.InsertDependency("app", "lib"); err != nil {
		t.Fatalf("InsertDependency() should be idempotent: %v", err)
	}

	// Verify only one dependency exists
	deps, err := store.GetDependencies("app")
	if err != nil {
		t.Fatalf("GetDependencies() failed: %v", err)
	}

	if len(deps) != 1 {
		t.Errorf("GetDependencies() returned %d dependencies, want 1", len(deps))
	}
}

func TestInsertAndGetUsageEvents(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Insert test package
	pkg := &brew.Package{
		Name:        "git",
		Version:     "2.40.0",
		InstalledAt: time.Now(),
		InstallType: "explicit",
		Tap:         "homebrew/core",
		IsCask:      false,
		SizeBytes:   20971520,
		HasBinary:   true,
		BinaryPaths: []string{"/usr/local/bin/git"},
	}
	if err := store.InsertPackage(pkg); err != nil {
		t.Fatalf("InsertPackage() failed: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	events := []*UsageEvent{
		{
			Package:    "git",
			EventType:  "exec",
			BinaryPath: "/usr/local/bin/git",
			Timestamp:  now.Add(-2 * time.Hour),
		},
		{
			Package:    "git",
			EventType:  "exec",
			BinaryPath: "/usr/local/bin/git",
			Timestamp:  now.Add(-1 * time.Hour),
		},
		{
			Package:    "git",
			EventType:  "exec",
			BinaryPath: "/usr/local/bin/git",
			Timestamp:  now,
		},
	}

	// Insert events
	for _, event := range events {
		if err := store.InsertUsageEvent(event); err != nil {
			t.Fatalf("InsertUsageEvent() failed: %v", err)
		}
	}

	// Get events since 90 minutes ago (should return 2 events)
	since := now.Add(-90 * time.Minute)
	retrieved, err := store.GetUsageEvents("git", since)
	if err != nil {
		t.Fatalf("GetUsageEvents() failed: %v", err)
	}

	if len(retrieved) != 2 {
		t.Errorf("GetUsageEvents() returned %d events, want 2", len(retrieved))
	}

	// Verify events are ordered by timestamp descending
	if !retrieved[0].Timestamp.After(retrieved[1].Timestamp) {
		t.Error("Events should be ordered by timestamp descending")
	}
}

func TestGetLastUsage(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Insert test package
	pkg := &brew.Package{
		Name:        "node",
		Version:     "20.0.0",
		InstalledAt: time.Now(),
		InstallType: "explicit",
		Tap:         "homebrew/core",
		IsCask:      false,
		SizeBytes:   104857600,
		HasBinary:   true,
		BinaryPaths: []string{"/usr/local/bin/node"},
	}
	if err := store.InsertPackage(pkg); err != nil {
		t.Fatalf("InsertPackage() failed: %v", err)
	}

	// Test with no usage events
	lastUsage, err := store.GetLastUsage("node")
	if err != nil {
		t.Fatalf("GetLastUsage() failed: %v", err)
	}
	if lastUsage != nil {
		t.Error("GetLastUsage() should return nil for package with no usage")
	}

	// Insert usage events
	now := time.Now().UTC().Truncate(time.Second)
	events := []*UsageEvent{
		{
			Package:    "node",
			EventType:  "exec",
			BinaryPath: "/usr/local/bin/node",
			Timestamp:  now.Add(-2 * time.Hour),
		},
		{
			Package:    "node",
			EventType:  "exec",
			BinaryPath: "/usr/local/bin/node",
			Timestamp:  now, // Most recent
		},
		{
			Package:    "node",
			EventType:  "exec",
			BinaryPath: "/usr/local/bin/node",
			Timestamp:  now.Add(-1 * time.Hour),
		},
	}

	for _, event := range events {
		if err := store.InsertUsageEvent(event); err != nil {
			t.Fatalf("InsertUsageEvent() failed: %v", err)
		}
	}

	// Get last usage
	lastUsage, err = store.GetLastUsage("node")
	if err != nil {
		t.Fatalf("GetLastUsage() failed: %v", err)
	}
	if lastUsage == nil {
		t.Fatal("GetLastUsage() should return timestamp")
	}

	// Verify it's the most recent timestamp
	if !lastUsage.Equal(now) {
		t.Errorf("GetLastUsage() = %v, want %v", lastUsage, now)
	}
}

func TestInsertAndGetSnapshot(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Insert snapshot
	id, err := store.InsertSnapshot("pre_removal", 5, "/path/to/snapshot.json")
	if err != nil {
		t.Fatalf("InsertSnapshot() failed: %v", err)
	}

	if id == 0 {
		t.Error("InsertSnapshot() should return non-zero ID")
	}

	// Get snapshot
	snapshot, err := store.GetSnapshot(id)
	if err != nil {
		t.Fatalf("GetSnapshot() failed: %v", err)
	}

	if snapshot.ID != id {
		t.Errorf("Snapshot.ID = %d, want %d", snapshot.ID, id)
	}
	if snapshot.Reason != "pre_removal" {
		t.Errorf("Snapshot.Reason = %s, want pre_removal", snapshot.Reason)
	}
	if snapshot.PackageCount != 5 {
		t.Errorf("Snapshot.PackageCount = %d, want 5", snapshot.PackageCount)
	}
	if snapshot.SnapshotPath != "/path/to/snapshot.json" {
		t.Errorf("Snapshot.SnapshotPath = %s, want /path/to/snapshot.json", snapshot.SnapshotPath)
	}
	if snapshot.CreatedAt.IsZero() {
		t.Error("Snapshot.CreatedAt should not be zero")
	}
}

func TestGetSnapshotNotFound(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	_, err := store.GetSnapshot(999)
	if err == nil {
		t.Error("GetSnapshot() should return error for nonexistent snapshot")
	}
}

func TestListSnapshots(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Insert multiple snapshots with different timestamps
	snapshots := []struct {
		reason string
		count  int
		path   string
	}{
		{"manual", 10, "/path/to/snapshot1.json"},
		{"pre_removal", 8, "/path/to/snapshot2.json"},
		{"manual", 12, "/path/to/snapshot3.json"},
	}

	var ids []int64
	for i, s := range snapshots {
		// Add small delay to ensure different timestamps
		if i > 0 {
			time.Sleep(10 * time.Millisecond)
		}
		id, err := store.InsertSnapshot(s.reason, s.count, s.path)
		if err != nil {
			t.Fatalf("InsertSnapshot() failed: %v", err)
		}
		ids = append(ids, id) //nolint:staticcheck
	}

	// List snapshots
	retrieved, err := store.ListSnapshots()
	if err != nil {
		t.Fatalf("ListSnapshots() failed: %v", err)
	}

	if len(retrieved) != len(snapshots) {
		t.Errorf("ListSnapshots() returned %d snapshots, want %d", len(retrieved), len(snapshots))
	}

	// Verify snapshots are ordered by creation time descending (newest first)
	for i := 0; i < len(retrieved)-1; i++ {
		if retrieved[i].CreatedAt.Before(retrieved[i+1].CreatedAt) {
			t.Error("Snapshots should be ordered by CreatedAt descending")
		}
	}
}

func TestInsertAndGetSnapshotPackages(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Insert snapshot
	snapshotID, err := store.InsertSnapshot("manual", 3, "/path/to/snapshot.json")
	if err != nil {
		t.Fatalf("InsertSnapshot() failed: %v", err)
	}

	// Insert snapshot packages
	packages := []*SnapshotPackage{
		{
			SnapshotID:  snapshotID,
			PackageName: "git",
			Version:     "2.40.0",
			Tap:         "homebrew/core",
			WasExplicit: true,
		},
		{
			SnapshotID:  snapshotID,
			PackageName: "node",
			Version:     "20.0.0",
			Tap:         "homebrew/core",
			WasExplicit: true,
		},
		{
			SnapshotID:  snapshotID,
			PackageName: "openssl",
			Version:     "3.0.0",
			Tap:         "homebrew/core",
			WasExplicit: false,
		},
	}

	for _, pkg := range packages {
		if err := store.InsertSnapshotPackage(snapshotID, pkg); err != nil {
			t.Fatalf("InsertSnapshotPackage() failed for %s: %v", pkg.PackageName, err)
		}
	}

	// Get snapshot packages
	retrieved, err := store.GetSnapshotPackages(snapshotID)
	if err != nil {
		t.Fatalf("GetSnapshotPackages() failed: %v", err)
	}

	if len(retrieved) != len(packages) {
		t.Errorf("GetSnapshotPackages() returned %d packages, want %d", len(retrieved), len(packages))
	}

	// Verify packages are sorted by name
	expectedOrder := []string{"git", "node", "openssl"}
	for i, pkg := range retrieved {
		if pkg.PackageName != expectedOrder[i] {
			t.Errorf("Package[%d].Name = %s, want %s", i, pkg.PackageName, expectedOrder[i])
		}
		if pkg.SnapshotID != snapshotID {
			t.Errorf("Package[%d].SnapshotID = %d, want %d", i, pkg.SnapshotID, snapshotID)
		}
	}
}

func TestCascadeDelete(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Insert package with dependencies and usage events
	pkg := &brew.Package{
		Name:        "app",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		InstallType: "explicit",
		Tap:         "homebrew/core",
		IsCask:      false,
		SizeBytes:   1048576,
		HasBinary:   true,
		BinaryPaths: []string{"/usr/local/bin/app"},
	}
	if err := store.InsertPackage(pkg); err != nil {
		t.Fatalf("InsertPackage() failed: %v", err)
	}

	dep := &brew.Package{
		Name:        "lib",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		InstallType: "dependency",
		Tap:         "homebrew/core",
		IsCask:      false,
		SizeBytes:   524288,
		HasBinary:   false,
		BinaryPaths: []string{},
	}
	if err := store.InsertPackage(dep); err != nil {
		t.Fatalf("InsertPackage() failed: %v", err)
	}

	// Add dependency
	if err := store.InsertDependency("app", "lib"); err != nil {
		t.Fatalf("InsertDependency() failed: %v", err)
	}

	// Add usage event
	event := &UsageEvent{
		Package:    "app",
		EventType:  "exec",
		BinaryPath: "/usr/local/bin/app",
		Timestamp:  time.Now(),
	}
	if err := store.InsertUsageEvent(event); err != nil {
		t.Fatalf("InsertUsageEvent() failed: %v", err)
	}

	// Delete package
	if err := store.DeletePackage("app"); err != nil {
		t.Fatalf("DeletePackage() failed: %v", err)
	}

	// Verify dependencies are deleted
	deps, err := store.GetDependencies("app")
	if err != nil {
		t.Fatalf("GetDependencies() failed: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("Dependencies should be deleted with package, got %d", len(deps))
	}

	// Verify usage events are deleted
	events, err := store.GetUsageEvents("app", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("GetUsageEvents() failed: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("Usage events should be deleted with package, got %d", len(events))
	}
}

func TestSnapshotCascadeDelete(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Insert snapshot
	snapshotID, err := store.InsertSnapshot("manual", 1, "/path/to/snapshot.json")
	if err != nil {
		t.Fatalf("InsertSnapshot() failed: %v", err)
	}

	// Insert snapshot package
	pkg := &SnapshotPackage{
		SnapshotID:  snapshotID,
		PackageName: "test",
		Version:     "1.0.0",
		Tap:         "homebrew/core",
		WasExplicit: true,
	}
	if err := store.InsertSnapshotPackage(snapshotID, pkg); err != nil {
		t.Fatalf("InsertSnapshotPackage() failed: %v", err)
	}

	// Delete snapshot
	_, err = store.db.Exec("DELETE FROM snapshots WHERE id = ?", snapshotID)
	if err != nil {
		t.Fatalf("Failed to delete snapshot: %v", err)
	}

	// Verify snapshot packages are deleted
	packages, err := store.GetSnapshotPackages(snapshotID)
	if err != nil {
		t.Fatalf("GetSnapshotPackages() failed: %v", err)
	}
	if len(packages) != 0 {
		t.Errorf("Snapshot packages should be deleted with snapshot, got %d", len(packages))
	}
}

func TestEmptyBinaryPaths(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Insert package with empty binary paths
	pkg := &brew.Package{
		Name:        "lib",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		InstallType: "dependency",
		Tap:         "homebrew/core",
		IsCask:      false,
		SizeBytes:   524288,
		HasBinary:   false,
		BinaryPaths: []string{},
	}

	if err := store.InsertPackage(pkg); err != nil {
		t.Fatalf("InsertPackage() failed: %v", err)
	}

	// Get package
	retrieved, err := store.GetPackage("lib")
	if err != nil {
		t.Fatalf("GetPackage() failed: %v", err)
	}

	if retrieved.BinaryPaths == nil {
		t.Error("BinaryPaths should not be nil")
	}
	if len(retrieved.BinaryPaths) != 0 {
		t.Errorf("BinaryPaths length = %d, want 0", len(retrieved.BinaryPaths))
	}
}

func TestGetUsageEventCountSince_ExcludesProbeEvents(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	pkg := &brew.Package{
		Name:        "imagemagick",
		Version:     "7.0.0",
		InstalledAt: time.Now(),
		InstallType: "dependency",
		Tap:         "homebrew/core",
		IsCask:      false,
		SizeBytes:   1048576,
		HasBinary:   true,
		BinaryPaths: []string{"/opt/homebrew/bin/Magick++-config"},
	}
	if err := store.InsertPackage(pkg); err != nil {
		t.Fatalf("InsertPackage() failed: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)

	probeEvent := &UsageEvent{
		Package:    "imagemagick",
		EventType:  "probe",
		BinaryPath: "/opt/homebrew/bin/Magick++-config",
		Timestamp:  now,
	}
	if err := store.InsertUsageEvent(probeEvent); err != nil {
		t.Fatalf("InsertUsageEvent() failed: %v", err)
	}

	count, err := store.GetUsageEventCountSince("imagemagick", now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("GetUsageEventCountSince() failed: %v", err)
	}
	if count != 0 {
		t.Errorf("GetUsageEventCountSince() = %d, want 0 (probe events must be excluded)", count)
	}

	execEvent := &UsageEvent{
		Package:    "imagemagick",
		EventType:  "exec",
		BinaryPath: "/opt/homebrew/bin/convert",
		Timestamp:  now,
	}
	if err := store.InsertUsageEvent(execEvent); err != nil {
		t.Fatalf("InsertUsageEvent() failed: %v", err)
	}

	count, err = store.GetUsageEventCountSince("imagemagick", now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("GetUsageEventCountSince() failed: %v", err)
	}
	if count != 1 {
		t.Errorf("GetUsageEventCountSince() = %d, want 1 (exec events must be counted)", count)
	}
}

func TestGetLastUsage_ExcludesProbeEvents(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	pkg := &brew.Package{
		Name:        "pkg-config",
		Version:     "0.29.2",
		InstalledAt: time.Now(),
		InstallType: "dependency",
		Tap:         "homebrew/core",
		IsCask:      false,
		SizeBytes:   524288,
		HasBinary:   true,
		BinaryPaths: []string{"/opt/homebrew/bin/pkg-config"},
	}
	if err := store.InsertPackage(pkg); err != nil {
		t.Fatalf("InsertPackage() failed: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)

	probeEvent := &UsageEvent{
		Package:    "pkg-config",
		EventType:  "probe",
		BinaryPath: "/opt/homebrew/bin/pkg-config",
		Timestamp:  now,
	}
	if err := store.InsertUsageEvent(probeEvent); err != nil {
		t.Fatalf("InsertUsageEvent() failed: %v", err)
	}

	last, err := store.GetLastUsage("pkg-config")
	if err != nil {
		t.Fatalf("GetLastUsage() failed: %v", err)
	}
	if last != nil {
		t.Errorf("GetLastUsage() = %v, want nil (probe events must be excluded)", last)
	}
}

func TestNilBinaryPaths(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Insert package with nil binary paths
	pkg := &brew.Package{
		Name:        "lib",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		InstallType: "dependency",
		Tap:         "homebrew/core",
		IsCask:      false,
		SizeBytes:   524288,
		HasBinary:   false,
		BinaryPaths: nil,
	}

	if err := store.InsertPackage(pkg); err != nil {
		t.Fatalf("InsertPackage() failed: %v", err)
	}

	// Get package
	retrieved, err := store.GetPackage("lib")
	if err != nil {
		t.Fatalf("GetPackage() failed: %v", err)
	}

	// JSON unmarshals null as nil, not empty slice
	// This is standard Go JSON behavior
	if len(retrieved.BinaryPaths) != 0 {
		t.Errorf("BinaryPaths length = %d, want 0", len(retrieved.BinaryPaths))
	}
}
