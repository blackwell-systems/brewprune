package app

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/blackwell-systems/brewprune/internal/store"
)

// TestRunStatus_DaemonStoppedSuggestsWatchDaemon verifies that when the
// daemon is not running, the stopped tracking line suggests
// 'brewprune watch --daemon' (not 'brew services start brewprune').
func TestRunStatus_DaemonStoppedSuggestsWatchDaemon(t *testing.T) {
	// Use a temp HOME so getDefaultPIDFile() returns a path with no PID file,
	// making IsDaemonRunning return false (daemon stopped).
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("failed to set HOME: %v", err)
	}
	defer os.Setenv("HOME", origHome)

	// Create a brewprune dir and a fake DB file so runStatus doesn't exit early.
	brewpruneDir := fmt.Sprintf("%s/.brewprune", tmpDir)
	if err := os.MkdirAll(brewpruneDir, 0755); err != nil {
		t.Fatalf("failed to create .brewprune dir: %v", err)
	}
	fakeDB := fmt.Sprintf("%s/brewprune.db", brewpruneDir)
	f, err := os.Create(fakeDB)
	if err != nil {
		t.Fatalf("failed to create temp db: %v", err)
	}
	f.Close()

	// Override global dbPath so runStatus uses our temp DB.
	origDBPath := dbPath
	dbPath = fakeDB
	defer func() { dbPath = origDBPath }()

	// Capture stdout.
	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("failed to create pipe: %v", pipeErr)
	}
	os.Stdout = w

	_ = runStatus(nil, nil)

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()
	os.Stdout = origStdout

	if !strings.Contains(output, "watch --daemon") {
		t.Errorf("expected stopped tracking line to suggest 'watch --daemon', got output:\n%s", output)
	}
	if strings.Contains(output, "brew services start") {
		t.Errorf("expected stopped tracking line NOT to suggest 'brew services start', got output:\n%s", output)
	}
}

// TestRunStatus_PathMissingWithEvents_ShowsNote verifies that when the shim
// directory is not on PATH but usage events exist, a note is printed explaining
// that the events came from the setup self-test rather than real shim tracking.
func TestRunStatus_PathMissingWithEvents_ShowsNote(t *testing.T) {
	// Use a temp HOME so getDefaultPIDFile and the shim dir path both resolve
	// to temp dirs that are definitely not on PATH.
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("failed to set HOME: %v", err)
	}
	defer os.Setenv("HOME", origHome)

	// Create a real SQLite DB with a package and a usage event so totalEvents > 0.
	brewpruneDir := fmt.Sprintf("%s/.brewprune", tmpDir)
	if err := os.MkdirAll(brewpruneDir, 0755); err != nil {
		t.Fatalf("failed to create .brewprune dir: %v", err)
	}
	realDB := fmt.Sprintf("%s/brewprune.db", brewpruneDir)

	st, err := store.New(realDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if err := st.CreateSchema(); err != nil {
		st.Close()
		t.Fatalf("failed to create schema: %v", err)
	}
	// Insert a package and a synthetic usage event.
	_, err = st.DB().Exec(
		"INSERT INTO packages (name, installed_at, install_type, version, tap, is_cask, size_bytes, has_binary, binary_paths) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"git", time.Now().Format(time.RFC3339), "formula", "2.43.0", "homebrew/core", false, 0, true, "",
	)
	if err != nil {
		st.Close()
		t.Fatalf("failed to insert package: %v", err)
	}
	_, err = st.DB().Exec(
		"INSERT INTO usage_events (package, event_type, binary_path, timestamp) VALUES (?, ?, ?, ?)",
		"git", "exec", "/usr/bin/git", time.Now().Format(time.RFC3339),
	)
	if err != nil {
		st.Close()
		t.Fatalf("failed to insert usage event: %v", err)
	}
	st.Close()

	// Override global dbPath so runStatus uses our temp DB.
	origDBPath := dbPath
	dbPath = realDB
	defer func() { dbPath = origDBPath }()

	// Ensure the shim dir (tmpDir/.brewprune/bin) is NOT on PATH.
	// Since tmpDir is a freshly created temp directory it won't be in PATH,
	// so isOnPATH will return false — the condition !pathOK is satisfied.

	// Capture stdout.
	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("failed to create pipe: %v", pipeErr)
	}
	os.Stdout = w

	_ = runStatus(nil, nil)

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()
	os.Stdout = origStdout

	if !strings.Contains(output, "setup self-test") {
		t.Errorf("expected note about setup self-test when PATH is missing and events > 0, got output:\n%s", output)
	}
	if !strings.Contains(output, "Real tracking starts") {
		t.Errorf("expected note about real tracking when PATH is missing and events > 0, got output:\n%s", output)
	}
}

