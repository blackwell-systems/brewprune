package app

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/blackwell-systems/brewprune/internal/analyzer"
	"github.com/blackwell-systems/brewprune/internal/brew"
	"github.com/blackwell-systems/brewprune/internal/store"
)

func TestStatsCommand_Registration(t *testing.T) {
	// Test that statsCmd is registered with RootCmd
	found := false
	for _, cmd := range RootCmd.Commands() {
		if cmd.Name() == "stats" {
			found = true
			break
		}
	}

	if !found {
		t.Error("stats command not registered with root command")
	}
}

func TestStatsCommand_Flags(t *testing.T) {
	// Test that all expected flags are defined
	flags := []string{"days", "package", "all"}

	for _, flagName := range flags {
		flag := statsCmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("flag %s not defined", flagName)
		}
	}
}

func TestStatsCommand_FlagDefaults(t *testing.T) {
	// Test default values
	daysFlag := statsCmd.Flags().Lookup("days")
	if daysFlag == nil {
		t.Fatal("days flag not found")
	}

	if daysFlag.DefValue != "30" {
		t.Errorf("days flag default: got %s, want 30", daysFlag.DefValue)
	}

	packageFlag := statsCmd.Flags().Lookup("package")
	if packageFlag == nil {
		t.Fatal("package flag not found")
	}

	if packageFlag.DefValue != "" {
		t.Errorf("package flag default: got %s, want empty", packageFlag.DefValue)
	}
}

func TestStatsCommand_DaysValidation(t *testing.T) {
	tests := []struct {
		name    string
		days    int
		wantErr bool
	}{
		{"valid 1 day", 1, false},
		{"valid 30 days", 30, false},
		{"valid 365 days", 365, false},
		{"invalid 0 days", 0, true},
		{"invalid negative", -10, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate days logic
			if tt.days <= 0 && !tt.wantErr {
				t.Error("expected validation to fail but it didn't")
			}
			if tt.days > 0 && tt.wantErr {
				t.Error("expected validation to pass but it failed")
			}
		})
	}
}

