package app

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/blackwell-systems/brewprune/internal/analyzer"
	"github.com/blackwell-systems/brewprune/internal/brew"
	"github.com/blackwell-systems/brewprune/internal/output"
	"github.com/blackwell-systems/brewprune/internal/scanner"
	"github.com/blackwell-systems/brewprune/internal/snapshots"
	"github.com/blackwell-systems/brewprune/internal/store"
	"github.com/spf13/cobra"
)

var (
	removeFlagSafe       bool
	removeFlagMedium     bool
	removeFlagRisky      bool
	removeFlagDryRun     bool
	removeFlagYes        bool
	removeFlagNoSnapshot bool
)

var removeCmd = &cobra.Command{
	Use:   "remove [packages...]",
	Short: "Remove unused Homebrew packages",
	Long: `Remove unused Homebrew packages based on confidence tiers or explicit list.

If no packages are specified, removes packages based on tier flags:
  --safe:   Remove only safe-tier packages (high confidence, no impact)
  --medium: Remove safe and medium-tier packages
  --risky:  Remove all unused packages (requires confirmation)

If packages are specified, validates and removes those specific packages.

Safety features:
  - Validates removal candidates before proceeding
  - Warns about dependent packages
  - Creates automatic snapshot (unless --no-snapshot)
  - Requires confirmation for risky operations

Examples:
  # Preview what would be removed (dry-run)
  brewprune remove --safe --dry-run

  # Actually remove safe packages
  brewprune remove --safe

  # Remove specific packages with confirmation
  brewprune remove package1 package2

  # Remove medium-tier packages without snapshot (dangerous!)
  brewprune remove --medium --no-snapshot --yes`,
	RunE: runRemove,
}

func init() {
	removeCmd.Flags().BoolVar(&removeFlagSafe, "safe", false, "Remove only safe-tier packages")
	removeCmd.Flags().BoolVar(&removeFlagMedium, "medium", false, "Remove safe and medium-tier packages")
	removeCmd.Flags().BoolVar(&removeFlagRisky, "risky", false, "Remove all packages (requires confirmation)")
	removeCmd.Flags().BoolVar(&removeFlagDryRun, "dry-run", false, "Show what would be removed without removing")
	removeCmd.Flags().BoolVar(&removeFlagYes, "yes", false, "Skip confirmation prompts")
	removeCmd.Flags().BoolVar(&removeFlagNoSnapshot, "no-snapshot", false, "Skip automatic snapshot creation (dangerous)")

	RootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
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

	// Initialize components
	anlzr := analyzer.New(st)
	snapshotDir := getSnapshotDir()
	snapMgr := snapshots.New(st, snapshotDir)

	// Drift check: warn if brew has new formulae not yet in the DB
	if allPkgs, err := st.ListPackages(); err == nil {
		pkgNames := make([]string, len(allPkgs))
		for i, p := range allPkgs {
			pkgNames[i] = p.Name
		}
		if newCount, _ := brew.CheckStaleness(pkgNames); newCount > 0 {
			fmt.Fprintf(os.Stderr, "⚠  %d new formulae since last scan. Run 'brewprune scan' to update shims.\n\n", newCount)
		}
	}

	var packagesToRemove []string
	var totalSize int64

	// Determine which packages to remove
	if len(args) > 0 {
		// User specified packages explicitly
		packagesToRemove = args

		// Validate packages exist and calculate size
		for _, pkg := range packagesToRemove {
			pkgInfo, err := st.GetPackage(pkg)
			if err != nil {
				return fmt.Errorf("package %s not found: %w", pkg, err)
			}
			totalSize += pkgInfo.SizeBytes
		}

		// Validate removal
		warnings, err := anlzr.ValidateRemoval(packagesToRemove)
		if err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}

		if len(warnings) > 0 {
			fmt.Println("\nWarnings:")
			for _, warning := range warnings {
				fmt.Printf("  ⚠️  %s\n", warning)
			}
			fmt.Println()
		}
	} else {
		// Use recommendation based on tier flags
		tier := determineTier()
		if tier == "" {
			return fmt.Errorf("no tier specified: use --safe, --medium, or --risky")
		}

		scores, err := getPackagesByTier(anlzr, tier)
		if err != nil {
			return fmt.Errorf("failed to get packages: %w", err)
		}

		if len(scores) == 0 {
			fmt.Printf("No %s packages found for removal.\n", tier)
			return nil
		}

		// Extract package names and calculate total size
		for _, score := range scores {
			packagesToRemove = append(packagesToRemove, score.Package)
			pkgInfo, err := st.GetPackage(score.Package)
			if err == nil {
				totalSize += pkgInfo.SizeBytes
			}
		}

		// Display table of packages to remove
		fmt.Printf("\nPackages to remove (%s tier):\n\n", tier)
		displayConfidenceScores(st, scores)
	}

	if len(packagesToRemove) == 0 {
		fmt.Println("No packages to remove.")
		return nil
	}

	// Show per-package score summary inline before confirmation
	if len(packagesToRemove) > 0 && len(args) == 0 {
		// Already shown via displayConfidenceScores for tier-based removal
	} else if len(args) > 0 {
		fmt.Println()
		for _, pkg := range packagesToRemove {
			score, err := anlzr.ComputeScore(pkg)
			if err == nil {
				fmt.Printf("  %-20s  %3d/100  %-6s  %s\n", pkg, score.Score, strings.ToUpper(score.Tier), score.Reason)
			}
		}
	}

	// Display summary
	fmt.Printf("\nSummary:\n")
	fmt.Printf("  Packages: %d\n", len(packagesToRemove))
	fmt.Printf("  Disk space to free: %s\n", formatSize(totalSize))
	if !removeFlagNoSnapshot {
		fmt.Printf("  Snapshot: will be created\n")
	} else {
		fmt.Printf("  Snapshot: SKIPPED (--no-snapshot)\n")
	}
	fmt.Println()

	// Dry-run mode - exit here
	if removeFlagDryRun {
		fmt.Println("Dry-run mode: no packages will be removed.")
		return nil
	}

	// Confirm removal
	if !removeFlagYes {
		if !confirmRemoval(len(packagesToRemove)) {
			fmt.Println("Removal cancelled.")
			return nil
		}
	}

	// Create snapshot unless --no-snapshot
	var snapshotID int64
	if !removeFlagNoSnapshot {
		fmt.Println("Creating snapshot...")
		snapshotID, err = snapMgr.CreateSnapshot(packagesToRemove, "before removal")
		if err != nil {
			return fmt.Errorf("failed to create snapshot: %w", err)
		}
		fmt.Printf("Snapshot created: ID %d\n\n", snapshotID)
	}

	// Remove packages
	fmt.Printf("Removing %d packages...\n", len(packagesToRemove))
	progress := output.NewProgress(len(packagesToRemove), "Removing packages")

	successCount := 0
	var failures []string

	for _, pkg := range packagesToRemove {
		// Check if it's a core dependency (safety check)
		if scanner.IsCoreDependency(pkg) {
			failures = append(failures, fmt.Sprintf("%s: core dependency, skipped", pkg))
			progress.Increment()
			continue
		}

		// Uninstall package
		if err := brew.Uninstall(pkg); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", pkg, err))
			progress.Increment()
			continue
		}

		// Update database
		if err := st.DeletePackage(pkg); err != nil {
			// Non-fatal - package was removed from system but not from DB
			fmt.Fprintf(os.Stderr, "\nWarning: removed %s but failed to update database: %v\n", pkg, err)
		}

		successCount++
		progress.Increment()
	}

	progress.Finish()

	// Display results
	fmt.Printf("\n✓ Removed %d packages, freed %s\n", successCount, formatSize(totalSize))

	if len(failures) > 0 {
		fmt.Printf("\n⚠️  %d failures:\n", len(failures))
		for _, failure := range failures {
			fmt.Printf("  - %s\n", failure)
		}
	}

	if !removeFlagNoSnapshot {
		fmt.Printf("\nSnapshot: ID %d\n", snapshotID)
		fmt.Printf("Undo with: brewprune undo %d\n", snapshotID)
	}

	return nil
}

