package output

import (
	"strings"
	"testing"
	"time"

	"github.com/blackwell-systems/brewprune/internal/brew"
	"github.com/blackwell-systems/brewprune/internal/store"
)

func TestRenderPackageTable(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		packages []*brew.Package
		contains []string
	}{
		{
			name:     "empty packages",
			packages: []*brew.Package{},
			contains: []string{"No packages found"},
		},
		{
			name: "single package",
			packages: []*brew.Package{
				{
					Name:        "node",
					Version:     "16.20.2",
					SizeBytes:   2147483648, // 2 GB
					InstalledAt: now.Add(-24 * time.Hour),
				},
			},
			contains: []string{"node", "16.20.2", "2.0 GB", "1 day ago"},
		},
		{
			name: "multiple packages sorted by name",
			packages: []*brew.Package{
				{
					Name:        "zsh",
					Version:     "5.9",
					SizeBytes:   1048576, // 1 MB
					InstalledAt: now.Add(-48 * time.Hour),
				},
				{
					Name:        "node",
					Version:     "16.20.2",
					SizeBytes:   2147483648, // 2 GB
					InstalledAt: now.Add(-24 * time.Hour),
				},
			},
			contains: []string{"node", "zsh", "16.20.2", "5.9", "2.0 GB", "1 MB"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderPackageTable(tt.packages)

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("RenderPackageTable() missing expected string %q\nGot:\n%s", expected, result)
				}
			}
		})
	}
}

func TestRenderConfidenceTable(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		scores   []ConfidenceScore
		contains []string
	}{
		{
			name:     "empty scores",
			scores:   []ConfidenceScore{},
			contains: []string{"No confidence scores"},
		},
		{
			name: "single safe score",
			scores: []ConfidenceScore{
				{
					Package:   "node",
					Score:     85,
					Tier:      "safe",
					LastUsed:  time.Time{}, // zero time = never
					SizeBytes: 6291456,     // 6 MB
					Uses7d:    0,
					DepCount:  0,
				},
			},
			contains: []string{"node", "6 MB", "0", "never", "0 packages", "SAFE"},
		},
		{
			name: "risky score shows keep",
			scores: []ConfidenceScore{
				{
					Package:   "openssl@3",
					Score:     30,
					Tier:      "risky",
					LastUsed:  now.Add(-2 * time.Hour),
					SizeBytes: 82837504, // 79 MB
					Uses7d:    0,
					DepCount:  14,
				},
			},
			contains: []string{"openssl@3", "79 MB", "14 packages", "keep"},
		},
		{
			name: "critical score shows keep",
			scores: []ConfidenceScore{
				{
					Package:    "git",
					Score:      40,
					Tier:       "risky",
					LastUsed:   now.Add(-24 * time.Minute),
					SizeBytes:  66060288, // 63 MB
					Uses7d:     8,
					DepCount:   5,
					IsCritical: true,
				},
			},
			contains: []string{"git", "63 MB", "8", "5 packages", "keep"},
		},
		{
			name: "medium score",
			scores: []ConfidenceScore{
				{
					Package:   "jq",
					Score:     65,
					Tier:      "medium",
					LastUsed:  now.Add(-24 * time.Minute),
					SizeBytes: 1048576, // 1 MB
					Uses7d:    1,
					DepCount:  0,
				},
			},
			contains: []string{"jq", "1 MB", "1", "0 packages", "MEDIUM"},
		},
		{
			name: "multiple scores with new columns",
			scores: []ConfidenceScore{
				{
					Package:   "ripgrep",
					Score:     90,
					Tier:      "safe",
					LastUsed:  time.Time{},
					SizeBytes: 6291456,
					Uses7d:    0,
					DepCount:  0,
				},
				{
					Package:    "openssl@3",
					Score:      30,
					Tier:       "risky",
					LastUsed:   time.Time{},
					SizeBytes:  82837504,
					Uses7d:     0,
					DepCount:   14,
					IsCritical: true,
				},
			},
			contains: []string{
				"ripgrep", "6 MB", "0 packages", "SAFE",
				"openssl@3", "79 MB", "14 packages", "keep",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderConfidenceTable(tt.scores)

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("RenderConfidenceTable() missing expected string %q\nGot:\n%s", expected, result)
				}
			}
		})
	}
}