// TestStatusPathConfiguredNotSourced verifies that status distinguishes
// "configured but not sourced" — the shell profile contains the PATH export
// but the current session hasn't sourced it yet.
func TestStatusPathConfiguredNotSourced(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	origShell := os.Getenv("SHELL")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("failed to set HOME: %v", err)
	}
	defer os.Setenv("HOME", origHome)

	// Set SHELL to zsh for consistent test behavior
	if err := os.Setenv("SHELL", "/bin/zsh"); err != nil {
		t.Fatalf("failed to set SHELL: %v", err)
	}
	defer os.Setenv("SHELL", origShell)

	// Create .brewprune directory and the shim bin/ subdirectory (so shimDirExists == true).
	brewpruneDir := fmt.Sprintf("%s/.brewprune", tmpDir)
	shimDir := fmt.Sprintf("%s/bin", brewpruneDir)
	if err := os.MkdirAll(shimDir, 0755); err != nil {
		t.Fatalf("failed to create shim dir: %v", err)
	}

	// Create a real SQLite DB with minimal schema
	realDB := fmt.Sprintf("%s/brewprune.db", brewpruneDir)
	st, err := store.New(realDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if err := st.CreateSchema(); err != nil {
		st.Close()
		t.Fatalf("failed to create schema: %v", err)
	}
	st.Close()

	// Override global dbPath
	origDBPath := dbPath
	dbPath = realDB
	defer func() { dbPath = origDBPath }()

	// Create .zprofile with brewprune PATH export (simulating post-quickstart state)
	zprofile := fmt.Sprintf("%s/.zprofile", tmpDir)
	profileContent := fmt.Sprintf("\n# brewprune shims\nexport PATH=%q:$PATH\n", shimDir)
	if err := os.WriteFile(zprofile, []byte(profileContent), 0644); err != nil {
		t.Fatalf("failed to write .zprofile: %v", err)
	}

	// Ensure shim dir is NOT in current PATH (simulating a session that hasn't sourced yet)
	// The temp dir won't be in PATH, so isOnPATH will return false.

	// Capture stdout
	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("failed to create pipe: %v", pipeErr)
	}
	os.Stdout = w

	_ = runStatus(nil, nil)

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()
	os.Stdout = origStdout

	// Should show "PATH configured (restart shell to activate)"
	if !strings.Contains(output, "PATH configured") {
		t.Errorf("expected 'PATH configured' when shell profile has export but session hasn't sourced, got output:\n%s", output)
	}
	if !strings.Contains(output, "restart shell to activate") {
		t.Errorf("expected 'restart shell to activate' message, got output:\n%s", output)
	}
	// Should NOT show "PATH missing ⚠"
	if strings.Contains(output, "PATH missing ⚠") {
		t.Errorf("should not show 'PATH missing ⚠' when shell profile is configured, got output:\n%s", output)
	}
}

