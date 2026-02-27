package watcher

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/blackwell-systems/brewprune/internal/store"
)

// Watcher polls the PATH shim usage log to track Homebrew package executions.
// When a user runs a shimmed command (e.g. git), the shim binary appends an
// entry to ~/.brewprune/usage.log. The Watcher processes that log every 30
// seconds and batch-inserts resolved usage events into the database.
type Watcher struct {
	store       *store.Store
	stopCh      chan struct{}
	wg          sync.WaitGroup
	batchTicker *time.Ticker
}

// New creates a new Watcher instance.
func New(st *store.Store) (*Watcher, error) {
	if st == nil {
		return nil, fmt.Errorf("store cannot be nil")
	}
	return &Watcher{
		store:  st,
		stopCh: make(chan struct{}),
	}, nil
}

// Start begins usage tracking by polling the shim log on a 30-second ticker.
// It also processes any events already in the log immediately on startup.
func (w *Watcher) Start() error {
	if err := ProcessUsageLog(w.store); err != nil {
		fmt.Fprintf(os.Stderr, "watcher: initial shim log processing: %v\n", err)
	}

	w.batchTicker = time.NewTicker(30 * time.Second)

	w.wg.Add(1)
	go w.runShimLogProcessor()

	return nil
}

// runShimLogProcessor polls the shim log on each tick and does a final flush
// when the stop signal is received.
func (w *Watcher) runShimLogProcessor() {
	defer w.wg.Done()

	for {
		select {
		case <-w.batchTicker.C:
			if err := ProcessUsageLog(w.store); err != nil {
				fmt.Fprintf(os.Stderr, "watcher: shim log processing error: %v\n", err)
			}
		case <-w.stopCh:
			if err := ProcessUsageLog(w.store); err != nil {
				fmt.Fprintf(os.Stderr, "watcher: final shim log flush error: %v\n", err)
			}
			return
		}
	}
}

// Stop halts the watcher and flushes any remaining log entries.
func (w *Watcher) Stop() error {
	close(w.stopCh)

	if w.batchTicker != nil {
		w.batchTicker.Stop()
	}

	w.wg.Wait()
	return nil
}
