// Package shim manages the PATH shim layer that intercepts Homebrew binary
// executions for usage tracking.
//
// Architecture:
//   - A single Go binary (~/.brewprune/bin/brewprune-shim) handles all shimmed commands.
//   - Hard links are created for each tracked Homebrew binary pointing to that binary.
//     Hard links share the same inode as brewprune-shim, so they appear as regular
//     executables (not symlinks) to enterprise EDR scanners like CrowdStrike Falcon.
//   - The shim binary determines which command was invoked via filepath.Base(os.Args[0]).
//   - Executions are logged to ~/.brewprune/usage.log for batch processing by the daemon.
package shim

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const shimBinaryName = "brewprune-shim"

// shimInode returns the inode number of the shim binary, used to identify
// hard-link shim entries in the shim directory.
func shimInode(shimBinary string) (uint64, error) {
	info, err := os.Stat(shimBinary)
	if err != nil {
		return 0, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("cannot read inode for %s", shimBinary)
	}
	return stat.Ino, nil
}

// isShimEntry reports whether a file in the shim directory is a shim — either
// a symlink (legacy) or a hard link sharing the shim binary's inode.
func isShimEntry(path string, shimBinaryIno uint64) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	// Legacy: symlink pointing into the shim directory.
	if info.Mode()&os.ModeSymlink != 0 {
		return true
	}
	// Current: hard link with the same inode as the shim binary.
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return stat.Ino == shimBinaryIno
	}
	return false
}

// createShimEntry creates a hard link from shimPath to the shim binary.
// Hard links appear as regular executables to filesystem scanners, avoiding
// removal by enterprise EDR tools (e.g. CrowdStrike Falcon) that target
// symlink-based PATH hijacking patterns.
func createShimEntry(shimBinary, shimPath string) error {
	// Remove any existing entry (stale symlink, wrong hard link, etc.).
	os.Remove(shimPath)
	return os.Link(shimBinary, shimPath)
}

// lookPathExcludingShimDir searches PATH for basename, skipping the shim
// directory. This prevents LookPath from finding our own shim hard links and
// treating them as the "real" binary — which would cause RefreshShims/
// GenerateShims to conclude the desired set is empty and delete all shims.
func lookPathExcludingShimDir(basename, shimDir string) (string, error) {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == shimDir {
			continue
		}
		candidate := filepath.Join(dir, basename)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return candidate, nil
		}
	}
	return "", &exec.Error{Name: basename, Err: exec.ErrNotFound}
}

// GetShimDir returns the directory where shim symlinks are stored.
// Default: ~/.brewprune/bin
func GetShimDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".brewprune", "bin"), nil
}

// GetUsageLogPath returns the path to the usage log written by the shim binary.
func GetUsageLogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".brewprune", "usage.log"), nil
}

// IsShimSetup reports whether the shim directory is correctly positioned in PATH
// (i.e., appears before the Homebrew bin directory).
// Returns (true, "") on success, or (false, reason) explaining what needs fixing.
func IsShimSetup() (bool, string) {
	shimDir, err := GetShimDir()
	if err != nil {
		return false, fmt.Sprintf("cannot get shim dir: %v", err)
	}

	pathDirs := filepath.SplitList(os.Getenv("PATH"))
	shimIdx := -1
	brewIdx := -1

	for i, dir := range pathDirs {
		if dir == shimDir {
			shimIdx = i
		}
		// Detect the Homebrew bin directory heuristically.
		if brewIdx == -1 && strings.HasSuffix(dir, "/bin") &&
			(strings.Contains(dir, "homebrew") || dir == "/usr/local/bin") {
			brewIdx = i
		}
	}

	if shimIdx == -1 {
		return false, fmt.Sprintf(
			"add shim directory to PATH before Homebrew:\n  export PATH=%q:$PATH",
			shimDir,
		)
	}
	if brewIdx != -1 && shimIdx > brewIdx {
		return false, fmt.Sprintf(
			"shim directory must appear before %s in PATH\n  export PATH=%q:$PATH",
			pathDirs[brewIdx], shimDir,
		)
	}
	return true, ""
}

