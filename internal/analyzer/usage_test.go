package analyzer

import (
	"testing"
	"time"

	"github.com/blackwell-systems/brewprune/internal/brew"
	"github.com/blackwell-systems/brewprune/internal/store"
)

func TestGetUsageStats_NeverUsed(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	installedAt := time.Now().AddDate(0, 0, -100)
	pkg := &brew.Package{
		Name:        "unused_pkg",
		Version:     "1.0.0",
		InstalledAt: installedAt,
		InstallType: "explicit",
		HasBinary:   true,
		SizeBytes:   1000000,
	}

	if err := s.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert package: %v", err)
	}

	analyzer := New(s)
	stats, err := analyzer.GetUsageStats("unused_pkg")
	if err != nil {
		t.Fatalf("GetUsageStats failed: %v", err)
	}

	if stats.Package != "unused_pkg" {
		t.Errorf("expected Package unused_pkg, got %s", stats.Package)
	}
	if stats.TotalUses != 0 {
		t.Errorf("expected TotalUses 0, got %d", stats.TotalUses)
	}
	if stats.LastUsed != nil {
		t.Errorf("expected LastUsed nil, got %v", stats.LastUsed)
	}
	if stats.DaysSince != -1 {
		t.Errorf("expected DaysSince -1, got %d", stats.DaysSince)
	}
	if stats.Frequency != "never" {
		t.Errorf("expected Frequency never, got %s", stats.Frequency)
	}
	// Compare timestamps with truncation to second since SQLite may lose precision
	if !stats.FirstSeen.Truncate(time.Second).Equal(installedAt.Truncate(time.Second)) {
		t.Errorf("expected FirstSeen %v, got %v", installedAt, stats.FirstSeen)
	}
}

func TestGetUsageStats_DailyUsage(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	installedAt := time.Now().AddDate(0, 0, -30)
	pkg := &brew.Package{
		Name:        "git",
		Version:     "2.43.0",
		InstalledAt: installedAt,
		InstallType: "explicit",
		HasBinary:   true,
		SizeBytes:   50000000,
	}

	if err := s.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert package: %v", err)
	}

	// Add multiple usage events
	for i := 0; i < 20; i++ {
		event := &store.UsageEvent{
			Package:   "git",
			EventType: "exec",
			Timestamp: time.Now().AddDate(0, 0, -i),
		}
		if err := s.InsertUsageEvent(event); err != nil {
			t.Fatalf("failed to insert usage event: %v", err)
		}
	}

	analyzer := New(s)
	stats, err := analyzer.GetUsageStats("git")
	if err != nil {
		t.Fatalf("GetUsageStats failed: %v", err)
	}

	if stats.TotalUses != 20 {
		t.Errorf("expected TotalUses 20, got %d", stats.TotalUses)
	}
	if stats.LastUsed == nil {
		t.Fatal("expected LastUsed not nil")
	}
	if stats.DaysSince < 0 || stats.DaysSince > 1 {
		t.Errorf("expected DaysSince 0-1, got %d", stats.DaysSince)
	}
	if stats.Frequency != "daily" {
		t.Errorf("expected Frequency daily, got %s", stats.Frequency)
	}
}

func TestGetUsageStats_WeeklyUsage(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	installedAt := time.Now().AddDate(0, 0, -60)
	pkg := &brew.Package{
		Name:        "node",
		Version:     "20.0.0",
		InstalledAt: installedAt,
		InstallType: "explicit",
		HasBinary:   true,
		SizeBytes:   80000000,
	}

	if err := s.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert package: %v", err)
	}

	// Add usage events - once per week
	for i := 0; i < 8; i++ {
		event := &store.UsageEvent{
			Package:   "node",
			EventType: "exec",
			Timestamp: time.Now().AddDate(0, 0, -(i * 7)),
		}
		if err := s.InsertUsageEvent(event); err != nil {
			t.Fatalf("failed to insert usage event: %v", err)
		}
	}

	analyzer := New(s)
	stats, err := analyzer.GetUsageStats("node")
	if err != nil {
		t.Fatalf("GetUsageStats failed: %v", err)
	}

	if stats.TotalUses != 8 {
		t.Errorf("expected TotalUses 8, got %d", stats.TotalUses)
	}
	if stats.Frequency != "daily" && stats.Frequency != "weekly" {
		t.Errorf("expected Frequency daily or weekly, got %s", stats.Frequency)
	}
}

func TestGetUsageStats_MonthlyUsage(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	installedAt := time.Now().AddDate(0, 0, -180)
	pkg := &brew.Package{
		Name:        "terraform",
		Version:     "1.6.0",
		InstalledAt: installedAt,
		InstallType: "explicit",
		HasBinary:   true,
		SizeBytes:   30000000,
	}

	if err := s.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert package: %v", err)
	}

	// Add usage events - last used 60 days ago, few times
	for i := 0; i < 3; i++ {
		event := &store.UsageEvent{
			Package:   "terraform",
			EventType: "exec",
			Timestamp: time.Now().AddDate(0, 0, -(60 + i*10)),
		}
		if err := s.InsertUsageEvent(event); err != nil {
			t.Fatalf("failed to insert usage event: %v", err)
		}
	}

	analyzer := New(s)
	stats, err := analyzer.GetUsageStats("terraform")
	if err != nil {
		t.Fatalf("GetUsageStats failed: %v", err)
	}

	if stats.TotalUses != 3 {
		t.Errorf("expected TotalUses 3, got %d", stats.TotalUses)
	}
	if stats.Frequency != "monthly" {
		t.Errorf("expected Frequency monthly, got %s", stats.Frequency)
	}
}

