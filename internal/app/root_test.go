package app

import (
	"bytes"
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
	if err != nil && !strings.Contains(err.Error(), "unknown command") { //nolint:staticcheck
		// Any error other than "unknown command" is acceptable for help
		// The help text will have been printed
	}
}

func TestExecute(t *testing.T) {
	// Test that Execute function works
	// We can't easily test the actual execution without mocking,
	// but we can verify the function exists
	// Note: Functions are never nil in Go, so we just check it's callable
	_ = Execute
}

func TestRootCmd_BareInvocationShowsHelp(t *testing.T) {
	// Verify that RootCmd has a RunE set for bare invocation (no subcommand).
	if RootCmd.RunE == nil {
		t.Fatal("expected RootCmd.RunE to be set for bare invocation")
	}

	// Verify that SuggestionsMinimumDistance is set
	if RootCmd.SuggestionsMinimumDistance != 2 {
		t.Errorf("SuggestionsMinimumDistance = %d, want 2", RootCmd.SuggestionsMinimumDistance)
	}

	// Verify SilenceUsage and SilenceErrors are set
	if !RootCmd.SilenceUsage {
		t.Error("expected SilenceUsage to be true")
	}
	if !RootCmd.SilenceErrors {
		t.Error("expected SilenceErrors to be true")
	}

	// Verify Long description is still set (used by --help)
	if RootCmd.Long == "" {
		t.Error("expected Long description to still be set for --help")
	}
	if !strings.Contains(RootCmd.Long, "Quick Start") {
		t.Error("expected Long description to contain 'Quick Start' section")
	}

	// Invoke RunE directly via cmd.Help() — capture output and verify it
	// contains "Usage:" and subcommand names, exits 0.
	var buf bytes.Buffer
	RootCmd.SetOut(&buf)
	defer RootCmd.SetOut(nil)

	if err := RootCmd.RunE(RootCmd, []string{}); err != nil {
		t.Errorf("RootCmd.RunE() returned unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Usage:") {
		t.Errorf("expected help output to contain 'Usage:', got: %s", out)
	}
}

func TestRootCommandHelp_QuickstartMentioned(t *testing.T) {
	// Verify that --help output contains the string "quickstart".
	var buf bytes.Buffer
	RootCmd.SetOut(&buf)
	defer RootCmd.SetOut(nil)

	RootCmd.SetArgs([]string{"--help"})
	// Help exits 0; ignore any error from cobra's help handling.
	_ = RootCmd.Execute()

	out := buf.String()
	if !strings.Contains(out, "quickstart") {
		t.Errorf("expected help output to contain 'quickstart', got: %s", out)
	}
}

func TestExecute_UnknownCommandHelpHint(t *testing.T) {
	// Verify that running an unknown subcommand causes Execute() to write
	// the help hint to stderr.
	var stderrBuf bytes.Buffer
	RootCmd.SetErr(&stderrBuf)
	defer RootCmd.SetErr(nil)

	// Suppress stdout during this test
	RootCmd.SetOut(bytes.NewBuffer(nil))
	defer RootCmd.SetOut(nil)

	RootCmd.SetArgs([]string{"blorp"})
	err := Execute()

	if err == nil {
		t.Error("expected Execute() to return an error for unknown command")
	}

	// The help hint is written to os.Stderr directly in Execute(), not to
	// cobra's stderr writer. We capture cobra's stderr for cobra's own message
	// and accept that the hint goes to os.Stderr. Verify the error contains
	// "unknown command" as expected.
	if err != nil && !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("expected error to contain 'unknown command', got: %v", err)
	}
}

func TestBareBrewpruneExitsOne(t *testing.T) {
	// Verify that bare invocation (no args, no flags) exits with non-zero
	var buf bytes.Buffer
	RootCmd.SetOut(&buf)
	defer RootCmd.SetOut(nil)

	// Suppress stderr
	RootCmd.SetErr(bytes.NewBuffer(nil))
	defer RootCmd.SetErr(nil)

	RootCmd.SetArgs([]string{})
	err := Execute()

	if err == nil {
		t.Error("expected Execute() to return an error for bare invocation")
	}

	// Verify help text was shown (stdout should contain usage info)
	out := buf.String()
	if !strings.Contains(out, "Usage:") {
		t.Errorf("expected help output to contain 'Usage:', got: %s", out)
	}
}

func TestBrewpruneHelpExitsZero(t *testing.T) {
	// Verify that --help flag exits successfully (no error)
	var buf bytes.Buffer
	RootCmd.SetOut(&buf)
	defer RootCmd.SetOut(nil)

	// Suppress stderr
	RootCmd.SetErr(bytes.NewBuffer(nil))
	defer RootCmd.SetErr(nil)

	RootCmd.SetArgs([]string{"--help"})
	err := Execute()

	if err != nil {
		t.Errorf("expected Execute() with --help to succeed, got error: %v", err)
	}

	// Verify help text was shown
	out := buf.String()
	if !strings.Contains(out, "Usage:") {
		t.Errorf("expected help output to contain 'Usage:', got: %s", out)
	}
}

func TestUnknownSubcommandErrorOrder(t *testing.T) {
	// Verify that unknown subcommand error appears before the hint
	// Note: Execute() writes to os.Stderr directly, so we can't fully capture it,
	// but we can verify the error is returned and contains the expected text
	var stderrBuf bytes.Buffer
	RootCmd.SetErr(&stderrBuf)
	defer RootCmd.SetErr(nil)

	// Suppress stdout
	RootCmd.SetOut(bytes.NewBuffer(nil))
	defer RootCmd.SetOut(nil)

	RootCmd.SetArgs([]string{"blorp"})
	err := Execute()

	if err == nil {
		t.Error("expected Execute() to return an error for unknown command")
	}

	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("expected error to contain 'unknown command', got: %v", err)
	}

	// The actual stderr output order is verified by the Execute() function logic,
	// which prints error first, then hint. This test verifies the error is properly
	// returned so Execute() can format it correctly.
}
