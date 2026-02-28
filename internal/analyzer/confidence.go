package analyzer

import (
	"fmt"
	"time"

	"github.com/blackwell-systems/brewprune/internal/scanner"
)

// Data quality thresholds for tracking confidence.
// These constants define when usage data becomes reliable enough
// for confident removal recommendations.
const (
	// MinimumTrackingDays is the minimum number of days of tracking data
	// recommended before making removal decisions. After this period,
	// data quality transitions from "COLLECTING" to "READY" status.
	//
	// Why 14 days?
	// - Captures at least two weekends of usage patterns
	// - Provides sufficient sampling for weekly workflows
	// - Balances data quality with reasonable onboarding time
	MinimumTrackingDays = 14

	// OptimalTrackingDays represents the ideal tracking duration
	// for high-confidence removal decisions. After this period,
	// data quality may be considered "EXCELLENT".
	OptimalTrackingDays = 30
)

// ClassifyConfidence returns a human-readable data quality level
// based on the number of days of tracking history.
//
// Returns:
//   - "COLLECTING (N of 14 days)" when days < MinimumTrackingDays
//   - "READY" when days >= MinimumTrackingDays
//   - "EXCELLENT" when days >= OptimalTrackingDays (if extended classification is used)
//
// The data quality level helps users understand when they have enough
// tracking data to make confident removal decisions.
func ClassifyConfidence(trackingDays int) string {
	if trackingDays < MinimumTrackingDays {
		return fmt.Sprintf("COLLECTING (%d of %d days)", trackingDays, MinimumTrackingDays)
	}
	return "READY"
}

// ComputeScore calculates the confidence score for removing a package.
// Score components:
//   - Usage (40 points): Last 7d=0, 30d=10, 90d=20, 1yr=30, never=40
//     0 = recently used (keep), 40 = never used (safe to remove)
//   - Dependencies (30 points): No deps=30, 1-3 unused=20, 1-3 used=10, 4+=0
//   - Age (20 points): >180d=20, >90d=15, >30d=10, <30d=0
//   - Type (10 points): Leaf with bins=10, lib no bins=5, core=0
func (a *Analyzer) ComputeScore(pkg string) (*ConfidenceScore, error) {
	// Get package info
	pkgInfo, err := a.store.GetPackage(pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to get package: %w", err)
	}

	score := &ConfidenceScore{
		Package:     pkg,
		SizeBytes:   pkgInfo.SizeBytes,
		InstalledAt: pkgInfo.InstalledAt,
		IsCask:      pkgInfo.IsCask,
	}

	// 1. Usage Score (40 points)
	score.UsageScore = a.computeUsageScore(pkg)

	// 2. Dependencies Score (30 points)
	dependents, err := a.store.GetDependents(pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to get dependents: %w", err)
	}
	score.DepsScore = a.computeDepsScore(dependents)

	// 3. Age Score (20 points)
	score.AgeScore = a.computeAgeScore(pkgInfo.InstalledAt)

	// 4. Type Score (10 points)
	score.TypeScore = a.computeTypeScore(pkg, pkgInfo.HasBinary, len(dependents))

	// Total score
	score.Score = score.UsageScore + score.DepsScore + score.AgeScore + score.TypeScore

	// Apply criticality penalty: cap critical packages at 70 (medium tier max)
	if scanner.IsCoreDependency(pkg) {
		score.IsCritical = true
		if score.Score > 70 {
			score.Score = 70
		}
	}

	// Determine tier
	if score.Score >= 80 {
		score.Tier = "safe"
	} else if score.Score >= 50 {
		score.Tier = "medium"
	} else {
		score.Tier = "risky"
	}

	// Generate reason and explanation
	score.Reason = a.generateReason(score, dependents, pkgInfo.HasBinary)
	score.Explanation = a.generateExplanation(score, pkg, dependents, &struct {
		InstalledAt time.Time
		HasBinary   bool
	}{
		InstalledAt: pkgInfo.InstalledAt,
		HasBinary:   pkgInfo.HasBinary,
	})

	return score, nil
}

// computeUsageScore calculates usage score based on last use time.
// 0 = recently used (keep), 40 = never used (safe to remove)
func (a *Analyzer) computeUsageScore(pkg string) int {
	lastUsed, err := a.store.GetLastUsage(pkg)
	if err != nil || lastUsed == nil {
		// Never used or error
		return 40
	}

	daysSince := int(time.Since(*lastUsed).Hours() / 24)

	if daysSince <= 7 {
		return 0
	} else if daysSince <= 30 {
		return 10
	} else if daysSince <= 90 {
		return 20
	} else if daysSince <= 365 {
		return 30
	}
	return 40
}

// computeDepsScore calculates dependency score based on dependents.
func (a *Analyzer) computeDepsScore(dependents []string) int {
	numDependents := len(dependents)

	if numDependents == 0 {
		return 30
	} else if numDependents <= 3 {
		// Check if dependents are used
		usedCount := 0
		for _, dep := range dependents {
			lastUsed, err := a.store.GetLastUsage(dep)
			if err == nil && lastUsed != nil {
				// Check if used in last 30 days
				daysSince := int(time.Since(*lastUsed).Hours() / 24)
				if daysSince <= 30 {
					usedCount++
				}
			}
		}

		if usedCount == 0 {
			// All dependents are unused
			return 20
		}
		// Some dependents are used
		return 10
	}

	// 4+ dependents
	return 0
}

