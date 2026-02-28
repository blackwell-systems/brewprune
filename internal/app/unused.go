package app

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/blackwell-systems/brewprune/internal/analyzer"
	"github.com/blackwell-systems/brewprune/internal/brew"
	"github.com/blackwell-systems/brewprune/internal/output"
	"github.com/blackwell-systems/brewprune/internal/store"
	"github.com/spf13/cobra"
)

var (
	unusedTier     string
	unusedMinScore int
	unusedSort     string
	unusedVerbose  bool
	unusedAll      bool
)

var unusedCmd = &cobra.Command{
	Use:   "unused",
	Short: "List unused packages with confidence scores",
	Long: `Analyze installed packages and display confidence scores for removal.

Use --verbose to see detailed scoring breakdown for each package.

The confidence score (0-100) is computed from:
  - Usage patterns (40 points): Recent activity indicates active use
  - Dependencies (30 points): Fewer dependents = safer to remove
  - Age (20 points): Older installations may be stale
  - Type (10 points): Leaf packages are safer than core dependencies

Packages are classified into tiers:
  - safe (80-100): High confidence for removal
  - medium (50-79): Review before removal
  - risky (0-49): Keep unless certain

Core dependencies (git, openssl, etc.) are capped at 70 to prevent accidental removal.`,
	Example: `  # Show all unused packages
  brewprune unused

  # Show only safe-to-remove packages
  brewprune unused --tier safe

  # Preview removal with --dry-run first
  brewprune unused --tier safe
  # Then: brewprune remove --safe --dry-run
  # Then: brewprune remove --safe

  # Show packages with score >= 70
  brewprune unused --min-score 70`,
	RunE: runUnused,
}

func init() {
	unusedCmd.Flags().StringVar(&unusedTier, "tier", "", "Filter by tier: safe, medium, risky")
	unusedCmd.Flags().IntVar(&unusedMinScore, "min-score", 0, "Minimum confidence score (0-100)")
	unusedCmd.Flags().StringVar(&unusedSort, "sort", "score", "Sort by: score, size, age")
	unusedCmd.Flags().BoolVarP(&unusedVerbose, "verbose", "v", false, "Show detailed explanation for each package")
	unusedCmd.Flags().BoolVar(&unusedAll, "all", false, "Show all tiers including risky")

	// Register with root command
	RootCmd.AddCommand(unusedCmd)
}

