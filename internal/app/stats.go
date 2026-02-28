package app

import (
	"fmt"
	"os"
	"time"

	isatty "github.com/mattn/go-isatty"

	"github.com/blackwell-systems/brewprune/internal/analyzer"
	"github.com/blackwell-systems/brewprune/internal/output"
	"github.com/blackwell-systems/brewprune/internal/store"
	"github.com/spf13/cobra"
)

var (
	statsDays    int
	statsPackage string
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show usage statistics for packages",
	Long: `Display usage statistics and trends for installed packages.

Without flags, shows usage trends for all packages in the last 30 days.
Use --package to view detailed statistics for a specific package.
Use --days to adjust the time window for analysis.

Usage frequency is classified as:
  - daily: Used in last 7 days with high frequency
  - weekly: Used in last 30 days
  - monthly: Used in last 90 days
  - never: No recorded usage`,
	Example: `  # Show usage trends for all packages (last 30 days)
  brewprune stats

  # Show usage trends for last 90 days
  brewprune stats --days 90

  # Show detailed stats for a specific package
  brewprune stats --package git

  # Show recent activity (last 7 days)
  brewprune stats --days 7`,
	RunE: runStats,
}

func init() {
	statsCmd.Flags().IntVar(&statsDays, "days", 30, "Time window in days")
	statsCmd.Flags().StringVar(&statsPackage, "package", "", "Show stats for specific package")

	// Register with root command
	RootCmd.AddCommand(statsCmd)
}

func runStats(cmd *cobra.Command, args []string) error {
	// Validate flags
	if statsDays <= 0 {
		return fmt.Errorf("invalid days: %d (must be positive)", statsDays)
	}

	// Get database path
	dbPath, err := getDBPath()
	if err != nil {
		return err
	}

	// Open store
	st, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer st.Close()

	// Create analyzer
	a := analyzer.New(st)

	// Check if specific package requested
	if statsPackage != "" {
		return showPackageStats(a, statsPackage)
	}

	// Show trends for all packages
	return showUsageTrends(a, statsDays)
}

// showPackageStats displays detailed statistics for a single package.
// [STATS-2] Applies minimal styling: bold package name header, color-coded
// frequency value. Colors are guarded by isatty.IsTerminal so non-TTY output
// (pipes, CI) stays plain.
func showPackageStats(a *analyzer.Analyzer, pkg string) error {
	stats, err := a.GetUsageStats(pkg)
	if err != nil {
		return fmt.Errorf("failed to get stats for %s: %w", pkg, err)
	}

	// Inline ANSI constants — no shared file dependency.
	const (
		ansiReset  = "\033[0m"
		ansiBold   = "\033[1m"
		ansiGreen  = "\033[32m"
		ansiYellow = "\033[33m"
		ansiRed    = "\033[31m"
		ansiGray   = "\033[90m"
	)

	useColors := isatty.IsTerminal(os.Stdout.Fd())

	bold := func(s string) string {
		if useColors {
			return ansiBold + s + ansiReset
		}
		return s
	}
	colorFreq := func(freq string) string {
		if !useColors {
			return freq
		}
		switch freq {
		case "daily":
			return ansiGreen + freq + ansiReset
		case "weekly":
			return ansiYellow + freq + ansiReset
		case "monthly", "rarely":
			return ansiRed + freq + ansiReset
		case "never":
			return ansiGray + freq + ansiReset
		default:
			return freq
		}
	}

	fmt.Printf("Package: %s\n", bold(stats.Package))
	fmt.Printf("Total Uses: %d\n", stats.TotalUses)

	if stats.LastUsed != nil {
		fmt.Printf("Last Used: %s\n", formatTime(*stats.LastUsed))
		fmt.Printf("Days Since: %d\n", stats.DaysSince)
	} else {
		fmt.Printf("Last Used: never\n")
		fmt.Printf("Days Since: N/A\n")
	}

	fmt.Printf("First Seen: %s\n", formatTime(stats.FirstSeen))
	fmt.Printf("Frequency: %s\n", colorFreq(stats.Frequency))

	return nil
}

// showUsageTrends displays usage trends for all packages.
// NOTE: RenderUsageTable is expected to sort by TotalRuns desc + LastUsed desc
// as a secondary sort. Agent C (Wave 2) will add the secondary sort by LastUsed
// to output/table.go.
func showUsageTrends(a *analyzer.Analyzer, days int) error {
	trends, err := a.GetUsageTrends(days)
	if err != nil {
		return fmt.Errorf("failed to get usage trends: %w", err)
	}

	if len(trends) == 0 {
		fmt.Println("No usage data found. Run 'brewprune watch' to collect usage data.")
		return nil
	}

	// Convert to output format
	outputStats := make(map[string]output.UsageStats)
	usedCount := 0

	for pkg, s := range trends {
		lastUsed := time.Time{}
		if s.LastUsed != nil {
			lastUsed = *s.LastUsed
			// Count packages used within the time window
			if s.DaysSince >= 0 && s.DaysSince <= days {
				usedCount++
			}
		}

		outputStats[pkg] = output.UsageStats{
			TotalRuns: s.TotalUses,
			LastUsed:  lastUsed,
			Frequency: s.Frequency,
			Trend:     "stable", // Default trend
		}
	}

	// Render table
	table := output.RenderUsageTable(outputStats)
	fmt.Print(table)

	// Show summary
	fmt.Printf("\nSummary: %d packages used in last %d days (out of %d total)\n",
		usedCount, days, len(trends))

	return nil
}

// formatTime formats a time for display.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.Format("2006-01-02 15:04:05")
}