// computeAgeScore calculates age score based on install date.
func (a *Analyzer) computeAgeScore(installedAt time.Time) int {
	daysSince := int(time.Since(installedAt).Hours() / 24)

	if daysSince > 180 {
		return 20
	} else if daysSince > 90 {
		return 15
	} else if daysSince > 30 {
		return 10
	}
	return 0
}

// computeTypeScore calculates type score based on package characteristics.
func (a *Analyzer) computeTypeScore(pkg string, hasBinary bool, numDependents int) int {
	// Core dependency check
	if scanner.IsCoreDependency(pkg) {
		return 0
	}

	// Leaf package with binaries
	if numDependents == 0 && hasBinary {
		return 10
	}

	// Library with no binaries
	if !hasBinary {
		return 5
	}

	return 0
}

// generateReason creates a human-readable explanation for the score.
func (a *Analyzer) generateReason(score *ConfidenceScore, dependents []string, hasBinary bool) string {
	if score.Tier == "safe" {
		if score.UsageScore == 0 {
			if len(dependents) == 0 {
				return "never used, no dependents"
			}
			return "never used, only unused dependents"
		}
		return "rarely used, safe to remove"
	}

	if score.Tier == "medium" {
		if len(dependents) > 0 && len(dependents) <= 3 {
			return "has few dependents, check before removing"
		}
		if !hasBinary {
			return "library with no binaries, check dependencies"
		}
		return "medium confidence, review usage"
	}

	// Risky
	if score.UsageScore == 0 {
		return "recently used, keep"
	}
	if len(dependents) >= 4 {
		return fmt.Sprintf("has %d dependents, keep", len(dependents))
	}
	if scanner.IsCoreDependency(score.Package) {
		return "core system dependency, keep"
	}
	return "low confidence for removal"
}

// generateExplanation creates detailed component breakdown for the score.
func (a *Analyzer) generateExplanation(score *ConfidenceScore, pkg string, dependents []string, pkgInfo *struct {
	InstalledAt time.Time
	HasBinary   bool
}) ScoreExplanation {
	explanation := ScoreExplanation{}

	// Usage detail
	lastUsed, err := a.store.GetLastUsage(pkg)
	if err != nil || lastUsed == nil {
		explanation.UsageDetail = "never observed execution"
	} else {
		daysSince := int(time.Since(*lastUsed).Hours() / 24)
		switch daysSince {
		case 0:
			explanation.UsageDetail = "used today"
		case 1:
			explanation.UsageDetail = "last used 1 day ago"
		default:
			explanation.UsageDetail = fmt.Sprintf("last used %d days ago", daysSince)
		}
	}

	// Dependencies detail
	numDependents := len(dependents)
	if numDependents == 0 {
		explanation.DepsDetail = "no dependents"
	} else {
		// Count used dependents
		usedCount := 0
		for _, dep := range dependents {
			lastUsed, err := a.store.GetLastUsage(dep)
			if err == nil && lastUsed != nil {
				daysSince := int(time.Since(*lastUsed).Hours() / 24)
				if daysSince <= 30 {
					usedCount++
				}
			}
		}

		unusedCount := numDependents - usedCount
		if usedCount == 0 {
			if numDependents == 1 {
				explanation.DepsDetail = "1 unused dependent"
			} else {
				explanation.DepsDetail = fmt.Sprintf("%d unused dependents", numDependents)
			}
		} else if unusedCount == 0 {
			if numDependents == 1 {
				explanation.DepsDetail = "1 used dependent"
			} else {
				explanation.DepsDetail = fmt.Sprintf("%d used dependents", numDependents)
			}
		} else {
			explanation.DepsDetail = fmt.Sprintf("%d used, %d unused dependents", usedCount, unusedCount)
		}
	}

	// Age detail
	daysSince := int(time.Since(pkgInfo.InstalledAt).Hours() / 24)
	switch daysSince {
	case 0:
		explanation.AgeDetail = "installed today"
	case 1:
		explanation.AgeDetail = "installed 1 day ago"
	default:
		explanation.AgeDetail = fmt.Sprintf("installed %d days ago", daysSince)
	}

	// Type detail
	if score.IsCritical {
		explanation.TypeDetail = "foundational package (reduced confidence)"
	} else if numDependents == 0 && pkgInfo.HasBinary {
		explanation.TypeDetail = "leaf package with binaries"
	} else if !pkgInfo.HasBinary {
		explanation.TypeDetail = "library-only (low confidence)"
	} else {
		explanation.TypeDetail = "intermediate package"
	}

	return explanation
}

// GetPackagesByTier returns all packages with scores in the specified tier.
func (a *Analyzer) GetPackagesByTier(tier string) ([]*ConfidenceScore, error) {
	// Validate tier
	if tier != "safe" && tier != "medium" && tier != "risky" {
		return nil, fmt.Errorf("invalid tier: %s (must be safe, medium, or risky)", tier)
	}

	packages, err := a.store.ListPackages()
	if err != nil {
		return nil, fmt.Errorf("failed to list packages: %w", err)
	}

	var scores []*ConfidenceScore
	for _, pkg := range packages {
		score, err := a.ComputeScore(pkg.Name)
		if err != nil {
			// Skip packages with errors
			continue
		}

		if score.Tier == tier {
			scores = append(scores, score)
		}
	}

	return scores, nil
}
