package brew

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// brewListOutput represents the structure of `brew list --json=v2` output
type brewListOutput struct {
	Formulae []brewFormula `json:"formulae"`
	Casks    []brewCask    `json:"casks"`
}

// brewFormula represents a Homebrew formula in JSON output
type brewFormula struct {
	Name          string                 `json:"name"`
	FullName      string                 `json:"full_name"`
	Tap           string                 `json:"tap"`
	Version       string                 `json:"version"`
	Installed     []brewInstalledVersion `json:"installed"`
	LinkedKeg     string                 `json:"linked_keg,omitempty"`
	InstalledTime int64                  `json:"installed_on_request,omitempty"`
}

// brewInstalledVersion represents an installed version
type brewInstalledVersion struct {
	Version       string `json:"version"`
	InstalledTime int64  `json:"installed_on_request,omitempty"`
}

// brewCask represents a Homebrew cask in JSON output
type brewCask struct {
	Token         string `json:"token"`
	FullToken     string `json:"full_token"`
	Tap           string `json:"tap"`
	Version       string `json:"version"`
	InstalledTime string `json:"installed_time,omitempty"`
}

// brewInfoOutput represents the structure of `brew info --json=v2` output
type brewInfoOutput struct {
	Formulae []brewFormulaInfo `json:"formulae"`
	Casks    []brewCaskInfo    `json:"casks"`
}

// brewFormulaInfo represents detailed formula information
type brewFormulaInfo struct {
	Name      string                 `json:"name"`
	FullName  string                 `json:"full_name"`
	Tap       string                 `json:"tap"`
	Version   string                 `json:"version"`
	Installed []brewInstalledVersion `json:"installed"`
	LinkedKeg string                 `json:"linked_keg,omitempty"`
}

// brewCaskInfo represents detailed cask information
type brewCaskInfo struct {
	Token     string `json:"token"`
	FullToken string `json:"full_token"`
	Tap       string `json:"tap"`
	Version   string `json:"version"`
}

// ListInstalled returns all installed Homebrew packages (formulae and casks)
func ListInstalled() ([]*Package, error) {
	cmd := exec.Command("brew", "list", "--json=v2")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("brew list failed: %w (stderr: %s)", err, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("brew list failed: %w", err)
	}

	var listOutput brewListOutput
	if err := json.Unmarshal(output, &listOutput); err != nil {
		return nil, fmt.Errorf("failed to parse brew list output: %w", err)
	}

	var packages []*Package

	// Process formulae
	for _, formula := range listOutput.Formulae {
		pkg := &Package{
			Name:        formula.Name,
			Version:     formula.Version,
			InstalledAt: time.Now(), // Default, will be refined if possible
			InstallType: "explicit", // Default, needs to be determined from deps
			Tap:         formula.Tap,
			IsCask:      false,
			SizeBytes:   0,    // Size needs separate calculation
			HasBinary:   true, // Assume formulae have binaries
			BinaryPaths: []string{},
		}

		// If we have installed info, use that timestamp
		if len(formula.Installed) > 0 && formula.Installed[0].InstalledTime > 0 {
			pkg.InstalledAt = time.Unix(formula.Installed[0].InstalledTime, 0)
		}

		packages = append(packages, pkg)
	}

	// Process casks
	for _, cask := range listOutput.Casks {
		pkg := &Package{
			Name:        cask.Token,
			Version:     cask.Version,
			InstalledAt: time.Now(),
			InstallType: "explicit",
			Tap:         cask.Tap,
			IsCask:      true,
			SizeBytes:   0,
			HasBinary:   false, // Casks typically don't install to bin
			BinaryPaths: []string{},
		}

		// Try to parse installed time if available
		if cask.InstalledTime != "" {
			if t, err := time.Parse(time.RFC3339, cask.InstalledTime); err == nil {
				pkg.InstalledAt = t
			}
		}

		packages = append(packages, pkg)
	}

	return packages, nil
}

// GetPackageInfo returns detailed information about a specific package
func GetPackageInfo(name string) (*Package, error) {
	cmd := exec.Command("brew", "info", "--json=v2", name)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("brew info failed for %s: %w (stderr: %s)", name, err, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("brew info failed for %s: %w", name, err)
	}

	var infoOutput brewInfoOutput
	if err := json.Unmarshal(output, &infoOutput); err != nil {
		return nil, fmt.Errorf("failed to parse brew info output: %w", err)
	}

	// Check formulae first
	if len(infoOutput.Formulae) > 0 {
		formula := infoOutput.Formulae[0]
		pkg := &Package{
			Name:        formula.Name,
			Version:     formula.Version,
			InstalledAt: time.Now(),
			InstallType: "explicit",
			Tap:         formula.Tap,
			IsCask:      false,
			SizeBytes:   0,
			HasBinary:   true,
			BinaryPaths: []string{},
		}

		if len(formula.Installed) > 0 && formula.Installed[0].InstalledTime > 0 {
			pkg.InstalledAt = time.Unix(formula.Installed[0].InstalledTime, 0)
		}

		return pkg, nil
	}

	// Check casks
	if len(infoOutput.Casks) > 0 {
		cask := infoOutput.Casks[0]
		pkg := &Package{
			Name:        cask.Token,
			Version:     cask.Version,
			InstalledAt: time.Now(),
			InstallType: "explicit",
			Tap:         cask.Tap,
			IsCask:      true,
			SizeBytes:   0,
			HasBinary:   false,
			BinaryPaths: []string{},
		}

		return pkg, nil
	}

	return nil, fmt.Errorf("package %s not found", name)
}

