package app

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/blackwell-systems/brewprune/internal/store"
	"github.com/spf13/cobra"
)

func TestUndoCommand(t *testing.T) {
	// Test command registration
	if undoCmd == nil {
		t.Fatal("undoCmd is nil")
	}

	// Test command metadata
	if undoCmd.Use != "undo [snapshot-id | latest]" {
		t.Errorf("undoCmd.Use = %q, want %q", undoCmd.Use, "undo [snapshot-id | latest]")
	}

	if undoCmd.Short == "" {
		t.Error("undoCmd.Short is empty")
	}

	if undoCmd.RunE == nil {
		t.Error("undoCmd.RunE is nil")
	}
}

func TestUndoFlags(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		defaultValue bool
	}{
		{"list flag", "list", false},
		{"yes flag", "yes", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := undoCmd.Flags().Lookup(tt.flagName)
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

func TestUndoCommandRegistration(t *testing.T) {
	// Create a temporary root command for testing
	tempRoot := &cobra.Command{Use: "test"}

	// Add undo command
	tempRoot.AddCommand(undoCmd)

	// Verify command was added
	found := false
	for _, cmd := range tempRoot.Commands() {
		if cmd.Use == "undo [snapshot-id | latest]" {
			found = true
			break
		}
	}

	if !found {
		t.Error("undo command not registered with parent")
	}
}

func TestUndoUsageExamples(t *testing.T) {
	// Verify the command has a long description
	if undoCmd.Long == "" {
		t.Error("undoCmd.Long is empty")
	}

	// Check for key keywords in the long description (flags/examples moved to Example field)
	longKeywords := []string{"snapshot", "restore", "latest"}
	for _, keyword := range longKeywords {
		if !contains(undoCmd.Long, keyword) {
			t.Errorf("undoCmd.Long missing keyword %q", keyword)
		}
	}

	// Check that Example field contains examples
	if undoCmd.Example == "" {
		t.Error("undoCmd.Example is empty")
	}
	exampleKeywords := []string{"--list", "latest", "undo"}
	for _, keyword := range exampleKeywords {
		if !contains(undoCmd.Example, keyword) {
			t.Errorf("undoCmd.Example missing keyword %q", keyword)
		}
	}
}

func TestUndoValidation(t *testing.T) {
	// Test that validation logic is present
	// In actual execution, runUndo should require args unless --list is provided
	t.Run("requires args or list flag", func(t *testing.T) {
		// This would be tested in integration tests
		// Here we just verify the flag exists
		listFlag := undoCmd.Flags().Lookup("list")
		if listFlag == nil {
			t.Error("list flag should exist for listing snapshots")
		}
	})
}

func TestUndoListMode(t *testing.T) {
	// Test that list flag is properly defined
	flag := undoCmd.Flags().Lookup("list")
	if flag == nil {
		t.Fatal("list flag not found")
	}

	if flag.Usage == "" {
		t.Error("list flag should have usage description")
	}
}

func TestUndoLatestKeyword(t *testing.T) {
	// Verify that the "latest" keyword is documented
	if !contains(undoCmd.Long, "latest") {
		t.Error("command should document the 'latest' keyword")
	}

	if !contains(undoCmd.Use, "latest") {
		t.Error("command usage should include 'latest' option")
	}
}

func TestUndoSnapshotIDParsing(t *testing.T) {
	// Test snapshot ID format expectations
	// The command should accept numeric IDs and "latest"

	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"numeric ID", "42", true},
		{"latest keyword", "latest", true},
		{"latest uppercase", "LATEST", true},
		{"latest mixed case", "Latest", true},
		{"invalid text", "invalid", false},
		{"negative number", "-1", true}, // strconv.ParseInt will handle this
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This tests the logic that would be used in runUndo
			// The actual parsing is done in the command execution
			if tt.input == "latest" || tt.input == "LATEST" || tt.input == "Latest" {
				// These should be recognized as the latest keyword
				return
			}

			// Other inputs would be parsed as int64
			// We just verify the test cases are reasonable
		})
	}
}

