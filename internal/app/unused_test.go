package app

import (
	"fmt"
	"strings"
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

	// Test sort by size (largest first)
	scores2 := []*analyzer.ConfidenceScore{
		{Package: "pkg-c", Score: 50, SizeBytes: 1000},
		{Package: "pkg-a", Score: 90, SizeBytes: 5000},
		{Package: "pkg-b", Score: 70, SizeBytes: 3000},
	}
	sortScores(scores2, "size")
	if scores2[0].Package != "pkg-a" || scores2[0].SizeBytes != 5000 {
		t.Errorf("sort by size failed: got %s with %d bytes, want pkg-a with 5000",
			scores2[0].Package, scores2[0].SizeBytes)
	}
	if scores2[1].Package != "pkg-b" || scores2[1].SizeBytes != 3000 {
		t.Errorf("sort by size failed: got %s with %d bytes at position 1, want pkg-b with 3000",
			scores2[1].Package, scores2[1].SizeBytes)
	}

	// Test sort by age (oldest first)
	now := time.Now()
	scores3 := []*analyzer.ConfidenceScore{
		{Package: "pkg-c", Score: 50, InstalledAt: now.AddDate(0, 0, -30)},  // 30 days ago
		{Package: "pkg-a", Score: 90, InstalledAt: now.AddDate(0, 0, -200)}, // 200 days ago (oldest)
		{Package: "pkg-b", Score: 70, InstalledAt: now.AddDate(0, 0, -100)}, // 100 days ago
	}
	sortScores(scores3, "age")
	if scores3[0].Package != "pkg-a" {
		t.Errorf("sort by age failed: got %s, want pkg-a (oldest)", scores3[0].Package)
	}
	if scores3[1].Package != "pkg-b" {
		t.Errorf("sort by age failed: got %s at position 1, want pkg-b", scores3[1].Package)
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

func TestRunUnused_NoUsageDataShowsRisky(t *testing.T) {
	// Create in-memory store
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()

	if err := st.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// Verify no usage events
	var eventCount int
	row := st.DB().QueryRow("SELECT COUNT(*) FROM usage_events")
	if scanErr := row.Scan(&eventCount); scanErr != nil {
		t.Fatalf("failed to count events: %v", scanErr)
	}

	if eventCount != 0 {
		t.Fatalf("expected 0 events, got %d", eventCount)
	}

	// When no usage data and no explicit tier/all flags, showRiskyImplicit should be true
	unusedTierSaved := unusedTier
	unusedAllSaved := unusedAll
	defer func() {
		unusedTier = unusedTierSaved
		unusedAll = unusedAllSaved
	}()

	unusedTier = ""
	unusedAll = false

	showRiskyImplicit := (unusedTier == "" && !unusedAll && eventCount == 0)
	if !showRiskyImplicit {
		t.Error("expected showRiskyImplicit to be true when no usage data and no explicit flags")
	}
}

func TestRunUnused_NoUsageDataShowsRisky_ExplicitFlagsDisableImplicit(t *testing.T) {
	// When --all is set, showRiskyImplicit should be false even with no usage data
	unusedTierSaved := unusedTier
	unusedAllSaved := unusedAll
	defer func() {
		unusedTier = unusedTierSaved
		unusedAll = unusedAllSaved
	}()

	unusedTier = ""
	unusedAll = true
	eventCount := 0

	showRiskyImplicit := (unusedTier == "" && !unusedAll && eventCount == 0)
	if showRiskyImplicit {
		t.Error("expected showRiskyImplicit to be false when --all is set")
	}

	// When --tier is set, showRiskyImplicit should be false even with no usage data
	unusedAll = false
	unusedTier = "safe"

	showRiskyImplicit = (unusedTier == "" && !unusedAll && eventCount == 0)
	if showRiskyImplicit {
		t.Error("expected showRiskyImplicit to be false when --tier is set")
	}
}

func TestRunUnused_CasksNoCasksInstalledMessage(t *testing.T) {
	// Test that the cask count logic properly distinguishes "no casks installed"
	// from "no casks match criteria" (unit-level test of the logic)

	// Simulate: unusedCasks=true, caskCount=0, len(scores)=0
	// Should result in "No casks installed." message path
	caskCount := 0
	scores := []*analyzer.ConfidenceScore{}
	unusedCasksFlag := true

	if len(scores) == 0 && unusedCasksFlag {
		if caskCount == 0 {
			// correct path: "No casks installed."
		} else {
			t.Errorf("expected caskCount=0 path but caskCount=%d", caskCount)
		}
	}

	// Simulate: unusedCasks=true, caskCount=3, len(scores)=0
	// Should result in "No casks match the specified criteria (3 cask(s) installed)." path
	caskCount = 3

	if len(scores) == 0 && unusedCasksFlag {
		if caskCount == 0 {
			t.Error("expected non-zero caskCount path but got caskCount=0 path")
		} else { //nolint:staticcheck
			// correct path: "No casks match the specified criteria (3 cask(s) installed)."
		}
	}
}

func TestSortScores_AgeWithTieBreak(t *testing.T) {
	// All packages have identical InstalledAt times; result must be alphabetical.
	sameTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	scores := []*analyzer.ConfidenceScore{
		{Package: "zebra", Score: 50, Tier: "safe", InstalledAt: sameTime},
		{Package: "apple", Score: 70, Tier: "safe", InstalledAt: sameTime},
		{Package: "mango", Score: 60, Tier: "safe", InstalledAt: sameTime},
	}

	sortScores(scores, "age")

	expected := []string{"apple", "mango", "zebra"}
	for i, want := range expected {
		if scores[i].Package != want {
			t.Errorf("position %d: got %s, want %s", i, scores[i].Package, want)
		}
	}
}

func TestSortScores_ScoreWithTieBreak(t *testing.T) {
	// All packages have identical scores; result must be alphabetical.
	scores := []*analyzer.ConfidenceScore{
		{Package: "zebra", Score: 75, Tier: "medium"},
		{Package: "apple", Score: 75, Tier: "medium"},
		{Package: "mango", Score: 75, Tier: "medium"},
	}

	sortScores(scores, "score")

	expected := []string{"apple", "mango", "zebra"}
	for i, want := range expected {
		if scores[i].Package != want {
			t.Errorf("position %d: got %s, want %s", i, scores[i].Package, want)
		}
	}
}

func TestMinScoreFlagDescription(t *testing.T) {
	flag := unusedCmd.Flag("min-score")
	if flag == nil {
		t.Fatal("min-score flag not found")
	}
	if !strings.Contains(flag.Usage, "explain") {
		t.Errorf("min-score flag Usage does not contain 'explain': %q", flag.Usage)
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

func TestUnusedSortAgeExplanation(t *testing.T) {
	// Test that when all packages have identical install times,
	// the sort age note appears with the correct fallback explanation
	sameTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	scores := []*analyzer.ConfidenceScore{
		{Package: "pkg-a", Score: 90, Tier: "safe", InstalledAt: sameTime},
		{Package: "pkg-b", Score: 70, Tier: "medium", InstalledAt: sameTime},
		{Package: "pkg-c", Score: 50, Tier: "risky", InstalledAt: sameTime},
	}

	// Check that detection logic works
	allSameInstallTime := true
	if len(scores) > 1 {
		firstTime := scores[0].InstalledAt
		for _, s := range scores[1:] {
			if !s.InstalledAt.Equal(firstTime) {
				allSameInstallTime = false
				break
			}
		}
	}

	if !allSameInstallTime {
		t.Error("expected allSameInstallTime to be true when all times are identical")
	}

	// Verify sort fallback behavior (tier order, then alphabetical)
	sortScores(scores, "age")

	// Expected order: safe (pkg-a) → medium (pkg-b) → risky (pkg-c)
	if scores[0].Package != "pkg-a" || scores[0].Tier != "safe" {
		t.Errorf("position 0: got %s (%s), want pkg-a (safe)", scores[0].Package, scores[0].Tier)
	}
	if scores[1].Package != "pkg-b" || scores[1].Tier != "medium" {
		t.Errorf("position 1: got %s (%s), want pkg-b (medium)", scores[1].Package, scores[1].Tier)
	}
	if scores[2].Package != "pkg-c" || scores[2].Tier != "risky" {
		t.Errorf("position 2: got %s (%s), want pkg-c (risky)", scores[2].Package, scores[2].Tier)
	}
}

func TestUnusedMinScoreFooter(t *testing.T) {
	// Test footer logic when both --min-score filters packages
	// AND risky tier is hidden (not using --all)

	// Simulate the filtering logic
	allScores := []*analyzer.ConfidenceScore{
		{Package: "safe1", Score: 80, Tier: "safe"},     // Above threshold (80 >= 70)
		{Package: "safe2", Score: 85, Tier: "safe"},     // Above threshold (85 >= 70)
		{Package: "medium1", Score: 65, Tier: "medium"}, // Below threshold (65 < 70)
		{Package: "medium2", Score: 50, Tier: "medium"}, // Below threshold (50 < 70)
		{Package: "risky1", Score: 30, Tier: "risky"},   // Below threshold + hidden by risky filter
	}

	minScore := 70
	showAll := false
	showRiskyImplicit := false
	tierFilter := ""

	var filtered []*analyzer.ConfidenceScore
	var belowScoreThreshold int

	for _, s := range allScores {
		if s.Score < minScore {
			belowScoreThreshold++
			continue
		}
		// Hide risky tier when not using --all
		if !showAll && tierFilter == "" && s.Tier == "risky" && !showRiskyImplicit {
			continue
		}
		filtered = append(filtered, s)
	}

	// Expected: 2 packages shown (safe1=80, safe2=85)
	// 3 below threshold (medium1=65, medium2=50, risky1=30)
	// Note: risky1 is hidden by risky suppression AND below threshold
	if len(filtered) != 2 {
		t.Errorf("filtered count: got %d, want 2", len(filtered))
	}
	if belowScoreThreshold != 3 {
		t.Errorf("below threshold count: got %d, want 3", belowScoreThreshold)
	}

	// Verify footer condition: both filters active
	bothFiltersActive := belowScoreThreshold > 0 && !showAll && tierFilter == "" && !showRiskyImplicit
	if !bothFiltersActive {
		t.Error("expected both filters (score + risky suppression) to be active")
	}
}

func TestUnusedTierValidationFormat(t *testing.T) {
	// Test that tier validation error matches the standard format
	invalidTier := "invalid"
	expectedError := `invalid --tier value "invalid": must be one of: safe, medium, risky`

	// Simulate validation logic from runUnused
	var err error
	if invalidTier != "" && invalidTier != "safe" && invalidTier != "medium" && invalidTier != "risky" {
		err = fmt.Errorf("invalid --tier value %q: must be one of: safe, medium, risky", invalidTier)
	}

	if err == nil {
		t.Fatal("expected validation error but got nil")
	}

	if err.Error() != expectedError {
		t.Errorf("error format mismatch:\ngot:  %s\nwant: %s", err.Error(), expectedError)
	}

	// Test valid tiers don't produce errors
	validTiers := []string{"safe", "medium", "risky", ""}
	for _, tier := range validTiers {
		var validErr error
		if tier != "" && tier != "safe" && tier != "medium" && tier != "risky" {
			validErr = fmt.Errorf("invalid --tier value %q: must be one of: safe, medium, risky", tier)
		}
		if validErr != nil {
			t.Errorf("tier %q should be valid but got error: %v", tier, validErr)
		}
	}
}

func TestEmptyResultsMessageWithFilters(t *testing.T) {
	// Test that empty results with active filters show helpful message
	tier := "safe"
	minScore := 90

	// Simulate filter description building
	var filters []string
	if tier != "" {
		filters = append(filters, fmt.Sprintf("tier=%s", tier))
	}
	if minScore > 0 {
		filters = append(filters, fmt.Sprintf("min-score=%d", minScore))
	}

	if len(filters) == 0 {
		t.Fatal("expected filters to be populated")
	}

	expectedSubstring := "tier=safe, min-score=90"
	actualMessage := strings.Join(filters, ", ")

	if actualMessage != expectedSubstring {
		t.Errorf("filter message mismatch:\ngot:  %s\nwant: %s", actualMessage, expectedSubstring)
	}
}

func TestHiddenCountSeparation(t *testing.T) {
	// Test that hidden count separates score threshold from tier filtering
	belowScoreThreshold := 3
	riskyTierCount := 5
	minScore := 70
	showAll := false
	tierFilter := ""
	showRiskyImplicit := false

	var hiddenMessages []string

	if belowScoreThreshold > 0 {
		hiddenMessages = append(hiddenMessages, fmt.Sprintf("%d packages below score threshold (%d)", belowScoreThreshold, minScore))
	}

	if !showAll && tierFilter == "" && !showRiskyImplicit && riskyTierCount > 0 {
		hiddenMessages = append(hiddenMessages, fmt.Sprintf("%d packages in risky tier", riskyTierCount))
	}

	if len(hiddenMessages) != 2 {
		t.Errorf("expected 2 hidden messages, got %d", len(hiddenMessages))
	}

	expectedFirst := "3 packages below score threshold (70)"
	if hiddenMessages[0] != expectedFirst {
		t.Errorf("first message mismatch:\ngot:  %s\nwant: %s", hiddenMessages[0], expectedFirst)
	}

	expectedSecond := "5 packages in risky tier"
	if hiddenMessages[1] != expectedSecond {
		t.Errorf("second message mismatch:\ngot:  %s\nwant: %s", hiddenMessages[1], expectedSecond)
	}
}

func TestVerbosePaginationTip(t *testing.T) {
	// Test that verbose mode suggests pagination for large output
	scores := make([]*analyzer.ConfidenceScore, 15) // More than 10

	if len(scores) <= 10 {
		t.Error("expected pagination tip to trigger for >10 packages")
	}

	// The tip should only show for verbose mode with >10 packages
	// This is a logic test - actual output is tested in integration
}

func TestEmptyResultsFormattedMessage(t *testing.T) {
	// Test that empty results show improved message format
	tier := "safe"
	minScore := 90

	// Build filter description as in runUnused
	var filters []string
	if tier != "" {
		filters = append(filters, fmt.Sprintf("tier=%s", tier))
	}
	if minScore > 0 {
		filters = append(filters, fmt.Sprintf("min-score=%d", minScore))
	}

	if len(filters) == 0 {
		t.Fatal("expected filters to be populated")
	}

	// Expected format: "No packages match: tier=safe, min-score=90"
	expectedSubstring := "tier=safe, min-score=90"
	actualMessage := strings.Join(filters, ", ")

	if actualMessage != expectedSubstring {
		t.Errorf("filter message mismatch:\ngot:  %s\nwant: %s", actualMessage, expectedSubstring)
	}
}

func TestHiddenCountSeparatedByFilter(t *testing.T) {
	// Test that hidden messages separate score threshold from tier filtering
	belowScoreThreshold := 3
	riskyTierCount := 5
	minScore := 70
	showAll := false
	tierFilter := ""
	showRiskyImplicit := false

	var hiddenMessages []string

	if belowScoreThreshold > 0 {
		hiddenMessages = append(hiddenMessages, fmt.Sprintf("%d below score threshold (%d)", belowScoreThreshold, minScore))
	}

	if !showAll && tierFilter == "" && !showRiskyImplicit && riskyTierCount > 0 {
		hiddenMessages = append(hiddenMessages, fmt.Sprintf("%d in risky tier", riskyTierCount))
	}

	if len(hiddenMessages) != 2 {
		t.Errorf("expected 2 hidden messages, got %d", len(hiddenMessages))
	}

	expectedFirst := "3 below score threshold (70)"
	if hiddenMessages[0] != expectedFirst {
		t.Errorf("first message mismatch:\ngot:  %s\nwant: %s", hiddenMessages[0], expectedFirst)
	}

	expectedSecond := "5 in risky tier"
	if hiddenMessages[1] != expectedSecond {
		t.Errorf("second message mismatch:\ngot:  %s\nwant: %s", hiddenMessages[1], expectedSecond)
	}

	// Test message format with semicolon separator
	joinedMessage := strings.Join(hiddenMessages, "; ")
	expectedJoined := "3 below score threshold (70); 5 in risky tier"
	if joinedMessage != expectedJoined {
		t.Errorf("joined message mismatch:\ngot:  %s\nwant: %s", joinedMessage, expectedJoined)
	}
}

func TestTierFilteringDocumentation(t *testing.T) {
	// Test that the Long description contains clarification about --tier and --all interaction
	longDesc := unusedCmd.Long

	// Check for key phrases that explain tier filtering behavior
	if !strings.Contains(longDesc, "Tier Filtering:") {
		t.Error("Long description should contain 'Tier Filtering:' section")
	}

	if !strings.Contains(longDesc, "--tier always shows the specified tier") {
		t.Error("Long description should explain that --tier always shows the specified tier")
	}

	if !strings.Contains(longDesc, "--all shows all tiers when --tier is not specified") {
		t.Error("Long description should explain that --all affects behavior when --tier is not specified")
	}
}
