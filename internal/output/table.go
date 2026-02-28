// Package output provides terminal output utilities for brewprune.
//
// This package includes:
//   - Table rendering functions for packages, confidence scores, usage stats, and snapshots
//   - Progress bars for long-running operations
//   - Spinners for indeterminate operations
//   - Human-readable formatting for sizes, dates, and other data
//
// All table rendering functions use ASCII characters and ANSI color codes for terminal output.
// Progress indicators are thread-safe and can be used from multiple goroutines.
package output

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mattn/go-isatty"

	"github.com/blackwell-systems/brewprune/internal/brew"
	"github.com/blackwell-systems/brewprune/internal/store"
)

// ANSI color codes for confidence tier display
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorGray   = "\033[90m"
)

// IsColorEnabled returns true if ANSI color codes should be emitted.
// It checks that os.Stdout is a TTY and that the NO_COLOR env var is not set.
func IsColorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return isatty.IsTerminal(os.Stdout.Fd())
}

// RenderPackageTable renders a table of packages with their details.
func RenderPackageTable(packages []*brew.Package) string {
	if len(packages) == 0 {
		return "No packages found.\n"
	}

	// Sort packages by name for consistent output
	sorted := make([]*brew.Package, len(packages))
	copy(sorted, packages)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	var sb strings.Builder

	// Header — Version column removed (pkg.Version is never populated from Homebrew metadata)
	sb.WriteString(fmt.Sprintf("%-20s %-8s %-13s %-13s\n",
		"Package", "Size", "Installed", "Last Used"))
	sb.WriteString(strings.Repeat("─", 60))
	sb.WriteString("\n")

	// Rows
	for _, pkg := range sorted {
		size := formatSize(pkg.SizeBytes)
		installed := formatRelativeTime(pkg.InstalledAt)
		lastUsed := "never" // Default, will be overridden by analyzer data

		sb.WriteString(fmt.Sprintf("%-20s %-8s %-13s %-13s\n",
			truncate(pkg.Name, 20),
			size,
			installed,
			lastUsed))
	}

	return sb.String()
}

// colorize wraps text in the given ANSI color code if color is enabled,
// otherwise returns the plain text.
func colorize(color, text string) string {
	if IsColorEnabled() {
		return color + text + colorReset
	}
	return text
}

// RenderConfidenceTable renders a table of packages with confidence scores.
// The ConfidenceScore type will be defined by the analyzer package.
// Note: Does not sort - expects scores to be pre-sorted by caller.
func RenderConfidenceTable(scores []ConfidenceScore) string {
	if len(scores) == 0 {
		return "No confidence scores available.\n"
	}

	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("%-16s %-8s %-7s %-10s %-16s %-13s %s\n",
		"Package", "Size", "Score", "Uses (7d)", "Last Used", "Depended On", "Status"))
	sb.WriteString(strings.Repeat("─", 88))
	sb.WriteString("\n")

	// Rows
	for _, score := range scores {
		size := formatSize(score.SizeBytes)
		depStr := formatDepCount(score.DepCount)
		scoreStr := fmt.Sprintf("%d/100", score.Score)

		// For risky/critical packages, show "⚠ risky" instead of tier name
		tierLabel := formatTierLabel(score.Tier, score.IsCritical)
		tierColor := getTierColor(score.Tier)

		// Cask packages show "n/a" for usage columns (shim tracking not applicable)
		var usesStr string
		var lastUsed string
		if score.IsCask {
			usesStr = "n/a"
			lastUsed = "n/a"
		} else {
			usesStr = fmt.Sprintf("%d", score.Uses7d)
			lastUsed = formatRelativeTime(score.LastUsed)
		}

		if IsColorEnabled() {
			sb.WriteString(fmt.Sprintf("%-16s %-8s %-7s %-10s %-16s %-13s %s%s%s\n",
				truncate(score.Package, 16),
				size,
				scoreStr,
				usesStr,
				lastUsed,
				depStr,
				tierColor,
				tierLabel,
				colorReset))
		} else {
			sb.WriteString(fmt.Sprintf("%-16s %-8s %-7s %-10s %-16s %-13s %s\n",
				truncate(score.Package, 16),
				size,
				scoreStr,
				usesStr,
				lastUsed,
				depStr,
				tierLabel))
		}
	}

	return sb.String()
}

// formatDepCount formats reverse dependency count for display.
func formatDepCount(count int) string {
	if count == 0 {
		return "\u2014"
	}
	if count == 1 {
		return "1 package"
	}
	return fmt.Sprintf("%d packages", count)
}

// formatTierLabel returns the display label for a tier in the table.
// Risky or critical packages show a warning indicator.
// Safe packages show "✓ safe", medium packages show "~ review".
func formatTierLabel(tier string, isCritical bool) string {
	switch strings.ToLower(tier) {
	case "safe":
		return "✓ safe"
	case "medium":
		return "~ review"
	default: // risky or critical
		return "⚠ risky"
	}
}