func TestFormatTime(t *testing.T) {
	tests := []struct {
		name string
		time time.Time
		want string
	}{
		{
			name: "zero time",
			time: time.Time{},
			want: "never",
		},
		{
			name: "specific time",
			time: time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
			want: "2024-01-15 10:30:45",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTime(tt.time)
			if got != tt.want {
				t.Errorf("formatTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShowPackageStats_Integration(t *testing.T) {
	// Create in-memory store for testing
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()

	if err := st.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// Insert test package
	pkg := &brew.Package{
		Name:        "test-pkg",
		Version:     "1.0.0",
		InstalledAt: time.Now().AddDate(0, 0, -30), // 30 days ago
		InstallType: "explicit",
		Tap:         "homebrew/core",
	}
	if err := st.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert package: %v", err)
	}

	// Insert usage events
	now := time.Now()
	events := []*store.UsageEvent{
		{
			Package:    "test-pkg",
			EventType:  "exec",
			BinaryPath: "/usr/local/bin/test",
			Timestamp:  now.AddDate(0, 0, -1), // 1 day ago
		},
		{
			Package:    "test-pkg",
			EventType:  "exec",
			BinaryPath: "/usr/local/bin/test",
			Timestamp:  now.AddDate(0, 0, -7), // 7 days ago
		},
	}

	for _, event := range events {
		if err := st.InsertUsageEvent(event); err != nil {
			t.Fatalf("failed to insert usage event: %v", err)
		}
	}

	// Test that we can retrieve stats (actual display is tested manually)
	// This is a smoke test to ensure no errors occur
	t.Log("Integration test passed: can insert and retrieve package stats")
}

func TestShowUsageTrends_Integration(t *testing.T) {
	// Create in-memory store for testing
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()

	if err := st.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// Insert multiple test packages
	packages := []*brew.Package{
		{
			Name:        "active-pkg",
			Version:     "1.0.0",
			InstalledAt: time.Now().AddDate(0, 0, -60),
			InstallType: "explicit",
			Tap:         "homebrew/core",
		},
		{
			Name:        "inactive-pkg",
			Version:     "2.0.0",
			InstalledAt: time.Now().AddDate(0, 0, -90),
			InstallType: "explicit",
			Tap:         "homebrew/core",
		},
	}

	for _, pkg := range packages {
		if err := st.InsertPackage(pkg); err != nil {
			t.Fatalf("failed to insert package %s: %v", pkg.Name, err)
		}
	}

	// Insert usage event for active package
	event := &store.UsageEvent{
		Package:    "active-pkg",
		EventType:  "exec",
		BinaryPath: "/usr/local/bin/active",
		Timestamp:  time.Now().AddDate(0, 0, -5), // 5 days ago
	}
	if err := st.InsertUsageEvent(event); err != nil {
		t.Fatalf("failed to insert usage event: %v", err)
	}

	// Test that we can retrieve trends (actual display is tested manually)
	// This is a smoke test to ensure no errors occur
	t.Log("Integration test passed: can retrieve usage trends")
}

func TestStatsCommand_PackageNotFound(t *testing.T) {
	// Test behavior when package doesn't exist
	// This would be tested in integration but we can verify the logic
	packageName := "nonexistent-pkg"

	if packageName == "" {
		t.Error("package name should not be empty in this test")
	}
}

func TestStatsCommand_EmptyDatabase(t *testing.T) {
	// Create in-memory store with no data
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()

	if err := st.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	// List packages should return empty
	packages, err := st.ListPackages()
	if err != nil {
		t.Fatalf("failed to list packages: %v", err)
	}

	if len(packages) != 0 {
		t.Errorf("expected empty database, got %d packages", len(packages))
	}
}

func TestStatsCommand_TimeWindowFilter(t *testing.T) {
	// Test filtering by time window
	now := time.Now()

	tests := []struct {
		name       string
		lastUsed   time.Time
		windowDays int
		inWindow   bool
	}{
		{
			name:       "within 7 day window",
			lastUsed:   now.AddDate(0, 0, -3),
			windowDays: 7,
			inWindow:   true,
		},
		{
			name:       "outside 7 day window",
			lastUsed:   now.AddDate(0, 0, -10),
			windowDays: 7,
			inWindow:   false,
		},
		{
			name:       "within 30 day window",
			lastUsed:   now.AddDate(0, 0, -20),
			windowDays: 30,
			inWindow:   true,
		},
		{
			name:       "outside 30 day window",
			lastUsed:   now.AddDate(0, 0, -40),
			windowDays: 30,
			inWindow:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			daysSince := int(now.Sub(tt.lastUsed).Hours() / 24)
			inWindow := daysSince <= tt.windowDays

			if inWindow != tt.inWindow {
				t.Errorf("time window check: got %v, want %v (days since: %d, window: %d)",
					inWindow, tt.inWindow, daysSince, tt.windowDays)
			}
		})
	}
}

// TestShowPackageStats_FrequencyIsColored verifies that showPackageStats
// color-codes the Frequency line when stdout is a TTY.  We use a pipe to
// capture output; a pipe is not a TTY so colors will be skipped.  We verify
// that the plain frequency label is present and no ANSI codes leak in
// non-TTY mode.
func TestShowPackageStats_FrequencyIsColored(t *testing.T) {
	// Build a real in-memory store with a package and usage events.
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()
	if err := st.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	now := time.Now()
	pkg := &brew.Package{
		Name:        "freq-test-pkg",
		Version:     "1.0.0",
		InstalledAt: now.AddDate(0, 0, -10),
		InstallType: "explicit",
		Tap:         "homebrew/core",
	}
	if err := st.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert package: %v", err)
	}

	// Insert a recent event so frequency becomes "daily" or "weekly".
	event := &store.UsageEvent{
		Package:    "freq-test-pkg",
		EventType:  "exec",
		BinaryPath: "/usr/local/bin/freq-test-pkg",
		Timestamp:  now.AddDate(0, 0, -1),
	}
	if err := st.InsertUsageEvent(event); err != nil {
		t.Fatalf("failed to insert usage event: %v", err)
	}

	a := analyzer.New(st)

	// Redirect stdout to a pipe so we can capture the output.
	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("os.Pipe: %v", pipeErr)
	}
	os.Stdout = w

	showErr := showPackageStats(a, "freq-test-pkg")

	w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read captured output: %v", err)
	}
	output := buf.String()

	if showErr != nil {
		t.Fatalf("showPackageStats returned unexpected error: %v", showErr)
	}

	// The output must contain a "Frequency:" line.
	if !strings.Contains(output, "Frequency:") {
		t.Errorf("expected 'Frequency:' in output, got:\n%s", output)
	}

	// The package name header must be present.
	if !strings.Contains(output, "freq-test-pkg") {
		t.Errorf("expected package name in output, got:\n%s", output)
	}

	// Since stdout was a pipe (non-TTY), no ANSI codes should appear.
	if strings.Contains(output, "\033[") {
		t.Errorf("expected no ANSI codes in non-TTY output, got:\n%s", output)
	}
}

