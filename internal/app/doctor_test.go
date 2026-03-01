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

// TestDoctorHelpNoFixFlag verifies `doctor --help` output does NOT contain --fix flag mention.
func TestDoctorHelpNoFixFlag(t *testing.T) {
	// Check the Long description directly instead of running a subprocess
	// (subprocess approach doesn't work in test context)
	helpText := doctorCmd.Long

	// Check that --fix flag is NOT mentioned
	if strings.Contains(helpText, "--fix flag") {
		t.Errorf("doctor --help should NOT mention '--fix flag', got:\n%s", helpText)
	}

	// Verify it doesn't mention "not yet implemented"
	if strings.Contains(helpText, "not yet implemented") {
		t.Errorf("doctor --help should NOT mention 'not yet implemented', got:\n%s", helpText)
	}
}

// TestDoctorPATHMessaging verifies that doctor uses three-state PATH messaging.
func TestDoctorPATHMessaging(t *testing.T) {
	tests := []struct {
		name              string
		setupFunc         func(tmpDir string) string // Returns shim dir path
		expectedOutput    string
		shouldHaveWarning bool
	}{
		{
			name: "PATH active",
			setupFunc: func(tmpDir string) string {
				// Create shim dir
				shimDir := filepath.Join(tmpDir, ".brewprune", "bin")
				if err := os.MkdirAll(shimDir, 0755); err != nil {
					t.Fatalf("MkdirAll shimDir: %v", err)
				}
				// Create shim binary
				shimBin := filepath.Join(shimDir, "brewprune-shim")
				if err := os.WriteFile(shimBin, []byte("#!/bin/sh\n"), 0755); err != nil {
					t.Fatalf("WriteFile shimBin: %v", err)
				}
				// Add to PATH
				origPath := os.Getenv("PATH")
				os.Setenv("PATH", shimDir+":"+origPath)
				return shimDir
			},
			expectedOutput:    "✓ PATH active",
			shouldHaveWarning: false,
		},
		{
			name: "PATH configured but not sourced",
			setupFunc: func(tmpDir string) string {
				// Create shim dir
				shimDir := filepath.Join(tmpDir, ".brewprune", "bin")
				if err := os.MkdirAll(shimDir, 0755); err != nil {
					t.Fatalf("MkdirAll shimDir: %v", err)
				}
				// Create shim binary
				shimBin := filepath.Join(shimDir, "brewprune-shim")
				if err := os.WriteFile(shimBin, []byte("#!/bin/sh\n"), 0755); err != nil {
					t.Fatalf("WriteFile shimBin: %v", err)
				}
				// Create .zprofile with PATH export
				os.Setenv("SHELL", "/bin/zsh")
				zprofile := filepath.Join(tmpDir, ".zprofile")
				profileContent := fmt.Sprintf("\n# brewprune shims\nexport PATH=%q:$PATH\n", shimDir)
				if err := os.WriteFile(zprofile, []byte(profileContent), 0644); err != nil {
					t.Fatalf("WriteFile .zprofile: %v", err)
				}
				return shimDir
			},
			expectedOutput:    "⚠ PATH configured (restart shell to activate)",
			shouldHaveWarning: true,
		},
		{
			name: "PATH missing",
			setupFunc: func(tmpDir string) string {
				// Create shim dir
				shimDir := filepath.Join(tmpDir, ".brewprune", "bin")
				if err := os.MkdirAll(shimDir, 0755); err != nil {
					t.Fatalf("MkdirAll shimDir: %v", err)
				}
				// Create shim binary
				shimBin := filepath.Join(shimDir, "brewprune-shim")
				if err := os.WriteFile(shimBin, []byte("#!/bin/sh\n"), 0755); err != nil {
					t.Fatalf("WriteFile shimBin: %v", err)
				}
				// Create .zprofile WITHOUT PATH export
				os.Setenv("SHELL", "/bin/zsh")
				zprofile := filepath.Join(tmpDir, ".zprofile")
				if err := os.WriteFile(zprofile, []byte("# empty\n"), 0644); err != nil {
					t.Fatalf("WriteFile .zprofile: %v", err)
				}
				return shimDir
			},
			expectedOutput:    "⚠ PATH missing",
			shouldHaveWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp environment
			tmpDir := t.TempDir()
			origHome := os.Getenv("HOME")
			origPath := os.Getenv("PATH")
			origShell := os.Getenv("SHELL")

			os.Setenv("HOME", tmpDir)
			defer func() {
				os.Setenv("HOME", origHome)
				os.Setenv("PATH", origPath)
				os.Setenv("SHELL", origShell)
			}()

			// Create database
			realDB := filepath.Join(tmpDir, ".brewprune", "brewprune.db")
			if err := os.MkdirAll(filepath.Dir(realDB), 0755); err != nil {
				t.Fatalf("MkdirAll: %v", err)
			}
			st, err := store.New(realDB)
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
			dbPath = realDB
			defer func() { dbPath = oldDBPath }()

			// Setup test-specific environment
			_ = tt.setupFunc(tmpDir)

			// Capture output
			out := captureStdout(t, func() {
				runDoctor(doctorCmd, []string{}) //nolint:errcheck
			})

			// Verify expected output
			if !strings.Contains(out, tt.expectedOutput) {
				t.Errorf("expected output to contain %q, got:\n%s", tt.expectedOutput, out)
			}
		})
	}
}

