package app

import (
	"fmt"

	"github.com/blackwell-systems/brewprune/internal/output"
	"github.com/blackwell-systems/brewprune/internal/scanner"
	"github.com/blackwell-systems/brewprune/internal/shim"
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

	// Build the shim binary and create per-command symlinks so the watch
	// daemon can intercept and log executions via ~/.brewprune/usage.log.
	var shimCount int
	if scanRefreshBinaries {
		if !scanQuiet {
			spinner = output.NewSpinner("Building shim binary...")
		}
		if err := shim.BuildShimBinary(); err != nil {
			if !scanQuiet {
				spinner.Stop()
				fmt.Printf("⚠ Could not build shim binary (usage tracking unavailable): %v\n", err)
			}
		} else {
			if !scanQuiet {
				spinner.StopWithMessage("✓ Shim binary built")
				spinner = output.NewSpinner("Generating PATH shims...")
			}

			var allBinaries []string
			for _, pkg := range packages {
				allBinaries = append(allBinaries, pkg.BinaryPaths...)
			}

			var shimErr error
			shimCount, shimErr = shim.GenerateShims(allBinaries)
			if !scanQuiet {
				if shimErr != nil {
					spinner.Stop()
					fmt.Printf("⚠ Shim generation incomplete: %v\n", shimErr)
				} else {
					spinner.StopWithMessage(fmt.Sprintf("✓ %d command shims created", shimCount))
				}
			}
		}
	}

	if !scanQuiet {
		fmt.Println()
		fmt.Printf("Scan complete: %d packages found (%s total)\n", len(packages), formatSize(totalSize))

		// Show PATH setup instructions if shims were created but shimDir not in PATH.
		if shimCount > 0 {
			if ok, reason := shim.IsShimSetup(); !ok {
				fmt.Printf("\n⚠ Usage tracking requires one more step:\n  %s\n", reason)
				fmt.Println("  Then restart your shell and run: brewprune watch --daemon")
			} else {
				fmt.Println("\n⚠️  NEXT STEP: Start usage tracking with 'brewprune watch --daemon'")
				fmt.Println("   Wait 1-2 weeks for meaningful recommendations.")
			}
		} else {
			fmt.Println("\n⚠️  NEXT STEP: Start usage tracking with 'brewprune watch --daemon'")
			fmt.Println("   Wait 1-2 weeks for meaningful recommendations.")
		}
		fmt.Println()

		// Display package table
		table := output.RenderPackageTable(packages)
		fmt.Print(table)
	}

	return nil
}