func TestRenderUsageTable(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		stats    map[string]UsageStats
		contains []string
	}{
		{
			name:     "empty stats",
			stats:    map[string]UsageStats{},
			contains: []string{"No usage statistics"},
		},
		{
			name: "single package stats",
			stats: map[string]UsageStats{
				"git": {
					TotalRuns: 156,
					LastUsed:  now.Add(-1 * time.Hour),
					Frequency: "daily",
					Trend:     "stable",
				},
			},
			contains: []string{"git", "156", "1 hour ago", "daily", "→"},
		},
		{
			name: "multiple packages sorted by runs",
			stats: map[string]UsageStats{
				"git": {
					TotalRuns: 156,
					LastUsed:  now.Add(-1 * time.Hour),
					Frequency: "daily",
					Trend:     "stable",
				},
				"python": {
					TotalRuns: 238,
					LastUsed:  now.Add(-30 * time.Minute),
					Frequency: "daily",
					Trend:     "increasing",
				},
				"node": {
					TotalRuns: 89,
					LastUsed:  now.Add(-7 * 24 * time.Hour),
					Frequency: "weekly",
					Trend:     "decreasing",
				},
			},
			contains: []string{
				"python", "238", "↑",
				"git", "156", "→",
				"node", "89", "↓",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderUsageTable(tt.stats)

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("RenderUsageTable() missing expected string %q\nGot:\n%s", expected, result)
				}
			}
		})
	}
}

func TestRenderSnapshotTable(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		snapshots []*store.Snapshot
		contains  []string
	}{
		{
			name:      "empty snapshots",
			snapshots: []*store.Snapshot{},
			contains:  []string{"No snapshots found"},
		},
		{
			name: "single snapshot",
			snapshots: []*store.Snapshot{
				{
					ID:           1,
					CreatedAt:    now.Add(-5 * time.Minute),
					Reason:       "Before removing node",
					PackageCount: 3,
					SnapshotPath: "/path/to/snapshot.json",
				},
			},
			contains: []string{"1", "5 minutes ago", "3", "Before removing node"},
		},
		{
			name: "multiple snapshots sorted by time",
			snapshots: []*store.Snapshot{
				{
					ID:           1,
					CreatedAt:    now.Add(-7 * 24 * time.Hour),
					Reason:       "Weekly cleanup",
					PackageCount: 5,
					SnapshotPath: "/path/to/snapshot1.json",
				},
				{
					ID:           2,
					CreatedAt:    now.Add(-1 * 24 * time.Hour),
					Reason:       "Before removing postgresql",
					PackageCount: 2,
					SnapshotPath: "/path/to/snapshot2.json",
				},
				{
					ID:           3,
					CreatedAt:    now.Add(-1 * time.Hour),
					Reason:       "Test snapshot",
					PackageCount: 1,
					SnapshotPath: "/path/to/snapshot3.json",
				},
			},
			contains: []string{
				"3", "1 hour ago", "Test snapshot",
				"2", "1 day ago", "Before removing postgresql",
				"1", "1 week ago", "Weekly cleanup",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderSnapshotTable(tt.snapshots)

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("RenderSnapshotTable() missing expected string %q\nGot:\n%s", expected, result)
				}
			}
		})
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"bytes", 512, "512 B"},
		{"kilobytes", 1024, "1 KB"},
		{"kilobytes rounded", 1536, "2 KB"},
		{"megabytes", 1048576, "1 MB"},
		{"megabytes rounded", 10485760, "10 MB"},
		{"gigabytes", 1073741824, "1.0 GB"},
		{"gigabytes with decimal", 2147483648, "2.0 GB"},
		{"large gigabytes", 10737418240, "10.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSize(tt.bytes)
			if got != tt.want {
				t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		time time.Time
		want string
	}{
		{"zero time", time.Time{}, "never"},
		{"just now", now.Add(-30 * time.Second), "just now"},
		{"one minute ago", now.Add(-1 * time.Minute), "1 minute ago"},
		{"minutes ago", now.Add(-45 * time.Minute), "45 minutes ago"},
		{"one hour ago", now.Add(-1 * time.Hour), "1 hour ago"},
		{"hours ago", now.Add(-3 * time.Hour), "3 hours ago"},
		{"one day ago", now.Add(-24 * time.Hour), "1 day ago"},
		{"days ago", now.Add(-5 * 24 * time.Hour), "5 days ago"},
		{"one week ago", now.Add(-7 * 24 * time.Hour), "1 week ago"},
		{"weeks ago", now.Add(-14 * 24 * time.Hour), "2 weeks ago"},
		{"one month ago", now.Add(-30 * 24 * time.Hour), "1 month ago"},
		{"months ago", now.Add(-90 * 24 * time.Hour), "3 months ago"},
		{"one year ago", now.Add(-365 * 24 * time.Hour), "1 year ago"},
		{"years ago", now.Add(-730 * 24 * time.Hour), "2 years ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRelativeTime(tt.time)
			if got != tt.want {
				t.Errorf("formatRelativeTime() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatTier(t *testing.T) {
	tests := []struct {
		tier string
		want string
	}{
		{"safe", "SAFE"},
		{"medium", "MEDIUM"},
		{"risky", "RISKY"},
		{"SAFE", "SAFE"},
	}

	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			got := formatTier(tt.tier)
			if got != tt.want {
				t.Errorf("formatTier(%q) = %q, want %q", tt.tier, got, tt.want)
			}
		})
	}
}

func TestGetTierColor(t *testing.T) {
	tests := []struct {
		tier string
		want string
	}{
		{"safe", colorGreen},
		{"SAFE", colorGreen},
		{"medium", colorYellow},
		{"MEDIUM", colorYellow},
		{"risky", colorRed},
		{"RISKY", colorRed},
		{"unknown", colorGray},
	}

	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			got := getTierColor(tt.tier)
			if got != tt.want {
				t.Errorf("getTierColor(%q) = %q, want %q", tt.tier, got, tt.want)
			}
		})
	}
}