// TestDoctorExitCodes verifies that doctor returns proper exit codes.
func TestDoctorExitCodes(t *testing.T) {
	t.Run("exit 0 for warnings only", func(t *testing.T) {
		// This is already tested by TestRunDoctor_WarningOnlyExitsCode0
		// and TestDoctorWarningExitsZero
		t.Skip("Already tested by TestRunDoctor_WarningOnlyExitsCode0")
	})

	t.Run("exit 1 for critical failures", func(t *testing.T) {
		// This is already tested by TestRunDoctor_CriticalIssueReturnsError
		t.Skip("Already tested by TestRunDoctor_CriticalIssueReturnsError")
	})
}

// TestIsColorEnabled_NoColor verifies that setting NO_COLOR=1 makes isColorEnabled return false.
func TestIsColorEnabled_NoColor(t *testing.T) {
	orig := os.Getenv("NO_COLOR")
	os.Setenv("NO_COLOR", "1")
	defer os.Setenv("NO_COLOR", orig)

	if isColorEnabled() {
		t.Error("expected isColorEnabled() to return false when NO_COLOR is set")
	}
}

// TestDoctorPATHHint_ZshConfig verifies that when SHELL=/bin/zsh the PATH
// configured action hint references ~/.zprofile.
func TestDoctorPATHHint_ZshConfig(t *testing.T) {
	origShell := os.Getenv("SHELL")
	os.Setenv("SHELL", "/bin/zsh")
	defer os.Setenv("SHELL", origShell)

	cfg := detectShellConfig()
	if cfg != "~/.zprofile" {
		t.Errorf("expected detectShellConfig() to return ~/.zprofile for zsh, got: %s", cfg)
	}
}

// TestDoctorPATHHint_BashConfig verifies that when SHELL=/bin/bash the PATH
// configured action hint references ~/.bash_profile.
func TestDoctorPATHHint_BashConfig(t *testing.T) {
	origShell := os.Getenv("SHELL")
	os.Setenv("SHELL", "/bin/bash")
	defer os.Setenv("SHELL", origShell)

	cfg := detectShellConfig()
	if cfg != "~/.bash_profile" {
		t.Errorf("expected detectShellConfig() to return ~/.bash_profile for bash, got: %s", cfg)
	}
}

// TestDoctorAliasesTip_SuppressedWhenDaemonRunning verifies that the aliases tip
// is not shown when the daemon is running and events are above the threshold.
// We test this indirectly by setting up an environment where the daemon appears
// running (PID file pointing to current process) and enough usage events exist.
func TestDoctorAliasesTip_SuppressedWhenDaemonRunning(t *testing.T) {
	// Create a temp home directory.
	tmpHome := t.TempDir()

	// Create shim binary so check 6 passes.
	shimDir := filepath.Join(tmpHome, ".brewprune", "bin")
	if err := os.MkdirAll(shimDir, 0755); err != nil {
		t.Fatalf("MkdirAll shimDir: %v", err)
	}
	shimBin := filepath.Join(shimDir, "brewprune-shim")
	if err := os.WriteFile(shimBin, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("WriteFile shimBin: %v", err)
	}

	// Create a database with one package and enough usage events to exceed threshold.
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
	// Insert enough usage events to exceed the 10-event threshold.
	for i := 0; i < 15; i++ {
		_, insertErr := st.DB().Exec(
			"INSERT INTO usage_events (package, event_type, timestamp) VALUES (?, ?, ?)",
			"testpkg", "exec", time.Now().Add(time.Duration(i)*time.Minute),
		)
		if insertErr != nil {
			st.Close()
			t.Fatalf("insert usage event: %v", insertErr)
		}
	}
	st.Close()

	// Override global dbPath and HOME.
	oldDBPath := dbPath
	dbPath = tmpDB
	defer func() { dbPath = oldDBPath }()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Create a PID file pointing to the current process (so daemon appears running).
	pidFile := filepath.Join(tmpHome, ".brewprune", "watch.pid")
	pidContent := fmt.Sprintf("%d", os.Getpid())
	if err := os.WriteFile(pidFile, []byte(pidContent), 0644); err != nil {
		t.Fatalf("WriteFile pidFile: %v", err)
	}

	out := captureStdout(t, func() {
		runDoctor(doctorCmd, []string{}) //nolint:errcheck — pipeline failure is expected
	})

	// The aliases tip should NOT appear when daemon is running with >10 events.
	if strings.Contains(out, "Tip: Create ~/.config/brewprune/aliases") {
		t.Errorf("aliases tip should be suppressed when daemon is running with enough events, got:\n%s", out)
	}
}

