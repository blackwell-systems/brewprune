// Package shim manages the PATH shim layer that intercepts Homebrew binary
// executions for usage tracking.
//
// Architecture:
//   - A single Go binary (~/.brewprune/bin/brewprune-shim) handles all shimmed commands.
//   - Symlinks are created for each tracked Homebrew binary pointing to that binary.
//   - The shim binary determines which command was invoked via filepath.Base(os.Args[0]).
//   - Executions are logged to ~/.brewprune/usage.log for batch processing by the daemon.
package shim

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const shimBinaryName = "brewprune-shim"

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

// BuildShimBinary compiles the shim binary from source and places it at
// <shimDir>/brewprune-shim. Requires Go toolchain on PATH.
func BuildShimBinary() error {
	shimDir, err := GetShimDir()
	if err != nil {
		return fmt.Errorf("cannot get shim dir: %w", err)
	}

	if err := os.MkdirAll(shimDir, 0755); err != nil {
		return fmt.Errorf("cannot create shim dir %s: %w", shimDir, err)
	}

	outputPath := filepath.Join(shimDir, shimBinaryName)
	cmd := exec.Command("go", "build", "-o", outputPath,
		"github.com/blackwell-systems/brewprune/cmd/brewprune-shim")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to compile shim binary: %w", err)
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
		found, err := exec.LookPath(basename)
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

		symlinkPath := filepath.Join(shimDir, basename)

		// Skip if already correctly shimmed.
		if existing, err := os.Readlink(symlinkPath); err == nil && existing == shimBinary {
			continue
		}

		// Remove stale symlink or regular file if present.
		os.Remove(symlinkPath)

		if err := os.Symlink(shimBinary, symlinkPath); err != nil {
			return count, fmt.Errorf("failed to create shim for %s: %w", basename, err)
		}
		count++
	}

	return count, nil
}

// RemoveShims removes all symlinks in the shim directory.
// Leaves the brewprune-shim binary itself intact.
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

	for _, entry := range entries {
		if entry.Name() == shimBinaryName {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			os.Remove(filepath.Join(shimDir, entry.Name()))
		}
	}

	return nil
}
