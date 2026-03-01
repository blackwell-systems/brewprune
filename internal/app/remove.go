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
	removeTierFlag       string
)

var removeCmd = &cobra.Command{
	Use:   "remove [packages...]",
	Short: "Remove unused Homebrew packages",
	Long: `Remove unused Homebrew packages based on confidence tiers or explicit list.

If no packages are specified, removes packages based on tier:
  --tier safe     Remove only safe-tier packages (high confidence, no impact)
  --tier medium   Remove safe and medium-tier packages
  --tier risky    Remove all unused packages (requires confirmation)

Tier shortcut flags (equivalent to --tier):
  --safe    same as --tier safe
  --medium  same as --tier medium
  --risky   same as --tier risky

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
	removeCmd.Flags().StringVar(&removeTierFlag, "tier", "", "Remove packages of specified tier: safe, medium, risky (shortcut: --safe, --medium, --risky)")

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
	var activeTier string // tracks the resolved tier for confirmation UX

	// Determine which packages to remove
	if len(args) > 0 {
		// User specified packages explicitly
		packagesToRemove = args

		// Validate packages exist and calculate size
		for _, pkg := range packagesToRemove {
			pkgInfo, err := st.GetPackage(pkg)
			if err != nil {
				return fmt.Errorf("package %q not found", pkg)
			}
			totalSize += pkgInfo.SizeBytes
		}

		// Validate removal
		warnings, err := anlzr.ValidateRemoval(packagesToRemove)
		if err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}

		// Compute scores for explicit packages to display the same table as tier-based removal
		var explicitScores []*analyzer.ConfidenceScore
		for _, pkg := range packagesToRemove {
			score, scoreErr := anlzr.ComputeScore(pkg)
			if scoreErr == nil {
				explicitScores = append(explicitScores, score)
			}
		}

		// Display warnings above the table
		if len(warnings) > 0 {
			fmt.Println()
			for _, warning := range warnings {
				fmt.Printf("  ⚠ %s\n", warning)
			}
		}

		// Display table consistent with tier-based removal
		if len(explicitScores) > 0 {
			fmt.Printf("\nPackages to remove (explicit):\n\n")
			displayConfidenceScores(st, explicitScores)
		}
	} else {
		// Use recommendation based on tier flags
		tier, tierErr := determineTier()
		if tierErr != nil {
			return tierErr
		}
		if tier == "" {
			return fmt.Errorf("no tier specified\n\nTry:\n  brewprune remove --safe --dry-run\n\nOr use --medium or --risky for more aggressive removal")
		}
		activeTier = tier

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

	// Display summary
	fmt.Printf("\nSummary:\n")
	fmt.Printf("  Packages: %d\n", len(packagesToRemove))
	fmt.Printf("  Disk space to free: %s\n", formatSize(totalSize))
	if !removeFlagNoSnapshot {
		fmt.Printf("  Snapshot: will be created\n")
	} else {
		// REMOVE-3: warn that removal cannot be undone when --no-snapshot is active
		isColor := os.Getenv("NO_COLOR") == ""
		if fi, err := os.Stdout.Stat(); err != nil || (fi.Mode()&os.ModeCharDevice) == 0 {
			isColor = false
		}
		if isColor {
			fmt.Printf("  \033[33m⚠  Snapshot: SKIPPED (--no-snapshot) — removal cannot be undone!\033[0m\n")
		} else {
			fmt.Printf("  ⚠  Snapshot: SKIPPED (--no-snapshot) — removal cannot be undone!\n")
		}
	}
	fmt.Println()

	// Dry-run mode - exit here
	if removeFlagDryRun {
		fmt.Println("Dry-run mode: no packages will be removed.")
		return nil
	}

	// Confirm removal
	if !removeFlagYes {
		if !confirmRemoval(len(packagesToRemove), activeTier) {
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
// --tier takes precedence over the boolean tier flags.
// Returns an error if multiple shorthand flags are set simultaneously, or if
// both --tier and a shorthand flag are set at the same time.
func determineTier() (string, error) {
	if removeTierFlag != "" {
		// Detect conflict: --tier combined with a shorthand flag
		if removeFlagSafe || removeFlagMedium || removeFlagRisky {
			shorthand := shorthandFlagName()
			return "", fmt.Errorf("cannot combine --tier with --%s: use one or the other", shorthand)
		}
		switch removeTierFlag {
		case "safe", "medium", "risky":
			return removeTierFlag, nil
		default:
			return "", fmt.Errorf("invalid --tier value %q: must be one of: safe, medium, risky", removeTierFlag)
		}
	}

	// Count how many shorthand flags are set
	setFlags := []string{}
	if removeFlagSafe {
		setFlags = append(setFlags, "--safe")
	}
	if removeFlagMedium {
		setFlags = append(setFlags, "--medium")
	}
	if removeFlagRisky {
		setFlags = append(setFlags, "--risky")
	}

	if len(setFlags) > 1 {
		return "", fmt.Errorf("only one tier flag can be specified at a time (got %s and %s)", setFlags[0], setFlags[1])
	}

	if removeFlagRisky {
		return "risky", nil
	}
	if removeFlagMedium {
		return "medium", nil
	}
	if removeFlagSafe {
		return "safe", nil
	}
	return "", nil
}

// shorthandFlagName returns the first shorthand tier flag that is set.
func shorthandFlagName() string {
	if removeFlagSafe {
		return "safe"
	}
	if removeFlagMedium {
		return "medium"
	}
	if removeFlagRisky {
		return "risky"
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
		return nil, fmt.Errorf("invalid --tier value %q: must be one of: safe, medium, risky", tier)
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

		// Look up IsCask from the store; default to false on error
		var isCask bool
		if pkgInfo, err := st.GetPackage(score.Package); err == nil {
			isCask = pkgInfo.IsCask
		}

		outputScores[i] = output.ConfidenceScore{
			Package:    score.Package,
			Score:      score.Score,
			Tier:       score.Tier,
			LastUsed:   getLastUsed(st, score.Package),
			Reason:     score.Reason,
			SizeBytes:  score.SizeBytes,
			Uses7d:     uses7d,
			DepCount:   depCount,
			IsCritical: score.IsCritical,
			IsCask:     isCask,
		}
	}

	fmt.Print(output.RenderConfidenceTable(outputScores))
}

// confirmRemoval prompts the user to confirm removal.
// For risky tier, it requires the literal string "yes" to confirm.
// For safe/medium tiers, it accepts "y" or "yes".
func confirmRemoval(count int, tier string) bool {
	reader := bufio.NewReader(os.Stdin)

	if tier == "risky" {
		fmt.Printf("WARNING: You are about to remove %d risky packages that may include core dependencies.\n", count)
		fmt.Println("This could break installed tools. Removal cannot be undone without a snapshot.")
		fmt.Print("Type \"yes\" to confirm (or press Enter to cancel): ")

		response, err := reader.ReadString('\n')
		if err != nil {
			return false
		}
		return strings.TrimSpace(response) == "yes"
	}

	fmt.Printf("Remove %d packages? [y/N]: ", count)

	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}
