package app

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blackwell-systems/brewprune/internal/brew"
	"github.com/blackwell-systems/brewprune/internal/store"
)

// TestRunDoctor_WarningOnlyExitsCode0 verifies that when the doctor command
// encounters warnings (but no critical failures), it returns nil (exit code 0)
// rather than returning an error or calling os.Exit.
func TestRunDoctor_WarningOnlyExitsCode0(t *testing.T) {
	// Create a minimal environment where DB checks pass but daemon check warns.
	tmpDir := t.TempDir()
	tmpDB := filepath.Join(tmpDir, "test.db")

	// Create a real database with one package so DB checks pass.
	st, err := store.New(tmpDB)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	if err := st.CreateSchema(); err != nil {
		st.Close()
		t.Fatalf("CreateSchema: %v", err)
	}
	pkg := &brew.Package{
		Name:        "testpkg",
		Version:     "1.0",
		InstalledAt: time.Now(),
		InstallType: "explicit",
	}
	if err := st.InsertPackage(pkg); err != nil {
		st.Close()
		t.Fatalf("InsertPackage: %v", err)
	}
	st.Close()

	// Override global dbPath
	oldDBPath := dbPath
	dbPath = tmpDB
	defer func() { dbPath = oldDBPath }()

	// Create a temp home with shim binary so check 6 passes
	tmpHome := t.TempDir()
	shimDir := filepath.Join(tmpHome, ".brewprune", "bin")
	if err := os.MkdirAll(shimDir, 0755); err != nil {
		t.Fatalf("MkdirAll shimDir: %v", err)
	}
	shimBin := filepath.Join(shimDir, "brewprune-shim")
	if err := os.WriteFile(shimBin, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("WriteFile shimBin: %v", err)
	}
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// No daemon PID file, so daemon check will produce a warning.
	// Pipeline test will be skipped since daemon is not running.
	err = runDoctor(doctorCmd, []string{})
	if err != nil {
		t.Errorf("expected runDoctor to return nil for warnings-only, got: %v", err)
	}
}

// TestRunDoctor_CriticalIssueReturnsError verifies that when runDoctor
// encounters a critical issue, it returns a non-nil error so main.go can
// print "Error: diagnostics failed" and exit 1.
func TestRunDoctor_CriticalIssueReturnsError(t *testing.T) {
	// Point at a path that cannot exist so DB stat fails as critical.
	oldDBPath := dbPath
	dbPath = "/dev/null/no/such/path/test.db"
	defer func() { dbPath = oldDBPath }()

	err := runDoctor(doctorCmd, []string{})
	if err == nil {
		t.Error("expected runDoctor to return non-nil error for critical issues")
	}
	if !strings.Contains(err.Error(), "diagnostics failed") {
		t.Errorf("expected error to contain 'diagnostics failed', got: %v", err)
	}
}

// captureStdout replaces os.Stdout with a pipe during f(), then restores it
// and returns all bytes written to stdout.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	f()

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

// TestRunDoctor_ActionLabelNotFix verifies that the doctor output never contains
// the string "Fix:" — all action hints must use "Action:" instead.
func TestRunDoctor_ActionLabelNotFix(t *testing.T) {
	oldDBPath := dbPath
	// Point at a missing DB so doctor prints its "not found" output (which
	// previously contained "Fix:").
	dbPath = "/dev/null/no/such/path/test.db"
	defer func() { dbPath = oldDBPath }()

	out := captureStdout(t, func() {
		runDoctor(doctorCmd, []string{}) //nolint:errcheck — expected to fail
	})

	if strings.Contains(out, "Fix:") {
		t.Errorf("doctor output must not contain 'Fix:' — found it in:\n%s", out)
	}
}

