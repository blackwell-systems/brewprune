package analyzer

import (
	"fmt"
	"time"

	"github.com/blackwell-systems/brewprune/internal/scanner"
)

// ComputeScore calculates the confidence score for removing a package.
// Score components:
// - Usage (40 points): Last 7d=40, 30d=30, 90d=20, 1yr=10, never=0
// - Dependencies (30 points): No deps=30, 1-3 unused=20, 1-3 used=10, 4+=0
// - Age (20 points): >180d=20, >90d=15, >30d=10, <30d=0
// - Type (10 points): Leaf with bins=10, lib no bins=5, core=0
func (a *Analyzer) ComputeScore(pkg string) (*ConfidenceScore, error) {
	// Get package info
	pkgInfo, err := a.store.GetPackage(pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to get package: %w", err)
	}

	score := &ConfidenceScore{
		Package: pkg,
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

	// Determine tier
	if score.Score >= 80 {
		score.Tier = "safe"
	} else if score.Score >= 50 {
		score.Tier = "medium"
	} else {
		score.Tier = "risky"
	}

	// Generate reason
	score.Reason = a.generateReason(score, dependents, pkgInfo.HasBinary)

	return score, nil
}

// computeUsageScore calculates usage score based on last use time.
func (a *Analyzer) computeUsageScore(pkg string) int {
	lastUsed, err := a.store.GetLastUsage(pkg)
	if err != nil || lastUsed == nil {
		// Never used or error
		return 0
	}

	daysSince := int(time.Since(*lastUsed).Hours() / 24)

	if daysSince <= 7 {
		return 40
	} else if daysSince <= 30 {
		return 30
	} else if daysSince <= 90 {
		return 20
	} else if daysSince <= 365 {
		return 10
	}
	return 0
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
	if score.UsageScore >= 30 {
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
