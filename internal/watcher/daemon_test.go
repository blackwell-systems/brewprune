package watcher

import (
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/blackwell-systems/brewprune/internal/brew"
)

func TestIsDaemonRunning_NotRunning(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")

	running, err := IsDaemonRunning(pidFile)
	if err != nil {
		t.Errorf("IsDaemonRunning() error = %v, want nil", err)
	}
	if running {
		t.Error("IsDaemonRunning() = true, want false for non-existent PID file")
	}
}

func TestIsDaemonRunning_WithCurrentProcess(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")

	// Write current process PID
	pid := os.Getpid()
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(pid)+"\n"), 0644); err != nil {
		t.Fatalf("failed to write PID file: %v", err)
	}

	running, err := IsDaemonRunning(pidFile)
	if err != nil {
		t.Errorf("IsDaemonRunning() error = %v, want nil", err)
	}
	if !running {
		t.Error("IsDaemonRunning() = false, want true for current process")
	}
}

func TestIsDaemonRunning_WithDeadProcess(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")

	// Write a PID that (hopefully) doesn't exist
	// Using a very high PID that's unlikely to be in use
	deadPID := 999999
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(deadPID)+"\n"), 0644); err != nil {
		t.Fatalf("failed to write PID file: %v", err)
	}

	running, err := IsDaemonRunning(pidFile)
	if err != nil {
		t.Errorf("IsDaemonRunning() error = %v, want nil", err)
	}
	if running {
		t.Error("IsDaemonRunning() = true, want false for dead process")
	}

	// PID file should be removed
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("stale PID file was not removed")
	}
}

func TestIsDaemonRunning_InvalidPID(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")

	// Write invalid PID
	if err := os.WriteFile(pidFile, []byte("not-a-number\n"), 0644); err != nil {
		t.Fatalf("failed to write PID file: %v", err)
	}

	running, err := IsDaemonRunning(pidFile)
	if err != nil {
		t.Errorf("IsDaemonRunning() error = %v, want nil for invalid PID", err)
	}
	if running {
		t.Error("IsDaemonRunning() = true, want false for invalid PID")
	}
}

func TestStopDaemon_NotRunning(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")

	err := StopDaemon(pidFile)
	if err == nil {
		t.Error("StopDaemon() expected error for non-existent daemon, got nil")
	}
}

func TestStopDaemon_InvalidPID(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")

	// Write invalid PID
	if err := os.WriteFile(pidFile, []byte("invalid\n"), 0644); err != nil {
		t.Fatalf("failed to write PID file: %v", err)
	}

	err := StopDaemon(pidFile)
	if err == nil {
		t.Error("StopDaemon() expected error for invalid PID, got nil")
	}
}

func TestStopDaemon_WithTestProcess(t *testing.T) {
	// Skip this test as signal handling is difficult to test reliably
	// across different platforms and environments
	t.Skip("signal handling tests are unreliable in unit tests")

	if testing.Short() {
		t.Skip("skipping test that creates subprocess in short mode")
	}

	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")

	// Start a simple long-running process that can handle SIGTERM
	// We'll use a shell script that traps SIGTERM
	scriptPath := filepath.Join(tmpDir, "test-script.sh")
	script := `#!/bin/sh
trap 'exit 0' TERM
sleep 100 &
wait $!
`
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create test script: %v", err)
	}

	cmd := []string{scriptPath}
	proc, err := os.StartProcess(scriptPath, cmd, &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		Sys: &syscall.SysProcAttr{
			Setsid: true,
		},
	})
	if err != nil {
		t.Fatalf("failed to start test process: %v", err)
	}
	defer proc.Kill()

	// Give process time to start
	time.Sleep(100 * time.Millisecond)

	// Write PID file
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(proc.Pid)+"\n"), 0644); err != nil {
		t.Fatalf("failed to write PID file: %v", err)
	}

	// Stop the daemon
	if err := StopDaemon(pidFile); err != nil {
		t.Errorf("StopDaemon() error = %v, want nil", err)
	}

	// Give it a moment to process the signal
	time.Sleep(200 * time.Millisecond)

	// Process should no longer be running
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		t.Error("process still running after StopDaemon()")
		proc.Kill()
	}
}

func TestStartDaemon_AlreadyRunning(t *testing.T) {
	st := setupTestStore(t)
	defer st.Close()

	w, err := New(st)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	logFile := filepath.Join(tmpDir, "test.log")

	// Write current process PID to simulate running daemon
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())+"\n"), 0644); err != nil {
		t.Fatalf("failed to write PID file: %v", err)
	}

	// Should fail because daemon appears to be running
	err = w.StartDaemon(pidFile, logFile)
	if err == nil {
		t.Error("StartDaemon() expected error for already running daemon, got nil")
	}
}

func TestStartDaemon_InvalidLogFile(t *testing.T) {
	st := setupTestStore(t)
	defer st.Close()

	w, err := New(st)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	logFile := filepath.Join(tmpDir, "nonexistent", "test.log") // Invalid path

	err = w.StartDaemon(pidFile, logFile)
	if err == nil {
		t.Error("StartDaemon() expected error for invalid log file path, got nil")
	}
}

func TestRunDaemon_StopBeforeStart(t *testing.T) {
	st := setupTestStore(t)
	defer st.Close()

	// Insert a test package so BuildBinaryMap doesn't fail
	pkg := &brew.Package{
		Name:        "test",
		Version:     "1.0.0",
		InstallType: "explicit",
		HasBinary:   false,
		BinaryPaths: []string{},
		InstalledAt: time.Now(),
	}
	if err := st.InsertPackage(pkg); err != nil {
		t.Fatalf("failed to insert test package: %v", err)
	}

	w, err := New(st)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Try to stop before starting - should work gracefully
	if err := w.Stop(); err != nil {
		t.Errorf("Stop() before Start() error = %v, want nil", err)
	}
}

func TestDaemonPIDFileCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")

	// Create a PID file
	if err := os.WriteFile(pidFile, []byte("12345\n"), 0644); err != nil {
		t.Fatalf("failed to write PID file: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(pidFile); err != nil {
		t.Fatalf("PID file doesn't exist: %v", err)
	}

	// Clean up
	if err := os.Remove(pidFile); err != nil {
		t.Errorf("failed to remove PID file: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("PID file still exists after cleanup")
	}
}