// setupMinimalEnv creates a temp home with a real DB (one package) and a stub
// shim binary so that all critical checks pass. It overrides dbPath and HOME
// for the duration of the test, restoring them via t.Cleanup.
// It returns the tmpHome directory.
func setupMinimalEnv(t *testing.T) string {
	t.Helper()
	tmpHome := t.TempDir()

	// Create shim binary.
	shimDir := filepath.Join(tmpHome, ".brewprune", "bin")
	if err := os.MkdirAll(shimDir, 0755); err != nil {
		t.Fatalf("MkdirAll shimDir: %v", err)
	}
	shimBin := filepath.Join(shimDir, "brewprune-shim")
	if err := os.WriteFile(shimBin, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("WriteFile shimBin: %v", err)
	}

	// Create a minimal DB with one package.
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

	oldDBPath := dbPath
	dbPath = tmpDB
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	t.Cleanup(func() {
		dbPath = oldDBPath
		os.Setenv("HOME", origHome)
	})

	return tmpHome
}

// TestDoctorAliasTip_NoCriticalIssues verifies that the alias tip appears in
// doctor output when there are no critical issues and the daemon is not running
// (i.e., the condition !daemonRunning || totalUsageEvents < 10 is true).
func TestDoctorAliasTip_NoCriticalIssues(t *testing.T) {
	tmpHome := setupMinimalEnv(t)

	// Ensure aliases file does not exist so the tip is shown.
	aliasFile := filepath.Join(tmpHome, ".config", "brewprune", "aliases")
	os.Remove(aliasFile) // ignore error — file may not exist

	// No daemon PID file — daemon is not running, so tip should appear.
	out := captureStdout(t, func() {
		runDoctor(doctorCmd, []string{}) //nolint:errcheck
	})

	if !strings.Contains(out, "Tip: Create ~/.config/brewprune/aliases") {
		t.Errorf("alias tip should appear when no critical issues and daemon not running, got:\n%s", out)
	}
}

// TestDoctorAliasTip_HiddenWhenCritical verifies that the alias tip does NOT
// appear when criticalIssues > 0 (e.g., database not found).
func TestDoctorAliasTip_HiddenWhenCritical(t *testing.T) {
	// Point at a path that cannot exist — triggers critical DB-not-found issue.
	oldDBPath := dbPath
	dbPath = "/dev/null/no/such/path/test.db"
	defer func() { dbPath = oldDBPath }()

	out := captureStdout(t, func() {
		runDoctor(doctorCmd, []string{}) //nolint:errcheck — expected to fail
	})

	if strings.Contains(out, "Tip: Create ~/.config/brewprune/aliases") {
		t.Errorf("alias tip must not appear when critical issues are present, got:\n%s", out)
	}
}

// TestDoctorAliasTip_NoBrewpruneHelpReference verifies that the alias tip
// output does NOT reference "brewprune help" (which contains no alias docs).
func TestDoctorAliasTip_NoBrewpruneHelpReference(t *testing.T) {
	tmpHome := setupMinimalEnv(t)

	// Ensure aliases file does not exist so the tip is shown.
	aliasFile := filepath.Join(tmpHome, ".config", "brewprune", "aliases")
	os.Remove(aliasFile) // ignore error

	out := captureStdout(t, func() {
		runDoctor(doctorCmd, []string{}) //nolint:errcheck
	})

	if strings.Contains(out, "brewprune help") {
		t.Errorf("alias tip must not reference 'brewprune help', got:\n%s", out)
	}
}

