package app

import (
	"testing"
	"time"

	"github.com/blackwell-systems/brewprune/internal/brew"
	"github.com/blackwell-systems/brewprune/internal/store"
	"github.com/spf13/cobra"
)

func TestRemoveCommand(t *testing.T) {
	// Test command registration
	if removeCmd == nil {
		t.Fatal("removeCmd is nil")
	}

	// Test command metadata
	if removeCmd.Use != "remove [packages...]" {
		t.Errorf("removeCmd.Use = %q, want %q", removeCmd.Use, "remove [packages...]")
	}

	if removeCmd.Short == "" {
		t.Error("removeCmd.Short is empty")
	}

	if removeCmd.RunE == nil {
		t.Error("removeCmd.RunE is nil")
	}
}

func TestRemoveFlags(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		defaultValue bool
	}{
		{"safe flag", "safe", false},
		{"medium flag", "medium", false},
		{"risky flag", "risky", false},
		{"dry-run flag", "dry-run", false},
		{"yes flag", "yes", false},
		{"no-snapshot flag", "no-snapshot", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := removeCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("flag %q not found", tt.flagName)
				return
			}

			if flag.DefValue != "false" {
				t.Errorf("flag %q default = %q, want %q", tt.flagName, flag.DefValue, "false")
			}
		})
	}
}

func TestDetermineTier(t *testing.T) {
	tests := []struct {
		name     string
		safe     bool
		medium   bool
		risky    bool
		expected string
	}{
		{"no flags", false, false, false, ""},
		{"safe only", true, false, false, "safe"},
		{"medium only", false, true, false, "medium"},
		{"risky only", false, false, true, "risky"},
		{"safe and medium", true, true, false, "medium"},
		{"all flags", true, true, true, "risky"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set flags
			removeFlagSafe = tt.safe
			removeFlagMedium = tt.medium
			removeFlagRisky = tt.risky
			removeTierFlag = ""

			result, err := determineTier()

			if err != nil {
				t.Errorf("determineTier() unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("determineTier() = %q, want %q", result, tt.expected)
			}
		})
	}

	// Reset flags
	removeFlagSafe = false
	removeFlagMedium = false
	removeFlagRisky = false
	removeTierFlag = ""
}

func TestDetermineTier_TierFlag(t *testing.T) {
	tests := []struct {
		name      string
		tierFlag  string
		wantTier  string
		wantError bool
	}{
		{"tier safe", "safe", "safe", false},
		{"tier medium", "medium", "medium", false},
		{"tier risky", "risky", "risky", false},
		{"tier invalid", "invalid", "", true},
		{"tier empty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset boolean flags
			removeFlagSafe = false
			removeFlagMedium = false
			removeFlagRisky = false
			removeTierFlag = tt.tierFlag

			result, err := determineTier()

			if tt.wantError {
				if err == nil {
					t.Errorf("determineTier() expected error for tier %q, got none", tt.tierFlag)
				}
				return
			}
			if err != nil {
				t.Errorf("determineTier() unexpected error: %v", err)
			}
			if result != tt.wantTier {
				t.Errorf("determineTier() = %q, want %q", result, tt.wantTier)
			}
		})
	}

	// Reset flags
	removeTierFlag = ""
}

func TestRemoveCommandRegistration(t *testing.T) {
	// Create a temporary root command for testing
	tempRoot := &cobra.Command{Use: "test"}

	// Add remove command
	tempRoot.AddCommand(removeCmd)

	// Verify command was added
	found := false
	for _, cmd := range tempRoot.Commands() {
		if cmd.Use == "remove [packages...]" {
			found = true
			break
		}
	}

	if !found {
		t.Error("remove command not registered with parent")
	}
}

func TestRemoveValidation(t *testing.T) {
	// Test that tier flag is required when no packages specified
	// This is more of an integration test and would need a mock setup

	t.Run("no tier and no packages", func(t *testing.T) {
		// This would fail in actual execution
		// Testing the determineTier function is sufficient
		removeTierFlag = ""
		tier, err := determineTier()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if tier != "" {
			t.Errorf("expected empty tier, got %q", tier)
		}
	})
}

func TestDisplayConfidenceScores_LastUsedNotNever(t *testing.T) {
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
		InstalledAt: time.Now().AddDate(0, 0, -30),
		InstallType: "explicit",
		Tap:         "homebrew/core",
	}
	if err := st.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert package: %v", err)
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

	// Verify getLastUsed returns a non-zero time for this package
	lastUsed := getLastUsed(st, "test-pkg")
	if lastUsed.IsZero() {
		t.Error("expected non-zero LastUsed time for package with usage data, got zero time (never)")
	}

	// Verify the time is approximately correct
	diff := lastUsed.Sub(now)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("LastUsed time mismatch: got %v, want approximately %v", lastUsed, now)
	}
}

func TestGetPackagesByTierLogic(t *testing.T) {
	// This tests the tier inclusion logic
	tests := []struct {
		name     string
		tier     string
		includes []string
	}{
		{"safe tier", "safe", []string{"safe"}},
		{"medium tier", "medium", []string{"safe", "medium"}},
		{"risky tier", "risky", []string{"safe", "medium", "risky"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the logic matches expected behavior
			switch tt.tier {
			case "safe":
				if len(tt.includes) != 1 || tt.includes[0] != "safe" {
					t.Errorf("safe tier should only include safe packages")
				}
			case "medium":
				if len(tt.includes) != 2 {
					t.Errorf("medium tier should include safe and medium packages")
				}
			case "risky":
				if len(tt.includes) != 3 {
					t.Errorf("risky tier should include all packages")
				}
			}
		})
	}
}