func runUnused(cmd *cobra.Command, args []string) error {
	// Validate flags
	if unusedTier != "" && unusedTier != "safe" && unusedTier != "medium" && unusedTier != "risky" {
		return fmt.Errorf("invalid tier: %s (must be safe, medium, or risky)", unusedTier)
	}

	if unusedMinScore < 0 || unusedMinScore > 100 {
		return fmt.Errorf("invalid min-score: %d (must be 0-100)", unusedMinScore)
	}

	if unusedSort != "score" && unusedSort != "size" && unusedSort != "age" {
		return fmt.Errorf("invalid sort: %s (must be score, size, or age)", unusedSort)
	}

	// Get database path
	dbPath, err := getDBPath()
	if err != nil {
		return err
	}

	// Open store
	st, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer st.Close()

	// Check for usage data and daemon status
	checkUsageWarning(st)

	// Create analyzer
	a := analyzer.New(st)

	// Get all packages
	packages, err := st.ListPackages()
	if err != nil {
		return fmt.Errorf("failed to list packages: %w", err)
	}

	if len(packages) == 0 {
		fmt.Println("No packages found. Run 'brewprune scan' first.")
		return nil
	}

	// Drift check: warn if brew has new formulae not yet in the DB
	pkgNames := make([]string, len(packages))
	for i, p := range packages {
		pkgNames[i] = p.Name
	}
	if newCount, _ := brew.CheckStaleness(pkgNames); newCount > 0 {
		fmt.Fprintf(os.Stderr, "⚠  %d new formulae since last scan. Run 'brewprune scan' to update shims.\n\n", newCount)
	}

	// Build IsCask lookup from packages list
	isCaskMap := make(map[string]bool, len(packages))
	for _, pkg := range packages {
		isCaskMap[pkg.Name] = pkg.IsCask
	}

	// Compute scores for all packages (before filtering)
	var allScores []*analyzer.ConfidenceScore
	for _, pkg := range packages {
		score, err := a.ComputeScore(pkg.Name)
		if err != nil {
			// Skip packages with errors but log warning
			fmt.Fprintf(os.Stderr, "Warning: failed to score %s: %v\n", pkg.Name, err)
			continue
		}
		allScores = append(allScores, score)
	}

	// Compute tier stats from all scores (before any filtering)
	var safeTier, mediumTier, riskyTier output.TierStats
	for _, s := range allScores {
		switch s.Tier {
		case "safe":
			safeTier.Count++
			safeTier.SizeBytes += s.SizeBytes
		case "medium":
			mediumTier.Count++
			mediumTier.SizeBytes += s.SizeBytes
		case "risky":
			riskyTier.Count++
			riskyTier.SizeBytes += s.SizeBytes
		}
	}

	// Apply filters
	var scores []*analyzer.ConfidenceScore
	for _, s := range allScores {
		if unusedTier != "" && s.Tier != unusedTier {
			continue
		}
		if s.Score < unusedMinScore {
			continue
		}
		// When --all is not set and no explicit --tier, hide risky packages
		if !unusedAll && unusedTier == "" && s.Tier == "risky" {
			continue
		}
		scores = append(scores, s)
	}

	// Print tier summary header
	fmt.Println(output.RenderTierSummary(safeTier, mediumTier, riskyTier, unusedAll || unusedTier != ""))
	fmt.Println()

	if len(scores) == 0 {
		fmt.Println("No packages match the specified criteria.")
		return nil
	}

	// Sort scores
	sortScores(scores, unusedSort)

	// Render table
	if unusedVerbose {
		// Verbose mode - show detailed breakdown
		verboseScores := make([]output.VerboseScore, len(scores))
		for i, s := range scores {
			verboseScores[i] = output.VerboseScore{
				Package:    s.Package,
				Score:      s.Score,
				Tier:       s.Tier,
				UsageScore: s.UsageScore,
				DepsScore:  s.DepsScore,
				AgeScore:   s.AgeScore,
				TypeScore:  s.TypeScore,
				Reason:     s.Reason,
				IsCritical: s.IsCritical,
				Explanation: struct {
					UsageDetail string
					DepsDetail  string
					AgeDetail   string
					TypeDetail  string
				}{
					UsageDetail: s.Explanation.UsageDetail,
					DepsDetail:  s.Explanation.DepsDetail,
					AgeDetail:   s.Explanation.AgeDetail,
					TypeDetail:  s.Explanation.TypeDetail,
				},
			}
		}
		table := output.RenderConfidenceTableVerbose(verboseScores)
		fmt.Print(table)
	} else {
		// Convert to output format for standard table
		sevenDaysAgo := time.Now().AddDate(0, 0, -7)
		outputScores := make([]output.ConfidenceScore, len(scores))
		for i, s := range scores {
			uses7d, _ := st.GetUsageEventCountSince(s.Package, sevenDaysAgo)
			depCount, _ := st.GetReverseDependencyCount(s.Package)

			outputScores[i] = output.ConfidenceScore{
				Package:    s.Package,
				Score:      s.Score,
				Tier:       s.Tier,
				LastUsed:   getLastUsed(st, s.Package),
				Reason:     s.Reason,
				SizeBytes:  s.SizeBytes,
				Uses7d:     uses7d,
				DepCount:   depCount,
				IsCritical: s.IsCritical,
				IsCask:     isCaskMap[s.Package],
			}
		}
		table := output.RenderConfidenceTable(outputScores)
		fmt.Print(table)
	}

	// Show reclaimable footer (replaces old summary block)
	fmt.Println()
	fmt.Println(output.RenderReclaimableFooter(safeTier, mediumTier, riskyTier, unusedAll || unusedTier != ""))

	// Add confidence assessment
	if err := showConfidenceAssessment(st); err != nil {
		// Don't fail the command, just log warning
		fmt.Fprintf(os.Stderr, "Warning: failed to show confidence assessment: %v\n", err)
	}

	return nil
}