func TestGetUsageTrends(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	// Insert multiple packages with different usage patterns
	packages := []struct {
		name      string
		daysAgo   int
		numEvents int
		lastUsed  int // days ago
	}{
		{"daily_pkg", 100, 50, 1},
		{"weekly_pkg", 90, 10, 20},
		{"monthly_pkg", 120, 3, 60},
		{"never_pkg", 80, 0, 0},
	}

	for _, p := range packages {
		pkg := &brew.Package{
			Name:        p.name,
			Version:     "1.0.0",
			InstalledAt: time.Now().AddDate(0, 0, -p.daysAgo),
			InstallType: "explicit",
			HasBinary:   true,
			SizeBytes:   1000000,
		}
		if err := s.InsertPackage(pkg); err != nil {
			t.Fatalf("failed to insert package: %v", err)
		}

		// Add usage events
		for i := 0; i < p.numEvents; i++ {
			event := &store.UsageEvent{
				Package:   p.name,
				EventType: "exec",
				Timestamp: time.Now().AddDate(0, 0, -(p.lastUsed + i)),
			}
			if err := s.InsertUsageEvent(event); err != nil {
				t.Fatalf("failed to insert usage event: %v", err)
			}
		}
	}

	analyzer := New(s)
	trends, err := analyzer.GetUsageTrends(90)
	if err != nil {
		t.Fatalf("GetUsageTrends failed: %v", err)
	}

	// Should have all packages
	if len(trends) != 4 {
		t.Errorf("expected 4 packages in trends, got %d", len(trends))
	}

	// Check daily package
	if stats, ok := trends["daily_pkg"]; ok {
		if stats.Frequency != "daily" {
			t.Errorf("expected daily_pkg frequency daily, got %s", stats.Frequency)
		}
	} else {
		t.Error("daily_pkg not in trends")
	}

	// Check never used package
	if stats, ok := trends["never_pkg"]; ok {
		if stats.Frequency != "never" {
			t.Errorf("expected never_pkg frequency never, got %s", stats.Frequency)
		}
	} else {
		t.Error("never_pkg not in trends")
	}
}

func TestGetUsageTrends_FilterByTimeWindow(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	// Package used recently
	recent := &brew.Package{
		Name:        "recent_pkg",
		Version:     "1.0.0",
		InstalledAt: time.Now().AddDate(0, 0, -100),
		InstallType: "explicit",
		HasBinary:   true,
		SizeBytes:   1000000,
	}
	if err := s.InsertPackage(recent); err != nil {
		t.Fatalf("failed to insert package: %v", err)
	}

	// Event within 30 days
	recentEvent := &store.UsageEvent{
		Package:   "recent_pkg",
		EventType: "exec",
		Timestamp: time.Now().AddDate(0, 0, -10),
	}
	if err := s.InsertUsageEvent(recentEvent); err != nil {
		t.Fatalf("failed to insert usage event: %v", err)
	}

	// Package used long ago
	old := &brew.Package{
		Name:        "old_pkg",
		Version:     "1.0.0",
		InstalledAt: time.Now().AddDate(0, 0, -200),
		InstallType: "explicit",
		HasBinary:   true,
		SizeBytes:   1000000,
	}
	if err := s.InsertPackage(old); err != nil {
		t.Fatalf("failed to insert package: %v", err)
	}

	// Event outside 30 days
	oldEvent := &store.UsageEvent{
		Package:   "old_pkg",
		EventType: "exec",
		Timestamp: time.Now().AddDate(0, 0, -100),
	}
	if err := s.InsertUsageEvent(oldEvent); err != nil {
		t.Fatalf("failed to insert usage event: %v", err)
	}

	analyzer := New(s)
	trends, err := analyzer.GetUsageTrends(30)
	if err != nil {
		t.Fatalf("GetUsageTrends failed: %v", err)
	}

	// Should include both packages
	if len(trends) != 2 {
		t.Errorf("expected 2 packages, got %d", len(trends))
	}

	// Recent package should show usage
	if stats, ok := trends["recent_pkg"]; ok {
		if stats.TotalUses == 0 {
			t.Error("expected recent_pkg to have usage")
		}
	}

	// Old package should show no usage in window
	if stats, ok := trends["old_pkg"]; ok {
		if stats.TotalUses != 0 {
			t.Errorf("expected old_pkg to have 0 uses in 30-day window, got %d", stats.TotalUses)
		}
	}
}

func TestComputeFrequency(t *testing.T) {
	analyzer := New(nil) // Don't need store for this test

	installedAt := time.Now().AddDate(0, 0, -100)

	tests := []struct {
		name              string
		lastUsed          *time.Time
		totalUses         int
		expectedFrequency string
	}{
		{
			name:              "never_used",
			lastUsed:          nil,
			totalUses:         0,
			expectedFrequency: "never",
		},
		{
			name:              "daily_recent_high_frequency",
			lastUsed:          ptr(time.Now().AddDate(0, 0, -3)),
			totalUses:         50,
			expectedFrequency: "daily",
		},
		{
			name:              "weekly_recent",
			lastUsed:          ptr(time.Now().AddDate(0, 0, -15)),
			totalUses:         10,
			expectedFrequency: "weekly",
		},
		{
			name:              "monthly_recent",
			lastUsed:          ptr(time.Now().AddDate(0, 0, -60)),
			totalUses:         3,
			expectedFrequency: "monthly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frequency := analyzer.computeFrequency(tt.lastUsed, tt.totalUses, installedAt)
			if frequency != tt.expectedFrequency {
				t.Errorf("expected frequency %s, got %s", tt.expectedFrequency, frequency)
			}
		})
	}
}
