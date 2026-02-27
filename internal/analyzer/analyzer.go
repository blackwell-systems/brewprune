package analyzer

import "github.com/blackwell-systems/brewprune/internal/store"

// Analyzer computes confidence scores and usage statistics for packages.
type Analyzer struct {
	store *store.Store
}

// New creates a new Analyzer instance with the given store.
func New(store *store.Store) *Analyzer {
	return &Analyzer{store: store}
}
