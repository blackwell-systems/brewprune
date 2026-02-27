package app

import (
	"fmt"
	"strings"

	"github.com/blackwell-systems/brewprune/internal/analyzer"
	"github.com/blackwell-systems/brewprune/internal/store"
	"github.com/spf13/cobra"
)

var explainCmd = &cobra.Command{
	Use:   "explain [package]",
	Short: "Show detailed scoring explanation for a package",
	Long: `Display detailed breakdown of removal confidence score for a specific package.

Shows component scores, reasoning, and recommendations for the package.`,
	Example: `  # Explain score for git package
  brewprune explain git

  # Explain score for node
  brewprune explain node`,
	Args: cobra.ExactArgs(1),
	RunE: runExplain,
}

func init() {
	RootCmd.AddCommand(explainCmd)
}

func runExplain(cmd *cobra.Command, args []string) error {
	packageName := args[0]

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

	// Check if package exists
	pkg, err := st.GetPackage(packageName)
	if err != nil {
		return fmt.Errorf("package not found: %s\nRun 'brewprune scan' to update package database", packageName)
	}

	// Compute score
	score, err := a.ComputeScore(packageName)
	if err != nil {
		return fmt.Errorf("failed to compute score: %w", err)
	}

	// Display detailed explanation
	renderExplanation(score, pkg.InstalledAt.Format("2006-01-02"))

	return nil
}

func renderExplanation(score *analyzer.ConfidenceScore, installedDate string) {
	// Color codes
	const (
		colorReset  = "\033[0m"
		colorGreen  = "\033[32m"
		colorYellow = "\033[33m"
		colorRed    = "\033[31m"
		colorBold   = "\033[1m"
	)

	// Get tier color
	var tierColor string
	switch score.Tier {
	case "safe":
		tierColor = colorGreen
	case "medium":
		tierColor = colorYellow
	case "risky":
		tierColor = colorRed
	default:
		tierColor = colorReset
	}

	// Header
	fmt.Printf("\n%sPackage: %s%s\n", colorBold, score.Package, colorReset)
	fmt.Printf("Score:   %s%d%s (%s%s%s)\n",
		tierColor, score.Score, colorReset,
		tierColor, strings.ToUpper(score.Tier), colorReset)
	fmt.Printf("Installed: %s\n", installedDate)

	// Detailed Breakdown Table
	fmt.Println("\nDetailed Breakdown:")
	fmt.Println("┌─────────────────────┬─────────┬──────────────────────────────────────┐")
	fmt.Println("│ Component           │ Points  │ Detail                               │")
	fmt.Println("├─────────────────────┼─────────┼──────────────────────────────────────┤")
	fmt.Printf("│ Usage               │ %2d/40   │ %-36s │\n",
		score.UsageScore, truncateDetail(score.Explanation.UsageDetail, 36))
	fmt.Printf("│ Dependencies        │ %2d/30   │ %-36s │\n",
		score.DepsScore, truncateDetail(score.Explanation.DepsDetail, 36))
	fmt.Printf("│ Age                 │ %2d/20   │ %-36s │\n",
		score.AgeScore, truncateDetail(score.Explanation.AgeDetail, 36))
	fmt.Printf("│ Type                │ %2d/10   │ %-36s │\n",
		score.TypeScore, truncateDetail(score.Explanation.TypeDetail, 36))

	if score.IsCritical {
		fmt.Println("│ Criticality Penalty │   -30   │ core dependency (capped at 70)       │")
	}

	fmt.Println("├─────────────────────┼─────────┼──────────────────────────────────────┤")
	fmt.Printf("│ %sTotal%s               │ %s%2d/100%s │ %s%-36s%s │\n",
		colorBold, colorReset,
		tierColor, score.Score, colorReset,
		tierColor, truncateDetail(strings.ToUpper(score.Tier)+" tier", 36), colorReset)
	fmt.Println("└─────────────────────┴─────────┴──────────────────────────────────────┘")

	// Why this tier
	fmt.Printf("\n%sWhy %s:%s %s\n", colorBold, strings.ToUpper(score.Tier), colorReset, score.Reason)

	// Recommendation
	fmt.Printf("\n%sRecommendation:%s ", colorBold, colorReset)
	switch score.Tier {
	case "safe":
		fmt.Printf("%sSafe to remove.%s This package scores high for removal confidence.\n", colorGreen, colorReset)
		fmt.Println("Run 'brewprune remove --safe' to remove all safe-tier packages.")
	case "medium":
		fmt.Printf("%sReview before removing.%s Check if you use this package indirectly.\n", colorYellow, colorReset)
		fmt.Println("If certain, run 'brewprune remove " + score.Package + "'")
	case "risky":
		fmt.Printf("%sDo not remove.%s ", colorRed, colorReset)
		if score.IsCritical {
			fmt.Println("This is a foundational package that other tools may")
			fmt.Println("depend on indirectly. Even though no direct usage has been recorded,")
			fmt.Println("removing it could break your development environment.")
		} else {
			fmt.Println("This package has recent activity or many dependents.")
			fmt.Println("Removing it may break workflows or other installed packages.")
		}
	}

	if score.IsCritical {
		fmt.Printf("\n%sProtected:%s YES (part of 47 core dependencies)\n", colorBold, colorReset)
	}

	fmt.Println()
}

func truncateDetail(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
