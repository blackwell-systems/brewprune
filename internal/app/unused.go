package app

import (
	"fmt"
	"os"
	"sort"
	"strings"
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
	unusedCasks    bool
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

Core dependencies (git, openssl, etc.) are capped at 70 to prevent accidental removal.

Tier Filtering:
  --tier always shows the specified tier (safe, medium, or risky) regardless of --all.
  --all shows all tiers when --tier is not specified.
  Without --tier or --all, safe and medium tiers are shown (risky hidden by default).

Example: 'brewprune unused --tier risky' shows only risky packages.
Example: 'brewprune unused --all' shows all three tiers.

When no --tier or --all flag is set and no usage data exists, the risky tier is
shown automatically with a warning banner.`,
	Example: `  # Show all unused packages
  brewprune unused

  # Show only safe-to-remove packages
  brewprune unused --tier safe

  # Preview removal with --dry-run first
  brewprune unused --tier safe
  # Then: brewprune remove --safe --dry-run
  # Then: brewprune remove --safe

  # Show packages with score >= 70
  brewprune unused --min-score 70

  # Show all packages including hidden risky tier
  brewprune unused --all

  # Show detailed scoring breakdown
  brewprune unused --tier safe -v`,
	RunE: runUnused,
}

func init() {
	unusedCmd.Flags().StringVar(&unusedTier, "tier", "", "Filter by tier: safe, medium, risky")
	unusedCmd.Flags().IntVar(&unusedMinScore, "min-score", 0, "Minimum confidence score (0-100). Use 'brewprune explain <package>' to see a package's score.")
	unusedCmd.Flags().StringVar(&unusedSort, "sort", "score", "Sort by: score, size, age")
	unusedCmd.Flags().BoolVarP(&unusedVerbose, "verbose", "v", false, "Show detailed explanation for each package")
	unusedCmd.Flags().BoolVar(&unusedAll, "all", false, "Show all tiers including risky")
	unusedCmd.Flags().BoolVar(&unusedCasks, "casks", false, "Include casks (GUI apps) in output")

	// Register with root command
	RootCmd.AddCommand(unusedCmd)
}

func runUnused(cmd *cobra.Command, args []string) error {
	// Validate flags
	if unusedTier != "" && unusedTier != "safe" && unusedTier != "medium" && unusedTier != "risky" {
		return fmt.Errorf("invalid --tier value %q: must be one of: safe, medium, risky", unusedTier)
	}

	// UNUSED-4: --all and --tier conflict check
	if unusedAll && unusedTier != "" {
		return fmt.Errorf("Error: --all and --tier cannot be used together; --tier already filters to a specific tier")
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

	// Determine if we should implicitly show risky packages (no usage data, no explicit flags)
	var eventCount int
	row := st.DB().QueryRow("SELECT COUNT(*) FROM usage_events")
	row.Scan(&eventCount)
	showRiskyImplicit := (unusedTier == "" && !unusedAll && eventCount == 0)

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
	var caskCount int
	for _, s := range allScores {
		if isCaskMap[s.Package] {
			caskCount++
			continue
		}
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

	// UNUSED-2: Early exit when --casks is set but no casks are in the database
	if unusedCasks && caskCount == 0 {
		fmt.Println("No casks found in the Homebrew database.")
		fmt.Println("Cask tracking requires cask packages to be installed (brew install --cask <name>).")
		return nil
	}

	// Apply filters
	var scores []*analyzer.ConfidenceScore
	var belowScoreThreshold int
	for _, s := range allScores {
		// Hide casks unless --casks flag is set
		if !unusedCasks && isCaskMap[s.Package] {
			continue
		}
		if unusedTier != "" && s.Tier != unusedTier {
			continue
		}
		if s.Score < unusedMinScore {
			belowScoreThreshold++
			continue
		}
		// When --all is not set and no explicit --tier, hide risky packages
		// unless we're implicitly showing them due to no usage data
		if !unusedAll && unusedTier == "" && s.Tier == "risky" && !showRiskyImplicit {
			continue
		}
		scores = append(scores, s)
	}

	// If no usage data and no explicit flags, print prominent banner
	if showRiskyImplicit {
		fmt.Println("⚠ No usage data yet — showing all packages (risky tier included).")
		fmt.Println("  Run 'brewprune watch --daemon' and wait 1-2 weeks for better recommendations.")
		fmt.Println("  Use 'brewprune unused --all' to always show all tiers.")
		fmt.Println()
	}

	// Print tier summary header (UNUSED-5: highlight active tier when --tier is set)
	tierSummary := output.RenderTierSummary(safeTier, mediumTier, riskyTier, unusedAll || unusedTier != "" || showRiskyImplicit, caskCount)
	if unusedTier != "" {
		tierSummary = highlightActiveTier(tierSummary, unusedTier)
	}
	fmt.Println(tierSummary)

	// Add clarifying text when min-score filter is active
	if unusedMinScore > 0 {
		totalBeforeMinScore := len(allScores)
		if !unusedCasks {
			totalBeforeMinScore -= caskCount
		}
		fmt.Printf("Showing %d of %d packages (score >= %d)\n", len(scores), totalBeforeMinScore, unusedMinScore)
	}

	fmt.Println()

	if len(scores) == 0 {
		if unusedCasks {
			if caskCount == 0 {
				fmt.Println("No casks installed.")
			} else {
				fmt.Printf("No casks match the specified criteria (%d cask(s) installed).\n", caskCount)
			}
		} else {
			// Build filter description
			var filters []string
			if unusedTier != "" {
				filters = append(filters, fmt.Sprintf("tier=%s", unusedTier))
			}
			if unusedMinScore > 0 {
				filters = append(filters, fmt.Sprintf("min-score=%d", unusedMinScore))
			}

			if len(filters) > 0 {
				fmt.Printf("No packages match: %s\n", strings.Join(filters, ", "))
				fmt.Println("\nSuggestions:")
				if unusedMinScore > 0 {
					fmt.Println("  • Try lowering --min-score")
				}
				if !unusedAll && unusedTier == "" {
					fmt.Println("  • Use --all to include risky tier")
				}
				if unusedTier != "" {
					fmt.Println("  • Try a different --tier (safe, medium, risky)")
				}
			} else {
				fmt.Println("No packages match the current filters.")
			}
		}
		return nil
	}

	// Sort scores
	sortScores(scores, unusedSort)

	// Check if age sort has no effect (all packages installed at same time)
	allSameInstallTime := false
	if unusedSort == "age" && len(scores) > 1 {
		allSameInstallTime = true
		firstTime := scores[0].InstalledAt
		for _, s := range scores[1:] {
			if !s.InstalledAt.Equal(firstTime) {
				allSameInstallTime = false
				break
			}
		}
	}

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

		// Suggest pagination for long output
		if len(scores) > 10 {
			fmt.Println()
			fmt.Println("Tip: For easier viewing of long output, pipe to less:")
			fmt.Println("     brewprune unused --verbose | less")
		}
	} else {
		// Convert to output format for standard table
		sevenDaysAgo := time.Now().AddDate(0, 0, -7)

		// Check if tracking has been active for less than 1 day
		firstEventTime, _ := st.GetFirstEventTime()
		trackingLessThanOneDay := !firstEventTime.IsZero() && time.Since(firstEventTime) < 24*time.Hour

		outputScores := make([]output.ConfidenceScore, len(scores))
		for i, s := range scores {
			uses7d, _ := st.GetUsageEventCountSince(s.Package, sevenDaysAgo)
			depCount, _ := st.GetReverseDependencyCount(s.Package)

			lastUsed := getLastUsed(st, s.Package)
			// If tracking < 1 day and package has no usage, mark as not-yet-tracked
			// instead of "never" (which is misleading on fresh installs)
			if trackingLessThanOneDay && lastUsed.IsZero() {
				// Use a sentinel time value to signal "not enough tracking data"
				// This will be rendered as "—" by the output package
				lastUsed = time.Unix(1, 0) // Special marker for "no tracking data yet"
			}

			outputScores[i] = output.ConfidenceScore{
				Package:    s.Package,
				Score:      s.Score,
				Tier:       s.Tier,
				LastUsed:   lastUsed,
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

	// Show age sort note if all packages have identical install times
	if allSameInstallTime {
		fmt.Println()
		fmt.Println("Note: All packages installed at the same time — age sort has no effect. Sorted by tier, then alphabetically.")
	}

	// Show reclaimable footer (replaces old summary block)
	fmt.Println()
	footer := output.RenderReclaimableFooter(safeTier, mediumTier, riskyTier, unusedAll || unusedTier != "")
	fmt.Println(footer)

	// Show filter explanation - separate counts for clarity
	var hiddenMessages []string

	if belowScoreThreshold > 0 {
		hiddenMessages = append(hiddenMessages, fmt.Sprintf("%d below score threshold (%d)", belowScoreThreshold, unusedMinScore))
	}

	if !unusedAll && unusedTier == "" && !showRiskyImplicit && riskyTier.Count > 0 {
		hiddenMessages = append(hiddenMessages, fmt.Sprintf("%d in risky tier", riskyTier.Count))
	}

	if len(hiddenMessages) > 0 {
		fmt.Printf("Hidden: %s", strings.Join(hiddenMessages, "; "))
		if !unusedAll && unusedTier == "" && !showRiskyImplicit {
			fmt.Print(" (use --all to show risky)")
		}
		fmt.Println()
	}

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
		sort.SliceStable(scores, func(i, j int) bool {
			if scores[i].Score != scores[j].Score {
				return scores[i].Score > scores[j].Score
			}
			return scores[i].Package < scores[j].Package // stable alpha fallback
		})
	case "size":
		sort.SliceStable(scores, func(i, j int) bool {
			if scores[i].SizeBytes != scores[j].SizeBytes {
				return scores[i].SizeBytes > scores[j].SizeBytes // Largest first
			}
			return scores[i].Package < scores[j].Package // stable alpha fallback
		})
	case "age":
		sort.SliceStable(scores, func(i, j int) bool {
			if !scores[i].InstalledAt.Equal(scores[j].InstalledAt) {
				return scores[i].InstalledAt.Before(scores[j].InstalledAt) // Oldest first
			}
			// Secondary: tier order (safe → medium → risky)
			tierOrder := map[string]int{"safe": 0, "medium": 1, "risky": 2}
			ti := tierOrder[scores[i].Tier]
			tj := tierOrder[scores[j].Tier]
			if ti != tj {
				return ti < tj
			}
			// Tertiary: alphabetical
			return scores[i].Package < scores[j].Package
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
// VISUAL-1: All ANSI escape sequences are guarded by a TTY check to prevent leakage
// when output is piped or redirected.
// UNUSED-3: A score inversion note is displayed in the Breakdown section to clarify
// that higher scores mean safer to remove.
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

	// VISUAL-1: TTY check — guard all ANSI sequences to prevent leakage when piped
	isColor := func() bool {
		if os.Getenv("NO_COLOR") != "" {
			return false
		}
		fi, err := os.Stdout.Stat()
		return err == nil && (fi.Mode()&os.ModeCharDevice) != 0
	}()

	// ANSI color codes for confidence levels (only used when isColor is true)
	const (
		colorRed    = "\033[31m"
		colorYellow = "\033[33m"
		colorGreen  = "\033[32m"
		colorReset  = "\033[0m"
	)

	// UNUSED-3: Score inversion note — printed once before confidence line.
	// Canonical verbose Breakdown format (for Agent G / explain.go to match):
	//   Breakdown:
	//     (score measures removal confidence: higher = safer to remove)
	//     Usage:        <n>/40 pts - <detail>
	//     Dependencies: <n>/30 pts - <detail>
	//     Age:          <n>/20 pts - <detail>
	//     Type:         <n>/10 pts - <detail>
	fmt.Println("Breakdown:")
	fmt.Println("  (score measures removal confidence: higher = safer to remove)")

	if isColor {
		if eventCount == 0 {
			fmt.Printf("Confidence: %sLOW%s (0 usage events recorded, tracking since: never)\n", colorRed, colorReset)
			fmt.Println("Tip: Wait 1-2 weeks with daemon running for better recommendations")
		} else if daysSinceTracking < 7 {
			fmt.Printf("Confidence: %sMEDIUM%s (%d events, tracking for %d days)\n", colorYellow, colorReset, eventCount, daysSinceTracking)
			fmt.Println("Tip: 1-2 weeks of data provides more reliable recommendations")
		} else {
			fmt.Printf("Confidence: %sHIGH%s (%d events, tracking for %d days)\n", colorGreen, colorReset, eventCount, daysSinceTracking)
		}
	} else {
		if eventCount == 0 {
			fmt.Printf("Confidence: LOW (0 usage events recorded, tracking since: never)\n")
			fmt.Println("Tip: Wait 1-2 weeks with daemon running for better recommendations")
		} else if daysSinceTracking < 7 {
			fmt.Printf("Confidence: MEDIUM (%d events, tracking for %d days)\n", eventCount, daysSinceTracking)
			fmt.Println("Tip: 1-2 weeks of data provides more reliable recommendations")
		} else {
			fmt.Printf("Confidence: HIGH (%d events, tracking for %d days)\n", eventCount, daysSinceTracking)
		}
	}

	return nil
}

// highlightActiveTier modifies a tier summary string to visually mark the active tier.
// For non-TTY: wraps the active tier label in brackets, e.g. [SAFE: 5 packages (39 MB)].
// For TTY: also bolds the bracketed section.
// Appends "(filtered to <tier>)" at the end of the line.
func highlightActiveTier(summary, activeTier string) string {
	// Determine if output is a TTY for bold support
	isTTY := false
	if os.Getenv("NO_COLOR") == "" {
		if fi, err := os.Stdout.Stat(); err == nil {
			isTTY = (fi.Mode() & os.ModeCharDevice) != 0
		}
	}

	upper := strings.ToUpper(activeTier)

	// Find the tier label in the summary and wrap it in brackets.
	// The summary format is: "SAFE: N packages (X MB) · MEDIUM: N (X MB) · RISKY: N (X MB)"
	// We need to wrap everything from "SAFE:" through the next " ·" (or end of ANSI/plain tier block).
	// Strategy: find the tier name (possibly preceded by ANSI codes) and bracket the segment.

	const boldOn = "\033[1m"
	const boldOff = "\033[0m"

	// We'll use a simple string replacement strategy.
	// Find tier keyword boundaries by looking for " · " separators.
	// Split on " · " (middle dot separator used in RenderTierSummary)
	sep := " \u00b7 "
	parts := strings.Split(summary, sep)

	for i, part := range parts {
		// Strip ANSI for comparison
		stripped := stripANSI(part)
		if strings.HasPrefix(stripped, upper+":") {
			if isTTY {
				parts[i] = boldOn + "[" + part + "]" + boldOff
			} else {
				parts[i] = "[" + part + "]"
			}
			break
		}
	}

	result := strings.Join(parts, sep)
	result += "  (filtered to " + activeTier + ")"
	return result
}

// stripANSI removes ANSI escape sequences from a string for plain-text comparison.
func stripANSI(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			// Skip until 'm'
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			i = j + 1
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}
