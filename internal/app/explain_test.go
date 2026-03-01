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
// scoring direction framing explaining that higher score = safer to remove.
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

	// The framing note now appears inline before the breakdown components.
	if !strings.Contains(output, "removal confidence score: 0 = keep") {
		t.Errorf("expected output to contain scoring framing 'removal confidence score: 0 = keep', got: %q", output)
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

// TestExplainNoteWording verifies that the explain output includes the inline
// scoring framing and a plain-text breakdown with all four scoring components.
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

	// Verify the inline framing note appears in the breakdown section.
	if !strings.Contains(output, "removal confidence score: 0 = keep") {
		t.Errorf("expected output to contain 'removal confidence score: 0 = keep', got: %q", output)
	}
	// Verify all four scoring components are present in plain-text format.
	if !strings.Contains(output, "Usage:") {
		t.Errorf("expected output to contain 'Usage:', got: %q", output)
	}
	if !strings.Contains(output, "Dependencies:") {
		t.Errorf("expected output to contain 'Dependencies:', got: %q", output)
	}
	if !strings.Contains(output, "Age:") {
		t.Errorf("expected output to contain 'Age:', got: %q", output)
	}
	if !strings.Contains(output, "Type:") {
		t.Errorf("expected output to contain 'Type:', got: %q", output)
	}
}

// captureRenderExplanation is a helper that redirects stdout, calls
// renderExplanation, and returns the captured output.
func captureRenderExplanation(score *analyzer.ConfidenceScore, installedDate string) string {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		panic("failed to create pipe: " + err.Error())
	}
	os.Stdout = w

	renderExplanation(score, installedDate)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

// makeTestScore returns a minimal ConfidenceScore suitable for render tests.
func makeTestScore(pkg string, isCritical bool) *analyzer.ConfidenceScore {
	tier := "safe"
	score := 85
	if isCritical {
		tier = "risky"
		score = 20
	}
	return &analyzer.ConfidenceScore{
		Package:    pkg,
		Score:      score,
		Tier:       tier,
		UsageScore: 40,
		DepsScore:  30,
		AgeScore:   15,
		TypeScore:  10,
		IsCritical: isCritical,
		Reason:     "test reason",
		Explanation: analyzer.ScoreExplanation{
			UsageDetail: "never observed execution",
			DepsDetail:  "no dependents",
			AgeDetail:   "installed 5+ years ago",
			TypeDetail:  "explicitly installed formula",
		},
	}
}

// TestRenderExplanation_ScoreNoteBeforeBreakdown verifies that the score framing
// ("removal confidence score: 0 = keep") appears before the first Usage: component line.
func TestRenderExplanation_ScoreNoteBeforeBreakdown(t *testing.T) {
	output := captureRenderExplanation(makeTestScore("git", false), "2024-01-01")

	noteIdx := strings.Index(output, "removal confidence score: 0 = keep")
	usageIdx := strings.Index(output, "Usage:")
	if noteIdx == -1 {
		t.Errorf("expected output to contain 'removal confidence score: 0 = keep', got: %q", output)
		return
	}
	if usageIdx == -1 {
		t.Errorf("expected output to contain 'Usage:', got: %q", output)
		return
	}
	if noteIdx > usageIdx {
		t.Errorf("score note (pos %d) must appear before 'Usage:' line (pos %d)", noteIdx, usageIdx)
	}
}

// TestRenderExplanation_NoBoxDrawing verifies that the box-drawing table characters
// have been removed from renderExplanation output.
func TestRenderExplanation_NoBoxDrawing(t *testing.T) {
	output := captureRenderExplanation(makeTestScore("node", false), "2024-01-01")

	if strings.ContainsRune(output, '┌') {
		t.Errorf("output must not contain box-drawing character '┌'; got: %q", output)
	}
	if strings.ContainsRune(output, '│') {
		t.Errorf("output must not contain box-drawing character '│'; got: %q", output)
	}
}

// TestRenderExplanation_CriticalTerminology verifies that critical packages use
// "Critical: YES" wording and do not reference "Criticality Penalty" or "-30".
func TestRenderExplanation_CriticalTerminology(t *testing.T) {
	output := captureRenderExplanation(makeTestScore("openssl", true), "2019-01-01")

	if !strings.Contains(output, "Critical: YES") {
		t.Errorf("expected output to contain 'Critical: YES', got: %q", output)
	}
	if strings.Contains(output, "Criticality Penalty") {
		t.Errorf("output must not contain 'Criticality Penalty', got: %q", output)
	}
	if strings.Contains(output, "-30") {
		t.Errorf("output must not contain '-30' penalty wording, got: %q", output)
	}
}

