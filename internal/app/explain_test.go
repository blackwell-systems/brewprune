package app

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestRunExplain_MissingArgError verifies that passing no arguments to
// explainCmd returns an error whose message contains "missing package name".
func TestRunExplain_MissingArgError(t *testing.T) {
	// Build a temporary root so we don't pollute the global command state.
	root := &cobra.Command{Use: "brewprune", SilenceUsage: true, SilenceErrors: true}

	// We need a fresh copy of the command to avoid state issues; re-use the
	// package-level explainCmd directly since it is stateless (no flags).
	root.AddCommand(explainCmd)

	var errBuf bytes.Buffer
	root.SetErr(&errBuf)

	root.SetArgs([]string{"explain"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected an error when no package name is provided, got nil")
	}
	if !strings.Contains(err.Error(), "missing package name") {
		t.Errorf("error message should contain 'missing package name', got: %q", err.Error())
	}
}

// TestRunExplain_NotFoundPrintedOnce ensures that when explain is invoked with
// a package name that does not exist in the database, the error message is
// written exactly once to stderr (not duplicated by main.go's error handler).
//
// The implementation uses fmt.Fprintf(os.Stderr, ...) + return nil, so RunE
// returns nil and main.go never calls its error path.  We verify this by
// checking that RunE returns nil.
func TestRunExplain_NotFoundPrintedOnce(t *testing.T) {
	// Point the DB flag at an empty temp directory so store.New succeeds but
	// GetPackage returns an error (package not in DB).
	tmpDB := t.TempDir() + "/test.db"

	oldDBPath := dbPath
	dbPath = tmpDB
	defer func() { dbPath = oldDBPath }()

	cmd := &cobra.Command{}
	err := runExplain(cmd, []string{"nonexistent-package-xyzzy"})
	if err != nil {
		t.Errorf("runExplain should return nil for missing package (print-to-stderr path), got: %v", err)
	}
}
