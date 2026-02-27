package app

import (
	"fmt"
	"os/exec"
	"time"

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

	// ── Step 1: Scan ──────────────────────────────────────────────────────────
	fmt.Println("Step 1/4: Scanning installed Homebrew packages")
	fmt.Println("Running: brewprune scan")
	fmt.Println()
	if err := runScan(cmd, args); err != nil {
		return fmt.Errorf("scan failed: %w", err)
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
		fmt.Printf("  brew found at %s — running: brew services start brewprune\n", brewPath)
		serviceCmd := exec.Command("brew", "services", "start", "brewprune") //nolint:gosec
		serviceCmd.Stdout = nil
		serviceCmd.Stderr = nil
		if serviceErr := serviceCmd.Run(); serviceErr != nil {
			fmt.Printf("  ⚠ brew services start failed (%v) — falling back to brewprune watch --daemon\n", serviceErr)
			if daemonErr := startWatchDaemonFallback(cmd, args); daemonErr != nil {
				fmt.Printf("  ⚠ Could not start daemon: %v\n", daemonErr)
				fmt.Println("  Run 'brewprune watch --daemon' manually after setup.")
			} else {
				fmt.Println("  ✓ Usage tracking daemon started (watch --daemon)")
			}
		} else {
			fmt.Println("  ✓ brewprune service started via brew services")
		}
	} else {
		fmt.Println("  brew not found in PATH — starting: brewprune watch --daemon")
		if daemonErr := startWatchDaemonFallback(cmd, args); daemonErr != nil {
			fmt.Printf("  ⚠ Could not start daemon: %v\n", daemonErr)
			fmt.Println("  Run 'brewprune watch --daemon' manually after setup.")
		} else {
			fmt.Println("  ✓ Usage tracking daemon started")
		}
	}
	fmt.Println()

	// ── Step 4: Self-test ─────────────────────────────────────────────────────
	fmt.Println("Step 4/4: Running self-test (tracking verified)")
	dbPath, dbErr := getDBPath()
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
			fmt.Println("  Waiting up to 35s for a usage event to appear in the database...")
			testErr := RunShimTest(db, 35*time.Second)
			if testErr != nil {
				fmt.Printf("  ⚠ Self-test did not confirm tracking: %v\n", testErr)
				fmt.Println("  Run 'brewprune doctor' for diagnostics")
			} else {
				fmt.Println("  ✓ Tracking verified — brewprune is working")
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

	return nil
}

// startWatchDaemonFallback starts the watch daemon using the internal runWatch
// path, mirroring what 'brewprune watch --daemon' does on the CLI.
func startWatchDaemonFallback(cmd *cobra.Command, args []string) error {
	watchDaemon = true
	return runWatch(cmd, args)
}
