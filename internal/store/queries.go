package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/blackwell-systems/brewprune/internal/brew"
)

// Package operations

// InsertPackage inserts or replaces a package in the database.
func (s *Store) InsertPackage(pkg *brew.Package) error {
	binaryPathsJSON, err := json.Marshal(pkg.BinaryPaths)
	if err != nil {
		return fmt.Errorf("failed to marshal binary paths: %w", err)
	}

	query := `
		INSERT OR REPLACE INTO packages
		(name, installed_at, install_type, version, tap, is_cask, size_bytes, has_binary, binary_paths)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.Exec(query,
		pkg.Name,
		pkg.InstalledAt.Format(time.RFC3339),
		pkg.InstallType,
		pkg.Version,
		pkg.Tap,
		pkg.IsCask,
		pkg.SizeBytes,
		pkg.HasBinary,
		string(binaryPathsJSON),
	)

	if err != nil {
		return fmt.Errorf("failed to insert package %s: %w", pkg.Name, err)
	}

	return nil
}

// GetPackage retrieves a package by name.
func (s *Store) GetPackage(name string) (*brew.Package, error) {
	query := `
		SELECT name, installed_at, install_type, version, tap, is_cask, size_bytes, has_binary, binary_paths
		FROM packages
		WHERE name = ?
	`

	var pkg brew.Package
	var installedAt string
	var binaryPathsJSON string

	err := s.db.QueryRow(query, name).Scan(
		&pkg.Name,
		&installedAt,
		&pkg.InstallType,
		&pkg.Version,
		&pkg.Tap,
		&pkg.IsCask,
		&pkg.SizeBytes,
		&pkg.HasBinary,
		&binaryPathsJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("package %s not found", name)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get package %s: %w", name, err)
	}

	// Parse installed_at timestamp
	pkg.InstalledAt, err = time.Parse(time.RFC3339, installedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse installed_at for %s: %w", name, err)
	}

	// Parse binary_paths JSON
	if err := json.Unmarshal([]byte(binaryPathsJSON), &pkg.BinaryPaths); err != nil {
		return nil, fmt.Errorf("failed to unmarshal binary paths for %s: %w", name, err)
	}

	return &pkg, nil
}

// ListPackages returns all packages.
func (s *Store) ListPackages() ([]*brew.Package, error) {
	query := `
		SELECT name, installed_at, install_type, version, tap, is_cask, size_bytes, has_binary, binary_paths
		FROM packages
		ORDER BY name
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list packages: %w", err)
	}
	defer rows.Close()

	var packages []*brew.Package
	for rows.Next() {
		var pkg brew.Package
		var installedAt string
		var binaryPathsJSON string

		err := rows.Scan(
			&pkg.Name,
			&installedAt,
			&pkg.InstallType,
			&pkg.Version,
			&pkg.Tap,
			&pkg.IsCask,
			&pkg.SizeBytes,
			&pkg.HasBinary,
			&binaryPathsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan package row: %w", err)
		}

		// Parse installed_at timestamp
		pkg.InstalledAt, err = time.Parse(time.RFC3339, installedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse installed_at for %s: %w", pkg.Name, err)
		}

		// Parse binary_paths JSON
		if err := json.Unmarshal([]byte(binaryPathsJSON), &pkg.BinaryPaths); err != nil {
			return nil, fmt.Errorf("failed to unmarshal binary paths for %s: %w", pkg.Name, err)
		}

		packages = append(packages, &pkg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating packages: %w", err)
	}

	return packages, nil
}