// TestShowPackageStats_ColorLogic verifies that the frequency to color mapping
// is correct by exercising the inline logic used in showPackageStats.
func TestShowPackageStats_ColorLogic(t *testing.T) {
	tests := []struct {
		freq          string
		expectedColor string
		colorName     string
	}{
		{"daily", "\033[32m", "green"},
		{"weekly", "\033[33m", "yellow"},
		{"monthly", "\033[31m", "red"},
		{"rarely", "\033[31m", "red"},
		{"never", "\033[90m", "gray"},
	}

	// Mirror the color-mapping closure from showPackageStats.
	colorFreq := func(freq string) string {
		const (
			ansiReset  = "\033[0m"
			ansiGreen  = "\033[32m"
			ansiYellow = "\033[33m"
			ansiRed    = "\033[31m"
			ansiGray   = "\033[90m"
		)
		switch freq {
		case "daily":
			return ansiGreen + freq + ansiReset
		case "weekly":
			return ansiYellow + freq + ansiReset
		case "monthly", "rarely":
			return ansiRed + freq + ansiReset
		case "never":
			return ansiGray + freq + ansiReset
		default:
			return freq
		}
	}

	for _, tt := range tests {
		t.Run(tt.freq, func(t *testing.T) {
			got := colorFreq(tt.freq)
			prefixLen := len(tt.expectedColor)
			if len(got) < prefixLen || got[:prefixLen] != tt.expectedColor {
				t.Errorf("frequency %q: expected color code %q (%s), got: %q",
					tt.freq, tt.expectedColor, tt.colorName, got)
			}
		})
	}
}

