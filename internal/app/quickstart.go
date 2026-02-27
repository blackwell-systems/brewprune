package app

import (
	"fmt"

	"github.com/spf13/cobra"
)

var quickstartCmd = &cobra.Command{
	Use:   "quickstart",
	Short: "Interactive walkthrough for first-time users",
	Long: `Guides you through the initial setup of brewprune.

This command walks you through:
  1. Scanning your installed packages
  2. Starting the usage tracking daemon
  3. Understanding the timeline and next steps`,
	RunE: runQuickstart,
}

func init() {
	RootCmd.AddCommand(quickstartCmd)
}

func runQuickstart(cmd *cobra.Command, args []string) error {
	fmt.Println("Welcome to brewprune! Let's get you set up.")
	fmt.Println()

	// Step 1: Scan
	fmt.Println("Step 1/3: Scanning installed packages")
	fmt.Println("Running: brewprune scan")
	fmt.Println()
	if err := runScan(cmd, args); err != nil {
		return err
	}

	// Step 2: Daemon
	fmt.Println("\nStep 2/3: Starting usage tracking")
	fmt.Println("The daemon monitors which packages you actually use.")
	fmt.Println("Running: brewprune watch --daemon")
	fmt.Println()
	// Set daemon flag and run watch
	watchDaemon = true
	if err := runWatch(cmd, args); err != nil {
		return err
	}

	// Step 3: Next steps
	fmt.Println("\n✓ Setup complete!")
	fmt.Println()
	fmt.Println("IMPORTANT: Wait 1-2 weeks for meaningful data")
	fmt.Println()
	fmt.Println("What happens next:")
	fmt.Println("  • The daemon runs in the background")
	fmt.Println("  • It tracks when you use packages")
	fmt.Println("  • After 1-2 weeks, run: brewprune unused --tier safe")
	fmt.Println()
	fmt.Println("Check status anytime: brewprune status")

	return nil
}
