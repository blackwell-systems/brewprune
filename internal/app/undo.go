package app

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/blackwell-systems/brewprune/internal/output"
	"github.com/blackwell-systems/brewprune/internal/snapshots"
	"github.com/blackwell-systems/brewprune/internal/store"
	"github.com/spf13/cobra"
)

var (
	undoFlagList bool
	undoFlagYes  bool
)

var undoCmd = &cobra.Command{
	Use:   "undo [snapshot-id | latest]",
	Short: "Restore packages from a snapshot",
	Long: `Restore previously removed packages from a snapshot.

Snapshots are automatically created before package removal operations
and can be used to rollback changes.

Arguments:
  snapshot-id  The numeric ID of the snapshot to restore
  latest       Restore the most recent snapshot

Flags:
  --list       List all available snapshots
  --yes        Skip confirmation prompt

Examples:
  brewprune undo --list           # List all snapshots
  brewprune undo latest           # Restore latest snapshot
  brewprune undo 42               # Restore snapshot ID 42
  brewprune undo 42 --yes         # Restore without confirmation`,
	RunE: runUndo,
}

func init() {
	undoCmd.Flags().BoolVar(&undoFlagList, "list", false, "List available snapshots")
	undoCmd.Flags().BoolVar(&undoFlagYes, "yes", false, "Skip confirmation prompt")

	RootCmd.AddCommand(undoCmd)
}

func runUndo(cmd *cobra.Command, args []string) error {
	// Open database
	dbPath, err := getDBPath()
	if err != nil {
		return fmt.Errorf("failed to get database path: %w", err)
	}
	st, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer st.Close()

	// Initialize snapshot manager
	snapshotDir := getSnapshotDir()
	snapMgr := snapshots.New(st, snapshotDir)

	// Handle --list flag
	if undoFlagList {
		return listSnapshots(snapMgr)
	}

	// Require snapshot ID or "latest"
	if len(args) == 0 {
		return fmt.Errorf("snapshot ID or 'latest' required (use --list to see available snapshots)")
	}

	snapshotArg := args[0]

	// Get snapshot ID
	var snapshotID int64
	if strings.ToLower(snapshotArg) == "latest" {
		// Get latest snapshot
		snaps, listErr := snapMgr.ListSnapshots()
		if listErr != nil {
			return fmt.Errorf("failed to list snapshots: %w", listErr)
		}

		// [UNDO-1] Friendly message when no snapshots exist instead of an error.
		if len(snaps) == 0 {
			fmt.Println("No snapshots available.")
			fmt.Println("\nSnapshots are automatically created before package removal.")
			fmt.Println("Use 'brewprune remove' to remove packages and create snapshots.")
			return nil
		}

		// Snapshots are ordered by creation time (newest first)
		snapshotID = snaps[0].ID
		fmt.Printf("Using latest snapshot: ID %d\n", snapshotID)
	} else {
		// Parse snapshot ID
		id, err := strconv.ParseInt(snapshotArg, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid snapshot ID: %s (must be a number or 'latest')", snapshotArg)
		}
		snapshotID = id
	}

	// Get snapshot details
	snapshot, err := st.GetSnapshot(snapshotID)
	if err != nil {
		return fmt.Errorf("snapshot %d not found: %w", snapshotID, err)
	}

	// Get snapshot packages
	snapshotPackages, err := st.GetSnapshotPackages(snapshotID)
	if err != nil {
		return fmt.Errorf("failed to get snapshot packages: %w", err)
	}

	// Display snapshot details
	fmt.Printf("\nSnapshot Details:\n")
	fmt.Printf("  ID: %d\n", snapshot.ID)
	fmt.Printf("  Created: %s\n", snapshot.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Reason: %s\n", snapshot.Reason)
	fmt.Printf("  Packages: %d\n", snapshot.PackageCount)
	fmt.Println()

	// Display packages to restore
	if len(snapshotPackages) > 0 {
		fmt.Println("Packages to restore:")
		for _, pkg := range snapshotPackages {
			explicitStr := ""
			if pkg.WasExplicit {
				explicitStr = " (explicit)"
			}
			fmt.Printf("  - %s@%s%s\n", pkg.PackageName, pkg.Version, explicitStr)
		}
		fmt.Println()
	}

	// Confirm restoration
	if !undoFlagYes {
		if !confirmRestore(len(snapshotPackages)) {
			fmt.Println("Restoration cancelled.")
			return nil
		}
	}

	// Restore snapshot
	fmt.Printf("Restoring %d packages...\n", len(snapshotPackages))
	progress := output.NewProgress(len(snapshotPackages), "Restoring packages")

	// Use a spinner for the restoration process since snapshots.RestoreSnapshot
	// doesn't provide per-package progress
	progress.Finish() // Clear the progress bar
	spinner := output.NewSpinner("Restoring packages from snapshot...")
	err = snapMgr.RestoreSnapshot(snapshotID)
	spinner.Stop()

	if err != nil {
		// Partial restoration may have occurred
		fmt.Fprintf(os.Stderr, "\n⚠️  Restoration completed with errors: %v\n", err)
		fmt.Println("\nSome packages may have been restored successfully.")
		fmt.Println("Run 'brewprune scan' to update the package database.")
		return nil // Don't return error - some packages may have been restored
	}

	fmt.Printf("\n✓ Restored %d packages from snapshot %d\n", len(snapshotPackages), snapshotID)
	fmt.Println("\nRun 'brewprune scan' to update the package database.")

	return nil
}

// listSnapshots displays all available snapshots.
func listSnapshots(snapMgr *snapshots.Manager) error {
	snaps, err := snapMgr.ListSnapshots()
	if err != nil {
		return fmt.Errorf("failed to list snapshots: %w", err)
	}

	if len(snaps) == 0 {
		fmt.Println("No snapshots available.")
		fmt.Println("\nSnapshots are automatically created before package removal.")
		fmt.Println("Use 'brewprune remove' to remove packages and create snapshots.")
		return nil
	}

	fmt.Printf("\nAvailable snapshots:\n\n")
	fmt.Print(output.RenderSnapshotTable(snaps))

	fmt.Printf("\nRestore with: brewprune undo <id>\n")

	return nil
}

// confirmRestore prompts the user to confirm restoration.
func confirmRestore(count int) bool {
	fmt.Printf("Restore %d packages? [y/N]: ", count)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}