// DeletePackage removes a package from the database.
func (s *Store) DeletePackage(name string) error {
	query := `DELETE FROM packages WHERE name = ?`
	result, err := s.db.Exec(query, name)
	if err != nil {
		return fmt.Errorf("failed to delete package %s: %w", name, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("package %s not found", name)
	}

	return nil
}

// Dependency operations

// InsertDependency records a dependency relationship.
func (s *Store) InsertDependency(pkg, dep string) error {
	query := `
		INSERT OR IGNORE INTO dependencies (package, depends_on)
		VALUES (?, ?)
	`

	_, err := s.db.Exec(query, pkg, dep)
	if err != nil {
		return fmt.Errorf("failed to insert dependency %s -> %s: %w", pkg, dep, err)
	}

	return nil
}

// GetDependencies returns all packages that the given package depends on.
func (s *Store) GetDependencies(pkg string) ([]string, error) {
	query := `
		SELECT depends_on
		FROM dependencies
		WHERE package = ?
		ORDER BY depends_on
	`

	rows, err := s.db.Query(query, pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to get dependencies for %s: %w", pkg, err)
	}
	defer rows.Close()

	var deps []string
	for rows.Next() {
		var dep string
		if err := rows.Scan(&dep); err != nil {
			return nil, fmt.Errorf("failed to scan dependency row: %w", err)
		}
		deps = append(deps, dep)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating dependencies: %w", err)
	}

	return deps, nil
}

// GetDependents returns all packages that depend on the given package.
func (s *Store) GetDependents(pkg string) ([]string, error) {
	query := `
		SELECT package
		FROM dependencies
		WHERE depends_on = ?
		ORDER BY package
	`

	rows, err := s.db.Query(query, pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to get dependents for %s: %w", pkg, err)
	}
	defer rows.Close()

	var dependents []string
	for rows.Next() {
		var dependent string
		if err := rows.Scan(&dependent); err != nil {
			return nil, fmt.Errorf("failed to scan dependent row: %w", err)
		}
		dependents = append(dependents, dependent)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating dependents: %w", err)
	}

	return dependents, nil
}

// Usage event operations

// InsertUsageEvent records a package usage event.
func (s *Store) InsertUsageEvent(event *UsageEvent) error {
	query := `
		INSERT INTO usage_events (package, event_type, binary_path, timestamp)
		VALUES (?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		event.Package,
		event.EventType,
		event.BinaryPath,
		event.Timestamp.Format(time.RFC3339),
	)

	if err != nil {
		return fmt.Errorf("failed to insert usage event for %s: %w", event.Package, err)
	}

	return nil
}

// GetUsageEvents returns usage events for a package since the given time.
func (s *Store) GetUsageEvents(pkg string, since time.Time) ([]*UsageEvent, error) {
	query := `
		SELECT package, event_type, binary_path, timestamp
		FROM usage_events
		WHERE package = ? AND timestamp >= ?
		ORDER BY timestamp DESC
	`

	rows, err := s.db.Query(query, pkg, since.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("failed to get usage events for %s: %w", pkg, err)
	}
	defer rows.Close()

	var events []*UsageEvent
	for rows.Next() {
		var event UsageEvent
		var timestamp string

		err := rows.Scan(
			&event.Package,
			&event.EventType,
			&event.BinaryPath,
			&timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan usage event row: %w", err)
		}

		// Parse timestamp
		event.Timestamp, err = time.Parse(time.RFC3339, timestamp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse timestamp for event: %w", err)
		}

		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating usage events: %w", err)
	}

	return events, nil
}

// GetLastUsage returns the timestamp of the most recent usage event for a package.
// Returns nil if no usage events exist.
func (s *Store) GetLastUsage(pkg string) (*time.Time, error) {
	query := `
		SELECT timestamp
		FROM usage_events
		WHERE package = ?
		ORDER BY timestamp DESC
		LIMIT 1
	`

	var timestamp string
	err := s.db.QueryRow(query, pkg).Scan(&timestamp)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get last usage for %s: %w", pkg, err)
	}

	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	return &t, nil
}

// Snapshot operations

// InsertSnapshot creates a new snapshot record and returns its ID.
func (s *Store) InsertSnapshot(reason string, pkgCount int, path string) (int64, error) {
	query := `
		INSERT INTO snapshots (created_at, reason, package_count, snapshot_path)
		VALUES (?, ?, ?, ?)
	`

	result, err := s.db.Exec(query,
		time.Now().Format(time.RFC3339),
		reason,
		pkgCount,
		path,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert snapshot: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get snapshot ID: %w", err)
	}

	return id, nil
}

// GetSnapshot retrieves a snapshot by ID.
func (s *Store) GetSnapshot(id int64) (*Snapshot, error) {
	query := `
		SELECT id, created_at, reason, package_count, snapshot_path
		FROM snapshots
		WHERE id = ?
	`

	var snapshot Snapshot
	var createdAt string

	err := s.db.QueryRow(query, id).Scan(
		&snapshot.ID,
		&createdAt,
		&snapshot.Reason,
		&snapshot.PackageCount,
		&snapshot.SnapshotPath,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("snapshot %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot %d: %w", id, err)
	}

	// Parse created_at timestamp
	snapshot.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse created_at for snapshot %d: %w", id, err)
	}

	return &snapshot, nil
}

// ListSnapshots returns all snapshots ordered by creation time (newest first).
func (s *Store) ListSnapshots() ([]*Snapshot, error) {
	query := `
		SELECT id, created_at, reason, package_count, snapshot_path
		FROM snapshots
		ORDER BY created_at DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []*Snapshot
	for rows.Next() {
		var snapshot Snapshot
		var createdAt string

		err := rows.Scan(
			&snapshot.ID,
			&createdAt,
			&snapshot.Reason,
			&snapshot.PackageCount,
			&snapshot.SnapshotPath,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan snapshot row: %w", err)
		}

		// Parse created_at timestamp
		snapshot.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at for snapshot %d: %w", snapshot.ID, err)
		}

		snapshots = append(snapshots, &snapshot)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating snapshots: %w", err)
	}

	return snapshots, nil
}

