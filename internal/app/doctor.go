package app

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/blackwell-systems/brewprune/internal/shim"
	"github.com/blackwell-systems/brewprune/internal/store"
	"github.com/blackwell-systems/brewprune/internal/watcher"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose common issues and check system health",
	Long: `Runs diagnostic checks on your brewprune installation.

Checks:
  • Database exists and is accessible
  • Daemon is running
  • Usage events are being recorded
  • Recommends next steps`,
	RunE: runDoctor,
}

func init() {
	RootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	fmt.Println("Running brewprune diagnostics...")
	fmt.Println()

	issues := 0

	// Check 1: Database exists
	dbPath, err := getDBPath()
	if err != nil {
		fmt.Println("✗ Database path error:", err)
		issues++
	} else if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Println("✗ Database not found at:", dbPath)
		fmt.Println("  Fix: Run 'brewprune scan' to create database")
		issues++
	} else {
		fmt.Println("✓ Database found:", dbPath)
	}

	// Check 2: Database accessible
	if issues == 0 {
		db, err := store.New(dbPath)
		if err != nil {
			fmt.Println("✗ Cannot open database:", err)
			issues++
		} else {
			defer db.Close()
			fmt.Println("✓ Database is accessible")

			// Check 3: Packages scanned
			packages, err := db.ListPackages()
			if err != nil {
				fmt.Println("✗ Cannot read packages:", err)
				issues++
			} else if len(packages) == 0 {
				fmt.Println("✗ No packages in database")
				fmt.Println("  Fix: Run 'brewprune scan'")
				issues++
			} else {
				fmt.Printf("✓ %d packages tracked\n", len(packages))
			}

			// Check 4: Events recorded
			var eventCount int
			row := db.DB().QueryRow("SELECT COUNT(*) FROM usage_events")
			if err := row.Scan(&eventCount); err != nil {
				fmt.Println("✗ Cannot read events:", err)
				issues++
			} else if eventCount == 0 {
				fmt.Println("⚠ No usage events recorded yet")
				fmt.Println("  This is normal for new installations")
			} else {
				fmt.Printf("✓ %d usage events recorded\n", eventCount)
			}
		}
	}

	// Check 5: Daemon running
	pidFile, err := getDefaultPIDFile()
	if err != nil {
		fmt.Println("✗ Failed to get PID file path:", err)
		issues++
	} else if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		fmt.Println("✗ Daemon not running (no PID file)")
		fmt.Println("  Fix: Run 'brewprune watch --daemon'")
		issues++
	} else {
		running, err := watcher.IsDaemonRunning(pidFile)
		if err != nil {
			fmt.Println("✗ Failed to check daemon status:", err)
			issues++
		} else if !running {
			fmt.Println("✗ Daemon not running (stale PID file)")
			fmt.Println("  Fix: Run 'brewprune watch --daemon'")
			issues++
		} else {
			// Get PID if daemon is running
			pidData, err := os.ReadFile(pidFile)
			if err == nil {
				pidStr := strings.TrimSpace(string(pidData))
				pid, _ := strconv.Atoi(pidStr)
				fmt.Printf("✓ Daemon running (PID %d)\n", pid)
			} else {
				fmt.Println("✓ Daemon running")
			}
		}
	}

	// Check 6: Shim binary exists
	shimDir, shimDirErr := shim.GetShimDir()
	if shimDirErr != nil {
		fmt.Println("✗ Cannot determine shim directory:", shimDirErr)
		issues++
	} else {
		shimBin := shimDir + "/brewprune-shim"
		if _, err := os.Stat(shimBin); os.IsNotExist(err) {
			fmt.Println("✗ Shim binary not found — usage tracking disabled")
			fmt.Println("  Fix: Run 'brewprune scan' to build it")
			issues++
		} else {
			fmt.Println("✓ Shim binary found:", shimBin)

			// Check 7: Shim directory in PATH
			if ok, reason := shim.IsShimSetup(); !ok {
				fmt.Println("✗ Shim directory not in PATH — executions won't be intercepted")
				fmt.Printf("  Fix: %s\n", reason)
				issues++
			} else {
				// Count symlinks in shim dir
				entries, err := os.ReadDir(shimDir)
				if err == nil {
					symlinkCount := 0
					for _, e := range entries {
						if info, err := e.Info(); err == nil && info.Mode()&os.ModeSymlink != 0 {
							symlinkCount++
						}
					}
					fmt.Printf("✓ PATH shims active (%d commands intercepted)\n", symlinkCount)
				} else {
					fmt.Println("✓ Shim directory in PATH")
				}
			}
		}
	}

	fmt.Println()
	if issues == 0 {
		fmt.Println("✓ All checks passed!")
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  • Wait 1-2 weeks for usage data")
		fmt.Println("  • Check status: brewprune status")
		fmt.Println("  • View recommendations: brewprune unused --tier safe")
	} else {
		fmt.Printf("Found %d issue(s). Follow the fixes above.\n", issues)
		return fmt.Errorf("diagnostics failed")
	}

	return nil
}
