package scanner

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blackwell-systems/brewprune/internal/brew"
	"github.com/blackwell-systems/brewprune/internal/store"
)

func setupTestStore(t *testing.T) *store.Store {
	t.Helper()

	// Create in-memory database for testing
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}

	// Create schema
	if err := s.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	return s
}

func TestNew(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	scanner := New(s)
	if scanner == nil {
		t.Fatal("expected non-nil scanner")
	}

	if scanner.store != s {
		t.Fatal("scanner store does not match provided store")
	}
}

func TestGetInventory(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	scanner := New(s)

	// Initially empty
	packages, err := scanner.GetInventory()
	if err != nil {
		t.Fatalf("failed to get inventory: %v", err)
	}
	if len(packages) != 0 {
		t.Fatalf("expected 0 packages, got %d", len(packages))
	}

	// Insert some test packages
	testPkgs := []*brew.Package{
		{
			Name:        "git",
			Version:     "2.43.0",
			InstalledAt: time.Now(),
			InstallType: "explicit",
			Tap:         "homebrew/core",
			HasBinary:   true,
			BinaryPaths: []string{"/opt/homebrew/bin/git"},
		},
		{
			Name:        "node",
			Version:     "20.10.0",
			InstalledAt: time.Now(),
			InstallType: "explicit",
			Tap:         "homebrew/core",
			HasBinary:   true,
			BinaryPaths: []string{"/opt/homebrew/bin/node", "/opt/homebrew/bin/npm"},
		},
	}

	for _, pkg := range testPkgs {
		if err := s.InsertPackage(pkg); err != nil {
			t.Fatalf("failed to insert package: %v", err)
		}
	}

	// Get inventory
	packages, err = scanner.GetInventory()
	if err != nil {
		t.Fatalf("failed to get inventory: %v", err)
	}

	if len(packages) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(packages))
	}

	// Verify package data
	foundGit := false
	foundNode := false
	for _, pkg := range packages {
		if pkg.Name == "git" {
			foundGit = true
			if pkg.Version != "2.43.0" {
				t.Errorf("git version mismatch: got %s, want 2.43.0", pkg.Version)
			}
			if len(pkg.BinaryPaths) != 1 {
				t.Errorf("git binary paths count: got %d, want 1", len(pkg.BinaryPaths))
			}
		}
		if pkg.Name == "node" {
			foundNode = true
			if pkg.Version != "20.10.0" {
				t.Errorf("node version mismatch: got %s, want 20.10.0", pkg.Version)
			}
			if len(pkg.BinaryPaths) != 2 {
				t.Errorf("node binary paths count: got %d, want 2", len(pkg.BinaryPaths))
			}
		}
	}

	if !foundGit {
		t.Error("git package not found in inventory")
	}
	if !foundNode {
		t.Error("node package not found in inventory")
	}
}

func TestExtractPackageFromPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "absolute path",
			path:     "/opt/homebrew/Cellar/git/2.43.0/bin/git",
			expected: "git",
		},
		{
			name:     "relative path",
			path:     "../Cellar/node/20.10.0/bin/node",
			expected: "node",
		},
		{
			name:     "complex path",
			path:     "/usr/local/Cellar/python@3.11/3.11.7/bin/python3",
			expected: "python@3.11",
		},
		{
			name:     "no cellar in path",
			path:     "/usr/bin/git",
			expected: "",
		},
		{
			name:     "cellar at end",
			path:     "/opt/homebrew/Cellar",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPackageFromPath(tt.path)
			if result != tt.expected {
				t.Errorf("extractPackageFromPath(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestRefreshBinaryPaths_Integration(t *testing.T) {
	// This is more of an integration test that would need actual brew binaries
	// For now, we'll create a mock directory structure
	s := setupTestStore(t)
	defer s.Close()

	_ = New(s)

	// Insert a test package
	pkg := &brew.Package{
		Name:        "git",
		Version:     "2.43.0",
		InstalledAt: time.Now(),
		InstallType: "explicit",
		Tap:         "homebrew/core",
		HasBinary:   false,
		BinaryPaths: []string{},
	}

	if err := s.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert package: %v", err)
	}

	// Note: RefreshBinaryPaths needs actual brew environment
	// In a real test environment, you would:
	// 1. Mock brew.GetBrewPrefix() to return a test directory
	// 2. Create test symlinks in that directory
	// 3. Verify that RefreshBinaryPaths correctly identifies them

	// For unit testing without mocking the brew package, we skip this test
	t.Skip("RefreshBinaryPaths requires brew integration - tested in integration tests")
}

func TestRefreshBinaryPaths_EmptyBinDir(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	_ = New(s)

	// Insert test packages
	pkg := &brew.Package{
		Name:        "git",
		Version:     "2.43.0",
		InstalledAt: time.Now(),
		InstallType: "explicit",
		Tap:         "homebrew/core",
		HasBinary:   true,
		BinaryPaths: []string{"/opt/homebrew/bin/git"},
	}

	if err := s.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert package: %v", err)
	}

	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	// Note: This test would need brew.GetBrewPrefix to be mockable
	// For now, we document the expected behavior
	t.Skip("RefreshBinaryPaths requires brew integration mocking")
}

func TestScanPackages_Mock(t *testing.T) {
	// This test demonstrates the expected behavior with mocked brew functions
	// In a real implementation, we would use a mocking framework or
	// dependency injection to replace the brew package functions

	s := setupTestStore(t)
	defer s.Close()

	_ = New(s)

	// Note: ScanPackages calls brew.ListInstalled() and brew.GetDependencyTree()
	// These need to be mocked for proper unit testing
	// Since Agent A is implementing the brew package in parallel,
	// we document the expected behavior here

	// Expected flow:
	// 1. brew.ListInstalled() returns list of packages
	// 2. For each package:
	//    a. Store package via s.store.InsertPackage()
	//    b. Get deps via brew.GetDependencyTree()
	//    c. Store each dependency via s.store.InsertDependency()
	// 3. Call RefreshBinaryPaths()

	t.Skip("ScanPackages requires brew package implementation from Agent A")
}

// Mock implementations for testing when brew package is not available
