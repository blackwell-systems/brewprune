package app

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/blackwell-systems/brewprune/internal/output"
	"github.com/blackwell-systems/brewprune/internal/store"
	"github.com/blackwell-systems/brewprune/internal/watcher"
	"github.com/spf13/cobra"
)

var (
	watchDaemon      bool
	watchDaemonChild bool
	watchPIDFile     string
	watchLogFile     string
	watchStop        bool

	watchCmd = &cobra.Command{
		Use:   "watch",
		Short: "Process shim log and record package usage",
		Long: `Process the PATH shim usage log to track which Homebrew packages you use.

When you run a shimmed command (e.g. git, gh, jq), the shim binary appends
an entry to ~/.brewprune/usage.log. The watch daemon reads that log every 30
seconds and records resolved usage events in the database. This data drives
the confidence scores shown by 'brewprune unused'.

Run 'brewprune scan' first to build the shim binary and create symlinks.
Then add ~/.brewprune/bin to the front of your PATH.

Watch modes:
  • Foreground (default): Run in current terminal with Ctrl+C to stop
  • Daemon: Run as background process (recommended)
  • Stop: Stop a running daemon

Usage data is written every 30 seconds to minimise I/O overhead.`,
		Example: `  # Run in foreground (Ctrl+C to stop)
  brewprune watch

  # Run as background daemon
  brewprune watch --daemon

  # Stop running daemon
  brewprune watch --stop

  # Use custom PID and log files
  brewprune watch --daemon --pid-file /tmp/watch.pid --log-file /tmp/watch.log`,
		RunE: runWatch,
	}
)

func init() {
	watchCmd.Flags().BoolVar(&watchDaemon, "daemon", false, "run as background daemon")
	watchCmd.Flags().BoolVar(&watchDaemonChild, "daemon-child", false, "internal flag for daemon child process")
	watchCmd.Flags().StringVar(&watchPIDFile, "pid-file", "", "PID file path (default: ~/.brewprune/watch.pid)")
	watchCmd.Flags().StringVar(&watchLogFile, "log-file", "", "log file path (default: ~/.brewprune/watch.log)")
	watchCmd.Flags().BoolVar(&watchStop, "stop", false, "stop running daemon")

	// Hide the internal daemon-child flag from help
	watchCmd.Flags().MarkHidden("daemon-child")
}

func runWatch(cmd *cobra.Command, args []string) error {
	// Get default paths if not specified
	if watchPIDFile == "" {
		defaultPID, err := getDefaultPIDFile()
		if err != nil {
			return fmt.Errorf("failed to get default PID file path: %w", err)
		}
		watchPIDFile = defaultPID
	}

	if watchLogFile == "" {
		defaultLog, err := getDefaultLogFile()
		if err != nil {
			return fmt.Errorf("failed to get default log file path: %w", err)
		}
		watchLogFile = defaultLog
	}

	// Handle stop command
	if watchStop {
		return stopWatchDaemon()
	}

	// Get database path
	dbPath, err := getDBPath()
	if err != nil {
		return fmt.Errorf("failed to get database path: %w", err)
	}

	// Open database
	db, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Create schema if needed
	if err := db.CreateSchema(); err != nil {
		return fmt.Errorf("failed to create database schema: %w", err)
	}

	// Create watcher
	w, err := watcher.New(db)
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	// Handle daemon mode
	if watchDaemon {
		return startWatchDaemon(w)
	}

	// Handle daemon child process
	if watchDaemonChild {
		return runWatchDaemonChild(w)
	}

	// Run in foreground
	return runWatchForeground(w)
}

func stopWatchDaemon() error {
	// Check if daemon is running
	running, err := watcher.IsDaemonRunning(watchPIDFile)
	if err != nil {
		return fmt.Errorf("failed to check daemon status: %w", err)
	}

	if !running {
		fmt.Println("Daemon is not running")
		return nil
	}

	spinner := output.NewSpinner("Stopping daemon...")
	if err := watcher.StopDaemon(watchPIDFile); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to stop daemon: %w", err)
	}
	spinner.StopWithMessage("✓ Daemon stopped")

	return nil
}

func startWatchDaemon(w *watcher.Watcher) error {
	// Check if already running
	running, err := watcher.IsDaemonRunning(watchPIDFile)
	if err != nil {
		return fmt.Errorf("failed to check daemon status: %w", err)
	}

	if running {
		return fmt.Errorf("daemon already running (PID file: %s)", watchPIDFile)
	}

	spinner := output.NewSpinner("Starting daemon...")
	if err := w.StartDaemon(watchPIDFile, watchLogFile); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to start daemon: %w", err)
	}
	spinner.StopWithMessage("✓ Daemon started")

	fmt.Printf("\nUsage tracking daemon started\n")
	fmt.Printf("  PID file: %s\n", watchPIDFile)
	fmt.Printf("  Log file: %s\n", watchLogFile)
	fmt.Printf("\nTo stop: brewprune watch --stop\n")

	return nil
}

func runWatchDaemonChild(w *watcher.Watcher) error {
	// This runs as the daemon child process
	// It should not print to stdout/stderr as they're redirected to log file
	return w.RunDaemon(watchPIDFile)
}

func runWatchForeground(w *watcher.Watcher) error {
	fmt.Println("Starting shim log processor (press Ctrl+C to stop)...")
	fmt.Println()

	spinner := output.NewSpinner("Processing usage log...")

	// Start the watcher
	if err := w.Start(); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to start watcher: %w", err)
	}
	spinner.StopWithMessage("✓ Shim log processor started")

	fmt.Println()
	fmt.Println("Processing ~/.brewprune/usage.log every 30 seconds.")
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	// Wait for shutdown signal
	sig := <-sigCh
	fmt.Printf("\nReceived signal %v, shutting down...\n", sig)

	// Stop the watcher
	spinner = output.NewSpinner("Stopping watcher...")
	if err := w.Stop(); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to stop watcher: %w", err)
	}
	spinner.StopWithMessage("✓ Watcher stopped")

	fmt.Println("Usage tracking stopped")

	return nil
}
