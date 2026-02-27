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

	// Open database to get statistics
	var packagesTracked int
	var eventsLogged int
	var trackingSince *time.Time
	var lastEvent *UsageEventInfo

	if dbExists {
		st, err := store.New(dbPath)
		if err == nil {
			defer st.Close()

			// Get package count
			packages, err := st.ListPackages()
			if err == nil {
				packagesTracked = len(packages)
			}

			// Get event statistics
			eventsLogged, trackingSince, lastEvent = getUsageStats(st)
		}
	}

	// Display status
	fmt.Println()

	// Daemon status
	if daemonRunning {
		fmt.Printf("Daemon Status:        ✓ Running (PID %d)\n", pid)
	} else {
		fmt.Printf("⚠ Daemon Status:      Not Running\n")
	}

	// Database status
	if dbExists {
		fmt.Printf("Database:             ✓ Found (%s)\n", dbPath)
	} else {
		fmt.Printf("Database:             ✗ Not Found (%s)\n", dbPath)
	}

	// Statistics
	fmt.Printf("Packages Tracked:     %d\n", packagesTracked)
	fmt.Printf("Events Logged:        %s\n", formatNumber(eventsLogged))

	// Tracking duration
	if trackingSince != nil {
		duration := time.Since(*trackingSince)
		fmt.Printf("Tracking Since:       %s\n", formatDuration(duration))
	} else {
		fmt.Printf("Tracking Since:       never\n")
	}

	// Last event
	if lastEvent != nil {
		timeSince := time.Since(lastEvent.Timestamp)
		fmt.Printf("\nLast Event: %s (%s)\n", formatDuration(timeSince), lastEvent.Package)
	}

	// Show warning if daemon not running
	if !daemonRunning {
		fmt.Println()
		fmt.Println("WARNING: The watch daemon is not running. Package usage is not being tracked.")
		fmt.Println()
		fmt.Println("Start tracking:  brewprune watch --daemon")

		logFile, err := getDefaultLogFile()
		if err == nil {
			fmt.Printf("View logs:       tail -f %s\n", logFile)
		}
	}

	fmt.Println()

	return nil
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
