package analyzer

import (
	"fmt"
	"testing"
	"time"

	"github.com/blackwell-systems/brewprune/internal/brew"
	"github.com/blackwell-systems/brewprune/internal/store"
)

func setupTestStore(t *testing.T) *store.Store {
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}

	if err := s.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	return s
}

func TestComputeScore_NeverUsedLeafPackage(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	// Insert a package that was never used, installed 200 days ago, has binaries, no dependents
	pkg := &brew.Package{
		Name:        "htop",
		Version:     "3.2.2",
		InstalledAt: time.Now().AddDate(0, 0, -200),
		InstallType: "explicit",
		HasBinary:   true,
		SizeBytes:   2000000,
	}

	if err := s.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert package: %v", err)
	}

	analyzer := New(s)
	score, err := analyzer.ComputeScore("htop")
	if err != nil {
		t.Fatalf("ComputeScore failed: %v", err)
	}

	// Expected scores:
	// Usage: 0 (never used)
	// Deps: 30 (no dependents)
	// Age: 20 (>180 days)
	// Type: 10 (leaf with binaries)
	// Total: 60

	if score.UsageScore != 0 {
		t.Errorf("expected UsageScore 0, got %d", score.UsageScore)
	}
	if score.DepsScore != 30 {
		t.Errorf("expected DepsScore 30, got %d", score.DepsScore)
	}
	if score.AgeScore != 20 {
		t.Errorf("expected AgeScore 20, got %d", score.AgeScore)
	}
	if score.TypeScore != 10 {
		t.Errorf("expected TypeScore 10, got %d", score.TypeScore)
	}
	if score.Score != 60 {
		t.Errorf("expected total Score 60, got %d", score.Score)
	}
	if score.Tier != "medium" {
		t.Errorf("expected tier medium, got %s", score.Tier)
	}
}

func TestComputeScore_RecentlyUsedPackage(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	pkg := &brew.Package{
		Name:        "jq",
		Version:     "1.7.0",
		InstalledAt: time.Now().AddDate(0, 0, -100),
		InstallType: "explicit",
		HasBinary:   true,
		SizeBytes:   500000,
	}

	if err := s.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert package: %v", err)
	}

	// Add recent usage event
	event := &store.UsageEvent{
		Package:   "jq",
		EventType: "exec",
		Timestamp: time.Now().AddDate(0, 0, -3),
	}
	if err := s.InsertUsageEvent(event); err != nil {
		t.Fatalf("failed to insert usage event: %v", err)
	}

	analyzer := New(s)
	score, err := analyzer.ComputeScore("jq")
	if err != nil {
		t.Fatalf("ComputeScore failed: %v", err)
	}

	// Expected scores:
	// Usage: 40 (used within 7 days)
	// Deps: 30 (no dependents)
	// Age: 15 (>90 days)
	// Type: 10 (leaf with binaries)
	// Total: 95

	if score.UsageScore != 40 {
		t.Errorf("expected UsageScore 40, got %d", score.UsageScore)
	}
	if score.Score != 95 {
		t.Errorf("expected total Score 95, got %d", score.Score)
	}
	if score.Tier != "safe" {
		t.Errorf("expected tier safe, got %s", score.Tier)
	}
}

func TestComputeScore_CoreDependency(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	pkg := &brew.Package{
		Name:        "openssl@3",
		Version:     "3.2.0",
		InstalledAt: time.Now().AddDate(0, 0, -200),
		InstallType: "dependency",
		HasBinary:   false,
		SizeBytes:   10000000,
	}

	if err := s.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert package: %v", err)
	}

	analyzer := New(s)
	score, err := analyzer.ComputeScore("openssl@3")
	if err != nil {
		t.Fatalf("ComputeScore failed: %v", err)
	}

	// Expected scores:
	// Usage: 0 (never used directly)
	// Deps: 30 (no dependents in test data)
	// Age: 20 (>180 days)
	// Type: 0 (core dependency)
	// Total: 50

	if score.TypeScore != 0 {
		t.Errorf("expected TypeScore 0 for core dependency, got %d", score.TypeScore)
	}
	if score.Score != 50 {
		t.Errorf("expected total Score 50, got %d", score.Score)
	}
	if score.Tier != "medium" {
		t.Errorf("expected tier medium, got %s", score.Tier)
	}
}

