package store

const schema = `
CREATE TABLE IF NOT EXISTS packages (
    name TEXT PRIMARY KEY,
    installed_at TIMESTAMP,
    install_type TEXT,
    version TEXT,
    tap TEXT,
    is_cask BOOLEAN,
    size_bytes INTEGER,
    has_binary BOOLEAN,
    binary_paths TEXT
);

CREATE TABLE IF NOT EXISTS dependencies (
    package TEXT NOT NULL,
    depends_on TEXT NOT NULL,
    PRIMARY KEY (package, depends_on),
    FOREIGN KEY (package) REFERENCES packages(name) ON DELETE CASCADE,
    FOREIGN KEY (depends_on) REFERENCES packages(name) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS usage_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    package TEXT NOT NULL,
    event_type TEXT NOT NULL,
    binary_path TEXT,
    timestamp TIMESTAMP NOT NULL,
    FOREIGN KEY (package) REFERENCES packages(name) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP NOT NULL,
    reason TEXT,
    package_count INTEGER,
    snapshot_path TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS snapshot_packages (
    snapshot_id INTEGER NOT NULL,
    package_name TEXT NOT NULL,
    version TEXT NOT NULL,
    tap TEXT,
    was_explicit BOOLEAN,
    FOREIGN KEY (snapshot_id) REFERENCES snapshots(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_usage_package ON usage_events(package);
CREATE INDEX IF NOT EXISTS idx_usage_timestamp ON usage_events(timestamp);
CREATE INDEX IF NOT EXISTS idx_deps_package ON dependencies(package);
CREATE INDEX IF NOT EXISTS idx_deps_depends ON dependencies(depends_on);
CREATE INDEX IF NOT EXISTS idx_snapshot_packages ON snapshot_packages(snapshot_id);
`
