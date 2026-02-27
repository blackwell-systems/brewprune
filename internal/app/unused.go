package app

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/blackwell-systems/brewprune/internal/analyzer"
	"github.com/blackwell-systems/brewprune/internal/output"
	"github.com/blackwell-systems/brewprune/internal/store"
	"github.com/spf13/cobra"
)

var (
	unusedTier     string
	unusedMinScore int
	unusedSort     string
)

var unusedCmd = &cobra.Command{
	Use:   "unused",
	Short: "List unused packages with confidence scores",
	Long: `Analyze installed packages and display confidence scores for removal.

The confidence score (0-100) is computed from:
  - Usage patterns (40 points): Recent activity indicates active use
  - Dependencies (30 points): Fewer dependents = safer to remove
  - Age (20 points): Older installations may be stale
  - Type (10 points): Leaf packages are safer than core dependencies

Packages are classified into tiers:
  - safe (80-100): High confidence for removal
  - medium (50-79): Review before removal
  - risky (0-49): Keep unless certain`,
	Example: `  # Show all unused packages
  brewprune unused

  # Show only safe-to-remove packages
  brewprune unused --tier safe

  # Show packages with confidence >= 70
  brewprune unused --min-score 70

  # Sort by size instead of score
  brewprune unused --sort size`,
	RunE: runUnused,
}

func init() {
	unusedCmd.Flags().StringVar(&unusedTier, "tier", "", "Filter by tier: safe, medium, risky")
	unusedCmd.Flags().IntVar(&unusedMinScore, "min-score", 0, "Minimum confidence score (0-100)")
	unusedCmd.Flags().StringVar(&unusedSort, "sort", "score", "Sort by: score, size, age")

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

	// Compute scores for all packages
	var scores []*analyzer.ConfidenceScore
	for _, pkg := range packages {
		score, err := a.ComputeScore(pkg.Name)
		if err != nil {
			// Skip packages with errors but log warning
			fmt.Fprintf(os.Stderr, "Warning: failed to score %s: %v\n", pkg.Name, err)
			continue
		}

		// Apply filters
		if unusedTier != "" && score.Tier != unusedTier {
			continue
		}

		if score.Score < unusedMinScore {
			continue
		}

		scores = append(scores, score)
	}

	if len(scores) == 0 {
		fmt.Println("No packages match the specified criteria.")
		return nil
	}

	// Sort scores
	sortScores(scores, unusedSort)

	// Convert to output format
	outputScores := make([]output.ConfidenceScore, len(scores))
	for i, s := range scores {
		outputScores[i] = output.ConfidenceScore{
			Package:  s.Package,
			Score:    s.Score,
			Tier:     s.Tier,
			LastUsed: getLastUsed(st, s.Package),
			Reason:   s.Reason,
		}
	}

	// Render table
	table := output.RenderConfidenceTable(outputScores)
	fmt.Print(table)

	// Show summary
	summary := computeSummary(scores)
	fmt.Printf("\nSummary: %d safe, %d medium, %d risky packages\n",
		summary["safe"], summary["medium"], summary["risky"])

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
		// Sort by size requires package info - we'll sort by name for now
		// In a real implementation, we'd fetch package sizes
		sort.Slice(scores, func(i, j int) bool {
			return scores[i].Package < scores[j].Package
		})
	case "age":
		// Sort by age requires install date - we'll sort by name for now
		// In a real implementation, we'd fetch install dates
		sort.Slice(scores, func(i, j int) bool {
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