// determineTier returns the highest tier based on flags.
func determineTier() string {
	if removeFlagRisky {
		return "risky"
	}
	if removeFlagMedium {
		return "medium"
	}
	if removeFlagSafe {
		return "safe"
	}
	return ""
}

// getPackagesByTier returns packages for the specified tier and all lower tiers.
func getPackagesByTier(anlzr *analyzer.Analyzer, tier string) ([]*analyzer.ConfidenceScore, error) {
	var allScores []*analyzer.ConfidenceScore

	// Include packages from the specified tier and safer tiers
	switch tier {
	case "risky":
		// Include all tiers
		for _, t := range []string{"safe", "medium", "risky"} {
			scores, err := anlzr.GetPackagesByTier(t)
			if err != nil {
				return nil, fmt.Errorf("failed to get %s tier: %w", t, err)
			}
			allScores = append(allScores, scores...)
		}
	case "medium":
		// Include safe and medium
		for _, t := range []string{"safe", "medium"} {
			scores, err := anlzr.GetPackagesByTier(t)
			if err != nil {
				return nil, fmt.Errorf("failed to get %s tier: %w", t, err)
			}
			allScores = append(allScores, scores...)
		}
	case "safe":
		// Only safe tier
		scores, err := anlzr.GetPackagesByTier("safe")
		if err != nil {
			return nil, err
		}
		allScores = scores
	default:
		return nil, fmt.Errorf("invalid tier: %s", tier)
	}

	return allScores, nil
}

// displayConfidenceScores displays packages with confidence scores in a table.
func displayConfidenceScores(st *store.Store, scores []*analyzer.ConfidenceScore) {
	if len(scores) == 0 {
		return
	}

	sevenDaysAgo := time.Now().AddDate(0, 0, -7)

	// Convert to output-compatible format
	outputScores := make([]output.ConfidenceScore, len(scores))
	for i, score := range scores {
		uses7d, _ := st.GetUsageEventCountSince(score.Package, sevenDaysAgo)
		depCount, _ := st.GetReverseDependencyCount(score.Package)

		outputScores[i] = output.ConfidenceScore{
			Package:    score.Package,
			Score:      score.Score,
			Tier:       score.Tier,
			LastUsed:   getNeverTime(),
			Reason:     score.Reason,
			SizeBytes:  score.SizeBytes,
			Uses7d:     uses7d,
			DepCount:   depCount,
			IsCritical: score.IsCritical,
		}
	}

	fmt.Print(output.RenderConfidenceTable(outputScores))
}

// confirmRemoval prompts the user to confirm removal.
func confirmRemoval(count int) bool {
	fmt.Printf("Remove %d packages? [y/N]: ", count)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}
