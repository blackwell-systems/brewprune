package watcher

import (
	"fmt"
	"path/filepath"
)

// BuildBinaryMap scans installed packages and builds a map of binary paths to package names.
func (w *Watcher) BuildBinaryMap() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Clear existing map
	w.binaryMap = make(map[string]string)

	// Get all packages from store
	packages, err := w.store.ListPackages()
	if err != nil {
		return fmt.Errorf("failed to list packages: %w", err)
	}

	// Build map of binary paths to package names
	for _, pkg := range packages {
		if pkg.HasBinary {
			for _, binaryPath := range pkg.BinaryPaths {
				w.binaryMap[binaryPath] = pkg.Name
			}
		}
	}

	return nil
}

// MatchPathToPackage matches a file path to a package name.
// Returns the package name and true if found, empty string and false otherwise.
func (w *Watcher) MatchPathToPackage(path string) (string, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Direct match
	if pkg, ok := w.binaryMap[path]; ok {
		return pkg, true
	}

	// Try resolving symlinks
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil && resolved != path {
		if pkg, ok := w.binaryMap[resolved]; ok {
			return pkg, true
		}
	}

	return "", false
}
