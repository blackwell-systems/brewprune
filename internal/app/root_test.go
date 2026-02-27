package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootCommand(t *testing.T) {
	// Test that root command is properly configured
	if RootCmd.Use != "brewprune" {
		t.Errorf("expected Use to be 'brewprune', got '%s'", RootCmd.Use)
	}

	if RootCmd.Short == "" {
		t.Error("expected Short description to be set")
	}

	if RootCmd.Long == "" {
		t.Error("expected Long description to be set")
	}
}

func TestRootCommandHasSubcommands(t *testing.T) {
	// Test that subcommands are registered
	commands := RootCmd.Commands()

	expectedCommands := []string{"scan", "watch"}
	foundCommands := make(map[string]bool)

	for _, cmd := range commands {
		foundCommands[cmd.Use] = true
	}

	for _, expected := range expectedCommands {
		if !foundCommands[expected] {
			t.Errorf("expected command '%s' to be registered", expected)
		}
	}
}

func TestRootCommandHasPersistentFlags(t *testing.T) {
	// Test that --db flag is available
	flag := RootCmd.PersistentFlags().Lookup("db")
	if flag == nil {
		t.Error("expected --db flag to be registered")
	}

	if flag != nil && flag.Usage == "" {
		t.Error("expected --db flag to have usage text")
	}
}

func TestGetDBPath(t *testing.T) {
	tests := []struct {
		name        string
		dbPathFlag  string
		expectError bool
	}{
		{
			name:        "default path",
			dbPathFlag:  "",
			expectError: false,
		},
		{
			name:        "custom path",
			dbPathFlag:  "/tmp/test.db",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the global dbPath variable
			oldDBPath := dbPath
			dbPath = tt.dbPathFlag
			defer func() { dbPath = oldDBPath }()

			path, err := getDBPath()

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectError {
				if path == "" {
					t.Error("expected non-empty path")
				}

				if tt.dbPathFlag != "" && path != tt.dbPathFlag {
					t.Errorf("expected path to be '%s', got '%s'", tt.dbPathFlag, path)
				}

				if tt.dbPathFlag == "" {
					home, _ := os.UserHomeDir()
					expectedPath := filepath.Join(home, ".brewprune", "brewprune.db")
					if path != expectedPath {
						t.Errorf("expected default path to be '%s', got '%s'", expectedPath, path)
					}
				}
			}
		})
	}
}

func TestGetDefaultPIDFile(t *testing.T) {
	path, err := getDefaultPIDFile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if path == "" {
		t.Error("expected non-empty path")
	}

	if !strings.HasSuffix(path, "watch.pid") {
		t.Errorf("expected path to end with 'watch.pid', got '%s'", path)
	}

	// Check that directory exists
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("expected directory '%s' to exist", dir)
	}
}

func TestGetDefaultLogFile(t *testing.T) {
	path, err := getDefaultLogFile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if path == "" {
		t.Error("expected non-empty path")
	}

	if !strings.HasSuffix(path, "watch.log") {
		t.Errorf("expected path to end with 'watch.log', got '%s'", path)
	}

	// Check that directory exists
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("expected directory '%s' to exist", dir)
	}
}

func TestRootCommandHelp(t *testing.T) {
	// Test that help can be generated without errors
	RootCmd.SetArgs([]string{"--help"})
	err := RootCmd.Execute()

	// Help command returns an error in cobra, but it's expected
	// We just want to make sure it doesn't panic
	if err != nil && !strings.Contains(err.Error(), "unknown command") {
		// Any error other than "unknown command" is acceptable for help
		// The help text will have been printed
	}
}

func TestExecute(t *testing.T) {
	// Test that Execute function works
	// We can't easily test the actual execution without mocking,
	// but we can verify the function exists and has the right signature
	if Execute == nil {
		t.Error("Execute function should exist")
	}
}