// TestDoctorPipelineFailureMessage_DaemonRunningPathNotActive verifies that when
// the daemon is running and the shim directory is configured in the shell profile
// but NOT in the active $PATH, the output tells the user to source their profile
// (or restart their shell), rather than blaming the daemon. With the pipeline
// SKIPPED for this state, the PATH check itself carries the source action hint.
func TestDoctorPipelineFailureMessage_DaemonRunningPathNotActive(t *testing.T) {
	tmpHome := setupMinimalEnv(t)

	shimDir := filepath.Join(tmpHome, ".brewprune", "bin")

	// Write a .zprofile that exports the shim dir so isConfiguredInShellProfile
	// returns true, but do NOT add shimDir to $PATH so isOnPATH returns false.
	os.Setenv("SHELL", "/bin/zsh")
	t.Cleanup(func() { os.Unsetenv("SHELL") })
	zprofile := filepath.Join(tmpHome, ".zprofile")
	profileContent := fmt.Sprintf("\n# brewprune shims\nexport PATH=%q:$PATH\n", shimDir)
	if err := os.WriteFile(zprofile, []byte(profileContent), 0644); err != nil {
		t.Fatalf("WriteFile .zprofile: %v", err)
	}

	// Ensure shimDir is NOT in $PATH.
	origPath := os.Getenv("PATH")
	// Strip shimDir from PATH in case it happens to be there.
	parts := strings.Split(origPath, ":")
	filtered := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != shimDir {
			filtered = append(filtered, p)
		}
	}
	os.Setenv("PATH", strings.Join(filtered, ":"))
	t.Cleanup(func() { os.Setenv("PATH", origPath) })

	// Create a PID file pointing at the current process so daemon appears running.
	pidFile := filepath.Join(tmpHome, ".brewprune", "watch.pid")
	pidContent := fmt.Sprintf("%d", os.Getpid())
	if err := os.WriteFile(pidFile, []byte(pidContent), 0644); err != nil {
		t.Fatalf("WriteFile pidFile: %v", err)
	}

	out := captureStdout(t, func() {
		runDoctor(doctorCmd, []string{}) //nolint:errcheck — warnings expected
	})

	// The PATH check or pipeline skip message must mention source/shell restart.
	if !strings.Contains(out, "source") && !strings.Contains(out, "restart your shell") {
		t.Errorf("expected output to mention 'source' or 'restart your shell', got:\n%s", out)
	}
	if strings.Contains(out, "watch --daemon") {
		t.Errorf("output must NOT mention 'watch --daemon' when daemon is running, got:\n%s", out)
	}
}

// TestDoctor_PipelineSkippedWhenPathNotActive verifies that when shims are
// configured in the shell profile but the shim directory is NOT in the active
// $PATH, the pipeline test is reported as SKIPPED (not FAIL) and the output
// does NOT contain "CRITICAL ISSUES".
func TestDoctor_PipelineSkippedWhenPathNotActive(t *testing.T) {
	tmpHome := setupMinimalEnv(t)

	shimDir := filepath.Join(tmpHome, ".brewprune", "bin")

	// Write a .zprofile that exports the shim dir so isConfiguredInShellProfile
	// returns true, but do NOT add shimDir to $PATH so isOnPATH returns false.
	os.Setenv("SHELL", "/bin/zsh")
	t.Cleanup(func() { os.Unsetenv("SHELL") })
	zprofile := filepath.Join(tmpHome, ".zprofile")
	profileContent := fmt.Sprintf("\n# brewprune shims\nexport PATH=%q:$PATH\n", shimDir)
	if err := os.WriteFile(zprofile, []byte(profileContent), 0644); err != nil {
		t.Fatalf("WriteFile .zprofile: %v", err)
	}

	// Ensure shimDir is NOT in $PATH.
	origPath := os.Getenv("PATH")
	parts := strings.Split(origPath, ":")
	filtered := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != shimDir {
			filtered = append(filtered, p)
		}
	}
	os.Setenv("PATH", strings.Join(filtered, ":"))
	t.Cleanup(func() { os.Setenv("PATH", origPath) })

	// Create a PID file pointing at the current process so daemon appears running.
	pidFile := filepath.Join(tmpHome, ".brewprune", "watch.pid")
	pidContent := fmt.Sprintf("%d", os.Getpid())
	if err := os.WriteFile(pidFile, []byte(pidContent), 0644); err != nil {
		t.Fatalf("WriteFile pidFile: %v", err)
	}

	var doctorErr error
	out := captureStdout(t, func() {
		doctorErr = runDoctor(doctorCmd, []string{})
	})

	// Pipeline test must be SKIPPED, not FAIL.
	if !strings.Contains(out, "SKIPPED") {
		t.Errorf("expected pipeline test to be SKIPPED when PATH not active, got:\n%s", out)
	}
	if strings.Contains(out, "FAIL") {
		t.Errorf("pipeline test must not show FAIL when PATH not active, got:\n%s", out)
	}

	// Must NOT report critical issues — this is expected post-install state.
	if strings.Contains(out, "CRITICAL ISSUES") || strings.Contains(out, "critical issue") {
		t.Errorf("doctor must not report critical issues for PATH-not-yet-active state, got:\n%s", out)
	}
	if doctorErr != nil && strings.Contains(doctorErr.Error(), "diagnostics failed") {
		t.Errorf("doctor must not return diagnostics-failed error for PATH-not-yet-active state, got: %v", doctorErr)
	}
}