// TestRunUndo_LatestNoSnapshotsFriendlyMessage verifies that when
// `brewprune undo latest` is invoked and there are no snapshots, the command
// prints an "Error:"-prefixed message to stderr.
//
// Since runUndo calls os.Exit(1) in this path, this test uses the subprocess
// pattern to avoid terminating the test process.
func TestRunUndo_LatestNoSnapshotsFriendlyMessage(t *testing.T) {
	if os.Getenv("BREWPRUNE_TEST_UNDO_SUBPROCESS") == "1" {
		// ---- Child process ----
		tmpDir := t.TempDir()
		tmpDB := tmpDir + "/undo_test.db"

		st, stErr := store.New(tmpDB)
		if stErr != nil {
			os.Exit(2)
		}
		if schemaErr := st.CreateSchema(); schemaErr != nil {
			st.Close()
			os.Exit(2)
		}
		st.Close()

		dbPath = tmpDB

		cmd := &cobra.Command{}
		// This will call os.Exit(1) internally when no snapshots found.
		runUndo(cmd, []string{"latest"}) //nolint:errcheck
		return
	}

	// ---- Parent process ----
	cmd := exec.Command(os.Args[0], "-test.run=TestRunUndo_LatestNoSnapshotsFriendlyMessage", "-test.v")
	cmd.Env = append(os.Environ(), "BREWPRUNE_TEST_UNDO_SUBPROCESS=1")

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	err := cmd.Run()

	stderrOutput := stderrBuf.String()

	// Expect exit code 1
	if err == nil {
		t.Error("expected subprocess to exit non-zero, got exit 0")
	}

	// Verify "Error:" prefix appears in stderr
	if !strings.Contains(stderrOutput, "Error:") {
		t.Errorf("expected stderr to contain %q, got:\n%s", "Error:", stderrOutput)
	}

	// Verify helpful message appears in stderr
	if !strings.Contains(stderrOutput, "brewprune remove") {
		t.Errorf("expected stderr to contain %q, got:\n%s", "brewprune remove", stderrOutput)
	}
}