// BuildShimBinary ensures the shim binary exists at <shimDir>/brewprune-shim.
//
// Strategy (in order):
//  1. If brewprune-shim is already in the same directory as the running
//     brewprune binary (true after `go install ./...` or a GoReleaser build),
//     copy it into shimDir.
//  2. Otherwise run `go install` for the shim package (dev workflow) and copy
//     from GOPATH/bin.
func BuildShimBinary() error {
	shimDir, err := GetShimDir()
	if err != nil {
		return fmt.Errorf("cannot get shim dir: %w", err)
	}

	if err := os.MkdirAll(shimDir, 0755); err != nil {
		return fmt.Errorf("cannot create shim dir %s: %w", shimDir, err)
	}

	outputPath := filepath.Join(shimDir, shimBinaryName)

	// Strategy 1: look for brewprune-shim next to the running brewprune binary.
	if self, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(self), shimBinaryName)
		if _, err := os.Stat(candidate); err == nil {
			return copyFile(candidate, outputPath)
		}
	}

	// Strategy 2: go install into GOPATH/bin, then copy.
	installCmd := exec.Command("go", "install",
		"github.com/blackwell-systems/brewprune/cmd/brewprune-shim")
	installCmd.Stdout = os.Stderr
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("failed to install shim binary: %w", err)
	}

	// Find the installed binary.
	gopath, err := goPath()
	if err != nil {
		return fmt.Errorf("cannot determine GOPATH: %w", err)
	}
	installed := filepath.Join(gopath, "bin", shimBinaryName)
	if _, err := os.Stat(installed); err != nil {
		return fmt.Errorf("shim binary not found at %s after install", installed)
	}

	return copyFile(installed, outputPath)
}

// goPath returns the effective GOPATH.
func goPath() (string, error) {
	out, err := exec.Command("go", "env", "GOPATH").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// copyFile copies src to dst, making dst executable.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("open dest: %w", err)
	}
	defer out.Close()

	buf := make([]byte, 1<<20) // 1 MB buffer
	for {
		n, readErr := in.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				return fmt.Errorf("write: %w", err)
			}
		}
		if readErr != nil {
			if readErr.Error() == "EOF" {
				break
			}
			return fmt.Errorf("read: %w", readErr)
		}
	}
	return nil
}

// GenerateShims creates symlinks in the shim directory for each Homebrew binary
// that would be found on the user's PATH. Only binaries where exec.LookPath
// resolves to the brew version are shimmed — this prevents shadowing system
// tools (e.g. /usr/bin/python vs. brew's python).
//
// binaries is a list of full binary paths, e.g. ["/opt/homebrew/bin/git", ...].
// Returns the count of newly created symlinks.
func GenerateShims(binaries []string) (int, error) {
	shimDir, err := GetShimDir()
	if err != nil {
		return 0, fmt.Errorf("cannot get shim dir: %w", err)
	}

	shimBinary := filepath.Join(shimDir, shimBinaryName)
	if _, err := os.Stat(shimBinary); os.IsNotExist(err) {
		return 0, fmt.Errorf(
			"shim binary not found at %s; run 'brewprune scan' first to build it",
			shimBinary,
		)
	}

	count := 0
	for _, binPath := range binaries {
		basename := filepath.Base(binPath)

		// Skip the shim binary itself.
		if basename == shimBinaryName {
			continue
		}

		// Only shim if the brew version is what the user would actually run.
		// Exclude the shim dir from PATH so we don't find our own symlinks.
		found, err := lookPathExcludingShimDir(basename, shimDir)
		if err != nil {
			continue // not on PATH at all
		}

		// Resolve symlinks before comparing so /opt/homebrew/bin/git (symlink
		// into Cellar) compares equal to binPath's resolved form.
		resolvedFound, _ := filepath.EvalSymlinks(found)
		resolvedBin, _ := filepath.EvalSymlinks(binPath)

		if resolvedFound != resolvedBin && found != binPath {
			// A different binary (e.g. system version) would be run — skip.
			continue
		}

		shimPath := filepath.Join(shimDir, basename)

		// Skip if already correctly shimmed (hard link to the shim binary).
		if info, err := os.Lstat(shimPath); err == nil {
			if stat, ok := info.Sys().(*syscall.Stat_t); ok {
				shimIno, _ := shimInode(shimBinary)
				if shimIno != 0 && stat.Ino == shimIno {
					continue // already a valid hard-link shim
				}
			}
		}

		if err := createShimEntry(shimBinary, shimPath); err != nil {
			return count, fmt.Errorf("failed to create shim for %s: %w", basename, err)
		}
		count++
	}

	return count, nil
}