// VerboseScore contains detailed score information for verbose output.
// This mirrors the structure from analyzer.ConfidenceScore but is defined here
// to avoid circular dependencies.
type VerboseScore struct {
	Package     string
	Score       int
	Tier        string
	UsageScore  int
	DepsScore   int
	AgeScore    int
	TypeScore   int
	Reason      string
	IsCritical  bool
	Explanation struct {
		UsageDetail string
		DepsDetail  string
		AgeDetail   string
		TypeDetail  string
	}
}

// RenderConfidenceTableVerbose renders a detailed table showing score breakdown.
func RenderConfidenceTableVerbose(scores []VerboseScore) string {
	if len(scores) == 0 {
		return "No packages to display.\n"
	}

	var sb strings.Builder

	for i, score := range scores {
		if i > 0 {
			sb.WriteString("\n")
		}

		// Header line
		tierStr := formatTier(score.Tier)
		tierColor := getTierColor(score.Tier)
		sb.WriteString(fmt.Sprintf("Package: %s\n", score.Package))
		if IsColorEnabled() {
			sb.WriteString(fmt.Sprintf("Score:   %s%d%s (%s)\n", tierColor, score.Score, colorReset, tierStr))
		} else {
			sb.WriteString(fmt.Sprintf("Score:   %d (%s)\n", score.Score, tierStr))
		}

		// Breakdown section
		sb.WriteString("\nBreakdown:\n")
		sb.WriteString(fmt.Sprintf("  Usage:        %2d/40 pts - %s\n", score.UsageScore, score.Explanation.UsageDetail))
		sb.WriteString(fmt.Sprintf("  Dependencies: %2d/30 pts - %s\n", score.DepsScore, score.Explanation.DepsDetail))
		sb.WriteString(fmt.Sprintf("  Age:          %2d/20 pts - %s\n", score.AgeScore, score.Explanation.AgeDetail))
		sb.WriteString(fmt.Sprintf("  Type:         %2d/10 pts - %s\n", score.TypeScore, score.Explanation.TypeDetail))

		if score.IsCritical {
			sb.WriteString("  Critical:     YES      - capped at 70 (core system dependency)\n")
		}

		sb.WriteString("\nReason: " + score.Reason + "\n")
		sb.WriteString(strings.Repeat("─", 72))
		sb.WriteString("\n")
	}

	return sb.String()
}

// RenderUsageTable renders a table of usage statistics.
// The UsageStats type will be defined by the analyzer package.
func RenderUsageTable(stats map[string]UsageStats) string {
	if len(stats) == 0 {
		return "No usage statistics available.\n"
	}

	// Convert map to slice for sorting
	type entry struct {
		pkg   string
		stats UsageStats
	}
	entries := make([]entry, 0, len(stats))
	for pkg, s := range stats {
		entries = append(entries, entry{pkg: pkg, stats: s})
	}

	// Sort by total runs descending, then by LastUsed descending (zero times sorted to bottom)
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].stats.TotalRuns != entries[j].stats.TotalRuns {
			return entries[i].stats.TotalRuns > entries[j].stats.TotalRuns
		}
		iZero := entries[i].stats.LastUsed.IsZero()
		jZero := entries[j].stats.LastUsed.IsZero()
		if iZero != jZero {
			return jZero
		}
		return entries[i].stats.LastUsed.After(entries[j].stats.LastUsed)
	})

	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("%-20s %-10s %-13s %-13s %s\n",
		"Package", "Total Runs", "Last Used", "Frequency", "Trend"))
	sb.WriteString(strings.Repeat("─", 80))
	sb.WriteString("\n")

	// Rows
	for _, e := range entries {
		lastUsed := formatRelativeTime(e.stats.LastUsed)
		trend := formatTrend(e.stats.Trend)

		sb.WriteString(fmt.Sprintf("%-20s %-10d %-13s %-13s %s\n",
			truncate(e.pkg, 20),
			e.stats.TotalRuns,
			lastUsed,
			e.stats.Frequency,
			trend))
	}

	return sb.String()
}

// RenderSnapshotTable renders a table of snapshots.
func RenderSnapshotTable(snapshots []*store.Snapshot) string {
	if len(snapshots) == 0 {
		return "No snapshots found.\n"
	}

	// Sort by creation time descending (newest first)
	sorted := make([]*store.Snapshot, len(snapshots))
	copy(sorted, snapshots)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.After(sorted[j].CreatedAt)
	})

	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("%-5s %-17s %-10s %s\n",
		"ID", "Created", "Packages", "Reason"))
	sb.WriteString(strings.Repeat("─", 80))
	sb.WriteString("\n")

	// Rows
	for _, snap := range sorted {
		created := formatRelativeTime(snap.CreatedAt)

		sb.WriteString(fmt.Sprintf("%-5d %-17s %-10d %s\n",
			snap.ID,
			created,
			snap.PackageCount,
			truncate(snap.Reason, 40)))
	}

	return sb.String()
}

