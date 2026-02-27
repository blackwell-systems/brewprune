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

// setupRefreshShimsEnv creates a temp home with a fake shim binary and a
// "brew bin" directory containing a fake binary named basename.
// It sets HOME and PATH so that LookPath(basename) resolves to brewBin/basename
// (i.e., the brew version is what would be run).
// Returns shimDir, the full path to the fake brew binary, and a cleanup func.
func setupRefreshShimsEnv(t *testing.T, basename string) (shimDir string, brewBinPath string) {
	t.Helper()

	tmpHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	originalPath := os.Getenv("PATH")
	t.Cleanup(func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("PATH", originalPath)
	})
	os.Setenv("HOME", tmpHome)

	shimDir = filepath.Join(tmpHome, ".brewprune", "bin")
	if err := os.MkdirAll(shimDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a fake shim binary.
	shimBin := filepath.Join(shimDir, shimBinaryName)
	if err := os.WriteFile(shimBin, []byte("fake-shim"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a fake "brew" bin directory with a fake binary.
	brewBinDir := filepath.Join(tmpHome, "brew", "bin")
	if err := os.MkdirAll(brewBinDir, 0755); err != nil {
		t.Fatal(err)
	}
	brewBinPath = filepath.Join(brewBinDir, basename)
	if err := os.WriteFile(brewBinPath, []byte("fake-brew-binary"), 0755); err != nil {
		t.Fatal(err)
	}

	// Put the brew bin dir first in PATH so LookPath finds it.
	// shimDir must NOT be in PATH (or after brewBinDir) to avoid circular lookup.
	os.Setenv("PATH", brewBinDir+":/usr/bin:/bin")

	return shimDir, brewBinPath
}

func TestRefreshShims_NoShimBinary(t *testing.T) {
	originalHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	t.Cleanup(func() { os.Setenv("HOME", originalHome) })
	os.Setenv("HOME", tmpHome)

	_, _, err := RefreshShims([]string{"/opt/homebrew/bin/git"})
	if err == nil {
		t.Fatal("RefreshShims() expected error when shim binary missing, got nil")
	}
}

func TestRefreshShims_AddsNewSymlinks(t *testing.T) {
	shimDir, brewBinPath := setupRefreshShimsEnv(t, "mytool")

	added, removed, err := RefreshShims([]string{brewBinPath})
	if err != nil {
		t.Fatalf("RefreshShims() error: %v", err)
	}
	if added != 1 {
		t.Errorf("RefreshShims() added = %d, want 1", added)
	}
	if removed != 0 {
		t.Errorf("RefreshShims() removed = %d, want 0", removed)
	}

	// Verify the symlink was created and points to the shim binary.
	shimBin := filepath.Join(shimDir, shimBinaryName)
	symlinkPath := filepath.Join(shimDir, "mytool")
	target, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("symlink not created: %v", err)
	}
	if target != shimBin {
		t.Errorf("symlink target = %q, want %q", target, shimBin)
	}
}

func TestRefreshShims_Idempotent(t *testing.T) {
	_, brewBinPath := setupRefreshShimsEnv(t, "mytool")

	// First call.
	added1, removed1, err := RefreshShims([]string{brewBinPath})
	if err != nil {
		t.Fatalf("first RefreshShims() error: %v", err)
	}

	// Second call — should add/remove nothing.
	added2, removed2, err := RefreshShims([]string{brewBinPath})
	if err != nil {
		t.Fatalf("second RefreshShims() error: %v", err)
	}

	if added1 != 1 {
		t.Errorf("first call: added = %d, want 1", added1)
	}
	if removed1 != 0 {
		t.Errorf("first call: removed = %d, want 0", removed1)
	}
	if added2 != 0 {
		t.Errorf("second call (idempotent): added = %d, want 0", added2)
	}
	if removed2 != 0 {
		t.Errorf("second call (idempotent): removed = %d, want 0", removed2)
	}
}

func TestRefreshShims_RemovesStaleSymlinks(t *testing.T) {
	shimDir, brewBinPath := setupRefreshShimsEnv(t, "mytool")

	// First call: add the symlink.
	_, _, err := RefreshShims([]string{brewBinPath})
	if err != nil {
		t.Fatalf("first RefreshShims() error: %v", err)
	}

	// Second call with empty binaries list: stale symlink should be removed.
	added, removed, err := RefreshShims([]string{})
	if err != nil {
		t.Fatalf("second RefreshShims() error: %v", err)
	}
	if added != 0 {
		t.Errorf("added = %d, want 0", added)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}

	// Confirm the symlink is gone.
	symlinkPath := filepath.Join(shimDir, "mytool")
	if _, err := os.Lstat(symlinkPath); !os.IsNotExist(err) {
		t.Errorf("stale symlink was not removed")
	}
}

// --- WriteShimVersion / ReadShimVersion tests ---

func TestWriteReadShimVersion_RoundTrip(t *testing.T) {
	originalHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	t.Cleanup(func() { os.Setenv("HOME", originalHome) })
	os.Setenv("HOME", tmpHome)

	const version = "v1.2.3"
	if err := WriteShimVersion(version); err != nil {
		t.Fatalf("WriteShimVersion() error: %v", err)
	}

	got, err := ReadShimVersion()
	if err != nil {
		t.Fatalf("ReadShimVersion() error: %v", err)
	}
	if got != version {
		t.Errorf("ReadShimVersion() = %q, want %q", got, version)
	}
}

func TestReadShimVersion_MissingFile(t *testing.T) {
	originalHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	t.Cleanup(func() { os.Setenv("HOME", originalHome) })
	os.Setenv("HOME", tmpHome)

	// No file written — should return ("", nil).
	got, err := ReadShimVersion()
	if err != nil {
		t.Fatalf("ReadShimVersion() error: %v, want nil", err)
	}
	if got != "" {
		t.Errorf("ReadShimVersion() = %q, want empty string", got)
	}
}

func TestWriteShimVersion_FilePermissions(t *testing.T) {
	originalHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	t.Cleanup(func() { os.Setenv("HOME", originalHome) })
	os.Setenv("HOME", tmpHome)

	if err := WriteShimVersion("v0.1.0"); err != nil {
		t.Fatalf("WriteShimVersion() error: %v", err)
	}

	versionPath := filepath.Join(tmpHome, ".brewprune", "shim.version")
	info, err := os.Stat(versionPath)
	if err != nil {
		t.Fatalf("stat version file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("version file permissions = %04o, want 0600", perm)
	}
}
