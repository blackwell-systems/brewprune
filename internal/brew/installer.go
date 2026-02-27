package brew

import (
	"fmt"
	"os/exec"
	"strings"
)

// Uninstall removes a package via brew uninstall
func Uninstall(pkgName string) error {
	cmd := exec.Command("brew", "uninstall", pkgName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("brew uninstall %s failed: %w (output: %s)", pkgName, err, string(output))
	}
	return nil
}

// Install installs a specific version of a package via brew install
// If version is empty, installs the latest version
func Install(pkgName, version string) error {
	var fullName string
	if version != "" {
		// Homebrew uses @ syntax for versioned packages (e.g., node@16)
		// If the version is already in the package name, use it as-is
		if strings.Contains(pkgName, "@") {
			fullName = pkgName
		} else {
			fullName = fmt.Sprintf("%s@%s", pkgName, version)
		}
	} else {
		fullName = pkgName
	}

	cmd := exec.Command("brew", "install", fullName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("brew install %s failed: %w (output: %s)", fullName, err, string(output))
	}
	return nil
}

// AddTap adds a Homebrew tap if not already present
func AddTap(tap string) error {
	// Check if tap already exists first
	exists, err := TapExists(tap)
	if err != nil {
		return fmt.Errorf("failed to check if tap exists: %w", err)
	}

	if exists {
		return nil // Already tapped, nothing to do
	}

	cmd := exec.Command("brew", "tap", tap)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("brew tap %s failed: %w (output: %s)", tap, err, string(output))
	}
	return nil
}

// TapExists checks if a tap is already added
func TapExists(tap string) (bool, error) {
	cmd := exec.Command("brew", "tap")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return false, fmt.Errorf("brew tap failed: %w (stderr: %s)", err, string(exitErr.Stderr))
		}
		return false, fmt.Errorf("brew tap failed: %w", err)
	}

	taps := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, t := range taps {
		if strings.TrimSpace(t) == tap {
			return true, nil
		}
	}

	return false, nil
}
