package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/blackwell-systems/brewprune/internal/config"
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

// isColorEnabled returns true when ANSI color output is appropriate:
// NO_COLOR must be unset and stdout must be a character device (TTY).
func isColorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// colorize wraps text with an ANSI escape code when color is enabled.
// code should be the SGR parameter string, e.g. "32" for green.
func colorize(code, text string) string {
	if !isColorEnabled() {
		return text
	}
	return "\033[" + code + "m" + text + "\033[0m"
}

// detectShellConfig returns the path to the user's shell startup file
// based on the SHELL environment variable.
func detectShellConfig() string {
	shell := os.Getenv("SHELL")
	switch {
	case strings.Contains(shell, "zsh"):
		return "~/.zprofile"
	case strings.Contains(shell, "bash"):
		return "~/.bash_profile"
	default:
		return "~/.profile"
	}
}

func runDoctor(cmd *cobra.Command, args []string) error {
	fmt.Println("Running brewprune diagnostics...")
	fmt.Println()

	// [DOCTOR-1] Track critical vs warning-level issues separately so we can
	// exit with code 0 for warnings-only and exit 1 for critical failures.
	criticalIssues := 0
	warningIssues := 0

	// Check 1: Database exists
	resolvedDBPath, err := getDBPath()
	if err != nil {
		fmt.Println(colorize("31", "✗") + " Database path error: " + fmt.Sprint(err))
		criticalIssues++
	} else if _, err := os.Stat(resolvedDBPath); os.IsNotExist(err) {
		fmt.Println(colorize("31", "✗") + " Database not found at: " + resolvedDBPath)
		fmt.Println("  Action: Run 'brewprune scan' to create database")
		criticalIssues++
	} else {
		fmt.Println(colorize("32", "✓") + " Database found: " + resolvedDBPath)
	}

	// Check 2: Database accessible
	var totalUsageEvents int
	if criticalIssues == 0 {
		db, err := store.New(resolvedDBPath)
		if err != nil {
			fmt.Println(colorize("31", "✗") + " Cannot open database: " + fmt.Sprint(err))
			criticalIssues++
		} else {
			defer db.Close()
			fmt.Println(colorize("32", "✓") + " Database is accessible")

			// Check 3: Packages scanned
			packages, err := db.ListPackages()
			if err != nil {
				fmt.Println(colorize("31", "✗") + " Cannot read packages: " + fmt.Sprint(err))
				criticalIssues++
			} else if len(packages) == 0 {
				fmt.Println(colorize("31", "✗") + " No packages in database")
				fmt.Println("  Action: Run 'brewprune scan'")
				criticalIssues++
			} else {
				fmt.Println(colorize("32", "✓") + fmt.Sprintf(" %d packages tracked", len(packages)))
			}

			// Check 4: Events recorded — warning only
			row := db.DB().QueryRow("SELECT COUNT(*) FROM usage_events")
			if err := row.Scan(&totalUsageEvents); err != nil {
				fmt.Println(colorize("33", "⚠") + " Cannot read events: " + fmt.Sprint(err))
				warningIssues++
			} else if totalUsageEvents == 0 {
				fmt.Println(colorize("33", "⚠") + " No usage events recorded yet")
				fmt.Println("  This is normal for new installations")
				warningIssues++
			} else {
				fmt.Println(colorize("32", "✓") + fmt.Sprintf(" %d usage events recorded", totalUsageEvents))
			}
		}
	}

	// Check 5: Daemon running — warning only
	// Track daemon status separately so we can skip/shorten pipeline test if not running.
	daemonRunning := false
	pidFile, err := getDefaultPIDFile()
	if err != nil {
		fmt.Println(colorize("33", "⚠") + " Failed to get PID file path: " + fmt.Sprint(err))
		warningIssues++
	} else if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		fmt.Println(colorize("33", "⚠") + " Daemon not running (no PID file)")
		fmt.Println("  Action: Run 'brewprune watch --daemon'")
		warningIssues++
	} else {
		running, err := watcher.IsDaemonRunning(pidFile)
		if err != nil {
			fmt.Println(colorize("33", "⚠") + " Failed to check daemon status: " + fmt.Sprint(err))
			warningIssues++
		} else if !running {
			fmt.Println(colorize("33", "⚠") + " Daemon not running (stale PID file)")
			fmt.Println("  Action: Run 'brewprune watch --daemon'")
			warningIssues++
		} else {
			daemonRunning = true
			// Get PID if daemon is running
			pidData, err := os.ReadFile(pidFile)
			if err == nil {
				pidStr := strings.TrimSpace(string(pidData))
				pid, _ := strconv.Atoi(pidStr)
				fmt.Println(colorize("32", "✓") + fmt.Sprintf(" Daemon running (PID %d)", pid))
			} else {
				fmt.Println(colorize("32", "✓") + " Daemon running")
			}
		}
	}

	// Check 6: Shim binary exists — critical
	shimDir, shimDirErr := shim.GetShimDir()
	if shimDirErr != nil {
		fmt.Println(colorize("31", "✗") + " Cannot determine shim directory: " + fmt.Sprint(shimDirErr))
		criticalIssues++
	} else {
		shimBin := shimDir + "/brewprune-shim"
		if _, err := os.Stat(shimBin); os.IsNotExist(err) {
			fmt.Println(colorize("31", "✗") + " Shim binary not found — usage tracking disabled")
			fmt.Println("  Action: Run 'brewprune scan' to build it")
			criticalIssues++
		} else {
			fmt.Println(colorize("32", "✓") + " Shim binary found: " + shimBin)

			// Check 7: Shim directory in PATH — warning only
			// Use three-state PATH messaging:
			// 1. PATH active: shim dir is in current $PATH
			// 2. PATH configured: shim dir is in shell profile but not yet sourced
			// 3. PATH missing: shim dir is not in shell profile
			pathOK := isOnPATH(shimDir)
			if pathOK {
				// Count symlinks in shim dir
				entries, err := os.ReadDir(shimDir)
				if err == nil {
					symlinkCount := 0
					for _, e := range entries {
						if info, err := e.Info(); err == nil && info.Mode()&os.ModeSymlink != 0 {
							symlinkCount++
						}
					}
					fmt.Println(colorize("32", "✓") + fmt.Sprintf(" PATH active (%d commands intercepted)", symlinkCount))
				} else {
					fmt.Println(colorize("32", "✓") + " PATH active")
				}
			} else if isConfiguredInShellProfile(shimDir) {
				fmt.Println(colorize("33", "⚠") + " PATH configured (restart shell to activate)")
				fmt.Println("  The shim directory is in your shell profile but not yet active")
				fmt.Println("  Action: Restart your shell or run: source " + detectShellConfig())
				warningIssues++
			} else {
				fmt.Println(colorize("33", "⚠") + " PATH missing — executions won't be intercepted")
				fmt.Println("  Action: Run 'brewprune quickstart' to configure PATH")
				warningIssues++
			}
		}
	}

	// Tip: alias config file — only show when there are no critical issues and
	// the daemon is not running (fresh setup) or total usage events are below
	// threshold, since the tip is most useful early on before the user has
	// established their workflow. Skip entirely when critical issues are present
	// so users focus on fixing the basics first.
	if criticalIssues == 0 && (!daemonRunning || totalUsageEvents < 10) {
		if cfgDir, err := config.Dir(); err == nil {
			aliasFile := filepath.Join(cfgDir, "aliases")
			if _, err := os.Stat(aliasFile); os.IsNotExist(err) {
				fmt.Println()
				fmt.Println("Tip: Create ~/.config/brewprune/aliases to declare alias mappings.")
				fmt.Println("     Format: one alias per line, e.g. ll=eza or g=git")
				fmt.Println("     Aliases help brewprune associate your custom commands with their packages.")
			}
		}
	}

	// Check 8: End-to-end pipeline test (only when no critical issues)
	if criticalIssues == 0 {
		// Skip or shorten pipeline test if daemon is not running, since the test
		// requires the daemon to record usage events.
		if !daemonRunning {
			fmt.Println("⊘ Pipeline test skipped (daemon not running)")
			fmt.Println("  The pipeline test requires a running daemon to record usage events")
		} else {
			pipelineStart := time.Now()
			db2, dbErr := store.New(resolvedDBPath)
			if dbErr != nil {
				fmt.Println(colorize("31", "✗") + " Pipeline test: cannot open database: " + fmt.Sprint(dbErr))
				criticalIssues++
			} else {
				defer db2.Close()
				spinner := output.NewSpinner("Running pipeline test (~30s)...")
				spinner.WithTimeout(35 * time.Second)
				spinner.Start()
				pipelineErr := RunShimTest(db2, 35*time.Second)
				pipelineElapsed := time.Since(pipelineStart).Round(time.Millisecond)
				if pipelineErr != nil {
					spinner.StopWithMessage(colorize("31", "✗") + fmt.Sprintf(" Pipeline test: fail (%v)", pipelineElapsed))
					fmt.Printf("  %v\n", pipelineErr)
					fmt.Println("  Action: Run 'brewprune watch --daemon' to restart the daemon")
					criticalIssues++
				} else {
					spinner.StopWithMessage(colorize("32", "✓") + fmt.Sprintf(" Pipeline test: pass (%v)", pipelineElapsed))
				}
			}
		}
	}

	fmt.Println()
	if criticalIssues == 0 && warningIssues == 0 {
		fmt.Println(colorize("32", "✓ All checks passed!"))
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  • Wait 1-2 weeks for usage data")
		fmt.Println("  • Check status: brewprune status")
		fmt.Println("  • View recommendations: brewprune unused --tier safe")
		return nil
	}

	if criticalIssues > 0 {
		fmt.Println(colorize("31", fmt.Sprintf("Found %d critical issue(s) and %d warning(s).", criticalIssues, warningIssues)))
		return fmt.Errorf("diagnostics failed")
	}

	// [DOCTOR-2] Warning-only path: exit 0 since warnings don't prevent usage.
	// Exit code 0 = success/warnings, exit code 1 = critical failure.
	fmt.Println(colorize("33", fmt.Sprintf("Found %d warning(s). System is functional but not fully configured.", warningIssues)))
	return nil
}