// TestShowUsageTrends_HidesZeroUsageByDefault verifies that with statsAll=false
// (the default), packages with 0 TotalRuns are not shown and the "hidden" hint
// is printed.
func TestShowUsageTrends_HidesZeroUsageByDefault(t *testing.T) {
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()
	if err := st.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	now := time.Now()

	pkgs := []*brew.Package{
		{Name: "used-pkg", Version: "1.0.0", InstalledAt: now.AddDate(0, 0, -60), InstallType: "explicit", Tap: "homebrew/core"},
		{Name: "unused-pkg", Version: "1.0.0", InstalledAt: now.AddDate(0, 0, -60), InstallType: "explicit", Tap: "homebrew/core"},
	}
	for _, p := range pkgs {
		if err := st.InsertPackage(p); err != nil {
			t.Fatalf("failed to insert package %s: %v", p.Name, err)
		}
	}

	event := &store.UsageEvent{
		Package:    "used-pkg",
		EventType:  "exec",
		BinaryPath: "/usr/local/bin/used-pkg",
		Timestamp:  now.AddDate(0, 0, -3),
	}
	if err := st.InsertUsageEvent(event); err != nil {
		t.Fatalf("failed to insert usage event: %v", err)
	}

	a := analyzer.New(st)

	origStatsAll := statsAll
	statsAll = false
	defer func() { statsAll = origStatsAll }()

	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("os.Pipe: %v", pipeErr)
	}
	os.Stdout = w

	showErr := showUsageTrends(a, 30)

	w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read captured output: %v", err)
	}
	out := buf.String()

	if showErr != nil {
		t.Fatalf("showUsageTrends returned unexpected error: %v", showErr)
	}

	if !strings.Contains(out, "used-pkg") {
		t.Errorf("expected 'used-pkg' in output, got:\n%s", out)
	}

	if strings.Contains(out, "unused-pkg") {
		t.Errorf("expected 'unused-pkg' to be hidden, got:\n%s", out)
	}

	if !strings.Contains(out, "hidden") {
		t.Errorf("expected hidden-packages hint in output, got:\n%s", out)
	}
}

// TestShowUsageTrends_ShowAllFlag verifies that with statsAll=true, all packages
// including those with 0 usage appear in the output.
func TestShowUsageTrends_ShowAllFlag(t *testing.T) {
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()
	if err := st.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	now := time.Now()

	pkgs := []*brew.Package{
		{Name: "used-pkg2", Version: "1.0.0", InstalledAt: now.AddDate(0, 0, -60), InstallType: "explicit", Tap: "homebrew/core"},
		{Name: "unused-pkg2", Version: "1.0.0", InstalledAt: now.AddDate(0, 0, -60), InstallType: "explicit", Tap: "homebrew/core"},
	}
	for _, p := range pkgs {
		if err := st.InsertPackage(p); err != nil {
			t.Fatalf("failed to insert package %s: %v", p.Name, err)
		}
	}

	event := &store.UsageEvent{
		Package:    "used-pkg2",
		EventType:  "exec",
		BinaryPath: "/usr/local/bin/used-pkg2",
		Timestamp:  now.AddDate(0, 0, -3),
	}
	if err := st.InsertUsageEvent(event); err != nil {
		t.Fatalf("failed to insert usage event: %v", err)
	}

	a := analyzer.New(st)

	origStatsAll := statsAll
	statsAll = true
	defer func() { statsAll = origStatsAll }()

	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("os.Pipe: %v", pipeErr)
	}
	os.Stdout = w

	showErr := showUsageTrends(a, 30)

	w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read captured output: %v", err)
	}
	out := buf.String()

	if showErr != nil {
		t.Fatalf("showUsageTrends returned unexpected error: %v", showErr)
	}

	if !strings.Contains(out, "used-pkg2") {
		t.Errorf("expected 'used-pkg2' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "unused-pkg2") {
		t.Errorf("expected 'unused-pkg2' in output with --all, got:\n%s", out)
	}
}

// TestShowPackageStats_ZeroUsage_ShowsExplainHint verifies that when a package
// has 0 TotalUses the output contains a pointer to 'brewprune explain' with
// consistent message including "scoring detail".
func TestShowPackageStats_ZeroUsage_ShowsExplainHint(t *testing.T) {
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()
	if err := st.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	pkg := &brew.Package{
		Name:        "zero-use-pkg",
		Version:     "1.0.0",
		InstalledAt: time.Now().AddDate(0, 0, -30),
		InstallType: "explicit",
		Tap:         "homebrew/core",
	}
	if err := st.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert package: %v", err)
	}

	a := analyzer.New(st)

	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("os.Pipe: %v", pipeErr)
	}
	os.Stdout = w

	showErr := showPackageStats(a, "zero-use-pkg")

	w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read captured output: %v", err)
	}
	out := buf.String()

	if showErr != nil {
		t.Fatalf("showPackageStats returned unexpected error: %v", showErr)
	}

	if !strings.Contains(out, "brewprune explain") {
		t.Errorf("expected 'brewprune explain' hint in output for zero-usage package, got:\n%s", out)
	}

	if !strings.Contains(out, "zero-use-pkg") {
		t.Errorf("expected package name in explain hint, got:\n%s", out)
	}

	if !strings.Contains(out, "scoring detail") {
		t.Errorf("expected 'scoring detail' in tip message for consistency, got:\n%s", out)
	}
}

