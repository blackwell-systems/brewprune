package watcher

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

// StartDaemon starts the watcher as a background daemon process.
// It forks the current process, writes the PID to pidFile, and redirects logs to logFile.
func (w *Watcher) StartDaemon(pidFile, logFile string) error {
	// Check if daemon is already running
	running, err := IsDaemonRunning(pidFile)
	if err != nil {
		return fmt.Errorf("failed to check daemon status: %w", err)
	}
	if running {
		return fmt.Errorf("daemon already running (PID file: %s)", pidFile)
	}

	// Open log file for output
	logF, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer logF.Close()

	// Fork the process
	// Note: This is a simplified daemon implementation
	// In production, you might want to use a proper daemonization library
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	cmd := exec.Command(executable, "watch", "--daemon-child")
	cmd.Stdout = logF
	cmd.Stderr = logF
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create new session
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon process: %w", err)
	}

	// Write PID file
	pid := cmd.Process.Pid
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", pid)), 0644); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Detach from parent
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("failed to release process: %w", err)
	}

	return nil
}

// RunDaemon runs the watcher in daemon mode (called by daemon child process).
// This should be called after forking and should run until stopped.
func (w *Watcher) RunDaemon(pidFile string) error {
	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	// Start the watcher
	if err := w.Start(); err != nil {
		return fmt.Errorf("failed to start watcher: %w", err)
	}

	// Wait for shutdown signal
	sig := <-sigCh
	fmt.Fprintf(os.Stderr, "received signal %v, shutting down...\n", sig)

	// Stop the watcher
	if err := w.Stop(); err != nil {
		return fmt.Errorf("failed to stop watcher: %w", err)
	}

	// Remove PID file
	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove PID file: %w", err)
	}

	return nil
}

// StopDaemon stops a running daemon by sending SIGTERM to the process.
func StopDaemon(pidFile string) error {
	// Read PID from file
	pidData, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("daemon not running (PID file not found)")
		}
		return fmt.Errorf("failed to read PID file: %w", err)
	}

	// Parse PID
	pidStr := strings.TrimSpace(string(pidData))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return fmt.Errorf("invalid PID in file: %w", err)
	}

	// Find the process
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	// Send SIGTERM
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM to process %d: %w", pid, err)
	}

	// Wait a bit for the process to terminate
	// In a real implementation, you might want to wait and retry or send SIGKILL
	return nil
}

// IsDaemonRunning checks if a daemon is running by checking the PID file.
func IsDaemonRunning(pidFile string) (bool, error) {
	// Check if PID file exists
	pidData, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read PID file: %w", err)
	}

	// Parse PID
	pidStr := strings.TrimSpace(string(pidData))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		// Invalid PID file, consider daemon not running
		return false, nil
	}

	// Check if process exists by sending signal 0
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, nil
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		// Process doesn't exist, remove stale PID file
		os.Remove(pidFile)
		return false, nil
	}

	return true, nil
}