// TestStatusPathNeverConfigured verifies that status shows "PATH missing ⚠"
// when the shell config file does NOT contain the brewprune PATH export.
func TestStatusPathNeverConfigured(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	origShell := os.Getenv("SHELL")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("failed to set HOME: %v", err)
	}
	defer os.Setenv("HOME", origHome)

	// Set SHELL to zsh for consistent test behavior
	if err := os.Setenv("SHELL", "/bin/zsh"); err != nil {
		t.Fatalf("failed to set SHELL: %v", err)
	}
	defer os.Setenv("SHELL", origShell)

	// Create .brewprune directory and the shim bin/ subdirectory (so shimDirExists == true).
	brewpruneDir := fmt.Sprintf("%s/.brewprune", tmpDir)
	shimDirNeverConfigured := fmt.Sprintf("%s/bin", brewpruneDir)
	if err := os.MkdirAll(shimDirNeverConfigured, 0755); err != nil {
		t.Fatalf("failed to create shim dir: %v", err)
	}

	// Create a real SQLite DB with minimal schema
	realDB := fmt.Sprintf("%s/brewprune.db", brewpruneDir)
	st, err := store.New(realDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if err := st.CreateSchema(); err != nil {
		st.Close()
		t.Fatalf("failed to create schema: %v", err)
	}
	st.Close()

	// Override global dbPath
	origDBPath := dbPath
	dbPath = realDB
	defer func() { dbPath = origDBPath }()

	// Create .zprofile WITHOUT brewprune PATH export (simulating pre-quickstart or failed setup)
	zprofile := fmt.Sprintf("%s/.zprofile", tmpDir)
	profileContent := "# Some other shell config\nexport EDITOR=vim\n"
	if err := os.WriteFile(zprofile, []byte(profileContent), 0644); err != nil {
		t.Fatalf("failed to write .zprofile: %v", err)
	}

	// Capture stdout
	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("failed to create pipe: %v", pipeErr)
	}
	os.Stdout = w

	_ = runStatus(nil, nil)

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()
	os.Stdout = origStdout

	// Should show "PATH missing ⚠"
	if !strings.Contains(output, "PATH missing ⚠") {
		t.Errorf("expected 'PATH missing ⚠' when shell profile lacks brewprune export, got output:\n%s", output)
	}
	// Should NOT show "PATH configured"
	if strings.Contains(output, "PATH configured") {
		t.Errorf("should not show 'PATH configured' when shell profile is missing export, got output:\n%s", output)
	}
}

// TestStatusPathConfiguredWithEvents_NoSelfTestNote verifies that when PATH is
// configured in shell profile but not yet sourced, AND usage events exist,
// the "setup self-test" note is NOT shown (it only shows for truly missing PATH).
func TestStatusPathConfiguredWithEvents_NoSelfTestNote(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	origShell := os.Getenv("SHELL")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("failed to set HOME: %v", err)
	}
	defer os.Setenv("HOME", origHome)

	// Set SHELL to zsh for consistent test behavior
	if err := os.Setenv("SHELL", "/bin/zsh"); err != nil {
		t.Fatalf("failed to set SHELL: %v", err)
	}
	defer os.Setenv("SHELL", origShell)

	// Create .brewprune directory and the shim bin/ subdirectory (so shimDirExists == true).
	brewpruneDir := fmt.Sprintf("%s/.brewprune", tmpDir)
	shimDirConfiguredWithEvents := fmt.Sprintf("%s/bin", brewpruneDir)
	if err := os.MkdirAll(shimDirConfiguredWithEvents, 0755); err != nil {
		t.Fatalf("failed to create shim dir: %v", err)
	}

	// Create a real SQLite DB with a package and a usage event
	realDB := fmt.Sprintf("%s/brewprune.db", brewpruneDir)
	st, err := store.New(realDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if err := st.CreateSchema(); err != nil {
		st.Close()
		t.Fatalf("failed to create schema: %v", err)
	}
	// Insert a package and a synthetic usage event.
	_, err = st.DB().Exec(
		"INSERT INTO packages (name, installed_at, install_type, version, tap, is_cask, size_bytes, has_binary, binary_paths) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"git", time.Now().Format(time.RFC3339), "formula", "2.43.0", "homebrew/core", false, 0, true, "",
	)
	if err != nil {
		st.Close()
		t.Fatalf("failed to insert package: %v", err)
	}
	_, err = st.DB().Exec(
		"INSERT INTO usage_events (package, event_type, binary_path, timestamp) VALUES (?, ?, ?, ?)",
		"git", "exec", "/usr/bin/git", time.Now().Format(time.RFC3339),
	)
	if err != nil {
		st.Close()
		t.Fatalf("failed to insert usage event: %v", err)
	}
	st.Close()

	// Override global dbPath
	origDBPath := dbPath
	dbPath = realDB
	defer func() { dbPath = origDBPath }()

	// Create .zprofile with brewprune PATH export (PATH is configured but not sourced)
	shimDir := shimDirConfiguredWithEvents
	zprofile := fmt.Sprintf("%s/.zprofile", tmpDir)
	profileContent := fmt.Sprintf("\n# brewprune shims\nexport PATH=%q:$PATH\n", shimDir)
	if err := os.WriteFile(zprofile, []byte(profileContent), 0644); err != nil {
		t.Fatalf("failed to write .zprofile: %v", err)
	}

	// Ensure shim dir is NOT in current PATH (simulating a session that hasn't sourced yet)
	// The temp dir won't be in PATH, so isOnPATH will return false.

	// Capture stdout
	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("failed to create pipe: %v", pipeErr)
	}
	os.Stdout = w

	_ = runStatus(nil, nil)

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()
	os.Stdout = origStdout

	// Should show "PATH configured (restart shell to activate)"
	if !strings.Contains(output, "PATH configured") {
		t.Errorf("expected 'PATH configured' when shell profile has export but session hasn't sourced, got output:\n%s", output)
	}
	// Should NOT show the self-test note (only appears when PATH is truly missing)
	if strings.Contains(output, "setup self-test") {
		t.Errorf("should not show 'setup self-test' note when PATH is configured (only waiting to be sourced), got output:\n%s", output)
	}
	if strings.Contains(output, "Real tracking starts") {
		t.Errorf("should not show 'Real tracking starts' note when PATH is configured (only waiting to be sourced), got output:\n%s", output)
	}
}

