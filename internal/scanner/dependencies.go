package scanner

import (
	"fmt"
	"strings"
)

// coreDependencies is a list of system-level packages that are commonly
// dependencies of many other packages. These should typically not be pruned.
var coreDependencies = map[string]bool{
	// Cryptography and certificates
	"openssl":         true,
	"openssl@1.1":     true,
	"openssl@3":       true,
	"ca-certificates": true,

	// Core libraries
	"icu4c":    true,
	"readline": true,
	"gettext":  true,
	"libffi":   true,
	"gmp":      true,
	"pcre":     true,
	"pcre2":    true,
	"zlib":     true,
	"xz":       true,
	"sqlite":   true,
	"ncurses":  true,

	// Python versions
	"python@3.12": true,
	"python@3.11": true,

	// Core utilities and tools
	"coreutils": true,
	"git":       true,
	"curl":      true,
	"wget":      true,

	// Build systems and tools
	"pkg-config": true,
	"pkgconf":    true,
	"cmake":      true,
	"autoconf":   true,
	"automake":   true,
	"libtool":    true,

	// Compilers
	"gcc":  true,
	"llvm": true,

	// Database libraries
	"gdbm":        true,
	"berkeley-db": true,

	// XML and config parsing
	"libxml2": true,
	"libxslt": true,
	"libyaml": true,
	"json-c":  true,
}

// BuildDependencyGraph builds a complete dependency graph for all packages
// in the database. Returns a map where keys are package names and values
// are lists of packages they depend on.
func (s *Scanner) BuildDependencyGraph() (map[string][]string, error) {
	packages, err := s.store.ListPackages()
	if err != nil {
		return nil, fmt.Errorf("failed to list packages: %w", err)
	}

	graph := make(map[string][]string)

	for _, pkg := range packages {
		deps, err := s.store.GetDependencies(pkg.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to get dependencies for %s: %w", pkg.Name, err)
		}
		graph[pkg.Name] = deps
	}

	return graph, nil
}

// GetDependents returns all packages that depend on the given package.
// This is a reverse dependency lookup.
func (s *Scanner) GetDependents(pkg string) ([]string, error) {
	dependents, err := s.store.GetDependents(pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to get dependents for %s: %w", pkg, err)
	}
	return dependents, nil
}

// GetLeafPackages returns all packages that have no dependents.
// These are candidates for pruning if they haven't been used recently.
func (s *Scanner) GetLeafPackages() ([]string, error) {
	packages, err := s.store.ListPackages()
	if err != nil {
		return nil, fmt.Errorf("failed to list packages: %w", err)
	}

	var leaves []string
	for _, pkg := range packages {
		dependents, err := s.store.GetDependents(pkg.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to get dependents for %s: %w", pkg.Name, err)
		}

		// If package has no dependents, it's a leaf
		if len(dependents) == 0 {
			leaves = append(leaves, pkg.Name)
		}
	}

	return leaves, nil
}

// IsCoreDependency checks if a package is considered a core system dependency
// that many other packages rely on. Core dependencies should typically not
// be pruned even if they appear unused.
func IsCoreDependency(pkg string) bool {
	// Check exact match first
	if coreDependencies[pkg] {
		return true
	}

	// Check for versioned variants (e.g., python@3.x)
	// Allow any python@3.x version
	if strings.HasPrefix(pkg, "python@3.") {
		return true
	}

	// Allow any openssl@ version
	if strings.HasPrefix(pkg, "openssl@") {
		return true
	}

	return false
}

// GetPruneCandidates returns packages that are good candidates for pruning.
// A package is a prune candidate if:
// - It has no dependents (is a leaf)
// - It is not a core dependency
// - It was installed as a dependency (not explicit)
func (s *Scanner) GetPruneCandidates() ([]string, error) {
	leaves, err := s.GetLeafPackages()
	if err != nil {
		return nil, fmt.Errorf("failed to get leaf packages: %w", err)
	}

	var candidates []string
	for _, pkgName := range leaves {
		// Skip core dependencies
		if IsCoreDependency(pkgName) {
			continue
		}

		// Check if package was installed as dependency
		pkg, err := s.store.GetPackage(pkgName)
		if err != nil {
			// If we can't get the package, skip it
			continue
		}

		// Only include if it was installed as a dependency
		if pkg.InstallType == "dependency" {
			candidates = append(candidates, pkgName)
		}
	}

	return candidates, nil
}

// GetDependencyChain returns the full chain of dependencies for a package.
// This recursively builds the tree of all packages that the given package
// depends on, directly or indirectly.
func (s *Scanner) GetDependencyChain(pkg string) ([]string, error) {
	visited := make(map[string]bool)
	var chain []string

	if err := s.buildDependencyChainRecursive(pkg, visited, &chain); err != nil {
		return nil, err
	}

	return chain, nil
}

// buildDependencyChainRecursive is a helper function that recursively builds
// the dependency chain for a package.
func (s *Scanner) buildDependencyChainRecursive(pkg string, visited map[string]bool, chain *[]string) error {
	// Avoid cycles
	if visited[pkg] {
		return nil
	}
	visited[pkg] = true

	// Get direct dependencies
	deps, err := s.store.GetDependencies(pkg)
	if err != nil {
		return fmt.Errorf("failed to get dependencies for %s: %w", pkg, err)
	}

	// Add each dependency and recurse
	for _, dep := range deps {
		*chain = append(*chain, dep)
		if err := s.buildDependencyChainRecursive(dep, visited, chain); err != nil {
			return err
		}
	}

	return nil
}
