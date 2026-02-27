package scanner

import (
	"testing"
	"time"

	"github.com/blackwell-systems/brewprune/internal/brew"
)

func TestBuildDependencyGraph(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	scanner := New(s)

	// Insert test packages
	packages := []*brew.Package{
		{Name: "git", Version: "2.43.0", InstalledAt: time.Now(), InstallType: "explicit"},
		{Name: "node", Version: "20.10.0", InstalledAt: time.Now(), InstallType: "explicit"},
		{Name: "openssl@3", Version: "3.2.0", InstalledAt: time.Now(), InstallType: "dependency"},
		{Name: "pcre2", Version: "10.42", InstalledAt: time.Now(), InstallType: "dependency"},
		{Name: "icu4c", Version: "74.2", InstalledAt: time.Now(), InstallType: "dependency"},
	}

	for _, pkg := range packages {
		if err := s.InsertPackage(pkg); err != nil {
			t.Fatalf("failed to insert package %s: %v", pkg.Name, err)
		}
	}

	// Insert dependencies
	// git depends on openssl@3 and pcre2
	// node depends on openssl@3 and icu4c
	deps := map[string][]string{
		"git":  {"openssl@3", "pcre2"},
		"node": {"openssl@3", "icu4c"},
	}

	for pkg, pkgDeps := range deps {
		for _, dep := range pkgDeps {
			if err := s.InsertDependency(pkg, dep); err != nil {
				t.Fatalf("failed to insert dependency %s -> %s: %v", pkg, dep, err)
			}
		}
	}

	// Build dependency graph
	graph, err := scanner.BuildDependencyGraph()
	if err != nil {
		t.Fatalf("failed to build dependency graph: %v", err)
	}

	// Verify graph structure
	if len(graph) != 5 {
		t.Errorf("expected graph with 5 packages, got %d", len(graph))
	}

	// Check git dependencies
	gitDeps := graph["git"]
	if len(gitDeps) != 2 {
		t.Errorf("expected git to have 2 dependencies, got %d", len(gitDeps))
	}
	if !contains(gitDeps, "openssl@3") {
		t.Error("git should depend on openssl@3")
	}
	if !contains(gitDeps, "pcre2") {
		t.Error("git should depend on pcre2")
	}

	// Check node dependencies
	nodeDeps := graph["node"]
	if len(nodeDeps) != 2 {
		t.Errorf("expected node to have 2 dependencies, got %d", len(nodeDeps))
	}
	if !contains(nodeDeps, "openssl@3") {
		t.Error("node should depend on openssl@3")
	}
	if !contains(nodeDeps, "icu4c") {
		t.Error("node should depend on icu4c")
	}

	// Check leaf packages (no dependencies)
	opensslDeps := graph["openssl@3"]
	if len(opensslDeps) != 0 {
		t.Errorf("expected openssl@3 to have 0 dependencies, got %d", len(opensslDeps))
	}
}

func TestGetDependents(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	scanner := New(s)

	// Insert test packages
	packages := []*brew.Package{
		{Name: "git", Version: "2.43.0", InstalledAt: time.Now(), InstallType: "explicit"},
		{Name: "node", Version: "20.10.0", InstalledAt: time.Now(), InstallType: "explicit"},
		{Name: "openssl@3", Version: "3.2.0", InstalledAt: time.Now(), InstallType: "dependency"},
	}

	for _, pkg := range packages {
		if err := s.InsertPackage(pkg); err != nil {
			t.Fatalf("failed to insert package: %v", err)
		}
	}

	// Both git and node depend on openssl@3
	if err := s.InsertDependency("git", "openssl@3"); err != nil {
		t.Fatalf("failed to insert dependency: %v", err)
	}
	if err := s.InsertDependency("node", "openssl@3"); err != nil {
		t.Fatalf("failed to insert dependency: %v", err)
	}

	// Get dependents of openssl@3
	dependents, err := scanner.GetDependents("openssl@3")
	if err != nil {
		t.Fatalf("failed to get dependents: %v", err)
	}

	if len(dependents) != 2 {
		t.Fatalf("expected 2 dependents, got %d", len(dependents))
	}

	if !contains(dependents, "git") {
		t.Error("git should be a dependent of openssl@3")
	}
	if !contains(dependents, "node") {
		t.Error("node should be a dependent of openssl@3")
	}

	// Get dependents of git (should be empty)
	dependents, err = scanner.GetDependents("git")
	if err != nil {
		t.Fatalf("failed to get dependents: %v", err)
	}

	if len(dependents) != 0 {
		t.Errorf("expected 0 dependents for git, got %d", len(dependents))
	}
}

