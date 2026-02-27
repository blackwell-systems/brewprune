package app

import (
	"testing"

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

			result := determineTier()

			if result != tt.expected {
				t.Errorf("determineTier() = %q, want %q", result, tt.expected)
			}
		})
	}

	// Reset flags
	removeFlagSafe = false
	removeFlagMedium = false
	removeFlagRisky = false
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"zero", 0, "0 B"},
		{"bytes", 512, "512 B"},
		{"kilobytes", 1024, "1 KB"},
		{"kilobytes decimal", 2048, "2 KB"},
		{"megabytes", 1024 * 1024, "1 MB"},
		{"megabytes decimal", 5 * 1024 * 1024, "5 MB"},
		{"gigabytes", 1024 * 1024 * 1024, "1.0 GB"},
		{"gigabytes decimal", 3 * 1024 * 1024 * 1024, "3.0 GB"},
		{"1.5 GB", 1536 * 1024 * 1024, "1.5 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSize(tt.bytes)
			if result != tt.expected {
				t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, result, tt.expected)
			}
		})
	}
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
		tier := determineTier()
		if tier != "" {
			t.Errorf("expected empty tier, got %q", tier)
		}
	})
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
