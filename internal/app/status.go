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
	if st, err := os.Stat(dbPath); err == nil {
		dbExists = true
		_ = st
	}

	if !dbExists {
		// Check if user provided a custom --db path
		if dbPath != "" && strings.Contains(dbPath, "/") {
			// Check if parent directory exists
			dbDir := dbPath
			if strings.Contains(dbPath, "/") {
				dbDir = dbPath[:strings.LastIndex(dbPath, "/")]
			}
			if _, dirErr := os.Stat(dbDir); os.IsNotExist(dirErr) {
				fmt.Printf("Error: database path does not exist: %s\n", dbPath)
				fmt.Println("Check the --db path or run 'brewprune quickstart' to set up.")
				return nil
			}
		}
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

	// Warn if daemon is running but no recent events
	if daemonRunning && events24h == 0 && totalEvents <= 2 {
		fmt.Printf("              ⚠ Daemon running but no events logged. Shims may not be intercepting commands.\n")
		fmt.Printf("              Run 'brewprune doctor' to diagnose.\n")
	}

	// Shims line
	shimStatus := "inactive"
	if shimActive {
		shimStatus = "active"
	}

	// Determine PATH status with three cases:
	// 1. PATH active: shim dir is in current $PATH
	// 2. PATH configured: shim dir is in shell profile but not yet sourced
	// 3. PATH missing: shim dir is not in shell profile
	var pathStatus string
	if pathOK {
		pathStatus = "PATH active ✓"
	} else if isConfiguredInShellProfile(shimDir) {
		pathStatus = "PATH configured (restart shell to activate)"
	} else {
		pathStatus = "PATH missing ⚠"
	}
	fmt.Printf(label+"%s · %d commands · %s\n", "Shims:", shimStatus, shimCount, pathStatus)
	// Only show the self-test note when PATH is genuinely missing from config,
	// not when it's just configured but not yet sourced.
	if !pathOK && !isConfiguredInShellProfile(shimDir) && totalEvents > 0 {
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

// isConfiguredInShellProfile checks if the given directory is configured in the
// shell profile file, even if not yet active in the current session's PATH.
// Returns true if the shell config file contains a brewprune PATH export for dir.
func isConfiguredInShellProfile(dir string) bool {
	// Detect the user's shell and determine config file path.
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	shellPath := os.Getenv("SHELL")
	shellName := strings.TrimPrefix(shellPath, "/bin/")
	shellName = strings.TrimPrefix(shellName, "/usr/bin/")
	shellName = strings.TrimPrefix(shellName, "/usr/local/bin/")

	var configPath string
	var searchPattern string

	switch shellName {
	case "zsh":
		configPath = fmt.Sprintf("%s/.zprofile", home)
		searchPattern = fmt.Sprintf("export PATH=%q", dir)
	case "bash":
		configPath = fmt.Sprintf("%s/.bash_profile", home)
		searchPattern = fmt.Sprintf("export PATH=%q", dir)
	case "fish":
		configPath = fmt.Sprintf("%s/.config/fish/conf.d/brewprune.fish", home)
		searchPattern = fmt.Sprintf("fish_add_path %s", dir)
	default:
		configPath = fmt.Sprintf("%s/.profile", home)
		searchPattern = fmt.Sprintf("export PATH=%q", dir)
	}

	// Read the config file and check if it contains the brewprune PATH export.
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Config file doesn't exist or can't be read.
		return false
	}

	content := string(data)
	// Check for the exact quoted path or the unquoted path (both valid).
	return strings.Contains(content, searchPattern) || strings.Contains(content, fmt.Sprintf("export PATH=%s", dir))
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
	if d < 5*time.Second {
		return "just now"
	}
	if d < time.Minute {
		secs := int(d.Seconds())
		if secs == 1 {
			return "1 second ago"
		}
		return fmt.Sprintf("%d seconds ago", secs)
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