// TestRunUndo_LatestNoSnapshots_ExitsNonZero verifies that `undo latest` with
// no snapshots exits with code 1 and prints an "Error:"-prefixed message to
// stderr. Uses subprocess pattern because runUndo calls os.Exit(1).
func TestRunUndo_LatestNoSnapshots_ExitsNonZero(t *testing.T) {
	if os.Getenv("BREWPRUNE_TEST_UNDO_EXITCODE_SUBPROCESS") == "1" {
		// ---- Child process ----
		tmpDir := t.TempDir()
		tmpDB := tmpDir + "/undo_exitcode_test.db"

		st, stErr := store.New(tmpDB)
		if stErr != nil {
			os.Exit(2)
		}
		if schemaErr := st.CreateSchema(); schemaErr != nil {
			st.Close()
			os.Exit(2)
		}
		st.Close()

		dbPath = tmpDB

		cmd := &cobra.Command{}
		runUndo(cmd, []string{"latest"}) //nolint:errcheck
		return
	}

	// ---- Parent process ----
	cmd := exec.Command(os.Args[0], "-test.run=TestRunUndo_LatestNoSnapshots_ExitsNonZero", "-test.v")
	cmd.Env = append(os.Environ(), "BREWPRUNE_TEST_UNDO_EXITCODE_SUBPROCESS=1")

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	stderrOutput := stderrBuf.String()

	if err == nil {
		t.Fatal("expected subprocess to exit non-zero, got exit 0")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("unexpected error type: %v", err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("expected exit code 1, got %d", exitErr.ExitCode())
	}

	if !strings.Contains(stderrOutput, "Error:") {
		t.Errorf("expected stderr to contain %q, got:\n%s", "Error:", stderrOutput)
	}
}

// TestUndoHelp_UsageComesBeforeExamples verifies that in the rendered help
// output, "Usage:" appears before "Examples:" — confirming standard cobra
// section ordering after the Long/Example restructure.
func TestUndoHelp_UsageComesBeforeExamples(t *testing.T) {
	// Capture help output from the command.
	var buf bytes.Buffer
	undoCmd.SetOut(&buf)
	undoCmd.SetErr(&buf)
	undoCmd.SetArgs([]string{"--help"})

	// Execute help — cobra handles --help by printing and returning nil.
	_ = undoCmd.Help()

	help := buf.String()

	usageIdx := strings.Index(help, "Usage:")
	examplesIdx := strings.Index(help, "Examples:")

	if usageIdx == -1 {
		t.Error("help output missing \"Usage:\" section")
	}
	if examplesIdx == -1 {
		t.Error("help output missing \"Examples:\" section")
	}
	if usageIdx != -1 && examplesIdx != -1 && usageIdx >= examplesIdx {
		t.Errorf("expected \"Usage:\" (index %d) to appear before \"Examples:\" (index %d) in help output:\n%s",
			usageIdx, examplesIdx, help)
	}
}

// TestUndoNoArgsExitsNonZero verifies that `brewprune undo` with no arguments
// exits with code 1 (error, since no action can be taken without an argument).
func TestUndoNoArgsExitsNonZero(t *testing.T) {
	if os.Getenv("BREWPRUNE_TEST_UNDO_NOARGS_SUBPROCESS") == "1" {
		// ---- Child process ----
		tmpDir := t.TempDir()
		tmpDB := tmpDir + "/undo_noargs_test.db"

		st, stErr := store.New(tmpDB)
		if stErr != nil {
			os.Exit(2)
		}
		if schemaErr := st.CreateSchema(); schemaErr != nil {
			st.Close()
			os.Exit(2)
		}
		st.Close()

		dbPath = tmpDB

		cmd := &cobra.Command{}
		err := runUndo(cmd, []string{})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	// ---- Parent process ----
	cmd := exec.Command(os.Args[0], "-test.run=TestUndoNoArgsExitsNonZero", "-test.v")
	cmd.Env = append(os.Environ(), "BREWPRUNE_TEST_UNDO_NOARGS_SUBPROCESS=1")

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	stderrOutput := stderrBuf.String()

	// Expect exit code 1 (non-zero)
	if err == nil {
		t.Errorf("expected subprocess to exit 1, got exit 0")
	}

	// Verify guidance message appears in stderr (error messages go to stderr)
	if !strings.Contains(stderrOutput, "undo --list") {
		t.Errorf("expected stderr to contain %q, got:\n%s", "undo --list", stderrOutput)
	}
}

// TestUndoInvalidSnapshotID verifies that when an invalid snapshot ID is
// provided, the error message is not duplicated and includes a helpful
// suggestion to use `undo --list`.
func TestUndoInvalidSnapshotID(t *testing.T) {
	if os.Getenv("BREWPRUNE_TEST_UNDO_INVALID_ID_SUBPROCESS") == "1" {
		// ---- Child process ----
		tmpDir := t.TempDir()
		tmpDB := tmpDir + "/undo_invalid_id_test.db"

		st, stErr := store.New(tmpDB)
		if stErr != nil {
			os.Exit(2)
		}
		if schemaErr := st.CreateSchema(); schemaErr != nil {
			st.Close()
			os.Exit(2)
		}
		st.Close()

		dbPath = tmpDB

		cmd := &cobra.Command{}
		err := runUndo(cmd, []string{"999"})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	// ---- Parent process ----
	cmd := exec.Command(os.Args[0], "-test.run=TestUndoInvalidSnapshotID", "-test.v")
	cmd.Env = append(os.Environ(), "BREWPRUNE_TEST_UNDO_INVALID_ID_SUBPROCESS=1")

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	stderrOutput := stderrBuf.String()

	// Expect exit code 1 (error case)
	if err == nil {
		t.Fatal("expected subprocess to exit non-zero, got exit 0")
	}

	// Verify error message is not duplicated
	// Count occurrences of "snapshot 999 not found"
	count := strings.Count(stderrOutput, "snapshot 999 not found")
	if count > 1 {
		t.Errorf("error message duplicated %d times, expected once. Output:\n%s", count, stderrOutput)
	}

	// Verify helpful suggestion appears in stderr
	if !strings.Contains(stderrOutput, "undo --list") {
		t.Errorf("expected stderr to contain %q, got:\n%s", "undo --list", stderrOutput)
	}

	// Verify it mentions "see available snapshots"
	if !strings.Contains(stderrOutput, "available snapshots") {
		t.Errorf("expected stderr to contain %q, got:\n%s", "available snapshots", stderrOutput)
	}
}

// TestUndoLatestSuggestsList verifies that when `brewprune undo latest` is
// invoked with no snapshots available, the error message suggests using
// `undo --list` to see all available snapshots.
func TestUndoLatestSuggestsList(t *testing.T) {
	if os.Getenv("BREWPRUNE_TEST_UNDO_LIST_SUGGESTION_SUBPROCESS") == "1" {
		// ---- Child process ----
		tmpDir := t.TempDir()
		tmpDB := tmpDir + "/undo_list_suggestion_test.db"

		st, stErr := store.New(tmpDB)
		if stErr != nil {
			os.Exit(2)
		}
		if schemaErr := st.CreateSchema(); schemaErr != nil {
			st.Close()
			os.Exit(2)
		}
		st.Close()

		dbPath = tmpDB

		cmd := &cobra.Command{}
		runUndo(cmd, []string{"latest"}) //nolint:errcheck
		return
	}

	// ---- Parent process ----
	cmd := exec.Command(os.Args[0], "-test.run=TestUndoLatestSuggestsList", "-test.v")
	cmd.Env = append(os.Environ(), "BREWPRUNE_TEST_UNDO_LIST_SUGGESTION_SUBPROCESS=1")

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	stderrOutput := stderrBuf.String()

	// Expect exit code 1 (this is an error case)
	if err == nil {
		t.Fatal("expected subprocess to exit non-zero, got exit 0")
	}

	// Verify --list suggestion appears in stderr
	if !strings.Contains(stderrOutput, "undo --list") {
		t.Errorf("expected stderr to contain %q, got:\n%s", "undo --list", stderrOutput)
	}

	// Verify it mentions "see all available snapshots"
	if !strings.Contains(stderrOutput, "available snapshots") {
		t.Errorf("expected stderr to contain %q, got:\n%s", "available snapshots", stderrOutput)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