// TestDoctor_PathActiveShowsActiveCheck verifies that when the shim directory
// is in the active $PATH, the doctor output contains a passing PATH check
// indicating the PATH is active.
func TestDoctor_PathActiveShowsActiveCheck(t *testing.T) {
	tmpHome := setupMinimalEnv(t)

	shimDir := filepath.Join(tmpHome, ".brewprune", "bin")

	// Add shimDir to $PATH so isOnPATH returns true.
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", shimDir+":"+origPath)
	t.Cleanup(func() { os.Setenv("PATH", origPath) })

	out := captureStdout(t, func() {
		runDoctor(doctorCmd, []string{}) //nolint:errcheck
	})

	// Must show PATH active check.
	if !strings.Contains(out, "PATH active") {
		t.Errorf("expected output to contain 'PATH active' when shim dir is in PATH, got:\n%s", out)
	}
}

// TestDoctor_PipelineFailsNormallyWhenPathActive verifies that when the shim
// PATH IS active but the pipeline test fails (e.g., shim doesn't record
// events), the output shows FAIL (not SKIPPED).
func TestDoctor_PipelineFailsNormallyWhenPathActive(t *testing.T) {
	tmpHome := setupMinimalEnv(t)

	shimDir := filepath.Join(tmpHome, ".brewprune", "bin")

	// Add shimDir to $PATH so isOnPATH returns true — PATH is active.
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", shimDir+":"+origPath)
	t.Cleanup(func() { os.Setenv("PATH", origPath) })

	// Create a PID file pointing at the current process so daemon appears running
	// and the pipeline test runs.
	pidFile := filepath.Join(tmpHome, ".brewprune", "watch.pid")
	pidContent := fmt.Sprintf("%d", os.Getpid())
	if err := os.WriteFile(pidFile, []byte(pidContent), 0644); err != nil {
		t.Fatalf("WriteFile pidFile: %v", err)
	}

	out := captureStdout(t, func() {
		runDoctor(doctorCmd, []string{}) //nolint:errcheck — pipeline failure expected
	})

	// Pipeline must attempt to run (not be skipped) — it will fail since the
	// stub shim can't record real events, but it must show "fail" not "SKIPPED".
	if strings.Contains(out, "Pipeline test: SKIPPED") {
		t.Errorf("pipeline test must not be SKIPPED when PATH is active, got:\n%s", out)
	}
	// The pipeline test should reach the spinner and show progress or failure.
	if !strings.Contains(out, "Running pipeline test") && !strings.Contains(out, "fail") {
		t.Errorf("expected pipeline test to run (show progress or fail) when PATH is active, got:\n%s", out)
	}
}

// TestDoctorPipelineFailureMessage_DaemonNotRunning verifies that when the
// pipeline test fails because the daemon is not running, the action message
// tells the user to run 'brewprune watch --daemon'.
//
// Note: the pipeline test is skipped (not failed) when the daemon is not running,
// so this test verifies the skip message does not incorrectly reference
// watch --daemon in the pipeline failure context; instead the daemon check itself
// already outputs the watch --daemon action.  We verify the skip message is shown
// and the daemon check action is present, but no pipeline-failure watch --daemon
// duplicate is emitted.
func TestDoctorPipelineFailureMessage_DaemonNotRunning(t *testing.T) {
	setupMinimalEnv(t)

	// No PID file — daemon is not running.
	// The pipeline test is skipped, so the skip message should appear.
	out := captureStdout(t, func() {
		runDoctor(doctorCmd, []string{}) //nolint:errcheck
	})

	// Verify that when daemon is not running, the daemon check action mentions watch --daemon.
	if !strings.Contains(out, "watch --daemon") {
		t.Errorf("expected output to mention 'watch --daemon' when daemon is not running, got:\n%s", out)
	}

	// The pipeline test should be skipped (not failed) since daemon is not running.
	if !strings.Contains(out, "Pipeline test skipped") {
		t.Errorf("expected 'Pipeline test skipped' message when daemon not running, got:\n%s", out)
	}
}
