// Package watcher monitors filesystem events to track Homebrew package usage.
//
// The watcher monitors binary execution in $(brew --prefix)/bin, $(brew --prefix)/sbin,
// and /Applications directories, matching executed binaries to installed packages and
// recording usage events to the database.
//
// Key features:
// - Real-time filesystem event monitoring via fsnotify
// - Binary path to package name matching with symlink resolution
// - Batched event writing (every 30 seconds) for performance
// - Daemon mode support with PID file management
// - Graceful shutdown with SIGTERM/SIGINT handling
//
// Example usage:
//
//	store, err := store.New("~/.brewprune/data.db")
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer store.Close()
//
//	watcher, err := watcher.New(store)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Start watching in foreground
//	if err := watcher.Start(); err != nil {
//		log.Fatal(err)
//	}
//	defer watcher.Stop()
//
//	// Or start as daemon
//	if err := watcher.StartDaemon("/tmp/brewprune.pid", "/tmp/brewprune.log"); err != nil {
//		log.Fatal(err)
//	}
package watcher
