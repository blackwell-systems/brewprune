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
			// Version column removed — pkg.Version is never populated from Homebrew metadata
			contains: []string{"node", "2.0 GB", "1 day ago"},
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
			// Version column removed — pkg.Version is never populated from Homebrew metadata
			contains: []string{"node", "zsh", "2.0 GB", "1 MB"},
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
			contains: []string{"node", "6 MB", "85/100", "0", "never", "\u2014", "✓ safe"},
		},
		{
			name: "risky score shows risky",
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
			contains: []string{"openssl@3", "79 MB", "30/100", "14 packages", "⚠ risky"},
		},
		{
			name: "critical score shows risky",
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
			contains: []string{"git", "63 MB", "40/100", "8", "5 packages", "⚠ risky"},
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
			contains: []string{"jq", "1 MB", "1", "\u2014", "~ medium"},
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
				"ripgrep", "90/100", "6 MB", "\u2014", "✓ safe",
				"openssl@3", "30/100", "79 MB", "14 packages", "⚠ risky",
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
		{"999 KB stays as KB", 999 * 1024, "999 KB"},  // 1023000 bytes < 1024000 → stays as KB
		{"1000 KB becomes 1 MB", 1000 * 1024, "1 MB"}, // 1024000 bytes = 1000 KB → 1 MB
		{"1004 KB becomes 1 MB", 1004 * 1024, "1 MB"}, // 1028096 bytes = 1004 KB → 1 MB
		{"1024 KB becomes 1 MB", 1024 * 1024, "1 MB"}, // Exact 1 MB
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

// TestRenderConfidenceTable_ScoreColumnPresent verifies the Score column appears
// in the header and that row scores are formatted as N/100.
func TestRenderConfidenceTable_ScoreColumnPresent(t *testing.T) {
	scores := []ConfidenceScore{
		{
			Package:   "wget",
			Score:     80,
			Tier:      "safe",
			LastUsed:  time.Time{},
			SizeBytes: 1048576,
			Uses7d:    2,
			DepCount:  0,
		},
	}

	result := RenderConfidenceTable(scores)

	if !strings.Contains(result, "Score") {
		t.Errorf("expected 'Score' column header in output, got:\n%s", result)
	}
	if !strings.Contains(result, "80/100") {
		t.Errorf("expected score formatted as '80/100' in output, got:\n%s", result)
	}
}

// TestRenderConfidenceTable_RiskyLabel verifies that a risky-tier package
// renders as "⚠ risky" (not "✗ keep").
func TestRenderConfidenceTable_RiskyLabel(t *testing.T) {
	scores := []ConfidenceScore{
		{
			Package:   "openssl@3",
			Score:     25,
			Tier:      "risky",
			LastUsed:  time.Now().Add(-48 * time.Hour),
			SizeBytes: 82837504,
			Uses7d:    0,
			DepCount:  12,
		},
	}

	result := RenderConfidenceTable(scores)

	if !strings.Contains(result, "⚠ risky") {
		t.Errorf("expected '⚠ risky' label for risky tier, got:\n%s", result)
	}
	if strings.Contains(result, "✗ keep") {
		t.Errorf("expected '✗ keep' to be gone, but still present in output:\n%s", result)
	}
}

// TestFormatTierLabel_Risky verifies formatTierLabel("risky", false) returns "⚠ risky".
func TestFormatTierLabel_Risky(t *testing.T) {
	got := formatTierLabel("risky", false)
	want := "⚠ risky"
	if got != want {
		t.Errorf("formatTierLabel(%q, false) = %q, want %q", "risky", got, want)
	}
}

// TestFormatTierLabel_Critical verifies formatTierLabel("risky", true) returns "⚠ risky".
func TestFormatTierLabel_Critical(t *testing.T) {
	got := formatTierLabel("risky", true)
	want := "⚠ risky"
	if got != want {
		t.Errorf("formatTierLabel(%q, true) = %q, want %q", "risky", got, want)
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

func TestRenderConfidenceTable_CaskDisplay(t *testing.T) {
	scores := []ConfidenceScore{
		{
			Package:   "firefox",
			Score:     85,
			Tier:      "safe",
			LastUsed:  time.Now().Add(-48 * time.Hour),
			SizeBytes: 209715200, // 200 MB
			Uses7d:    5,
			DepCount:  0,
			IsCask:    true,
		},
	}

	result := RenderConfidenceTable(scores)

	// IsCask=true should show "n/a" for both usage columns
	if !strings.Contains(result, "n/a") {
		t.Errorf("expected 'n/a' for cask usage columns, got:\n%s", result)
	}

	// Should NOT contain the numeric uses value or relative time
	if strings.Contains(result, "2 days ago") {
		t.Errorf("cask row should show 'n/a' instead of relative time, got:\n%s", result)
	}
}

func TestRenderTierSummary_ShowAll(t *testing.T) {
	safe := TierStats{Count: 5, SizeBytes: 45088768}      // ~43 MB
	medium := TierStats{Count: 19, SizeBytes: 195035136}  // ~186 MB
	risky := TierStats{Count: 143, SizeBytes: 4509715456} // ~4.2 GB

	result := RenderTierSummary(safe, medium, risky, true, 0)

	for _, want := range []string{"SAFE", "5 packages", "MEDIUM", "19", "RISKY", "143"} {
		if !strings.Contains(result, want) {
			t.Errorf("RenderTierSummary(showAll=true) missing %q\nGot: %s", want, result)
		}
	}

	// When showAll=true, risky should show size, not "hidden"
	if strings.Contains(result, "hidden") {
		t.Errorf("RenderTierSummary(showAll=true) should not contain 'hidden'\nGot: %s", result)
	}
}

func TestRenderTierSummary_HideRisky(t *testing.T) {
	safe := TierStats{Count: 5, SizeBytes: 45088768}
	medium := TierStats{Count: 19, SizeBytes: 195035136}
	risky := TierStats{Count: 143, SizeBytes: 4509715456}

	result := RenderTierSummary(safe, medium, risky, false, 0)

	if !strings.Contains(result, "hidden, use --all") {
		t.Errorf("RenderTierSummary(showAll=false) should contain 'hidden, use --all'\nGot: %s", result)
	}
}

func TestRenderReclaimableFooter_ShowAll(t *testing.T) {
	safe := TierStats{Count: 5, SizeBytes: 45088768}
	medium := TierStats{Count: 19, SizeBytes: 195035136}
	risky := TierStats{Count: 143, SizeBytes: 4509715456}

	result := RenderReclaimableFooter(safe, medium, risky, true)

	if !strings.Contains(result, "Reclaimable:") {
		t.Errorf("expected 'Reclaimable:' prefix, got: %s", result)
	}
	if !strings.Contains(result, "(safe)") || !strings.Contains(result, "(medium)") || !strings.Contains(result, "(risky)") {
		t.Errorf("expected all tier labels, got: %s", result)
	}
	if strings.Contains(result, "hidden") {
		t.Errorf("showAll=true should not contain 'hidden', got: %s", result)
	}
}

func TestRenderReclaimableFooter_HideRisky(t *testing.T) {
	safe := TierStats{Count: 5, SizeBytes: 45088768}
	medium := TierStats{Count: 19, SizeBytes: 195035136}
	risky := TierStats{Count: 143, SizeBytes: 4509715456}

	result := RenderReclaimableFooter(safe, medium, risky, false)

	if !strings.Contains(result, "risky, hidden") {
		t.Errorf("showAll=false should contain 'risky, hidden', got: %s", result)
	}
}

func TestRenderReclaimableFooterCumulative(t *testing.T) {
	safe := TierStats{Count: 5, SizeBytes: 40894464}     // ~39 MB
	medium := TierStats{Count: 19, SizeBytes: 188743680} // ~180 MB
	risky := TierStats{Count: 143, SizeBytes: 140509184} // ~134 MB

	result := RenderReclaimableFooterCumulative(safe, medium, risky)

	// Verify it contains "Reclaimable:" prefix
	if !strings.Contains(result, "Reclaimable:") {
		t.Errorf("expected 'Reclaimable:' prefix, got: %s", result)
	}

	// Verify it contains "safe" label (not in parens)
	if !strings.Contains(result, "safe,") {
		t.Errorf("expected 'safe,' in cumulative format, got: %s", result)
	}

	// Verify it contains "if medium included"
	if !strings.Contains(result, "if medium included") {
		t.Errorf("expected 'if medium included' phrase, got: %s", result)
	}

	// Verify it contains "total"
	if !strings.Contains(result, "total") {
		t.Errorf("expected 'total' in cumulative format, got: %s", result)
	}

	// Verify cumulative values are correct
	// safe: 39 MB, safe+medium: ~219 MB, total: ~353 MB
	safeSize := formatSize(safe.SizeBytes)
	mediumCumulative := formatSize(safe.SizeBytes + medium.SizeBytes)
	totalSize := formatSize(safe.SizeBytes + medium.SizeBytes + risky.SizeBytes)

	if !strings.Contains(result, safeSize) {
		t.Errorf("expected safe size %s in result, got: %s", safeSize, result)
	}
	if !strings.Contains(result, mediumCumulative) {
		t.Errorf("expected medium cumulative %s in result, got: %s", mediumCumulative, result)
	}
	if !strings.Contains(result, totalSize) {
		t.Errorf("expected total size %s in result, got: %s", totalSize, result)
	}
}

func TestFormatDepCount_Zero(t *testing.T) {
	got := formatDepCount(0)
	if got != "\u2014" {
		t.Errorf("formatDepCount(0) = %q, want %q", got, "\u2014")
	}
}

// TestIsColorEnabled_NoColor verifies that IsColorEnabled returns false when
// the NO_COLOR environment variable is set.
func TestIsColorEnabled_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if IsColorEnabled() {
		t.Error("IsColorEnabled() = true with NO_COLOR=1, want false")
	}
}

// TestIsColorEnabled_NonTTY verifies that IsColorEnabled returns false when
// stdout is not a terminal (e.g. during testing where stdout is a pipe/buffer).
func TestIsColorEnabled_NonTTY(t *testing.T) {
	// In a standard test run stdout is not a TTY, so IsColorEnabled must be false
	// (assuming NO_COLOR is not already set, but we clear it to be safe).
	t.Setenv("NO_COLOR", "")
	// os.Stdout during go test is a pipe, not a TTY.
	if IsColorEnabled() {
		t.Skip("stdout appears to be a TTY — skipping non-TTY assertion (running in interactive terminal?)")
	}
}

// TestRenderPackageTable_NoVersionColumn verifies that the "Version" column
// header is not present in RenderPackageTable output.
func TestRenderPackageTable_NoVersionColumn(t *testing.T) {
	packages := []*brew.Package{
		{
			Name:      "wget",
			Version:   "1.21.4",
			SizeBytes: 1048576,
		},
	}
	result := RenderPackageTable(packages)
	if strings.Contains(result, "Version") {
		t.Errorf("RenderPackageTable() should not contain 'Version' column header, got:\n%s", result)
	}
	if strings.Contains(result, "1.21.4") {
		t.Errorf("RenderPackageTable() should not contain version string '1.21.4', got:\n%s", result)
	}
}

// TestRenderUsageTable_SortedByRunsThenLastUsed verifies that packages with
// equal TotalRuns are ordered by LastUsed descending, with zero times last.
func TestRenderUsageTable_SortedByRunsThenLastUsed(t *testing.T) {
	now := time.Now()
	stats := map[string]UsageStats{
		"alpha": {
			TotalRuns: 10,
			LastUsed:  now.Add(-1 * time.Hour),
			Frequency: "daily",
			Trend:     "stable",
		},
		"beta": {
			TotalRuns: 10,
			LastUsed:  now.Add(-24 * time.Hour),
			Frequency: "daily",
			Trend:     "stable",
		},
		"gamma": {
			TotalRuns: 10,
			LastUsed:  time.Time{}, // zero — should sort last among equal-run entries
			Frequency: "rarely",
			Trend:     "stable",
		},
		"delta": {
			TotalRuns: 20,
			LastUsed:  now.Add(-2 * time.Hour),
			Frequency: "daily",
			Trend:     "up",
		},
	}

	result := RenderUsageTable(stats)

	// delta (20 runs) must appear before alpha/beta/gamma (10 runs each)
	idxDelta := strings.Index(result, "delta")
	idxAlpha := strings.Index(result, "alpha")
	idxBeta := strings.Index(result, "beta")
	idxGamma := strings.Index(result, "gamma")

	if idxDelta == -1 || idxAlpha == -1 || idxBeta == -1 || idxGamma == -1 {
		t.Fatalf("Expected all packages in output, got:\n%s", result)
	}
	if idxDelta > idxAlpha {
		t.Errorf("delta (20 runs) should appear before alpha (10 runs), got:\n%s", result)
	}
	// alpha (1h ago) must appear before beta (24h ago)
	if idxAlpha > idxBeta {
		t.Errorf("alpha (1h ago) should appear before beta (24h ago) when equal runs, got:\n%s", result)
	}
	// beta (24h ago) must appear before gamma (zero time)
	if idxBeta > idxGamma {
		t.Errorf("beta (24h ago) should appear before gamma (never) when equal runs, got:\n%s", result)
	}
}

// TestTierSummaryColorCoded verifies that RenderTierSummary applies ANSI
// color codes to SAFE/MEDIUM/RISKY labels when IsColorEnabled() is true.
// Note: IsColorEnabled() checks both TTY status and NO_COLOR env var,
// so this test only verifies the output format when colors are enabled.
func TestTierSummaryColorCoded(t *testing.T) {
	// Skip if NO_COLOR is set or stdout is not a TTY
	if !IsColorEnabled() {
		t.Skip("Skipping color test: colors disabled (NO_COLOR set or stdout not a TTY)")
	}

	safe := TierStats{Count: 5, SizeBytes: 45088768}
	medium := TierStats{Count: 19, SizeBytes: 195035136}
	risky := TierStats{Count: 143, SizeBytes: 4509715456}

	result := RenderTierSummary(safe, medium, risky, true, 0)

	// Verify color codes are present
	if !strings.Contains(result, colorGreen+"SAFE"+colorReset) {
		t.Errorf("Expected SAFE to be wrapped in green color codes, got: %s", result)
	}
	if !strings.Contains(result, colorYellow+"MEDIUM"+colorReset) {
		t.Errorf("Expected MEDIUM to be wrapped in yellow color codes, got: %s", result)
	}
	if !strings.Contains(result, colorRed+"RISKY"+colorReset) {
		t.Errorf("Expected RISKY to be wrapped in red color codes, got: %s", result)
	}
}

// TestTierSummaryPlainTextWhenNoTTY verifies that RenderTierSummary does NOT
// include ANSI color codes when stdout is not a terminal (e.g., piped output).
// This test simulates non-TTY behavior by setting NO_COLOR.
func TestTierSummaryPlainTextWhenNoTTY(t *testing.T) {
	// Force colors off
	t.Setenv("NO_COLOR", "1")

	safe := TierStats{Count: 5, SizeBytes: 45088768}
	medium := TierStats{Count: 19, SizeBytes: 195035136}
	risky := TierStats{Count: 143, SizeBytes: 4509715456}

	result := RenderTierSummary(safe, medium, risky, true, 0)

	// Verify NO color codes are present
	if strings.Contains(result, "\033[") {
		t.Errorf("Expected no ANSI color codes when NO_COLOR=1, got: %s", result)
	}

	// Verify plain text labels are present
	if !strings.Contains(result, "SAFE: 5 packages") {
		t.Errorf("Expected plain 'SAFE: 5 packages' label, got: %s", result)
	}
	if !strings.Contains(result, "MEDIUM: 19") {
		t.Errorf("Expected plain 'MEDIUM: 19' label, got: %s", result)
	}
	if !strings.Contains(result, "RISKY: 143") {
		t.Errorf("Expected plain 'RISKY: 143' label, got: %s", result)
	}
}
