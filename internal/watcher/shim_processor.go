package watcher

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/blackwell-systems/brewprune/internal/store"
)

const maxShimLogLinesPerTick = 10_000

// ProcessUsageLog reads new entries from ~/.brewprune/usage.log since the last
// processed offset, resolves binary names to package names, and batch-inserts
// usage events into the store in a single transaction.
//
// Log format (one entry per line, written by cmd/brewprune-shim):
//
//	<unix_nano>,<argv0_path>
//
// Example:
//
//	1709012345678901234,/Users/alice/.brewprune/bin/git
//
// This is designed to be called on the watcher's 30-second ticker. It returns
// nil (no error) when the log file does not yet exist.
func ProcessUsageLog(st *store.Store) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("shim_processor: get home dir: %w", err)
	}

	logPath := filepath.Join(homeDir, ".brewprune", "usage.log")
	offsetPath := filepath.Join(homeDir, ".brewprune", "usage.offset")

	// No-op: shim has not been set up yet.
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return nil
	}

	// Read byte offset from last run (0 if first run).
	offset, err := readShimOffset(offsetPath)
	if err != nil {
		return fmt.Errorf("shim_processor: read offset: %w", err)
	}

	// Build basename → package name lookup from stored packages (fallback).
	binaryMap, err := buildBasenameMap(st)
	if err != nil {
		return fmt.Errorf("shim_processor: build binary map: %w", err)
	}

	// Build full opt path → package name lookup (preferred, avoids basename collisions).
	optPathMap, err := buildOptPathMap(st)
	if err != nil {
		return fmt.Errorf("shim_processor: build opt path map: %w", err)
	}

	// Open log and seek to last known position.
	f, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("shim_processor: open log: %w", err)
	}
	defer f.Close()

	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			// Offset may be stale after log rotation — reset to 0.
			log.Printf("shim_processor: seek failed (offset=%d), resetting: %v", offset, err)
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				return fmt.Errorf("shim_processor: seek reset failed: %w", err)
			}
		}
	}

	// Collect up to maxShimLogLinesPerTick events.
	type pendingEvent struct {
		pkg        string
		binaryPath string
		timestamp  time.Time
	}
	var events []pendingEvent

	scanner := bufio.NewScanner(f)
	for scanner.Scan() && len(events) < maxShimLogLinesPerTick {
		line := scanner.Text()
		if line == "" {
			continue
		}

		tsNano, argv0, ok := parseShimLogLine(line)
		if !ok {
			log.Printf("shim_processor: skipping malformed line: %q", line)
			continue
		}

		basename := filepath.Base(argv0)

		// Try full opt path first to avoid basename collisions between formulae.
		// Apple Silicon Homebrew installs to /opt/homebrew/bin; Intel to /usr/local/bin.
		pkg, found := optPathMap["/opt/homebrew/bin/"+basename]
		if !found {
			pkg, found = optPathMap["/usr/local/bin/"+basename]
		}
		if !found {
			// Fall back to basename-only match for any remaining cases.
			pkg, found = binaryMap[basename]
		}
		if !found {
			continue // Not a managed Homebrew binary.
		}

		events = append(events, pendingEvent{
			pkg:        pkg,
			binaryPath: argv0,
			timestamp:  time.Unix(0, tsNano),
		})
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("shim_processor: scan log: %w", err)
	}

	// Capture the new file offset after scanning.
	newOffset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("shim_processor: get new offset: %w", err)
	}

	if len(events) == 0 {
		// Advance offset even if no events matched (skip unknown binaries).
		if newOffset != offset {
			return writeShimOffsetAtomic(offsetPath, newOffset)
		}
		return nil
	}

	// Batch-insert all resolved events in a single transaction.
	tx, err := st.DB().Begin()
	if err != nil {
		return fmt.Errorf("shim_processor: begin transaction: %w", err)
	}

	stmt, err := tx.Prepare(`INSERT INTO usage_events (package, event_type, binary_path, timestamp) VALUES (?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback() //nolint:errcheck
		return fmt.Errorf("shim_processor: prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, e := range events {
		eventType := "exec"
		if isConfigProbe(filepath.Base(e.binaryPath)) {
			eventType = "probe"
		}
		if _, err := stmt.Exec(e.pkg, eventType, e.binaryPath, e.timestamp.Format("2006-01-02T15:04:05Z07:00")); err != nil {
			tx.Rollback() //nolint:errcheck
			return fmt.Errorf("shim_processor: insert event for %s: %w", e.pkg, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("shim_processor: commit: %w", err)
	}

	// Only advance the offset after successful commit — crash-safe.
	return writeShimOffsetAtomic(offsetPath, newOffset)
}

// buildBasenameMap builds a map of binary basename → package name from
// all packages stored in the database.
//
// Example: {"git": "git", "rg": "ripgrep", "node": "node"}
func buildBasenameMap(st *store.Store) (map[string]string, error) {
	packages, err := st.ListPackages()
	if err != nil {
		return nil, fmt.Errorf("list packages: %w", err)
	}

	m := make(map[string]string, len(packages)*2)
	for _, pkg := range packages {
		// Map each stored binary path's basename to this package.
		for _, binPath := range pkg.BinaryPaths {
			m[filepath.Base(binPath)] = pkg.Name
		}
		// Fallback: also map the package name itself (handles packages where
		// BinaryPaths wasn't populated but the main binary matches the name).
		if _, exists := m[pkg.Name]; !exists {
			m[pkg.Name] = pkg.Name
		}
	}
	return m, nil
}

// buildOptPathMap builds a map of full opt binary path → package name from
// all packages stored in the database. This is the preferred lookup because it
// disambiguates formulae that ship binaries sharing the same basename (e.g.
// two packages both providing a "convert" binary).
//
// Example: {"/opt/homebrew/bin/git": "git", "/opt/homebrew/bin/rg": "ripgrep"}
func buildOptPathMap(st *store.Store) (map[string]string, error) {
	packages, err := st.ListPackages()
	if err != nil {
		return nil, fmt.Errorf("list packages: %w", err)
	}

	m := make(map[string]string, len(packages)*2)
	for _, pkg := range packages {
		for _, binPath := range pkg.BinaryPaths {
			m[binPath] = pkg.Name
		}
	}
	return m, nil
}

// parseShimLogLine parses a line of the form "<unix_nano>,<argv0_path>".
// Returns (0, "", false) on any parse error.
func parseShimLogLine(line string) (int64, string, bool) {
	idx := strings.IndexByte(line, ',')
	if idx <= 0 || idx >= len(line)-1 {
		return 0, "", false
	}

	tsStr := line[:idx]
	argv0 := line[idx+1:]

	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil || ts <= 0 {
		return 0, "", false
	}
	if argv0 == "" {
		return 0, "", false
	}

	return ts, argv0, true
}

// readShimOffset reads the byte offset from the offset tracking file.
// Returns 0 if the file does not exist.
func readShimOffset(offsetPath string) (int64, error) {
	data, err := os.ReadFile(offsetPath)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(data))
	if s == "" {
		return 0, nil
	}
	offset, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse offset %q: %w", s, err)
	}
	return offset, nil
}

// isConfigProbe reports whether name is a build-system compiler-flag probe
// binary (e.g. "pkg-config", "freetype-config", "Magick++-config").
// These are invoked automatically by build systems and must not affect usage scores.
func isConfigProbe(name string) bool {
	return name == "pkg-config" || strings.HasSuffix(name, "-config")
}

// writeShimOffsetAtomic writes newOffset to offsetPath via a temp-file rename,
// ensuring the update is atomic and crash-safe.
func writeShimOffsetAtomic(offsetPath string, newOffset int64) error {
	dir := filepath.Dir(offsetPath)
	tmpPath := filepath.Join(dir, ".offset.tmp")

	if err := os.WriteFile(tmpPath, []byte(strconv.FormatInt(newOffset, 10)), 0600); err != nil {
		return fmt.Errorf("write temp offset file: %w", err)
	}
	if err := os.Rename(tmpPath, offsetPath); err != nil {
		return fmt.Errorf("rename offset file: %w", err)
	}
	return nil
}