// InsertSnapshotPackage adds a package to a snapshot.
func (s *Store) InsertSnapshotPackage(snapshotID int64, pkg *SnapshotPackage) error {
	query := `
		INSERT INTO snapshot_packages (snapshot_id, package_name, version, tap, was_explicit)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		snapshotID,
		pkg.PackageName,
		pkg.Version,
		pkg.Tap,
		pkg.WasExplicit,
	)

	if err != nil {
		return fmt.Errorf("failed to insert snapshot package %s: %w", pkg.PackageName, err)
	}

	return nil
}

// GetSnapshotPackages returns all packages in a snapshot.
func (s *Store) GetSnapshotPackages(snapshotID int64) ([]*SnapshotPackage, error) {
	query := `
		SELECT snapshot_id, package_name, version, tap, was_explicit
		FROM snapshot_packages
		WHERE snapshot_id = ?
		ORDER BY package_name
	`

	rows, err := s.db.Query(query, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot packages: %w", err)
	}
	defer rows.Close()

	var packages []*SnapshotPackage
	for rows.Next() {
		var pkg SnapshotPackage

		err := rows.Scan(
			&pkg.SnapshotID,
			&pkg.PackageName,
			&pkg.Version,
			&pkg.Tap,
			&pkg.WasExplicit,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan snapshot package row: %w", err)
		}

		packages = append(packages, &pkg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating snapshot packages: %w", err)
	}

	return packages, nil
}

// GetEventCount returns the total number of usage events recorded.
func (s *Store) GetEventCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM usage_events").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get event count: %w", err)
	}
	return count, nil
}

// GetFirstEventTime returns the timestamp of the first usage event recorded.
// Returns zero time if no events exist.
func (s *Store) GetFirstEventTime() (time.Time, error) {
	var timestamp string
	err := s.db.QueryRow("SELECT MIN(timestamp) FROM usage_events").Scan(&timestamp)
	if err == sql.ErrNoRows || timestamp == "" {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get first event time: %w", err)
	}

	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	return t, nil
}
