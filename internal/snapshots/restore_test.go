package snapshots

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blackwell-systems/brewprune/internal/brew"
	"github.com/blackwell-systems/brewprune/internal/store"
)

func TestLoadSnapshotFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create test snapshot data
	snapshotData := &SnapshotData{
		CreatedAt: time.Now(),
		Reason:    "test",
		Packages: []*PackageSnapshot{
			{
				Name:         "node",
				Version:      "20.0.0",
				Tap:          "homebrew/core",
				WasExplicit:  true,
				Dependencies: []string{"icu4c"},
			},
		},
		BrewVersion: "4.0.0",
	}

	// Write snapshot file
	snapshotPath := filepath.Join(tempDir, "test.json")
	jsonData, err := json.Marshal(snapshotData)
	if err != nil {
		t.Fatalf("Failed to marshal snapshot data: %v", err)
	}

	if err := os.WriteFile(snapshotPath, jsonData, 0644); err != nil {
		t.Fatalf("Failed to write snapshot file: %v", err)
	}

	// Test loading
	loaded, err := loadSnapshotFile(snapshotPath)
	if err != nil {
		t.Fatalf("Failed to load snapshot file: %v", err)
	}

	if loaded.Reason != "test" {
		t.Errorf("Expected reason 'test', got '%s'", loaded.Reason)
	}

	if len(loaded.Packages) != 1 {
		t.Errorf("Expected 1 package, got %d", len(loaded.Packages))
	}

	if loaded.Packages[0].Name != "node" {
		t.Errorf("Expected package name 'node', got '%s'", loaded.Packages[0].Name)
	}

	if loaded.BrewVersion != "4.0.0" {
		t.Errorf("Expected brew version '4.0.0', got '%s'", loaded.BrewVersion)
	}
}

func TestLoadSnapshotFileNotFound(t *testing.T) {
	_, err := loadSnapshotFile("/nonexistent/path.json")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

func TestLoadSnapshotFileInvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	invalidPath := filepath.Join(tempDir, "invalid.json")

	if err := os.WriteFile(invalidPath, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("Failed to write invalid JSON file: %v", err)
	}

	_, err := loadSnapshotFile(invalidPath)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestRestoreSnapshot(t *testing.T) {
	// Note: This test does NOT actually call brew commands
	// It tests the logic around snapshot restoration

	tempDir := t.TempDir()
	snapshotDir := filepath.Join(tempDir, "snapshots")

	// Create in-memory database
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer db.Close()

	if err := db.CreateSchema(); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Create snapshot directory
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		t.Fatalf("Failed to create snapshot directory: %v", err)
	}

	// Create test snapshot data
	snapshotData := &SnapshotData{
		CreatedAt: time.Now(),
		Reason:    "test_restore",
		Packages: []*PackageSnapshot{
			{
				Name:         "testpkg",
				Version:      "1.0.0",
				Tap:          "homebrew/core",
				WasExplicit:  true,
				Dependencies: []string{},
			},
		},
		BrewVersion: "4.0.0",
	}

	// Write snapshot file
	snapshotPath := filepath.Join(snapshotDir, "test.json")
	jsonData, err := json.Marshal(snapshotData)
	if err != nil {
		t.Fatalf("Failed to marshal snapshot data: %v", err)
	}

	if err := os.WriteFile(snapshotPath, jsonData, 0644); err != nil {
		t.Fatalf("Failed to write snapshot file: %v", err)
	}

	// Insert snapshot into database
	snapshotID, err := db.InsertSnapshot("test_restore", 1, snapshotPath)
	if err != nil {
		t.Fatalf("Failed to insert snapshot: %v", err)
	}

	// Create manager
	manager := New(db, snapshotDir)

	// Test restoring snapshot
	// Note: This will actually try to call brew commands
	// In a real test environment, we'd mock the brew package
	// For now, we just verify the function returns an error
	// since the package doesn't exist
	t.Run("RestoreNonexistentSnapshot", func(t *testing.T) {
		err := manager.RestoreSnapshot(9999)
		if err == nil {
			t.Error("Expected error for nonexistent snapshot, got nil")
		}
	})

	t.Run("RestoreValidSnapshot", func(t *testing.T) {
		// This test would require mocking brew.Install and brew.AddTap
		// For now, we just verify the snapshot can be loaded
		snapshot, err := db.GetSnapshot(snapshotID)
		if err != nil {
			t.Fatalf("Failed to get snapshot: %v", err)
		}

		if snapshot.ID != snapshotID {
			t.Errorf("Expected snapshot ID %d, got %d", snapshotID, snapshot.ID)
		}

		// Load the snapshot file to verify it's valid
		loaded, err := loadSnapshotFile(snapshot.SnapshotPath)
		if err != nil {
			t.Fatalf("Failed to load snapshot file: %v", err)
		}

		if len(loaded.Packages) != 1 {
			t.Errorf("Expected 1 package, got %d", len(loaded.Packages))
		}

		// We can't test actual restoration without mocking brew commands
		// or having a real brew environment. The actual RestoreSnapshot
		// call would fail here since it tries to run brew commands.
	})
}

func TestRestorePackageLogic(t *testing.T) {
	// Test the package restoration logic structure
	// This doesn't actually call brew, just validates the data structure

	testPackage := &PackageSnapshot{
		Name:         "node",
		Version:      "20.0.0",
		Tap:          "homebrew/core",
		WasExplicit:  true,
		Dependencies: []string{"icu4c"},
	}

	// Verify package structure
	if testPackage.Name != "node" {
		t.Errorf("Expected name 'node', got '%s'", testPackage.Name)
	}

	if testPackage.Version != "20.0.0" {
		t.Errorf("Expected version '20.0.0', got '%s'", testPackage.Version)
	}

	if !testPackage.WasExplicit {
		t.Error("Expected WasExplicit to be true")
	}

	if len(testPackage.Dependencies) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(testPackage.Dependencies))
	}
}

