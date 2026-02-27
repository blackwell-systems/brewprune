package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/blackwell-systems/brewprune/internal/shim"
	"github.com/blackwell-systems/brewprune/internal/store"
)

// shimBinaryName is the name of the shim dispatcher binary, excluded from
// selection as a test target to prevent an exec loop.
const shimBinaryExclude = "brewprune-shim"

// findShimTestBinary scans shimDir for a suitable shim symlink to exec during
// the pipeline test. It skips the brewprune-shim binary itself to avoid an
// infinite exec loop and returns the full path of the first symlink found, or
// "" if none exist.
func findShimTestBinary(shimDir string) string {
	entries, err := os.ReadDir(shimDir)
	if err != nil {
		return ""
	}

	// Prefer well-known safe binaries first.
	preferred := []string{"git", "ls", "echo", "true"}
	for _, name := range preferred {
		if name == shimBinaryExclude {
			continue
		}
		p := filepath.Join(shimDir, name)
		if info, err := os.Lstat(p); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return p
		}
	}

	// Fall back to the first available symlink that isn't the shim binary.
	for _, e := range entries {
		if e.Name() == shimBinaryExclude {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return filepath.Join(shimDir, e.Name())
		}
	}

	return ""
}

// safeArgsFor returns innocuous arguments to pass when executing the given
// binary name during a shim test.
func safeArgsFor(name string) []string {
	switch name {
	case "git":
		return []string{"--version"}
	case "echo":
		return []string{"brewprune-shimtest"}
	case "true":
		return nil
	default:
		// For ls and anything else, list /tmp which always exists.
		return []string{"-la", "/tmp"}
	}
}

// RunShimTest executes a known shimmed binary, polls the usage_events table
// for up to maxWait, and returns nil if an event appears.
// Returns an error describing the failure point if the pipeline is broken.
func RunShimTest(st *store.Store, maxWait time.Duration) error {
	// Locate the shim directory.
	shimDir, err := shim.GetShimDir()
	if err != nil {
		return fmt.Errorf("cannot determine shim directory: %w", err)
	}

	// Gracefully skip when no scan has been run yet.
	if _, err := os.Stat(shimDir); os.IsNotExist(err) {
		return fmt.Errorf("shim directory %s does not exist — run 'brewprune scan' first", shimDir)
	}

	// Pick a safe binary to exec via the shim.
	shimPath := findShimTestBinary(shimDir)
	if shimPath == "" {
		return fmt.Errorf("no shimmed binaries found in %s — run 'brewprune scan' to create shims", shimDir)
	}

	binaryName := filepath.Base(shimPath)
	args := safeArgsFor(binaryName)

	// Record timestamp before exec so we can filter for events after this point.
	before := time.Now()

	// Execute the shimmed binary. Discard its output — we only care about the
	// side-effect (a new usage_events row written by the daemon).
	cmdArgs := append([]string{shimPath}, args...)
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...) //nolint:gosec
	cmd.Stdout = nil
	cmd.Stderr = nil
	if runErr := cmd.Run(); runErr != nil {
		// Some binaries (e.g. git) exit non-zero for version; that's fine.
		// Only fail if the binary couldn't be exec'd at all.
		if _, ok := runErr.(*exec.ExitError); !ok {
			return fmt.Errorf("failed to exec shimmed binary %s: %w", shimPath, runErr)
		}
	}

	// Poll the usage_events table until we see a new event or time out.
	deadline := time.Now().Add(maxWait)
	pollInterval := 500 * time.Millisecond

	query := `SELECT COUNT(*) FROM usage_events WHERE timestamp >= ?`

	for time.Now().Before(deadline) {
		var count int
		row := st.DB().QueryRow(query, before.Format(time.RFC3339))
		if err := row.Scan(&count); err != nil {
			return fmt.Errorf("error querying usage_events: %w", err)
		}
		if count > 0 {
			return nil
		}
		time.Sleep(pollInterval)
	}

	elapsed := time.Since(before).Round(time.Millisecond)
	return fmt.Errorf(
		"no usage event recorded after %v (waited %v) — shim executed %s but daemon did not write to database",
		elapsed, maxWait, binaryName,
	)
}
