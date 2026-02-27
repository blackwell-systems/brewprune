package snapshots

import (
	"time"

	"github.com/blackwell-systems/brewprune/internal/store"
)

// SnapshotData represents the JSON structure stored in snapshot files.
type SnapshotData struct {
	CreatedAt   time.Time
	Reason      string
	Packages    []*PackageSnapshot
	BrewVersion string
}

// PackageSnapshot represents a package in a snapshot file.
type PackageSnapshot struct {
	Name         string
	Version      string
	Tap          string
	WasExplicit  bool
	Dependencies []string
}

// Manager manages snapshot creation, restoration, and cleanup.
type Manager struct {
	store       *store.Store
	snapshotDir string
}

// New creates a new snapshot Manager.
func New(store *store.Store, snapshotDir string) *Manager {
	return &Manager{
		store:       store,
		snapshotDir: snapshotDir,
	}
}
