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

	// Create .brewprune directory
	brewpruneDir := fmt.Sprintf("%s/.brewprune", tmpDir)
	if err := os.MkdirAll(brewpruneDir, 0755); err != nil {
		t.Fatalf("failed to create .brewprune dir: %v", err)
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
	shimDir := fmt.Sprintf("%s/.brewprune/bin", tmpDir)
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

	// Create .brewprune directory
	brewpruneDir := fmt.Sprintf("%s/.brewprune", tmpDir)
	if err := os.MkdirAll(brewpruneDir, 0755); err != nil {
		t.Fatalf("failed to create .brewprune dir: %v", err)
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
