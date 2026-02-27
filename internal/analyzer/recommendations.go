package analyzer

import (
	"fmt"
	"sort"
)

// GetRecommendations returns packages recommended for removal based on safe tier.
// Returns safe packages sorted by size (largest first).
func (a *Analyzer) GetRecommendations() (*Recommendation, error) {
	safePackages, err := a.GetPackagesByTier("safe")
	if err != nil {
		return nil, fmt.Errorf("failed to get safe packages: %w", err)
	}

	if len(safePackages) == 0 {
		return &Recommendation{
			Packages:        []string{},
			TotalSize:       0,
			Tier:            "safe",
			ExpectedSavings: 0,
		}, nil
	}

	// Get package info to calculate total size
	var packageNames []string
	var totalSize int64

	// Create a map of package names to sizes for sorting
	type pkgSize struct {
		name string
		size int64
	}
	var pkgSizes []pkgSize

	for _, score := range safePackages {
		pkgInfo, err := a.store.GetPackage(score.Package)
		if err != nil {
			// Skip packages we can't get info for
			continue
		}

		pkgSizes = append(pkgSizes, pkgSize{
			name: score.Package,
			size: pkgInfo.SizeBytes,
		})
		totalSize += pkgInfo.SizeBytes
	}

	// Sort by size (largest first)
	sort.Slice(pkgSizes, func(i, j int) bool {
		return pkgSizes[i].size > pkgSizes[j].size
	})

	// Extract sorted package names
	for _, ps := range pkgSizes {
		packageNames = append(packageNames, ps.name)
	}

	return &Recommendation{
		Packages:        packageNames,
		TotalSize:       totalSize,
		Tier:            "safe",
		ExpectedSavings: totalSize,
	}, nil
}

// ValidateRemoval validates that a list of packages can be safely removed.
// Returns a list of warnings for packages that may cause issues.
func (a *Analyzer) ValidateRemoval(packages []string) ([]string, error) {
	var warnings []string

	for _, pkg := range packages {
		// Check if package exists
		pkgInfo, err := a.store.GetPackage(pkg)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: package not found in database", pkg))
			continue
		}

		// Get dependents
		dependents, err := a.store.GetDependents(pkg)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: failed to check dependents", pkg))
			continue
		}

		// Warn if package has dependents
		if len(dependents) > 0 {
			// Check if dependents are also being removed
			dependentsNotRemoved := []string{}
			for _, dep := range dependents {
				isBeingRemoved := false
				for _, removePkg := range packages {
					if dep == removePkg {
						isBeingRemoved = true
						break
					}
				}
				if !isBeingRemoved {
					dependentsNotRemoved = append(dependentsNotRemoved, dep)
				}
			}

			if len(dependentsNotRemoved) > 0 {
				warnings = append(warnings,
					fmt.Sprintf("%s: has %d dependents that will remain: %v",
						pkg, len(dependentsNotRemoved), dependentsNotRemoved))
			}
		}

		// Check confidence score
		score, err := a.ComputeScore(pkg)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: failed to compute confidence score", pkg))
			continue
		}

		if score.Tier == "risky" {
			warnings = append(warnings,
				fmt.Sprintf("%s: risky to remove (score: %d, reason: %s)",
					pkg, score.Score, score.Reason))
		}

		// Warn if recently used
		if score.UsageScore >= 30 {
			lastUsed, err := a.store.GetLastUsage(pkg)
			if err == nil && lastUsed != nil {
				warnings = append(warnings,
					fmt.Sprintf("%s: used recently (%s)",
						pkg, lastUsed.Format("2006-01-02")))
			}
		}

		// Check if it's explicitly installed
		if pkgInfo.InstallType == "explicit" {
			warnings = append(warnings,
				fmt.Sprintf("%s: explicitly installed (not a dependency)", pkg))
		}
	}

	return warnings, nil
}