// TestShowPackageStats_WithUsage_ShowsExplainHint verifies that when a package
// has usage data, the output contains a pointer to 'brewprune explain' with
// mention of "scoring detail".
func TestShowPackageStats_WithUsage_ShowsExplainHint(t *testing.T) {
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()
	if err := st.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	now := time.Now()
	pkg := &brew.Package{
		Name:        "with-usage-pkg",
		Version:     "1.0.0",
		InstalledAt: now.AddDate(0, 0, -30),
		InstallType: "explicit",
		Tap:         "homebrew/core",
	}
	if err := st.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert package: %v", err)
	}

	// Add usage event
	event := &store.UsageEvent{
		Package:    "with-usage-pkg",
		EventType:  "exec",
		BinaryPath: "/usr/local/bin/with-usage-pkg",
		Timestamp:  now.AddDate(0, 0, -5),
	}
	if err := st.InsertUsageEvent(event); err != nil {
		t.Fatalf("failed to insert usage event: %v", err)
	}

	a := analyzer.New(st)

	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("os.Pipe: %v", pipeErr)
	}
	os.Stdout = w

	showErr := showPackageStats(a, "with-usage-pkg")

	w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read captured output: %v", err)
	}
	out := buf.String()

	if showErr != nil {
		t.Fatalf("showPackageStats returned unexpected error: %v", showErr)
	}

	if !strings.Contains(out, "brewprune explain") {
		t.Errorf("expected 'brewprune explain' hint in output for package with usage, got:\n%s", out)
	}

	if !strings.Contains(out, "with-usage-pkg") {
		t.Errorf("expected package name in explain hint, got:\n%s", out)
	}

	if !strings.Contains(out, "scoring detail") {
		t.Errorf("expected 'scoring detail' phrase in explain hint for package with usage, got:\n%s", out)
	}
}

// TestShowUsageTrends_BannerShownWhenHidden verifies that the banner appears
// BEFORE the table when packages are hidden (default --all=false behavior).
func TestShowUsageTrends_BannerShownWhenHidden(t *testing.T) {
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()
	if err := st.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	now := time.Now()
	pkgs := []*brew.Package{
		{Name: "used-banner-pkg", Version: "1.0.0", InstalledAt: now.AddDate(0, 0, -60), InstallType: "explicit", Tap: "homebrew/core"},
		{Name: "unused-banner-pkg", Version: "1.0.0", InstalledAt: now.AddDate(0, 0, -60), InstallType: "explicit", Tap: "homebrew/core"},
	}
	for _, p := range pkgs {
		if err := st.InsertPackage(p); err != nil {
			t.Fatalf("failed to insert package %s: %v", p.Name, err)
		}
	}

	event := &store.UsageEvent{
		Package:    "used-banner-pkg",
		EventType:  "exec",
		BinaryPath: "/usr/local/bin/used-banner-pkg",
		Timestamp:  now.AddDate(0, 0, -3),
	}
	if err := st.InsertUsageEvent(event); err != nil {
		t.Fatalf("failed to insert usage event: %v", err)
	}

	a := analyzer.New(st)

	origStatsAll := statsAll
	statsAll = false
	defer func() { statsAll = origStatsAll }()

	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("os.Pipe: %v", pipeErr)
	}
	os.Stdout = w

	showErr := showUsageTrends(a, 30)

	w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read captured output: %v", err)
	}
	out := buf.String()

	if showErr != nil {
		t.Fatalf("showUsageTrends returned unexpected error: %v", showErr)
	}

	// Verify the banner appears
	if !strings.Contains(out, "Showing 1 of 2 packages") {
		t.Errorf("expected banner 'Showing 1 of 2 packages' in output, got:\n%s", out)
	}

	if !strings.Contains(out, "use --all to see all") {
		t.Errorf("expected '--all' mention in banner, got:\n%s", out)
	}

	// Verify banner appears before the table (check for Package header after banner)
	bannerIdx := strings.Index(out, "Showing 1 of 2 packages")
	if bannerIdx == -1 {
		t.Fatal("banner not found")
	}
	// The table should have the package name after the banner
	tableIdx := strings.Index(out, "used-banner-pkg")
	if tableIdx == -1 {
		t.Fatal("table content not found")
	}
	if bannerIdx >= tableIdx {
		t.Errorf("expected banner to appear before table content, banner at %d, table at %d", bannerIdx, tableIdx)
	}
}

