package watcher

import (
	"testing"

	"github.com/blackwell-systems/brewprune/internal/store"
)

// setupTestStore creates an in-memory SQLite store for tests and registers
// cleanup with t.Cleanup so callers don't need explicit defer.
func setupTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("setupTestStore: open: %v", err)
	}
	if err := st.CreateSchema(); err != nil {
		st.Close()
		t.Fatalf("setupTestStore: schema: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}
