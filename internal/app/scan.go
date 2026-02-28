package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/brewprune/internal/brew"
	"github.com/blackwell-systems/brewprune/internal/config"
	"github.com/blackwell-systems/brewprune/internal/output"
	"github.com/blackwell-systems/brewprune/internal/scanner"
	"github.com/blackwell-systems/brewprune/internal/shim"
	"github.com/blackwell-systems/brewprune/internal/store"
	"github.com/blackwell-systems/brewprune/internal/watcher"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var (
	scanRefreshBinaries bool
	scanQuiet           bool
	scanRefreshShims    bool

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

  # Fast path: refresh shims only
  brewprune scan --refresh-shims

  # Scan quietly (suppress output)
  brewprune scan --quiet`,
		RunE: runScan,
	}
)

func init() {
	scanCmd.Flags().BoolVar(&scanRefreshBinaries, "refresh-binaries", true, "refresh binary path mappings")
	scanCmd.Flags().BoolVar(&scanQuiet, "quiet", false, "suppress output")
	scanCmd.Flags().BoolVar(&scanRefreshShims, "refresh-shims", false, "fast path: diff and update shims only, skip full dep tree rebuild")
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

	// Fast path: refresh shims only without a full dep tree rebuild.
	if scanRefreshShims {
		return runRefreshShims(db)
	}

	// Check if this is a re-scan by getting existing packages
	existingPackages, err := db.ListPackages()
	if err != nil {
		existingPackages = nil // Treat error as empty (first scan)
	}
	isFirstScan := len(existingPackages) == 0

	// Create scanner
	s := scanner.New(db)

	// Scan packages quietly to check for changes first
	isTTY := isatty.IsTerminal(os.Stdout.Fd())
	var spinner *output.Spinner

	if !scanQuiet && !isFirstScan {
		// On re-scan, be quiet during discovery to check for changes
		if isTTY {
			spinner = output.NewSpinner("Checking for changes...")
			spinner.Start()
		}
	} else if !scanQuiet {
		// On first scan, show verbose output
		fmt.Println("Scanning installed Homebrew packages...")
		if isTTY {
			spinner = output.NewSpinner("Discovering packages...")
			spinner.Start()
		} else {
			fmt.Println("Discovering packages...")
		}
	}

	if err := s.ScanPackages(); err != nil {
		if !scanQuiet && isTTY {
			spinner.Stop()
		}
		return fmt.Errorf("failed to scan packages: %w", err)
	}

	// Get new packages and compare
	newPackages, err := s.GetInventory()
	if err != nil {
		if !scanQuiet && isTTY {
			spinner.Stop()
		}
		return fmt.Errorf("failed to get inventory: %w", err)
	}

	// Detect changes: compare package names
	hasChanges := detectChanges(existingPackages, newPackages)

	// If no changes on re-scan, show terse output and exit
	if !isFirstScan && !hasChanges && !scanQuiet {
		if isTTY {
			spinner.StopWithMessage(fmt.Sprintf("✓ Database up to date (%d packages, 0 changes)", len(newPackages)))
		} else {
			fmt.Printf("✓ Database up to date (%d packages, 0 changes)\n", len(newPackages))
		}
		return nil
	}

	// Continue with verbose output for first scan or when changes detected
	if !scanQuiet && isTTY {
		if isFirstScan {
			spinner.StopWithMessage("✓ Packages discovered")
		} else {
			spinner.StopWithMessage("✓ Changes detected")
			fmt.Println("Scanning installed Homebrew packages...")
		}
	}

	// Build dependency graph
	if !scanQuiet {
		if isTTY {
			spinner = output.NewSpinner("Building dependency graph...")
			spinner.Start()
		} else {
			fmt.Println("Building dependency graph...")
		}
	}
	depGraph, err := s.BuildDependencyGraph()
	if err != nil {
		if !scanQuiet && isTTY {
			spinner.Stop()
		}
		return fmt.Errorf("failed to build dependency graph: %w", err)
	}
	if !scanQuiet {
		if isTTY {
			spinner.StopWithMessage(fmt.Sprintf("✓ Dependency graph built (%d packages)", len(depGraph)))
		}
	}

	// Refresh binary paths if requested
	if scanRefreshBinaries {
		if !scanQuiet {
			if isTTY {
				spinner = output.NewSpinner("Refreshing binary paths...")
				spinner.Start()
			} else {
				fmt.Println("Refreshing binary paths...")
			}
		}
		if err := s.RefreshBinaryPaths(); err != nil {
			if !scanQuiet && isTTY {
				spinner.Stop()
			}
			return fmt.Errorf("failed to refresh binary paths: %w", err)
		}
		if !scanQuiet && isTTY {
			spinner.StopWithMessage("✓ Binary paths refreshed")
		}
	}

	// Re-fetch inventory after building dep graph and refreshing binaries
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
			if isTTY {
				spinner = output.NewSpinner("Building shim binary...")
				spinner.Start()
			} else {
				fmt.Println("Building shim binary...")
			}
		}
		if err := shim.BuildShimBinary(); err != nil {
			if !scanQuiet {
				if isTTY {
					spinner.Stop()
				}
				fmt.Printf("⚠ Could not build shim binary (usage tracking unavailable): %v\n", err)
			}
		} else {
			if !scanQuiet {
				if isTTY {
					spinner.StopWithMessage("✓ Shim binary built")
					spinner = output.NewSpinner("Generating PATH shims...")
					spinner.Start()
				} else {
					fmt.Println("Generating PATH shims...")
				}
			}

			var allBinaries []string
			for _, pkg := range packages {
				allBinaries = append(allBinaries, pkg.BinaryPaths...)
			}

			var shimErr error
			shimCount, shimErr = shim.GenerateShims(allBinaries)
			if shimErr == nil {
				// Generate alias shims from ~/.brewprune/aliases.
				aliasCount, aliasErr := generateAliasShims(db)
				if aliasErr != nil && !scanQuiet {
					fmt.Printf("⚠ Alias shim generation incomplete: %v\n", aliasErr)
				} else {
					shimCount += aliasCount
				}
			}
			if !scanQuiet {
				if shimErr != nil {
					if isTTY {
						spinner.Stop()
					}
					fmt.Printf("⚠ Shim generation incomplete: %v\n", shimErr)
				} else {
					shimDir, shimDirErr := shim.GetShimDir()
					var shimMsg string
					if shimCount == 0 {
						existing := 0
						if shimDirErr == nil {
							existing = countSymlinks(shimDir)
						}
						shimMsg = fmt.Sprintf("✓ %d shims up to date (0 new)", existing)
					} else {
						shimMsg = fmt.Sprintf("✓ %d command shims created", shimCount)
					}
					if isTTY {
						spinner.StopWithMessage(shimMsg)
					} else {
						fmt.Println(shimMsg)
					}
				}
			}
		}
	}

	if !scanQuiet {
		fmt.Println()
		fmt.Printf("Scan complete: %d packages found (%s total)\n", len(packages), formatSize(totalSize))

		// Show next-step guidance based on daemon state.
		pidFile, pidErr := getDefaultPIDFile()
		daemonAlreadyRunning := false
		if pidErr == nil {
			if running, runErr := watcher.IsDaemonRunning(pidFile); runErr == nil && running {
				daemonAlreadyRunning = true
			}
		}

		if shimCount > 0 {
			if ok, reason := shim.IsShimSetup(); !ok {
				fmt.Printf("\n⚠ Usage tracking requires one more step:\n  %s\n", reason)
				fmt.Println("  Then restart your shell and run: brewprune watch --daemon")
			} else if daemonAlreadyRunning {
				fmt.Println("\n✓ Daemon is running — usage tracking is active.")
			} else {
				fmt.Println("\n⚠ NEXT STEP: Start usage tracking with 'brewprune watch --daemon'")
				fmt.Println("   Wait 1-2 weeks for meaningful recommendations.")
			}
		} else {
			if daemonAlreadyRunning {
				fmt.Println("\n✓ Daemon is running — usage tracking is active.")
			} else {
				fmt.Println("\n⚠ NEXT STEP: Start usage tracking with 'brewprune watch --daemon'")
				fmt.Println("   Wait 1-2 weeks for meaningful recommendations.")
			}
		}
		fmt.Println()

		// Display package table
		table := output.RenderPackageTable(packages)
		fmt.Print(table)
	}

	return nil
}

// runRefreshShims implements the --refresh-shims fast path.
//
// It reads the current binary list from the DB (no brew invocation or dep tree
// rebuild), calls shim.RefreshShims to add/remove symlinks for changed packages,
// and optionally calls shim.BuildShimBinary + shim.WriteShimVersion when the
// shim binary itself needs to be (re)installed. This is the path used by the
// Homebrew formula's post_install hook after an upgrade.
func runRefreshShims(db *store.Store) error {
	// Load all packages stored in the DB.
	packages, err := db.ListPackages()
	if err != nil {
		return fmt.Errorf("failed to list packages from database: %w", err)
	}

	// Collect every binary path stored across all packages.
	var allBinaries []string
	for _, pkg := range packages {
		allBinaries = append(allBinaries, pkg.BinaryPaths...)
	}

	// Ensure the shim binary exists. If it is missing (e.g. first run after
	// upgrade) build it now so that RefreshShims can create valid symlinks.
	shimBinaryMissing := false
	shimDir, dirErr := shim.GetShimDir()
	if dirErr == nil {
		shimBin := filepath.Join(shimDir, "brewprune-shim")
		if _, statErr := os.Stat(shimBin); os.IsNotExist(statErr) {
			shimBinaryMissing = true
		}
	}

	var version string
	if shimBinaryMissing {
		if err := shim.BuildShimBinary(); err != nil {
			if !scanQuiet {
				fmt.Printf("warning: could not build shim binary: %v\n", err)
			}
		} else {
			version = "refreshed"
		}
	}

	// Perform the incremental diff — only adds/removes symlinks for changes.
	added, removed, err := shim.RefreshShims(allBinaries)
	if err != nil {
		return fmt.Errorf("failed to refresh shims: %w", err)
	}

	// Write shim version marker when a new binary was built.
	if version != "" {
		if err := shim.WriteShimVersion(version); err != nil && !scanQuiet {
			fmt.Printf("warning: could not write shim version: %v\n", err)
		}
	}

	if !scanQuiet {
		fmt.Printf("Refreshed shims: +%d added, -%d removed\n", added, removed)
	}

	return nil
}

// generateAliasShims reads the XDG config aliases file, creates a shim symlink
// for each declared alias, and augments the target package's BinaryPaths in the
// database so the shim processor can resolve the alias name to the canonical
// package. Returns the number of new symlinks created.
func generateAliasShims(db *store.Store) (int, error) {
	cfgDir, err := config.Dir()
	if err != nil {
		return 0, fmt.Errorf("cannot determine config directory: %w", err)
	}

	aliasCfg, err := config.LoadAliases(cfgDir)
	if err != nil {
		return 0, fmt.Errorf("failed to load alias config: %w", err)
	}

	if len(aliasCfg.Aliases) == 0 {
		return 0, nil
	}

	shimDir, err := shim.GetShimDir()
	if err != nil {
		return 0, fmt.Errorf("cannot get shim dir: %w", err)
	}

	shimBinary := filepath.Join(shimDir, "brewprune-shim")
	if _, err := os.Stat(shimBinary); os.IsNotExist(err) {
		return 0, fmt.Errorf(
			"shim binary not found at %s; run 'brewprune scan' first to build it",
			shimBinary,
		)
	}

	count := 0
	for alias, pkgName := range aliasCfg.Aliases {
		symlinkPath := filepath.Join(shimDir, alias)

		// Skip if already correctly shimmed.
		if existing, err := os.Readlink(symlinkPath); err == nil && existing == shimBinary {
			// Ensure the alias path is registered in the package's BinaryPaths.
			registerAliasBinaryPath(db, alias, pkgName)
			continue
		}

		// Remove stale symlink or file if present.
		os.Remove(symlinkPath)

		if err := os.Symlink(shimBinary, symlinkPath); err != nil {
			return count, fmt.Errorf("failed to create alias shim for %s: %w", alias, err)
		}
		count++

		// Register the alias in the package's BinaryPaths so the shim processor
		// can resolve the alias basename to the canonical package name.
		registerAliasBinaryPath(db, alias, pkgName)
	}

	return count, nil
}

// registerAliasBinaryPath adds a virtual binary path entry (using the Homebrew
// opt prefix) for the given alias to the named package in the database. This
// ensures the shim processor's basename lookup resolves alias → package.
// Errors are silently ignored: alias tracking degrades gracefully if the
// target package is not yet scanned.
func registerAliasBinaryPath(db *store.Store, alias, pkgName string) {
	pkg, err := db.GetPackage(pkgName)
	if err != nil {
		return // Package not scanned yet — skip silently.
	}

	// Virtual path used only for basename resolution by the shim processor.
	virtualPath := filepath.Join("/opt/homebrew/bin", alias)

	// Avoid adding duplicates.
	for _, p := range pkg.BinaryPaths {
		if p == virtualPath {
			return
		}
	}

	pkg.BinaryPaths = append(pkg.BinaryPaths, virtualPath)
	db.InsertPackage(pkg) //nolint:errcheck — best-effort
}

// detectChanges compares two package lists and returns true if there are any
// differences (added, removed, or changed packages).
func detectChanges(oldPkgs, newPkgs []*brew.Package) bool {
	// Different package count means changes
	if len(oldPkgs) != len(newPkgs) {
		return true
	}

	// Build a map of old packages by name for quick lookup
	oldMap := make(map[string]*brew.Package)
	for _, pkg := range oldPkgs {
		oldMap[pkg.Name] = pkg
	}

	// Check if any new packages differ from old ones
	for _, newPkg := range newPkgs {
		oldPkg, exists := oldMap[newPkg.Name]
		if !exists {
			// New package added
			return true
		}

		// Check if version changed
		if oldPkg.Version != newPkg.Version {
			return true
		}

		// Check if binary count changed (indicates binary path refresh needed)
		if len(oldPkg.BinaryPaths) != len(newPkg.BinaryPaths) {
			return true
		}
	}

	return false
}