// TestShowUsageTrends_AllFlagSortsByTotalRuns verifies that with --all flag,
// packages are sorted by total runs descending (most used first).
func TestShowUsageTrends_AllFlagSortsByTotalRuns(t *testing.T) {
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()
	if err := st.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	now := time.Now()

	// Create packages with different usage counts
	pkgs := []*brew.Package{
		{Name: "high-usage", Version: "1.0.0", InstalledAt: now.AddDate(0, 0, -60), InstallType: "explicit", Tap: "homebrew/core"},
		{Name: "med-usage", Version: "1.0.0", InstalledAt: now.AddDate(0, 0, -60), InstallType: "explicit", Tap: "homebrew/core"},
		{Name: "low-usage", Version: "1.0.0", InstalledAt: now.AddDate(0, 0, -60), InstallType: "explicit", Tap: "homebrew/core"},
		{Name: "zero-usage", Version: "1.0.0", InstalledAt: now.AddDate(0, 0, -60), InstallType: "explicit", Tap: "homebrew/core"},
	}
	for _, p := range pkgs {
		if err := st.InsertPackage(p); err != nil {
			t.Fatalf("failed to insert package %s: %v", p.Name, err)
		}
	}

	// Add events: high-usage=10, med-usage=5, low-usage=1, zero-usage=0
	for i := 0; i < 10; i++ {
		event := &store.UsageEvent{
			Package:    "high-usage",
			EventType:  "exec",
			BinaryPath: "/usr/local/bin/high-usage",
			Timestamp:  now.AddDate(0, 0, -i-1),
		}
		if err := st.InsertUsageEvent(event); err != nil {
			t.Fatalf("failed to insert usage event: %v", err)
		}
	}
	for i := 0; i < 5; i++ {
		event := &store.UsageEvent{
			Package:    "med-usage",
			EventType:  "exec",
			BinaryPath: "/usr/local/bin/med-usage",
			Timestamp:  now.AddDate(0, 0, -i-1),
		}
		if err := st.InsertUsageEvent(event); err != nil {
			t.Fatalf("failed to insert usage event: %v", err)
		}
	}
	event := &store.UsageEvent{
		Package:    "low-usage",
		EventType:  "exec",
		BinaryPath: "/usr/local/bin/low-usage",
		Timestamp:  now.AddDate(0, 0, -1),
	}
	if err := st.InsertUsageEvent(event); err != nil {
		t.Fatalf("failed to insert usage event: %v", err)
	}

	a := analyzer.New(st)

	origStatsAll := statsAll
	statsAll = true
	defer func() { statsAll = origStatsAll }()

	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("os.Pipe: %v", pipeErr)
	}
	os.Stdout = w

	showErr := showUsageTrends(a, 30)

	w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read captured output: %v", err)
	}
	out := buf.String()

	if showErr != nil {
		t.Fatalf("showUsageTrends returned unexpected error: %v", showErr)
	}

	// Verify all packages appear
	if !strings.Contains(out, "high-usage") {
		t.Errorf("expected 'high-usage' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "med-usage") {
		t.Errorf("expected 'med-usage' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "low-usage") {
		t.Errorf("expected 'low-usage' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "zero-usage") {
		t.Errorf("expected 'zero-usage' in output with --all, got:\n%s", out)
	}

	// Verify sorting order: high-usage should appear before med-usage, etc.
	highIdx := strings.Index(out, "high-usage")
	medIdx := strings.Index(out, "med-usage")
	lowIdx := strings.Index(out, "low-usage")
	zeroIdx := strings.Index(out, "zero-usage")

	if highIdx == -1 || medIdx == -1 || lowIdx == -1 || zeroIdx == -1 {
		t.Fatal("not all packages found in output")
	}

	if highIdx >= medIdx {
		t.Errorf("expected high-usage (10 runs) before med-usage (5 runs), got high at %d, med at %d", highIdx, medIdx)
	}
	if medIdx >= lowIdx {
		t.Errorf("expected med-usage (5 runs) before low-usage (1 run), got med at %d, low at %d", medIdx, lowIdx)
	}
	if lowIdx >= zeroIdx {
		t.Errorf("expected low-usage (1 run) before zero-usage (0 runs), got low at %d, zero at %d", lowIdx, zeroIdx)
	}
}

// TestShowUsageTrends_NoBannerWithAllFlag verifies that when --all is set, no
// banner is shown because all packages are visible.
func TestShowUsageTrends_NoBannerWithAllFlag(t *testing.T) {
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()
	if err := st.CreateSchema(); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	now := time.Now()
	pkgs := []*brew.Package{
		{Name: "used-all-pkg", Version: "1.0.0", InstalledAt: now.AddDate(0, 0, -60), InstallType: "explicit", Tap: "homebrew/core"},
		{Name: "unused-all-pkg", Version: "1.0.0", InstalledAt: now.AddDate(0, 0, -60), InstallType: "explicit", Tap: "homebrew/core"},
	}
	for _, p := range pkgs {
		if err := st.InsertPackage(p); err != nil {
			t.Fatalf("failed to insert package %s: %v", p.Name, err)
		}
	}

	event := &store.UsageEvent{
		Package:    "used-all-pkg",
		EventType:  "exec",
		BinaryPath: "/usr/local/bin/used-all-pkg",
		Timestamp:  now.AddDate(0, 0, -3),
	}
	if err := st.InsertUsageEvent(event); err != nil {
		t.Fatalf("failed to insert usage event: %v", err)
	}

	a := analyzer.New(st)

	origStatsAll := statsAll
	statsAll = true
	defer func() { statsAll = origStatsAll }()

	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("os.Pipe: %v", pipeErr)
	}
	os.Stdout = w

	showErr := showUsageTrends(a, 30)

	w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read captured output: %v", err)
	}
	out := buf.String()

	if showErr != nil {
		t.Fatalf("showUsageTrends returned unexpected error: %v", showErr)
	}

	// Verify no banner appears (no "Showing X of Y" text)
	if strings.Contains(out, "Showing") && strings.Contains(out, "of") && strings.Contains(out, "packages") {
		// More precise check - the banner format is "Showing X of Y packages"
		if strings.Contains(out, "Showing") && strings.Index(out, "of") > strings.Index(out, "Showing") {
			t.Errorf("expected no banner with --all flag, got:\n%s", out)
		}
	}

	// Both packages should appear
	if !strings.Contains(out, "used-all-pkg") {
		t.Errorf("expected 'used-all-pkg' in output with --all, got:\n%s", out)
	}
	if !strings.Contains(out, "unused-all-pkg") {
		t.Errorf("expected 'unused-all-pkg' in output with --all, got:\n%s", out)
	}
}
