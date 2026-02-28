package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/blackwell-systems/brewprune/internal/analyzer"
	"github.com/blackwell-systems/brewprune/internal/store"
	"github.com/spf13/cobra"
)

var explainCmd = &cobra.Command{
	Use:   "explain <package>",
	Short: "Show detailed scoring explanation for a package",
	Long: `Display detailed breakdown of removal confidence score for a specific package.

Shows component scores, reasoning, and recommendations for the package.`,
	Example: `  # Explain score for git package
  brewprune explain git

  # Explain score for node
  brewprune explain node`,
	// [EXPLAIN-2] Custom Args validator with a friendly error message.
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("missing package name. Usage: brewprune explain <package>")
		}
		return cobra.ExactArgs(1)(cmd, args)
	},
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
	// [EXPLAIN-1] Print directly to stderr and call os.Exit(1) so main.go's
	// error handler is never reached (guaranteeing exactly one print) AND the
	// exit code is non-zero for the error condition.
	_, err = st.GetPackage(packageName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: package not found: %s\n\nCheck the name with 'brew list' or 'brew search %s'.\nIf you just installed it, run 'brewprune scan' to update the index.\n", packageName, packageName)
		os.Exit(1)
	}

	// Compute score
	score, err := a.ComputeScore(packageName)
	if err != nil {
		return fmt.Errorf("failed to compute score: %w", err)
	}

	// We need the package install date; fetch it again (GetPackage already
	// succeeded above so this is safe).
	pkg, _ := st.GetPackage(packageName)
	installedDate := ""
	if pkg != nil {
		installedDate = pkg.InstalledAt.Format("2006-01-02")
	}

	// Display detailed explanation
	renderExplanation(score, installedDate)

	return nil
}

// renderExplanation displays a detailed breakdown of a package's confidence score.
//
// ANSI Formatting Note: This function uses ANSI escape codes (e.g., \033[1m for bold,
// \033[32m for green) to provide colored terminal output. These codes render correctly
// in standard terminal environments but may appear as raw text (e.g., [1m, [0m) when:
//   - Output is redirected to a file or pipe
//   - Running in non-ANSI environments (e.g., certain Docker containers)
//   - Terminal does not support ANSI escape sequences
//
// This is expected behavior and not a bug. The raw codes are harmless and can be
// filtered using tools like `sed` or by setting NO_COLOR=1 in future enhancements.
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
	fmt.Println("┌─────────────────────┬─────────┬────────────────────────────────────────────────────┐")
	fmt.Println("│ Component           │  Score  │ Detail                                             │")
	fmt.Println("├─────────────────────┼─────────┼────────────────────────────────────────────────────┤")
	fmt.Printf("│ Usage               │ %2d/40   │ %-50s │\n",
		score.UsageScore, truncateDetail(score.Explanation.UsageDetail, 50))
	fmt.Printf("│ Dependencies        │ %2d/30   │ %-50s │\n",
		score.DepsScore, truncateDetail(score.Explanation.DepsDetail, 50))
	fmt.Printf("│ Age                 │ %2d/20   │ %-50s │\n",
		score.AgeScore, truncateDetail(score.Explanation.AgeDetail, 50))
	fmt.Printf("│ Type                │ %2d/10   │ %-50s │\n",
		score.TypeScore, truncateDetail(score.Explanation.TypeDetail, 50))

	if score.IsCritical {
		fmt.Println("│ Criticality Penalty │   -30   │ core dependency (capped at 70)                     │")
	}

	fmt.Println("├─────────────────────┼─────────┼────────────────────────────────────────────────────┤")
	// [EXPLAIN-3] Compute padding on plain string first, then wrap with color
	// to avoid ANSI codes inflating the fmt width count.
	tierLabel := truncateDetail(strings.ToUpper(score.Tier)+" tier", 50)
	paddedLabel := fmt.Sprintf("%-50s", tierLabel)
	fmt.Printf("│ %sTotal%s               │ %s%2d/100%s │ %s%s%s │\n",
		colorBold, colorReset,
		tierColor, score.Score, colorReset,
		tierColor, paddedLabel, colorReset)
	fmt.Println("└─────────────────────┴─────────┴────────────────────────────────────────────────────┘")

	fmt.Println()
	fmt.Println("Note: Higher removal score = more confident to remove.")
	fmt.Println("      Usage component: 0/40 means recently used (fewer points toward removal).")
	fmt.Println("      40/40 means no usage ever observed.")

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
