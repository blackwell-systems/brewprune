package snapshots

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/brewprune/internal/brew"
)

// RestoreSnapshot restores packages from a snapshot.
// It will install all packages from the snapshot in their recorded versions.
func (m *Manager) RestoreSnapshot(id int64) error {
	// Get snapshot metadata from database
	snapshot, err := m.store.GetSnapshot(id)
	if err != nil {
		return fmt.Errorf("failed to get snapshot: %w", err)
	}

	// Read snapshot JSON file
	snapshotData, err := loadSnapshotFile(snapshot.SnapshotPath)
	if err != nil {
		return fmt.Errorf("failed to load snapshot file: %w", err)
	}

	// Check brew version compatibility (warn only, don't fail)
	currentBrewVersion, err := getBrewVersion()
	if err == nil && currentBrewVersion != snapshotData.BrewVersion {
		fmt.Fprintf(os.Stderr, "Warning: snapshot was created with Homebrew %s, current version is %s\n",
			snapshotData.BrewVersion, currentBrewVersion)
	}

	// Track results
	var successCount, failureCount int
	var failures []string

	// Restore packages
	for _, pkg := range snapshotData.Packages {
		if err := restorePackage(pkg); err != nil {
			failureCount++
			failures = append(failures, fmt.Sprintf("%s: %v", pkg.Name, err))
			fmt.Fprintf(os.Stderr, "Failed to restore %s: %v\n", pkg.Name, err)
		} else {
			successCount++
			fmt.Printf("Restored %s\n", formatRestoredPkg(pkg.Name, pkg.Version))
		}
	}

	// Report results
	if failureCount > 0 {
		return fmt.Errorf("restored %d/%d packages, %d failures: %v",
			successCount, len(snapshotData.Packages), failureCount, failures)
	}

	return nil
}

// formatRestoredPkg returns "name@version" when version is non-empty,
// or just "name" when version is empty, avoiding a bare trailing "@".
func formatRestoredPkg(name, version string) string {
	if version != "" {
		return name + "@" + version
	}
	return name
}

// restorePackage restores a single package from a snapshot.
func restorePackage(pkg *PackageSnapshot) error {
	// Add tap if needed and not empty
	if pkg.Tap != "" && pkg.Tap != "homebrew/core" {
		if err := brew.AddTap(pkg.Tap); err != nil {
			return fmt.Errorf("failed to add tap %s: %w", pkg.Tap, err)
		}
	}

	// Attempt to install the specific version
	// Note: Homebrew may not have the exact version available
	err := brew.Install(pkg.Name, pkg.Version)
	if err != nil {
		// Try installing without version as fallback
		fmt.Fprintf(os.Stderr, "Warning: version %s not available for %s, installing latest\n",
			pkg.Version, pkg.Name)

		err = brew.Install(pkg.Name, "")
		if err != nil {
			return fmt.Errorf("failed to install: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Installed latest version of %s (exact version %s not available)\n",
			pkg.Name, pkg.Version)
	}

	return nil
}

// loadSnapshotFile reads and parses a snapshot JSON file.
func loadSnapshotFile(path string) (*SnapshotData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot file: %w", err)
	}

	var snapshotData SnapshotData
	if err := json.Unmarshal(data, &snapshotData); err != nil {
		return nil, fmt.Errorf("failed to parse snapshot JSON: %w", err)
	}

	return &snapshotData, nil
}