// sortScores sorts confidence scores by the specified criteria.
func sortScores(scores []*analyzer.ConfidenceScore, sortBy string) {
	switch sortBy {
	case "score":
		sort.Slice(scores, func(i, j int) bool {
			return scores[i].Score > scores[j].Score
		})
	case "size":
		sort.Slice(scores, func(i, j int) bool {
			return scores[i].SizeBytes > scores[j].SizeBytes // Largest first
		})
	case "age":
		sort.Slice(scores, func(i, j int) bool {
			return scores[i].InstalledAt.Before(scores[j].InstalledAt) // Oldest first
		})
	}
}

// computeSummary counts packages by tier.
func computeSummary(scores []*analyzer.ConfidenceScore) map[string]int {
	summary := map[string]int{
		"safe":   0,
		"medium": 0,
		"risky":  0,
	}

	for _, score := range scores {
		summary[score.Tier]++
	}

	return summary
}

// getLastUsed retrieves the last usage time for a package.
func getLastUsed(st *store.Store, pkg string) time.Time {
	lastUsed, err := st.GetLastUsage(pkg)
	if err != nil || lastUsed == nil {
		return time.Time{}
	}
	return *lastUsed
}

// checkUsageWarning checks if the daemon is running and if usage data exists,
// displaying a warning banner if no tracking is active.
func checkUsageWarning(st *store.Store) {
	// Check if any usage events exist
	var eventCount int
	row := st.DB().QueryRow("SELECT COUNT(*) FROM usage_events")
	if err := row.Scan(&eventCount); err != nil {
		// If we can't query, silently continue
		return
	}

	// If we have usage events, no warning needed
	if eventCount > 0 {
		return
	}

	// No usage events - show warning
	fmt.Println()
	fmt.Println("⚠ WARNING: No usage data available")
	fmt.Println()
	fmt.Println("The watch daemon has not recorded any package usage yet.")
	fmt.Println("Recommendations are based on heuristics only (install age, dependencies, type).")
	fmt.Println()
	fmt.Println("To track actual usage:")
	fmt.Println("  1. Start daemon:  brewprune watch --daemon")
	fmt.Println("  2. Wait 1-2 weeks for meaningful data")
	fmt.Println("  3. Re-run:        brewprune unused")
	fmt.Println()
	fmt.Println("Current recommendations are LOW CONFIDENCE without usage tracking.")
	fmt.Println("─────────────────────────────────────────────────────────────────────────")
	fmt.Println()
}

// showConfidenceAssessment displays the overall confidence level based on tracking data.
func showConfidenceAssessment(st *store.Store) error {
	eventCount, err := st.GetEventCount()
	if err != nil {
		return err
	}

	firstEventTime, err := st.GetFirstEventTime()
	if err != nil {
		return err
	}

	// Calculate days since tracking started
	var daysSinceTracking int
	if !firstEventTime.IsZero() {
		daysSinceTracking = int(time.Since(firstEventTime).Hours() / 24)
	}

	fmt.Println()

	if eventCount == 0 {
		fmt.Println("Confidence: LOW (0 usage events recorded, tracking since: never)")
		fmt.Println("Tip: Wait 1-2 weeks with daemon running for better recommendations")
	} else if daysSinceTracking < 7 {
		fmt.Printf("Confidence: MEDIUM (%d events, tracking for %d days)\n", eventCount, daysSinceTracking)
		fmt.Println("Tip: 1-2 weeks of data provides more reliable recommendations")
	} else {
		fmt.Printf("Confidence: HIGH (%d events, tracking for %d days)\n", eventCount, daysSinceTracking)
	}

	return nil
}
