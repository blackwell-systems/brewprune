package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blackwell-systems/brewprune/internal/brew"
)

func TestBuildBinaryMap(t *testing.T) {
	st := setupTestStore(t)
	defer st.Close()

	// Insert test packages
	packages := []*brew.Package{
		{
			Name:        "git",
			Version:     "2.40.0",
			InstallType: "explicit",
			HasBinary:   true,
			BinaryPaths: []string{"/usr/local/bin/git", "/usr/local/bin/git-upload-pack"},
			InstalledAt: time.Now(),
		},
		{
			Name:        "wget",
			Version:     "1.21.0",
			InstallType: "explicit",
			HasBinary:   true,
			BinaryPaths: []string{"/usr/local/bin/wget"},
			InstalledAt: time.Now(),
		},
		{
			Name:        "python",
			Version:     "3.11.0",
			InstallType: "dependency",
			HasBinary:   false,
			BinaryPaths: []string{},
			InstalledAt: time.Now(),
		},
	}

	for _, pkg := range packages {
		if err := st.InsertPackage(pkg); err != nil {
			t.Fatalf("failed to insert package: %v", err)
		}
	}

	w, err := New(st)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Build binary map
	if err := w.BuildBinaryMap(); err != nil {
		t.Fatalf("BuildBinaryMap() error = %v", err)
	}

	// Verify map contents
	expected := map[string]string{
		"/usr/local/bin/git":             "git",
		"/usr/local/bin/git-upload-pack": "git",
		"/usr/local/bin/wget":            "wget",
	}

	if len(w.binaryMap) != len(expected) {
		t.Errorf("binaryMap size = %d, want %d", len(w.binaryMap), len(expected))
	}

	for path, pkg := range expected {
		if got, ok := w.binaryMap[path]; !ok {
			t.Errorf("binaryMap missing path %s", path)
		} else if got != pkg {
			t.Errorf("binaryMap[%s] = %s, want %s", path, got, pkg)
		}
	}
}

func TestMatchPathToPackage(t *testing.T) {
	st := setupTestStore(t)
	defer st.Close()

	w, err := New(st)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Populate binary map
	w.binaryMap = map[string]string{
		"/usr/local/bin/git":  "git",
		"/usr/local/bin/wget": "wget",
	}

	tests := []struct {
		name    string
		path    string
		wantPkg string
		wantOk  bool
	}{
		{
			name:    "exact match",
			path:    "/usr/local/bin/git",
			wantPkg: "git",
			wantOk:  true,
		},
		{
			name:    "another exact match",
			path:    "/usr/local/bin/wget",
			wantPkg: "wget",
			wantOk:  true,
		},
		{
			name:    "no match",
			path:    "/usr/local/bin/unknown",
			wantPkg: "",
			wantOk:  false,
		},
		{
			name:    "empty path",
			path:    "",
			wantPkg: "",
			wantOk:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg, ok := w.MatchPathToPackage(tt.path)
			if ok != tt.wantOk {
				t.Errorf("MatchPathToPackage(%q) ok = %v, want %v", tt.path, ok, tt.wantOk)
			}
			if pkg != tt.wantPkg {
				t.Errorf("MatchPathToPackage(%q) pkg = %q, want %q", tt.path, pkg, tt.wantPkg)
			}
		})
	}
}

func TestMatchPathToPackage_Symlinks(t *testing.T) {
	st := setupTestStore(t)
	defer st.Close()

	w, err := New(st)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Create a temporary directory with a file and symlink
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target")
	symlinkPath := filepath.Join(tmpDir, "link")

	// Create target file
	if err := os.WriteFile(targetPath, []byte("test"), 0755); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	// Create symlink
	if err := os.Symlink(targetPath, symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Resolve both paths to their canonical forms (handles /var -> /private/var on macOS)
	resolvedTarget, err := filepath.EvalSymlinks(targetPath)
	if err != nil {
		t.Fatalf("failed to resolve target: %v", err)
	}

	resolvedSymlink, err := filepath.EvalSymlinks(symlinkPath)
	if err != nil {
		t.Fatalf("failed to resolve symlink: %v", err)
	}

	// Add resolved target to binary map
	w.binaryMap = map[string]string{
		resolvedTarget: "test-package",
	}

	// Should match via symlink resolution
	pkg, ok := w.MatchPathToPackage(symlinkPath)
	if !ok {
		t.Errorf("MatchPathToPackage(symlink) expected match, got none (resolved: %q -> %q)", symlinkPath, resolvedSymlink)
	} else if pkg != "test-package" {
		t.Errorf("MatchPathToPackage(symlink) = %q, want %q", pkg, "test-package")
	}
}