// TestStatusPathActive verifies that status shows "PATH active ✓"
// when the shim directory is already in the current $PATH.
func TestStatusPathActive(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	origPath := os.Getenv("PATH")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("failed to set HOME: %v", err)
	}
	defer os.Setenv("HOME", origHome)

	// Create .brewprune directory
	brewpruneDir := fmt.Sprintf("%s/.brewprune", tmpDir)
	shimDir := fmt.Sprintf("%s/bin", brewpruneDir)
	if err := os.MkdirAll(shimDir, 0755); err != nil {
		t.Fatalf("failed to create shim dir: %v", err)
	}

	// Add shim dir to current PATH (simulating a session that has sourced the profile)
	newPath := fmt.Sprintf("%s:%s", shimDir, origPath)
	if err := os.Setenv("PATH", newPath); err != nil {
		t.Fatalf("failed to set PATH: %v", err)
	}
	defer os.Setenv("PATH", origPath)

	// Create a real SQLite DB with minimal schema
	realDB := fmt.Sprintf("%s/brewprune.db", brewpruneDir)
	st, err := store.New(realDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if err := st.CreateSchema(); err != nil {
		st.Close()
		t.Fatalf("failed to create schema: %v", err)
	}
	st.Close()

	// Override global dbPath
	origDBPath := dbPath
	dbPath = realDB
	defer func() { dbPath = origDBPath }()

	// Capture stdout
	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("failed to create pipe: %v", pipeErr)
	}
	os.Stdout = w

	_ = runStatus(nil, nil)

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()
	os.Stdout = origStdout

	// Should show "PATH active ✓"
	if !strings.Contains(output, "PATH active ✓") {
		t.Errorf("expected 'PATH active ✓' when shim dir is in current PATH, got output:\n%s", output)
	}
	// Should NOT show "PATH missing" or "PATH configured"
	if strings.Contains(output, "PATH missing") {
		t.Errorf("should not show 'PATH missing' when PATH is active, got output:\n%s", output)
	}
	if strings.Contains(output, "PATH configured (restart") {
		t.Errorf("should not show 'PATH configured (restart' when PATH is active, got output:\n%s", output)
	}
}

// TestStatusLastScanNeverWhenZeroFormulae verifies that when the database
// contains zero formulae (i.e. no scan has ever been run), the status output
// shows "never" for the last scan line and does NOT show "just now".
func TestStatusLastScanNeverWhenZeroFormulae(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("failed to set HOME: %v", err)
	}
	defer os.Setenv("HOME", origHome)

	// Create .brewprune directory and an empty (schema-only) DB — no packages.
	brewpruneDir := fmt.Sprintf("%s/.brewprune", tmpDir)
	if err := os.MkdirAll(brewpruneDir, 0755); err != nil {
		t.Fatalf("failed to create .brewprune dir: %v", err)
	}
	realDB := fmt.Sprintf("%s/brewprune.db", brewpruneDir)

	st, err := store.New(realDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if err := st.CreateSchema(); err != nil {
		st.Close()
		t.Fatalf("failed to create schema: %v", err)
	}
	st.Close()

	// Override global dbPath so runStatus uses our temp DB.
	origDBPath := dbPath
	dbPath = realDB
	defer func() { dbPath = origDBPath }()

	// Capture stdout.
	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("failed to create pipe: %v", pipeErr)
	}
	os.Stdout = w

	_ = runStatus(nil, nil)

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()
	os.Stdout = origStdout

	if !strings.Contains(output, "never") {
		t.Errorf("expected 'never' in last scan line when formulaeCount == 0, got output:\n%s", output)
	}
	if strings.Contains(output, "just now") {
		t.Errorf("expected NO 'just now' in last scan line when formulaeCount == 0, got output:\n%s", output)
	}
}