// TestRunDoctor_PipelineTestShowsProgress verifies that when all critical checks
// pass, doctor shows a progress indication ("Running pipeline test...") before
// reporting the pipeline result.
//
// This test sets up a minimal but complete environment: a temp home with a real
// database (containing one package) and a stub shim binary, so that checks 1–6
// all pass and check 8 (the pipeline test) is reached.  The pipeline test will
// fail (the stub shim won't actually record events), but the progress line must
// still appear in the output before the failure message.
func TestRunDoctor_PipelineTestShowsProgress(t *testing.T) {
	// Create a temp home directory that the shim package will use for its
	// default path (~/.brewprune/bin).
	tmpHome := t.TempDir()
	shimDir := filepath.Join(tmpHome, ".brewprune", "bin")
	if err := os.MkdirAll(shimDir, 0755); err != nil {
		t.Fatalf("MkdirAll shimDir: %v", err)
	}

	// Create a stub shim binary (empty executable) so check 6 passes.
	shimBin := filepath.Join(shimDir, "brewprune-shim")
	if err := os.WriteFile(shimBin, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("WriteFile shimBin: %v", err)
	}

	// Create a real database with one package so checks 2 & 3 pass.
	tmpDB := filepath.Join(tmpHome, "test.db")
	st, err := store.New(tmpDB)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	if err := st.CreateSchema(); err != nil {
		st.Close()
		t.Fatalf("CreateSchema: %v", err)
	}
	pkg := &brew.Package{
		Name:        "testpkg",
		Version:     "1.0",
		InstalledAt: time.Now(),
		InstallType: "explicit",
	}
	if err := st.InsertPackage(pkg); err != nil {
		st.Close()
		t.Fatalf("InsertPackage: %v", err)
	}
	st.Close()

	// Override global dbPath and HOME so getDBPath() and GetShimDir() both
	// resolve into our temp directory.
	oldDBPath := dbPath
	dbPath = tmpDB
	defer func() { dbPath = oldDBPath }()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Create a PID file so the daemon check passes and the pipeline test runs.
	pidFile := filepath.Join(tmpHome, ".brewprune", "watch.pid")
	pidContent := fmt.Sprintf("%d", os.Getpid()) // Use current process PID so it appears running
	if err := os.WriteFile(pidFile, []byte(pidContent), 0644); err != nil {
		t.Fatalf("WriteFile pidFile: %v", err)
	}

	out := captureStdout(t, func() {
		runDoctor(doctorCmd, []string{}) //nolint:errcheck — pipeline failure is expected
	})

	// The spinner must have emitted the progress line (non-TTY path prints
	// the message once with a trailing newline).
	if !strings.Contains(out, "Running pipeline test") {
		t.Errorf("expected doctor output to contain 'Running pipeline test', got:\n%s", out)
	}
}

// TestDoctorWarningExitsZero verifies that doctor exits with code 0 (not 1) when only warnings found.
// This test uses a subprocess pattern to verify the exit code behavior.
// Per the UX audit finding, warnings-only should exit 0 so scripting workflows aren't broken.
func TestDoctorWarningExitsZero(t *testing.T) {
	if os.Getenv("BREWPRUNE_TEST_DOCTOR_WARNING_SUBPROCESS") == "1" {
		// ---- Child process ----
		// Create a minimal environment where DB checks pass but daemon check warns.
		tmpDir := t.TempDir()
		tmpDB := filepath.Join(tmpDir, "test.db")

		// Create a real database with one package so DB checks pass.
		st, err := store.New(tmpDB)
		if err != nil {
			t.Fatalf("store.New: %v", err)
		}
		if err := st.CreateSchema(); err != nil {
			st.Close()
			t.Fatalf("CreateSchema: %v", err)
		}
		pkg := &brew.Package{
			Name:        "testpkg",
			Version:     "1.0",
			InstalledAt: time.Now(),
			InstallType: "explicit",
		}
		if err := st.InsertPackage(pkg); err != nil {
			st.Close()
			t.Fatalf("InsertPackage: %v", err)
		}
		st.Close()

		// Override global dbPath
		dbPath = tmpDB

		// Create a temp home with shim binary so check 6 passes
		tmpHome := t.TempDir()
		shimDir := filepath.Join(tmpHome, ".brewprune", "bin")
		if err := os.MkdirAll(shimDir, 0755); err != nil {
			t.Fatalf("MkdirAll shimDir: %v", err)
		}
		shimBin := filepath.Join(shimDir, "brewprune-shim")
		if err := os.WriteFile(shimBin, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatalf("WriteFile shimBin: %v", err)
		}
		os.Setenv("HOME", tmpHome)

		// No daemon PID file, so daemon check will produce a warning and exit 0.
		// Pipeline test will be skipped since daemon is not running.
		// This should result in warnings-only (no critical issues) and thus exit 0.
		runDoctor(doctorCmd, []string{}) //nolint:errcheck
		return
	}

	// ---- Parent process ----
	cmd := exec.Command(os.Args[0], "-test.run=TestDoctorWarningExitsZero", "-test.v")
	cmd.Env = append(os.Environ(), "BREWPRUNE_TEST_DOCTOR_WARNING_SUBPROCESS=1")
	err := cmd.Run()

	if exitErr, ok := err.(*exec.ExitError); ok {
		code := exitErr.ExitCode()
		t.Errorf("expected exit code 0 from doctor with warnings-only, got exit code %d", code)
	} else if err != nil {
		t.Errorf("unexpected error running subprocess: %v", err)
	}
	// else err == nil means exit 0, which is what we want
}

// TestDoctorHelpIncludesFixNote verifies `doctor --help` output contains the --fix note.
func TestDoctorHelpIncludesFixNote(t *testing.T) {
	// Capture the help output by invoking the command with --help
	cmd := exec.Command(os.Args[0], "doctor", "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// --help exits with code 0, but if there's an actual error, fail
		if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 0 {
			t.Fatalf("failed to run doctor --help: %v\nOutput: %s", err, out)
		}
	}

	helpText := string(out)

	// Check for the key phrases from the --fix note
	if !strings.Contains(helpText, "--fix flag is not yet implemented") {
		t.Errorf("doctor --help should mention '--fix flag is not yet implemented', got:\n%s", helpText)
	}

	if !strings.Contains(helpText, "brewprune quickstart") {
		t.Errorf("doctor --help should mention 'brewprune quickstart' as fix alternative, got:\n%s", helpText)
	}
}
