package app

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/brewprune/internal/analyzer"
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

// TestRunExplain_NotFound_ExitsNonZero verifies that explain with a
// nonexistent package calls os.Exit(1) (non-zero exit).
//
// Because os.Exit(1) terminates the process, we use the subprocess pattern:
// the test re-executes itself as a child process with a special environment
// variable, and the parent verifies the exit code is 1.
func TestRunExplain_NotFound_ExitsNonZero(t *testing.T) {
	if os.Getenv("BREWPRUNE_TEST_EXPLAIN_NOTFOUND_SUBPROCESS") == "1" {
		// ---- Child process ----
		// Point at an empty temp DB so GetPackage returns not-found.
		tmpDB := filepath.Join(t.TempDir(), "test.db")
		dbPath = tmpDB

		cmd := &cobra.Command{}
		// runExplain will call os.Exit(1) — the child exits with code 1.
		_ = runExplain(cmd, []string{"nonexistent-package-xyzzy"})
		// Should never reach here.
		os.Exit(0)
		return
	}

	// ---- Parent process ----
	proc := exec.Command(os.Args[0], "-test.run=TestRunExplain_NotFound_ExitsNonZero", "-test.v")
	proc.Env = append(os.Environ(), "BREWPRUNE_TEST_EXPLAIN_NOTFOUND_SUBPROCESS=1")
	err := proc.Run()
	if err == nil {
		t.Error("expected subprocess to exit non-zero for package-not-found, got exit 0")
		return
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Errorf("unexpected error running subprocess: %v", err)
		return
	}
	code := exitErr.ExitCode()
	if code != 1 {
		t.Errorf("expected exit code 1 from subprocess for not-found package, got %d", code)
	}
}

// TestRunExplain_NotFoundPrintedOnce ensures that when explain is invoked with
// a package name that does not exist in the database, the error message is
// written exactly once to stderr (not duplicated by main.go's error handler).
//
// Since runExplain now calls os.Exit(1) for not-found, this test uses the
// subprocess pattern. The parent verifies exit code is 1 (not 0) and that the
// error is not duplicated by also checking there is no double-print.
func TestRunExplain_NotFoundPrintedOnce(t *testing.T) {
	if os.Getenv("BREWPRUNE_TEST_EXPLAIN_PRINTONCE_SUBPROCESS") == "1" {
		// ---- Child process ----
		tmpDB := filepath.Join(t.TempDir(), "test.db")
		dbPath = tmpDB

		cmd := &cobra.Command{}
		_ = runExplain(cmd, []string{"nonexistent-package-xyzzy"})
		os.Exit(0)
		return
	}

	// ---- Parent process ----
	proc := exec.Command(os.Args[0], "-test.run=TestRunExplain_NotFoundPrintedOnce", "-test.v")
	proc.Env = append(os.Environ(), "BREWPRUNE_TEST_EXPLAIN_PRINTONCE_SUBPROCESS=1")

	var stderrBuf bytes.Buffer
	proc.Stderr = &stderrBuf

	err := proc.Run()

	// We expect a non-zero exit (exit 1 from os.Exit(1)).
	if err == nil {
		t.Error("expected subprocess to exit non-zero for not-found package, got exit 0")
		return
	}

	stderrOutput := stderrBuf.String()

	// The error message should appear exactly once.
	const marker = "Error: package not found:"
	count := strings.Count(stderrOutput, marker)
	if count != 1 {
		t.Errorf("expected error message to appear exactly once in stderr, got %d times; stderr: %q",
			count, stderrOutput)
	}
}

// TestRenderExplanation_ScoringNote verifies that renderExplanation outputs the
// scoring direction note explaining that lower usage score means keep.
func TestRenderExplanation_ScoringNote(t *testing.T) {
	// Redirect stdout to capture output.
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	score := &analyzer.ConfidenceScore{
		Package:    "git",
		Score:      5,
		Tier:       "risky",
		UsageScore: 0,
		DepsScore:  5,
		AgeScore:   0,
		TypeScore:  0,
		Reason:     "recently used",
		Explanation: analyzer.ScoreExplanation{
			UsageDetail: "used today",
			DepsDetail:  "no dependents",
			AgeDetail:   "installed 10 days ago",
			TypeDetail:  "leaf package",
		},
	}

	renderExplanation(score, "2025-01-01")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Note:") {
		t.Errorf("expected output to contain 'Note:', got: %q", output)
	}
	if !strings.Contains(output, "recently used") {
		t.Errorf("expected output to contain 'recently used', got: %q", output)
	}
}