func TestFormatTrend(t *testing.T) {
	tests := []struct {
		trend string
		want  string
	}{
		{"up", "↑"},
		{"increasing", "↑"},
		{"down", "↓"},
		{"decreasing", "↓"},
		{"stable", "→"},
		{"unknown", "—"},
		{"", "—"},
	}

	for _, tt := range tests {
		t.Run(tt.trend, func(t *testing.T) {
			got := formatTrend(tt.trend)
			if got != tt.want {
				t.Errorf("formatTrend(%q) = %q, want %q", tt.trend, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"shorter than max", "hello", 10, "hello"},
		{"equal to max", "hello", 5, "hello"},
		{"longer than max", "hello world", 8, "hello..."},
		{"very short max", "hello", 2, "he"},
		{"max of 3", "hello", 3, "hel"},
		{"max of 4", "hello world", 4, "h..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

// Visual test - prints actual table output for manual verification
func TestVisualPackageTable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping visual test in short mode")
	}

	now := time.Now()
	packages := []*brew.Package{
		{
			Name:        "node",
			Version:     "16.20.2",
			SizeBytes:   2147483648, // 2 GB
			InstalledAt: now.Add(-142 * 24 * time.Hour),
		},
		{
			Name:        "postgresql@14",
			Version:     "14.10",
			SizeBytes:   933281792, // 890 MB
			InstalledAt: now.Add(-89 * 24 * time.Hour),
		},
		{
			Name:        "python@3.12",
			Version:     "3.12.1",
			SizeBytes:   52428800, // 50 MB
			InstalledAt: now.Add(-30 * 24 * time.Hour),
		},
	}

	t.Log("\n" + RenderPackageTable(packages))
}

// Visual test - prints actual confidence table for manual verification
func TestVisualConfidenceTable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping visual test in short mode")
	}

	now := time.Now()
	scores := []ConfidenceScore{
		{
			Package:   "ripgrep",
			Score:     90,
			Tier:      "safe",
			LastUsed:  time.Time{},
			SizeBytes: 6291456,
			Uses7d:    0,
			DepCount:  0,
		},
		{
			Package:   "jq",
			Score:     65,
			Tier:      "medium",
			LastUsed:  now.Add(-24 * time.Minute),
			SizeBytes: 1048576,
			Uses7d:    1,
			DepCount:  0,
		},
		{
			Package:    "openssl@3",
			Score:      30,
			Tier:       "risky",
			LastUsed:   time.Time{},
			SizeBytes:  82837504,
			Uses7d:     0,
			DepCount:   14,
			IsCritical: true,
		},
	}

	t.Log("\n" + RenderConfidenceTable(scores))
}