// TestFormatDuration_JustNow verifies that a zero duration returns "just now".
func TestFormatDuration_JustNow(t *testing.T) {
	got := formatDuration(0)
	if got != "just now" {
		t.Errorf("formatDuration(0) = %q, want %q", got, "just now")
	}
}

// TestFormatDuration_FourSeconds verifies that a 4-second duration (sub-threshold) returns "just now".
func TestFormatDuration_FourSeconds(t *testing.T) {
	got := formatDuration(4 * time.Second)
	if got != "just now" {
		t.Errorf("formatDuration(4s) = %q, want %q", got, "just now")
	}
}

// TestFormatDuration_Seconds verifies that a 10-second duration returns "10 seconds ago".
func TestFormatDuration_Seconds(t *testing.T) {
	got := formatDuration(10 * time.Second)
	if got != "10 seconds ago" {
		t.Errorf("formatDuration(10s) = %q, want %q", got, "10 seconds ago")
	}
}

// TestFormatDuration_SingularSecond verifies that a 6-second duration returns "6 seconds ago" (not "6 second ago").
func TestFormatDuration_SingularSecond(t *testing.T) {
	got := formatDuration(6 * time.Second)
	if got != "6 seconds ago" {
		t.Errorf("formatDuration(6s) = %q, want %q", got, "6 seconds ago")
	}
}

// TestFormatDuration_OneSecondBoundary verifies that a 1-second duration (sub-5s threshold) returns "just now".
func TestFormatDuration_OneSecondBoundary(t *testing.T) {
	got := formatDuration(1 * time.Second)
	if got != "just now" {
		t.Errorf("formatDuration(1s) = %q, want %q", got, "just now")
	}
}

// TestStatusPATHMessaging verifies that status.go uses three-state PATH messaging consistently.
func TestStatusPATHMessaging(t *testing.T) {
	tests := []struct {
		name            string
		pathActive      bool
		pathConfigured  bool
		expectedMessage string
	}{
		{
			name:            "PATH active",
			pathActive:      true,
			pathConfigured:  true,
			expectedMessage: "PATH active ✓",
		},
		{
			name:            "PATH configured but not sourced",
			pathActive:      false,
			pathConfigured:  true,
			expectedMessage: "PATH configured (restart shell to activate)",
		},
		{
			name:            "PATH missing",
			pathActive:      false,
			pathConfigured:  false,
			expectedMessage: "PATH missing ⚠",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			origHome := os.Getenv("HOME")
			origPath := os.Getenv("PATH")
			origShell := os.Getenv("SHELL")

			if err := os.Setenv("HOME", tmpDir); err != nil {
				t.Fatalf("failed to set HOME: %v", err)
			}
			defer func() {
				os.Setenv("HOME", origHome)
				os.Setenv("PATH", origPath)
				os.Setenv("SHELL", origShell)
			}()

			// Set shell to zsh for consistent test behavior
			os.Setenv("SHELL", "/bin/zsh")

			// Create .brewprune directory
			brewpruneDir := fmt.Sprintf("%s/.brewprune", tmpDir)
			shimDir := fmt.Sprintf("%s/bin", brewpruneDir)
			if err := os.MkdirAll(shimDir, 0755); err != nil {
				t.Fatalf("failed to create shim dir: %v", err)
			}

			// Setup PATH based on test case
			if tt.pathActive {
				newPath := fmt.Sprintf("%s:%s", shimDir, origPath)
				os.Setenv("PATH", newPath)
			}

			// Setup shell profile based on test case
			if tt.pathConfigured {
				zprofile := fmt.Sprintf("%s/.zprofile", tmpDir)
				profileContent := fmt.Sprintf("\n# brewprune shims\nexport PATH=%q:$PATH\n", shimDir)
				if err := os.WriteFile(zprofile, []byte(profileContent), 0644); err != nil {
					t.Fatalf("failed to write .zprofile: %v", err)
				}
			} else {
				// Create empty .zprofile
				zprofile := fmt.Sprintf("%s/.zprofile", tmpDir)
				if err := os.WriteFile(zprofile, []byte("# empty\n"), 0644); err != nil {
					t.Fatalf("failed to write .zprofile: %v", err)
				}
			}

			// Create a real SQLite DB with minimal schema
			realDB := fmt.Sprintf("%s/brewprune.db", brewpruneDir)
			st, err := store.New(realDB)
			if err != nil {
				t.Fatalf("failed to create store: %v", err)
			}
			if err := st.CreateSchema(); err != nil {
				st.Close()
				t.Fatalf("failed to create schema: %v", err)
			}
			st.Close()

			// Override global dbPath
			origDBPath := dbPath
			dbPath = realDB
			defer func() { dbPath = origDBPath }()

			// Capture stdout
			origStdout := os.Stdout
			r, w, pipeErr := os.Pipe()
			if pipeErr != nil {
				t.Fatalf("failed to create pipe: %v", pipeErr)
			}
			os.Stdout = w

			_ = runStatus(nil, nil)

			w.Close()
			var buf bytes.Buffer
			buf.ReadFrom(r)
			output := buf.String()
			os.Stdout = origStdout

			// Verify expected message
			if !strings.Contains(output, tt.expectedMessage) {
				t.Errorf("expected output to contain %q, got:\n%s", tt.expectedMessage, output)
			}
		})
	}
}

