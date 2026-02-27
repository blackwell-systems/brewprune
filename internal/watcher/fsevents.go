package watcher

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/blackwell-systems/brewprune/internal/store"
	"github.com/fsnotify/fsnotify"
)

// Watcher monitors filesystem events to track package binary usage.
type Watcher struct {
	store       *store.Store
	binaryMap   map[string]string // binary path -> package name
	eventQueue  chan *store.UsageEvent
	watcher     *fsnotify.Watcher
	stopCh      chan struct{}
	wg          sync.WaitGroup
	mu          sync.RWMutex
	batchTicker *time.Ticker
}

// New creates a new Watcher instance.
func New(st *store.Store) (*Watcher, error) {
	if st == nil {
		return nil, fmt.Errorf("store cannot be nil")
	}

	return &Watcher{
		store:      st,
		binaryMap:  make(map[string]string),
		eventQueue: make(chan *store.UsageEvent, 1000),
		stopCh:     make(chan struct{}),
	}, nil
}

// Start begins monitoring filesystem events.
func (w *Watcher) Start() error {
	// Build the binary map first
	if err := w.BuildBinaryMap(); err != nil {
		return fmt.Errorf("failed to build binary map: %w", err)
	}

	// Create fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	w.watcher = watcher

	// Get brew prefix
	brewPrefix, err := getBrewPrefix()
	if err != nil {
		w.watcher.Close()
		return fmt.Errorf("failed to get brew prefix: %w", err)
	}

	// Add watch directories
	watchDirs := []string{
		filepath.Join(brewPrefix, "bin"),
		filepath.Join(brewPrefix, "sbin"),
		"/Applications",
	}

	for _, dir := range watchDirs {
		if err := w.addWatchIfExists(dir); err != nil {
			w.watcher.Close()
			return err
		}
	}

	// Start batch ticker (write events every 30 seconds)
	w.batchTicker = time.NewTicker(30 * time.Second)

	// Start event processor goroutine
	w.wg.Add(1)
	go w.processEvents()

	// Start batch writer goroutine
	w.wg.Add(1)
	go w.batchWriter()

	return nil
}

// Stop halts the watcher and flushes remaining events.
func (w *Watcher) Stop() error {
	// Signal stop
	close(w.stopCh)

	// Stop the batch ticker
	if w.batchTicker != nil {
		w.batchTicker.Stop()
	}

	// Close the watcher
	if w.watcher != nil {
		if err := w.watcher.Close(); err != nil {
			return fmt.Errorf("failed to close watcher: %w", err)
		}
	}

	// Wait for goroutines to finish
	w.wg.Wait()

	// Flush remaining events
	close(w.eventQueue)
	for event := range w.eventQueue {
		if err := w.store.InsertUsageEvent(event); err != nil {
			return fmt.Errorf("failed to flush event: %w", err)
		}
	}

	return nil
}

// addWatchIfExists adds a directory to the watcher if it exists.
func (w *Watcher) addWatchIfExists(dir string) error {
	if _, err := os.Stat(dir); err == nil {
		if err := w.watcher.Add(dir); err != nil {
			return fmt.Errorf("failed to watch %s: %w", dir, err)
		}
	}
	return nil
}

// processEvents handles fsnotify events and queues usage events.
func (w *Watcher) processEvents() {
	defer w.wg.Done()

	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Filter for relevant operations (open, execute, chmod for execution)
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Chmod) {
				w.handleFileEvent(event.Name)
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "watcher error: %v\n", err)

		case <-w.stopCh:
			return
		}
	}
}

// handleFileEvent processes a file event and queues a usage event if applicable.
func (w *Watcher) handleFileEvent(path string) {
	// Check if this is an executable
	info, err := os.Stat(path)
	if err != nil {
		return
	}

	// Skip directories
	if info.IsDir() {
		return
	}

	// Check if executable
	if info.Mode()&0111 == 0 {
		return
	}

	// Match to package
	pkg, found := w.MatchPathToPackage(path)
	if !found {
		return
	}

	// Determine event type
	eventType := "exec"
	if filepath.HasPrefix(path, "/Applications") {
		eventType = "app_launch"
	}

	// Queue the event
	event := &store.UsageEvent{
		Package:    pkg,
		EventType:  eventType,
		BinaryPath: path,
		Timestamp:  time.Now(),
	}

	select {
	case w.eventQueue <- event:
	default:
		// Queue is full, drop event
		fmt.Fprintf(os.Stderr, "warning: event queue full, dropping event for %s\n", pkg)
	}
}

// batchWriter periodically writes queued events to the database.
func (w *Watcher) batchWriter() {
	defer w.wg.Done()

	events := make([]*store.UsageEvent, 0, 100)

	for {
		select {
		case <-w.batchTicker.C:
			// Drain queue and write batch
			events = events[:0]
		drainLoop:
			for {
				select {
				case event := <-w.eventQueue:
					events = append(events, event)
					if len(events) >= 100 {
						break drainLoop
					}
				default:
					break drainLoop
				}
			}

			// Write events to database
			for _, event := range events {
				if err := w.store.InsertUsageEvent(event); err != nil {
					fmt.Fprintf(os.Stderr, "failed to write usage event: %v\n", err)
				}
			}

		case <-w.stopCh:
			return
		}
	}
}

// getBrewPrefix returns the Homebrew installation prefix.
func getBrewPrefix() (string, error) {
	cmd := exec.Command("brew", "--prefix")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute brew --prefix: %w", err)
	}

	prefix := string(output)
	if len(prefix) > 0 && prefix[len(prefix)-1] == '\n' {
		prefix = prefix[:len(prefix)-1]
	}

	return prefix, nil
}