// GetDependencyTree returns the dependency tree for a package
// Returns a map where keys are package names and values are their direct dependencies
func GetDependencyTree(pkg string) (map[string][]string, error) {
	cmd := exec.Command("brew", "deps", "--tree", pkg)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("brew deps failed for %s: %w (stderr: %s)", pkg, err, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("brew deps failed for %s: %w", pkg, err)
	}

	return parseDependencyTree(string(output))
}

// parseDependencyTree parses the output of `brew deps --tree`
// Example input:
//
//	node
//	├── icu4c
//	├── libnghttp2
//	└── openssl@3
//	    └── ca-certificates
func parseDependencyTree(output string) (map[string][]string, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		return make(map[string][]string), nil
	}

	tree := make(map[string][]string)
	stack := []string{} // Stack to track parent packages at each depth

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Remove all tree drawing characters and leading/trailing spaces
		// Tree characters to remove: │ ├ └ ─
		pkgName := line
		pkgName = strings.ReplaceAll(pkgName, "│", "")
		pkgName = strings.ReplaceAll(pkgName, "├", "")
		pkgName = strings.ReplaceAll(pkgName, "└", "")
		pkgName = strings.ReplaceAll(pkgName, "─", "")
		pkgName = strings.TrimSpace(pkgName)

		if pkgName == "" {
			continue
		}

		// Calculate depth by counting leading characters in original line
		// Standard tree output uses 4 characters per level
		leadingSpaces := 0
		for _, ch := range line {
			if ch == ' ' || ch == '│' || ch == '├' || ch == '└' || ch == '─' {
				leadingSpaces++
			} else {
				break
			}
		}

		// Determine the depth level (each level is ~4 chars)
		depth := leadingSpaces / 4
		if leadingSpaces > 0 && depth == 0 {
			depth = 1
		}

		// Root package (depth 0)
		if depth == 0 {
			stack = []string{pkgName}
			if _, exists := tree[pkgName]; !exists {
				tree[pkgName] = []string{}
			}
			continue
		}

		// Adjust stack to current depth
		if depth > len(stack) {
			depth = len(stack)
		}
		stack = stack[:depth]

		// Add to parent's dependencies
		if len(stack) > 0 {
			parent := stack[len(stack)-1]

			// Check if this dependency already exists for this parent
			alreadyExists := false
			for _, dep := range tree[parent] {
				if dep == pkgName {
					alreadyExists = true
					break
				}
			}

			if !alreadyExists {
				tree[parent] = append(tree[parent], pkgName)
			}

			// Initialize entry for this package if not exists
			if _, exists := tree[pkgName]; !exists {
				tree[pkgName] = []string{}
			}
		}

		// Push current package onto stack
		stack = append(stack, pkgName)
	}

	return tree, nil
}

// GetBrewPrefix returns the Homebrew installation prefix
func GetBrewPrefix() (string, error) {
	cmd := exec.Command("brew", "--prefix")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("brew --prefix failed: %w (stderr: %s)", err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("brew --prefix failed: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// PackageExists checks if a package exists in available formulae/casks
func PackageExists(name string) (bool, error) {
	// Try as formula first
	cmd := exec.Command("brew", "search", "--formula", fmt.Sprintf("^%s$", name))
	output, err := cmd.Output()
	if err != nil {
		// Search command returns non-zero if not found, but that's not an error for us
		if exitErr, ok := err.(*exec.ExitError); ok {
			// If stderr has content, it's a real error
			if len(exitErr.Stderr) > 0 {
				return false, fmt.Errorf("brew search failed: %w (stderr: %s)", err, string(exitErr.Stderr))
			}
		}
	}

	// If we got output, package exists
	if strings.TrimSpace(string(output)) != "" {
		return true, nil
	}

	// Try as cask
	cmd = exec.Command("brew", "search", "--cask", fmt.Sprintf("^%s$", name))
	output, err = cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if len(exitErr.Stderr) > 0 {
				return false, fmt.Errorf("brew search failed: %w (stderr: %s)", err, string(exitErr.Stderr))
			}
		}
	}

	return strings.TrimSpace(string(output)) != "", nil
}
