package app

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/blackwell-systems/brewprune/internal/store"
	"github.com/blackwell-systems/brewprune/internal/watcher"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check daemon status and tracking statistics",
	Long: `Display the current status of the brewprune daemon and tracking statistics.

Shows:
  • Daemon running status and PID
  • Database location and validity
  • Number of packages being tracked
  • Total usage events logged
  • Time since tracking started
  • Most recent package activity

This command helps verify that usage tracking is working correctly.`,
	Example: `  # Check status
  brewprune status`,
	RunE: runStatus,
}

func init() {
	// Register with root command
	RootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Get default paths
	pidFile, err := getDefaultPIDFile()
	if err != nil {
		return fmt.Errorf("failed to get PID file path: %w", err)
	}

	dbPath, err := getDBPath()
	if err != nil {
		return fmt.Errorf("failed to get database path: %w", err)
	}

	// Check daemon status
	daemonRunning, err := watcher.IsDaemonRunning(pidFile)
	if err != nil {
		return fmt.Errorf("failed to check daemon status: %w", err)
	}

	// Get PID if daemon is running
	var pid int
	if daemonRunning {
		pidData, err := os.ReadFile(pidFile)
		if err == nil {
			pidStr := strings.TrimSpace(string(pidData))
			pid, _ = strconv.Atoi(pidStr)
		}
	}

	// Check database
	dbExists := false
	if _, err := os.Stat(dbPath); err == nil {
		dbExists = true
	}

	if !dbExists {
		fmt.Println("brewprune is not set up — run 'brewprune scan' to get started.")
		return nil
	}

	// Open database to get statistics
	var formulaeCount int
	var totalEvents int
	var events24h int
	var trackingSince *time.Time
	var dbSize int64

	st, err := store.New(dbPath)
	if err == nil {
		defer st.Close()

		packages, err := st.ListPackages()
		if err == nil {
			formulaeCount = len(packages)
		}
		totalEvents, trackingSince, _ = getUsageStats(st)
		events24h = getEvents24h(st)
	}

	if fi, err := os.Stat(dbPath); err == nil {
		dbSize = fi.Size()
	}

	// Shim info
	shimDir, _ := os.UserHomeDir()
	shimDir = shimDir + "/.brewprune/bin"
	shimCount := countSymlinks(shimDir)
	shimActive := shimCount > 0
	pathOK := isOnPATH(shimDir)

	const label = "%-14s"

	fmt.Println()

	// Tracking line
	if daemonRunning {
		pidSince := daemonSince(pidFile)
		fmt.Printf(label+"running (since %s, PID %d)\n", "Tracking:", pidSince, pid)
	} else {
		fmt.Printf(label+"stopped  (run 'brewprune watch --daemon')\n", "Tracking:")
	}

	// Events line
	fmt.Printf(label+"%s total · %d in last 24h\n", "Events:", formatNumber(totalEvents), events24h)

	// Shims line
	shimStatus := "inactive"
	if shimActive {
		shimStatus = "active"
	}
	pathStatus := "PATH ok"
	if !pathOK {
		pathStatus = "PATH missing ⚠"
	}
	fmt.Printf(label+"%s · %d commands · %s\n", "Shims:", shimStatus, shimCount, pathStatus)
	if !pathOK && totalEvents > 0 {
		fmt.Printf("              %s\n", "Note: events are from setup self-test, not real shim interception.")
		fmt.Printf("              %s\n", "Real tracking starts when PATH is fixed and shims are in front of Homebrew.")
	}

	// Last scan line (use DB mtime as proxy)
	dbMtime := "unknown"
	if fi, err := os.Stat(dbPath); err == nil {
		dbMtime = formatDuration(time.Since(fi.ModTime()))
	}
	fmt.Printf(label+"%s · %d formulae · %s\n", "Last scan:", dbMtime, formulaeCount, formatSize(dbSize))

	// Data quality line
	var quality string
	if trackingSince != nil {
		days := int(time.Since(*trackingSince).Hours() / 24)
		if days < 14 {
			quality = fmt.Sprintf("COLLECTING (%d of 14 days)", days)
		} else {
			quality = "READY"
		}
	} else {
		quality = "COLLECTING (0 of 14 days)"
	}
	fmt.Printf(label+"%s\n", "Data quality:", quality)

	fmt.Println()
	return nil
}

// getEvents24h counts usage events in the last 24 hours.
func getEvents24h(st *store.Store) int {
	cutoff := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	var count int
	row := st.DB().QueryRow("SELECT COUNT(*) FROM usage_events WHERE timestamp >= ?", cutoff)
	row.Scan(&count)
	return count
}

// countSymlinks counts symlinks in dir (non-recursive).
func countSymlinks(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if e.Type()&os.ModeSymlink != 0 {
			count++
		}
	}
	return count
}

// isOnPATH reports whether dir appears in the current PATH.
func isOnPATH(dir string) bool {
	path := os.Getenv("PATH")
	for _, p := range strings.Split(path, ":") {
		if p == dir {
			return true
		}
	}
	return false
}

// daemonSince returns a human-readable age of the PID file (proxy for daemon start time).
func daemonSince(pidFile string) string {
	fi, err := os.Stat(pidFile)
	if err != nil {
		return "unknown"
	}
	return formatDuration(time.Since(fi.ModTime()))
}

// UsageEventInfo holds information about a usage event
type UsageEventInfo struct {
	Package   string
	Timestamp time.Time
}

// getUsageStats queries usage statistics from the database
func getUsageStats(st *store.Store) (int, *time.Time, *UsageEventInfo) {
	// Count total events
	var totalEvents int
	row := st.DB().QueryRow("SELECT COUNT(*) FROM usage_events")
	row.Scan(&totalEvents)

	// Get earliest event (tracking start time)
	var trackingSince *time.Time
	var timestampStr string
	row = st.DB().QueryRow("SELECT timestamp FROM usage_events ORDER BY timestamp ASC LIMIT 1")
	err := row.Scan(&timestampStr)
	if err == nil {
		t, err := time.Parse(time.RFC3339, timestampStr)
		if err == nil {
			trackingSince = &t
		}
	}

	// Get most recent event
	var lastEvent *UsageEventInfo
	var pkg string
	row = st.DB().QueryRow("SELECT package, timestamp FROM usage_events ORDER BY timestamp DESC LIMIT 1")
	err = row.Scan(&pkg, &timestampStr)
	if err == nil {
		t, err := time.Parse(time.RFC3339, timestampStr)
		if err == nil {
			lastEvent = &UsageEventInfo{
				Package:   pkg,
				Timestamp: t,
			}
		}
	}

	return totalEvents, trackingSince, lastEvent
}

// formatNumber formats a number with thousands separators
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%s,%03d", formatNumber(n/1000), n%1000)
}

// formatDuration formats a duration in human-readable form
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds ago", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day ago"
	}
	if days < 30 {
		return fmt.Sprintf("%d days ago", days)
	}
	if days < 60 {
		return "1 month ago"
	}
	months := days / 30
	if months < 12 {
		return fmt.Sprintf("%d months ago", months)
	}
	years := months / 12
	if years == 1 {
		return "1 year ago"
	}
	return fmt.Sprintf("%d years ago", years)
}
