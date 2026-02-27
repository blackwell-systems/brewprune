// Package watcher tracks Homebrew package usage via PATH shims.
//
// When a user runs a shimmed command (e.g. git), a tiny interceptor binary
// appends an entry to ~/.brewprune/usage.log. The Watcher polls that log
// every 30 seconds, resolves binary names to package names, and batch-inserts
// usage events into the database.
//
// Key features:
//   - Shim log polling (no special permissions required)
//   - Crash-safe offset tracking (temp file + rename pattern)
//   - Batched SQLite inserts (single transaction per tick)
//   - Daemon mode support with PID file management
//   - Graceful shutdown with SIGTERM/SIGINT handling
//
// Example usage:
//
//	st, err := store.New("~/.brewprune/brewprune.db")
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer st.Close()
//
//	w, err := watcher.New(st)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Start watching in foreground
//	if err := w.Start(); err != nil {
//		log.Fatal(err)
//	}
//	defer w.Stop()
//
//	// Or start as daemon
//	if err := w.StartDaemon("/tmp/brewprune.pid", "/tmp/brewprune.log"); err != nil {
//		log.Fatal(err)
//	}
package watcher
