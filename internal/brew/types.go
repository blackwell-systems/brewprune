package brew

import "time"

// Package represents a Homebrew package (formula or cask).
type Package struct {
	Name        string
	Version     string
	InstalledAt time.Time
	InstallType string // "explicit" or "dependency"
	Tap         string // e.g., "homebrew/core", "user/tap"
	IsCask      bool
	SizeBytes   int64
	HasBinary   bool
	BinaryPaths []string
}

// Dependency represents a package dependency relationship.
type Dependency struct {
	Package   string
	DependsOn string
}
