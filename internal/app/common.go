package app

import (
	"os"
	"path/filepath"
	"time"
)

// getSnapshotDir returns the directory for snapshot storage.
// Uses $HOME/.brewprune/snapshots by default.
func getSnapshotDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory
		return "snapshots"
	}

	snapshotDir := filepath.Join(home, ".brewprune", "snapshots")
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		// Fallback to current directory
		return "snapshots"
	}

	return snapshotDir
}

// getNeverTime returns a zero time value representing "never used".
func getNeverTime() time.Time {
	return time.Time{}
}
