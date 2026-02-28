package analyzer

import (
	"testing"
	"time"

	"github.com/blackwell-systems/brewprune/internal/brew"
	"github.com/blackwell-systems/brewprune/internal/store"
)

func TestGetRecommendations(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	// Note: to get safe tier (80-100), we need packages with usage within last 90 days
	// Usage 7d=40, Deps none=30, Age>90d=15, Type leaf+binary=10 = 95 (safe)
	// Usage 30d=30, Deps none=30, Age>180d=20, Type leaf+binary=10 = 90 (safe)

	// Insert packages with different safety levels
	packages := []struct {
		name        string
		daysAgo     int
		sizeBytes   int64
		hasBinary   bool
		usedDaysAgo int // 0 means never used
	}{
		{"large_safe", 200, 100000000, true, 20},    // 100MB - used 20d ago: 30+30+20+10=90
		{"medium_safe", 180, 50000000, true, 60},    // 50MB - used 60d ago: 20+30+20+10=80
		{"small_medium", 200, 1000000, true, 0},     // 1MB - never used: 0+30+20+10=60 (medium)
		{"risky_recent", 10, 80000000, true, 3},     // Recent install and use
		{"risky_no_binary", 200, 5000000, false, 0}, // Library
	}

	for _, p := range packages {
		pkg := &brew.Package{
			Name:        p.name,
			Version:     "1.0.0",
			InstalledAt: time.Now().AddDate(0, 0, -p.daysAgo),
			InstallType: "explicit",
			HasBinary:   p.hasBinary,
			SizeBytes:   p.sizeBytes,
		}
		if err := s.InsertPackage(pkg); err != nil {
			t.Fatalf("failed to insert package: %v", err)
		}

		// Add usage event if specified
		if p.usedDaysAgo > 0 {
			event := &store.UsageEvent{
				Package:   p.name,
				EventType: "exec",
				Timestamp: time.Now().AddDate(0, 0, -p.usedDaysAgo),
			}
			if err := s.InsertUsageEvent(event); err != nil {
				t.Fatalf("failed to insert usage event: %v", err)
			}
		}
	}

	analyzer := New(s)
	rec, err := analyzer.GetRecommendations()
	if err != nil {
		t.Fatalf("GetRecommendations failed: %v", err)
	}

	// Should have 2 safe packages
	if len(rec.Packages) != 2 {
		t.Errorf("expected 2 safe packages, got %d", len(rec.Packages))
	}

	if rec.Tier != "safe" {
		t.Errorf("expected tier safe, got %s", rec.Tier)
	}

	// Verify packages are sorted by size (largest first)
	if len(rec.Packages) >= 2 {
		firstPkg, _ := s.GetPackage(rec.Packages[0])
		secondPkg, _ := s.GetPackage(rec.Packages[1])
		if firstPkg.SizeBytes < secondPkg.SizeBytes {
			t.Error("packages not sorted by size (largest first)")
		}
	}

	// Verify total size calculation
	if rec.TotalSize == 0 {
		t.Error("expected non-zero total size")
	}
	if rec.ExpectedSavings != rec.TotalSize {
		t.Errorf("expected ExpectedSavings %d to equal TotalSize %d", rec.ExpectedSavings, rec.TotalSize)
	}
}

func TestGetRecommendations_NeverUsedPackageIsSafe(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	// A never-used, recently installed package scores 40+30+0+10=80 (safe)
	// Under corrected scoring: never used = high removal pressure = safe tier
	pkg := &brew.Package{
		Name:        "medium_pkg",
		Version:     "1.0.0",
		InstalledAt: time.Now().AddDate(0, 0, -20),
		InstallType: "explicit",
		HasBinary:   true,
		SizeBytes:   10000000,
	}
	if err := s.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert package: %v", err)
	}

	// No usage events: UsageScore=40, total=40+30+0+10=80 → safe tier
	analyzer := New(s)
	rec, err := analyzer.GetRecommendations()
	if err != nil {
		t.Fatalf("GetRecommendations failed: %v", err)
	}

	if len(rec.Packages) != 1 {
		t.Errorf("expected 1 safe package (never used), got %d", len(rec.Packages))
	}
	if rec.TotalSize != 10000000 {
		t.Errorf("expected total size 10000000, got %d", rec.TotalSize)
	}
}

