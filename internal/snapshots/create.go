package snapshots

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/blackwell-systems/brewprune/internal/store"
)

// CreateSnapshot creates a snapshot of the specified packages and returns the snapshot ID.
// If packages is empty, snapshots all installed packages.
func (m *Manager) CreateSnapshot(packages []string, reason string) (int64, error) {
	// Ensure snapshot directory exists
	if err := os.MkdirAll(m.snapshotDir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	// Get brew version for compatibility tracking
	brewVersion, err := getBrewVersion()
	if err != nil {
		return 0, fmt.Errorf("failed to get brew version: %w", err)
	}

	// If no packages specified, snapshot all installed packages
	if len(packages) == 0 {
		allPkgs, err := m.store.ListPackages()
		if err != nil {
			return 0, fmt.Errorf("failed to list packages: %w", err)
		}

		packages = make([]string, len(allPkgs))
		for i, pkg := range allPkgs {
			packages[i] = pkg.Name
		}
	}

	// Build snapshot data
	snapshotData := &SnapshotData{
		CreatedAt:   time.Now(),
		Reason:      reason,
		Packages:    make([]*PackageSnapshot, 0, len(packages)),
		BrewVersion: brewVersion,
	}

	// Gather package information
	for _, pkgName := range packages {
		pkg, err := m.store.GetPackage(pkgName)
		if err != nil {
			return 0, fmt.Errorf("failed to get package %s: %w", pkgName, err)
		}

		// Get dependencies
		deps, err := m.store.GetDependencies(pkgName)
		if err != nil {
			return 0, fmt.Errorf("failed to get dependencies for %s: %w", pkgName, err)
		}

		pkgSnapshot := &PackageSnapshot{
			Name:         pkg.Name,
			Version:      pkg.Version,
			Tap:          pkg.Tap,
			WasExplicit:  pkg.InstallType == "explicit",
			Dependencies: deps,
		}

		snapshotData.Packages = append(snapshotData.Packages, pkgSnapshot)
	}

	// Generate snapshot filename: YYYY-MM-DD-HHMMSS.json
	timestamp := time.Now().Format("2006-01-02-150405")
	snapshotFilename := fmt.Sprintf("%s.json", timestamp)
	snapshotPath := filepath.Join(m.snapshotDir, snapshotFilename)

	// Write snapshot JSON file
	jsonData, err := json.MarshalIndent(snapshotData, "", "  ")
	if err != nil {
		return 0, fmt.Errorf("failed to marshal snapshot data: %w", err)
	}

	if err := os.WriteFile(snapshotPath, jsonData, 0644); err != nil {
		return 0, fmt.Errorf("failed to write snapshot file: %w", err)
	}

	// Insert snapshot record into database
	snapshotID, err := m.store.InsertSnapshot(reason, len(packages), snapshotPath)
	if err != nil {
		// Try to clean up the JSON file if DB insert fails
		os.Remove(snapshotPath)
		return 0, fmt.Errorf("failed to insert snapshot into database: %w", err)
	}

	// Insert snapshot packages into database
	for _, pkg := range snapshotData.Packages {
		snapshotPkg := &store.SnapshotPackage{
			SnapshotID:  snapshotID,
			PackageName: pkg.Name,
			Version:     pkg.Version,
			Tap:         pkg.Tap,
			WasExplicit: pkg.WasExplicit,
		}

		if err := m.store.InsertSnapshotPackage(snapshotID, snapshotPkg); err != nil {
			return 0, fmt.Errorf("failed to insert snapshot package %s: %w", pkg.Name, err)
		}
	}

	return snapshotID, nil
}

// ListSnapshots returns all snapshots from the database.
func (m *Manager) ListSnapshots() ([]*store.Snapshot, error) {
	snapshots, err := m.store.ListSnapshots()
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}
	return snapshots, nil
}

// CleanupOldSnapshots removes snapshots older than 90 days.
func (m *Manager) CleanupOldSnapshots() error {
	snapshots, err := m.store.ListSnapshots()
	if err != nil {
		return fmt.Errorf("failed to list snapshots: %w", err)
	}

	cutoffDate := time.Now().AddDate(0, 0, -90)
	deletedCount := 0

	for _, snapshot := range snapshots {
		if snapshot.CreatedAt.Before(cutoffDate) {
			// Remove the JSON file if it exists
			if err := os.Remove(snapshot.SnapshotPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to delete snapshot file %s: %w", snapshot.SnapshotPath, err)
			}

			// Note: We're not deleting from the database to maintain historical records
			// The database entry serves as an audit log
			deletedCount++
		}
	}

	return nil
}

// getBrewVersion returns the current Homebrew version.
func getBrewVersion() (string, error) {
	cmd := exec.Command("brew", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute brew --version: %w", err)
	}

	// Parse "Homebrew X.Y.Z" from first line
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return "", fmt.Errorf("empty brew --version output")
	}

	// Extract version from "Homebrew X.Y.Z"
	parts := strings.Fields(lines[0])
	if len(parts) < 2 {
		return "", fmt.Errorf("unexpected brew --version format: %s", lines[0])
	}

	return parts[1], nil
}