func TestComputeScore_WithDependents(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	// Insert a library package
	lib := &brew.Package{
		Name:        "libpng",
		Version:     "1.6.40",
		InstalledAt: time.Now().AddDate(0, 0, -150),
		InstallType: "dependency",
		HasBinary:   false,
		SizeBytes:   500000,
	}
	if err := s.InsertPackage(lib); err != nil {
		t.Fatalf("failed to insert library: %v", err)
	}

	// Insert packages that depend on the library
	for i := 1; i <= 5; i++ {
		dep := &brew.Package{
			Name:        "dependent" + string(rune('0'+i)),
			Version:     "1.0.0",
			InstalledAt: time.Now().AddDate(0, 0, -100),
			InstallType: "explicit",
			HasBinary:   true,
			SizeBytes:   1000000,
		}
		if err := s.InsertPackage(dep); err != nil {
			t.Fatalf("failed to insert dependent: %v", err)
		}

		// Create dependency relationship
		if err := s.InsertDependency(dep.Name, "libpng"); err != nil {
			t.Fatalf("failed to insert dependency: %v", err)
		}
	}

	analyzer := New(s)
	score, err := analyzer.ComputeScore("libpng")
	if err != nil {
		t.Fatalf("ComputeScore failed: %v", err)
	}

	// Expected scores:
	// Usage: 0 (never used directly)
	// Deps: 0 (5 dependents, 4+)
	// Age: 15 (>90 days)
	// Type: 5 (library with no binaries)
	// Total: 20

	if score.DepsScore != 0 {
		t.Errorf("expected DepsScore 0 for 5 dependents, got %d", score.DepsScore)
	}
	if score.Score != 20 {
		t.Errorf("expected total Score 20, got %d", score.Score)
	}
	if score.Tier != "risky" {
		t.Errorf("expected tier risky, got %s", score.Tier)
	}
}

func TestComputeScore_TierBoundaries(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	tests := []struct {
		name          string
		daysAgo       int
		hasDependent  bool
		expectedTier  string
		expectedScore int
	}{
		{"medium_boundary_60", 200, false, "medium", 60}, // 0+30+20+10=60 (medium, not safe)
		{"medium_boundary_50", 40, false, "medium", 50},  // 0+30+10+10=50
		{"risky_boundary_20", 20, true, "risky", 20},     // 0+10+0+10=20
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &brew.Package{
				Name:        tt.name,
				Version:     "1.0.0",
				InstalledAt: time.Now().AddDate(0, 0, -tt.daysAgo),
				InstallType: "explicit",
				HasBinary:   true,
				SizeBytes:   1000000,
			}
			if err := s.InsertPackage(pkg); err != nil {
				t.Fatalf("failed to insert package: %v", err)
			}

			if tt.hasDependent {
				dep := &brew.Package{
					Name:        tt.name + "_dep",
					Version:     "1.0.0",
					InstalledAt: time.Now().AddDate(0, 0, -50),
					InstallType: "explicit",
					HasBinary:   true,
					SizeBytes:   1000000,
				}
				if err := s.InsertPackage(dep); err != nil {
					t.Fatalf("failed to insert dependent: %v", err)
				}
				if err := s.InsertDependency(dep.Name, tt.name); err != nil {
					t.Fatalf("failed to insert dependency: %v", err)
				}
			}

			analyzer := New(s)
			score, err := analyzer.ComputeScore(tt.name)
			if err != nil {
				t.Fatalf("ComputeScore failed: %v", err)
			}

			if score.Score != tt.expectedScore {
				t.Errorf("expected score %d, got %d", tt.expectedScore, score.Score)
			}
			if score.Tier != tt.expectedTier {
				t.Errorf("expected tier %s, got %s", tt.expectedTier, score.Tier)
			}
		})
	}
}

