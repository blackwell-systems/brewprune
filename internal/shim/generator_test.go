package shim

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetShimDir(t *testing.T) {
	dir, err := GetShimDir()
	if err != nil {
		t.Fatalf("GetShimDir() error: %v", err)
	}
	if !strings.HasSuffix(dir, filepath.Join(".brewprune", "bin")) {
		t.Errorf("GetShimDir() = %q, want suffix %q", dir, filepath.Join(".brewprune", "bin"))
	}
}

func TestGetUsageLogPath(t *testing.T) {
	path, err := GetUsageLogPath()
	if err != nil {
		t.Fatalf("GetUsageLogPath() error: %v", err)
	}
	if !strings.HasSuffix(path, filepath.Join(".brewprune", "usage.log")) {
		t.Errorf("GetUsageLogPath() = %q, want suffix .brewprune/usage.log", path)
	}
}

func TestIsShimSetup_NotInPath(t *testing.T) {
	// Temporarily override PATH to something that doesn't include the shim dir.
	original := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", original) })

	os.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")

	ok, reason := IsShimSetup()
	if ok {
		t.Fatal("IsShimSetup() = true, want false when shim dir not in PATH")
	}
	if reason == "" {
		t.Fatal("IsShimSetup() returned empty reason")
	}
}

func TestIsShimSetup_InPathBeforeBrew(t *testing.T) {
	shimDir, err := GetShimDir()
	if err != nil {
		t.Fatal(err)
	}

	original := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", original) })

	// Place shim dir first, before homebrew.
	os.Setenv("PATH", shimDir+":/opt/homebrew/bin:/usr/bin:/bin")

	ok, reason := IsShimSetup()
	if !ok {
		t.Errorf("IsShimSetup() = false (%s), want true when shim dir is first", reason)
	}
}

func TestGenerateShims_NoShimBinary(t *testing.T) {
	// Should return an error when the shim binary hasn't been built yet.
	// We use a temp dir so we don't accidentally find a real shim binary.
	original := os.Getenv("HOME")
	tmpHome := t.TempDir()
	t.Cleanup(func() { os.Setenv("HOME", original) })
	os.Setenv("HOME", tmpHome)

	_, err := GenerateShims([]string{"/opt/homebrew/bin/git"})
	if err == nil {
		t.Fatal("GenerateShims() expected error when shim binary missing, got nil")
	}
}

func TestRemoveShims_EmptyDir(t *testing.T) {
	original := os.Getenv("HOME")
	tmpHome := t.TempDir()
	t.Cleanup(func() { os.Setenv("HOME", original) })
	os.Setenv("HOME", tmpHome)

	// Should not error when shim dir doesn't exist yet.
	if err := RemoveShims(); err != nil {
		t.Errorf("RemoveShims() on missing dir = %v, want nil", err)
	}
}

func TestRemoveShims_LeavesShimBinaryIntact(t *testing.T) {
	original := os.Getenv("HOME")
	tmpHome := t.TempDir()
	t.Cleanup(func() { os.Setenv("HOME", original) })
	os.Setenv("HOME", tmpHome)

	shimDir := filepath.Join(tmpHome, ".brewprune", "bin")
	if err := os.MkdirAll(shimDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a fake shim binary and a symlink.
	shimBin := filepath.Join(shimDir, shimBinaryName)
	if err := os.WriteFile(shimBin, []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(shimDir, "git")
	if err := os.Symlink(shimBin, symlink); err != nil {
		t.Fatal(err)
	}

	if err := RemoveShims(); err != nil {
		t.Fatalf("RemoveShims() error: %v", err)
	}

	// Shim binary should still exist.
	if _, err := os.Stat(shimBin); err != nil {
		t.Errorf("shim binary was removed: %v", err)
	}

	// Symlink should be gone.
	if _, err := os.Lstat(symlink); !os.IsNotExist(err) {
		t.Errorf("symlink was not removed")
	}
}
