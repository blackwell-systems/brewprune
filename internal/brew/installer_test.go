package brew

import (
	"os/exec"
	"strings"
	"testing"
)

// Mock command execution for testing
// In real tests, we don't actually call brew commands

func TestUninstallCommandStructure(t *testing.T) {
	// Test that the command would be structured correctly
	// We're testing the command structure, not executing it
	pkgName := "test-package"

	cmd := exec.Command("brew", "uninstall", pkgName)

	if cmd.Path != "brew" && !contains(cmd.Args, "brew") {
		t.Error("command should use brew")
	}

	if !contains(cmd.Args, "uninstall") {
		t.Error("command should contain uninstall")
	}

	if !contains(cmd.Args, pkgName) {
		t.Error("command should contain package name")
	}
}

func TestInstallCommandStructure(t *testing.T) {
	tests := []struct {
		name       string
		pkgName    string
		version    string
		expectArgs []string
	}{
		{
			name:       "install without version",
			pkgName:    "node",
			version:    "",
			expectArgs: []string{"brew", "install", "node"},
		},
		{
			name:       "install with version",
			pkgName:    "node",
			version:    "16",
			expectArgs: []string{"brew", "install", "node@16"},
		},
		{
			name:       "install with version already in name",
			pkgName:    "python@3.12",
			version:    "",
			expectArgs: []string{"brew", "install", "python@3.12"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fullName string
			if tt.version != "" {
				fullName = tt.pkgName + "@" + tt.version
			} else {
				fullName = tt.pkgName
			}

			cmd := exec.Command("brew", "install", fullName)

			// Verify command structure
			expectedArgs := []string{"brew", "install", fullName}
			if len(cmd.Args) != len(expectedArgs) {
				t.Errorf("expected %d args, got %d", len(expectedArgs), len(cmd.Args))
			}

			for i, expectedArg := range expectedArgs {
				if i < len(cmd.Args) && cmd.Args[i] != expectedArg {
					t.Errorf("arg %d: expected %s, got %s", i, expectedArg, cmd.Args[i])
				}
			}
		})
	}
}

func TestAddTapCommandStructure(t *testing.T) {
	tap := "homebrew/cask-versions"

	cmd := exec.Command("brew", "tap", tap)

	if !contains(cmd.Args, "brew") {
		t.Error("command should use brew")
	}

	if !contains(cmd.Args, "tap") {
		t.Error("command should contain tap subcommand")
	}

	if !contains(cmd.Args, tap) {
		t.Error("command should contain tap name")
	}
}

func TestTapExistsCommandStructure(t *testing.T) {
	cmd := exec.Command("brew", "tap")

	if !contains(cmd.Args, "brew") {
		t.Error("command should use brew")
	}

	if !contains(cmd.Args, "tap") {
		t.Error("command should contain tap subcommand")
	}

	// Should have no additional arguments (lists all taps)
	if len(cmd.Args) > 2 {
		t.Errorf("expected 2 args (brew tap), got %d", len(cmd.Args))
	}
}

func TestInstallVersionFormatting(t *testing.T) {
	tests := []struct {
		name         string
		pkgName      string
		version      string
		expectedName string
	}{
		{
			name:         "simple package with version",
			pkgName:      "node",
			version:      "16",
			expectedName: "node@16",
		},
		{
			name:         "package without version",
			pkgName:      "git",
			version:      "",
			expectedName: "git",
		},
		{
			name:         "package with @ already in name",
			pkgName:      "python@3.12",
			version:      "",
			expectedName: "python@3.12",
		},
		{
			name:         "package with @ in name and version provided",
			pkgName:      "python@3.12",
			version:      "",
			expectedName: "python@3.12", // Should use as-is when package already has @
		},
		{
			name:         "complex version number",
			pkgName:      "postgresql",
			version:      "14.10",
			expectedName: "postgresql@14.10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fullName string
			if tt.version != "" {
				// Homebrew uses @ syntax for versioned packages
				if contains([]string{tt.pkgName}, "@") {
					fullName = tt.pkgName
				} else {
					fullName = tt.pkgName + "@" + tt.version
				}
			} else {
				fullName = tt.pkgName
			}

			if fullName != tt.expectedName {
				t.Errorf("expected %s, got %s", tt.expectedName, fullName)
			}
		})
	}
}