func TestGetPackagesByTier(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	// Insert packages with different characteristics
	packages := []struct {
		name      string
		daysAgo   int
		hasBinary bool
		lastUsed  *time.Time
	}{
		{"safe_pkg_1", 200, true, nil},
		{"safe_pkg_2", 150, true, nil},
		{"medium_pkg", 50, false, nil},
		{"risky_pkg", 10, true, ptr(time.Now().AddDate(0, 0, -3))},
	}

	for _, p := range packages {
		pkg := &brew.Package{
			Name:        p.name,
			Version:     "1.0.0",
			InstalledAt: time.Now().AddDate(0, 0, -p.daysAgo),
			InstallType: "explicit",
			HasBinary:   p.hasBinary,
			SizeBytes:   1000000,
		}
		if err := s.InsertPackage(pkg); err != nil {
			t.Fatalf("failed to insert package: %v", err)
		}

		if p.lastUsed != nil {
			event := &store.UsageEvent{
				Package:   p.name,
				EventType: "exec",
				Timestamp: *p.lastUsed,
			}
			if err := s.InsertUsageEvent(event); err != nil {
				t.Fatalf("failed to insert usage event: %v", err)
			}
		}
	}

	analyzer := New(s)

	// Test getting safe packages
	safePackages, err := analyzer.GetPackagesByTier("safe")
	if err != nil {
		t.Fatalf("GetPackagesByTier(safe) failed: %v", err)
	}

	// We expect at least one safe package
	if len(safePackages) < 1 {
		t.Errorf("expected at least 1 safe package, got %d", len(safePackages))
	}

	// Verify all returned packages are indeed safe
	for _, score := range safePackages {
		if score.Tier != "safe" {
			t.Errorf("package %s has tier %s, expected safe", score.Package, score.Tier)
		}
	}
}

func TestGetPackagesByTier_InvalidTier(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	analyzer := New(s)
	_, err := analyzer.GetPackagesByTier("invalid")
	if err == nil {
		t.Error("expected error for invalid tier, got nil")
	}
}

func TestComputeScore_CriticalityPenalty(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	// Test that core dependencies get capped at 70 even with high scores
	tests := []struct {
		name          string
		pkg           string
		daysAgo       int
		expectedScore int
		expectedTier  string
		isCritical    bool
	}{
		{
			name:          "git_capped",
			pkg:           "git",
			daysAgo:       200,
			expectedScore: 70, // Would be 80+ without cap (0+30+20+10=60, but let's test never used)
			expectedTier:  "medium",
			isCritical:    true,
		},
		{
			name:          "openssl_capped",
			pkg:           "openssl@3",
			daysAgo:       200,
			expectedScore: 50, // 0+30+20+0=50 (TypeScore is 0 for core deps)
			expectedTier:  "medium",
			isCritical:    true,
		},
		{
			name:          "coreutils_capped",
			pkg:           "coreutils",
			daysAgo:       200,
			expectedScore: 60, // 0+30+20+10=60 (if it has binaries)
			expectedTier:  "medium",
			isCritical:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &brew.Package{
				Name:        tt.pkg,
				Version:     "1.0.0",
				InstalledAt: time.Now().AddDate(0, 0, -tt.daysAgo),
				InstallType: "explicit",
				HasBinary:   true,
				SizeBytes:   1000000,
			}
			if err := s.InsertPackage(pkg); err != nil {
				t.Fatalf("failed to insert package: %v", err)
			}

			analyzer := New(s)
			score, err := analyzer.ComputeScore(tt.pkg)
			if err != nil {
				t.Fatalf("ComputeScore failed: %v", err)
			}

			if score.IsCritical != tt.isCritical {
				t.Errorf("expected IsCritical=%v, got %v", tt.isCritical, score.IsCritical)
			}

			// For critical packages, score should be capped at 70
			if score.IsCritical && score.Score > 70 {
				t.Errorf("critical package score %d exceeds cap of 70", score.Score)
			}

			// Critical packages should never be in "safe" tier
			if score.IsCritical && score.Tier == "safe" {
				t.Errorf("critical package should never be in 'safe' tier, got: %s", score.Tier)
			}
		})
	}
}

