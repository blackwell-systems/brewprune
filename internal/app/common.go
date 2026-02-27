package app

import (
	"fmt"
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

// formatSize converts bytes to human-readable size.
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.0f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.0f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