func TestSnapshotDataStructure(t *testing.T) {
	// Verify SnapshotData can be marshaled and unmarshaled correctly
	original := &SnapshotData{
		CreatedAt: time.Now().Round(time.Second), // Round to avoid precision issues
		Reason:    "test",
		Packages: []*PackageSnapshot{
			{
				Name:         "pkg1",
				Version:      "1.0.0",
				Tap:          "homebrew/core",
				WasExplicit:  true,
				Dependencies: []string{"dep1", "dep2"},
			},
			{
				Name:         "pkg2",
				Version:      "2.0.0",
				Tap:          "user/tap",
				WasExplicit:  false,
				Dependencies: []string{},
			},
		},
		BrewVersion: "4.0.0",
	}

	// Marshal
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal
	var restored SnapshotData
	if err := json.Unmarshal(jsonData, &restored); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify
	if restored.Reason != original.Reason {
		t.Errorf("Expected reason '%s', got '%s'", original.Reason, restored.Reason)
	}

	if len(restored.Packages) != len(original.Packages) {
		t.Errorf("Expected %d packages, got %d", len(original.Packages), len(restored.Packages))
	}

	if restored.BrewVersion != original.BrewVersion {
		t.Errorf("Expected brew version '%s', got '%s'", original.BrewVersion, restored.BrewVersion)
	}

	// Check first package
	if restored.Packages[0].Name != "pkg1" {
		t.Errorf("Expected package name 'pkg1', got '%s'", restored.Packages[0].Name)
	}

	if len(restored.Packages[0].Dependencies) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(restored.Packages[0].Dependencies))
	}
}

func TestManagerCreation(t *testing.T) {
	// Create in-memory database
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer db.Close()

	if err := db.CreateSchema(); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Create manager
	snapshotDir := "/tmp/test-snapshots"
	manager := New(db, snapshotDir)

	if manager == nil {
		t.Fatal("Expected non-nil manager")
	}

	if manager.store == nil {
		t.Error("Expected non-nil store")
	}

	if manager.snapshotDir != snapshotDir {
		t.Errorf("Expected snapshot dir '%s', got '%s'", snapshotDir, manager.snapshotDir)
	}
}

func TestIntegrationCreateAndRestore(t *testing.T) {
	// This is an integration test that creates a snapshot and verifies
	// it can be loaded for restoration (without actually calling brew)

	tempDir := t.TempDir()
	snapshotDir := filepath.Join(tempDir, "snapshots")

	// Create in-memory database
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer db.Close()

	if err := db.CreateSchema(); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Insert test packages
	pkg1 := &brew.Package{
		Name:        "pkg1",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		InstallType: "explicit",
		Tap:         "homebrew/core",
		IsCask:      false,
		SizeBytes:   1000000,
		HasBinary:   true,
		BinaryPaths: []string{"/usr/local/bin/pkg1"},
	}

	pkg2 := &brew.Package{
		Name:        "pkg2",
		Version:     "2.0.0",
		InstalledAt: time.Now(),
		InstallType: "dependency",
		Tap:         "user/tap",
		IsCask:      false,
		SizeBytes:   500000,
		HasBinary:   false,
		BinaryPaths: []string{},
	}

	if err := db.InsertPackage(pkg1); err != nil {
		t.Fatalf("Failed to insert pkg1: %v", err)
	}

	if err := db.InsertPackage(pkg2); err != nil {
		t.Fatalf("Failed to insert pkg2: %v", err)
	}

	// Add dependency
	if err := db.InsertDependency("pkg1", "pkg2"); err != nil {
		t.Fatalf("Failed to insert dependency: %v", err)
	}

	// Create manager
	manager := New(db, snapshotDir)

	// Create snapshot
	snapshotID, err := manager.CreateSnapshot([]string{"pkg1", "pkg2"}, "integration_test")
	if err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	// Verify snapshot
	snapshot, err := db.GetSnapshot(snapshotID)
	if err != nil {
		t.Fatalf("Failed to get snapshot: %v", err)
	}

	// Load snapshot data
	snapshotData, err := loadSnapshotFile(snapshot.SnapshotPath)
	if err != nil {
		t.Fatalf("Failed to load snapshot file: %v", err)
	}

	// Verify packages are in snapshot
	if len(snapshotData.Packages) != 2 {
		t.Errorf("Expected 2 packages in snapshot, got %d", len(snapshotData.Packages))
	}

	// Verify package details
	foundPkg1 := false
	foundPkg2 := false

	for _, pkg := range snapshotData.Packages {
		if pkg.Name == "pkg1" {
			foundPkg1 = true
			if pkg.Version != "1.0.0" {
				t.Errorf("Expected pkg1 version '1.0.0', got '%s'", pkg.Version)
			}
			if !pkg.WasExplicit {
				t.Error("Expected pkg1 to be explicit")
			}
			if len(pkg.Dependencies) != 1 {
				t.Errorf("Expected 1 dependency for pkg1, got %d", len(pkg.Dependencies))
			}
		}

		if pkg.Name == "pkg2" {
			foundPkg2 = true
			if pkg.Version != "2.0.0" {
				t.Errorf("Expected pkg2 version '2.0.0', got '%s'", pkg.Version)
			}
			if pkg.WasExplicit {
				t.Error("Expected pkg2 to not be explicit")
			}
		}
	}

	if !foundPkg1 {
		t.Error("pkg1 not found in snapshot")
	}

	if !foundPkg2 {
		t.Error("pkg2 not found in snapshot")
	}
}