// TestStatus_ShimsLabelWhenZeroShimsPathConfigured verifies that when shimCount == 0
// and PATH is configured in the shell profile but not yet active in the session,
// the Shims line shows "not yet active" and does NOT include "0 shims" or "0 commands".
func TestStatus_ShimsLabelWhenZeroShimsPathConfigured(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	origShell := os.Getenv("SHELL")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("failed to set HOME: %v", err)
	}
	defer os.Setenv("HOME", origHome)
	if err := os.Setenv("SHELL", "/bin/zsh"); err != nil {
		t.Fatalf("failed to set SHELL: %v", err)
	}
	defer os.Setenv("SHELL", origShell)

	// Create .brewprune/bin (shim dir exists but has no symlinks).
	brewpruneDir := fmt.Sprintf("%s/.brewprune", tmpDir)
	shimDir := fmt.Sprintf("%s/bin", brewpruneDir)
	if err := os.MkdirAll(shimDir, 0755); err != nil {
		t.Fatalf("failed to create shim dir: %v", err)
	}

	// Create a real SQLite DB with minimal schema.
	realDB := fmt.Sprintf("%s/brewprune.db", brewpruneDir)
	st, err := store.New(realDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if err := st.CreateSchema(); err != nil {
		st.Close()
		t.Fatalf("failed to create schema: %v", err)
	}
	st.Close()

	// Override global dbPath.
	origDBPath := dbPath
	dbPath = realDB
	defer func() { dbPath = origDBPath }()

	// Write .zprofile with brewprune PATH export so isConfiguredInShellProfile returns true.
	zprofile := fmt.Sprintf("%s/.zprofile", tmpDir)
	profileContent := fmt.Sprintf("\n# brewprune shims\nexport PATH=%q:$PATH\n", shimDir)
	if err := os.WriteFile(zprofile, []byte(profileContent), 0644); err != nil {
		t.Fatalf("failed to write .zprofile: %v", err)
	}

	// shimDir is NOT in current PATH — isOnPATH returns false.

	// Capture stdout.
	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("failed to create pipe: %v", pipeErr)
	}
	os.Stdout = w

	_ = runStatus(nil, nil)

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()
	os.Stdout = origStdout

	// Should show "not yet active" (not "inactive") when shimCount == 0 and PATH is configured.
	if !strings.Contains(output, "not yet active") {
		t.Errorf("expected 'not yet active' in Shims line when shimCount==0 and PATH is configured, got:\n%s", output)
	}
	// Should NOT include "0 shims" or "0 commands".
	if strings.Contains(output, "0 shims") {
		t.Errorf("Shims line should omit count when shimCount==0, but got '0 shims' in:\n%s", output)
	}
	if strings.Contains(output, "0 commands") {
		t.Errorf("Shims line should not say '0 commands', got:\n%s", output)
	}
}

