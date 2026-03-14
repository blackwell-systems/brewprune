package shim

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGenerateShims_SkipsNonExistentBinaries verifies that GenerateShims does
// not create shims for binaries that don't exist on disk, even if they are
// present in the binaries list (e.g., stale database entries).
//
// This test validates the fix for the issue where broken symlinks like
// /opt/homebrew/bin/cat (from bat package) would create shims that failed
// at runtime with "cannot find real binary".
func TestGenerateShims_SkipsNonExistentBinaries(t *testing.T) {
	// Setup: Create temp HOME with .brewprune/bin
	tmpHome := t.TempDir()
	shimDir := filepath.Join(tmpHome, ".brewprune", "bin")
	if err := os.MkdirAll(shimDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create shim binary
	shimBinary := filepath.Join(shimDir, shimBinaryName)
	if err := os.WriteFile(shimBinary, []byte("#!/bin/sh\necho shim\n"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a real binary that exists
	realBinDir := filepath.Join(tmpHome, "brew-bin")
	if err := os.MkdirAll(realBinDir, 0755); err != nil {
		t.Fatal(err)
	}
	realBin := filepath.Join(realBinDir, "existing-tool")
	if err := os.WriteFile(realBin, []byte("#!/bin/sh\necho real\n"), 0755); err != nil {
		t.Fatal(err)
	}

	// Override HOME and PATH for test
	origHome := os.Getenv("HOME")
	origPath := os.Getenv("PATH")
	os.Setenv("HOME", tmpHome)
	os.Setenv("PATH", realBinDir+":"+origPath)
	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("PATH", origPath)
	}()

	// Test with mix of existing and non-existing binaries
	binaries := []string{
		realBin,                               // exists - should create shim
		"/nonexistent/path/to/broken-tool",    // doesn't exist - should skip
		filepath.Join(realBinDir, "fake-cat"), // doesn't exist - should skip
	}

	count, err := GenerateShims(binaries)
	if err != nil {
		t.Fatalf("GenerateShims failed: %v", err)
	}

	// Should only create 1 shim (for the existing binary)
	if count != 1 {
		t.Errorf("Expected 1 shim created, got %d", count)
	}

	// Verify only the real binary got a shim
	existingShim := filepath.Join(shimDir, "existing-tool")
	if _, err := os.Lstat(existingShim); err != nil {
		t.Errorf("Expected shim for existing-tool to be created: %v", err)
	}

	// Verify non-existent binaries did NOT get shims
	brokenShim := filepath.Join(shimDir, "broken-tool")
	if _, err := os.Lstat(brokenShim); err == nil {
		t.Error("Shim for broken-tool should not have been created (binary doesn't exist)")
	}

	catShim := filepath.Join(shimDir, "fake-cat")
	if _, err := os.Lstat(catShim); err == nil {
		t.Error("Shim for fake-cat should not have been created (binary doesn't exist)")
	}
}

// TestGenerateShims_SkipsBrewpruneItself verifies that brewprune never creates
// a shim for itself, preventing circular self-tracking of usage events.
func TestGenerateShims_SkipsBrewpruneItself(t *testing.T) {
	tmpHome := t.TempDir()
	shimDir := filepath.Join(tmpHome, ".brewprune", "bin")
	if err := os.MkdirAll(shimDir, 0755); err != nil {
		t.Fatal(err)
	}

	shimBinary := filepath.Join(shimDir, shimBinaryName)
	if err := os.WriteFile(shimBinary, []byte("#!/bin/sh\necho shim\n"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a mock brewprune binary
	brewBinDir := filepath.Join(tmpHome, "brew-bin")
	if err := os.MkdirAll(brewBinDir, 0755); err != nil {
		t.Fatal(err)
	}
	brewpruneBin := filepath.Join(brewBinDir, "brewprune")
	if err := os.WriteFile(brewpruneBin, []byte("#!/bin/sh\necho brewprune\n"), 0755); err != nil {
		t.Fatal(err)
	}

	origHome := os.Getenv("HOME")
	origPath := os.Getenv("PATH")
	os.Setenv("HOME", tmpHome)
	os.Setenv("PATH", brewBinDir+":"+origPath)
	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("PATH", origPath)
	}()

	// Try to create shim for brewprune
	binaries := []string{brewpruneBin}
	count, err := GenerateShims(binaries)
	if err != nil {
		t.Fatalf("GenerateShims failed: %v", err)
	}

	// Should create 0 shims (brewprune is explicitly skipped)
	if count != 0 {
		t.Errorf("Expected 0 shims created for brewprune, got %d", count)
	}

	// Verify no brewprune shim was created
	brewpruneShim := filepath.Join(shimDir, "brewprune")
	if _, err := os.Lstat(brewpruneShim); err == nil {
		t.Error("Shim for brewprune should not have been created (prevents circular self-tracking)")
	}
}
