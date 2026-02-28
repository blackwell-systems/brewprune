package app

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/blackwell-systems/brewprune/internal/output"
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

	// [DOCTOR-1] Track critical vs warning-level issues separately so we can
	// exit with code 2 for warnings-only (no double-print) and exit 1 for
	// critical failures.
	criticalIssues := 0
	warningIssues := 0

	// Check 1: Database exists
	resolvedDBPath, err := getDBPath()
	if err != nil {
		fmt.Println("✗ Database path error:", err)
		criticalIssues++
	} else if _, err := os.Stat(resolvedDBPath); os.IsNotExist(err) {
		fmt.Println("✗ Database not found at:", resolvedDBPath)
		fmt.Println("  Action: Run 'brewprune scan' to create database")
		criticalIssues++
	} else {
		fmt.Println("✓ Database found:", resolvedDBPath)
	}

	// Check 2: Database accessible
	if criticalIssues == 0 {
		db, err := store.New(resolvedDBPath)
		if err != nil {
			fmt.Println("✗ Cannot open database:", err)
			criticalIssues++
		} else {
			defer db.Close()
			fmt.Println("✓ Database is accessible")

			// Check 3: Packages scanned
			packages, err := db.ListPackages()
			if err != nil {
				fmt.Println("✗ Cannot read packages:", err)
				criticalIssues++
			} else if len(packages) == 0 {
				fmt.Println("✗ No packages in database")
				fmt.Println("  Action: Run 'brewprune scan'")
				criticalIssues++
			} else {
				fmt.Printf("✓ %d packages tracked\n", len(packages))
			}

			// Check 4: Events recorded — warning only
			var eventCount int
			row := db.DB().QueryRow("SELECT COUNT(*) FROM usage_events")
			if err := row.Scan(&eventCount); err != nil {
				fmt.Println("⚠ Cannot read events:", err)
				warningIssues++
			} else if eventCount == 0 {
				fmt.Println("⚠ No usage events recorded yet")
				fmt.Println("  This is normal for new installations")
				warningIssues++
			} else {
				fmt.Printf("✓ %d usage events recorded\n", eventCount)
			}
		}
	}

	// Check 5: Daemon running — warning only
	pidFile, err := getDefaultPIDFile()
	if err != nil {
		fmt.Println("⚠ Failed to get PID file path:", err)
		warningIssues++
	} else if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		fmt.Println("⚠ Daemon not running (no PID file)")
		fmt.Println("  Action: Run 'brewprune watch --daemon'")
		warningIssues++
	} else {
		running, err := watcher.IsDaemonRunning(pidFile)
		if err != nil {
			fmt.Println("⚠ Failed to check daemon status:", err)
			warningIssues++
		} else if !running {
			fmt.Println("⚠ Daemon not running (stale PID file)")
			fmt.Println("  Action: Run 'brewprune watch --daemon'")
			warningIssues++
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

	// Check 6: Shim binary exists — critical
	shimDir, shimDirErr := shim.GetShimDir()
	if shimDirErr != nil {
		fmt.Println("✗ Cannot determine shim directory:", shimDirErr)
		criticalIssues++
	} else {
		shimBin := shimDir + "/brewprune-shim"
		if _, err := os.Stat(shimBin); os.IsNotExist(err) {
			fmt.Println("✗ Shim binary not found — usage tracking disabled")
			fmt.Println("  Action: Run 'brewprune scan' to build it")
			criticalIssues++
		} else {
			fmt.Println("✓ Shim binary found:", shimBin)

			// Check 7: Shim directory in PATH — warning only
			if ok, reason := shim.IsShimSetup(); !ok {
				fmt.Println("⚠ Shim directory not in PATH — executions won't be intercepted")
				fmt.Printf("  Action: %s\n", reason)
				warningIssues++
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

	// Check 8: End-to-end pipeline test (only when no critical issues)
	if criticalIssues == 0 {
		pipelineStart := time.Now()
		db2, dbErr := store.New(resolvedDBPath)
		if dbErr != nil {
			fmt.Println("✗ Pipeline test: cannot open database:", dbErr)
			criticalIssues++
		} else {
			defer db2.Close()
			spinner := output.NewSpinner("Running pipeline test...")
			pipelineErr := RunShimTest(db2, 35*time.Second)
			pipelineElapsed := time.Since(pipelineStart).Round(time.Millisecond)
			if pipelineErr != nil {
				spinner.StopWithMessage(fmt.Sprintf("✗ Pipeline test: fail (%v)", pipelineElapsed))
				fmt.Printf("  %v\n", pipelineErr)
				fmt.Println("  Action: Run 'brewprune scan' to rebuild shims and restart the daemon")
				criticalIssues++
			} else {
				spinner.StopWithMessage(fmt.Sprintf("✓ Pipeline test: pass (%v)", pipelineElapsed))
			}
		}
	}

	fmt.Println()
	if criticalIssues == 0 && warningIssues == 0 {
		fmt.Println("✓ All checks passed!")
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  • Wait 1-2 weeks for usage data")
		fmt.Println("  • Check status: brewprune status")
		fmt.Println("  • View recommendations: brewprune unused --tier safe")
		return nil
	}

	if criticalIssues > 0 {
		fmt.Printf("Found %d critical issue(s) and %d warning(s).\n", criticalIssues, warningIssues)
		return fmt.Errorf("diagnostics failed")
	}

	// [DOCTOR-2] Warning-only path: use os.Exit(2) so main.go's error handler
	// is never reached, eliminating the double-print of the error message.
	fmt.Printf("Found %d warning(s). System is functional but not fully configured.\n", warningIssues)
	os.Exit(2)
	return nil // unreachable; satisfies compiler
}