// TestStatus_ShimsLabelWhenShimsPresent verifies that when shimCount > 0 and PATH is
// active, the Shims line shows "N shims" (not "N commands") and says "active".
func TestStatus_ShimsLabelWhenShimsPresent(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	origPath := os.Getenv("PATH")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("failed to set HOME: %v", err)
	}
	defer os.Setenv("HOME", origHome)

	// Create .brewprune/bin with a few symlinks.
	brewpruneDir := fmt.Sprintf("%s/.brewprune", tmpDir)
	shimDir := fmt.Sprintf("%s/bin", brewpruneDir)
	if err := os.MkdirAll(shimDir, 0755); err != nil {
		t.Fatalf("failed to create shim dir: %v", err)
	}

	// Create 3 symlinks in the shim dir.
	for _, name := range []string{"brew", "git", "node"} {
		target := fmt.Sprintf("%s/%s", shimDir, name)
		if err := os.Symlink("/usr/bin/true", target); err != nil {
			t.Fatalf("failed to create symlink %s: %v", name, err)
		}
	}

	// Put shimDir in current PATH so isOnPATH returns true.
	newPath := fmt.Sprintf("%s:%s", shimDir, origPath)
	if err := os.Setenv("PATH", newPath); err != nil {
		t.Fatalf("failed to set PATH: %v", err)
	}
	defer os.Setenv("PATH", origPath)

	// Create a real SQLite DB with minimal schema.
	realDB := fmt.Sprintf("%s/brewprune.db", brewpruneDir)
	st, err := store.New(realDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if err := st.CreateSchema(); err != nil {
		st.Close()
		t.Fatalf("failed to create schema: %v", err)
	}
	st.Close()

	// Override global dbPath.
	origDBPath := dbPath
	dbPath = realDB
	defer func() { dbPath = origDBPath }()

	// Capture stdout.
	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("failed to create pipe: %v", pipeErr)
	}
	os.Stdout = w

	_ = runStatus(nil, nil)

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()
	os.Stdout = origStdout

	// Should show "3 shims" (not "3 commands").
	if !strings.Contains(output, "3 shims") {
		t.Errorf("expected '3 shims' in Shims line when shimCount==3, got:\n%s", output)
	}
	if strings.Contains(output, "commands") {
		t.Errorf("Shims line should not say 'commands', got:\n%s", output)
	}
	// Should show "active".
	if !strings.Contains(output, "active") {
		t.Errorf("expected 'active' in Shims line when PATH is active and shims exist, got:\n%s", output)
	}
}

// TestStatus_ShimDirMissingShowsNotInstalled verifies that when the shim directory
// does not exist at all, the Shims line shows "not installed" rather than "inactive".
func TestStatus_ShimDirMissingShowsNotInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("failed to set HOME: %v", err)
	}
	defer os.Setenv("HOME", origHome)

	// Create .brewprune directory but NOT the bin/ subdirectory.
	brewpruneDir := fmt.Sprintf("%s/.brewprune", tmpDir)
	if err := os.MkdirAll(brewpruneDir, 0755); err != nil {
		t.Fatalf("failed to create .brewprune dir: %v", err)
	}
	// Explicitly confirm bin/ does NOT exist.
	shimDir := fmt.Sprintf("%s/bin", brewpruneDir)
	if _, err := os.Stat(shimDir); err == nil {
		t.Fatalf("shim dir should not exist for this test, but it does: %s", shimDir)
	}

	// Create a real SQLite DB with minimal schema.
	realDB := fmt.Sprintf("%s/brewprune.db", brewpruneDir)
	st, err := store.New(realDB)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if err := st.CreateSchema(); err != nil {
		st.Close()
		t.Fatalf("failed to create schema: %v", err)
	}
	st.Close()

	// Override global dbPath.
	origDBPath := dbPath
	dbPath = realDB
	defer func() { dbPath = origDBPath }()

	// Capture stdout.
	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("failed to create pipe: %v", pipeErr)
	}
	os.Stdout = w

	_ = runStatus(nil, nil)

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()
	os.Stdout = origStdout

	// Should show "not installed".
	if !strings.Contains(output, "not installed") {
		t.Errorf("expected 'not installed' in Shims line when shim dir is missing, got:\n%s", output)
	}
	// Should NOT show "inactive · PATH configured" or "inactive · PATH missing".
	if strings.Contains(output, "inactive") {
		t.Errorf("Shims line should not say 'inactive' when shim dir is missing, got:\n%s", output)
	}
}