func TestGetLeafPackages(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	scanner := New(s)

	// Insert test packages
	packages := []*brew.Package{
		{Name: "git", Version: "2.43.0", InstalledAt: time.Now(), InstallType: "explicit"},
		{Name: "node", Version: "20.10.0", InstalledAt: time.Now(), InstallType: "explicit"},
		{Name: "openssl@3", Version: "3.2.0", InstalledAt: time.Now(), InstallType: "dependency"},
		{Name: "pcre2", Version: "10.42", InstalledAt: time.Now(), InstallType: "dependency"},
	}

	for _, pkg := range packages {
		if err := s.InsertPackage(pkg); err != nil {
			t.Fatalf("failed to insert package: %v", err)
		}
	}

	// git depends on openssl@3
	// node depends on openssl@3
	// pcre2 has no dependents (leaf)
	if err := s.InsertDependency("git", "openssl@3"); err != nil {
		t.Fatalf("failed to insert dependency: %v", err)
	}
	if err := s.InsertDependency("node", "openssl@3"); err != nil {
		t.Fatalf("failed to insert dependency: %v", err)
	}

	// Get leaf packages
	leaves, err := scanner.GetLeafPackages()
	if err != nil {
		t.Fatalf("failed to get leaf packages: %v", err)
	}

	// git, node, and pcre2 are leaves (nothing depends on them)
	if len(leaves) != 3 {
		t.Errorf("expected 3 leaf packages, got %d: %v", len(leaves), leaves)
	}

	if !contains(leaves, "git") {
		t.Error("git should be a leaf package")
	}
	if !contains(leaves, "node") {
		t.Error("node should be a leaf package")
	}
	if !contains(leaves, "pcre2") {
		t.Error("pcre2 should be a leaf package")
	}

	// openssl@3 should NOT be a leaf (git and node depend on it)
	if contains(leaves, "openssl@3") {
		t.Error("openssl@3 should not be a leaf package")
	}
}

func TestIsCoreDependency(t *testing.T) {
	tests := []struct {
		name     string
		pkg      string
		expected bool
	}{
		{"openssl", "openssl", true},
		{"openssl@1.1", "openssl@1.1", true},
		{"openssl@3", "openssl@3", true},
		{"openssl@4", "openssl@4", true},
		{"icu4c", "icu4c", true},
		{"readline", "readline", true},
		{"gettext", "gettext", true},
		{"libffi", "libffi", true},
		{"gmp", "gmp", true},
		{"pcre", "pcre", true},
		{"pcre2", "pcre2", true},
		{"ca-certificates", "ca-certificates", true},
		{"zlib", "zlib", true},
		{"xz", "xz", true},
		{"sqlite", "sqlite", true},
		{"python@3.11", "python@3.11", true},
		{"python@3.12", "python@3.12", true},
		{"python@3.13", "python@3.13", true},
		{"git", "git", false},
		{"node", "node", false},
		{"wget", "wget", false},
		{"curl", "curl", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCoreDependency(tt.pkg)
			if result != tt.expected {
				t.Errorf("IsCoreDependency(%q) = %v, want %v", tt.pkg, result, tt.expected)
			}
		})
	}
}

func TestGetPruneCandidates(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	scanner := New(s)

	// Insert test packages
	packages := []*brew.Package{
		{Name: "git", Version: "2.43.0", InstalledAt: time.Now(), InstallType: "explicit"},
		{Name: "wget", Version: "1.21.4", InstalledAt: time.Now(), InstallType: "explicit"},
		{Name: "openssl@3", Version: "3.2.0", InstalledAt: time.Now(), InstallType: "dependency"},
		{Name: "pcre2", Version: "10.42", InstalledAt: time.Now(), InstallType: "dependency"},
		{Name: "libidn2", Version: "2.3.4", InstalledAt: time.Now(), InstallType: "dependency"},
		{Name: "libunistring", Version: "1.1", InstalledAt: time.Now(), InstallType: "dependency"},
	}

	for _, pkg := range packages {
		if err := s.InsertPackage(pkg); err != nil {
			t.Fatalf("failed to insert package: %v", err)
		}
	}

	// Dependencies:
	// git -> openssl@3, pcre2
	// wget -> libidn2
	// (libunistring has no dependents - it's an orphaned dependency)
	if err := s.InsertDependency("git", "openssl@3"); err != nil {
		t.Fatalf("failed to insert dependency: %v", err)
	}
	if err := s.InsertDependency("git", "pcre2"); err != nil {
		t.Fatalf("failed to insert dependency: %v", err)
	}
	if err := s.InsertDependency("wget", "libidn2"); err != nil {
		t.Fatalf("failed to insert dependency: %v", err)
	}

	// Get prune candidates
	candidates, err := scanner.GetPruneCandidates()
	if err != nil {
		t.Fatalf("failed to get prune candidates: %v", err)
	}

	// Expected candidates:
	// - libunistring: leaf (nothing depends on it), dependency, not core
	// NOT candidates:
	// - git: explicit install (is a leaf but explicit)
	// - wget: explicit install (is a leaf but explicit)
	// - openssl@3: not a leaf (git depends on it), and is core
	// - pcre2: not a leaf (git depends on it)
	// - libidn2: not a leaf (wget depends on it)

	if len(candidates) != 1 {
		t.Errorf("expected 1 prune candidate, got %d: %v", len(candidates), candidates)
	}

	if !contains(candidates, "libunistring") {
		t.Error("libunistring should be a prune candidate")
	}

	// These should NOT be candidates
	if contains(candidates, "git") {
		t.Error("git should not be a prune candidate (explicit install)")
	}
	if contains(candidates, "wget") {
		t.Error("wget should not be a prune candidate (explicit install)")
	}
	if contains(candidates, "openssl@3") {
		t.Error("openssl@3 should not be a prune candidate (core dependency)")
	}
	if contains(candidates, "pcre2") {
		t.Error("pcre2 should not be a prune candidate (git depends on it)")
	}
	if contains(candidates, "libidn2") {
		t.Error("libidn2 should not be a prune candidate (wget depends on it)")
	}
}

