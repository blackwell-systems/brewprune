package analyzer

import (
	"fmt"
	"time"
)

// GetUsageStats returns usage statistics for a specific package.
func (a *Analyzer) GetUsageStats(pkg string) (*UsageStats, error) {
	// Get package info for FirstSeen
	pkgInfo, err := a.store.GetPackage(pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to get package: %w", err)
	}

	// Get all usage events for the package
	events, err := a.store.GetUsageEvents(pkg, time.Time{})
	if err != nil {
		return nil, fmt.Errorf("failed to get usage events: %w", err)
	}

	stats := &UsageStats{
		Package:   pkg,
		TotalUses: len(events),
		FirstSeen: pkgInfo.InstalledAt,
		DaysSince: -1, // Default to -1 if never used
	}

	// Find last used time
	if len(events) > 0 {
		// Events are ordered by timestamp DESC, so first is most recent
		stats.LastUsed = &events[0].Timestamp
		stats.DaysSince = int(time.Since(*stats.LastUsed).Hours() / 24)
	}

	// Compute frequency
	stats.Frequency = a.computeFrequency(stats.LastUsed, stats.TotalUses, pkgInfo.InstalledAt)

	return stats, nil
}

// computeFrequency determines usage frequency classification.
func (a *Analyzer) computeFrequency(lastUsed *time.Time, totalUses int, installedAt time.Time) string {
	if lastUsed == nil {
		return "never"
	}

	daysSinceInstall := int(time.Since(installedAt).Hours() / 24)
	if daysSinceInstall == 0 {
		daysSinceInstall = 1 // Avoid division by zero
	}

	// Calculate average uses per day
	usesPerDay := float64(totalUses) / float64(daysSinceInstall)

	// Also check recency
	daysSinceLastUse := int(time.Since(*lastUsed).Hours() / 24)

	// Daily: used in last 7 days and high frequency
	if daysSinceLastUse <= 7 && usesPerDay >= 0.5 {
		return "daily"
	}

	// Weekly: used in last 30 days
	if daysSinceLastUse <= 30 {
		return "weekly"
	}

	// Monthly: used in last 90 days
	if daysSinceLastUse <= 90 {
		return "monthly"
	}

	// If used but not recently, still classify by frequency
	if usesPerDay >= 0.5 {
		return "daily"
	} else if usesPerDay >= 0.1 {
		return "weekly"
	} else if usesPerDay > 0 {
		return "monthly"
	}

	return "never"
}

// GetUsageTrends returns usage statistics for all packages over the specified time window.
func (a *Analyzer) GetUsageTrends(days int) (map[string]*UsageStats, error) {
	packages, err := a.store.ListPackages()
	if err != nil {
		return nil, fmt.Errorf("failed to list packages: %w", err)
	}

	trends := make(map[string]*UsageStats)

	for _, pkg := range packages {
		stats, err := a.GetUsageStats(pkg.Name)
		if err != nil {
			// Skip packages with errors
			continue
		}

		// Filter by time window if LastUsed is within range
		if stats.LastUsed != nil {
			daysSince := int(time.Since(*stats.LastUsed).Hours() / 24)
			if daysSince > days {
				// Outside the time window, but include with zero recent uses
				// Recalculate stats for the time window
				windowStats, err := a.getUsageStatsInWindow(pkg.Name, days)
				if err != nil {
					continue
				}
				trends[pkg.Name] = windowStats
				continue
			}
		}

		trends[pkg.Name] = stats
	}

	return trends, nil
}

// getUsageStatsInWindow returns usage stats for a specific time window.
func (a *Analyzer) getUsageStatsInWindow(pkg string, days int) (*UsageStats, error) {
	pkgInfo, err := a.store.GetPackage(pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to get package: %w", err)
	}

	// Get events within the time window
	since := time.Now().AddDate(0, 0, -days)
	events, err := a.store.GetUsageEvents(pkg, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage events: %w", err)
	}

	stats := &UsageStats{
		Package:   pkg,
		TotalUses: len(events),
		FirstSeen: pkgInfo.InstalledAt,
		DaysSince: -1,
	}

	if len(events) > 0 {
		stats.LastUsed = &events[0].Timestamp
		stats.DaysSince = int(time.Since(*stats.LastUsed).Hours() / 24)
	}

	stats.Frequency = a.computeFrequency(stats.LastUsed, stats.TotalUses, pkgInfo.InstalledAt)

	return stats, nil
}