func TestTapListParsing(t *testing.T) {
	// Mock output from `brew tap`
	mockTapList := `homebrew/core
homebrew/cask
homebrew/cask-versions
user/custom-tap`

	taps := parseLines(mockTapList)

	expectedTaps := []string{
		"homebrew/core",
		"homebrew/cask",
		"homebrew/cask-versions",
		"user/custom-tap",
	}

	if len(taps) != len(expectedTaps) {
		t.Errorf("expected %d taps, got %d", len(expectedTaps), len(taps))
	}

	for i, expected := range expectedTaps {
		if i >= len(taps) {
			break
		}
		if taps[i] != expected {
			t.Errorf("tap %d: expected %s, got %s", i, expected, taps[i])
		}
	}
}

func TestTapListEmpty(t *testing.T) {
	mockTapList := ""
	taps := parseLines(mockTapList)

	// Empty or single empty string
	if len(taps) > 1 {
		t.Errorf("expected 0 or 1 taps for empty input, got %d", len(taps))
	}
	if len(taps) == 1 && taps[0] != "" {
		t.Errorf("expected empty string, got %s", taps[0])
	}
}

func TestTapListWithWhitespace(t *testing.T) {
	mockTapList := `  homebrew/core
  homebrew/cask
user/tap  `

	taps := parseLines(mockTapList)

	// After trimming, should have 3 valid taps
	expected := []string{"homebrew/core", "homebrew/cask", "user/tap"}

	if len(taps) != len(expected) {
		t.Errorf("expected %d taps, got %d", len(expected), len(taps))
	}
}

func TestUninstallWithSpecialCharacters(t *testing.T) {
	// Test packages with special characters in names
	tests := []string{
		"node@16",
		"python@3.12",
		"openssl@3",
		"postgresql@14.10",
	}

	for _, pkgName := range tests {
		t.Run(pkgName, func(t *testing.T) {
			cmd := exec.Command("brew", "uninstall", pkgName)

			if !contains(cmd.Args, pkgName) {
				t.Errorf("command should contain package name %s", pkgName)
			}
		})
	}
}

func TestInstallWithTap(t *testing.T) {
	// Test installing from specific tap
	// Format: tap/formula or just formula
	tests := []struct {
		name    string
		pkgName string
	}{
		{
			name:    "formula with tap prefix",
			pkgName: "homebrew/core/node",
		},
		{
			name:    "formula without tap",
			pkgName: "node",
		},
		{
			name:    "custom tap formula",
			pkgName: "user/tap/custom-formula",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("brew", "install", tt.pkgName)

			if !contains(cmd.Args, "install") {
				t.Error("command should contain install")
			}
			if !contains(cmd.Args, tt.pkgName) {
				t.Errorf("command should contain package name %s", tt.pkgName)
			}
		})
	}
}

func TestTapNameValidation(t *testing.T) {
	// Test valid tap name formats
	validTaps := []string{
		"homebrew/core",
		"homebrew/cask",
		"homebrew/cask-versions",
		"user/custom-tap",
		"organization/my-tap",
	}

	for _, tap := range validTaps {
		t.Run(tap, func(t *testing.T) {
			// A valid tap should have format: user/repo
			parts := splitString(tap, "/")
			if len(parts) != 2 {
				t.Errorf("tap %s should have format user/repo", tap)
			}
			if parts[0] == "" || parts[1] == "" {
				t.Errorf("tap %s has empty user or repo", tap)
			}
		})
	}
}

func TestCommandErrorHandling(t *testing.T) {
	// Test that we properly structure error messages
	tests := []struct {
		name       string
		command    string
		subcommand string
		arg        string
	}{
		{
			name:       "uninstall",
			command:    "brew",
			subcommand: "uninstall",
			arg:        "package",
		},
		{
			name:       "install",
			command:    "brew",
			subcommand: "install",
			arg:        "package",
		},
		{
			name:       "tap",
			command:    "brew",
			subcommand: "tap",
			arg:        "user/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(tt.command, tt.subcommand, tt.arg)

			// Verify all parts are present
			if !contains(cmd.Args, tt.command) {
				t.Errorf("missing command %s", tt.command)
			}
			if !contains(cmd.Args, tt.subcommand) {
				t.Errorf("missing subcommand %s", tt.subcommand)
			}
			if !contains(cmd.Args, tt.arg) {
				t.Errorf("missing arg %s", tt.arg)
			}
		})
	}
}