// TestRenderExplanation_DetailNotTruncated verifies that a detail string of
// 40 characters renders without "..." (was truncated at 36 chars before fix).
func TestRenderExplanation_DetailNotTruncated(t *testing.T) {
	// Redirect stdout to capture output.
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	// 40-character detail string — previously would be truncated to 33 + "..."
	detail40 := "last used 45 days ago (long detail!!)"
	if len(detail40) != 37 {
		// adjust to be exactly 40 chars
		detail40 = "last used 45 days ago (long detail here!)"
	}
	// Ensure it's at most 50 chars (new limit) and more than 36 (old limit)
	detail40 = "last used 45 days ago -- extended info!!"
	// len = 41, which exceeds old limit of 36 but fits in new limit of 50

	score := &analyzer.ConfidenceScore{
		Package:    "testpkg",
		Score:      60,
		Tier:       "safe",
		UsageScore: 40,
		DepsScore:  20,
		AgeScore:   0,
		TypeScore:  0,
		Reason:     "not used recently",
		Explanation: analyzer.ScoreExplanation{
			UsageDetail: detail40,
			DepsDetail:  "no dependents",
			AgeDetail:   "installed 365 days ago",
			TypeDetail:  "leaf package",
		},
	}

	renderExplanation(score, "2024-01-01")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// The full detail string should appear untruncated (no "..." appended).
	if !strings.Contains(output, detail40) {
		t.Errorf("expected output to contain full detail string %q without truncation, output: %q",
			detail40, output)
	}
	// Double-check that the truncated version is NOT present.
	truncatedVersion := detail40[:33] + "..."
	if strings.Contains(output, truncatedVersion) {
		t.Errorf("output still contains old-truncated string %q; detail column was not widened",
			truncatedVersion)
	}
}

// TestExplainNoteWording verifies that the explain note includes clarification
// about both endpoints of the usage score (0/40 = recently used, 40/40 = never used).
func TestExplainNoteWording(t *testing.T) {
	// Redirect stdout to capture output.
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	score := &analyzer.ConfidenceScore{
		Package:    "testpkg",
		Score:      20,
		Tier:       "medium",
		UsageScore: 20,
		DepsScore:  0,
		AgeScore:   0,
		TypeScore:  0,
		Reason:     "moderate usage",
		Explanation: analyzer.ScoreExplanation{
			UsageDetail: "used 30 days ago",
			DepsDetail:  "no dependents",
			AgeDetail:   "installed 100 days ago",
			TypeDetail:  "leaf package",
		},
	}

	renderExplanation(score, "2024-01-01")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify the note contains the improved wording with both endpoints.
	if !strings.Contains(output, "0/40 means recently used") {
		t.Errorf("expected output to contain '0/40 means recently used', got: %q", output)
	}
	if !strings.Contains(output, "40/40 means no usage ever observed") {
		t.Errorf("expected output to contain '40/40 means no usage ever observed', got: %q", output)
	}
	if !strings.Contains(output, "fewer points toward removal") {
		t.Errorf("expected output to contain 'fewer points toward removal', got: %q", output)
	}
}

// TestExplainNotFoundSuggestion verifies that the error message for a
// nonexistent package suggests both scan (for recently installed packages)
// and checking the package name (for typos).
func TestExplainNotFoundSuggestion(t *testing.T) {
	if os.Getenv("BREWPRUNE_TEST_EXPLAIN_NOTFOUND_SUGGESTION_SUBPROCESS") == "1" {
		// ---- Child process ----
		tmpDB := filepath.Join(t.TempDir(), "test.db")
		dbPath = tmpDB

		cmd := &cobra.Command{}
		_ = runExplain(cmd, []string{"nonexistent-package-xyzzy"})
		os.Exit(0)
		return
	}

	// ---- Parent process ----
	proc := exec.Command(os.Args[0], "-test.run=TestExplainNotFoundSuggestion", "-test.v")
	proc.Env = append(os.Environ(), "BREWPRUNE_TEST_EXPLAIN_NOTFOUND_SUGGESTION_SUBPROCESS=1")

	var stderrBuf bytes.Buffer
	proc.Stderr = &stderrBuf

	err := proc.Run()

	// We expect a non-zero exit (exit 1 from os.Exit(1)).
	if err == nil {
		t.Error("expected subprocess to exit non-zero for not-found package, got exit 0")
		return
	}

	stderrOutput := stderrBuf.String()

	// Verify the error message contains the improved suggestions.
	if !strings.Contains(stderrOutput, "Check the name with") {
		t.Errorf("expected error message to contain 'Check the name with', got: %q", stderrOutput)
	}
	if !strings.Contains(stderrOutput, "brew list") {
		t.Errorf("expected error message to contain 'brew list', got: %q", stderrOutput)
	}
	if !strings.Contains(stderrOutput, "brew search") {
		t.Errorf("expected error message to contain 'brew search', got: %q", stderrOutput)
	}
	if !strings.Contains(stderrOutput, "If you just installed it") {
		t.Errorf("expected error message to contain 'If you just installed it', got: %q", stderrOutput)
	}
	if !strings.Contains(stderrOutput, "brewprune scan") {
		t.Errorf("expected error message to contain 'brewprune scan', got: %q", stderrOutput)
	}
}
