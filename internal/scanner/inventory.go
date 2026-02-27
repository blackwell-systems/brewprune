package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/brewprune/internal/brew"
)

// ScanPackages scans all installed packages via brew and stores them in the database.
// This includes package metadata, dependencies, and binary paths.
func (s *Scanner) ScanPackages() error {
	// Get all installed packages from brew
	packages, err := brew.ListInstalled()
	if err != nil {
		return fmt.Errorf("failed to list installed packages: %w", err)
	}

	// Store each package first
	for _, pkg := range packages {
		if err := s.store.InsertPackage(pkg); err != nil {
			return fmt.Errorf("failed to insert package %s: %w", pkg.Name, err)
		}
	}

	// Get all dependencies in one call (much faster than per-package)
	depsTree, err := brew.GetAllDependencies()
	if err != nil {
		// Log warning but continue - dependencies are optional
		// Some systems may not have any packages with dependencies
	} else {
		// Store all dependency relationships
		for pkgName, deps := range depsTree {
			for _, dep := range deps {
				// Check if both package and dependency exist before inserting relationship
				// This skips runtime dependencies that aren't installed as top-level packages
				if _, err := s.store.GetPackage(pkgName); err != nil {
					continue // Skip if parent package doesn't exist
				}
				if _, err := s.store.GetPackage(dep); err != nil {
					continue // Skip if dependency doesn't exist
				}

				if err := s.store.InsertDependency(pkgName, dep); err != nil {
					return fmt.Errorf("failed to insert dependency %s -> %s: %w", pkgName, dep, err)
				}
			}
		}
	}

	// Refresh binary paths for all packages
	if err := s.RefreshBinaryPaths(); err != nil {
		return fmt.Errorf("failed to refresh binary paths: %w", err)
	}

	return nil
}

// GetInventory returns the current package inventory from the database.
// This is a cached view and does not re-scan brew.
func (s *Scanner) GetInventory() ([]*brew.Package, error) {
	packages, err := s.store.ListPackages()
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory: %w", err)
	}
	return packages, nil
}

// RefreshBinaryPaths rescans the brew prefix bin directory and updates
// binary paths for all packages in the database.
func (s *Scanner) RefreshBinaryPaths() error {
	// Get brew prefix
	prefix, err := brew.GetBrewPrefix()
	if err != nil {
		return fmt.Errorf("failed to get brew prefix: %w", err)
	}

	binDir := filepath.Join(prefix, "bin")

	// Read all files in bin directory
	entries, err := os.ReadDir(binDir)
	if err != nil {
		return fmt.Errorf("failed to read bin directory: %w", err)
	}

	// Get all packages from database
	packages, err := s.store.ListPackages()
	if err != nil {
		return fmt.Errorf("failed to list packages: %w", err)
	}

	// Build a map of package names for quick lookup
	pkgMap := make(map[string]*brew.Package)
	for _, pkg := range packages {
		pkgMap[pkg.Name] = pkg
		// Reset binary paths - we'll rebuild them
		pkg.BinaryPaths = []string{}
		pkg.HasBinary = false
	}

	// Scan binaries and match to packages
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		fullPath := filepath.Join(binDir, name)

		// Check if this is a symlink
		info, err := os.Lstat(fullPath)
		if err != nil {
			continue
		}

		if info.Mode()&os.ModeSymlink != 0 {
			// Resolve symlink to find which package it belongs to
			target, err := os.Readlink(fullPath)
			if err != nil {
				continue
			}

			// If not absolute, make it absolute relative to binDir
			if !filepath.IsAbs(target) {
				target = filepath.Join(binDir, target)
			}

			// Extract package name from the symlink target path
			// Typical path: /opt/homebrew/Cellar/PACKAGE/VERSION/bin/BINARY
			// or: ../Cellar/PACKAGE/VERSION/bin/BINARY
			pkgName := extractPackageFromPath(target)
			if pkgName == "" {
				continue
			}

			// Update package binary paths
			if pkg, exists := pkgMap[pkgName]; exists {
				pkg.BinaryPaths = append(pkg.BinaryPaths, fullPath)
				pkg.HasBinary = true
			}
		}
	}

	// Update all packages in database with new binary paths
	for _, pkg := range packages {
		if err := s.store.InsertPackage(pkg); err != nil {
			return fmt.Errorf("failed to update package %s: %w", pkg.Name, err)
		}
	}

	return nil
}

// extractPackageFromPath extracts the package name from a Cellar path.
// Example: /opt/homebrew/Cellar/git/2.43.0/bin/git -> "git"
// Example: ../Cellar/node/20.10.0/bin/node -> "node"
func extractPackageFromPath(path string) string {
	// Normalize path
	path = filepath.Clean(path)

	// Split path into components
	parts := strings.Split(path, string(filepath.Separator))

	// Find "Cellar" in the path
	for i, part := range parts {
		if part == "Cellar" && i+1 < len(parts) {
			// Next component is the package name
			return parts[i+1]
		}
	}

	return ""
}