// RefreshShims performs an incremental diff of the desired shim set against the
// symlinks that currently exist in the shim directory.
//
// For each basename derived from binaries, the same LookPath collision-safety
// logic as GenerateShims is applied: a symlink is only created if the brew
// version of the binary is what the user would actually run.
//
// Returns the count of symlinks added and the count removed.
// This function does NOT rebuild the shim binary itself; call BuildShimBinary
// separately when the binary needs to be updated.
func RefreshShims(binaries []string) (added int, removed int, err error) {
	shimDir, err := GetShimDir()
	if err != nil {
		return 0, 0, fmt.Errorf("cannot get shim dir: %w", err)
	}

	shimBinary := filepath.Join(shimDir, shimBinaryName)
	if _, err := os.Stat(shimBinary); os.IsNotExist(err) {
		return 0, 0, fmt.Errorf(
			"shim binary not found at %s; run 'brewprune scan' first to build it",
			shimBinary,
		)
	}

	// Read existing symlinks in shimDir (skip the shim binary itself).
	entries, err := os.ReadDir(shimDir)
	if err != nil && !os.IsNotExist(err) {
		return 0, 0, fmt.Errorf("cannot read shim dir %s: %w", shimDir, err)
	}

	current := make(map[string]struct{})
	// Get the shim binary's inode to identify hard-link shim entries.
	shimBinaryIno, _ := shimInode(shimBinary)

	for _, entry := range entries {
		name := entry.Name()
		if name == shimBinaryName {
			continue
		}
		entryPath := filepath.Join(shimDir, name)
		if isShimEntry(entryPath, shimBinaryIno) {
			current[name] = struct{}{}
		}
	}

	// Build the desired set — apply the same LookPath collision-safety logic as
	// GenerateShims so we only shim binaries that the brew version would run.
	// Exclude the shim dir from PATH so existing shim entries don't shadow
	// the real binaries and cause the desired set to be empty.
	desired := make(map[string]struct{})
	for _, binPath := range binaries {
		basename := filepath.Base(binPath)
		if basename == shimBinaryName {
			continue
		}

		found, err := lookPathExcludingShimDir(basename, shimDir)
		if err != nil {
			continue // not on PATH at all
		}

		resolvedFound, _ := filepath.EvalSymlinks(found)
		resolvedBin, _ := filepath.EvalSymlinks(binPath)

		if resolvedFound != resolvedBin && found != binPath {
			continue // system version would be run — skip
		}

		desired[basename] = struct{}{}
	}

	// Create hard-link shim entries that are desired but not yet present.
	for basename := range desired {
		shimPath := filepath.Join(shimDir, basename)
		if _, exists := current[basename]; exists {
			// Already present — verify it's the correct inode.
			if shimBinaryIno != 0 {
				if info, err := os.Lstat(shimPath); err == nil {
					if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat.Ino == shimBinaryIno {
						continue // already a valid hard-link shim
					}
				}
			}
			// Stale entry — remove and recreate.
			os.Remove(shimPath)
		}

		if err := createShimEntry(shimBinary, shimPath); err != nil {
			return added, removed, fmt.Errorf("failed to create shim for %s: %w", basename, err)
		}
		added++
	}

	// Remove shim entries that are present but no longer desired.
	for basename := range current {
		if _, exists := desired[basename]; !exists {
			if err := os.Remove(filepath.Join(shimDir, basename)); err != nil {
				return added, removed, fmt.Errorf("failed to remove stale shim for %s: %w", basename, err)
			}
			removed++
		}
	}

	return added, removed, nil
}

// shimVersionPath returns the path to the shim version file.
func shimVersionPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".brewprune", "shim.version"), nil
}

// WriteShimVersion writes version atomically to ~/.brewprune/shim.version using
// a temp-file rename pattern so the update is crash-safe.
func WriteShimVersion(version string) error {
	versionPath, err := shimVersionPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(versionPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create directory %s: %w", dir, err)
	}

	tmpPath := filepath.Join(dir, ".shim.version.tmp")
	if err := os.WriteFile(tmpPath, []byte(version), 0600); err != nil {
		return fmt.Errorf("write temp version file: %w", err)
	}
	if err := os.Rename(tmpPath, versionPath); err != nil {
		return fmt.Errorf("rename version file: %w", err)
	}
	return nil
}

// ReadShimVersion returns the version string from ~/.brewprune/shim.version.
// Returns ("", nil) if the file does not exist — this is not an error; it simply
// means that a shim scan has not been run yet.
func ReadShimVersion() (string, error) {
	versionPath, err := shimVersionPath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(versionPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read shim version: %w", err)
	}
	return string(data), nil
}

// RemoveShims removes all shim entries (symlinks or hard links) in the shim
// directory. Leaves the brewprune-shim binary itself intact.
func RemoveShims() error {
	shimDir, err := GetShimDir()
	if err != nil {
		return fmt.Errorf("cannot get shim dir: %w", err)
	}

	entries, err := os.ReadDir(shimDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot read shim dir %s: %w", shimDir, err)
	}

	shimBinary := filepath.Join(shimDir, shimBinaryName)
	shimBinaryIno, _ := shimInode(shimBinary)

	for _, entry := range entries {
		if entry.Name() == shimBinaryName {
			continue
		}
		entryPath := filepath.Join(shimDir, entry.Name())
		if isShimEntry(entryPath, shimBinaryIno) {
			os.Remove(entryPath)
		}
	}

	return nil
}
