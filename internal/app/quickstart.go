package app

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/blackwell-systems/brewprune/internal/output"
	"github.com/blackwell-systems/brewprune/internal/shell"
	"github.com/blackwell-systems/brewprune/internal/shim"
	"github.com/blackwell-systems/brewprune/internal/store"
	"github.com/spf13/cobra"
)

var quickstartCmd = &cobra.Command{
	Use:   "quickstart",
	Short: "End-to-end setup workflow for new brewprune installations",
	Long: `Runs the complete brewprune setup workflow in a single command.

Steps performed:
  1. Scan installed Homebrew packages and build shims
  2. Ensure ~/.brewprune/bin is in PATH (writes to shell config if needed)
  3. Start the usage tracking service
  4. Run a self-test to confirm the shim → daemon → database pipeline works

This command is non-interactive and safe to run from a Homebrew post_install hook.`,
	RunE: runQuickstart,
}

func init() {
	RootCmd.AddCommand(quickstartCmd)
}

func runQuickstart(cmd *cobra.Command, args []string) error {
	fmt.Println("Welcome to brewprune! Running end-to-end setup...")
	fmt.Println()

	// Check if database already exists and has packages
	// If so, skip scan to avoid database lock conflicts with running daemon
	dbPath, dbErr := getDBPath()
	var skipScan bool
	if dbErr == nil {
		if db, openErr := store.New(dbPath); openErr == nil {
			packages, pkgErr := db.ListPackages()
			db.Close()
			if pkgErr == nil && len(packages) > 0 {
				skipScan = true
			}
		}
	}

	// ── Step 1: Scan ──────────────────────────────────────────────────────────
	fmt.Println("Step 1/4: Scanning installed Homebrew packages")

	if skipScan {
		fmt.Println("  ✓ Database already populated, skipping scan")
	} else {
		// Run scan quietly to avoid duplicate PATH warnings and verbose table output.
		// We'll print a summary instead of the full 40-row package table.
		originalQuiet := scanQuiet
		scanQuiet = true
		defer func() { scanQuiet = originalQuiet }()

		if err := runScan(cmd, args); err != nil {
			// Check if it's a database lock error
			if strings.Contains(err.Error(), "database is locked") || strings.Contains(err.Error(), "SQLITE_BUSY") {
				fmt.Println("  ⚠ Database is locked (daemon may be running)")
				fmt.Println("  If setup is already complete, this is normal. Check: brewprune status")
				skipScan = true
			} else {
				return fmt.Errorf("scan failed: %w", err)
			}
		}
	}

	// Print a concise summary (if scan was run or DB exists)
	if !skipScan {
		dbPath, dbErr = getDBPath()
		if dbErr == nil {
			db, openErr := store.New(dbPath)
			if openErr == nil {
				defer db.Close()
				packages, pkgErr := db.ListPackages()
				if pkgErr == nil {
					var totalSize int64
					for _, pkg := range packages {
						totalSize += pkg.SizeBytes
					}
					fmt.Printf("  ✓ Scan complete: %d packages, %s\n", len(packages), formatSize(totalSize))
				}
			}
		}
	}
	fmt.Println()

	// ── Step 2: Ensure ~/.brewprune/bin is in PATH ────────────────────────────
	fmt.Println("Step 2/4: Verifying ~/.brewprune/bin is in PATH")
	shimDir, shimDirErr := shim.GetShimDir()
	if shimDirErr != nil {
		fmt.Printf("  ⚠ Could not determine shim directory: %v\n", shimDirErr)
		fmt.Println("  Skipping PATH setup.")
	} else {
		added, configFile, pathErr := shell.EnsurePathEntry(shimDir)
		if pathErr != nil {
			fmt.Printf("  ⚠ Could not update shell config: %v\n", pathErr)
			fmt.Println("  Please add the following line to your shell config manually:")
			fmt.Printf("    export PATH=%q:$PATH\n", shimDir)
		} else if added {
			fmt.Printf("  ✓ Added %s to PATH in %s\n", shimDir, configFile)
			fmt.Println("  Restart your shell (or source the config file) for this to take effect.")
		} else {
			fmt.Printf("  ✓ %s is already in PATH\n", shimDir)
		}
	}
	fmt.Println()

	// ── Step 3: Start the service ─────────────────────────────────────────────
	fmt.Println("Step 3/4: Starting usage tracking service")
	if brewPath, lookErr := exec.LookPath("brew"); lookErr == nil {
		if runtime.GOOS == "linux" {
			// brew services is not reliable on Linux; skip directly to daemon
			fmt.Println("  brew found but using daemon mode (brew services not supported on Linux)")
			if daemonErr := startWatchDaemonFallback(cmd, args); daemonErr != nil {
				if strings.Contains(daemonErr.Error(), "already running") {
					fmt.Println("  ✓ Daemon already running")
				} else {
					fmt.Printf("  ⚠ Could not start daemon: %v\n", daemonErr)
					fmt.Println("  Run 'brewprune watch --daemon' manually after setup.")
				}
			} else {
				fmt.Println("  ✓ Usage tracking daemon started (watch --daemon)")
			}
		} else {
			// macOS: try brew services first
			fmt.Printf("  brew found at %s — running: brew services start brewprune\n", brewPath)
			serviceCmd := exec.Command("brew", "services", "start", "brewprune") //nolint:gosec
			serviceCmd.Stdout = nil
			serviceCmd.Stderr = nil
			if serviceErr := serviceCmd.Run(); serviceErr != nil {
				fmt.Println("  brew services unavailable — using daemon mode")
				if daemonErr := startWatchDaemonFallback(cmd, args); daemonErr != nil {
					if strings.Contains(daemonErr.Error(), "already running") {
						fmt.Println("  ✓ Daemon already running")
					} else {
						fmt.Printf("  ⚠ Could not start daemon: %v\n", daemonErr)
						fmt.Println("  Run 'brewprune watch --daemon' manually after setup.")
					}
				} else {
					fmt.Println("  ✓ Usage tracking daemon started (watch --daemon)")
				}
			} else {
				fmt.Println("  ✓ brewprune service started via brew services")
			}
		}
	} else {
		fmt.Println("  brew not found in PATH — starting: brewprune watch --daemon")
		if daemonErr := startWatchDaemonFallback(cmd, args); daemonErr != nil {
			if strings.Contains(daemonErr.Error(), "already running") {
				fmt.Println("  ✓ Daemon already running")
			} else {
				fmt.Printf("  ⚠ Could not start daemon: %v\n", daemonErr)
				fmt.Println("  Run 'brewprune watch --daemon' manually after setup.")
			}
		} else {
			fmt.Println("  ✓ Usage tracking daemon started")
		}
	}
	fmt.Println()

	// ── Step 4: Self-test ─────────────────────────────────────────────────────
	fmt.Println("Step 4/4: Running self-test (tracking verified)")
	dbPath, dbErr = getDBPath()
	if dbErr != nil {
		fmt.Printf("  ⚠ Could not get database path: %v\n", dbErr)
		fmt.Println("  Run 'brewprune doctor' for diagnostics")
	} else {
		db, openErr := store.New(dbPath)
		if openErr != nil {
			fmt.Printf("  ⚠ Could not open database: %v\n", openErr)
			fmt.Println("  Run 'brewprune doctor' for diagnostics")
		} else {
			defer db.Close()
			spinner := output.NewSpinner("Verifying shim → daemon → database pipeline (up to 35s)...")
			testErr := RunShimTest(db, 35*time.Second)
			if testErr != nil {
				spinner.StopWithMessage(fmt.Sprintf("  ⚠ Self-test did not confirm tracking: %v", testErr))
				fmt.Println("  Run 'brewprune doctor' for diagnostics")
			} else {
				spinner.StopWithMessage("  ✓ Tracking verified — brewprune is working")
			}
		}
	}
	fmt.Println()

	// ── Summary ───────────────────────────────────────────────────────────────
	fmt.Println("Setup complete!")
	fmt.Println()
	fmt.Println("IMPORTANT: Wait 1-2 weeks before acting on recommendations.")
	fmt.Println()
	fmt.Println("What happens next:")
	fmt.Println("  • The daemon runs in the background, tracking Homebrew binary usage")
	fmt.Println("  • After 1-2 weeks, run: brewprune unused --tier safe")
	fmt.Println()
	fmt.Println("Check status anytime: brewprune status")
	fmt.Println("Run diagnostics:      brewprune doctor")
	fmt.Println()
	fmt.Println("Note: If doctor reports 'PATH missing', restart your shell or run:")
	fmt.Println("  source ~/.profile  (or ~/.zshrc / ~/.bashrc depending on your shell)")

	return nil
}

// startWatchDaemonFallback starts the watch daemon using the internal runWatch
// path, mirroring what 'brewprune watch --daemon' does on the CLI.
func startWatchDaemonFallback(cmd *cobra.Command, args []string) error {
	watchDaemon = true
	return runWatch(cmd, args)
}