func TestValidateRemoval_NoDependents(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	// Insert a safe package
	pkg := &brew.Package{
		Name:        "safe_pkg",
		Version:     "1.0.0",
		InstalledAt: time.Now().AddDate(0, 0, -200),
		InstallType: "explicit",
		HasBinary:   true,
		SizeBytes:   10000000,
	}
	if err := s.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert package: %v", err)
	}

	analyzer := New(s)
	warnings, err := analyzer.ValidateRemoval([]string{"safe_pkg"})
	if err != nil {
		t.Fatalf("ValidateRemoval failed: %v", err)
	}

	// Should have minimal warnings for explicit install
	foundExplicitWarning := false
	for _, w := range warnings {
		if w == "safe_pkg: explicitly installed (not a dependency)" {
			foundExplicitWarning = true
		}
	}
	if !foundExplicitWarning {
		t.Error("expected warning about explicit installation")
	}
}

func TestValidateRemoval_WithDependents(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	// Insert a library package
	lib := &brew.Package{
		Name:        "libfoo",
		Version:     "1.0.0",
		InstalledAt: time.Now().AddDate(0, 0, -100),
		InstallType: "dependency",
		HasBinary:   false,
		SizeBytes:   5000000,
	}
	if err := s.InsertPackage(lib); err != nil {
		t.Fatalf("failed to insert library: %v", err)
	}

	// Insert a package that depends on it
	app := &brew.Package{
		Name:        "myapp",
		Version:     "1.0.0",
		InstalledAt: time.Now().AddDate(0, 0, -100),
		InstallType: "explicit",
		HasBinary:   true,
		SizeBytes:   20000000,
	}
	if err := s.InsertPackage(app); err != nil {
		t.Fatalf("failed to insert app: %v", err)
	}

	// Create dependency relationship
	if err := s.InsertDependency("myapp", "libfoo"); err != nil {
		t.Fatalf("failed to insert dependency: %v", err)
	}

	analyzer := New(s)
	warnings, err := analyzer.ValidateRemoval([]string{"libfoo"})
	if err != nil {
		t.Fatalf("ValidateRemoval failed: %v", err)
	}

	// Should warn about dependents
	foundDependentWarning := false
	for _, w := range warnings {
		if w == "libfoo: has 1 dependents that will remain: [myapp]" {
			foundDependentWarning = true
		}
	}
	if !foundDependentWarning {
		t.Errorf("expected warning about dependents, got: %v", warnings)
	}
}

func TestValidateRemoval_RecentlyUsed(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	pkg := &brew.Package{
		Name:        "git",
		Version:     "2.43.0",
		InstalledAt: time.Now().AddDate(0, 0, -100),
		InstallType: "explicit",
		HasBinary:   true,
		SizeBytes:   50000000,
	}
	if err := s.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert package: %v", err)
	}

	// Add recent usage
	event := &store.UsageEvent{
		Package:   "git",
		EventType: "exec",
		Timestamp: time.Now().AddDate(0, 0, -3),
	}
	if err := s.InsertUsageEvent(event); err != nil {
		t.Fatalf("failed to insert usage event: %v", err)
	}

	analyzer := New(s)
	warnings, err := analyzer.ValidateRemoval([]string{"git"})
	if err != nil {
		t.Fatalf("ValidateRemoval failed: %v", err)
	}

	// Should warn about recent use - check for substring match
	foundRecentUseWarning := false
	for _, w := range warnings {
		if len(w) > 4 && w[:4] == "git:" && (w[5:18] == "used recently" || (len(w) > 18 && w[5:18] == "used recently")) {
			foundRecentUseWarning = true
		}
	}
	if !foundRecentUseWarning {
		t.Errorf("expected warning about recent use, got: %v", warnings)
	}
}

