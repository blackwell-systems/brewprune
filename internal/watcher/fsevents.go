package watcher

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/blackwell-systems/brewprune/internal/store"
)

// logProcessingStats writes a per-cycle summary line to stderr when lines were processed.
func logProcessingStats(stats ProcessingStats) {
	if stats.LinesRead > 0 {
		fmt.Fprintf(os.Stderr, "%s brewprune-watch: processed %d lines, resolved %d packages, skipped %d\n",
			time.Now().UTC().Format(time.RFC3339), stats.LinesRead, stats.Resolved, stats.Skipped)
	}
}

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
	if stats, err := ProcessUsageLog(w.store); err != nil {
		fmt.Fprintf(os.Stderr, "watcher: initial shim log processing: %v\n", err)
	} else {
		logProcessingStats(stats)
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
			if stats, err := ProcessUsageLog(w.store); err != nil {
				fmt.Fprintf(os.Stderr, "watcher: shim log processing error: %v\n", err)
			} else {
				logProcessingStats(stats)
			}
		case <-w.stopCh:
			if stats, err := ProcessUsageLog(w.store); err != nil {
				fmt.Fprintf(os.Stderr, "watcher: final shim log flush error: %v\n", err)
			} else {
				logProcessingStats(stats)
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
