package app

import (
	"fmt"

	"github.com/blackwell-systems/brewprune/internal/output"
	"github.com/blackwell-systems/brewprune/internal/scanner"
	"github.com/blackwell-systems/brewprune/internal/store"
	"github.com/spf13/cobra"
)

var (
	scanRefreshBinaries bool
	scanQuiet           bool

	scanCmd = &cobra.Command{
		Use:   "scan",
		Short: "Scan and index installed Homebrew packages",
		Long: `Scan all installed Homebrew packages and store them in the brewprune database.

This command discovers all installed packages via brew, builds the dependency graph,
and optionally refreshes binary path mappings. The package inventory is cached in
the database for fast access by other commands.

The scan command should be run:
  • After installing brewprune for the first time
  • After installing or removing packages manually with brew
  • Periodically to keep the database in sync with brew`,
		Example: `  # Scan all packages
  brewprune scan

  # Scan without refreshing binary paths
  brewprune scan --refresh-binaries=false

  # Scan quietly (suppress output)
  brewprune scan --quiet`,
		RunE: runScan,
	}
)

func init() {
	scanCmd.Flags().BoolVar(&scanRefreshBinaries, "refresh-binaries", true, "refresh binary path mappings")
	scanCmd.Flags().BoolVar(&scanQuiet, "quiet", false, "suppress output")
}

func runScan(cmd *cobra.Command, args []string) error {
	// Get database path
	dbPath, err := getDBPath()
	if err != nil {
		return fmt.Errorf("failed to get database path: %w", err)
	}

	// Open database
	db, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Create schema if needed
	if err := db.CreateSchema(); err != nil {
		return fmt.Errorf("failed to create database schema: %w", err)
	}

	// Create scanner
	s := scanner.New(db)

	if !scanQuiet {
		fmt.Println("Scanning installed Homebrew packages...")
	}

	// Scan packages
	spinner := output.NewSpinner("Discovering packages...")
	if err := s.ScanPackages(); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to scan packages: %w", err)
	}
	spinner.StopWithMessage("✓ Packages discovered")

	// Build dependency graph
	if !scanQuiet {
		spinner = output.NewSpinner("Building dependency graph...")
	}
	depGraph, err := s.BuildDependencyGraph()
	if err != nil {
		if !scanQuiet {
			spinner.Stop()
		}
		return fmt.Errorf("failed to build dependency graph: %w", err)
	}
	if !scanQuiet {
		spinner.StopWithMessage(fmt.Sprintf("✓ Dependency graph built (%d packages)", len(depGraph)))
	}

	// Refresh binary paths if requested
	if scanRefreshBinaries {
		if !scanQuiet {
			spinner = output.NewSpinner("Refreshing binary paths...")
		}
		if err := s.RefreshBinaryPaths(); err != nil {
			if !scanQuiet {
				spinner.Stop()
			}
			return fmt.Errorf("failed to refresh binary paths: %w", err)
		}
		if !scanQuiet {
			spinner.StopWithMessage("✓ Binary paths refreshed")
		}
	}

	// Get inventory for display
	packages, err := s.GetInventory()
	if err != nil {
		return fmt.Errorf("failed to get inventory: %w", err)
	}

	// Calculate total size
	var totalSize int64
	for _, pkg := range packages {
		totalSize += pkg.SizeBytes
	}

	if !scanQuiet {
		fmt.Println()
		fmt.Printf("Scan complete: %d packages found (%s total)\n", len(packages), formatSize(totalSize))
		fmt.Println()

		// Display package table
		table := output.RenderPackageTable(packages)
		fmt.Print(table)
	}

	return nil
}

// formatSize converts bytes to human-readable size
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
