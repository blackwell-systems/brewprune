package store

import "time"

// Snapshot represents a point-in-time backup of installed packages.
type Snapshot struct {
	ID           int64
	CreatedAt    time.Time
	Reason       string
	PackageCount int
	SnapshotPath string
}

// SnapshotPackage represents a package in a snapshot.
type SnapshotPackage struct {
	SnapshotID  int64
	PackageName string
	Version     string
	Tap         string
	WasExplicit bool
}

// UsageEvent records when a package binary was executed.
type UsageEvent struct {
	Package    string
	EventType  string // "exec" or "app_launch"
	BinaryPath string
	Timestamp  time.Time
}
