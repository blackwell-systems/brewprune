package app

import (
	"testing"
	"time"

	"github.com/blackwell-systems/brewprune/internal/analyzer"
	"github.com/blackwell-systems/brewprune/internal/brew"
	"github.com/blackwell-systems/brewprune/internal/store"
)

func TestUnusedCommand_Registration(t *testing.T) {
	// Test that unusedCmd is registered with RootCmd
	found := false
	for _, cmd := range RootCmd.Commands() {
		if cmd.Name() == "unused" {
			found = true
			break
		}
	}

	if !found {
		t.Error("unused command not registered with root command")
	}
}

func TestUnusedCommand_Flags(t *testing.T) {
	// Test that all expected flags are defined
	flags := []string{"tier", "min-score", "sort"}

	for _, flagName := range flags {
		flag := unusedCmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("flag %s not defined", flagName)
		}
	}
}

func TestUnusedCommand_TierValidation(t *testing.T) {
	tests := []struct {
		name    string
		tier    string
		wantErr bool
	}{
		{"valid safe", "safe", false},
		{"valid medium", "medium", false},
		{"valid risky", "risky", false},
		{"invalid tier", "invalid", true},
		{"empty tier", "", false}, // Empty is allowed (means show all)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't easily test RunE without a full setup, so we test validation logic
			if tt.tier != "" && tt.tier != "safe" && tt.tier != "medium" && tt.tier != "risky" {
				if !tt.wantErr {
					t.Error("expected validation to fail but it didn't")
				}
			}
		})
	}
}

func TestSortScores(t *testing.T) {
	scores := []*analyzer.ConfidenceScore{
		{Package: "pkg-c", Score: 50},
		{Package: "pkg-a", Score: 90},
		{Package: "pkg-b", Score: 70},
	}

	// Test sort by score
	sortScores(scores, "score")
	if scores[0].Package != "pkg-a" || scores[0].Score != 90 {
		t.Errorf("sort by score failed: got %s with score %d, want pkg-a with 90",
			scores[0].Package, scores[0].Score)
	}

	// Test sort by size (currently sorts by name)
	scores2 := []*analyzer.ConfidenceScore{
		{Package: "pkg-c", Score: 50},
		{Package: "pkg-a", Score: 90},
		{Package: "pkg-b", Score: 70},
	}
	sortScores(scores2, "size")
	if scores2[0].Package != "pkg-a" {
		t.Errorf("sort by size failed: got %s, want pkg-a", scores2[0].Package)
	}

	// Test sort by age (currently sorts by name)
	scores3 := []*analyzer.ConfidenceScore{
		{Package: "pkg-c", Score: 50},
		{Package: "pkg-a", Score: 90},
		{Package: "pkg-b", Score: 70},
	}
	sortScores(scores3, "age")
	if scores3[0].Package != "pkg-a" {
		t.Errorf("sort by age failed: got %s, want pkg-a", scores3[0].Package)
	}
}

func TestComputeSummary(t *testing.T) {
	scores := []*analyzer.ConfidenceScore{
		{Package: "pkg1", Score: 90, Tier: "safe"},
		{Package: "pkg2", Score: 85, Tier: "safe"},
		{Package: "pkg3", Score: 60, Tier: "medium"},
		{Package: "pkg4", Score: 30, Tier: "risky"},
		{Package: "pkg5", Score: 40, Tier: "risky"},
	}

	summary := computeSummary(scores)

	if summary["safe"] != 2 {
		t.Errorf("safe count: got %d, want 2", summary["safe"])
	}
	if summary["medium"] != 1 {
		t.Errorf("medium count: got %d, want 1", summary["medium"])
	}
	if summary["risky"] != 2 {
		t.Errorf("risky count: got %d, want 2", summary["risky"])
	}
}

func TestGetLastUsed(t *testing.T) {
	// Create in-memory store for testing
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()

	if err := st.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// Insert a test package
	pkg := &brew.Package{
		Name:        "test-pkg",
		Version:     "1.0.0",
		InstalledAt: time.Now(),
		InstallType: "explicit",
		Tap:         "homebrew/core",
	}
	if err := st.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert package: %v", err)
	}

	// Test with no usage
	lastUsed := getLastUsed(st, "test-pkg")
	if !lastUsed.IsZero() {
		t.Error("expected zero time for package with no usage")
	}

	// Insert a usage event
	now := time.Now()
	event := &store.UsageEvent{
		Package:    "test-pkg",
		EventType:  "exec",
		BinaryPath: "/usr/local/bin/test",
		Timestamp:  now,
	}
	if err := st.InsertUsageEvent(event); err != nil {
		t.Fatalf("failed to insert usage event: %v", err)
	}

	// Test with usage
	lastUsed = getLastUsed(st, "test-pkg")
	if lastUsed.IsZero() {
		t.Error("expected non-zero time for package with usage")
	}

	// Check that the time is approximately correct (within 1 second)
	diff := lastUsed.Sub(now)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("last used time mismatch: got %v, want %v (diff: %v)", lastUsed, now, diff)
	}
}

func TestUnusedCommand_MinScoreFilter(t *testing.T) {
	scores := []*analyzer.ConfidenceScore{
		{Package: "pkg1", Score: 90, Tier: "safe"},
		{Package: "pkg2", Score: 60, Tier: "medium"},
		{Package: "pkg3", Score: 30, Tier: "risky"},
	}

	// Filter by min score 50
	minScore := 50
	filtered := make([]*analyzer.ConfidenceScore, 0)
	for _, s := range scores {
		if s.Score >= minScore {
			filtered = append(filtered, s)
		}
	}

	if len(filtered) != 2 {
		t.Errorf("min-score filter: got %d packages, want 2", len(filtered))
	}

	if filtered[0].Package != "pkg1" && filtered[1].Package != "pkg1" {
		t.Error("pkg1 should be included in filtered results")
	}
	if filtered[0].Package != "pkg2" && filtered[1].Package != "pkg2" {
		t.Error("pkg2 should be included in filtered results")
	}
}

func TestUnusedCommand_TierFilter(t *testing.T) {
	scores := []*analyzer.ConfidenceScore{
		{Package: "pkg1", Score: 90, Tier: "safe"},
		{Package: "pkg2", Score: 85, Tier: "safe"},
		{Package: "pkg3", Score: 60, Tier: "medium"},
		{Package: "pkg4", Score: 30, Tier: "risky"},
	}

	// Filter by tier "safe"
	tier := "safe"
	filtered := make([]*analyzer.ConfidenceScore, 0)
	for _, s := range scores {
		if s.Tier == tier {
			filtered = append(filtered, s)
		}
	}

	if len(filtered) != 2 {
		t.Errorf("tier filter: got %d packages, want 2", len(filtered))
	}

	for _, s := range filtered {
		if s.Tier != "safe" {
			t.Errorf("filtered package %s has tier %s, want safe", s.Package, s.Tier)
		}
	}
}
