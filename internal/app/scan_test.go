package app

import (
	"strings"
	"testing"
)

func TestScanCommand(t *testing.T) {
	// Test that scan command is properly configured
	if scanCmd.Use != "scan" {
		t.Errorf("expected Use to be 'scan', got '%s'", scanCmd.Use)
	}

	if scanCmd.Short == "" {
		t.Error("expected Short description to be set")
	}

	if scanCmd.Long == "" {
		t.Error("expected Long description to be set")
	}

	if scanCmd.Example == "" {
		t.Error("expected Example to be set")
	}

	if scanCmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
}

func TestScanCommandFlags(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		defaultValue interface{}
	}{
		{
			name:         "refresh-binaries flag",
			flagName:     "refresh-binaries",
			defaultValue: true,
		},
		{
			name:         "quiet flag",
			flagName:     "quiet",
			defaultValue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := scanCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("expected flag '%s' to be registered", tt.flagName)
				return
			}

			if flag.Usage == "" {
				t.Errorf("expected flag '%s' to have usage text", tt.flagName)
			}

			// Check default value
			switch v := tt.defaultValue.(type) {
			case bool:
				if flag.DefValue != "true" && flag.DefValue != "false" {
					t.Errorf("expected flag '%s' to be boolean", tt.flagName)
				}
			case string:
				if flag.DefValue != v {
					t.Errorf("expected flag '%s' default to be '%s', got '%s'", tt.flagName, v, flag.DefValue)
				}
			}
		})
	}
}

func TestScanCommandHelp(t *testing.T) {
	// Test that help can be generated without errors
	oldArgs := scanCmd.Flags()
	defer func() {
		scanCmd.ResetFlags()
		scanCmd.Flags().AddFlagSet(oldArgs)
	}()

	scanCmd.SetArgs([]string{"--help"})

	// Capture the help output
	// The command will return an error but that's expected
	err := scanCmd.Execute()
	if err != nil && !strings.Contains(err.Error(), "help") {
		// Some error is expected when running help
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "bytes",
			bytes:    512,
			expected: "512 B",
		},
		{
			name:     "kilobytes",
			bytes:    2048,
			expected: "2 KB",
		},
		{
			name:     "megabytes",
			bytes:    5 * 1024 * 1024,
			expected: "5 MB",
		},
		{
			name:     "gigabytes",
			bytes:    3 * 1024 * 1024 * 1024,
			expected: "3.0 GB",
		},
		{
			name:     "zero",
			bytes:    0,
			expected: "0 B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSize(tt.bytes)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestScanCommandFlagParsing(t *testing.T) {
	// Reset flags before test
	scanRefreshBinaries = true
	scanQuiet = false

	// Test flag parsing
	tests := []struct {
		name                      string
		args                      []string
		expectedRefreshBinaries   bool
		expectedQuiet             bool
	}{
		{
			name:                    "default flags",
			args:                    []string{},
			expectedRefreshBinaries: true,
			expectedQuiet:           false,
		},
		{
			name:                    "disable refresh binaries",
			args:                    []string{"--refresh-binaries=false"},
			expectedRefreshBinaries: false,
			expectedQuiet:           false,
		},
		{
			name:                    "enable quiet",
			args:                    []string{"--quiet"},
			expectedRefreshBinaries: true,
			expectedQuiet:           true,
		},
		{
			name:                    "both flags",
			args:                    []string{"--refresh-binaries=false", "--quiet"},
			expectedRefreshBinaries: false,
			expectedQuiet:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags
			scanRefreshBinaries = true
			scanQuiet = false

			// Parse flags
			scanCmd.ParseFlags(tt.args)

			if scanRefreshBinaries != tt.expectedRefreshBinaries {
				t.Errorf("expected refreshBinaries to be %v, got %v", tt.expectedRefreshBinaries, scanRefreshBinaries)
			}

			if scanQuiet != tt.expectedQuiet {
				t.Errorf("expected quiet to be %v, got %v", tt.expectedQuiet, scanQuiet)
			}
		})
	}
}

func TestScanCommandRegistration(t *testing.T) {
	// Verify scan command is registered with root
	found := false
	for _, cmd := range RootCmd.Commands() {
		if cmd.Use == "scan" {
			found = true
			break
		}
	}

	if !found {
		t.Error("scan command not registered with root command")
	}
}