func TestValidateRemoval_RiskyPackage(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	// Insert a recently installed AND recently used package (risky: low age + low usage score)
	// Under corrected scoring: recently used = UsageScore=0; installed 10 days = AgeScore=0
	// Total: 0+30+0+10=40 → risky tier
	pkg := &brew.Package{
		Name:        "newpkg",
		Version:     "1.0.0",
		InstalledAt: time.Now().AddDate(0, 0, -10),
		InstallType: "explicit",
		HasBinary:   true,
		SizeBytes:   10000000,
	}
	if err := s.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert package: %v", err)
	}

	// Add recent usage event so UsageScore=0 (recently used = keep)
	event := &store.UsageEvent{
		Package:   "newpkg",
		EventType: "exec",
		Timestamp: time.Now().AddDate(0, 0, -2),
	}
	if err := s.InsertUsageEvent(event); err != nil {
		t.Fatalf("failed to insert usage event: %v", err)
	}

	analyzer := New(s)
	warnings, err := analyzer.ValidateRemoval([]string{"newpkg"})
	if err != nil {
		t.Fatalf("ValidateRemoval failed: %v", err)
	}

	// Should warn about risky removal (score: 0+30+0+10=40, risky tier)
	foundRiskyWarning := false
	for _, w := range warnings {
		if len(w) > 7 && w[:7] == "newpkg:" && len(w) > 20 && w[8:23] == "risky to remove" {
			foundRiskyWarning = true
		}
	}
	if !foundRiskyWarning {
		t.Errorf("expected risky warning, got: %v", warnings)
	}
}

func TestValidateRemoval_PackageNotFound(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	analyzer := New(s)
	warnings, err := analyzer.ValidateRemoval([]string{"nonexistent"})
	if err != nil {
		t.Fatalf("ValidateRemoval failed: %v", err)
	}

	// Should warn about missing package
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(warnings))
	}
	if warnings[0] != "nonexistent: package not found in database" {
		t.Errorf("expected not found warning, got: %s", warnings[0])
	}
}

func TestValidateRemoval_DependentAlsoRemoved(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	// Insert a library and its dependent
	lib := &brew.Package{
		Name:        "libbar",
		Version:     "1.0.0",
		InstalledAt: time.Now().AddDate(0, 0, -100),
		InstallType: "dependency",
		HasBinary:   false,
		SizeBytes:   5000000,
	}
	if err := s.InsertPackage(lib); err != nil {
		t.Fatalf("failed to insert library: %v", err)
	}

	app := &brew.Package{
		Name:        "appbar",
		Version:     "1.0.0",
		InstalledAt: time.Now().AddDate(0, 0, -100),
		InstallType: "explicit",
		HasBinary:   true,
		SizeBytes:   20000000,
	}
	if err := s.InsertPackage(app); err != nil {
		t.Fatalf("failed to insert app: %v", err)
	}

	if err := s.InsertDependency("appbar", "libbar"); err != nil {
		t.Fatalf("failed to insert dependency: %v", err)
	}

	analyzer := New(s)
	// Remove both library and its dependent
	warnings, err := analyzer.ValidateRemoval([]string{"libbar", "appbar"})
	if err != nil {
		t.Fatalf("ValidateRemoval failed: %v", err)
	}

	// Should not warn about libbar having dependents since appbar is also being removed
	foundDependentWarning := false
	for _, w := range warnings {
		if len(w) > 6 && w[:6] == "libbar" && w[8:] == "has" {
			foundDependentWarning = true
		}
	}
	if foundDependentWarning {
		t.Errorf("should not warn about dependents when they're also being removed: %v", warnings)
	}
}

func TestValidateRemoval_MultiplePackages(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	// Insert multiple safe packages
	for i := 1; i <= 3; i++ {
		pkg := &brew.Package{
			Name:        "pkg" + string(rune('0'+i)),
			Version:     "1.0.0",
			InstalledAt: time.Now().AddDate(0, 0, -200),
			InstallType: "explicit",
			HasBinary:   true,
			SizeBytes:   10000000,
		}
		if err := s.InsertPackage(pkg); err != nil {
			t.Fatalf("failed to insert package: %v", err)
		}
	}

	analyzer := New(s)
	warnings, err := analyzer.ValidateRemoval([]string{"pkg1", "pkg2", "pkg3"})
	if err != nil {
		t.Fatalf("ValidateRemoval failed: %v", err)
	}

	// Should have warnings for all three packages (explicit install warning)
	if len(warnings) < 3 {
		t.Errorf("expected at least 3 warnings, got %d", len(warnings))
	}
}