func TestGetDependencyChain(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	scanner := New(s)

	// Insert test packages
	packages := []*brew.Package{
		{Name: "git", Version: "2.43.0", InstalledAt: time.Now(), InstallType: "explicit"},
		{Name: "openssl@3", Version: "3.2.0", InstalledAt: time.Now(), InstallType: "dependency"},
		{Name: "pcre2", Version: "10.42", InstalledAt: time.Now(), InstallType: "dependency"},
		{Name: "bzip2", Version: "1.0.8", InstalledAt: time.Now(), InstallType: "dependency"},
	}

	for _, pkg := range packages {
		if err := s.InsertPackage(pkg); err != nil {
			t.Fatalf("failed to insert package: %v", err)
		}
	}

	// Dependencies (with chain):
	// git -> openssl@3, pcre2
	// openssl@3 -> bzip2
	if err := s.InsertDependency("git", "openssl@3"); err != nil {
		t.Fatalf("failed to insert dependency: %v", err)
	}
	if err := s.InsertDependency("git", "pcre2"); err != nil {
		t.Fatalf("failed to insert dependency: %v", err)
	}
	if err := s.InsertDependency("openssl@3", "bzip2"); err != nil {
		t.Fatalf("failed to insert dependency: %v", err)
	}

	// Get dependency chain for git
	chain, err := scanner.GetDependencyChain("git")
	if err != nil {
		t.Fatalf("failed to get dependency chain: %v", err)
	}

	// Should include: openssl@3, pcre2, bzip2
	if len(chain) != 3 {
		t.Errorf("expected 3 dependencies in chain, got %d: %v", len(chain), chain)
	}

	if !contains(chain, "openssl@3") {
		t.Error("chain should include openssl@3")
	}
	if !contains(chain, "pcre2") {
		t.Error("chain should include pcre2")
	}
	if !contains(chain, "bzip2") {
		t.Error("chain should include bzip2")
	}

	// Get dependency chain for leaf package
	chain, err = scanner.GetDependencyChain("bzip2")
	if err != nil {
		t.Fatalf("failed to get dependency chain: %v", err)
	}

	if len(chain) != 0 {
		t.Errorf("expected 0 dependencies for bzip2, got %d", len(chain))
	}
}

func TestGetDependencyChain_Cycle(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	scanner := New(s)

	// Insert test packages
	packages := []*brew.Package{
		{Name: "a", Version: "1.0", InstalledAt: time.Now(), InstallType: "explicit"},
		{Name: "b", Version: "1.0", InstalledAt: time.Now(), InstallType: "dependency"},
		{Name: "c", Version: "1.0", InstalledAt: time.Now(), InstallType: "dependency"},
	}

	for _, pkg := range packages {
		if err := s.InsertPackage(pkg); err != nil {
			t.Fatalf("failed to insert package: %v", err)
		}
	}

	// Create a cycle: a -> b -> c -> a
	if err := s.InsertDependency("a", "b"); err != nil {
		t.Fatalf("failed to insert dependency: %v", err)
	}
	if err := s.InsertDependency("b", "c"); err != nil {
		t.Fatalf("failed to insert dependency: %v", err)
	}
	if err := s.InsertDependency("c", "a"); err != nil {
		t.Fatalf("failed to insert dependency: %v", err)
	}

	// Get dependency chain - should handle cycle gracefully
	chain, err := scanner.GetDependencyChain("a")
	if err != nil {
		t.Fatalf("failed to get dependency chain: %v", err)
	}

	// Should include b and c, but not create infinite loop
	if !contains(chain, "b") {
		t.Error("chain should include b")
	}
	if !contains(chain, "c") {
		t.Error("chain should include c")
	}

	// The chain should not be empty and should not panic
	if len(chain) == 0 {
		t.Error("chain should not be empty")
	}
}

// Helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