// TestRenderExplanation_NoANSIWhenPiped verifies that when stdout is not a TTY
// (simulated by replacing it with a pipe), no ANSI escape sequences are emitted.
// Since the test itself redirects os.Stdout to a pipe, isColor() inside
// renderExplanation will detect a non-TTY and suppress color codes.
func TestRenderExplanation_NoANSIWhenPiped(t *testing.T) {
	output := captureRenderExplanation(makeTestScore("wget", false), "2023-06-01")

	if strings.Contains(output, "\033[") {
		t.Errorf("output must not contain ANSI escape sequences when stdout is a pipe, got: %q", output)
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

// TestExplainMediumRecommendationIncludesDryRun verifies that for a MEDIUM-tier
// package, the recommendation output contains --dry-run so users preview before removing.
func TestExplainMediumRecommendationIncludesDryRun(t *testing.T) {
	score := &analyzer.ConfidenceScore{
		Package:    "git",
		Score:      45,
		Tier:       "medium",
		UsageScore: 20,
		DepsScore:  15,
		AgeScore:   10,
		TypeScore:  0,
		Reason:     "moderate usage signal",
		Explanation: analyzer.ScoreExplanation{
			UsageDetail: "used 45 days ago",
			DepsDetail:  "no dependents",
			AgeDetail:   "installed 200 days ago",
			TypeDetail:  "leaf package",
		},
	}

	output := captureRenderExplanation(score, "2024-06-01")

	if !strings.Contains(output, "--dry-run") {
		t.Errorf("expected MEDIUM-tier recommendation to contain '--dry-run', got: %q", output)
	}
	if !strings.Contains(output, "brewprune remove git") {
		t.Errorf("expected MEDIUM-tier recommendation to contain 'brewprune remove git', got: %q", output)
	}
}

// TestExplainSafeRecommendationIncludesDryRun verifies that for a SAFE-tier
// package, the recommendation output contains --dry-run so users preview before removing.
func TestExplainSafeRecommendationIncludesDryRun(t *testing.T) {
	score := &analyzer.ConfidenceScore{
		Package:    "wget",
		Score:      85,
		Tier:       "safe",
		UsageScore: 40,
		DepsScore:  25,
		AgeScore:   15,
		TypeScore:  5,
		Reason:     "not used recently, no dependents",
		Explanation: analyzer.ScoreExplanation{
			UsageDetail: "last used 180 days ago",
			DepsDetail:  "no dependents",
			AgeDetail:   "installed 730 days ago",
			TypeDetail:  "leaf package",
		},
	}

	output := captureRenderExplanation(score, "2022-06-01")

	if !strings.Contains(output, "--dry-run") {
		t.Errorf("expected SAFE-tier recommendation to contain '--dry-run', got: %q", output)
	}
	if !strings.Contains(output, "brewprune remove --safe") {
		t.Errorf("expected SAFE-tier recommendation to contain 'brewprune remove --safe', got: %q", output)
	}
}

// TestRunExplain_NilPackageGraceful verifies that when GetPackage returns
// (nil, nil) on the second call in runExplain, the function does not panic.
//
// The nil guard at lines 75-79 of explain.go protects against this:
//
//	pkg, _ := st.GetPackage(packageName)
//	installedDate := ""
//	if pkg != nil {
//	    installedDate = pkg.InstalledAt.Format("2006-01-02")
//	}
//
// In practice, store.GetPackage never returns (nil, nil) — it always wraps
// sql.ErrNoRows as a non-nil error. However the nil guard defends against
// any future store implementation or QEMU-specific race condition that could
// produce this state. This test exercises the renderExplanation path with an
// empty installedDate (the value set when pkg == nil) and verifies no panic.
func TestRunExplain_NilPackageGraceful(t *testing.T) {
	// Simulate the state after the nil guard fires: pkg was nil so installedDate
	// is the empty string "". renderExplanation must not panic in this case.
	score := &analyzer.ConfidenceScore{
		Package:    "curl",
		Score:      15,
		Tier:       "risky",
		UsageScore: 0,
		DepsScore:  5,
		AgeScore:   10,
		TypeScore:  0,
		IsCritical: true,
		Reason:     "core system dependency, keep",
		Explanation: analyzer.ScoreExplanation{
			UsageDetail: "used today",
			DepsDetail:  "4 used dependents",
			AgeDetail:   "installed 730 days ago",
			TypeDetail:  "foundational package (reduced confidence)",
		},
	}

	// Capture output; the key assertion is that this does not panic.
	output := captureRenderExplanation(score, "")

	// With an empty installedDate the "Installed:" line should still render
	// (just with an empty value) rather than crashing.
	if !strings.Contains(output, "Installed:") {
		t.Errorf("expected output to contain 'Installed:' line even with empty date, got: %q", output)
	}
	if !strings.Contains(output, "curl") {
		t.Errorf("expected output to contain package name 'curl', got: %q", output)
	}
}

// TestExplain_PackageNotFoundAfterUndo verifies that when a package is missing
// from the DB (as happens after 'brewprune undo'), runExplain exits with a
// helpful message that includes a hint about running 'brewprune scan', and does
// not crash with a segfault (exit 139).
//
// Uses the subprocess pattern because runExplain calls os.Exit(1) on not-found.
func TestExplain_PackageNotFoundAfterUndo(t *testing.T) {
	if os.Getenv("BREWPRUNE_TEST_EXPLAIN_UNDO_SUBPROCESS") == "1" {
		// ---- Child process ----
		// Use an empty temp DB to simulate post-undo state where packages
		// were removed from the DB (brew reinstalled them, but scan wasn't run).
		tmpDB := filepath.Join(t.TempDir(), "test.db")
		dbPath = tmpDB

		cmd := &cobra.Command{}
		_ = runExplain(cmd, []string{"git"})
		// runExplain calls os.Exit(1) for not-found; should not reach here.
		os.Exit(0)
		return
	}

	// ---- Parent process ----
	proc := exec.Command(os.Args[0], "-test.run=TestExplain_PackageNotFoundAfterUndo", "-test.v")
	proc.Env = append(os.Environ(), "BREWPRUNE_TEST_EXPLAIN_UNDO_SUBPROCESS=1")

	var stderrBuf bytes.Buffer
	proc.Stderr = &stderrBuf

	err := proc.Run()

	// Must exit non-zero (exit 1), not zero, and not 139 (segfault).
	if err == nil {
		t.Error("expected subprocess to exit non-zero for package-not-found after undo, got exit 0")
		return
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Errorf("unexpected error running subprocess: %v", err)
		return
	}
	code := exitErr.ExitCode()
	if code == 139 {
		t.Errorf("subprocess crashed with exit 139 (segfault) — nil pointer dereference after undo state")
		return
	}
	if code != 1 {
		t.Errorf("expected exit code 1 for not-found after undo, got %d", code)
	}

	stderrOutput := stderrBuf.String()
	// The message must suggest running brewprune scan.
	if !strings.Contains(stderrOutput, "brewprune scan") {
		t.Errorf("expected error message to contain 'brewprune scan' hint, got: %q", stderrOutput)
	}
	// The undo-specific hint must be present.
	if !strings.Contains(stderrOutput, "brewprune undo") {
		t.Errorf("expected error message to contain 'brewprune undo' context hint, got: %q", stderrOutput)
	}
}

// TestExplain_ProtectedCountMessage verifies that the Protected line for a
// critical package uses the new descriptive wording instead of the numeric
// "part of 47 core dependencies" which was confusing to new users.
func TestExplain_ProtectedCountMessage(t *testing.T) {
	output := captureRenderExplanation(makeTestScore("openssl", true), "2019-01-01")

	if !strings.Contains(output, "core system dependency") {
		t.Errorf("expected Protected line to contain 'core system dependency', got: %q", output)
	}
	if !strings.Contains(output, "kept even if unused") {
		t.Errorf("expected Protected line to contain 'kept even if unused', got: %q", output)
	}
	// Old wording must be absent.
	if strings.Contains(output, "part of 47 core dependencies") {
		t.Errorf("expected old '47 core dependencies' wording to be removed, got: %q", output)
	}
}

// TestExplain_RecommendationNumberedList verifies that the safe-tier
// recommendation uses the two-step numbered list format.
func TestExplain_RecommendationNumberedList(t *testing.T) {
	output := captureRenderExplanation(makeTestScore("wget", false), "2022-01-01")

	if !strings.Contains(output, "1. Preview:") {
		t.Errorf("expected recommendation to contain '1. Preview:', got: %q", output)
	}
	if !strings.Contains(output, "2. Remove:") {
		t.Errorf("expected recommendation to contain '2. Remove:', got: %q", output)
	}
	// Both steps must reference the safe remove command.
	if !strings.Contains(output, "brewprune remove --safe --dry-run") {
		t.Errorf("expected step 1 to contain 'brewprune remove --safe --dry-run', got: %q", output)
	}
	if !strings.Contains(output, "brewprune remove --safe") {
		t.Errorf("expected step 2 to contain 'brewprune remove --safe', got: %q", output)
	}
}