func TestScoreExplanation_Fields(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	// Insert a test package
	pkg := &brew.Package{
		Name:        "test-pkg",
		Version:     "1.0.0",
		InstalledAt: time.Now().AddDate(0, 0, -45),
		InstallType: "explicit",
		HasBinary:   true,
		SizeBytes:   1000000,
	}
	if err := s.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert package: %v", err)
	}

	// Add usage event
	event := &store.UsageEvent{
		Package:   "test-pkg",
		EventType: "exec",
		Timestamp: time.Now().AddDate(0, 0, -10),
	}
	if err := s.InsertUsageEvent(event); err != nil {
		t.Fatalf("failed to insert usage event: %v", err)
	}

	analyzer := New(s)
	score, err := analyzer.ComputeScore("test-pkg")
	if err != nil {
		t.Fatalf("ComputeScore failed: %v", err)
	}

	// Test that Explanation fields are populated
	if score.Explanation.UsageDetail == "" {
		t.Error("UsageDetail should not be empty")
	}
	if score.Explanation.DepsDetail == "" {
		t.Error("DepsDetail should not be empty")
	}
	if score.Explanation.AgeDetail == "" {
		t.Error("AgeDetail should not be empty")
	}
	if score.Explanation.TypeDetail == "" {
		t.Error("TypeDetail should not be empty")
	}

	// Test specific content expectations
	if score.Explanation.UsageDetail != "last used 10 days ago" {
		t.Errorf("expected 'last used 10 days ago', got: %s", score.Explanation.UsageDetail)
	}
	if score.Explanation.DepsDetail != "no dependents" {
		t.Errorf("expected 'no dependents', got: %s", score.Explanation.DepsDetail)
	}
	if score.Explanation.AgeDetail != "installed 45 days ago" {
		t.Errorf("expected 'installed 45 days ago', got: %s", score.Explanation.AgeDetail)
	}
	if score.Explanation.TypeDetail != "leaf package with binaries" {
		t.Errorf("expected 'leaf package with binaries', got: %s", score.Explanation.TypeDetail)
	}
}

func TestScoreExplanation_CriticalPackage(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	pkg := &brew.Package{
		Name:        "cmake",
		Version:     "3.28.0",
		InstalledAt: time.Now().AddDate(0, 0, -100),
		InstallType: "dependency",
		HasBinary:   true,
		SizeBytes:   50000000,
	}
	if err := s.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert package: %v", err)
	}

	analyzer := New(s)
	score, err := analyzer.ComputeScore("cmake")
	if err != nil {
		t.Fatalf("ComputeScore failed: %v", err)
	}

	// Check that critical packages have the right TypeDetail
	if !score.IsCritical {
		t.Error("cmake should be marked as critical")
	}
	if score.Explanation.TypeDetail != "foundational package (reduced confidence)" {
		t.Errorf("expected 'foundational package (reduced confidence)', got: %s", score.Explanation.TypeDetail)
	}
}

func TestScoreExplanation_WithDependents(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	// Insert a library
	lib := &brew.Package{
		Name:        "test-lib",
		Version:     "1.0.0",
		InstalledAt: time.Now().AddDate(0, 0, -100),
		InstallType: "dependency",
		HasBinary:   false,
		SizeBytes:   500000,
	}
	if err := s.InsertPackage(lib); err != nil {
		t.Fatalf("failed to insert library: %v", err)
	}

	// Insert dependents
	for i := 1; i <= 3; i++ {
		dep := &brew.Package{
			Name:        fmt.Sprintf("dependent-%d", i),
			Version:     "1.0.0",
			InstalledAt: time.Now().AddDate(0, 0, -50),
			InstallType: "explicit",
			HasBinary:   true,
			SizeBytes:   1000000,
		}
		if err := s.InsertPackage(dep); err != nil {
			t.Fatalf("failed to insert dependent: %v", err)
		}
		if err := s.InsertDependency(dep.Name, "test-lib"); err != nil {
			t.Fatalf("failed to insert dependency: %v", err)
		}
	}

	analyzer := New(s)
	score, err := analyzer.ComputeScore("test-lib")
	if err != nil {
		t.Fatalf("ComputeScore failed: %v", err)
	}

	// Should show unused dependents
	if score.Explanation.DepsDetail != "3 unused dependents" {
		t.Errorf("expected '3 unused dependents', got: %s", score.Explanation.DepsDetail)
	}
}

func ptr(t time.Time) *time.Time {
	return &t
}
