// Package shell provides utilities for writing shell configuration files.
package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnsurePathEntry checks whether dir is on PATH and, if not, appends the
// export line to the appropriate shell config file.
// Returns (added bool, configFile string, err error).
// added=false means it was already on PATH (no change made).
func EnsurePathEntry(dir string) (added bool, configFile string, err error) {
	// Check if dir is already in the current PATH env var.
	pathEnv := os.Getenv("PATH")
	for _, entry := range filepath.SplitList(pathEnv) {
		if entry == dir {
			return false, "", nil
		}
	}

	// Detect the user's shell and choose the config file accordingly.
	shellPath := os.Getenv("SHELL")
	shellName := filepath.Base(shellPath)

	home, err := os.UserHomeDir()
	if err != nil {
		return false, "", fmt.Errorf("cannot determine home directory: %w", err)
	}

	var configPath string
	var isFish bool

	switch shellName {
	case "zsh":
		configPath = filepath.Join(home, ".zprofile")
	case "bash":
		configPath = filepath.Join(home, ".bash_profile")
	case "fish":
		configPath = filepath.Join(home, ".config", "fish", "conf.d", "brewprune.fish")
		isFish = true
	default:
		configPath = filepath.Join(home, ".profile")
	}

	// Ensure the parent directory exists (needed for fish conf.d path).
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return false, "", fmt.Errorf("cannot create config directory %s: %w", filepath.Dir(configPath), err)
	}

	// Check if the entry already exists in the config file
	existingContent, readErr := os.ReadFile(configPath)
	if readErr == nil {
		// File exists, check if brewprune shims marker is present
		if strings.Contains(string(existingContent), "# brewprune shims") {
			// Already configured
			return false, configPath, nil
		}
	}

	// Build the export line to append.
	var line string
	if isFish {
		line = fmt.Sprintf("\n# brewprune shims\nfish_add_path %s\n", dir)
	} else {
		line = fmt.Sprintf("\n# brewprune shims\nexport PATH=%q:$PATH\n", dir)
	}

	// Open the file for appending, creating it if it doesn't exist.
	f, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return false, "", fmt.Errorf("cannot open config file %s: %w", configPath, err)
	}
	defer f.Close()

	if _, err := fmt.Fprint(f, line); err != nil {
		return false, "", fmt.Errorf("cannot write to config file %s: %w", configPath, err)
	}

	return true, configPath, nil
}
