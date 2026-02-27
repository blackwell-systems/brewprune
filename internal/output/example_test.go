package output_test

import (
	"fmt"
	"time"

	"github.com/blackwell-systems/brewprune/internal/brew"
	"github.com/blackwell-systems/brewprune/internal/output"
	"github.com/blackwell-systems/brewprune/internal/store"
)

// Example showing how to render a package table
func ExampleRenderPackageTable() {
	packages := []*brew.Package{
		{
			Name:        "node",
			Version:     "16.20.2",
			SizeBytes:   2147483648, // 2 GB
			InstalledAt: time.Now().Add(-142 * 24 * time.Hour),
		},
		{
			Name:        "postgresql",
			Version:     "14.10",
			SizeBytes:   933281792, // 890 MB
			InstalledAt: time.Now().Add(-89 * 24 * time.Hour),
		},
	}

	table := output.RenderPackageTable(packages)
	fmt.Println(table)
}

// Example showing how to use a progress bar
func ExampleProgressBar() {
	// Create a progress bar for 100 items
	progress := output.NewProgress(100, "Processing packages")

	// Simulate processing
	for i := 0; i < 100; i++ {
		// Do some work...
		progress.Increment()
	}

	// Mark as complete
	progress.Finish()
}

// Example showing how to use a spinner
func ExampleSpinner() {
	// Create and start a spinner
	spinner := output.NewSpinner("Analyzing dependencies")

	// Simulate some work
	time.Sleep(2 * time.Second)

	// Stop the spinner
	spinner.Stop()
	fmt.Println("Analysis complete!")
}

// Example showing how to render snapshot table
func ExampleRenderSnapshotTable() {
	snapshots := []*store.Snapshot{
		{
			ID:           1,
			CreatedAt:    time.Now().Add(-5 * time.Minute),
			Reason:       "Before removing node",
			PackageCount: 3,
		},
		{
			ID:           2,
			CreatedAt:    time.Now().Add(-1 * time.Hour),
			Reason:       "Weekly cleanup",
			PackageCount: 5,
		},
	}

	table := output.RenderSnapshotTable(snapshots)
	fmt.Println(table)
}

// Example showing confidence score rendering
func ExampleRenderConfidenceTable() {
	scores := []output.ConfidenceScore{
		{
			Package:  "node",
			Score:    85,
			Tier:     "safe",
			LastUsed: time.Time{}, // never used
			Reason:   "No usage in 90 days",
		},
		{
			Package:  "python",
			Score:    45,
			Tier:     "risky",
			LastUsed: time.Now().Add(-2 * time.Hour),
			Reason:   "Recently used",
		},
	}

	table := output.RenderConfidenceTable(scores)
	fmt.Println(table)
}

// Example showing usage statistics rendering
func ExampleRenderUsageTable() {
	stats := map[string]output.UsageStats{
		"git": {
			TotalRuns: 156,
			LastUsed:  time.Now().Add(-1 * time.Hour),
			Frequency: "daily",
			Trend:     "stable",
		},
		"python": {
			TotalRuns: 238,
			LastUsed:  time.Now().Add(-30 * time.Minute),
			Frequency: "daily",
			Trend:     "increasing",
		},
	}

	table := output.RenderUsageTable(stats)
	fmt.Println(table)
}
