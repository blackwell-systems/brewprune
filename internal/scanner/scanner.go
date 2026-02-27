package scanner

import "github.com/blackwell-systems/brewprune/internal/store"

// Scanner manages package inventory and dependency graph operations.
type Scanner struct {
	store *store.Store
}

// New creates a new Scanner instance with the given store.
func New(store *store.Store) *Scanner {
	return &Scanner{store: store}
}
