package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	dbPath string

	// RootCmd is the root command for brewprune
	RootCmd = &cobra.Command{
		Use:   "brewprune",
		Short: "Smart Homebrew package cleanup with usage tracking",
		Long: `brewprune tracks Homebrew package usage and provides heuristic-scored
removal recommendations with automatic snapshots for easy rollback.

IMPORTANT: You must run 'brewprune watch --daemon' to track package usage.
Without the daemon running, recommendations are based on heuristics only
(install age, dependencies, type) - not actual usage data.

Quick Start:
  1. brewprune scan
  2. brewprune watch --daemon  # Keep this running!
  3. Wait 1-2 weeks for usage data
  4. brewprune unused --tier safe

Features:
  • Real-time usage tracking via PATH shims
  • Heuristic-based removal recommendations
  • Automatic snapshot creation before removals
  • One-command rollback capability
  • Dependency-aware pruning

Examples:
  # Check daemon status
  brewprune status

  # Scan installed packages
  brewprune scan

  # Start usage tracking
  brewprune watch --daemon

  # View usage-based recommendations
  brewprune unused

  # Remove unused packages safely
  brewprune remove --safe

  # Undo last removal
  brewprune undo latest`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath, _ := getDBPath()
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				fmt.Println("brewprune: Homebrew package cleanup with usage tracking")
				fmt.Println()
				fmt.Println("Run 'brewprune quickstart' to get started.")
				fmt.Println("Run 'brewprune --help' for the full reference.")
			} else {
				fmt.Println("brewprune: Homebrew package cleanup with usage tracking")
				fmt.Println()
				fmt.Println("Tip: Run 'brewprune status' to check tracking status.")
				fmt.Println("     Run 'brewprune unused' to view recommendations.")
				fmt.Println("     Run 'brewprune --help' for all commands.")
			}
			return nil
		},
	}
)

func init() {
	// Global flags
	RootCmd.PersistentFlags().StringVar(&dbPath, "db", "", "database path (default: ~/.brewprune/brewprune.db)")

	// Enable cobra's built-in suggestion feature for unknown subcommands
	RootCmd.SuggestionsMinimumDistance = 2

	// Register subcommands
	RootCmd.AddCommand(scanCmd)
	RootCmd.AddCommand(watchCmd)
	// Note: unused, stats, remove, undo commands will be added by other agents
}

// Execute runs the root command
func Execute() error {
	return RootCmd.Execute()
}

// getDBPath returns the database path, using the flag value or default
func getDBPath() (string, error) {
	if dbPath != "" {
		return dbPath, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Create .brewprune directory if it doesn't exist
	brewpruneDir := filepath.Join(home, ".brewprune")
	if err := os.MkdirAll(brewpruneDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create brewprune directory: %w", err)
	}

	return filepath.Join(brewpruneDir, "brewprune.db"), nil
}

// getDefaultPIDFile returns the default PID file path
func getDefaultPIDFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	brewpruneDir := filepath.Join(home, ".brewprune")
	if err := os.MkdirAll(brewpruneDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create brewprune directory: %w", err)
	}

	return filepath.Join(brewpruneDir, "watch.pid"), nil
}

// getDefaultLogFile returns the default log file path
func getDefaultLogFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	brewpruneDir := filepath.Join(home, ".brewprune")
	if err := os.MkdirAll(brewpruneDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create brewprune directory: %w", err)
	}

	return filepath.Join(brewpruneDir, "watch.log"), nil
}