// formatSize converts bytes to human-readable size (GB, MB, KB).
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.0f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.0f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// formatRelativeTime converts a timestamp to relative time (e.g., "2 days ago").
func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}

	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case diff < 30*24*time.Hour:
		weeks := int(diff.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	case diff < 365*24*time.Hour:
		months := int(diff.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(diff.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

// formatTier returns a display string for confidence tier.
func formatTier(tier string) string {
	return strings.ToUpper(tier)
}

// getTierColor returns the ANSI color code for a confidence tier.
func getTierColor(tier string) string {
	switch strings.ToLower(tier) {
	case "safe":
		return colorGreen
	case "medium":
		return colorYellow
	case "risky":
		return colorRed
	default:
		return colorGray
	}
}

// formatTrend returns a visual representation of usage trend.
func formatTrend(trend string) string {
	switch strings.ToLower(trend) {
	case "up", "increasing":
		return "↑"
	case "down", "decreasing":
		return "↓"
	case "stable":
		return "→"
	default:
		return "—"
	}
}

// truncate truncates a string to maxLen, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// ConfidenceScore represents a package's confidence score for removal.
// This is a placeholder definition - the actual type will come from analyzer package.
type ConfidenceScore struct {
	Package    string
	Score      int
	Tier       string // "safe", "medium", "risky"
	LastUsed   time.Time
	Reason     string
	SizeBytes  int64 // Package size in bytes
	Uses7d     int   // Usage count in the last 7 days
	DepCount   int   // Number of reverse dependencies (packages depending on this one)
	IsCritical bool  // True if package is a core dependency
	IsCask     bool  // True if package is a cask (GUI app)
}

// TierStats holds aggregated statistics for a confidence tier.
type TierStats struct {
	Count     int
	SizeBytes int64
}

// UsageStats represents usage statistics for a package.
// This is a placeholder definition - the actual type will come from analyzer package.
type UsageStats struct {
	TotalRuns int
	LastUsed  time.Time
	Frequency string // "daily", "weekly", "monthly", "rarely"
	Trend     string // "increasing", "stable", "decreasing"
}

// RenderTierSummary renders a colored one-line tier breakdown header.
// Format: "SAFE: 5 packages (43 MB) · MEDIUM: 19 (186 MB) · RISKY: 143 (hidden, use --all)"
// When showAll is true, risky shows its size instead of "hidden".
func RenderTierSummary(safe, medium, risky TierStats, showAll bool, caskCount int) string {
	var sb strings.Builder

	// Safe tier
	if IsColorEnabled() {
		sb.WriteString(fmt.Sprintf("%sSAFE%s: %d packages (%s)",
			colorGreen, colorReset, safe.Count, formatSize(safe.SizeBytes)))
	} else {
		sb.WriteString(fmt.Sprintf("SAFE: %d packages (%s)",
			safe.Count, formatSize(safe.SizeBytes)))
	}

	sb.WriteString(" \u00b7 ")

	// Medium tier
	if IsColorEnabled() {
		sb.WriteString(fmt.Sprintf("%sMEDIUM%s: %d (%s)",
			colorYellow, colorReset, medium.Count, formatSize(medium.SizeBytes)))
	} else {
		sb.WriteString(fmt.Sprintf("MEDIUM: %d (%s)",
			medium.Count, formatSize(medium.SizeBytes)))
	}

	sb.WriteString(" \u00b7 ")

	// Risky tier
	if showAll {
		if IsColorEnabled() {
			sb.WriteString(fmt.Sprintf("%sRISKY%s: %d (%s)",
				colorRed, colorReset, risky.Count, formatSize(risky.SizeBytes)))
		} else {
			sb.WriteString(fmt.Sprintf("RISKY: %d (%s)",
				risky.Count, formatSize(risky.SizeBytes)))
		}
	} else {
		if IsColorEnabled() {
			sb.WriteString(fmt.Sprintf("%sRISKY%s: %d (hidden, use --all)",
				colorRed, colorReset, risky.Count))
		} else {
			sb.WriteString(fmt.Sprintf("RISKY: %d (hidden, use --all)",
				risky.Count))
		}
	}

	if caskCount > 0 {
		sb.WriteString(fmt.Sprintf(" \u00b7 %d casks (not tracked)", caskCount))
	}

	return sb.String()
}

// RenderReclaimableFooter renders the reclaimable space summary per tier.
// Format: "Reclaimable: 43 MB (safe) · 186 MB (medium) · 4.2 GB (risky, hidden)"
// When showAll is true, risky shows without "hidden".
func RenderReclaimableFooter(safe, medium, risky TierStats, showAll bool) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Reclaimable: %s (safe) \u00b7 %s (medium) \u00b7 %s (risky",
		formatSize(safe.SizeBytes), formatSize(medium.SizeBytes), formatSize(risky.SizeBytes)))

	if !showAll {
		sb.WriteString(", hidden")
	}

	sb.WriteString(")")

	return sb.String()
}