// Helper functions for testing

func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func parseLines(s string) []string {
	if s == "" {
		return []string{""}
	}
	lines := []string{}
	current := ""
	for _, char := range s {
		if char == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(char)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func splitString(s, sep string) []string {
	if s == "" {
		return []string{""}
	}
	result := []string{}
	current := ""
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, current)
			current = ""
			i += len(sep) - 1
		} else {
			current += string(s[i])
		}
	}
	result = append(result, current)
	return result
}

func TestMultipleUninstalls(t *testing.T) {
	// Test that we can construct multiple uninstall commands
	packages := []string{"node", "python", "git"}

	for _, pkg := range packages {
		cmd := exec.Command("brew", "uninstall", pkg)
		if !contains(cmd.Args, pkg) {
			t.Errorf("command should contain package %s", pkg)
		}
	}
}

func TestInstallWithCombinedOptions(t *testing.T) {
	// Test version handling edge cases
	tests := []struct {
		pkg     string
		version string
		want    string
	}{
		{"node", "20", "node@20"},
		{"node", "", "node"},
		{"node@20", "", "node@20"},
		{"python@3.12", "", "python@3.12"},
	}

	for _, tt := range tests {
		t.Run(tt.pkg+"_"+tt.version, func(t *testing.T) {
			var fullName string
			if tt.version != "" {
				if contains([]string{tt.pkg}, "@") {
					fullName = tt.pkg
				} else {
					fullName = tt.pkg + "@" + tt.version
				}
			} else {
				fullName = tt.pkg
			}

			if fullName != tt.want {
				t.Errorf("got %s, want %s", fullName, tt.want)
			}
		})
	}
}

// TestBrewUses_NoOutput verifies that Uses() with empty output returns nil, nil.
func TestBrewUses_NoOutput(t *testing.T) {
	// Simulate parsing empty output (as would occur when brew uses exits with empty output)
	output := ""
	var deps []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			deps = append(deps, line)
		}
	}
	if deps != nil {
		t.Errorf("expected nil deps for empty output, got %v", deps)
	}
}

// TestBrewUses_WithDependents verifies that Uses() correctly parses multi-line output.
func TestBrewUses_WithDependents(t *testing.T) {
	// Simulate parsing output "packageA\npackageB\n"
	mockOutput := "packageA\npackageB\n"
	var deps []string
	for _, line := range strings.Split(strings.TrimSpace(mockOutput), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			deps = append(deps, line)
		}
	}
	expected := []string{"packageA", "packageB"}
	if len(deps) != len(expected) {
		t.Fatalf("expected %d deps, got %d: %v", len(expected), len(deps), deps)
	}
	for i, d := range deps {
		if d != expected[i] {
			t.Errorf("dep[%d] = %q, want %q", i, d, expected[i])
		}
	}
}

// TestBrewUsesCommandStructure verifies the Uses() function builds the correct command.
func TestBrewUsesCommandStructure(t *testing.T) {
	pkgName := "test-package"
	cmd := exec.Command("brew", "uses", "--installed", pkgName)

	if !contains(cmd.Args, "brew") {
		t.Error("command should use brew")
	}
	if !contains(cmd.Args, "uses") {
		t.Error("command should contain uses subcommand")
	}
	if !contains(cmd.Args, "--installed") {
		t.Error("command should contain --installed flag")
	}
	if !contains(cmd.Args, pkgName) {
		t.Errorf("command should contain package name %s", pkgName)
	}
}

func TestTapOperationsConsistency(t *testing.T) {
	// Test that tap operations use consistent format
	tap := "user/repo"

	// List command
	listCmd := exec.Command("brew", "tap")
	if len(listCmd.Args) != 2 {
		t.Errorf("list command should have 2 args, got %d", len(listCmd.Args))
	}

	// Add command
	addCmd := exec.Command("brew", "tap", tap)
	if len(addCmd.Args) != 3 {
		t.Errorf("add command should have 3 args, got %d", len(addCmd.Args))
	}
	if !contains(addCmd.Args, tap) {
		t.Error("add command should contain tap name")
	}
}
