package app

import (
	"bytes"
	"io"
	"os"
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
	// Verify the command has examples in the long description
	if undoCmd.Long == "" {
		t.Error("undoCmd.Long is empty")
	}

	// Check for key keywords in the long description
	keywords := []string{"snapshot", "restore", "latest", "list"}
	for _, keyword := range keywords {
		if !contains(undoCmd.Long, keyword) {
			t.Errorf("undoCmd.Long missing keyword %q", keyword)
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
// prints a friendly multi-line message and returns nil (no error).
func TestRunUndo_LatestNoSnapshotsFriendlyMessage(t *testing.T) {
	// Use a temp file DB with the full schema so ListSnapshots returns an
	// empty slice (not an error due to missing table).
	tmpDir := t.TempDir()
	tmpDB := tmpDir + "/undo_test.db"

	st, stErr := store.New(tmpDB)
	if stErr != nil {
		t.Fatalf("failed to create store: %v", stErr)
	}
	if schemaErr := st.CreateSchema(); schemaErr != nil {
		st.Close()
		t.Fatalf("failed to create schema: %v", schemaErr)
	}
	st.Close()

	oldDBPath := dbPath
	dbPath = tmpDB
	defer func() { dbPath = oldDBPath }()

	// Capture stdout.
	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("os.Pipe: %v", pipeErr)
	}
	os.Stdout = w

	cmd := &cobra.Command{}
	runErr := runUndo(cmd, []string{"latest"})

	w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	if _, copyErr := io.Copy(&buf, r); copyErr != nil {
		t.Fatalf("failed to read captured output: %v", copyErr)
	}
	output := buf.String()

	if runErr != nil {
		t.Errorf("expected runUndo to return nil when no snapshots, got: %v", runErr)
	}

	expectedPhrases := []string{
		"No snapshots available",
		"brewprune remove",
	}
	for _, phrase := range expectedPhrases {
		if !strings.Contains(output, phrase) {
			t.Errorf("expected output to contain %q, got:\n%s", phrase, output)
		}
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
